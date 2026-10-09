package sql

import (
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/impishMD/taskexec/db"
	"github.com/impishMD/taskexec/pkg/task_logger"
	"github.com/impishMD/taskexec/util"
	"github.com/stretchr/testify/require"
)

func workflowFixture(t *testing.T) (*SqlDb, db.WorkflowTemplate) {
	t.Helper()
	previous := util.Config
	store := InitConfigCreateTestStore()
	if dialect := os.Getenv("TASKEXEC_TEST_WORKFLOW_DIALECT"); dialect != "" {
		require.Contains(t, []string{util.DbDriverPostgres, util.DbDriverMySQL}, dialect)
		store.Close()
		config := &util.DbConfig{Hostname: os.Getenv("TASKEXEC_TEST_WORKFLOW_HOST"), Username: "taskexec", Password: os.Getenv("TASKEXEC_TEST_WORKFLOW_PASSWORD"), DbName: "taskexec_workflow_qa", Options: map[string]string{}}
		util.Config.Dialect = dialect
		if dialect == util.DbDriverPostgres {
			config.Options["sslmode"] = "disable"
			util.Config.Postgres = config
		} else {
			util.Config.MySQL = config
		}
		store = CreateDb(dialect)
		store.Connect()
		require.NoError(t, db.Migrate(store, nil))
	}
	t.Cleanup(func() { store.Close(); util.Config = previous })
	util.Config.Apps = map[string]util.App{"bash": {}}
	p, err := store.CreateProject(db.Project{Name: "workflow"})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, store.DeleteProject(p.ID)) })
	key, err := store.CreateAccessKey(db.AccessKey{ProjectID: &p.ID, Name: "none", Type: db.AccessKeyNone})
	require.NoError(t, err)
	repo, err := store.CreateRepository(db.Repository{ProjectID: p.ID, Name: "repo", GitURL: "https://example.test/repo", GitBranch: "main", SSHKeyID: key.ID})
	require.NoError(t, err)
	tpl, err := store.CreateTemplate(db.Template{ProjectID: p.ID, Name: "script", App: db.AppBash, RepositoryID: repo.ID, Playbook: "test.sh"})
	require.NoError(t, err)
	delay := 1
	wf, err := store.CreateWorkflowTemplate(db.WorkflowTemplate{ProjectID: p.ID, Name: "pipeline", StartVersion: new("v0.0.1"), Nodes: []db.WorkflowNode{
		{ID: -1, TemplateID: tpl.ID, TaskParams: &db.TaskParams{Environment: `{"KEY":"original"}`}},
		{ID: -2, Kind: db.WorkflowNodeDelayKind, DelaySeconds: &delay},
	}, Edges: []db.WorkflowEdge{{SourceNodeID: -1, DestinationNodeID: -2, Condition: db.WorkflowEdgeOnSuccess}}})
	require.NoError(t, err)
	return store, wf
}
func TestWorkflowStoreRevisionIsolationAndDeletion(t *testing.T) {
	store, graph := workflowFixture(t)
	require.Equal(t, 1, graph.Revision)
	run, err := store.CreateWorkflowRun(db.WorkflowRun{ProjectID: graph.ProjectID, WorkflowTemplateID: graph.ID, RevisionID: graph.RevisionID})
	require.NoError(t, err)
	require.Equal(t, "v0.0.1", *run.Version)
	changed := graph
	changed.Nodes = append([]db.WorkflowNode{}, graph.Nodes...)
	changed.Nodes[0].TaskParams = &db.TaskParams{Environment: `{"KEY":"updated"}`}
	require.NoError(t, store.UpdateWorkflowTemplate(changed))
	current, err := store.GetWorkflowTemplate(graph.ProjectID, graph.ID)
	require.NoError(t, err)
	require.Equal(t, 2, current.Revision)
	require.NotEqual(t, graph.Nodes[0].ID, current.Nodes[0].ID)
	old, err := store.GetWorkflowRevisionGraph(graph.ProjectID, run.RevisionID)
	require.NoError(t, err)
	require.Equal(t, graph.Nodes, old.Nodes)
	require.Equal(t, graph.Edges, old.Edges)
	revisions, err := store.GetWorkflowRevisions(graph.ProjectID, graph.ID)
	require.NoError(t, err)
	require.Len(t, revisions, 2)
	require.True(t, revisions[1].HasRuns)
	_, err = store.GetWorkflowRevisionGraph(graph.ProjectID+1, graph.RevisionID)
	require.ErrorIs(t, err, db.ErrNotFound)
	require.ErrorIs(t, store.DeleteWorkflowTemplate(graph.ProjectID, graph.ID), db.ErrInvalidOperation)
	task, err := store.CreateTask(db.Task{ProjectID: graph.ProjectID, TemplateID: graph.Nodes[0].TemplateID, WorkflowRunID: &run.ID, WorkflowNodeID: &graph.Nodes[0].ID, Status: task_logger.TaskSuccessStatus, Created: time.Now()}, 0)
	require.NoError(t, err)
	require.ErrorIs(t, store.DeleteTaskWithOutputs(graph.ProjectID, task.ID), db.ErrInvalidOperation)
	run.Status = db.WorkflowRunSuccess
	require.NoError(t, store.UpdateWorkflowRun(run))
	next, err := store.CreateWorkflowRun(db.WorkflowRun{ProjectID: graph.ProjectID, WorkflowTemplateID: graph.ID, RevisionID: current.RevisionID})
	require.NoError(t, err)
	require.Equal(t, "v0.0.2", *next.Version)
	next.Status = db.WorkflowRunStopped
	require.NoError(t, store.UpdateWorkflowRun(next))
	require.NoError(t, store.DeleteWorkflowTemplate(graph.ProjectID, graph.ID))
	retained, err := store.GetTask(graph.ProjectID, task.ID)
	require.NoError(t, err)
	require.Nil(t, retained.WorkflowRunID)
	require.Nil(t, retained.WorkflowNodeID)
	var paramsCount int
	require.NoError(t, store.selectOne(&paramsCount, "select count(*) from project__task_params where project_id=?", graph.ProjectID))
	require.Zero(t, paramsCount, "deleting a graph must remove its unreferenced revision parameters")
}
func TestWorkflowStoreConcurrentSavesAndValidation(t *testing.T) {
	store, graph := workflowFixture(t)
	var wg sync.WaitGroup
	results := make(chan error, 8)
	for i := 0; i < 8; i++ {
		wg.Go(func() {
			copy := graph
			copy.Name = fmt.Sprintf("pipeline-%d", i)
			results <- store.UpdateWorkflowTemplate(copy)
		})
	}
	wg.Wait()
	close(results)
	for err := range results {
		require.NoError(t, err)
	}
	revisions, err := store.GetWorkflowRevisions(graph.ProjectID, graph.ID)
	require.NoError(t, err)
	require.Len(t, revisions, 9)
	require.Equal(t, 9, revisions[0].Number)
	foreign := graph
	foreign.ProjectID++
	_, err = store.CreateWorkflowTemplate(foreign)
	require.Error(t, err)
	cycle := graph
	cycle.Edges = append(append([]db.WorkflowEdge{}, graph.Edges...), db.WorkflowEdge{SourceNodeID: graph.Nodes[1].ID, DestinationNodeID: graph.Nodes[0].ID, Condition: db.WorkflowEdgeAlways})
	_, err = store.CreateWorkflowTemplate(cycle)
	require.Error(t, err)
	duplicate := graph
	duplicate.Nodes = []db.WorkflowNode{graph.Nodes[0], graph.Nodes[0]}
	_, err = store.CreateWorkflowTemplate(duplicate)
	require.Error(t, err)
}

func TestWorkflowStoreRetentionAndTemplateReferences(t *testing.T) {
	store, graph := workflowFixture(t)
	tplID := graph.Nodes[0].TemplateID
	refs, err := store.GetTemplateRefs(graph.ProjectID, tplID)
	require.NoError(t, err)
	require.Equal(t, []db.ObjectReferrer{{ID: graph.ID, Name: graph.Name}}, refs.Workflows)
	require.ErrorIs(t, store.DeleteTemplate(graph.ProjectID, tplID), db.ErrInvalidOperation)
	run, err := store.CreateWorkflowRun(db.WorkflowRun{ProjectID: graph.ProjectID, WorkflowTemplateID: graph.ID, RevisionID: graph.RevisionID})
	require.NoError(t, err)
	task, err := store.CreateTask(db.Task{ProjectID: graph.ProjectID, TemplateID: tplID, WorkflowRunID: &run.ID, WorkflowNodeID: &graph.Nodes[0].ID, Status: task_logger.TaskSuccessStatus, Created: time.Now().Add(-time.Hour)}, 0)
	require.NoError(t, err)
	for i := 0; i < 5; i++ {
		_, err = store.CreateTask(db.Task{ProjectID: graph.ProjectID, TemplateID: tplID, Status: task_logger.TaskSuccessStatus, Created: time.Now().Add(time.Duration(i) * time.Second)}, 2)
		require.NoError(t, err)
	}
	_, err = store.GetTask(graph.ProjectID, task.ID)
	require.NoError(t, err)
	tpl, err := store.GetTemplate(graph.ProjectID, tplID)
	require.NoError(t, err)
	var count int
	require.NoError(t, store.selectOne(&count, "select count(*) from task where template_id=?", tplID))
	require.Equal(t, count, tpl.Tasks)
	require.Equal(t, 3, count)
}
