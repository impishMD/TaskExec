package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/impishMD/taskexec/api/helpers"
	"github.com/impishMD/taskexec/db"
	"github.com/impishMD/taskexec/db/sql"
	"github.com/impishMD/taskexec/services/server"
	"github.com/impishMD/taskexec/util"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Exercise the real router, authentication, permission middleware and SQL store.
// A controller-only test would miss a handler registered under the wrong guard.
type coreFixture struct {
	store          *sql.SqlDb
	router         http.Handler
	project, other db.Project
	tokens         map[string]string
}

func newCoreFixture(t *testing.T) coreFixture {
	t.Helper()
	store := setupSessionTest(t)
	util.Config.Debugging = &util.DebuggingConfig{}
	p, err := store.CreateProject(db.Project{Name: "primary"})
	require.NoError(t, err)
	other, err := store.CreateProject(db.Project{Name: "other"})
	require.NoError(t, err)
	f := coreFixture{store: store, project: p, other: other, tokens: map[string]string{}}
	for _, name := range []string{"admin", "owner", "manager", "guest", "outsider"} {
		u := createUserOptionsTestUser(t, store, name)
		if name == "admin" {
			u = makeAdmin(t, store, u)
		}
		if name != "outsider" && name != "admin" {
			_, err = store.CreateProjectUser(db.ProjectUser{ProjectID: p.ID, UserID: u.ID, Role: db.ProjectUserRole(name)})
			require.NoError(t, err)
		}
		token, err := store.CreateAPIToken(db.APIToken{ID: "coretesttoken" + name, UserID: u.ID})
		require.NoError(t, err)
		f.tokens[name] = token.ID
	}
	f.router = Route(store, nil, nil, store, nil, nil, nil, nil, nil, nil, nil, nil, nil, server.NewRunnerService(store), nil, nil)
	return f
}

func (f coreFixture) request(t *testing.T, actor, method, path, body string, status int) *httptest.ResponseRecorder {
	t.Helper()
	r := httptest.NewRequest(method, "/api"+path, strings.NewReader(body))
	r.Header.Set("Authorization", "Bearer "+f.tokens[actor])
	r.Header.Set("Content-Type", "application/json")
	r = helpers.SetContextValue(r, "store", f.store)
	w := httptest.NewRecorder()
	f.router.ServeHTTP(w, r)
	require.Equal(t, status, w.Code, "%s %s as %s: %s", method, path, actor, w.Body.String())
	return w
}

func TestCoreProjectRunners_IsolationAndLifecycle(t *testing.T) {
	f := newCoreFixture(t)
	base := fmt.Sprintf("/project/%d/runners", f.project.ID)
	body := fmt.Sprintf(`{"name":"project-runner","registered":true,"active":true,"project_id":%d,"tags":["linux"]}`, f.other.ID)
	w := f.request(t, "manager", "POST", base, body, 201)
	var created struct {
		db.Runner
		Token string `json:"token"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &created))
	require.NotEmpty(t, created.Token)
	require.Equal(t, &f.project.ID, created.ProjectID, "body cannot select another project")
	path := fmt.Sprintf("%s/%d", base, created.ID)
	for _, endpoint := range []string{base, path} {
		w = f.request(t, "guest", "GET", endpoint, "", 200)
		assert.NotContains(t, w.Body.String(), created.Token)
		assert.NotContains(t, w.Body.String(), `"token"`)
	}
	f.request(t, "outsider", "GET", base, "", 404)
	f.request(t, "guest", "POST", base, body, 403)
	foreign, err := f.store.CreateRunner(db.Runner{Name: "foreign", ProjectID: &f.other.ID})
	require.NoError(t, err)
	global, err := f.store.CreateRunner(db.Runner{Name: "global"})
	require.NoError(t, err)
	for _, runner := range []db.Runner{foreign, global} {
		foreignPath := fmt.Sprintf("%s/%d", base, runner.ID)
		for _, op := range []struct{ method, suffix, body string }{
			{"GET", "", ""}, {"PUT", "", body}, {"DELETE", "", ""},
			{"POST", "/active", `{"active":true}`}, {"POST", "/registration-token", ""}, {"DELETE", "/cache", ""},
		} {
			f.request(t, "owner", op.method, foreignPath+op.suffix, op.body, 404)
		}
	}
	w = f.request(t, "owner", "GET", fmt.Sprintf("/project/%d/runner_tags", f.project.ID), "", 200)
	assert.Contains(t, w.Body.String(), "linux")
	f.request(t, "guest", "POST", path+"/registration-token", "", 403)
	f.request(t, "manager", "PUT", path, fmt.Sprintf(`{"name":"updated","project_id":%d,"active":true}`, f.other.ID), 204)
	runner, err := f.store.GetRunner(f.project.ID, created.ID)
	require.NoError(t, err)
	assert.Equal(t, "updated", runner.Name)
	assert.Equal(t, created.Token, runner.Token)
	f.request(t, "manager", "DELETE", path+"/cache", "", 204)
	w = f.request(t, "manager", "POST", path+"/registration-token", "", 200)
	var registration struct {
		Token string `json:"registration_token"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &registration))
	runner, err = f.store.GetRunner(f.project.ID, created.ID)
	require.NoError(t, err)
	require.NotEmpty(t, registration.Token)
	assert.Equal(t, server.HashRunnerRegistrationToken(registration.Token), *runner.RegistrationTokenHash)
	assert.Empty(t, runner.Token)
	assert.True(t, runner.Active, "registration rotation preserves the configured enabled state")
	f.request(t, "manager", "DELETE", path, "", 204)
	f.request(t, "owner", "GET", path, "", 404)
	f.request(t, "owner", "GET", "/runners", "", 403)
	f.request(t, "admin", "GET", "/runners", "", 200)
}

func TestCoreRoles_ScopesValidationAndPermissions(t *testing.T) {
	f := newCoreFixture(t)
	base := fmt.Sprintf("/project/%d/roles", f.project.ID)
	body := fmt.Sprintf(`{"slug":"operators","name":" Operators ","permissions":1,"project_id":%d}`, f.other.ID)
	for _, actor := range []string{"guest", "manager"} {
		f.request(t, actor, "POST", base, body, 403)
	}
	f.request(t, "outsider", "GET", base, "", 404)
	f.request(t, "owner", "POST", "/roles", body, 403)
	f.request(t, "owner", "POST", base, body, 201)
	role, err := f.store.GetProjectRole(f.project.ID, "operators")
	require.NoError(t, err)
	assert.Equal(t, "Operators", role.Name)
	f.request(t, "admin", "GET", fmt.Sprintf("/project/%d/roles/operators", f.other.ID), "", 404)
	for _, method := range []string{"GET", "PUT", "DELETE"} {
		f.request(t, "admin", method, "/roles/operators", body, 404)
	}
	for _, bad := range []string{
		`{"slug":"owner","name":"Owner","permissions":15}`,
		`{"slug":"bad role","name":"Bad"}`, `{"slug":"invalid","name":" "}`,
		`{"slug":"invalid","name":"Invalid","permissions":16}`,
	} {
		f.request(t, "owner", "POST", base, bad, 400)
	}
	f.request(t, "owner", "PUT", base+"/operators", `{"slug":"renamed","name":"Name"}`, 400)
	f.request(t, "owner", "PUT", base+"/operators", `{"slug":"operators","name":"Changed","permissions":4}`, 204)
	f.request(t, "admin", "POST", "/roles", `{"slug":"global-ops","name":"Global","permissions":1}`, 201)
	w := f.request(t, "guest", "GET", base+"/all", "", 200)
	assert.Contains(t, w.Body.String(), "global-ops")
	assert.Contains(t, w.Body.String(), "operators")
	w = f.request(t, "guest", "GET", base, "", 200)
	assert.NotContains(t, w.Body.String(), "global-ops")
	member := createUserOptionsTestUser(t, f.store, "custom")
	_, err = f.store.CreateProjectUser(db.ProjectUser{ProjectID: f.project.ID, UserID: member.ID, Role: "operators"})
	require.NoError(t, err)
	f.request(t, "owner", "DELETE", base+"/operators", "", 409)
	require.NoError(t, f.store.DeleteProjectUser(f.project.ID, member.ID))
	f.request(t, "owner", "DELETE", base+"/operators", "", 204)
	f.request(t, "admin", "DELETE", "/roles/global-ops", "", 204)
}

func TestCoreTemplatePermissions_RequireUserManagement(t *testing.T) {
	f := newCoreFixture(t)
	key, err := f.store.CreateAccessKey(db.AccessKey{ProjectID: &f.project.ID, Type: db.AccessKeyNone})
	require.NoError(t, err)
	repo, err := f.store.CreateRepository(db.Repository{ProjectID: f.project.ID, Name: "repo", GitURL: "https://example.com/repo.git", GitBranch: "main", SSHKeyID: key.ID})
	require.NoError(t, err)
	tpl, err := f.store.CreateTemplate(db.Template{ProjectID: f.project.ID, RepositoryID: repo.ID, Name: "test", Playbook: "test.yml"})
	require.NoError(t, err)
	base := fmt.Sprintf("/project/%d/templates/%d/perms", f.project.ID, tpl.ID)
	list := f.request(t, "owner", "GET", base, "", 200)
	assert.JSONEq(t, `[]`, list.Body.String(), "the empty permissions page must remain usable")
	f.request(t, "manager", "POST", base, `{"role_slug":"manager","permissions":15}`, 403)
	f.request(t, "owner", "POST", base, `{"role_slug":"missing","permissions":1}`, 404)
	f.request(t, "owner", "POST", base, `{"role_slug":"guest","permissions":1}`, 404)
	f.request(t, "admin", "POST", fmt.Sprintf("/project/%d/roles", f.other.ID), `{"slug":"foreign-role","name":"Foreign"}`, 201)
	f.request(t, "owner", "POST", base, `{"role_slug":"foreign-role","permissions":1}`, 404)
	f.request(t, "admin", "POST", "/roles", `{"slug":"template-only","name":"Template only"}`, 201)
	w := f.request(t, "owner", "POST", base, `{"role_slug":"template-only","permissions":1}`, 201)
	var perm db.TemplateRolePerm
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &perm))
	path := fmt.Sprintf("%s/%d", base, perm.ID)
	f.request(t, "manager", "PUT", path, `{"permissions":15}`, 403)
	f.request(t, "manager", "DELETE", path, "", 403)
	f.request(t, "admin", "DELETE", "/roles/template-only", "", 409)
	f.request(t, "owner", "DELETE", path, "", 204)
	f.request(t, "admin", "DELETE", "/roles/template-only", "", 204)
}

func TestCoreAnsibleSummary_ReadAccessAndProjectIsolation(t *testing.T) {
	f := newCoreFixture(t)
	key, err := f.store.CreateAccessKey(db.AccessKey{ProjectID: &f.project.ID, Type: db.AccessKeyNone})
	require.NoError(t, err)
	repo, err := f.store.CreateRepository(db.Repository{ProjectID: f.project.ID, Name: "repo", GitURL: "https://example.com/repo.git", GitBranch: "main", SSHKeyID: key.ID})
	require.NoError(t, err)
	inv, err := f.store.CreateInventory(db.Inventory{ProjectID: f.project.ID, Type: db.InventoryStatic, Inventory: "localhost ansible_connection=local"})
	require.NoError(t, err)
	tpl, err := f.store.CreateTemplate(db.Template{ProjectID: f.project.ID, RepositoryID: repo.ID, Name: "test", Playbook: "test.yml", App: db.AppAnsible, InventoryID: &inv.ID})
	require.NoError(t, err)
	task, err := f.store.CreateTask(db.Task{ProjectID: f.project.ID, TemplateID: tpl.ID}, 0)
	require.NoError(t, err)
	for _, suffix := range []string{"ansible/hosts", "ansible/errors", "stages"} {
		path := fmt.Sprintf("/project/%d/tasks/%d/%s", f.project.ID, task.ID, suffix)
		for _, actor := range []string{"admin", "owner", "manager", "guest"} {
			w := f.request(t, actor, "GET", path, "", 200)
			assert.JSONEq(t, `[]`, w.Body.String())
		}
		f.request(t, "outsider", "GET", path, "", 404)
		f.request(t, "admin", "GET", fmt.Sprintf("/project/%d/tasks/%d/%s", f.other.ID, task.ID, suffix), "", 400)
	}
	require.NoError(t, f.store.CreateAnsibleTaskHost(db.AnsibleTaskHost{ProjectID: f.project.ID, TaskID: task.ID, Host: "host", Ok: 1}))
	w := f.request(t, "guest", "GET", fmt.Sprintf("/project/%d/tasks/%d/ansible/hosts", f.project.ID, task.ID), "", 200)
	assert.Contains(t, w.Body.String(), `"host":"host"`)
}
