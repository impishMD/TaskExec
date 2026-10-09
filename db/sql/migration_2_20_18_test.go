package sql

import (
	"testing"

	"github.com/impishMD/taskexec/db"
	"github.com/stretchr/testify/require"
)

func TestMigration_2_20_18_PreservesUsersAndStorageReferences(t *testing.T) {
	version := "2.20.17"
	store := InitConfigCreateTestStoreAt(&version)
	t.Cleanup(store.Close)
	user, err := store.ImportUser(db.UserWithPwd{User: db.User{
		Username: "existing", Name: "Existing admin", Email: "existing@example.com",
		Password: "existing-password-hash", Admin: true, Alert: true,
	}})
	require.NoError(t, err)
	_, err = store.exec("update `user` set pro = ? where id = ?", true, user.ID)
	require.NoError(t, err)
	project, err := store.CreateProject(db.Project{Name: "Existing project"})
	require.NoError(t, err)
	storage, err := store.CreateSecretStorage(db.SecretStorage{
		ProjectID: project.ID, Name: "Secrets", Type: "openbao", ReadOnly: true,
		Params: db.MapStringAnyField{"url": "https://vault.example.com", "mount": "secret"},
	})
	require.NoError(t, err)
	key, err := store.CreateAccessKey(db.AccessKey{
		Name: "DNS", ProjectID: &project.ID, Type: db.AccessKeyString,
		SourceStorageID: &storage.ID, SourceStorageType: new(db.AccessKeySourceStorageVault),
		SourceStorageKey: new("proxmox"), SourceMapping: db.MapStringAnyField{"value": "dns_server"},
	})
	require.NoError(t, err)

	require.NoError(t, db.Migrate(store, nil))
	restoredUser, err := store.GetUser(user.ID)
	require.NoError(t, err)
	require.Equal(t, user, restoredUser)
	restoredStorage, err := store.GetSecretStorage(project.ID, storage.ID)
	require.NoError(t, err)
	storage.Type = db.SecretStorageTypeVault
	require.Equal(t, storage, restoredStorage)
	restoredKey, err := store.GetAccessKey(project.ID, key.ID)
	require.NoError(t, err)
	require.Equal(t, key.SourceStorageID, restoredKey.SourceStorageID)
	require.Equal(t, key.SourceStorageKey, restoredKey.SourceStorageKey)
	require.Equal(t, key.SourceMapping, restoredKey.SourceMapping)
	_, err = store.exec("select pro from `user`")
	require.Error(t, err)
	restoredUser.Name = "Updated admin"
	require.NoError(t, store.UpdateUser(db.UserWithPwd{User: restoredUser}))
	restoredUser, err = store.GetUser(user.ID)
	require.NoError(t, err)
	require.Equal(t, "Updated admin", restoredUser.Name)
}
