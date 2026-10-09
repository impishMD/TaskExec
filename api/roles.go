package api

import (
	"net/http"
	"strings"

	"github.com/gorilla/mux"
	"github.com/impishMD/taskexec/api/helpers"
	"github.com/impishMD/taskexec/db"
	"github.com/impishMD/taskexec/services/audit"
)

type RolesController struct{ roleRepo db.RoleRepository }

func NewRolesController(repo db.RoleRepository) *RolesController {
	return &RolesController{roleRepo: repo}
}

func roleProject(r *http.Request) *int {
	project := helpers.GetFromContext(r, "project").(db.Project)
	return &project.ID
}

func (c *RolesController) role(r *http.Request, projectID *int) (db.Role, error) {
	slug := mux.Vars(r)["role_slug"]
	if projectID == nil {
		return c.roleRepo.GetGlobalRoleBySlug(slug)
	}
	return c.roleRepo.GetProjectRole(*projectID, slug)
}

func (c *RolesController) list(w http.ResponseWriter, r *http.Request, projectID *int, includeGlobal bool) {
	roles := make([]db.Role, 0)
	if projectID != nil {
		items, err := c.roleRepo.GetProjectRoles(*projectID)
		if err != nil {
			helpers.WriteError(w, err)
			return
		}
		roles = append(roles, items...)
	}
	if projectID == nil || includeGlobal {
		items, err := c.roleRepo.GetGlobalRoles()
		if err != nil {
			helpers.WriteError(w, err)
			return
		}
		roles = append(roles, items...)
	}
	helpers.WriteJSON(w, http.StatusOK, roles)
}

func (c *RolesController) get(w http.ResponseWriter, r *http.Request, projectID *int) {
	role, err := c.role(r, projectID)
	if err != nil {
		helpers.WriteError(w, err)
		return
	}
	helpers.WriteJSON(w, http.StatusOK, role)
}

func (c *RolesController) save(w http.ResponseWriter, r *http.Request, projectID *int, create bool) {
	var role db.Role
	if !helpers.Bind(w, r, &role) {
		return
	}
	role.ProjectID = projectID
	role.Name = strings.TrimSpace(role.Name)
	if !create {
		existing, err := c.role(r, projectID)
		if err != nil {
			helpers.WriteError(w, err)
			return
		}
		if role.Slug != existing.Slug {
			helpers.WriteErrorStatus(w, "Role slug cannot be changed", http.StatusBadRequest)
			return
		}
	}
	if err := db.ValidateRole(role); err != nil {
		helpers.WriteError(w, err)
		return
	}
	var err error
	if create {
		role, err = c.roleRepo.CreateRole(role)
	} else {
		err = c.roleRepo.UpdateRole(role)
	}
	if err != nil {
		helpers.WriteError(w, err)
		return
	}
	globalKind, projectKind := audit.IAMRoleUpdate, audit.IAMProjectRoleDefinitionUpdate
	if create {
		globalKind, projectKind = audit.IAMRoleCreate, audit.IAMProjectRoleDefinitionCreate
	}
	recordRoleChange(r, role, globalKind, projectKind, audit.RoleMetadata{Permissions: audit.PermissionNames(role.Permissions)})
	if create {
		helpers.WriteJSON(w, http.StatusCreated, role)
	} else {
		w.WriteHeader(http.StatusNoContent)
	}
}

func (c *RolesController) delete(w http.ResponseWriter, r *http.Request, projectID *int) {
	role, err := c.role(r, projectID)
	if err != nil {
		helpers.WriteError(w, err)
		return
	}
	if err = c.roleRepo.DeleteRole(role.Slug, projectID); err != nil {
		helpers.WriteError(w, err)
		return
	}
	recordRoleChange(r, role, audit.IAMRoleDelete, audit.IAMProjectRoleDefinitionDelete, nil)
	w.WriteHeader(http.StatusNoContent)
}

func recordRoleChange(r *http.Request, role db.Role, globalKind, projectKind audit.Kind, metadata any) {
	event := audit.Event{Kind: globalKind, Target: &audit.Target{Type: audit.TargetRole, ID: role.Slug, Name: role.Name}, Metadata: metadata}
	if role.ProjectID != nil {
		event.Kind, event.ProjectID, event.Target.Type = projectKind, *role.ProjectID, audit.TargetProjectRoleDefinition
	}
	helpers.Audit(r).Record(r.Context(), event)
}

func (c *RolesController) GetRoles(w http.ResponseWriter, r *http.Request)      { c.list(w, r, nil, false) }
func (c *RolesController) GetGlobalRole(w http.ResponseWriter, r *http.Request) { c.get(w, r, nil) }
func (c *RolesController) AddRole(w http.ResponseWriter, r *http.Request)       { c.save(w, r, nil, true) }
func (c *RolesController) UpdateRole(w http.ResponseWriter, r *http.Request) {
	c.save(w, r, nil, false)
}
func (c *RolesController) DeleteRole(w http.ResponseWriter, r *http.Request) { c.delete(w, r, nil) }
func (c *RolesController) GetProjectRoles(w http.ResponseWriter, r *http.Request) {
	c.list(w, r, roleProject(r), false)
}
func (c *RolesController) GetProjectAndGlobalRoles(w http.ResponseWriter, r *http.Request) {
	c.list(w, r, roleProject(r), true)
}
func (c *RolesController) GetProjectRole(w http.ResponseWriter, r *http.Request) {
	c.get(w, r, roleProject(r))
}
func (c *RolesController) AddProjectRole(w http.ResponseWriter, r *http.Request) {
	c.save(w, r, roleProject(r), true)
}
func (c *RolesController) UpdateProjectRole(w http.ResponseWriter, r *http.Request) {
	c.save(w, r, roleProject(r), false)
}
func (c *RolesController) DeleteProjectRole(w http.ResponseWriter, r *http.Request) {
	c.delete(w, r, roleProject(r))
}
