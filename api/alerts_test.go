package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/impishMD/taskexec/api/helpers"
	"github.com/impishMD/taskexec/db"
	"github.com/impishMD/taskexec/db/sql"
	"github.com/impishMD/taskexec/util"
	"github.com/stretchr/testify/require"
)

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
	for _, method := range []string{"GET", "PUT", "HEAD"} {
		require.Equal(t, http.StatusForbidden, request(false, db.CanUpdateProject, 0, method, `{}`).Code)
		for _, permission := range []db.ProjectUserPermission{0, db.CanRunProjectTasks, db.CanManageProjectResources} {
			require.Equal(t, http.StatusForbidden, request(false, permission, project.ID, method, `{}`).Code)
		}
	}
	w := request(true, 0, 0, "PUT", `{"token":"123:global-secret"}`)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	require.NotContains(t, w.Body.String(), "global-secret")
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
	}
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
