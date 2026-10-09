package api

import (
	"net/http"

	"github.com/impishMD/taskexec/api/helpers"
	"github.com/impishMD/taskexec/db"
	"github.com/impishMD/taskexec/pkg/features"
	"github.com/impishMD/taskexec/services/server"
	"github.com/impishMD/taskexec/util"
	log "github.com/sirupsen/logrus"
)

type SystemInfoController struct{}

type SystemInfo struct {
	Version           string            `json:"version"`
	Ansible           string            `json:"ansible"`
	WebHost           string            `json:"web_host"`
	UseRemoteRunner   bool              `json:"use_remote_runner"`
	AuthMethods       LoginAuthMethods  `json:"auth_methods"`
	LoginWithPassword bool              `json:"login_with_password"`
	Features          features.Features `json:"features"`

	GitClient        string            `json:"git_client"`
	ScheduleTimezone string            `json:"schedule_timezone"`
	Teams            *util.TeamsConfig `json:"teams"`
	Roles            []db.Role         `json:"roles"`
	BoltdbUsed       bool              `json:"boltdb_used"`
	JWT              SystemInfoJWT     `json:"jwt"`
}

// SystemInfoJWT exposes the global JWT configuration for the WebUI.
type SystemInfoJWT struct {
	Enabled bool   `json:"enabled"`
	MaxTTL  string `json:"max_ttl,omitempty"`
}

func NewSystemInfoController() *SystemInfoController { return &SystemInfoController{} }

func (c *SystemInfoController) GetSystemInfo(w http.ResponseWriter, r *http.Request) {
	user := helpers.GetFromContext(r, "user").(*db.User)

	var authMethods LoginAuthMethods

	if util.Config.Mfa.Totp.Enabled {
		authMethods.Totp = &LoginTotpAuthMethod{
			AllowRecovery: util.Config.Mfa.Totp.AllowRecovery,
		}
	}

	timezone := util.Config.Schedule.Timezone

	if timezone == "" {
		timezone = "UTC"
	}

	roles, err := helpers.Store(r).GetGlobalRoles()
	if err != nil {
		log.WithFields(log.Fields{
			"context": "system_info",
			"user_id": user.ID,
		}).WithError(err).Error("Failed to get roles")
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	settings, err := server.GetServerSettings(helpers.Store(r))
	if err != nil {
		writeServerSettingsError(w, err)
		return
	}
	body := SystemInfo{
		Version:           util.Version(),
		Ansible:           util.AnsibleVersion(),
		WebHost:           util.Config.WebHost,
		UseRemoteRunner:   settings.UseRemoteRunner,
		AuthMethods:       authMethods,
		LoginWithPassword: !util.Config.PasswordLoginDisable,
		Features:          features.Available(),
		GitClient:         util.Config.GitClientId,
		ScheduleTimezone:  timezone,
		Teams:             util.Config.Teams,
		Roles:             roles,
		BoltdbUsed:        util.Config.Dialect == "bolt",
		JWT: SystemInfoJWT{
			Enabled: util.Config.JWT.Enabled,
			MaxTTL:  util.Config.JWT.MaxTTL,
		},
	}

	helpers.WriteJSON(w, http.StatusOK, body)
}
