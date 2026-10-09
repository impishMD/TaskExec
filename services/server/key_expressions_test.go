package server

import (
	"encoding/json"
	"testing"

	"github.com/impishMD/taskexec/db"
	"github.com/stretchr/testify/require"
)

func TestKeySourceExpressionsUseFreshValuesAndKeepOnlyReferences(t *testing.T) {
	f := newVaultFixture(t, 2, db.SecretStorageTypeVault)
	f.remote.put("database", map[string]any{"username": "app", "password": "first-secret", "port": float64(5432), "nested": map[string]any{"enabled": true}, "dns.servers": []any{"192.0.2.53"}})
	key := f.reference("database", "database", db.AccessKeyObject)
	key.ReferenceOnly = true
	key, err := f.keys.Create(key)
	require.NoError(t, err)
	env, err := f.store.CreateEnvironment(db.Environment{
		Name: "sources", ProjectID: f.project.ID,
		JSON:              `{"port":"{{ test.port }}","config":{"enabled":"{{ test.nested.enabled }}"},"dns":"{{ test[\"dns.servers\"][0] }}","ansible":"{{ inventory_hostname }}"}`,
		ENV:               new(`{"DATABASE_URL":"postgres://{{ test.username }}:{{ test.password }}@db:{{ test.port }}","LITERAL":"test.password"}`),
		KeySources:        []db.EnvironmentKeySource{{Prefix: "test", KeyID: key.ID}, {Prefix: "second", KeyID: key.ID}},
		SecretExpressions: []db.EnvironmentSecretExpression{{Name: "PASSWORD", Type: db.EnvironmentSecretEnv, Expression: "{{ second.password }}"}, {Name: "all_fields", Type: db.EnvironmentSecretVar, Expression: "{{ test }}"}},
		KeyBindings:       []db.EnvironmentKeyBinding{{Name: "direct", Type: db.EnvironmentSecretVar, KeyID: key.ID, Field: new("username")}},
	})
	require.NoError(t, err)
	for _, password := range []string{"first-secret", "rotated-secret"} {
		f.remote.put("database", map[string]any{"username": "app", "password": password, "port": float64(5432), "nested": map[string]any{"enabled": true}, "dns.servers": []any{"192.0.2.53"}})
		loaded, err := f.store.GetEnvironment(f.project.ID, env.ID)
		require.NoError(t, err)
		require.Len(t, loaded.KeySources, 2)
		require.NoError(t, f.encryption.FillEnvironmentSecrets(&loaded, true))
		values := map[string]any{}
		for _, secret := range loaded.Secrets {
			var value any
			require.NoError(t, json.Unmarshal(secret.JSONValue, &value))
			values[secret.Name] = value
		}
		require.Equal(t, float64(5432), values["port"])
		require.Equal(t, map[string]any{"enabled": true}, values["config"])
		require.Equal(t, "192.0.2.53", values["dns"])
		require.Equal(t, password, values["PASSWORD"])
		require.Equal(t, "postgres://app:"+password+"@db:5432", values["DATABASE_URL"])
		require.Equal(t, password, values["all_fields"].(map[string]any)["password"])
		require.Equal(t, "app", values["direct"])
		require.JSONEq(t, `{"ansible":"{{ inventory_hostname }}"}`, loaded.JSON)
		require.JSONEq(t, `{"LITERAL":"test.password"}`, *loaded.ENV)
		stored, err := f.store.GetEnvironment(f.project.ID, env.ID)
		require.NoError(t, err)
		encoded, err := json.Marshal(stored)
		require.NoError(t, err)
		require.NotContains(t, string(encoded), password)
		require.NotContains(t, string(encoded), "192.0.2.53")
		require.Contains(t, string(encoded), "{{ test.port }}")
	}
	// Unlinking a source without fixing its expressions is rejected atomically.
	env.KeySources = nil
	require.Error(t, f.store.UpdateEnvironment(env))
	unchanged, err := f.store.GetEnvironment(f.project.ID, env.ID)
	require.NoError(t, err)
	require.Len(t, unchanged.KeySources, 2)
	require.Error(t, f.keys.Delete(f.project.ID, key.ID, true))
	// Missing fields fail task preparation; no stale value or template reaches a task.
	f.remote.put("database", map[string]any{"username": "changed"})
	require.ErrorContains(t, f.encryption.FillEnvironmentSecrets(&unchanged, true), "field is missing")
}

func TestKeySourceValidationAndProjectIsolation(t *testing.T) {
	f := newVaultFixture(t, 2, db.SecretStorageTypeVault)
	key, err := f.keys.Create(db.AccessKey{Name: "token", Type: db.AccessKeyString, ProjectID: &f.project.ID, String: "private"})
	require.NoError(t, err)
	other, err := f.store.CreateProject(db.Project{Name: "other"})
	require.NoError(t, err)
	for _, sources := range [][]db.EnvironmentKeySource{
		{{Prefix: "same", KeyID: key.ID}, {Prefix: "same", KeyID: key.ID}},
		{{Prefix: "bad-prefix", KeyID: key.ID}},
		{{Prefix: "__proto__", KeyID: key.ID}},
	} {
		_, err = f.store.CreateEnvironment(db.Environment{Name: "invalid", ProjectID: f.project.ID, JSON: "{}", KeySources: sources})
		require.Error(t, err)
	}
	_, err = f.store.CreateEnvironment(db.Environment{Name: "foreign", ProjectID: other.ID, JSON: "{}", KeySources: []db.EnvironmentKeySource{{Prefix: "test", KeyID: key.ID}}})
	require.Error(t, err)
	_, err = f.store.CreateEnvironment(db.Environment{Name: "expression", ProjectID: f.project.ID, JSON: `{"v":"{{ test | unsafe }}"}`, KeySources: []db.EnvironmentKeySource{{Prefix: "test", KeyID: key.ID}}})
	require.ErrorContains(t, err, "invalid key expression")
}

func TestKeyExpressionResolverDoesNotEvaluateRemoteTemplates(t *testing.T) {
	sources := []db.EnvironmentKeySource{{Prefix: "test", KeyID: 1}}
	reads := 0
	read := func(string) (any, error) {
		reads++
		return map[string]any{"password": "{{ test.other }}", "other": "do-not-expand", "null": nil}, nil
	}
	value, used, err := resolveKeyValue("{{ test.password }}", sources, read)
	require.NoError(t, err)
	require.True(t, used)
	require.Equal(t, "{{ test.other }}", value)
	require.Equal(t, 1, reads)
	value, _, err = resolveKeyValue("{{ test.null }}", sources, read)
	require.NoError(t, err)
	require.Nil(t, value)
	_, _, err = resolveKeyValue("{{ test[\"missing\"] }}", sources, read)
	require.Error(t, err)
}
