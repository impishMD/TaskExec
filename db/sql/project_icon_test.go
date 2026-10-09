package sql

import (
	"testing"
	"time"

	"github.com/impishMD/taskexec/db"
	"github.com/stretchr/testify/require"
)

// Historical migration fixtures must seed columns that existed at that version.
func createLegacyTestProject(store *SqlDb, project db.Project) (db.Project, error) {
	project.Created = time.Now()
	id, err := store.insert("id",
		"insert into project(name, created, type, alert, alert_chat, max_parallel_tasks) values (?, ?, ?, ?, ?, ?)",
		project.Name, project.Created, project.Type, project.Alert, project.AlertChat, project.MaxParallelTasks)
	project.ID = id
	return project, err
}

func TestProjectIconMigration(t *testing.T) {
	version := "2.20.16"
	store := InitConfigCreateTestStoreAt(&version)
	t.Cleanup(store.Close)
	project, err := createLegacyTestProject(store, db.Project{Name: "existing", Alert: true})
	require.NoError(t, err)
	require.NoError(t, db.Migrate(store, nil))
	restored, err := store.GetProject(project.ID)
	require.NoError(t, err)
	require.Equal(t, "existing", restored.Name)
	require.True(t, restored.Alert)
	require.Nil(t, restored.Icon)
	restored.Icon = new("mdi-server")
	require.NoError(t, store.UpdateProject(restored))
	restored, err = store.GetProject(project.ID)
	require.NoError(t, err)
	require.Equal(t, "mdi-server", *restored.Icon)
}
