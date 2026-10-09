package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/impishMD/taskexec/db"
	"github.com/impishMD/taskexec/db/sql"
	"github.com/impishMD/taskexec/util"
	"github.com/stretchr/testify/require"
)

// A strict KV endpoint: the tests exercise real HTTP, encrypted SQL credentials,
// the services and transactional imports without requiring a running provider.
type testVault struct {
	mu       sync.Mutex
	server   *httptest.Server
	version  int
	token    string
	docs     map[string]map[string]any
	versions map[string]int
	failure  string
}

func newTestVault(t *testing.T, version int) *testVault {
	t.Helper()
	v := &testVault{version: version, token: "fixture-token", docs: map[string]map[string]any{}, versions: map[string]int{}}
	v.server = httptest.NewServer(http.HandlerFunc(v.serve))
	t.Cleanup(v.server.Close)
	return v
}
func (v *testVault) put(path string, data map[string]any) {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.docs[path] = data
	v.versions[path]++
}
func (v *testVault) serve(w http.ResponseWriter, r *http.Request) {
	v.mu.Lock()
	defer v.mu.Unlock()
	if r.Header.Get("X-Vault-Token") != v.token || r.Header.Get("X-Vault-Namespace") != "engineering" {
		http.Error(w, "fixture-token must not be exposed", 403)
		return
	}
	prefix := "/v1/secret/"
	if v.version == 2 {
		if r.Method == "LIST" {
			prefix += "metadata/"
		} else {
			prefix += "data/"
		}
	}
	if !strings.HasPrefix(r.URL.Path, prefix) {
		http.Error(w, "wrong KV endpoint", 400)
		return
	}
	path := strings.TrimPrefix(r.URL.Path, prefix)
	if v.failure != "" && path == v.failure {
		http.Error(w, "fixture-token secret-response", 503)
		return
	}
	send := func(data any) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"data": data})
	}
	switch r.Method {
	case "GET":
		data, ok := v.docs[path]
		if !ok {
			w.WriteHeader(404)
			return
		}
		if v.version == 2 {
			send(map[string]any{"data": data, "metadata": map[string]any{"version": v.versions[path]}})
		} else {
			send(data)
		}
	case "POST":
		var data map[string]any
		if json.NewDecoder(r.Body).Decode(&data) != nil {
			w.WriteHeader(400)
			return
		}
		if v.version == 2 {
			opts, _ := data["options"].(map[string]any)
			if opts["cas"] != float64(v.versions[path]) {
				http.Error(w, "CAS mismatch", 400)
				return
			}
			data, _ = data["data"].(map[string]any)
		}
		v.docs[path] = data
		v.versions[path]++
		send(map[string]any{"version": v.versions[path]})
	case "DELETE":
		delete(v.docs, path)
		w.WriteHeader(204)
	case "LIST":
		prefix := strings.TrimRight(path, "/")
		if prefix != "" {
			prefix += "/"
		}
		names := map[string]bool{}
		for p := range v.docs {
			if !strings.HasPrefix(p, prefix) {
				continue
			}
			name := strings.TrimPrefix(p, prefix)
			if i := strings.Index(name, "/"); i >= 0 {
				name = name[:i+1]
			}
			if name != "" {
				names[name] = true
			}
		}
		list := make([]string, 0, len(names))
		for name := range names {
			list = append(list, name)
		}
		send(map[string]any{"keys": list})
	default:
		w.WriteHeader(405)
	}
}

type vaultFixture struct {
	store      *sql.SqlDb
	project    db.Project
	storage    db.SecretStorage
	keys       AccessKeyService
	encryption AccessKeyEncryptionService
	service    SecretStorageService
	remote     *testVault
}

func newVaultFixture(t *testing.T, version int, provider db.SecretStorageType) vaultFixture {
	t.Helper()
	old := util.Config
	store := sql.InitConfigCreateTestStore()
	t.Cleanup(func() { store.Close(); util.Config = old })
	util.Config.AccessKeyEncryption = "hHYgPrhQTZYm7UFTvcdNfKJMB3wtAXtJENUButH+DmM="
	util.Config.Dirs = &util.ConfigDirs{Secrets: t.TempDir()}
	p, err := store.CreateProject(db.Project{Name: "Vault fixture"})
	require.NoError(t, err)
	remote := newTestVault(t, version)
	enc := NewAccessKeyEncryptionService(store, store, store, store)
	keys := NewAccessKeyService(store, enc, store, store)
	service := NewSecretStorageService(store, store, keys, enc)
	storage, err := service.Create(db.SecretStorage{ProjectID: p.ID, Name: "Vault fixture", Type: provider, ReadOnly: true, Secret: remote.token, Params: map[string]any{"url": remote.server.URL, "kv_version": version, "namespace": "engineering"}})
	require.NoError(t, err)
	require.Empty(t, storage.Secret)
	return vaultFixture{store, p, storage, keys, enc, service, remote}
}
func (f vaultFixture) reference(name, path string, kind db.AccessKeyType, field ...string) db.AccessKey {
	mapping := db.MapStringAnyField{}
	switch kind {
	case db.AccessKeyString:
		mapping["value"] = "value"
		if len(field) > 0 {
			mapping["value"] = field[0]
		}
	case db.AccessKeySSH:
		mapping["private_key"] = "private_key"
	case db.AccessKeyLoginPassword:
		mapping["password"] = "password"
	}
	return db.AccessKey{Name: name, Type: kind, ProjectID: &f.project.ID, SourceStorageID: &f.storage.ID, SourceStorageType: new(db.AccessKeySourceStorageVault), SourceStorageKey: &path, SourceMapping: mapping}
}
func (f vaultFixture) load(t *testing.T, key db.AccessKey) db.AccessKey {
	t.Helper()
	k, err := f.store.GetAccessKey(f.project.ID, key.ID)
	require.NoError(t, err)
	require.Nil(t, k.Secret, "external plaintext must never be persisted")
	require.NoError(t, f.encryption.DeserializeSecret(&k))
	return k
}

func TestVaultReadAndWriteAcrossProvidersAndVersions(t *testing.T) {
	for _, provider := range []db.SecretStorageType{db.SecretStorageTypeVault} {
		for _, version := range []int{1, 2} {
			t.Run(fmt.Sprintf("%s/KV%d", provider, version), func(t *testing.T) {
				f := newVaultFixture(t, version, provider)
				f.remote.put("app", map[string]any{"value": "old", "sibling": "keep", "number": 42})
				k, err := f.keys.Create(f.reference("app", "app", db.AccessKeyString))
				require.NoError(t, err)
				require.Equal(t, "old", f.load(t, k).String)
				f.remote.put("app", map[string]any{"value": "rotated", "sibling": "keep", "number": 42})
				require.Equal(t, "rotated", f.load(t, k).String)
				number := f.reference("number", "app", db.AccessKeyString, "number")
				require.NoError(t, f.encryption.DeserializeSecret(&number))
				require.Equal(t, "42", number.String)
				f.remote.put("ssh", map[string]any{"private_key": "ssh-private", "login": "deploy", "passphrase": "pass"})
				ssh := f.reference("ssh", "ssh", db.AccessKeySSH)
				ssh.SourceMapping["login"] = "login"
				ssh.SourceMapping["passphrase"] = "passphrase"
				require.NoError(t, f.encryption.DeserializeSecret(&ssh))
				require.Equal(t, "deploy", ssh.SshKey.Login)
				f.remote.put("login", map[string]any{"login": "user", "password": "secret"})
				login := f.reference("login", "login", db.AccessKeyLoginPassword)
				require.NoError(t, f.encryption.DeserializeSecret(&login))
				require.Equal(t, "secret", login.LoginPassword.Password)
				k.String = "blocked"
				k.OverrideSecret = true
				require.ErrorIs(t, f.keys.Update(k), ErrReadOnlyStorage)
				require.Equal(t, "rotated", f.load(t, k).String)
				f.storage.ReadOnly = false
				require.NoError(t, f.service.Update(f.storage))
				k.String = "written"
				require.NoError(t, f.keys.Update(k))
				require.Equal(t, "written", f.load(t, k).String)
				number.String = "43"
				number.OverrideSecret = true
				number, err = f.keys.Create(number)
				require.NoError(t, err)
				require.NoError(t, f.keys.Delete(f.project.ID, number.ID))
				sibling := f.reference("sibling", "app", db.AccessKeyString, "sibling")
				require.NoError(t, f.encryption.DeserializeSecret(&sibling))
				require.Equal(t, "keep", sibling.String)
				require.NoError(t, f.keys.Delete(f.project.ID, k.ID))
				require.ErrorContains(t, f.encryption.DeserializeSecret(&k), "field is missing")
				generated := f.reference("generated", "", db.AccessKeyString)
				generated.String = "generated-value"
				generated, err = f.keys.Create(generated)
				require.NoError(t, err)
				require.NotEmpty(t, *generated.SourceStorageKey)
				require.Equal(t, "generated-value", f.load(t, generated).String)
			})
		}
	}
}

func TestVaultCredentialsValidationAndFailureIsolation(t *testing.T) {
	f := newVaultFixture(t, 2, db.SecretStorageTypeVault)
	tokens, err := f.store.GetAccessKeys(f.project.ID, db.GetAccessKeyOptions{Owner: db.AccessKeySecretStorage, StorageID: &f.storage.ID}, db.RetrieveQueryParams{})
	require.NoError(t, err)
	require.Len(t, tokens, 1)
	require.NotNil(t, tokens[0].Secret)
	require.NotContains(t, *tokens[0].Secret, f.remote.token)
	f.remote.put("app", map[string]any{"value": "one"})
	k := f.reference("key", "app", db.AccessKeyString)
	for _, source := range []db.AccessKeySourceStorageType{db.AccessKeySourceStorageEnv, db.AccessKeySourceStorageFile} {
		f.storage.SourceStorageType = &source
		if source == db.AccessKeySourceStorageEnv {
			t.Setenv("TASKEXEC_TEST_VAULT_TOKEN", f.remote.token)
			f.storage.Secret = "TASKEXEC_TEST_VAULT_TOKEN"
		} else {
			path := filepath.Join(util.Config.Dirs.Secrets, "token")
			require.NoError(t, os.WriteFile(path, []byte(f.remote.token+"\n"), 0600))
			f.storage.Secret = path
		}
		require.NoError(t, f.service.Update(f.storage))
		require.NoError(t, f.encryption.DeserializeSecret(&k))
		tokens, err = f.store.GetAccessKeys(f.project.ID, db.GetAccessKeyOptions{Owner: db.AccessKeySecretStorage, StorageID: &f.storage.ID}, db.RetrieveQueryParams{})
		require.NoError(t, err)
		require.Nil(t, tokens[0].Secret)
	}
	f.storage.Secret = ""
	require.NoError(t, f.service.Update(f.storage))
	require.NoError(t, f.encryption.DeserializeSecret(&k))
	foreign, err := f.store.CreateProject(db.Project{Name: "foreign"})
	require.NoError(t, err)
	foreignKey := k
	foreignKey.ProjectID = &foreign.ID
	require.ErrorIs(t, f.encryption.DeserializeSecret(&foreignKey), db.ErrNotFound)
	for _, p := range []string{"", "../app", "app/../other", "app?version=2", "app#", "app%2Fother"} {
		invalid := f.reference("invalid", p, db.AccessKeyString)
		require.Error(t, f.encryption.DeserializeSecret(&invalid), p)
	}
	missing := f.reference("missing", "app", db.AccessKeyString, "missing")
	require.ErrorContains(t, f.encryption.DeserializeSecret(&missing), "missing")
	f.remote.mu.Lock()
	f.remote.failure = "app"
	f.remote.mu.Unlock()
	err = f.encryption.DeserializeSecret(&k)
	require.ErrorContains(t, err, "HTTP 503")
	require.NotContains(t, err.Error(), "fixture-token")
	require.NotContains(t, err.Error(), "secret-response")
	for _, params := range []map[string]any{{"url": "https://user:password@example.com"}, {"url": "file:///tmp/token"}, {"url": f.remote.server.URL, "mount": "../sys"}, {"url": f.remote.server.URL, "kv_version": 3}, {"url": f.remote.server.URL, "token": "unsafe"}} {
		s := f.storage
		s.Params = params
		require.Error(t, f.service.Update(s))
	}
	redirected := false
	target := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { redirected = true }))
	defer target.Close()
	redirect := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, target.URL, 307) }))
	defer redirect.Close()
	f.storage.Params["url"] = redirect.URL
	require.NoError(t, f.service.Update(f.storage))
	require.ErrorContains(t, f.encryption.DeserializeSecret(&k), "HTTP 307")
	require.False(t, redirected)
}
