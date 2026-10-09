package export

import (
	"github.com/impishMD/taskexec/db"
	"github.com/impishMD/taskexec/db/sql"
	"github.com/stretchr/testify/require"
	"strconv"
	"testing"
)

func TestEnvironmentBindingsRestoreAfterKeysWithRemappedIDs(t *testing.T) {
	store := sql.InitConfigCreateTestStore()
	defer store.Close()
	project, err := store.CreateProject(db.Project{Name: "restored"})
	require.NoError(t, err)
	key, err := store.CreateAccessKey(db.AccessKey{Name: "blabla", ProjectID: &project.ID, Type: db.AccessKeyObject})
	require.NoError(t, err)
	env, err := store.CreateEnvironment(db.Environment{Name: "vars", ProjectID: project.ID, JSON: "{}"})
	require.NoError(t, err)
	mapper := NewKeyMapper()
	require.NoError(t, mapper.mapKeys(Project, GlobalScope, "101", strconv.Itoa(project.ID)))
	require.NoError(t, mapper.mapKeys(Environment, "101", "202", strconv.Itoa(env.ID)))
	require.NoError(t, mapper.mapKeys(AccessKey, "101", "303", strconv.Itoa(key.ID)))
	chain := InitProjectExporters(mapper, true, true)
	_, err = getSortedKeys(chain.exporters, func(e TypeExporter) []string { return e.importDependsOn() })
	require.NoError(t, err, "no dependency cycle between keys and groups")
	exporter := &EnvironmentBindingsExporter{}
	require.NoError(t, exporter.restoreValue(EntityObject[db.Environment]{scope: "101", value: db.Environment{ID: 202, ProjectID: 101, KeySources: []db.EnvironmentKeySource{{Prefix: "test", KeyID: 303}}, SecretExpressions: []db.EnvironmentSecretExpression{{Name: "SECRET", Type: db.EnvironmentSecretEnv, Expression: "{{ test.dns_server }}"}}, KeyBindings: []db.EnvironmentKeyBinding{{Name: "dns", KeyID: 303, Type: db.EnvironmentSecretVar, Field: new("dns_server")}}}}, store, chain))
	restored, err := store.GetEnvironment(project.ID, env.ID)
	require.NoError(t, err)
	require.Equal(t, key.ID, restored.KeySources[0].KeyID)
	require.Equal(t, "{{ test.dns_server }}", restored.SecretExpressions[0].Expression)
	require.Equal(t, key.ID, restored.KeyBindings[0].KeyID)
	require.Equal(t, "dns_server", *restored.KeyBindings[0].Field)
}
