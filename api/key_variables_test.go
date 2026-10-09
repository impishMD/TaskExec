package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/impishMD/taskexec/db"
	"github.com/impishMD/taskexec/services/server"
	"github.com/stretchr/testify/require"
)

func TestVaultVariablePreviewPermissionsAndFreshValues(t *testing.T) {
	f := newCoreFixture(t)
	enc := server.NewAccessKeyEncryptionService(f.store, f.store, f.store, f.store)
	keys := server.NewAccessKeyService(f.store, enc, f.store, f.store)
	service := server.NewSecretStorageService(f.store, f.store, keys, enc)
	f.router = Route(f.store, nil, nil, f.store, nil, nil, nil, enc, nil, service, keys, nil, nil, server.NewRunnerService(f.store), nil, nil)
	var reads atomic.Int64
	var version atomic.Int64
	version.Store(1)
	remote := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "LIST" {
			require.Equal(t, "/v1/secret/metadata/", r.URL.Path)
			_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"keys": []string{"apps/", "proxmox"}}})
			return
		}
		require.Equal(t, "GET", r.Method)
		require.Equal(t, "/v1/secret/data/proxmox", r.URL.Path)
		reads.Add(1)
		_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"data": map[string]any{"dns_server": fmt.Sprintf("private-value-%d", version.Load()), "enabled": true}, "metadata": map[string]any{"version": version.Load()}}})
	}))
	defer remote.Close()
	storage, err := service.Create(db.SecretStorage{ProjectID: f.project.ID, Name: "Vault", Type: db.SecretStorageTypeVault, ReadOnly: true, Secret: "private-token", Params: db.MapStringAnyField{"url": remote.URL}})
	require.NoError(t, err)
	key, err := keys.Create(db.AccessKey{Name: "blabla", Type: db.AccessKeyObject, ProjectID: &f.project.ID, SourceStorageType: new(db.AccessKeySourceStorageVault), SourceStorageID: &storage.ID, SourceStorageKey: new("proxmox"), ReferenceOnly: true})
	require.NoError(t, err)
	base := fmt.Sprintf("/project/%d/keys/%d", f.project.ID, key.ID)
	fields := fmt.Sprintf("/project/%d/secret_storages/%d/fields", f.project.ID, storage.ID)
	paths := fmt.Sprintf("/project/%d/secret_storages/%d/paths", f.project.ID, storage.ID)
	for _, endpoint := range []string{base + "/preview", base + "/fields", fields, paths} {
		f.request(t, "guest", "POST", endpoint, `{"path":"proxmox"}`, 403)
		f.request(t, "outsider", "POST", endpoint, `{"path":"proxmox"}`, 404)
	}
	require.EqualValues(t, 0, reads.Load())
	for _, endpoint := range []string{base, fmt.Sprintf("/project/%d/keys", f.project.ID)} {
		response := f.request(t, "guest", "GET", endpoint, "", 200)
		require.NotContains(t, response.Body.String(), "private-")
	}
	require.EqualValues(t, 0, reads.Load())
	listing := f.request(t, "manager", "POST", paths, `{"path":""}`, 200)
	require.Equal(t, "no-store", listing.Header().Get("Cache-Control"))
	require.JSONEq(t, `{"paths":["apps/","proxmox"]}`, listing.Body.String())
	response := f.request(t, "manager", "POST", fields, `{"path":"proxmox"}`, 200)
	require.Equal(t, "no-store", response.Header().Get("Cache-Control"))
	require.Contains(t, response.Body.String(), "dns_server")
	require.NotContains(t, response.Body.String(), "private-")
	response = f.request(t, "manager", "POST", base+"/preview", "", 200)
	require.Contains(t, response.Body.String(), "private-value-1")
	require.Contains(t, response.Body.String(), "read_at")
	require.Equal(t, "no-store", response.Header().Get("Cache-Control"))
	version.Store(2)
	response = f.request(t, "manager", "POST", base+"/preview", "", 200)
	require.Contains(t, response.Body.String(), "private-value-2")
	require.NotContains(t, response.Body.String(), "private-token")
	foreign, err := f.store.CreateAccessKey(db.AccessKey{Name: "other", Type: db.AccessKeyObject, ProjectID: &f.other.ID})
	require.NoError(t, err)
	f.request(t, "manager", "POST", fmt.Sprintf("/project/%d/keys/%d/preview", f.project.ID, foreign.ID), "", 404)
	credential, err := keys.Create(db.AccessKey{Name: "ssh", Type: db.AccessKeySSH, ProjectID: &f.project.ID, SourceStorageType: new(db.AccessKeySourceStorageVault), SourceStorageID: &storage.ID, SourceStorageKey: new("proxmox"), SourceMapping: db.MapStringAnyField{"private_key": "dns_server"}, ReferenceOnly: true})
	require.NoError(t, err)
	f.request(t, "manager", "POST", fmt.Sprintf("/project/%d/keys/%d/preview", f.project.ID, credential.ID), "", 400)
	description := f.request(t, "manager", "POST", base+"/fields", "", 200)
	require.JSONEq(t, `{"paths":["", ".dns_server", ".enabled"]}`, description.Body.String())
	require.NotContains(t, description.Body.String(), "private-")
	require.Equal(t, "no-store", description.Header().Get("Cache-Control"))
	require.EqualValues(t, 4, reads.Load(), "credentials must not be read by the value-preview endpoint")
}
