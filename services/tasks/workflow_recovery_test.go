package tasks

import (
	"context"
	"github.com/impishMD/taskexec/db"
	"github.com/impishMD/taskexec/pkg/task_logger"
	"github.com/impishMD/taskexec/services/audit"
	"github.com/impishMD/taskexec/services/server"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

func TestWorkflowRecoveryDoesNotReplayInterruptedLocalTasks(t *testing.T) {
	f := newTaskRunnerRunFixture(t)
	t.Cleanup(f.store.Close)
	f.pool.workflowRepo = f.store
	f.pool.workflowService = server.NewWorkflowService(f.store, f.store, &f.pool, nil)
	graph, err := f.store.CreateWorkflowTemplate(db.WorkflowTemplate{ProjectID: f.template.ProjectID, Name: "recovery", Nodes: []db.WorkflowNode{{ID: 1, TemplateID: f.template.ID}}})
	require.NoError(t, err)
	runner, err := f.store.CreateRunner(db.Runner{Name: "remote", Active: true})
	require.NoError(t, err)
	for _, remote := range []bool{false, true} {
		run, err := f.store.CreateWorkflowRun(db.WorkflowRun{ProjectID: graph.ProjectID, WorkflowTemplateID: graph.ID, RevisionID: graph.RevisionID})
		require.NoError(t, err)
		task := db.Task{ProjectID: graph.ProjectID, TemplateID: f.template.ID, WorkflowRunID: &run.ID, WorkflowNodeID: &graph.Nodes[0].ID, Status: task_logger.TaskRunningStatus}
		if remote {
			task.RunnerID = &runner.ID
		}
		task, err = f.store.CreateTask(task, 0)
		require.NoError(t, err)
		require.NoError(t, f.pool.RecoverWorkflowTasks())
		persisted, err := f.store.GetTaskByID(task.ID)
		require.NoError(t, err)
		run, err = f.store.GetWorkflowRunByID(graph.ProjectID, run.ID)
		require.NoError(t, err)
		if remote {
			require.Equal(t, db.WorkflowRunRunning, run.Status)
			require.Equal(t, task_logger.TaskRunningStatus, persisted.Status)
		} else {
			require.Equal(t, db.WorkflowRunStopped, run.Status)
			require.Equal(t, task_logger.TaskStoppedStatus, persisted.Status)
			require.NotNil(t, persisted.End)
		}
		records, err := f.store.GetWorkflowRunTasks(graph.ProjectID, run.ID, db.RetrieveQueryParams{})
		require.NoError(t, err)
		require.Len(t, records, 1)
	}
}
func TestPublicTaskCannotForgeWorkflowAssociation(t *testing.T) {
	f := newTaskRunnerRunFixture(t)
	t.Cleanup(f.store.Close)
	f.pool.register = make(chan *TaskRunner, 1)
	task, err := f.pool.AddTaskFrom(context.Background(), audit.TriggerAPI, db.Task{TemplateID: f.template.ID, WorkflowRunID: new(123), WorkflowNodeID: new(456)}, nil, "", f.template.ProjectID, false)
	require.NoError(t, err)
	require.Nil(t, task.WorkflowRunID)
	require.Nil(t, task.WorkflowNodeID)
	queued := <-f.pool.register
	require.Nil(t, queued.Task.WorkflowRunID)
}

func TestWorkflowArtifactsPersistThroughTaskLogsAndFeedOnlyDownstream(t *testing.T) {
	f := newTaskRunnerRunFixture(t)
	t.Cleanup(f.store.Close)
	f.pool.workflowRepo = f.store
	f.pool.workflowService = server.NewWorkflowService(f.store, f.store, &f.pool, nil)
	graph, err := f.store.CreateWorkflowTemplate(db.WorkflowTemplate{ProjectID: f.template.ProjectID, Name: "artifacts", Nodes: []db.WorkflowNode{{ID: 1, TemplateID: f.template.ID}, {ID: 2, TemplateID: f.template.ID}}, Edges: []db.WorkflowEdge{{SourceNodeID: 1, DestinationNodeID: 2, Condition: db.WorkflowEdgeOnSuccess}}})
	require.NoError(t, err)
	run, err := f.store.CreateWorkflowRun(db.WorkflowRun{ProjectID: graph.ProjectID, WorkflowTemplateID: graph.ID, RevisionID: graph.RevisionID})
	require.NoError(t, err)
	root, err := f.store.CreateTask(db.Task{ProjectID: graph.ProjectID, TemplateID: f.template.ID, WorkflowRunID: &run.ID, WorkflowNodeID: &graph.Nodes[0].ID, Status: task_logger.TaskSuccessStatus, End: new(time.Now())}, 0)
	require.NoError(t, err)
	logger := &TaskRunner{Task: root, Template: db.Template{App: db.AppAnsible}}
	f.pool.writeLogs([]logRecord{{task: logger, output: db.WorkflowArtifactsMarker + `{"build":42,"override":"upstream"}`}})
	saved, err := f.store.GetTaskByID(root.ID)
	require.NoError(t, err)
	require.NotNil(t, saved.Artifacts)
	f.pool.writeLogs([]logRecord{{task: logger, output: db.WorkflowArtifactsMarker + `["invalid"]`}})
	child, err := f.store.CreateTask(db.Task{ProjectID: graph.ProjectID, TemplateID: f.template.ID, WorkflowRunID: &run.ID, WorkflowNodeID: &graph.Nodes[1].ID, Environment: `{"override":"explicit"}`}, 0)
	require.NoError(t, err)
	tr := &TaskRunner{Task: child, pool: &f.pool, Environment: db.Environment{JSON: `{"group":"value","build":0}`}}
	require.NoError(t, tr.populateTaskEnvironment())
	require.JSONEq(t, `{"group":"value","build":42,"override":"explicit"}`, tr.Environment.JSON)
}
