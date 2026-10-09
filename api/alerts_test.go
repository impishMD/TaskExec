package api

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/impishMD/taskexec/api/helpers"
	"github.com/impishMD/taskexec/db"
	"github.com/impishMD/taskexec/db/sql"
	"github.com/impishMD/taskexec/util"
	"github.com/stretchr/testify/require"
)

type alertTestTransport func(*http.Request) (*http.Response, error)

func (f alertTestTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestAlertSettingsPermissionsAndWriteOnlySecrets(t *testing.T) {
	old := util.Config
	store := sql.InitConfigCreateTestStore()
	defer func() { store.Close(); util.Config = old }()
	project, err := store.CreateProject(db.Project{Name: "alerts"})
	require.NoError(t, err)
	request := func(admin bool, permission db.ProjectUserPermission, scope int, method, body string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, "/api/alerts/telegram", bytes.NewBufferString(body))
		r = helpers.SetContextValue(r, "store", store)
		r = helpers.SetContextValue(r, "user", &db.User{Admin: admin})
		if scope > 0 {
			r = helpers.SetContextValue(r, "project", project)
			r = helpers.SetContextValue(r, "permissions", permission)
		}
		w := httptest.NewRecorder()
		alertSettings(w, r)
		return w
	}
	for _, method := range []string{"GET", "PUT", "HEAD", "POST"} {
		require.Equal(t, http.StatusForbidden, request(false, db.CanUpdateProject, 0, method, `{}`).Code)
		for _, permission := range []db.ProjectUserPermission{0, db.CanRunProjectTasks, db.CanManageProjectResources} {
			require.Equal(t, http.StatusForbidden, request(false, permission, project.ID, method, `{}`).Code)
		}
	}
	w := request(true, 0, 0, "PUT", `{"token":"123:global-secret"}`)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	require.NotContains(t, w.Body.String(), "global-secret")
	oldTransport := http.DefaultTransport
	defer func() { http.DefaultTransport = oldTransport }()
	calls := 0
	http.DefaultTransport = alertTestTransport(func(r *http.Request) (*http.Response, error) {
		calls++
		require.Equal(t, "api.telegram.org", r.URL.Host)
		require.Equal(t, "/bot123:global-secret/sendMessage", r.URL.Path)
		var body map[string]string
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		require.Equal(t, "@preview", body["chat_id"])
		require.Contains(t, body["text"], "тестовое оповещение")
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"ok":true}`))}, nil
	})
	w = request(true, 0, 0, "POST", `{"chat_id":"@preview","locale":"ru"}`)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	require.JSONEq(t, `{"sent":true}`, w.Body.String())
	for _, admin := range []bool{false, true} {
		w = request(admin, db.CanUpdateProject, project.ID, "PUT", `{"enabled":true,"chat_id":"-100123","token":"456:project-secret"}`)
		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
		require.NotContains(t, w.Body.String(), "project-secret")
		w = request(admin, db.CanUpdateProject, project.ID, "GET", "")
		require.Equal(t, http.StatusOK, w.Code)
		var body map[string]any
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
		require.Equal(t, true, body["has_token"])
		require.Equal(t, true, body["global_token_configured"])
		require.NotContains(t, body, "token")
		require.NotContains(t, body, "secret")
		before, err := store.GetAlertChannel(project.ID, "telegram")
		require.NoError(t, err)
		w = request(admin, db.CanUpdateProject, project.ID, "POST", `{"chat_id":"@preview","token":"","locale":"ru"}`)
		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
		require.JSONEq(t, `{"sent":true}`, w.Body.String())
		after, err := store.GetAlertChannel(project.ID, "telegram")
		require.NoError(t, err)
		require.Equal(t, before, after, "testing must not save the submitted chat or remove the override")
	}
	require.Equal(t, 3, calls)
	w = request(true, 0, 0, "PUT", `{"token":"123:secret/bad"}`)
	require.Equal(t, http.StatusBadRequest, w.Code)
	require.NotContains(t, w.Body.String(), "123:secret/bad")
	// A separate project must start disabled without an inherited chat or override.
	another, err := store.CreateProject(db.Project{Name: "another"})
	require.NoError(t, err)
	project = another
	w = request(false, db.CanUpdateProject, another.ID, "GET", "")
	require.Equal(t, http.StatusOK, w.Code)
	require.Contains(t, w.Body.String(), `"enabled":false`)
	require.Contains(t, w.Body.String(), `"has_token":false`)
}
