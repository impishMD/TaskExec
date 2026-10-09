package tasks

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/impishMD/taskexec/db"
	"github.com/stretchr/testify/require"
)

func TestVariableKeysPreserveTypesThroughWorkerAndAnsible(t *testing.T) {
	setupExecutorConfig(t)
	object := json.RawMessage(`{"dns_server":"192.0.2.53","ports":[53,5353],"enabled":true,"nested":{"ttl":120},"empty":null}`)
	request := struct {
		Environment db.Environment `json:"environment"`
	}{Environment: db.Environment{JSON: "{}", Secrets: []db.EnvironmentSecret{
		{Name: "blabla", Type: db.EnvironmentSecretVar, JSONValue: object},
		{Name: "dns", Type: db.EnvironmentSecretVar, JSONValue: json.RawMessage(`"192.0.2.53"`)},
		{Name: "CONFIG", Type: db.EnvironmentSecretEnv, JSONValue: object},
		{Name: "DNS", Type: db.EnvironmentSecretEnv, JSONValue: json.RawMessage(`"192.0.2.53"`)},
	}}}
	// The private worker protocol must not turn an object into a quoted JSON string.
	encoded, err := json.Marshal(request)
	require.NoError(t, err)
	var received struct {
		Environment db.Environment `json:"environment"`
	}
	require.NoError(t, json.Unmarshal(encoded, &received))
	executor := LocalExecutor{Environment: received.Environment, Template: db.Template{Playbook: "site.yml"}, Inventory: db.Inventory{Type: db.InventoryFile, Inventory: "hosts"}}
	args, _, err := executor.getPlaybookArgs("admin", nil)
	require.NoError(t, err)
	extra := map[string]any{}
	for i, arg := range args {
		if arg == "--extra-vars" {
			require.NoError(t, json.Unmarshal([]byte(args[i+1]), &extra))
		}
	}
	require.Equal(t, "192.0.2.53", extra["dns"])
	require.Equal(t, "192.0.2.53", extra["blabla"].(map[string]any)["dns_server"])
	require.Equal(t, true, extra["blabla"].(map[string]any)["enabled"])
	require.Equal(t, []any{float64(53), float64(5353)}, extra["blabla"].(map[string]any)["ports"])
	require.NotContains(t, extra, "CONFIG")
	env, err := executor.getEnvironmentENV()
	require.NoError(t, err)
	require.Contains(t, env, "DNS=192.0.2.53")
	for _, entry := range env {
		if strings.HasPrefix(entry, "CONFIG=") {
			require.JSONEq(t, string(object), strings.TrimPrefix(entry, "CONFIG="))
		}
	}
	shell, err := executor.getShellArgs("admin", nil)
	require.NoError(t, err)
	require.Contains(t, shell, "dns=192.0.2.53")
	for _, arg := range shell {
		if strings.HasPrefix(arg, "blabla=") {
			require.JSONEq(t, string(object), strings.TrimPrefix(arg, "blabla="))
		}
	}
}
