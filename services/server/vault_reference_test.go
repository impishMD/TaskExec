package server

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/impishMD/taskexec/db"
	"github.com/impishMD/taskexec/util"
	"github.com/stretchr/testify/require"
)

func TestVaultReferenceOnlyNeverWrites(t *testing.T) {
	for _, readonly := range []bool{true, false} {
		t.Run(map[bool]string{true: "read-only", false: "writable"}[readonly], func(t *testing.T) {
			f := newVaultFixture(t, 2, db.SecretStorageTypeVault)
			f.storage.ReadOnly = readonly
			require.NoError(t, f.service.Update(f.storage))
			f.remote.put("proxmox", map[string]any{"dns_server": "192.0.2.53", "other": "keep"})
			key := f.reference("test", "proxmox", db.AccessKeyString, "dns_server")
			key.ReferenceOnly = true
			// No Vault requests should be necessary even if the provider is unavailable.
			f.remote.failure = "proxmox"
			key, err := f.keys.Create(key)
			require.NoError(t, err)
			key.Name = "renamed"
			key.Type = db.AccessKeySSH
			key.SourceMapping = db.MapStringAnyField{"private_key": "dns_server"}
			require.NoError(t, f.keys.Update(key))
			saved, err := f.store.GetAccessKey(f.project.ID, key.ID)
			require.NoError(t, err)
			require.Equal(t, db.AccessKeySSH, saved.Type)
			require.Equal(t, "renamed", saved.Name)
			require.Nil(t, saved.Secret)
			key.Type = db.AccessKeyString
			key.SourceMapping = db.MapStringAnyField{"value": "dns_server"}
			require.NoError(t, f.keys.Update(key))
			f.remote.failure = ""
			require.Equal(t, "192.0.2.53", f.load(t, key).String)
			require.Equal(t, 1, f.remote.versions["proxmox"])
			// Removing the UI reference must not delete a writable Vault value either.
			require.NoError(t, f.keys.Delete(f.project.ID, key.ID, true))
			require.Equal(t, "192.0.2.53", f.remote.docs["proxmox"]["dns_server"])
			require.Equal(t, "keep", f.remote.docs["proxmox"]["other"])
		})
	}
}

func TestKeySourceChangesPreserveRemoteValues(t *testing.T) {
	for _, readonly := range []bool{true, false} {
		for _, destination := range []string{"local", "env", "file"} {
			t.Run(map[bool]string{true: "read-only", false: "writable"}[readonly]+"/"+destination, func(t *testing.T) {
				f := newVaultFixture(t, 2, db.SecretStorageTypeVault)
				f.storage.ReadOnly = readonly
				require.NoError(t, f.service.Update(f.storage))
				f.remote.put("proxmox", map[string]any{"dns_server": "192.0.2.53", "other": "keep"})
				key := f.reference("test", "proxmox", db.AccessKeyString, "dns_server")
				key.ReferenceOnly = true
				key, err := f.keys.Create(key)
				require.NoError(t, err)

				// Change only TaskExec's source, even while Vault is unavailable.
				f.remote.failure = "proxmox"
				key.ReferenceOnly = false
				key.OverrideSecret = true
				key.IgnorePlain = true
				switch destination {
				case "local":
					key.SourceStorageType = nil
					key.String = "replacement value"
				case "env":
					t.Setenv("TASKEXEC_KEY_SOURCE_TEST", "replacement value")
					key.SourceStorageType = new(db.AccessKeySourceStorageEnv)
					key.SourceStorageKey = new("TASKEXEC_KEY_SOURCE_TEST")
				case "file":
					path := filepath.Join(util.Config.Dirs.Secrets, "test-secret")
					require.NoError(t, os.WriteFile(path, []byte("replacement value"), 0600))
					key.SourceStorageType = new(db.AccessKeySourceStorageFile)
					key.SourceStorageKey = &path
				}
				require.NoError(t, f.keys.Update(key))
				saved, err := f.store.GetAccessKey(f.project.ID, key.ID)
				require.NoError(t, err)
				require.Nil(t, saved.SourceStorageID, "old Vault connection must not leak into the new source")
				if destination == "local" {
					require.Nil(t, saved.SourceStorageKey)
					require.NotNil(t, saved.Secret)
				} else {
					require.Nil(t, saved.Secret)
				}
				require.NoError(t, f.encryption.DeserializeSecret(&saved))
				require.Equal(t, "replacement value", saved.String)

				// Switching back to a Vault reference never writes the local value there.
				back := f.reference("test", "proxmox", db.AccessKeyString, "dns_server")
				back.ID = key.ID
				back.ReferenceOnly = true
				require.NoError(t, f.keys.Update(back))
				f.remote.failure = ""
				require.Equal(t, "192.0.2.53", f.load(t, back).String)
				require.Equal(t, 1, f.remote.versions["proxmox"])
				require.Equal(t, "keep", f.remote.docs["proxmox"]["other"])
			})
		}
	}
}

func TestVaultReferenceOnlyValidatesSourcesAndRejectsPayloads(t *testing.T) {
	f := newVaultFixture(t, 2, db.SecretStorageTypeVault)
	key := f.reference("test", "proxmox", db.AccessKeyString, "dns_server")
	key.ReferenceOnly = true
	bad := key
	bad.String = "do not write"
	_, err := f.keys.Create(bad)
	require.ErrorContains(t, err, "reference-only")
	bad = key
	bad.GenerateSSHKey = true
	_, err = f.keys.Create(bad)
	require.ErrorContains(t, err, "reference-only")
	bad = key
	bad.OverrideSecret = true
	_, err = f.keys.Create(bad)
	require.ErrorContains(t, err, "reference-only")
	bad = key
	bad.SourceStorageKey = new("../private")
	_, err = f.keys.Create(bad)
	require.ErrorContains(t, err, "invalid Vault")
	other, err := f.store.CreateProject(db.Project{Name: "other"})
	require.NoError(t, err)
	bad = key
	bad.ProjectID = &other.ID
	_, err = f.keys.Create(bad)
	require.ErrorIs(t, err, db.ErrNotFound)
	key, err = f.keys.Create(key)
	require.NoError(t, err)
	bad = key
	bad.LoginPassword.Password = "unexpected"
	require.ErrorContains(t, f.keys.Update(bad), "reference-only")
	saved, err := f.store.GetAccessKey(f.project.ID, key.ID)
	require.NoError(t, err)
	require.Equal(t, db.AccessKeyString, saved.Type)
}

// References made by older versions remain usable after discovery is retired.
func TestVaultFormerlySynchronizedReferenceCanBeEdited(t *testing.T) {
	f := newVaultFixture(t, 2, db.SecretStorageTypeVault)
	f.remote.put("proxmox", map[string]any{"dns_server": "192.0.2.53"})
	key := f.reference("dns", "proxmox", db.AccessKeyString)
	key.Synchronized = true
	key.SourceMapping = db.MapStringAnyField{"value": "dns_server"}
	key, err := f.store.CreateAccessKey(key)
	require.NoError(t, err)
	key.ReferenceOnly = true
	key.Name = "custom"
	key.Type = db.AccessKeyObject
	key.SourceMapping = nil
	require.NoError(t, f.keys.Update(key))
	saved, err := f.store.GetAccessKey(f.project.ID, key.ID)
	require.NoError(t, err)
	require.NoError(t, f.encryption.DeserializeSecret(&saved))
	require.Equal(t, "192.0.2.53", saved.Object["dns_server"])
	require.Equal(t, 1, f.remote.versions["proxmox"])
}
