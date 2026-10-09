package api

import (
	"net/http"

	"github.com/impishMD/taskexec/api/helpers"
	"github.com/impishMD/taskexec/db"
	"github.com/impishMD/taskexec/services/alerting"
	"github.com/impishMD/taskexec/services/audit"
)

// Project settings contain credentials, so reads also require permission to
// update the project. Tokens are write-only even for administrators.
func alertSettings(w http.ResponseWriter, r *http.Request) {
	user := helpers.GetFromContext(r, "user").(*db.User)
	projectID := 0
	if value, ok := helpers.GetOkFromContext(r, "project"); ok {
		projectID = value.(db.Project).ID
		permissions := helpers.GetFromContext(r, "permissions").(db.ProjectUserPermission)
		if !user.Admin && permissions&db.CanUpdateProject == 0 {
			helpers.RecordDenied(r, "update_project", projectID)
			w.WriteHeader(http.StatusForbidden)
			return
		}
	} else if !user.Admin {
		w.WriteHeader(http.StatusForbidden)
		return
	}
	service := alerting.Service{Store: helpers.Store(r)}
	if r.Method == http.MethodGet || r.Method == http.MethodHead {
		settings, err := service.GetTelegram(projectID)
		if err != nil {
			helpers.WriteError(w, err)
			return
		}
		helpers.WriteJSON(w, http.StatusOK, settings)
		return
	}
	var update alerting.TelegramUpdate
	if !helpers.Bind(w, r, &update) {
		return
	}
	settings, err := service.UpdateTelegram(projectID, update)
	if err != nil {
		helpers.WriteError(w, err)
		return
	}
	helpers.Audit(r).Record(r.Context(), audit.Event{
		Kind:      audit.SystemSettingsUpdate,
		ProjectID: projectID,
		Metadata:  audit.SettingsMetadata{Keys: []string{"alerts.telegram"}},
	})
	helpers.WriteJSON(w, http.StatusOK, settings)
}
