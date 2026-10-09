package server

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/impishMD/taskexec/db"
	"github.com/impishMD/taskexec/db/sql"
	"github.com/impishMD/taskexec/pkg/task_logger"
	"github.com/impishMD/taskexec/pkg/tz"
	"github.com/impishMD/taskexec/util"
	"github.com/stretchr/testify/require"
)

type workflowQueueFixture struct{ store *sql.SqlDb }

func (q *workflowQueueFixture) AddTask(task db.Task, uid *int, _ string, p int, _ bool) (db.Task, error) {
	task.ProjectID, task.UserID, task.Created, task.Status = p, uid, tz.Now(), task_logger.TaskWaitingStatus
	return q.store.CreateTask(task, 0)
}
func (q *workflowQueueFixture) StopTasksByWorkflowRun(p, run int, _ bool) {
	tasks, _ := q.store.GetWorkflowRunTasks(p, run, db.RetrieveQueryParams{})
	for _, task := range tasks {
		if !task.Status.IsFinished() {
			task.Status = task_logger.TaskStoppedStatus
			_ = q.store.UpdateTask(task.Task)
		}
	}
}
func workflowServiceFixture(t *testing.T) (*sql.SqlDb, db.Project, db.Template, *workflowQueueFixture, WorkflowService) {
	t.Helper()
	old := util.Config
	store := sql.InitConfigCreateTestStore()
	t.Cleanup(func() { store.Close(); util.Config = old })
	util.Config.Apps = map[string]util.App{"bash": {}}
	p, err := store.CreateProject(db.Project{Name: "Workflow engine"})
	require.NoError(t, err)
	key, err := store.CreateAccessKey(db.AccessKey{ProjectID: &p.ID, Type: db.AccessKeyNone})
	require.NoError(t, err)
	repo, err := store.CreateRepository(db.Repository{ProjectID: p.ID, Name: "repo", GitURL: "https://example.test/repo", GitBranch: "main", SSHKeyID: key.ID})
	require.NoError(t, err)
	tpl, err := store.CreateTemplate(db.Template{ProjectID: p.ID, Name: "shell", App: db.AppBash, RepositoryID: repo.ID, Playbook: "test.sh"})
	require.NoError(t, err)
	queue := &workflowQueueFixture{store: store}
	return store, p, tpl, queue, NewWorkflowService(store, store, queue, nil)
}
func workflowTasks(t *testing.T, store *sql.SqlDb, run db.WorkflowRun) []db.TaskWithTpl {
	t.Helper()
	tasks, err := store.GetWorkflowRunTasks(run.ProjectID, run.ID, db.RetrieveQueryParams{})
	require.NoError(t, err)
	return tasks
}
func finishWorkflowTask(t *testing.T, store *sql.SqlDb, service WorkflowService, task db.Task, status task_logger.TaskStatus) {
	t.Helper()
	task.Status, task.End = status, new(tz.Now())
	require.NoError(t, store.UpdateTask(task))
	require.NoError(t, service.HandleWorkflowTaskCompletion(task))
}
func TestWorkflowEngineBranchesApprovalsDelaysAndRestart(t *testing.T) {
	for _, fail := range []bool{false, true} {
		t.Run(fmt.Sprint(fail), func(t *testing.T) {
			store, p, tpl, queue, service := workflowServiceFixture(t)
			graph, err := store.CreateWorkflowTemplate(db.WorkflowTemplate{ProjectID: p.ID, Name: "branches", Nodes: []db.WorkflowNode{
				{ID: 1, TemplateID: tpl.ID}, {ID: 2, TemplateID: tpl.ID}, {ID: 3, TemplateID: tpl.ID},
				{ID: 4, Kind: db.WorkflowNodeApprovalKind, ConvergenceMode: db.WorkflowConvergenceAny},
				{ID: 5, Kind: db.WorkflowNodeDelayKind, DelaySeconds: new(10)}, {ID: 6, TemplateID: tpl.ID},
			}, Edges: []db.WorkflowEdge{
				{SourceNodeID: 1, DestinationNodeID: 2, Condition: db.WorkflowEdgeOnSuccess},
				{SourceNodeID: 1, DestinationNodeID: 3, Condition: db.WorkflowEdgeOnFailure},
				{SourceNodeID: 2, DestinationNodeID: 4, Condition: db.WorkflowEdgeOnSuccess},
				{SourceNodeID: 3, DestinationNodeID: 4, Condition: db.WorkflowEdgeOnSuccess},
				{SourceNodeID: 4, DestinationNodeID: 5, Condition: db.WorkflowEdgeOnSuccess},
				{SourceNodeID: 5, DestinationNodeID: 6, Condition: db.WorkflowEdgeOnSuccess},
			}})
			require.NoError(t, err)
			run, err := service.StartWorkflow(graph, nil)
			require.NoError(t, err)
			tasks := workflowTasks(t, store, run)
			require.Len(t, tasks, 1)
			status := task_logger.TaskSuccessStatus
			branch := graph.Nodes[1].ID
			if fail {
				status = task_logger.TaskFailStatus
				branch = graph.Nodes[2].ID
			}
			finishWorkflowTask(t, store, service, tasks[0].Task, status)
			var wg sync.WaitGroup
			results := make(chan error, 20)
			for i := 0; i < 20; i++ {
				wg.Go(func() { results <- service.ProgressWorkflowRun(p.ID, run.ID, nil) })
			}
			wg.Wait()
			close(results)
			for err := range results {
				require.NoError(t, err)
			}
			tasks = workflowTasks(t, store, run)
			require.Len(t, tasks, 2)
			require.Equal(t, branch, *tasks[0].WorkflowNodeID)
			finishWorkflowTask(t, store, service, tasks[0].Task, task_logger.TaskSuccessStatus)
			run, err = store.GetWorkflowRunByID(p.ID, run.ID)
			require.NoError(t, err)
			require.Equal(t, db.WorkflowRunApproval, run.Status)
			_, err = service.ResolveWorkflowApproval(p.ID, graph.ID, run.ID, graph.Nodes[3].ID, db.WorkflowApprovalApproved, nil)
			require.NoError(t, err)
			_, err = service.ResolveWorkflowApproval(p.ID, graph.ID, run.ID, graph.Nodes[3].ID, db.WorkflowApprovalRejected, nil)
			require.ErrorIs(t, err, db.ErrInvalidOperation)
			require.Len(t, workflowTasks(t, store, run), 2, "delay must hold the next task")
			_, err = store.Sql().Exec("update project__workflow_delay set resume_at=? where workflow_run_id=?", tz.Now().Add(-time.Minute), run.ID)
			require.NoError(t, err)
			// Fresh service/reconciler instances restore solely from persisted rows.
			service = NewWorkflowService(store, store, queue, nil)
			reconciler := NewWorkflowReconciler(store, service)
			reconciler.Start()
			require.Eventually(t, func() bool { return len(workflowTasks(t, store, run)) == 3 }, 3*time.Second, 20*time.Millisecond)
			reconciler.Stop()
			tasks = workflowTasks(t, store, run)
			require.Equal(t, graph.Nodes[5].ID, *tasks[0].WorkflowNodeID)
			finishWorkflowTask(t, store, service, tasks[0].Task, task_logger.TaskSuccessStatus)
			run, err = store.GetWorkflowRunByID(p.ID, run.ID)
			require.NoError(t, err)
			expected := db.WorkflowRunSuccess
			if fail {
				expected = db.WorkflowRunFailed
			}
			require.Equal(t, expected, run.Status)
			require.NotNil(t, run.End)
			require.NoError(t, service.ProgressWorkflowRun(p.ID, run.ID, nil))
			require.Len(t, workflowTasks(t, store, run), 3)
		})
	}
}
func TestWorkflowEngineAllConvergenceAndStop(t *testing.T) {
	store, p, tpl, _, service := workflowServiceFixture(t)
	graph, err := store.CreateWorkflowTemplate(db.WorkflowTemplate{ProjectID: p.ID, Name: "all", Nodes: []db.WorkflowNode{{ID: 1, TemplateID: tpl.ID}, {ID: 2, TemplateID: tpl.ID}, {ID: 3, TemplateID: tpl.ID}, {ID: 4, TemplateID: tpl.ID}}, Edges: []db.WorkflowEdge{
		{SourceNodeID: 1, DestinationNodeID: 2, Condition: db.WorkflowEdgeOnSuccess}, {SourceNodeID: 1, DestinationNodeID: 3, Condition: db.WorkflowEdgeOnSuccess},
		{SourceNodeID: 2, DestinationNodeID: 4, Condition: db.WorkflowEdgeOnSuccess}, {SourceNodeID: 3, DestinationNodeID: 4, Condition: db.WorkflowEdgeOnSuccess},
	}})
	require.NoError(t, err)
	run, err := service.StartWorkflow(graph, nil)
	require.NoError(t, err)
	finishWorkflowTask(t, store, service, workflowTasks(t, store, run)[0].Task, task_logger.TaskSuccessStatus)
	tasks := workflowTasks(t, store, run)
	require.Len(t, tasks, 3)
	finishWorkflowTask(t, store, service, tasks[0].Task, task_logger.TaskSuccessStatus)
	require.Len(t, workflowTasks(t, store, run), 3, "all waits for both branches")
	run, err = service.StopWorkflowRun(p.ID, run.ID, nil)
	require.NoError(t, err)
	require.Equal(t, db.WorkflowRunStopped, run.Status)
	require.NoError(t, service.ProgressWorkflowRun(p.ID, run.ID, nil))
	require.Len(t, workflowTasks(t, store, run), 3)
	for _, task := range workflowTasks(t, store, run) {
		require.True(t, task.Status.IsFinished())
	}
}
func TestWorkflowEngineApprovalTimeoutAndArtifactScope(t *testing.T) {
	store, p, tpl, _, service := workflowServiceFixture(t)
	graph, err := store.CreateWorkflowTemplate(db.WorkflowTemplate{ProjectID: p.ID, Name: "timeout", Nodes: []db.WorkflowNode{{ID: 1, Kind: db.WorkflowNodeApprovalKind, ApprovalTimeout: new(1)}, {ID: 2, TemplateID: tpl.ID}}, Edges: []db.WorkflowEdge{{SourceNodeID: 1, DestinationNodeID: 2, Condition: db.WorkflowEdgeOnSuccess}}})
	require.NoError(t, err)
	run, err := service.StartWorkflow(graph, nil)
	require.NoError(t, err)
	_, err = store.Sql().Exec("update project__workflow_approval set created=? where workflow_run_id=?", tz.Now().Add(-time.Minute), run.ID)
	require.NoError(t, err)
	require.NoError(t, service.ProgressWorkflowRun(p.ID, run.ID, nil))
	run, err = store.GetWorkflowRunByID(p.ID, run.ID)
	require.NoError(t, err)
	require.Equal(t, db.WorkflowRunFailed, run.Status)
	require.Empty(t, workflowTasks(t, store, run))

	graph, err = store.CreateWorkflowTemplate(db.WorkflowTemplate{ProjectID: p.ID, Name: "artifacts", Nodes: []db.WorkflowNode{{ID: 1, TemplateID: tpl.ID}, {ID: 2, TemplateID: tpl.ID}, {ID: 3, TemplateID: tpl.ID}}, Edges: []db.WorkflowEdge{{SourceNodeID: 1, DestinationNodeID: 2, Condition: db.WorkflowEdgeOnSuccess}, {SourceNodeID: 1, DestinationNodeID: 3, Condition: db.WorkflowEdgeOnSuccess}}})
	require.NoError(t, err)
	run, err = service.StartWorkflow(graph, nil)
	require.NoError(t, err)
	root := workflowTasks(t, store, run)[0].Task
	require.NoError(t, store.UpdateTaskArtifacts(p.ID, root.ID, new(`{"shared":{"root":true}}`)))
	finishWorkflowTask(t, store, service, root, task_logger.TaskSuccessStatus)
	tasks := workflowTasks(t, store, run)
	require.Len(t, tasks, 3)
	sibling := tasks[0].Task
	current := tasks[1].Task
	require.NoError(t, store.UpdateTaskArtifacts(p.ID, sibling.ID, new(`{"sibling_secret":"not-an-ancestor"}`)))
	finishWorkflowTask(t, store, service, sibling, task_logger.TaskSuccessStatus)
	artifacts, err := service.GetWorkflowRunArtifacts(p.ID, run.ID, &current.ID)
	require.NoError(t, err)
	require.Contains(t, artifacts, "shared")
	require.NotContains(t, artifacts, "sibling_secret")
	_, err = service.GetWorkflowRunArtifacts(p.ID+1, run.ID, nil)
	require.ErrorIs(t, err, db.ErrNotFound)
}

func TestWorkflowApprovalCannotBeApprovedAfterDeadlineBeforeReconciler(t *testing.T) {
	store, p, _, _, service := workflowServiceFixture(t)
	graph, err := store.CreateWorkflowTemplate(db.WorkflowTemplate{ProjectID: p.ID, Name: "deadline", Nodes: []db.WorkflowNode{{ID: 1, Kind: db.WorkflowNodeApprovalKind, ApprovalTimeout: new(1)}}})
	require.NoError(t, err)
	run, err := service.StartWorkflow(graph, nil)
	require.NoError(t, err)
	_, err = store.Sql().Exec("update project__workflow_approval set created=? where workflow_run_id=?", time.Now().Add(-time.Minute), run.ID)
	require.NoError(t, err)
	_, err = service.ResolveWorkflowApproval(p.ID, graph.ID, run.ID, graph.Nodes[0].ID, db.WorkflowApprovalApproved, nil)
	require.ErrorIs(t, err, db.ErrInvalidOperation)
	approval, err := store.GetWorkflowApproval(p.ID, run.ID, graph.Nodes[0].ID)
	require.NoError(t, err)
	require.Equal(t, db.WorkflowApprovalRejected, approval.Status)
}

func TestWorkflowWaitsForLogBarrierBeforeStartingChild(t *testing.T) {
	store, p, tpl, _, service := workflowServiceFixture(t)
	graph, err := store.CreateWorkflowTemplate(db.WorkflowTemplate{ProjectID: p.ID, Name: "log barrier", Nodes: []db.WorkflowNode{{ID: 1, TemplateID: tpl.ID}, {ID: 2, TemplateID: tpl.ID}}, Edges: []db.WorkflowEdge{{SourceNodeID: 1, DestinationNodeID: 2, Condition: db.WorkflowEdgeOnSuccess}}})
	require.NoError(t, err)
	run, err := service.StartWorkflow(graph, nil)
	require.NoError(t, err)
	task := workflowTasks(t, store, run)[0].Task
	task.Status = task_logger.TaskSuccessStatus
	require.NoError(t, store.UpdateTask(task))
	require.NoError(t, service.ProgressWorkflowRun(p.ID, run.ID, nil))
	require.Len(t, workflowTasks(t, store, run), 1, "terminal status precedes final log/artifact persistence")
	finishWorkflowTask(t, store, service, task, task_logger.TaskSuccessStatus)
	require.Len(t, workflowTasks(t, store, run), 2)
}
