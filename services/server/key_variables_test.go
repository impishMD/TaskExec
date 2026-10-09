package server

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/impishMD/taskexec/db"
	"github.com/stretchr/testify/require"
)

func TestVaultFieldMappingAndVariableBindings(t *testing.T) {
	f := newVaultFixture(t, 2, db.SecretStorageTypeVault)
	doc := map[string]any{"dns_server": "192.0.2.53", "user": "deploy", "pwd": "private-password", "pem": "private-key", "phrase": "private-phrase", "nested": map[string]any{"ports": []any{float64(53)}, "enabled": true}, "number": float64(42), "empty": nil}
	f.remote.put("proxmox", doc)
	fields, err := f.service.DescribeVaultSecret(context.Background(), f.project.ID, f.storage.ID, "proxmox")
	require.NoError(t, err)
	encoded, err := json.Marshal(fields)
	require.NoError(t, err)
	require.NotContains(t, string(encoded), "private-password")
	require.Contains(t, string(encoded), `"type":"object"`)

	for _, tc := range []struct {
		kind    db.AccessKeyType
		mapping db.MapStringAnyField
	}{
		{db.AccessKeySSH, db.MapStringAnyField{"private_key": "pem", "login": "user", "passphrase": "phrase"}},
		{db.AccessKeyLoginPassword, db.MapStringAnyField{"password": "pwd", "login": "user"}},
		{db.AccessKeyString, db.MapStringAnyField{"value": "number"}},
		{db.AccessKeyObject, nil},
	} {
		key := f.reference(string(tc.kind), "proxmox", tc.kind)
		key.SourceMapping, key.ReferenceOnly = tc.mapping, true
		key, err = f.keys.Create(key)
		require.NoError(t, err)
		resolved := f.load(t, key)
		switch tc.kind {
		case db.AccessKeySSH:
			require.Equal(t, db.SshKey{Login: "deploy", PrivateKey: "private-key", Passphrase: "private-phrase"}, resolved.SshKey)
		case db.AccessKeyLoginPassword:
			require.Equal(t, db.LoginPassword{Login: "deploy", Password: "private-password"}, resolved.LoginPassword)
		case db.AccessKeyString:
			require.Equal(t, "42", resolved.String)
		case db.AccessKeyObject:
			require.Equal(t, doc, resolved.Object)
		}
	}
	key := f.reference("blabla", "proxmox", db.AccessKeyObject)
	key.ReferenceOnly = true
	key, err = f.keys.Create(key)
	require.NoError(t, err)
	env, err := f.store.CreateEnvironment(db.Environment{Name: "vars", ProjectID: f.project.ID, JSON: "{}", KeyBindings: []db.EnvironmentKeyBinding{
		{Name: "blabla", Type: db.EnvironmentSecretVar, KeyID: key.ID},
		{Name: "DNS", Type: db.EnvironmentSecretEnv, KeyID: key.ID, Field: new("dns_server")},
		{Name: "network", Type: db.EnvironmentSecretVar, KeyID: key.ID, Field: new("nested")},
	}})
	require.NoError(t, err)
	preview, err := ReadVariableKey(f.store, f.encryption, f.project.ID, key.ID)
	require.NoError(t, err)
	require.Equal(t, doc, preview)
	doc["dns_server"] = "192.0.2.54"
	f.remote.put("proxmox", doc)
	loaded, err := f.store.GetEnvironment(f.project.ID, env.ID)
	require.NoError(t, err)
	require.Empty(t, loaded.Secrets)
	require.NoError(t, f.encryption.FillEnvironmentSecrets(&loaded, true))
	require.Len(t, loaded.Secrets, 3)
	for _, s := range loaded.Secrets {
		var value any
		require.NoError(t, json.Unmarshal(s.JSONValue, &value))
		switch s.Name {
		case "blabla":
			require.Equal(t, doc, value)
		case "DNS":
			require.Equal(t, "192.0.2.54", value)
		case "network":
			require.Equal(t, doc["nested"], value)
		}
	}
	persisted, err := f.store.GetEnvironment(f.project.ID, env.ID)
	require.NoError(t, err)
	require.Empty(t, persisted.Secrets)
	require.Equal(t, "{}", persisted.JSON)
	require.Error(t, f.keys.Delete(*key.ProjectID, key.ID, true))
	key.Type, key.SourceMapping = db.AccessKeyString, db.MapStringAnyField{"value": "dns_server"}
	require.ErrorContains(t, f.keys.Update(key), "key type cannot be used")
	key.Type, key.SourceMapping = db.AccessKeyObject, nil
	require.NoError(t, f.keys.Update(key))
	// Validate references and collisions before replacing either the row or bindings.
	env.Name = "invalid replacement"
	env.JSON = `{"blabla":"collision"}`
	require.ErrorContains(t, f.store.UpdateEnvironment(env), "conflicts")
	persisted, err = f.store.GetEnvironment(f.project.ID, env.ID)
	require.NoError(t, err)
	require.Equal(t, "vars", persisted.Name)
	require.Len(t, persisted.KeyBindings, 3)
	other, err := f.store.CreateProject(db.Project{Name: "other"})
	require.NoError(t, err)
	env.ProjectID, env.ID, env.JSON = other.ID, 0, "{}"
	_, err = f.store.CreateEnvironment(env)
	require.Error(t, err)
	empty, err := f.store.GetEnvironments(other.ID, db.RetrieveQueryParams{})
	require.NoError(t, err)
	require.Empty(t, empty)
}

func TestVaultMappingsValidateFieldsAndRejectOldReferences(t *testing.T) {
	doc := map[string]any{"pw": "secret", "bad": map[string]any{"password": "nested"}, "n": 42, "nil": nil, "empty": "", "field#name": "valid"}
	for _, tc := range []struct {
		kind    db.AccessKeyType
		mapping db.MapStringAnyField
		want    string
	}{
		{db.AccessKeyString, nil, "required Vault field mapping"},
		{db.AccessKeyString, db.MapStringAnyField{"value": "missing"}, "mapped field is missing"},
		{db.AccessKeyString, db.MapStringAnyField{"value": "bad"}, "must be scalar"},
		{db.AccessKeyString, db.MapStringAnyField{"value": "nil"}, "must be scalar"},
		{db.AccessKeySSH, db.MapStringAnyField{"private_key": "n"}, "must be a string"},
		{db.AccessKeyLoginPassword, db.MapStringAnyField{"password": "empty"}, "must not be empty"},
		{db.AccessKeyObject, db.MapStringAnyField{"value": "pw"}, "invalid Vault field mapping"},
	} {
		_, err := resolveVaultDocument(&db.AccessKey{Type: tc.kind, SourceMapping: tc.mapping}, doc)
		require.ErrorContains(t, err, tc.want)
	}
	value, err := resolveVaultDocument(&db.AccessKey{Type: db.AccessKeyLoginPassword, SourceMapping: db.MapStringAnyField{"password": "pw"}}, doc)
	require.NoError(t, err)
	require.JSONEq(t, `{"password":"secret"}`, value)
	value, err = resolveVaultDocument(&db.AccessKey{Type: db.AccessKeyString, SourceMapping: db.MapStringAnyField{"value": "field#name"}}, doc)
	require.NoError(t, err)
	require.Equal(t, "valid", value)
	_, _, err = vaultReference(&db.AccessKey{Type: db.AccessKeyString, SourceStorageKey: new("app#value")})
	require.ErrorContains(t, err, "must not contain a field selector")
}

func TestWholeObjectEnvironmentExpansion(t *testing.T) {
	f := newVaultFixture(t, 2, db.SecretStorageTypeVault)
	doc := map[string]any{"password": "private-password", "username": "deploy", "nested": map[string]any{"enabled": true}, "ports": []any{float64(53)}, "nothing": nil}
	f.remote.put("apps/db", doc)
	key := f.reference("test", "apps/db", db.AccessKeyObject)
	key.ReferenceOnly = true
	key, err := f.keys.Create(key)
	require.NoError(t, err)
	binding := db.EnvironmentKeyBinding{Name: "test", Type: db.EnvironmentSecretEnv, KeyID: key.ID}
	env := db.Environment{ProjectID: f.project.ID, JSON: "{}", KeyBindings: []db.EnvironmentKeyBinding{binding}}
	require.NoError(t, f.encryption.FillEnvironmentSecrets(&env, true))
	got := map[string]any{}
	for _, secret := range env.Secrets {
		var value any
		require.NoError(t, json.Unmarshal(secret.JSONValue, &value))
		got[secret.Name] = value
	}
	require.Equal(t, map[string]any{"test": doc, "test_password": doc["password"], "test_username": doc["username"], "test_nested": doc["nested"], "test_ports": doc["ports"], "test_nothing": nil}, got)
	// Selected structured fields stay a single variable; expansion applies only to Entire value.
	binding.Field = new("nested")
	expanded, err := expandKeyBinding(binding, doc["nested"])
	require.NoError(t, err)
	require.Len(t, expanded, 1)
	require.Equal(t, "test", expanded[0].Name)
	binding.Field = nil
	for _, tc := range []struct {
		name string
		env  db.Environment
	}{
		{"plain", db.Environment{ENV: new(`{"test_password":"existing"}`)}},
		{"secret", db.Environment{Secrets: []db.EnvironmentSecret{{Name: "test_password", Type: db.EnvironmentSecretEnv}}}},
		{"binding", db.Environment{KeyBindings: []db.EnvironmentKeyBinding{{Name: "test_password", Type: db.EnvironmentSecretEnv, KeyID: key.ID, Field: new("username")}}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			current := tc.env
			current.ProjectID = f.project.ID
			current.KeyBindings = append([]db.EnvironmentKeyBinding{binding}, current.KeyBindings...)
			before := append([]db.EnvironmentSecret{}, current.Secrets...)
			require.ErrorContains(t, f.encryption.FillEnvironmentSecrets(&current, true), "conflicts")
			require.Equal(t, len(before), len(current.Secrets), "never append a partially expanded binding")
		})
	}
	for _, field := range []string{"bad-name", "bad.name", "space name", "ключ"} {
		_, err := expandKeyBinding(binding, map[string]any{field: "must-not-leak"})
		require.ErrorContains(t, err, "invalid environment field name")
		require.NotContains(t, err.Error(), "must-not-leak")
	}
	doc["password"] = "rotated-password"
	_, err = expandKeyBinding(db.EnvironmentKeyBinding{Name: "TASKEXEC", Type: db.EnvironmentSecretEnv}, map[string]any{"JWT": "must-not-leak"})
	require.ErrorContains(t, err, "invalid environment field name")
	f.remote.put("apps/db", doc)
	next := db.Environment{ProjectID: f.project.ID, KeyBindings: []db.EnvironmentKeyBinding{binding}}
	require.NoError(t, f.encryption.FillEnvironmentSecrets(&next, true))
	for _, secret := range next.Secrets {
		if secret.Name == "test_password" {
			require.JSONEq(t, `"rotated-password"`, string(secret.JSONValue))
		}
	}
}

func TestVaultDirectoryCompletion(t *testing.T) {
	for _, version := range []int{1, 2} {
		t.Run(fmt.Sprint(version), func(t *testing.T) {
			f := newVaultFixture(t, version, db.SecretStorageTypeVault)
			f.remote.put("proxmox", map[string]any{"value": "private"})
			f.remote.put("apps/db", map[string]any{"password": "private"})
			f.remote.put("apps/ci/token", map[string]any{"value": "private"})
			paths, err := f.service.ListVaultSecrets(context.Background(), f.project.ID, f.storage.ID, "")
			require.NoError(t, err)
			require.Equal(t, []string{"apps/", "proxmox"}, paths)
			paths, err = f.service.ListVaultSecrets(context.Background(), f.project.ID, f.storage.ID, "apps/")
			require.NoError(t, err)
			require.Equal(t, []string{"apps/ci/", "apps/db"}, paths)
			paths, err = f.service.ListVaultSecrets(context.Background(), f.project.ID, f.storage.ID, "missing/")
			require.NoError(t, err)
			require.Empty(t, paths)
			for _, invalid := range []string{"../bad", "a/../b", "secret#field"} {
				_, err = f.service.ListVaultSecrets(context.Background(), f.project.ID, f.storage.ID, invalid)
				require.Error(t, err)
			}
		})
	}
}
