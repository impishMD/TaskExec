package projects

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/gorilla/mux"
	"github.com/impishMD/taskexec/api/helpers"
	"github.com/impishMD/taskexec/db"
	"github.com/impishMD/taskexec/services/audit"
	"github.com/impishMD/taskexec/services/server"
	"github.com/impishMD/taskexec/util"
	log "github.com/sirupsen/logrus"
)

// ProjectMiddleware ensures a project exists and loads it to the context
func ProjectMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user := helpers.GetFromContext(r, "user").(*db.User)

		projectID, ok := helpers.GetIntParamOrAbort("project_id", w, r)

		if !ok {
			return
		}

		// check if user in project's team
		projectUser, err := helpers.Store(r).GetProjectUser(projectID, user.ID)

		if !user.Admin && err != nil {
			helpers.WriteError(w, err)
			return
		}

		project, err := helpers.Store(r).GetProject(projectID)

		if err != nil {
			helpers.WriteError(w, err)
			return
		}

		roleSlug := projectUser.Role

		permissions := roleSlug.GetPermissions()

		// Built-in roles are defined in code and are the source of truth for their
		// permissions. Only custom roles are resolved from the database, otherwise a
		// project role sharing a built-in slug (e.g. "manager") could override the
		// built-in permissions and escalate privileges.
		if !roleSlug.IsValid() {
			role, err := helpers.Store(r).GetProjectOrGlobalRoleBySlug(projectID, string(projectUser.Role))

			if err == nil {
				roleSlug = db.ProjectUserRole(role.Slug)
				permissions = role.Permissions
			} else if !errors.Is(err, db.ErrNotFound) {
				helpers.WriteError(w, err)
				return
			}
		}

		if helpers.HasParam("template_id", r) {
			templateID, templateOk := helpers.GetIntParamOrAbort("template_id", w, r)
			if !templateOk {
				return
			}
			var perm db.ProjectUserPermission
			perm, err = helpers.Store(r).GetTemplatePermission(project.ID, templateID, user.ID)
			if err != nil {
				helpers.WriteError(w, err)
				return
			}

			permissions |= perm
		}

		r = helpers.SetContextValue(r, "projectUserRole", roleSlug)
		r = helpers.SetContextValue(r, "permissions", permissions)
		r = helpers.SetContextValue(r, "project", project)
		next.ServeHTTP(w, r)
	})
}

// GetMustCanMiddleware ensures that the user has administrator rights
func GetMustCanMiddleware(permissions db.ProjectUserPermission) mux.MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			me := helpers.GetFromContext(r, "user").(*db.User)

			userPerms := helpers.GetFromContext(r, "permissions").(db.ProjectUserPermission)

			can := (userPerms & permissions) == permissions

			if !me.Admin && r.Method != "GET" && r.Method != "HEAD" && !can {
				projectID := 0
				if project, ok := helpers.GetOkFromContext(r, "project"); ok {
					projectID = project.(db.Project).ID
				}
				// Every call site passes a single permission bit.
				helpers.RecordDenied(r, strings.Join(audit.PermissionNames(permissions), ","), projectID)
				w.WriteHeader(http.StatusForbidden)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

type ProjectController struct {
	ProjectService server.ProjectService
}

func (c *ProjectController) UpdateProject(w http.ResponseWriter, r *http.Request) {
	project := helpers.GetFromContext(r, "project").(db.Project)
	var body struct {
		db.Project
		Icon json.RawMessage `json:"icon"`
	}

	if !helpers.Bind(w, r, &body) {
		return
	}

	if body.ID != project.ID {
		helpers.WriteJSON(w, http.StatusBadRequest, map[string]string{
			"error": "Project ID in body and URL must be the same",
		})
		return
	}

	if len(body.Icon) == 0 {
		body.Project.Icon = project.Icon
	} else if err := json.Unmarshal(body.Icon, &body.Project.Icon); err != nil {
		helpers.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid project icon"})
		return
	}
	if err := body.Project.ValidateIcon(); err != nil {
		helpers.WriteError(w, err)
		return
	}
	err := c.ProjectService.UpdateProject(body.Project)

	if err != nil {
		helpers.WriteError(w, err)
		return
	}

	helpers.Audit(r).Record(r.Context(), audit.Event{
		Kind:      audit.ResourceProjectUpdate,
		Target:    audit.ResourceTarget(audit.TargetProject, project.ID, body.Name),
		ProjectID: project.ID,
	})

	w.WriteHeader(http.StatusNoContent)
}

// DeleteProject removes a project from the database
func (c *ProjectController) DeleteProject(w http.ResponseWriter, r *http.Request) {
	project := helpers.GetFromContext(r, "project").(db.Project)

	err := c.ProjectService.DeleteProject(project.ID)

	if err != nil {
		helpers.WriteError(w, err)
		return
	}

	helpers.Audit(r).Record(r.Context(), audit.Event{
		Kind:      audit.ResourceProjectDelete,
		Target:    audit.ResourceTarget(audit.TargetProject, project.ID, project.Name),
		ProjectID: project.ID,
	})

	err = util.Config.ClearProjectTmpDir(project.ID)
	if err != nil {
		log.Error(err)
	}

	w.WriteHeader(http.StatusNoContent)
}

// GetProject returns a project details
func GetProject(w http.ResponseWriter, r *http.Request) {
	helpers.WriteJSON(w, http.StatusOK, helpers.GetFromContext(r, "project"))
}

func GetUserRole(w http.ResponseWriter, r *http.Request) {
	var result struct {
		Role        db.ProjectUserRole       `json:"role"`
		Permissions db.ProjectUserPermission `json:"permissions"`
	}
	result.Role = helpers.GetFromContext(r, "projectUserRole").(db.ProjectUserRole)
	result.Permissions = helpers.GetFromContext(r, "permissions").(db.ProjectUserPermission)
	helpers.WriteJSON(w, http.StatusOK, result)
}

func ClearCache(w http.ResponseWriter, r *http.Request) {
	project := helpers.GetFromContext(r, "project").(db.Project)

	err := util.Config.ClearProjectTmpDir(project.ID)
	if err != nil {
		helpers.WriteError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
