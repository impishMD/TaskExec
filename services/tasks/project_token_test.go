package tasks

import (
	"context"
	"testing"
	"time"

	"github.com/impishMD/taskexec/db"
	"github.com/impishMD/taskexec/pkg/task_logger"
	"github.com/impishMD/taskexec/services/audit"
	"github.com/impishMD/taskexec/services/audit/audittest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProjectTokenTaskAttributionAndQueueRevocation(t *testing.T) {
	f := newTaskRunnerRunFixture(t)
	t.Cleanup(f.store.Close)
	f.pool.register = make(chan *TaskRunner, 1)
	token := db.ProjectToken{ProjectID: f.template.ProjectID, Name: "CI", AllTemplates: true, Scopes: []string{db.TokenRunTasks}}
	_, err := token.IssueSecret()
	require.NoError(t, err)
	token, err = f.store.CreateProjectToken(token, "")
	require.NoError(t, err)
	rec := &audittest.Recorder{}
	f.pool.SetAuditRecorder(rec)
	ctx := audit.WithActor(context.Background(), audit.ProjectTokenActor(token.ID, token.Name))
	task, err := f.pool.AddTaskFrom(ctx, audit.TriggerAPI, db.Task{TemplateID: f.template.ID}, new(999), "CI", f.template.ProjectID, false)
	require.NoError(t, err)
	assert.Nil(t, task.UserID)
	assert.Equal(t, &token.ID, task.ProjectTokenID)
	assert.Equal(t, "CI", task.ProjectTokenName)
	stored, err := f.store.GetTask(f.template.ProjectID, task.ID)
	require.NoError(t, err)
	assert.Equal(t, &token.ID, stored.ProjectTokenID)
	event, err := rec.Only(audit.TaskExecutionCreate)
	require.NoError(t, err)
	assert.Equal(t, audit.ActorProjectToken, event.Actor.Type)
	runner := <-f.pool.register
	require.NoError(t, f.store.RevokeProjectToken(token.ProjectID, token.ID))
	runner.run()
	assert.Equal(t, task_logger.TaskStoppedStatus, runner.Task.Status)
	_, err = f.pool.AddTaskFrom(ctx, audit.TriggerAPI, db.Task{TemplateID: f.template.ID}, nil, "CI", f.template.ProjectID, false)
	require.Error(t, err)
}

func TestProjectTokenAutorunKeepsScopeAndExpiry(t *testing.T) {
	f := newTaskRunnerRunFixture(t)
	t.Cleanup(f.store.Close)
	token := db.ProjectToken{ProjectID: f.template.ProjectID, Name: "CI", TemplateIDs: []int{f.template.ID + 100}, Scopes: []string{db.TokenRunTasks}}
	_, err := token.IssueSecret()
	require.NoError(t, err)
	token, err = f.store.CreateProjectToken(token, "")
	require.NoError(t, err)
	runner := TaskRunner{Task: db.Task{ProjectTokenID: &token.ID, ProjectTokenName: "CI"}}
	_, err = f.pool.AddTaskFrom(runner.autorunContext(), audit.TriggerAutorun, db.Task{TemplateID: f.template.ID}, nil, "", f.template.ProjectID, false)
	require.Error(t, err)
	token.ExpiresAt = new(time.Now().Add(-time.Hour))
	_, err = token.IssueSecret()
	require.NoError(t, err)
	token, err = f.store.CreateProjectToken(token, "")
	require.NoError(t, err)
	runner.Task.ProjectTokenID = &token.ID
	_, err = f.pool.AddTaskFrom(runner.autorunContext(), audit.TriggerAutorun, db.Task{TemplateID: f.template.ID}, nil, "", f.template.ProjectID, false)
	require.Error(t, err)
}
