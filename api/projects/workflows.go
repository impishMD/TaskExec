package projects

import (
	"errors"
	"net/http"

	"github.com/impishMD/taskexec/api/helpers"
	"github.com/impishMD/taskexec/db"
	"github.com/impishMD/taskexec/services/audit"
	"github.com/impishMD/taskexec/services/server"
)

// WorkflowController loads workflow-related entities into the request context
// for the workflow route middleware chain.
type WorkflowController struct {
	workflowRepo db.WorkflowManager
	service      server.WorkflowService
}

func NewWorkflowController(workflowRepo db.WorkflowManager, service server.WorkflowService) *WorkflowController {
	return &WorkflowController{
		workflowRepo: workflowRepo,
		service:      service,
	}
}

func workflowError(w http.ResponseWriter, err error) {
	if errors.Is(err, db.ErrInvalidOperation) {
		helpers.WriteErrorStatus(w, "Workflow action conflicts with its current state", http.StatusConflict)
		return
	}
	helpers.WriteError(w, err)
}
func (c *WorkflowController) GetWorkflows(w http.ResponseWriter, r *http.Request) {
	project := helpers.GetFromContext(r, "project").(db.Project)
	items, err := c.workflowRepo.GetWorkflowTemplates(project.ID, helpers.QueryParams(r.URL))
	if err != nil {
		workflowError(w, err)
		return
	}
	helpers.WriteJSON(w, http.StatusOK, items)
}
func (c *WorkflowController) GetWorkflow(w http.ResponseWriter, r *http.Request) {
	helpers.WriteJSON(w, http.StatusOK, helpers.GetFromContext(r, "workflow"))
}
func (c *WorkflowController) saveWorkflow(w http.ResponseWriter, r *http.Request, create bool) {
	project := helpers.GetFromContext(r, "project").(db.Project)
	var workflow db.WorkflowTemplate
	if !helpers.Bind(w, r, &workflow) {
		return
	}
	workflow.ProjectID = project.ID
	workflow.RevisionAuthorID = &helpers.UserFromContext(r).ID
	var err error
	kind := audit.ResourceWorkflowCreate
	if create {
		workflow, err = c.workflowRepo.CreateWorkflowTemplate(workflow)
	} else {
		workflow.ID = helpers.GetFromContext(r, "workflow").(db.WorkflowTemplate).ID
		err = c.workflowRepo.UpdateWorkflowTemplate(workflow)
		if err == nil {
			workflow, err = c.workflowRepo.GetWorkflowTemplate(project.ID, workflow.ID)
		}
		kind = audit.ResourceWorkflowUpdate
	}
	if err != nil {
		workflowError(w, err)
		return
	}
	helpers.Audit(r).Record(r.Context(), audit.Event{Kind: kind, ProjectID: project.ID, Target: audit.ResourceTarget(audit.TargetWorkflow, workflow.ID, workflow.Name), Metadata: audit.WorkflowMetadata{RevisionID: workflow.RevisionID}})
	if create {
		helpers.WriteJSON(w, http.StatusCreated, workflow)
	} else {
		w.WriteHeader(http.StatusNoContent)
	}
}
func (c *WorkflowController) AddWorkflow(w http.ResponseWriter, r *http.Request) {
	c.saveWorkflow(w, r, true)
}
func (c *WorkflowController) UpdateWorkflow(w http.ResponseWriter, r *http.Request) {
	c.saveWorkflow(w, r, false)
}
func (c *WorkflowController) RemoveWorkflow(w http.ResponseWriter, r *http.Request) {
	workflow := helpers.GetFromContext(r, "workflow").(db.WorkflowTemplate)
	if err := c.workflowRepo.DeleteWorkflowTemplate(workflow.ProjectID, workflow.ID); err != nil {
		workflowError(w, err)
		return
	}
	helpers.Audit(r).Record(r.Context(), audit.Event{Kind: audit.ResourceWorkflowDelete, ProjectID: workflow.ProjectID, Target: audit.ResourceTarget(audit.TargetWorkflow, workflow.ID, workflow.Name), Metadata: audit.WorkflowMetadata{}})
	w.WriteHeader(http.StatusNoContent)
}
func (c *WorkflowController) GetWorkflowRevisions(w http.ResponseWriter, r *http.Request) {
	workflow := helpers.GetFromContext(r, "workflow").(db.WorkflowTemplate)
	items, err := c.workflowRepo.GetWorkflowRevisions(workflow.ProjectID, workflow.ID)
	if err != nil {
		workflowError(w, err)
		return
	}
	helpers.WriteJSON(w, http.StatusOK, items)
}
func (c *WorkflowController) GetWorkflowRevision(w http.ResponseWriter, r *http.Request) {
	workflow := helpers.GetFromContext(r, "workflow").(db.WorkflowTemplate)
	id, ok := helpers.GetIntParamOrAbort("revision_id", w, r)
	if !ok {
		return
	}
	graph, err := c.workflowRepo.GetWorkflowRevisionGraph(workflow.ProjectID, id)
	if err != nil {
		workflowError(w, err)
		return
	}
	if graph.ID != workflow.ID {
		http.NotFound(w, r)
		return
	}
	helpers.WriteJSON(w, http.StatusOK, graph)
}
func (c *WorkflowController) RunWorkflow(w http.ResponseWriter, r *http.Request) {
	workflow := helpers.GetFromContext(r, "workflow").(db.WorkflowTemplate)
	run, err := c.service.StartWorkflow(workflow, helpers.UserFromContext(r))
	if err != nil {
		workflowError(w, err)
		return
	}
	helpers.WriteJSON(w, http.StatusCreated, run)
}
func (c *WorkflowController) StopWorkflowRun(w http.ResponseWriter, r *http.Request) {
	run := helpers.GetFromContext(r, "workflow_run").(db.WorkflowRun)
	run, err := c.service.StopWorkflowRun(run.ProjectID, run.ID, helpers.UserFromContext(r))
	if err != nil {
		workflowError(w, err)
		return
	}
	helpers.WriteJSON(w, http.StatusOK, run)
}
func (c *WorkflowController) GetWorkflowRuns(w http.ResponseWriter, r *http.Request) {
	workflow := helpers.GetFromContext(r, "workflow").(db.WorkflowTemplate)
	items, err := c.workflowRepo.GetWorkflowRuns(workflow.ProjectID, workflow.ID, helpers.QueryParams(r.URL))
	if err != nil {
		workflowError(w, err)
		return
	}
	helpers.WriteJSON(w, http.StatusOK, items)
}
func (c *WorkflowController) GetWorkflowRun(w http.ResponseWriter, r *http.Request) {
	run := helpers.GetFromContext(r, "workflow_run").(db.WorkflowRun)
	graph, err := c.workflowRepo.GetWorkflowRevisionGraph(run.ProjectID, run.RevisionID)
	if err != nil {
		workflowError(w, err)
		return
	}
	tasks, err := c.workflowRepo.GetWorkflowRunTasks(run.ProjectID, run.ID, db.RetrieveQueryParams{})
	if err != nil {
		workflowError(w, err)
		return
	}
	approvals, err := c.workflowRepo.GetWorkflowApprovals(run.ProjectID, run.ID)
	if err != nil {
		workflowError(w, err)
		return
	}
	delays, err := c.workflowRepo.GetWorkflowDelays(run.ProjectID, run.ID)
	if err != nil {
		workflowError(w, err)
		return
	}
	type nodeRun struct {
		Node     db.WorkflowNode      `json:"node"`
		Task     *db.TaskWithTpl      `json:"task,omitempty"`
		Approval *db.WorkflowApproval `json:"approval,omitempty"`
		Delay    *db.WorkflowDelay    `json:"delay,omitempty"`
	}
	nodes := make([]nodeRun, 0, len(graph.Nodes))
	for _, node := range graph.Nodes {
		entry := nodeRun{Node: node}
		for _, task := range tasks {
			if task.WorkflowNodeID != nil && *task.WorkflowNodeID == node.ID {
				entry.Task = &task
				break
			}
		}
		for _, approval := range approvals {
			if approval.WorkflowNodeID == node.ID {
				entry.Approval = &approval
				break
			}
		}
		for _, delay := range delays {
			if delay.WorkflowNodeID == node.ID {
				entry.Delay = &delay
				break
			}
		}
		nodes = append(nodes, entry)
	}
	helpers.WriteJSON(w, http.StatusOK, map[string]any{"run": run, "workflow_name": graph.Name, "revision": graph.Revision, "nodes": nodes, "edges": graph.Edges})
}
func (c *WorkflowController) GetWorkflowRunArtifacts(w http.ResponseWriter, r *http.Request) {
	run := helpers.GetFromContext(r, "workflow_run").(db.WorkflowRun)
	result, err := c.service.GetWorkflowRunArtifacts(run.ProjectID, run.ID, nil)
	if err != nil {
		workflowError(w, err)
		return
	}
	helpers.WriteJSON(w, http.StatusOK, result)
}
func (c *WorkflowController) GetWorkflowApprovals(w http.ResponseWriter, r *http.Request) {
	run := helpers.GetFromContext(r, "workflow_run").(db.WorkflowRun)
	items, err := c.workflowRepo.GetWorkflowApprovals(run.ProjectID, run.ID)
	if err != nil {
		workflowError(w, err)
		return
	}
	helpers.WriteJSON(w, http.StatusOK, items)
}
func (c *WorkflowController) ResolveWorkflowApproval(w http.ResponseWriter, r *http.Request) {
	run := helpers.GetFromContext(r, "workflow_run").(db.WorkflowRun)
	id, ok := helpers.GetIntParamOrAbort("node_id", w, r)
	if !ok {
		return
	}
	var body struct {
		Status db.WorkflowApprovalStatus `json:"status"`
	}
	if !helpers.Bind(w, r, &body) {
		return
	}
	item, err := c.service.ResolveWorkflowApproval(run.ProjectID, run.WorkflowTemplateID, run.ID, id, body.Status, helpers.UserFromContext(r))
	if err != nil {
		workflowError(w, err)
		return
	}
	helpers.WriteJSON(w, http.StatusOK, item)
}

func (c *WorkflowController) WorkflowsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		project := helpers.GetFromContext(r, "project").(db.Project)
		workflowID, ok := helpers.GetIntParamOrAbort("workflow_id", w, r)
		if !ok {
			return
		}

		workflow, err := c.workflowRepo.GetWorkflowTemplate(project.ID, workflowID)
		if err != nil {
			helpers.WriteError(w, err)
			return
		}

		r = helpers.SetContextValue(r, "workflow", workflow)
		next.ServeHTTP(w, r)
	})
}

func (c *WorkflowController) WorkflowRunsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		project := helpers.GetFromContext(r, "project").(db.Project)
		workflow := helpers.GetFromContext(r, "workflow").(db.WorkflowTemplate)

		runID, ok := helpers.GetIntParamOrAbort("run_id", w, r)
		if !ok {
			return
		}

		run, err := c.workflowRepo.GetWorkflowRun(project.ID, workflow.ID, runID)
		if err != nil {
			helpers.WriteError(w, err)
			return
		}

		r = helpers.SetContextValue(r, "workflow_run", run)
		next.ServeHTTP(w, r)
	})
}
