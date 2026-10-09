package sql

import (
	"github.com/impishMD/taskexec/db"
	"github.com/impishMD/taskexec/pkg/common_errors"
	"github.com/impishMD/taskexec/pkg/tz"
)

const workflowNodeColumns = "id,workflow_template_id,revision_id,template_id,kind,convergence_mode,approval_timeout,approval_message,task_params_id,note,delay_seconds,position_x,position_y"

func (d *SqlDb) GetWorkflowTemplates(projectID int, params db.RetrieveQueryParams) ([]db.WorkflowTemplate, error) {
	items := []db.WorkflowTemplate{}
	_, err := d.selectAll(&items, "select * from project__workflow_template where project_id=? order by name,id", projectID)
	if err != nil {
		return nil, err
	}
	for i := range items {
		items[i], err = d.GetWorkflowTemplate(projectID, items[i].ID)
		if err != nil {
			return nil, err
		}
		runs, e := d.GetWorkflowRuns(projectID, items[i].ID, db.RetrieveQueryParams{Count: 1})
		if e != nil {
			return nil, e
		}
		if len(runs) > 0 {
			items[i].LastRun = &runs[0]
		}
	}
	return items, nil
}
func (d *SqlDb) GetWorkflowTemplate(p, id int) (db.WorkflowTemplate, error) {
	var revision db.WorkflowRevision
	err := d.selectOne(&revision, "select * from project__workflow_revision where project_id=? and workflow_template_id=? order by number desc limit 1", p, id)
	if err != nil {
		return db.WorkflowTemplate{}, err
	}
	return d.GetWorkflowRevisionGraph(p, revision.ID)
}
func (d *SqlDb) GetWorkflowRevisionGraph(p, revisionID int) (workflow db.WorkflowTemplate, err error) {
	var revision db.WorkflowRevision
	err = d.selectOne(&revision, "select * from project__workflow_revision where project_id=? and id=?", p, revisionID)
	if err != nil {
		return
	}
	err = d.selectOne(&workflow, "select * from project__workflow_template where project_id=? and id=?", p, revision.WorkflowTemplateID)
	if err != nil {
		return
	}
	workflow.RevisionID, workflow.Revision = revision.ID, revision.Number
	workflow.Nodes, workflow.Edges = []db.WorkflowNode{}, []db.WorkflowEdge{}
	_, err = d.selectAll(&workflow.Nodes, "select "+workflowNodeColumns+" from project__workflow_node where workflow_template_id=? and revision_id=? order by id", workflow.ID, revision.ID)
	if err != nil {
		return
	}
	_, err = d.selectAll(&workflow.Edges, "select * from project__workflow_edge where workflow_template_id=? and revision_id=? order by id", workflow.ID, revision.ID)
	if err != nil {
		return
	}
	for i := range workflow.Nodes {
		if id := workflow.Nodes[i].TaskParamsID; id != nil {
			var params db.TaskParams
			err = d.selectOne(&params, "select * from project__task_params where project_id=? and id=?", p, *id)
			if err != nil {
				return
			}
			workflow.Nodes[i].TaskParams = &params
		}
	}
	return
}
func (d *SqlDb) GetWorkflowRevisions(p, id int) ([]db.WorkflowRevision, error) {
	if _, err := d.GetWorkflowTemplate(p, id); err != nil {
		return nil, err
	}
	revisions := []db.WorkflowRevision{}
	_, err := d.selectAll(&revisions, "select * from project__workflow_revision where project_id=? and workflow_template_id=? order by number desc", p, id)
	if err != nil {
		return nil, err
	}
	for i := range revisions {
		var count int
		if err = d.selectOne(&count, "select count(*) from project__workflow_run where project_id=? and revision_id=?", p, revisions[i].ID); err != nil {
			return nil, err
		}
		revisions[i].HasRuns = count > 0
	}
	return revisions, nil
}
func (d *SqlDb) saveWorkflow(workflow db.WorkflowTemplate, create bool) (db.WorkflowTemplate, error) {
	if err := db.ValidateWorkflowTemplate(d, workflow); err != nil {
		return db.WorkflowTemplate{}, err
	}
	for _, node := range workflow.Nodes {
		if node.TaskParams != nil && node.TaskParams.InventoryID != nil {
			inv, err := d.GetInventory(workflow.ProjectID, *node.TaskParams.InventoryID)
			if err != nil {
				return db.WorkflowTemplate{}, common_errors.NewValidationError("workflow inventory must belong to this project")
			}
			template, err := d.GetTemplate(workflow.ProjectID, node.TemplateID)
			if err != nil {
				return db.WorkflowTemplate{}, err
			}
			if !template.App.HasInventoryType(inv.Type) {
				return db.WorkflowTemplate{}, common_errors.NewValidationError("workflow inventory type does not match template")
			}
		}
	}
	tx, err := d.Sql().Begin()
	if err != nil {
		return db.WorkflowTemplate{}, err
	}
	defer func() { _ = tx.Rollback() }()
	if create {
		workflow.ID = 0
		if err = tx.Insert(&workflow); err != nil {
			return db.WorkflowTemplate{}, err
		}
	} else {
		// The row update serializes revision allocation across concurrent saves.
		_, err = d.execTx(tx, "update project__workflow_template set name=?,description=?,start_version=? where project_id=? and id=?", workflow.Name, workflow.Description, workflow.StartVersion, workflow.ProjectID, workflow.ID)
		if err != nil {
			return db.WorkflowTemplate{}, err
		}
		count, e := tx.SelectInt(d.PrepareQuery("select count(*) from project__workflow_template where project_id=? and id=?"), workflow.ProjectID, workflow.ID)
		if e != nil {
			return db.WorkflowTemplate{}, e
		}
		if count != 1 {
			return db.WorkflowTemplate{}, db.ErrNotFound
		}
	}
	number, err := tx.SelectInt(d.PrepareQuery("select coalesce(max(number),0)+1 from project__workflow_revision where project_id=? and workflow_template_id=?"), workflow.ProjectID, workflow.ID)
	if err != nil {
		return db.WorkflowTemplate{}, err
	}
	revision := db.WorkflowRevision{ProjectID: workflow.ProjectID, WorkflowTemplateID: workflow.ID, Number: int(number), Created: tz.Now(), CreatedByUserID: workflow.RevisionAuthorID}
	if err = tx.Insert(&revision); err != nil {
		return db.WorkflowTemplate{}, err
	}
	ids := map[int]int{}
	for _, original := range workflow.Nodes {
		node := original
		node.ID, node.WorkflowTemplateID, node.RevisionID = 0, workflow.ID, revision.ID
		node.Kind, node.ConvergenceMode = node.EffectiveKind(), node.EffectiveConvergenceMode()
		node.TaskParamsID = nil
		if node.Kind != db.WorkflowNodeTaskKind {
			node.TemplateID = 0
			node.TaskParams = nil
		}
		if node.TaskParams != nil {
			params := *node.TaskParams
			params.ID, params.ProjectID = 0, workflow.ProjectID
			if err = tx.Insert(&params); err != nil {
				return db.WorkflowTemplate{}, err
			}
			node.TaskParamsID = &params.ID
		}
		if err = tx.Insert(&node); err != nil {
			return db.WorkflowTemplate{}, err
		}
		ids[original.ID] = node.ID
	}
	for _, original := range workflow.Edges {
		edge := original
		edge.ID, edge.WorkflowTemplateID, edge.RevisionID = 0, workflow.ID, revision.ID
		edge.SourceNodeID, edge.DestinationNodeID = ids[original.SourceNodeID], ids[original.DestinationNodeID]
		if err = tx.Insert(&edge); err != nil {
			return db.WorkflowTemplate{}, err
		}
	}
	if err = tx.Commit(); err != nil {
		return db.WorkflowTemplate{}, err
	}
	return d.GetWorkflowRevisionGraph(workflow.ProjectID, revision.ID)
}
func (d *SqlDb) CreateWorkflowTemplate(w db.WorkflowTemplate) (db.WorkflowTemplate, error) {
	return d.saveWorkflow(w, true)
}
func (d *SqlDb) UpdateWorkflowTemplate(w db.WorkflowTemplate) error {
	_, err := d.saveWorkflow(w, false)
	return err
}
func (d *SqlDb) DeleteWorkflowTemplate(p, id int) error {
	tx, err := d.Sql().Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	_, err = d.execTx(tx, "update project__workflow_template set name=name where project_id=? and id=?", p, id)
	if err != nil {
		return err
	}
	count, err := tx.SelectInt(d.PrepareQuery("select count(*) from project__workflow_run where project_id=? and workflow_template_id=? and status in ('running','approval')"), p, id)
	if err != nil {
		return err
	}
	if count > 0 {
		return db.ErrInvalidOperation
	}
	// Explicit deletion removes workflow runs, but keeps their ordinary task
	// logs. Their graph associations must be cleared before removing the runs.
	_, err = d.execTx(tx, "update task set workflow_run_id=null,workflow_node_id=null where project_id=? and workflow_run_id in (select id from project__workflow_run where project_id=? and workflow_template_id=?)", p, p, id)
	if err != nil {
		return err
	}
	_, err = d.execTx(tx, "delete from project__workflow_run where project_id=? and workflow_template_id=?", p, id)
	if err != nil {
		return err
	}
	var paramsIDs []int
	if _, err = tx.Select(&paramsIDs, d.PrepareQuery("select distinct task_params_id from project__workflow_node where workflow_template_id=? and task_params_id is not null"), id); err != nil {
		return err
	}
	err = requireDeletedRow(d.execTx(tx, "delete from project__workflow_template where project_id=? and id=?", p, id))
	if err != nil {
		return err
	}
	for _, paramsID := range paramsIDs {
		// Legacy imports may share parameters with a schedule or integration.
		// Delete only this graph's parameter rows that have no remaining owner.
		_, err = d.execTx(tx, `delete from project__task_params where project_id=? and id=?
            and not exists (select 1 from project__workflow_node where task_params_id=?)
            and not exists (select 1 from project__schedule where task_params_id=?)
            and not exists (select 1 from project__integration where task_params_id=?)`, p, paramsID, paramsID, paramsID, paramsID)
		if err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (d *SqlDb) GetWorkflowRuns(p, workflowID int, params db.RetrieveQueryParams) ([]db.WorkflowRun, error) {
	items := []db.WorkflowRun{}
	query := "select * from project__workflow_run where project_id=? and workflow_template_id=? order by id desc"
	args := []any{p, workflowID}
	if params.Count > 0 {
		query += " limit ?"
		args = append(args, params.Count)
	}
	_, err := d.selectAll(&items, query, args...)
	return items, err
}
func (d *SqlDb) GetWorkflowRun(p, workflowID, id int) (run db.WorkflowRun, err error) {
	err = d.selectOne(&run, "select * from project__workflow_run where project_id=? and workflow_template_id=? and id=?", p, workflowID, id)
	return
}
func (d *SqlDb) GetWorkflowRunByID(p, id int) (run db.WorkflowRun, err error) {
	err = d.selectOne(&run, "select * from project__workflow_run where project_id=? and id=?", p, id)
	return
}
func (d *SqlDb) GetActiveWorkflowRuns() ([]db.WorkflowRun, error) {
	items := []db.WorkflowRun{}
	_, err := d.selectAll(&items, "select * from project__workflow_run where status in ('running','approval') order by id")
	return items, err
}
func (d *SqlDb) CreateWorkflowRun(run db.WorkflowRun) (db.WorkflowRun, error) {
	tx, err := d.Sql().Begin()
	if err != nil {
		return run, err
	}
	defer func() { _ = tx.Rollback() }()
	_, err = d.execTx(tx, "update project__workflow_template set name=name where project_id=? and id=?", run.ProjectID, run.WorkflowTemplateID)
	if err != nil {
		return run, err
	}
	var template db.WorkflowTemplate
	err = tx.SelectOne(&template, d.PrepareQuery("select * from project__workflow_template where project_id=? and id=?"), run.ProjectID, run.WorkflowTemplateID)
	if err != nil {
		return run, err
	}
	count, err := tx.SelectInt(d.PrepareQuery("select count(*) from project__workflow_revision where id=? and project_id=? and workflow_template_id=?"), run.RevisionID, run.ProjectID, run.WorkflowTemplateID)
	if err != nil {
		return run, err
	}
	if count != 1 {
		return run, db.ErrNotFound
	}
	if template.StartVersion != nil && *template.StartVersion != "" {
		var latest []db.WorkflowRun
		_, err = tx.Select(&latest, d.PrepareQuery("select * from project__workflow_run where project_id=? and workflow_template_id=? order by id desc limit 1"), run.ProjectID, run.WorkflowTemplateID)
		if err != nil {
			return run, err
		}
		run.Version = template.StartVersion
		if len(latest) > 0 && latest[0].Version != nil {
			next := db.GetNextBuildVersion(*template.StartVersion, *latest[0].Version)
			run.Version = &next
		}
	}
	run.ID = 0
	run.Status = db.WorkflowRunRunning
	run.Start = new(tz.Now())
	run.End = nil
	run.RootTaskID = nil
	if err = tx.Insert(&run); err != nil {
		return run, err
	}
	err = tx.Commit()
	return run, err
}
func (d *SqlDb) UpdateWorkflowRun(run db.WorkflowRun) error {
	_, err := d.UpdateWorkflowRunStatusUnless(run, nil)
	return err
}
func (d *SqlDb) UpdateWorkflowRunStatusUnless(run db.WorkflowRun, excluded []db.WorkflowRunStatus) (bool, error) {
	if _, err := d.GetWorkflowRun(run.ProjectID, run.WorkflowTemplateID, run.ID); err != nil {
		return false, err
	}
	query := "update project__workflow_run set status=?, `end`=? where project_id=? and workflow_template_id=? and id=?"
	args := []any{run.Status, run.End, run.ProjectID, run.WorkflowTemplateID, run.ID}
	for _, status := range excluded {
		query += " and status<>?"
		args = append(args, status)
	}
	result, err := d.exec(query, args...)
	if err != nil {
		return false, err
	}
	count, err := result.RowsAffected()
	return count > 0, err
}
func (d *SqlDb) SetWorkflowRunRootTask(p, runID, taskID int) (bool, error) {
	task, err := d.GetTask(p, taskID)
	if err != nil {
		return false, err
	}
	if task.WorkflowRunID == nil || *task.WorkflowRunID != runID {
		return false, db.ErrNotFound
	}
	result, err := d.exec("update project__workflow_run set root_task_id=? where project_id=? and id=? and root_task_id is null", taskID, p, runID)
	if err != nil {
		return false, err
	}
	count, err := result.RowsAffected()
	return count > 0, err
}

func (d *SqlDb) workflowRunNode(p, runID, nodeID int, kind db.WorkflowNodeKind) error {
	var count int
	err := d.selectOne(&count, "select count(*) from project__workflow_run r join project__workflow_node n on n.revision_id=r.revision_id and n.workflow_template_id=r.workflow_template_id where r.project_id=? and r.id=? and n.id=? and n.kind=?", p, runID, nodeID, kind)
	if err != nil {
		return err
	}
	if count != 1 {
		return db.ErrNotFound
	}
	return nil
}
func (d *SqlDb) GetWorkflowApprovals(p, runID int) ([]db.WorkflowApproval, error) {
	items := []db.WorkflowApproval{}
	_, err := d.selectAll(&items, "select * from project__workflow_approval where project_id=? and workflow_run_id=? order by id", p, runID)
	return items, err
}
func (d *SqlDb) GetWorkflowApproval(p, runID, nodeID int) (item db.WorkflowApproval, err error) {
	err = d.selectOne(&item, "select * from project__workflow_approval where project_id=? and workflow_run_id=? and workflow_node_id=?", p, runID, nodeID)
	return
}
func (d *SqlDb) CreateWorkflowApproval(item db.WorkflowApproval) (db.WorkflowApproval, error) {
	if err := d.workflowRunNode(item.ProjectID, item.WorkflowRunID, item.WorkflowNodeID, db.WorkflowNodeApprovalKind); err != nil {
		return item, err
	}
	item.ID = 0
	item.Created = tz.Now()
	item.Status = db.WorkflowApprovalPending
	item.Resolved = nil
	item.ResolvedByUserID = nil
	err := d.Sql().Insert(&item)
	return item, err
}
func (d *SqlDb) UpdateWorkflowApproval(item db.WorkflowApproval) error {
	_, err := d.ResolveWorkflowApprovalIfPending(item)
	return err
}
func (d *SqlDb) ResolveWorkflowApprovalIfPending(item db.WorkflowApproval) (bool, error) {
	if item.Status != db.WorkflowApprovalApproved && item.Status != db.WorkflowApprovalRejected {
		return false, common_errors.NewValidationError("approval must be approved or rejected")
	}
	if _, err := d.GetWorkflowApproval(item.ProjectID, item.WorkflowRunID, item.WorkflowNodeID); err != nil {
		return false, err
	}
	result, err := d.exec("update project__workflow_approval set status=?,resolved=?,resolved_by_user_id=? where project_id=? and workflow_run_id=? and workflow_node_id=? and status='pending'", item.Status, tz.Now(), item.ResolvedByUserID, item.ProjectID, item.WorkflowRunID, item.WorkflowNodeID)
	if err != nil {
		return false, err
	}
	count, err := result.RowsAffected()
	return count > 0, err
}
func (d *SqlDb) GetWorkflowDelays(p, runID int) ([]db.WorkflowDelay, error) {
	items := []db.WorkflowDelay{}
	_, err := d.selectAll(&items, "select * from project__workflow_delay where project_id=? and workflow_run_id=? order by id", p, runID)
	return items, err
}
func (d *SqlDb) GetWorkflowDelay(p, runID, nodeID int) (item db.WorkflowDelay, err error) {
	err = d.selectOne(&item, "select * from project__workflow_delay where project_id=? and workflow_run_id=? and workflow_node_id=?", p, runID, nodeID)
	return
}
func (d *SqlDb) CreateWorkflowDelay(item db.WorkflowDelay) (db.WorkflowDelay, error) {
	if err := d.workflowRunNode(item.ProjectID, item.WorkflowRunID, item.WorkflowNodeID, db.WorkflowNodeDelayKind); err != nil {
		return item, err
	}
	item.ID = 0
	item.Created = tz.Now()
	item.Status = db.WorkflowDelayWaiting
	item.Resolved = nil
	err := d.Sql().Insert(&item)
	return item, err
}
func (d *SqlDb) UpdateWorkflowDelay(item db.WorkflowDelay) error {
	_, err := d.ResolveWorkflowDelayIfWaiting(item)
	return err
}
func (d *SqlDb) ResolveWorkflowDelayIfWaiting(item db.WorkflowDelay) (bool, error) {
	if item.Status != db.WorkflowDelaySuccess && item.Status != db.WorkflowDelayStopped {
		return false, common_errors.NewValidationError("delay must be finished or stopped")
	}
	if _, err := d.GetWorkflowDelay(item.ProjectID, item.WorkflowRunID, item.WorkflowNodeID); err != nil {
		return false, err
	}
	result, err := d.exec("update project__workflow_delay set status=?,resolved=? where project_id=? and workflow_run_id=? and workflow_node_id=? and status='waiting'", item.Status, tz.Now(), item.ProjectID, item.WorkflowRunID, item.WorkflowNodeID)
	if err != nil {
		return false, err
	}
	count, err := result.RowsAffected()
	return count > 0, err
}
func (d *SqlDb) GetExpiredWorkflowDelays() ([]db.WorkflowDelay, error) {
	items := []db.WorkflowDelay{}
	_, err := d.selectAll(&items, "select * from project__workflow_delay where status='waiting' and resume_at<=? order by id", tz.Now())
	return items, err
}

var _ db.WorkflowManager = (*SqlDb)(nil)
