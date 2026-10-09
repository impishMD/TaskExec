package tasks

import (
	"errors"
	"testing"

	"github.com/impishMD/taskexec/db"
	"github.com/impishMD/taskexec/pkg/task_logger"
	"github.com/impishMD/taskexec/services/server"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTaskRouting_UsesSavedDefaultForNewTasks(t *testing.T) {
	f := newTaskRunnerRunFixture(t)
	t.Cleanup(f.store.Close)
	f.pool.register = make(chan *TaskRunner, 1)
	var queued []*TaskRunner
	for _, remote := range []bool{false, true, false} {
		require.NoError(t, server.SaveServerSettings(f.store, server.ServerSettings{UseRemoteRunner: remote}))
		_, err := f.pool.AddTask(db.Task{TemplateID: f.template.ID}, nil, "", f.template.ProjectID, false)
		require.NoError(t, err)
		tr := <-f.pool.register
		queued = append(queued, tr)
		if remote {
			assert.IsType(t, &RemoteJob{}, tr.job)
		} else {
			assert.IsType(t, &LocalExecutor{}, tr.job)
		}
	}
	assert.IsType(t, &LocalExecutor{}, queued[0].job)
	assert.IsType(t, &RemoteJob{}, queued[1].job, "changing the default leaves queued handlers unchanged")
}

func TestTaskRouting_AssignedRunnerSurvivesDisablingDefault(t *testing.T) {
	f := newTaskRunnerRunFixture(t)
	t.Cleanup(f.store.Close)
	runner, err := f.store.CreateRunner(db.Runner{Name: "assigned", Active: true})
	require.NoError(t, err)
	f.task.RunnerID = &runner.ID
	f.task.Status = task_logger.TaskRunningStatus
	require.NoError(t, f.store.UpdateTask(f.task))
	require.NoError(t, server.SaveServerSettings(f.store, server.ServerSettings{UseRemoteRunner: false}))
	tr, err := f.pool.HydrateTaskRunnerFromDB(f.task.ID)
	require.NoError(t, err)
	assert.IsType(t, &RemoteJob{}, tr.job)
	assert.Equal(t, &runner.ID, tr.Task.RunnerID)
}

type unavailableSettingsStore struct{ db.Store }

func (unavailableSettingsStore) GetOption(string) (string, error) {
	return "", errors.New("database unavailable")
}

func TestTaskRouting_ReadFailureDoesNotRunLocally(t *testing.T) {
	f := newTaskRunnerRunFixture(t)
	t.Cleanup(f.store.Close)
	f.pool.register = make(chan *TaskRunner, 1)
	for _, store := range []db.Store{unavailableSettingsStore{f.store}, f.store} {
		// Test both an unavailable database and a malformed persisted option.
		require.NoError(t, f.store.SetOption(server.RemoteRunnersEnabledOption, "not-a-boolean"))
		f.pool.store = store
		task, err := f.pool.AddTask(db.Task{TemplateID: f.template.ID}, nil, "", f.template.ProjectID, false)
		require.Error(t, err)
		assert.Empty(t, f.pool.register)
		persisted, err := f.store.GetTaskByID(task.ID)
		require.NoError(t, err)
		assert.Equal(t, task_logger.TaskFailStatus, persisted.Status)
	}
}

func TestTaskRouting_ExplicitTagsOverrideDefault(t *testing.T) {
	f := newTaskRunnerRunFixture(t)
	t.Cleanup(f.store.Close)
	f.pool.register = make(chan *TaskRunner, 1)
	tag := "docker"
	f.template.RunnerTag = &tag
	require.NoError(t, f.store.UpdateTemplate(f.template))
	_, err := f.pool.AddTask(db.Task{TemplateID: f.template.ID}, nil, "", f.template.ProjectID, false)
	require.NoError(t, err)
	tr := <-f.pool.register
	require.IsType(t, &RemoteJob{}, tr.job)
	assert.Equal(t, &tag, tr.job.(*RemoteJob).RunnerTag)

	// Inventory tags have the same explicit routing semantics even if the
	// default cannot be read. No fallback to local execution is allowed.
	f.template.RunnerTag = nil
	require.NoError(t, f.store.UpdateTemplate(f.template))
	inv, err := f.store.GetInventory(f.template.ProjectID, *f.template.InventoryID)
	require.NoError(t, err)
	inv.RunnerTag = &tag
	require.NoError(t, f.store.UpdateInventory(inv))
	f.pool.inventoryService = server.NewInventoryService(f.store, f.store, f.store, &EncryptionServiceMock{})
	f.pool.store = unavailableSettingsStore{f.store}
	_, err = f.pool.AddTask(db.Task{TemplateID: f.template.ID}, nil, "", f.template.ProjectID, false)
	require.NoError(t, err)
	tr = <-f.pool.register
	require.IsType(t, &RemoteJob{}, tr.job)
	assert.Equal(t, &tag, tr.job.(*RemoteJob).RunnerTag)
}
