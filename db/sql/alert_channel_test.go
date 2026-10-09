package sql

import (
	"github.com/impishMD/taskexec/db"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestAlertChannelsMigrationAndProjectIsolation(t *testing.T) {
	version := "2.20.10"
	store := InitConfigCreateTestStoreAt(&version)
	defer store.Close()
	first, err := createLegacyTestProject(store, db.Project{Name: "first", Alert: true})
	require.NoError(t, err)
	second, err := createLegacyTestProject(store, db.Project{Name: "second"})
	require.NoError(t, err)
	require.NoError(t, db.Migrate(store, nil))
	require.NoError(t, db.Migrate(store, nil))
	existing, err := store.GetProject(first.ID)
	require.NoError(t, err)
	require.True(t, existing.Alert, "legacy non-Telegram alert settings remain unchanged")
	missing, err := store.GetAlertChannel(first.ID, "telegram")
	require.NoError(t, err)
	require.False(t, missing.Enabled)
	require.Equal(t, "{}", missing.Settings)
	for _, id := range []int{0, first.ID, second.ID} {
		require.NoError(t, store.SetAlertChannel(db.AlertChannel{ProjectID: id, Channel: "telegram", Enabled: id != 0, Settings: `{"chat_id":"123"}`, Secret: "ciphertext"}))
	}
	require.NoError(t, store.SetAlertChannel(db.AlertChannel{ProjectID: first.ID, Channel: "future-channel", Settings: "{}", Secret: "other"}))
	require.NoError(t, store.SetAlertChannel(db.AlertChannel{ProjectID: first.ID, Channel: "telegram", Settings: "{}"}))
	global, err := store.GetAlertChannel(0, "telegram")
	require.NoError(t, err)
	require.Equal(t, "ciphertext", global.Secret)
	other, err := store.GetAlertChannel(second.ID, "telegram")
	require.NoError(t, err)
	require.True(t, other.Enabled)
	require.NoError(t, store.DeleteProject(first.ID))
	channels, err := store.GetAlertChannelsWithSecrets()
	require.NoError(t, err)
	require.Len(t, channels, 2)
	require.Error(t, store.SetAlertChannel(db.AlertChannel{ProjectID: first.ID, Channel: "telegram", Settings: "{}"}), "deleted project cannot receive settings")
}
