package api

import (
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/impishMD/taskexec/api/helpers"
	"github.com/impishMD/taskexec/db"
	"github.com/impishMD/taskexec/pkg/task_logger"
	"github.com/impishMD/taskexec/services/server"
	"github.com/impishMD/taskexec/services/tasks"
	"github.com/impishMD/taskexec/util"
	"github.com/stretchr/testify/require"
)

func TestTerraformBackendProtocolAndPermissions(t *testing.T) {
	f := newCoreFixture(t)
	enc := server.NewAccessKeyEncryptionService(f.store, f.store, f.store, f.store)
	keys := server.NewAccessKeyService(f.store, enc, f.store, f.store)
	f.router = Route(f.store, f.store, nil, f.store, nil, nil, nil, enc, nil, nil, keys, nil, nil, server.NewRunnerService(f.store), nil, nil)
	inv, err := f.store.CreateInventory(db.Inventory{ProjectID: f.project.ID, Name: "default", Type: db.InventoryTerraformWorkspace})
	require.NoError(t, err)
	key, err := keys.Create(db.AccessKey{Name: "backend", ProjectID: &f.project.ID, Type: db.AccessKeyLoginPassword, LoginPassword: db.LoginPassword{Login: "operator", Password: "fixture-password"}})
	require.NoError(t, err)
	prefix := fmt.Sprintf("/project/%d/inventory/%d/terraform", f.project.ID, inv.ID)
	body := fmt.Sprintf(`{"auth_key_id":%d,"project_id":%d}`, key.ID, f.other.ID)
	f.request(t, "guest", "POST", prefix+"/aliases", body, 403)
	w := f.request(t, "manager", "POST", prefix+"/aliases", body, 201)
	var alias db.TerraformInventoryAlias
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &alias))
	require.Equal(t, f.project.ID, alias.ProjectID)
	require.Len(t, alias.Alias, 32)
	aliasPath := prefix + "/aliases/" + alias.Alias
	f.request(t, "guest", "GET", aliasPath, "", 200)
	f.request(t, "outsider", "GET", aliasPath, "", 404)
	backend := func(method, suffix, payload, password string, status int) *httptest.ResponseRecorder {
		t.Helper()
		r := httptest.NewRequest(method, "/api/terraform/"+alias.Alias+suffix, strings.NewReader(payload))
		r.SetBasicAuth("operator", password)
		r = helpers.SetContextValue(r, "store", f.store)
		w := httptest.NewRecorder()
		f.router.ServeHTTP(w, r)
		require.Equal(t, status, w.Code, "%s: %s", method, w.Body.String())
		require.Equal(t, "no-store", w.Header().Get("Cache-Control"))
		return w
	}
	backend("GET", "", "", "bad", 401)
	backend("GET", "", "", "fixture-password", 404)
	lock := `{"ID":"lock-one","Operation":"apply","Who":"test"}`
	backend("LOCK", "", lock, "fixture-password", 200)
	conflict := backend("LOCK", "", `{"ID":"lock-two"}`, "fixture-password", 409)
	require.Contains(t, conflict.Body.String(), `"ID":"lock-one"`)
	state := `{"version":4,"serial":1,"lineage":"fixture","outputs":{"token":{"value":"state-secret","sensitive":true}}}`
	backend("POST", "", state, "fixture-password", 409)
	backend("POST", "?ID=wrong", state, "fixture-password", 409)
	backend("POST", "?ID=lock-one", state, "fixture-password", 200)
	require.JSONEq(t, state, backend("GET", "", "", "fixture-password", 200).Body.String())
	w = f.request(t, "guest", "GET", prefix+"/states", "", 200)
	require.NotContains(t, w.Body.String(), "state-secret")
	var states []db.TerraformInventoryState
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &states))
	require.Len(t, states, 1)
	statePath := fmt.Sprintf("%s/states/%d", prefix, states[0].ID)
	f.request(t, "guest", "GET", statePath, "", 403)
	f.request(t, "guest", "GET", prefix+"/states/latest", "", 403)
	require.Contains(t, f.request(t, "manager", "GET", statePath, "", 200).Body.String(), "state-secret")
	f.request(t, "manager", "DELETE", statePath, "", 409)
	backend("UNLOCK", "", `{"ID":"wrong"}`, "fixture-password", 409)
	backend("UNLOCK", "", lock, "fixture-password", 200)
	backend("POST", "", `[]`, "fixture-password", 400)
	backend("LOCK", "", `{}`, "fixture-password", 400)
	backend("DELETE", "", "", "fixture-password", 200)
	backend("GET", "", "", "fixture-password", 404)
	f.request(t, "manager", "GET", statePath, "", 200) // Reset retains history.
	f.request(t, "manager", "DELETE", aliasPath, "", 204)
	backend("GET", "", "", "fixture-password", 404)
	f.request(t, "manager", "DELETE", statePath, "", 204)
}

func TestTerraformTaskAliasExpiresAndUsesTaskWorkspace(t *testing.T) {
	f := newCoreFixture(t)
	util.Config.Apps = map[string]util.App{"tofu": {}}
	key, err := f.store.CreateAccessKey(db.AccessKey{ProjectID: &f.project.ID, Type: db.AccessKeyNone})
	require.NoError(t, err)
	repo, err := f.store.CreateRepository(db.Repository{ProjectID: f.project.ID, Name: "repo", GitURL: "https://example.com/repo.git", GitBranch: "main", SSHKeyID: key.ID})
	require.NoError(t, err)
	inv, err := f.store.CreateInventory(db.Inventory{ProjectID: f.project.ID, Type: db.InventoryTofuWorkspace, Name: "task workspace"})
	require.NoError(t, err)
	tpl, err := f.store.CreateTemplate(db.Template{ProjectID: f.project.ID, RepositoryID: repo.ID, Name: "tofu", App: db.AppTofu, InventoryID: &inv.ID})
	require.NoError(t, err)
	task, err := f.store.CreateTask(db.Task{ProjectID: f.project.ID, TemplateID: tpl.ID, Status: task_logger.TaskRunningStatus}, 0)
	require.NoError(t, err)
	state := tasks.NewMemoryTaskStateStore()
	state.SetAlias("active-task-capability", &tasks.TaskRunner{Task: task, Template: tpl, Inventory: inv})
	pool := new(tasks.CreateTaskPool(f.store, state, f.store, nil, nil, nil, nil, nil))
	f.router = Route(f.store, f.store, nil, f.store, pool, nil, nil, nil, nil, nil, nil, nil, nil, server.NewRunnerService(f.store), nil, nil)
	backend := func(method, payload string, status int) {
		t.Helper()
		r := httptest.NewRequest(method, "/api/terraform/active-task-capability", strings.NewReader(payload))
		r = helpers.SetContextValue(r, "store", f.store)
		w := httptest.NewRecorder()
		f.router.ServeHTTP(w, r)
		require.Equal(t, status, w.Code, w.Body.String())
	}
	backend("POST", `{"version":4,"project_id":999,"inventory_id":999}`, 200)
	saved, err := f.store.GetLatestTerraformInventoryState(f.project.ID, inv.ID)
	require.NoError(t, err)
	require.Equal(t, &task.ID, saved.TaskID)
	_, err = f.store.GetLatestTerraformInventoryState(f.other.ID, inv.ID)
	require.ErrorIs(t, err, db.ErrNotFound)
	backend("GET", "", 200)
	task.Status = task_logger.TaskSuccessStatus
	require.NoError(t, f.store.UpdateTask(task))
	backend("GET", "", 404)
	backend("POST", `{"version":4}`, 404)
}
