package sql

import (
	"testing"
	"time"

	"github.com/impishMD/taskexec/db"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProjectTokenMigrationAndRotationRollback(t *testing.T) {
	version := "2.20.18"
	store := InitConfigCreateTestStoreAt(&version)
	t.Cleanup(store.Close)
	task := ansibleTaskFixture(t, store)
	require.NoError(t, db.Migrate(store, nil))
	stored, err := store.GetTask(task.ProjectID, task.ID)
	require.NoError(t, err)
	assert.Nil(t, stored.ProjectTokenID)
	assert.Empty(t, stored.ProjectTokenName)

	// Creator snapshots intentionally do not require an existing user record.
	token := db.ProjectToken{ProjectID: task.ProjectID, Name: "CI", CreatorID: 9999,
		CreatorName: "former-user", Scopes: []string{db.TokenRunTasks}, TemplateIDs: []int{task.TemplateID}, Overrides: []string{"git_branch"}}
	secret, err := token.IssueSecret()
	require.NoError(t, err)
	token, err = store.CreateProjectToken(token, "")
	require.NoError(t, err)

	// A failed replacement insert must roll back revocation of the old secret.
	_, err = store.CreateProjectToken(token, token.ID)
	require.Error(t, err)
	unchanged, err := store.GetProjectToken(task.ProjectID, token.ID)
	require.NoError(t, err)
	assert.True(t, unchanged.IsActive(time.Now()))
	assert.True(t, unchanged.MatchesSecret(secret))
	assert.Equal(t, token.Scopes, unchanged.Scopes)
	assert.Equal(t, token.TemplateIDs, unchanged.TemplateIDs)
	listed, err := store.GetProjectTokens(task.ProjectID)
	require.NoError(t, err)
	require.Len(t, listed, 1)
	assert.Equal(t, token.Scopes, listed[0].Scopes)
	assert.Equal(t, token.TemplateIDs, listed[0].TemplateIDs)
	assert.Equal(t, token.Overrides, listed[0].Overrides)

	_, err = store.exec("delete from project where id=?", task.ProjectID)
	require.NoError(t, err)
	_, err = store.GetProjectTokenByID(token.ID)
	assert.ErrorIs(t, err, db.ErrNotFound)
}
