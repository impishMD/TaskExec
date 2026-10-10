package sql

import (
	"testing"
	"time"

	"github.com/impishMD/taskexec/db"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func ansibleTaskFixture(t *testing.T, store *SqlDb) db.Task {
	t.Helper()
	project, err := createLegacyTestProject(store, db.Project{Name: "summary"})
	require.NoError(t, err)
	keyID, err := store.insert("id", "insert into access_key (project_id, name, type) values (?, ?, ?)", project.ID, "fixture", db.AccessKeyNone)
	key := db.AccessKey{ID: keyID}
	require.NoError(t, err)
	repo, err := store.CreateRepository(db.Repository{ProjectID: project.ID, SSHKeyID: key.ID, Name: "summary", GitURL: "/tmp/ansible-summary", GitBranch: "main"})
	require.NoError(t, err)
	inv, err := store.CreateInventory(db.Inventory{ProjectID: project.ID, Type: db.InventoryStatic, Inventory: "localhost ansible_connection=local"})
	require.NoError(t, err)
	// This fixture also seeds an older schema for migration tests.
	templateID, err := store.insert("id", "insert into project__template (project_id, repository_id, app, inventory_id, name, playbook) values (?, ?, ?, ?, ?, ?)", project.ID, repo.ID, db.AppAnsible, inv.ID, "summary", "test.yml")
	require.NoError(t, err)
	task := db.Task{ProjectID: project.ID, TemplateID: templateID, Created: time.Now(), Status: "waiting"}
	task.ID, err = store.insert("id", "insert into task (project_id, template_id, created, status, playbook, environment) values (?, ?, ?, ?, '', '')", task.ProjectID, task.TemplateID, task.Created, task.Status)
	require.NoError(t, err)
	return task
}

func TestAnsibleTaskRepository_ScopeReplayAndCascade(t *testing.T) {
	store := InitConfigCreateTestStore()
	defer store.Close()
	task := ansibleTaskFixture(t, store)
	other := ansibleTaskFixture(t, store)
	created := time.Now().UTC().Truncate(time.Second)
	host := db.AnsibleTaskHost{ProjectID: task.ProjectID, TaskID: task.ID, Host: "host", Ok: 1, Created: &created}
	item := db.AnsibleTaskError{ProjectID: task.ProjectID, TaskID: task.ID, Host: "host", Task: "Install", Error: "failure", Created: &created}
	for i := 0; i < 2; i++ {
		require.NoError(t, store.CreateAnsibleTaskHost(host))
		require.NoError(t, store.CreateAnsibleTaskError(item))
	}
	host.Ok = 2
	require.NoError(t, store.CreateAnsibleTaskHost(host))
	hosts, err := store.GetAnsibleTaskHosts(task.ProjectID, task.ID)
	require.NoError(t, err)
	require.Len(t, hosts, 1)
	assert.Equal(t, 2, hosts[0].Ok)
	items, err := store.GetAnsibleTaskErrors(task.ProjectID, task.ID)
	require.NoError(t, err)
	require.Len(t, items, 1)
	assert.True(t, items[0].Created.Equal(created))
	for _, projectID := range []int{other.ProjectID, task.ProjectID + 999} {
		_, err = store.GetAnsibleTaskHosts(projectID, task.ID)
		assert.ErrorIs(t, err, db.ErrNotFound)
		_, err = store.GetAnsibleTaskErrors(projectID, task.ID)
		assert.ErrorIs(t, err, db.ErrNotFound)
		host.ProjectID, item.ProjectID = projectID, projectID
		assert.ErrorIs(t, store.CreateAnsibleTaskHost(host), db.ErrNotFound)
		assert.ErrorIs(t, store.CreateAnsibleTaskError(item), db.ErrNotFound)
	}
	require.NoError(t, store.DeleteTaskWithOutputs(task.ProjectID, task.ID))
	for _, table := range []string{"task__ansible_host", "task__ansible_error"} {
		count, err := store.Sql().SelectInt("select count(*) from " + table)
		require.NoError(t, err)
		assert.Zero(t, count)
	}
	hosts, err = store.GetAnsibleTaskHosts(other.ProjectID, other.ID)
	require.NoError(t, err)
	assert.Equal(t, []db.AnsibleTaskHost{}, hosts)
	items, err = store.GetAnsibleTaskErrors(other.ProjectID, other.ID)
	require.NoError(t, err)
	assert.Equal(t, []db.AnsibleTaskError{}, items)
}

func TestAnsibleTaskMigration_PreservesHistoricalRows(t *testing.T) {
	version := "2.20.11"
	store := InitConfigCreateTestStoreAt(&version)
	defer store.Close()
	task := ansibleTaskFixture(t, store)
	_, err := store.exec("insert into task__ansible_host (project_id, task_id, host, changed, failed, ignored, ok, rescued, skipped, unreachable) values (?, ?, 'old-host', 1, 0, 0, 1, 0, 0, 0)", task.ProjectID, task.ID)
	require.NoError(t, err)
	_, err = store.exec("insert into task__ansible_error (project_id, task_id, task, error) values (?, ?, 'Old task', 'Old error')", task.ProjectID, task.ID)
	require.NoError(t, err)
	require.NoError(t, db.Migrate(store, nil))
	hosts, err := store.GetAnsibleTaskHosts(task.ProjectID, task.ID)
	require.NoError(t, err)
	require.Len(t, hosts, 1)
	assert.Equal(t, "old-host", hosts[0].Host)
	assert.Nil(t, hosts[0].Created)
	items, err := store.GetAnsibleTaskErrors(task.ProjectID, task.ID)
	require.NoError(t, err)
	require.Len(t, items, 1)
	assert.Equal(t, "Old error", items[0].Error)
	assert.Empty(t, items[0].Host)
	assert.Nil(t, items[0].Created)
	hosts[0].Ok = 5
	require.NoError(t, store.CreateAnsibleTaskHost(hosts[0]))
}
