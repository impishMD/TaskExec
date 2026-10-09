package api

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/impishMD/taskexec/db"
	"github.com/impishMD/taskexec/services/server"
	"github.com/impishMD/taskexec/util"
	"github.com/stretchr/testify/require"
)

func TestVaultStorageAPI_PermissionsIsolationAndTokenRedaction(t *testing.T) {
	f := newCoreFixture(t)
	util.Config.AccessKeyEncryption = "hHYgPrhQTZYm7UFTvcdNfKJMB3wtAXtJENUButH+DmM="
	enc := server.NewAccessKeyEncryptionService(f.store, f.store, f.store, f.store)
	keys := server.NewAccessKeyService(f.store, enc, f.store, f.store)
	service := server.NewSecretStorageService(f.store, f.store, keys, enc)
	f.router = Route(f.store, nil, nil, f.store, nil, nil, nil, enc, nil, service, keys, nil, nil, server.NewRunnerService(f.store), nil, nil)
	base := fmt.Sprintf("/project/%d/secret_storages", f.project.ID)
	body := fmt.Sprintf(`{"name":"Test Vault","project_id":%d,"type":"vault","readonly":true,"secret":"fixture-api-token","params":{"url":"http://127.0.0.1:8200","kv_version":2}}`, f.project.ID)
	f.request(t, "guest", "POST", base, body, 403)
	f.request(t, "outsider", "GET", base, "", 404)
	w := f.request(t, "manager", "POST", base, body, 201)
	require.NotContains(t, w.Body.String(), "fixture-api-token")
	var storage db.SecretStorage
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &storage))
	path := fmt.Sprintf("%s/%d", base, storage.ID)
	for _, endpoint := range []string{base, path} {
		w = f.request(t, "guest", "GET", endpoint, "", 200)
		require.NotContains(t, w.Body.String(), "fixture-api-token")
		require.Contains(t, w.Body.String(), "Test Vault")
	}
	f.request(t, "guest", "PUT", path, body, 403)
	f.request(t, "guest", "DELETE", path, "", 403)
	f.request(t, "guest", "POST", path+"/sync", body, 404)
	f.request(t, "owner", "GET", fmt.Sprintf("/project/%d/secret_storages/%d", f.other.ID, storage.ID), "", 404)
	storage.Name = "Updated Vault"
	storage.Secret = ""
	encoded, err := json.Marshal(storage)
	require.NoError(t, err)
	f.request(t, "manager", "PUT", path, string(encoded), 200)
	credentials, err := f.store.GetAccessKeys(f.project.ID, db.GetAccessKeyOptions{Owner: db.AccessKeySecretStorage, StorageID: &storage.ID}, db.RetrieveQueryParams{})
	require.NoError(t, err)
	require.Len(t, credentials, 1)
	require.NoError(t, enc.DeserializeSecret(&credentials[0]))
	require.Equal(t, "fixture-api-token", credentials[0].String)
	storage.Secret = "rotated-api-token"
	encoded, err = json.Marshal(storage)
	require.NoError(t, err)
	w = f.request(t, "manager", "PUT", path, string(encoded), 200)
	require.NotContains(t, w.Body.String(), storage.Secret)
	// A missing reference cannot create a seemingly usable read-only key.
	keyBody := fmt.Sprintf(`{"name":"ref","type":"string","project_id":%d,"source_storage_type":"vault","source_storage_id":%d,"source_storage_key":"app","source_mapping":{"value":"token"}}`, f.project.ID, storage.ID)
	w = f.request(t, "manager", "POST", fmt.Sprintf("/project/%d/keys", f.project.ID), keyBody, 201)
	var key db.AccessKey
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &key))
	f.request(t, "manager", "DELETE", path, "", 400)
	f.request(t, "manager", "DELETE", fmt.Sprintf("/project/%d/keys/%d", f.project.ID, key.ID), "", 204)
	f.request(t, "manager", "DELETE", path, "", 204)
	f.request(t, "owner", "GET", path, "", 404)
	credentials, err = f.store.GetAccessKeys(f.project.ID, db.GetAccessKeyOptions{Owner: db.AccessKeySecretStorage, StorageID: &storage.ID}, db.RetrieveQueryParams{})
	require.NoError(t, err)
	require.Empty(t, credentials)
	storage.ID = 0
	storage.Type = db.SecretStorageTypeAwsSm
	encoded, err = json.Marshal(storage)
	require.NoError(t, err)
	f.request(t, "owner", "POST", base, string(encoded), 400)
	legacy, err := f.store.CreateSecretStorage(db.SecretStorage{ProjectID: f.project.ID, Name: "Legacy DVLS", Type: db.SecretStorageTypeDvls, Params: db.MapStringAnyField{"app_key": "legacy-private-value"}})
	require.NoError(t, err)
	for _, endpoint := range []string{base, fmt.Sprintf("%s/%d", base, legacy.ID)} {
		w = f.request(t, "guest", "GET", endpoint, "", 200)
		require.Contains(t, w.Body.String(), "Legacy DVLS")
		require.NotContains(t, w.Body.String(), "legacy-private-value")
	}
	f.request(t, "owner", "DELETE", fmt.Sprintf("%s/%d", base, legacy.ID), "", 204)
}

func TestVaultStructuredCredentialsAPIRedactionAndTestScope(t *testing.T) {
	f := newCoreFixture(t)
	util.Config.AccessKeyEncryption = "hHYgPrhQTZYm7UFTvcdNfKJMB3wtAXtJENUButH+DmM="
	enc := server.NewAccessKeyEncryptionService(f.store, f.store, f.store, f.store)
	keys := server.NewAccessKeyService(f.store, enc, f.store, f.store)
	service := server.NewSecretStorageService(f.store, f.store, keys, enc)
	f.router = Route(f.store, nil, nil, f.store, nil, nil, nil, enc, nil, service, keys, nil, nil, server.NewRunnerService(f.store), nil, nil)
	base := fmt.Sprintf("/project/%d/secret_storages", f.project.ID)
	storage := db.SecretStorage{ProjectID: f.project.ID, Name: "AppRole", Type: db.SecretStorageTypeVault, ReadOnly: true, Params: db.MapStringAnyField{"url": "http://127.0.0.1:1", "auth_method": "approle"}, Credentials: map[string]db.VaultCredential{"role_id": {Source: "database", Value: "api-private-role"}, "secret_id": {Source: "database", Value: "api-private-secret"}}}
	body, err := json.Marshal(storage)
	require.NoError(t, err)
	f.request(t, "guest", "POST", base+"/test", string(body), 403)
	response := f.request(t, "manager", "POST", base, string(body), 201)
	require.NotContains(t, response.Body.String(), "api-private")
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &storage))
	path := fmt.Sprintf("%s/%d", base, storage.ID)
	for _, endpoint := range []string{base, path} {
		response = f.request(t, "guest", "GET", endpoint, "", 200)
		require.NotContains(t, response.Body.String(), "api-private")
	}
	response = f.request(t, "manager", "GET", path, "", 200)
	var edit db.SecretStorage
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &edit))
	require.True(t, edit.Credentials["secret_id"].Configured)
	require.Empty(t, edit.Credentials["secret_id"].Value)
	edit.Credentials["secret_id"] = db.VaultCredential{Source: "database", Value: "api-private-rotated"}
	body, err = json.Marshal(edit)
	require.NoError(t, err)
	response = f.request(t, "manager", "PUT", path, string(body), 200)
	require.NotContains(t, response.Body.String(), "api-private")
	response = f.request(t, "manager", "POST", path+"/sync", string(body), 404)
	require.NotContains(t, response.Body.String(), "api-private")
	response = f.request(t, "manager", "POST", base+"/test", string(body), 400)
	require.NotContains(t, response.Body.String(), "api-private")
	// A request cannot borrow another project's stored credentials.
	edit.ProjectID = f.other.ID
	body, err = json.Marshal(edit)
	require.NoError(t, err)
	response = f.request(t, "owner", "POST", fmt.Sprintf("/project/%d/secret_storages/test", f.other.ID), string(body), 404)
	require.NotContains(t, response.Body.String(), "api-private")
}

func TestVaultReferenceAPI_MetadataChangesDoNotWriteRemoteSecrets(t *testing.T) {
	f := newCoreFixture(t)
	enc := server.NewAccessKeyEncryptionService(f.store, f.store, f.store, f.store)
	keys := server.NewAccessKeyService(f.store, enc, f.store, f.store)
	service := server.NewSecretStorageService(f.store, f.store, keys, enc)
	f.router = Route(f.store, nil, nil, f.store, nil, nil, nil, enc, nil, service, keys, nil, nil, server.NewRunnerService(f.store), nil, nil)
	// No provider is available. Creating/editing/removing references must work offline.
	storage, err := service.Create(db.SecretStorage{ProjectID: f.project.ID, Name: "Writable vault", Type: db.SecretStorageTypeVault, ReadOnly: false, Secret: "fixture-token", Params: db.MapStringAnyField{"url": "http://127.0.0.1:1"}})
	require.NoError(t, err)
	base := fmt.Sprintf("/project/%d/keys", f.project.ID)
	key := db.AccessKey{Name: "test", ProjectID: &f.project.ID, Type: db.AccessKeyString, SourceStorageType: new(db.AccessKeySourceStorageVault), SourceStorageID: &storage.ID, SourceStorageKey: new("proxmox"), SourceMapping: db.MapStringAnyField{"value": "dns_server"}, ReferenceOnly: true}
	encode := func() string { b, e := json.Marshal(key); require.NoError(t, e); return string(b) }
	f.request(t, "guest", "POST", base, encode(), 403)
	response := f.request(t, "manager", "POST", base, encode(), 201)
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &key))
	path := fmt.Sprintf("%s/%d", base, key.ID)
	key.ReferenceOnly = true
	key.Type = db.AccessKeySSH
	key.SourceMapping = db.MapStringAnyField{"private_key": "dns_server"}
	f.request(t, "guest", "PUT", path, encode(), 403)
	f.request(t, "manager", "PUT", path, encode(), 204)
	saved, err := f.store.GetAccessKey(f.project.ID, key.ID)
	require.NoError(t, err)
	require.Equal(t, db.AccessKeySSH, saved.Type)
	require.Nil(t, saved.Secret)
	key.Type = db.AccessKeyString
	key.SourceMapping = db.MapStringAnyField{"value": "dns_server"}
	key.Name = "renamed"
	storage.ReadOnly = true
	require.NoError(t, service.Update(storage))
	f.request(t, "manager", "PUT", path, encode(), 204)
	key.String = "must not be written"
	response = f.request(t, "manager", "PUT", path, encode(), 400)
	require.Contains(t, response.Body.String(), "reference-only")
	key.String = ""
	key.SourceStorageID = new(storage.ID + 10000)
	f.request(t, "manager", "PUT", path, encode(), 404)
	key.SourceStorageID = &storage.ID
	storage.ReadOnly = false
	require.NoError(t, service.Update(storage))
	f.request(t, "manager", "DELETE", path+"?reference_only=true", "", 204)
}
