package api

import (
	"net/http"
	"runtime"

	"github.com/impishMD/taskexec/api/helpers"
	"github.com/impishMD/taskexec/services/alerting"
	"github.com/impishMD/taskexec/services/server"
	"github.com/impishMD/taskexec/util"
)

func getAdminInfo(w http.ResponseWriter, r *http.Request) {
	// Database info
	dbInfo := map[string]any{
		"dialect": util.Config.Dialect,
	}

	// Authentication
	authInfo := map[string]any{
		"password_login_enabled": !util.Config.PasswordLoginDisable,
	}

	if util.Config.Mfa != nil {
		if util.Config.Mfa.Totp != nil {
			authInfo["totp_enabled"] = util.Config.Mfa.Totp.Enabled
		} else {
			authInfo["totp_enabled"] = false
		}
		authInfo["email_otp_enabled"] = false
	}

	// LDAP
	authInfo["ldap_enabled"] = len(util.Config.ActiveLdapProviders()) > 0

	// OpenID Connect providers
	oidcProviders := []string{}
	for name := range util.Config.OidcProviders {
		oidcProviders = append(oidcProviders, name)
	}
	authInfo["oidc_providers"] = oidcProviders

	// Notifications
	telegram, err := (alerting.Service{Store: helpers.Store(r)}).GetTelegram(0)
	if err != nil {
		helpers.WriteError(w, err)
		return
	}
	notifications := map[string]bool{
		"email":           util.Config.EmailAlert,
		"telegram":        telegram.HasToken,
		"slack":           util.Config.SlackAlert,
		"rocketchat":      util.Config.RocketChatAlert,
		"microsoft_teams": util.Config.MicrosoftTeamsAlert,
		"dingtalk":        util.Config.DingTalkAlert,
		"gotify":          util.Config.GotifyAlert,
	}

	// Runners
	settings, err := server.GetServerSettings(helpers.Store(r))
	if err != nil {
		writeServerSettingsError(w, err)
		return
	}
	runnersInfo := map[string]any{
		"use_remote_runner": settings.UseRemoteRunner,
	}

	// Task settings
	taskSettings := map[string]any{
		"max_parallel_tasks":     util.Config.MaxParallelTasks,
		"max_task_duration_sec":  util.Config.MaxTaskDurationSec,
		"max_tasks_per_template": util.Config.MaxTasksPerTemplate,
	}

	// System
	systemInfo := map[string]any{
		"version":       util.Version(),
		"ansible":       util.AnsibleVersion(),
		"git_client":    util.Config.GitClientId,
		"go_version":    runtime.Version(),
		"go_arch":       runtime.GOARCH,
		"go_os":         runtime.GOOS,
		"tmp_path":      util.Config.TmpPath,
		"home_dir_mode": util.Config.HomeDirMode,
	}

	if util.Config.Schedule != nil && util.Config.Schedule.Timezone != "" {
		systemInfo["schedule_timezone"] = util.Config.Schedule.Timezone
	} else {
		systemInfo["schedule_timezone"] = "UTC"
	}

	// Feature flags
	features := map[string]any{
		"non_admin_can_create_project": util.Config.NonAdminCanCreateProject,
	}

	body := map[string]any{
		"system":        systemInfo,
		"database":      dbInfo,
		"auth":          authInfo,
		"notifications": notifications,
		"runners":       runnersInfo,
		"task_settings": taskSettings,
		"features":      features,
	}

	helpers.WriteJSON(w, http.StatusOK, body)
}
