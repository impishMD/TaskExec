package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/impishMD/taskexec/db"
	"github.com/impishMD/taskexec/db/sql"
	"github.com/impishMD/taskexec/services/alerting"
	"github.com/impishMD/taskexec/services/server"
	"github.com/impishMD/taskexec/util"
	"github.com/stretchr/testify/require"
)

func TestRekeyBackupAndRollbackIncludeAlertCredentials(t *testing.T) {
	oldConfig := util.Config
	store := sql.InitConfigCreateTestStore()
	defer func() { store.Close(); util.Config = oldConfig }()
	project, err := store.CreateProject(db.Project{Name: "alerts"})
	require.NoError(t, err)
	service := alerting.Service{Store: store}
	token := "123:old-token"
	for _, id := range []int{0, project.ID} {
		_, err = service.UpdateTelegram(id, alerting.TelegramUpdate{Token: &token})
		require.NoError(t, err)
	}
	path := filepath.Join(t.TempDir(), "backup.jsonl")
	require.NoError(t, backupAccessKeys(store, path))
	backup, err := os.ReadFile(path)
	require.NoError(t, err)
	require.NotContains(t, string(backup), token)
	require.Contains(t, string(backup), `"channel":"telegram"`)
	token = "456:new-token"
	for _, id := range []int{0, project.ID} {
		_, err = service.UpdateTelegram(id, alerting.TelegramUpdate{Token: &token})
		require.NoError(t, err)
	}
	encryption := server.NewAccessKeyEncryptionService(store, store, store, store)
	require.NoError(t, rollbackAccessKeys(store, encryption, path))
	for _, id := range []int{0, project.ID} {
		channel, err := store.GetAlertChannel(id, alerting.Telegram)
		require.NoError(t, err)
		secret, err := util.Config.DecryptAccessSecret(channel.Secret)
		require.NoError(t, err)
		require.Equal(t, "123:old-token", string(secret))
	}
}
