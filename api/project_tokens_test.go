package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/impishMD/taskexec/db"
	"github.com/impishMD/taskexec/pkg/task_logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type issuedProjectToken struct {
	db.ProjectToken
	Token string `json:"token"`
}

func tokenTestTemplate(t *testing.T, f coreFixture, project int, name string) db.Template {
	t.Helper()
	key, err := f.store.CreateAccessKey(db.AccessKey{ProjectID: &project, Type: db.AccessKeyNone})
	require.NoError(t, err)
	repo, err := f.store.CreateRepository(db.Repository{ProjectID: project, SSHKeyID: key.ID, Name: name, GitURL: "https://example.com/repo", GitBranch: "main"})
	require.NoError(t, err)
	tpl, err := f.store.CreateTemplate(db.Template{ProjectID: project, Name: name, RepositoryID: repo.ID, App: db.AppBash, Playbook: "run.sh"})
	require.NoError(t, err)
	return tpl
}

func issueProjectToken(t *testing.T, f coreFixture, body string) issuedProjectToken {
	t.Helper()
	w := f.request(t, "owner", "POST", fmt.Sprintf("/project/%d/tokens", f.project.ID), body, 201)
	var token issuedProjectToken
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &token))
	require.NotEmpty(t, token.Token)
	assert.NotContains(t, w.Body.String(), "secret_hash")
	f.tokens[token.Name] = token.Token
	return token
}

func TestProjectTokenManagementAndCreatorIndependence(t *testing.T) {
	f := newCoreFixture(t)
	base := fmt.Sprintf("/project/%d/tokens", f.project.ID)
	body := `{"name":"ci","scopes":["templates:read"],"all_templates":true}`
	for _, actor := range []string{"manager", "guest"} {
		f.request(t, actor, "GET", base, "", 403)
		f.request(t, actor, "POST", base, body, 403)
	}
	f.request(t, "outsider", "GET", base, "", 404)
	f.request(t, "admin", "GET", base, "", 200)
	created := issueProjectToken(t, f, body)
	stored, err := f.store.GetProjectToken(f.project.ID, created.ID)
	require.NoError(t, err)
	assert.NotEqual(t, created.Token, stored.SecretHash)
	assert.True(t, stored.MatchesSecret(created.Token))
	w := f.request(t, "owner", "GET", base, "", 200)
	assert.NotContains(t, w.Body.String(), created.Token)
	assert.NotContains(t, w.Body.String(), stored.SecretHash)
	var listed []db.ProjectToken
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &listed))
	require.Len(t, listed, 1)
	assert.Equal(t, created.Scopes, listed[0].Scopes)
	f.request(t, "ci", "GET", base, "", 403)
	f.request(t, "ci", "POST", base, body, 403)
	// Token survives both removal from the team and deletion of its creator.
	require.NoError(t, f.store.DeleteProjectUser(f.project.ID, stored.CreatorID))
	f.request(t, "ci", "GET", fmt.Sprintf("/project/%d/templates", f.project.ID), "", 200)
	require.NoError(t, f.store.DeleteUser(stored.CreatorID))
	f.request(t, "ci", "GET", fmt.Sprintf("/project/%d/templates", f.project.ID), "", 200)
	stored, err = f.store.GetProjectToken(f.project.ID, created.ID)
	require.NoError(t, err)
	assert.Equal(t, "owner", stored.CreatorName)
	assert.NotNil(t, stored.LastUsedAt)
	f.request(t, "admin", "DELETE", base+"/"+created.ID, "", 204)
	f.request(t, "ci", "GET", fmt.Sprintf("/project/%d/templates", f.project.ID), "", 401)
}

func TestProjectTokenIsolationAndScopes(t *testing.T) {
	f := newCoreFixture(t)
	allowed := tokenTestTemplate(t, f, f.project.ID, "allowed")
	other := tokenTestTemplate(t, f, f.project.ID, "hidden")
	foreign := tokenTestTemplate(t, f, f.other.ID, "foreign")
	body := fmt.Sprintf(`{"name":"ci","scopes":["templates:read","tasks:read"],"template_ids":[%d]}`, allowed.ID)
	issued := issueProjectToken(t, f, body)
	p := fmt.Sprintf("/project/%d", f.project.ID)
	w := f.request(t, "ci", "GET", p+"/templates", "", 200)
	assert.Contains(t, w.Body.String(), "allowed")
	assert.NotContains(t, w.Body.String(), "hidden")
	assert.NotContains(t, w.Body.String(), "repository_id")
	f.request(t, "ci", "GET", fmt.Sprintf("%s/templates/%d", p, other.ID), "", 403)
	f.request(t, "ci", "GET", fmt.Sprintf("/project/%d/templates/%d", f.other.ID, foreign.ID), "", 403)
	for _, endpoint := range []string{"/projects", "/users", "/user", "/user/tokens", "/settings", "/events", "/tasks", "/ws", p, p + "/keys", p + "/environment", p + "/backup", p + "/alerts/telegram"} {
		f.request(t, "ci", "GET", endpoint, "", 403)
	}
	task, err := f.store.CreateTask(db.Task{ProjectID: f.project.ID, TemplateID: allowed.ID, Environment: `{"private":"hidden value"}`, Status: task_logger.TaskSuccessStatus}, 0)
	require.NoError(t, err)
	_, err = f.store.CreateTask(db.Task{ProjectID: f.project.ID, TemplateID: other.ID}, 0)
	require.NoError(t, err)
	w = f.request(t, "ci", "GET", p+"/tasks", "", 200)
	var items []map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &items))
	require.Len(t, items, 1)
	assert.Equal(t, float64(task.ID), items[0]["id"])
	w = f.request(t, "ci", "GET", fmt.Sprintf("%s/tasks/%d", p, task.ID), "", 200)
	assert.NotContains(t, w.Body.String(), "hidden value")
	f.request(t, "ci", "GET", fmt.Sprintf("%s/tasks/%d/output", p, task.ID), "", 403)
	f.request(t, "ci", "POST", p+"/tasks", fmt.Sprintf(`{"template_id":%d}`, allowed.ID), 403)
	f.tokens["wrongcase"] = strings.ToLower(issued.Token)
	if f.tokens["wrongcase"] != issued.Token {
		f.request(t, "wrongcase", "GET", p+"/templates", "", 401)
	}
}

func TestProjectTokenRunOverridesAndOwnTasks(t *testing.T) {
	f := newCoreFixture(t)
	tpl := tokenTestTemplate(t, f, f.project.ID, "run")
	other := tokenTestTemplate(t, f, f.project.ID, "other")
	issued := issueProjectToken(t, f, fmt.Sprintf(`{"name":"ci","scopes":["tasks:run","tasks:stop","tasks:logs"],"template_ids":[%d]}`, tpl.ID))
	p := fmt.Sprintf("/project/%d", f.project.ID)
	for _, suffix := range []string{`,"user_id":1`, `,"project_token_id":"spoof"`, `,"commit_hash":"abc"`, `,"schedule_id":1`, `,"workflow_run_id":1`} {
		f.request(t, "ci", "POST", p+"/tasks", fmt.Sprintf(`{"template_id":%d%s}`, tpl.ID, suffix), 400)
	}
	for _, suffix := range []string{`,"git_branch":"evil"`, `,"playbook":"evil.sh"`, `,"arguments":"[]"`, `,"inventory_id":1`, `,"params":{"destroy":true}`} {
		f.request(t, "ci", "POST", p+"/tasks", fmt.Sprintf(`{"template_id":%d%s}`, tpl.ID, suffix), 403)
	}
	f.request(t, "ci", "POST", p+"/tasks", fmt.Sprintf(`{"template_id":%d,"environment":"{\"UNDECLARED\":\"value\"}"}`, tpl.ID), 400)
	f.request(t, "ci", "POST", p+"/tasks", fmt.Sprintf(`{"template_id":%d}`, other.ID), 403)
	task, err := f.store.CreateTask(db.Task{ProjectID: f.project.ID, TemplateID: tpl.ID}, 0)
	require.NoError(t, err)
	f.request(t, "ci", "POST", fmt.Sprintf("%s/tasks/%d/stop", p, task.ID), `{}`, 403)
	f.request(t, "ci", "POST", fmt.Sprintf("%s/tasks/%d/confirm", p, task.ID), `{}`, 403)
	f.request(t, "ci", "GET", fmt.Sprintf("%s/tasks/%d/output", p, task.ID), "", 200)
	assert.NotEmpty(t, issued.ID)
}

func TestProjectTokenRotationExpiryAndValidation(t *testing.T) {
	f := newCoreFixture(t)
	base := fmt.Sprintf("/project/%d/tokens", f.project.ID)
	body := `{"name":"ci","scopes":["templates:read"],"all_templates":true}`
	created := issueProjectToken(t, f, body)
	w := f.request(t, "owner", "POST", base+"/"+created.ID+"/rotate", body, 201)
	var rotated issuedProjectToken
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &rotated))
	f.tokens["rotated"] = rotated.Token
	p := fmt.Sprintf("/project/%d/templates", f.project.ID)
	f.request(t, "ci", "GET", p, "", 401)
	f.request(t, "rotated", "GET", p, "", 200)
	f.request(t, "owner", "POST", base+"/"+created.ID+"/rotate", body, 404)
	f.request(t, "admin", "POST", fmt.Sprintf("/project/%d/tokens/%s/rotate", f.other.ID, rotated.ID), body, 404)
	for _, body := range []string{
		`{"name":"ci","scopes":["admin"],"all_templates":true}`,
		`{"name":"ci","scopes":["tasks:run"]}`,
		`{"name":"ci","scopes":["templates:read"],"all_templates":true,"overrides":["environment"]}`,
		`{"name":"ci","scopes":["tasks:run"],"all_templates":true,"overrides":["user_id"]}`,
		`{"name":"ci","scopes":["tasks:run"],"all_templates":true,"expires_at":"2000-01-01T00:00:00Z"}`,
	} {
		f.request(t, "owner", "POST", base, body, 400)
	}
	// Existing expired records must not authenticate, regardless of creator.
	expired := db.ProjectToken{ProjectID: f.project.ID, Name: "expired", Scopes: []string{db.TokenReadTemplates}, AllTemplates: true, ExpiresAt: new(time.Now().Add(-time.Hour))}
	secret, err := expired.IssueSecret()
	require.NoError(t, err)
	_, err = f.store.CreateProjectToken(expired, "")
	require.NoError(t, err)
	f.tokens["expired"] = secret
	f.request(t, "expired", "GET", p, "", http.StatusUnauthorized)
}

func TestProjectTokenSurveyValidation(t *testing.T) {
	tpl := db.Template{SurveyVars: []db.SurveyVar{
		{Name: "count", Type: db.SurveyVarInt, Required: true},
		{Name: "region", Type: db.SurveyVarEnum, Values: []db.SurveyVarEnumValue{{Value: "local"}}},
		{Name: "services", Type: db.SurveyVarSelect, Values: []db.SurveyVarEnumValue{{Value: "api"}}},
		{Name: "password", Type: db.SurveyVarStr},
	}}
	require.NoError(t, validateTokenSurvey(tpl, `{"count":2,"region":"local","services":["api"]}`, `{"password":"example"}`))
	for _, values := range []string{
		`{}`, `null`, `[]`, `{"count":1.5}`, `{"count":"2"}`, `{"count":2,"region":"foreign"}`,
		`{"count":2,"services":["unknown"]}`, `{"count":2,"undeclared":"value"}`,
	} {
		assert.Error(t, validateTokenSurvey(tpl, values, ""), values)
	}
	assert.Error(t, validateTokenSurvey(tpl, `{"count":2,"password":"public"}`, `{"password":"private"}`))
}

func TestProjectTokenSurveyDefaultsAndSecrets(t *testing.T) {
	tpl := db.Template{SurveyVars: []db.SurveyVar{
		{Name: "count", Type: db.SurveyVarInt, Required: true, DefaultValue: &db.SurveyVarDefaultValue{Values: []string{"2"}}},
		{Name: "password", Type: "secret", DefaultValue: &db.SurveyVarDefaultValue{Values: []string{"saved-secret"}}},
		{Name: "services", Type: db.SurveyVarSelect, Required: true, Values: []db.SurveyVarEnumValue{{Value: "api"}}, DefaultValue: &db.SurveyVarDefaultValue{Values: []string{"api"}}},
	}}
	values, secrets, err := projectTokenSurveyDefaults(tpl, "", "")
	require.NoError(t, err)
	assert.JSONEq(t, `{"count":2,"services":["api"]}`, values)
	assert.JSONEq(t, `{"password":"saved-secret"}`, secrets)
	require.NoError(t, validateTokenSurvey(tpl, values, secrets))
	values, secrets, err = projectTokenSurveyDefaults(tpl, `{"count":3,"password":"replacement","services":[]}`, "")
	require.NoError(t, err)
	assert.NotContains(t, values, "replacement")
	assert.JSONEq(t, `{"password":"replacement"}`, secrets)
	assert.Error(t, validateTokenSurvey(tpl, values, secrets), "an explicit empty required value must not fall back to its default")
	_, _, err = projectTokenSurveyDefaults(tpl, `{"password":"one"}`, `{"password":"two"}`)
	assert.Error(t, err)
}
