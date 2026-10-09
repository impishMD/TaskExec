package sql

import (
	"testing"

	"github.com/impishMD/taskexec/db"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// slug is the primary key of the whole role table, so a global and a project role never share a slug.
func setupProjectRole(t *testing.T) (*SqlDb, db.Project, db.Role) {
	t.Helper()
	store := InitConfigCreateTestStore()
	project, err := store.CreateProject(db.Project{Name: "proj"})
	require.NoError(t, err)
	role, err := store.CreateRole(db.Role{Slug: "ops", Name: "Ops", Permissions: db.CanRunProjectTasks, ProjectID: &project.ID})
	require.NoError(t, err)
	return store, project, role
}

func TestUpdateRole_GlobalScopeDoesNotTouchProjectRole(t *testing.T) {
	store, project, _ := setupProjectRole(t)

	err := store.UpdateRole(db.Role{Slug: "ops", Name: "Changed", Permissions: db.CanManageProjectResources})

	assert.ErrorIs(t, err, db.ErrNotFound)
	role, err := store.GetProjectRole(project.ID, "ops")
	require.NoError(t, err)
	assert.Equal(t, "Ops", role.Name)
	assert.Equal(t, db.CanRunProjectTasks, role.Permissions)
}

func TestUpdateRole_OtherProjectIsNotFound(t *testing.T) {
	store, project, _ := setupProjectRole(t)
	other := project.ID + 1

	err := store.UpdateRole(db.Role{Slug: "ops", Name: "Changed", ProjectID: &other})

	assert.ErrorIs(t, err, db.ErrNotFound)
	role, err := store.GetProjectRole(project.ID, "ops")
	require.NoError(t, err)
	assert.Equal(t, "Ops", role.Name)
}

func TestUpdateRole_SameScopeIsUpdated(t *testing.T) {
	store, project, role := setupProjectRole(t)
	role.Name = "Changed"

	require.NoError(t, store.UpdateRole(role))

	got, err := store.GetProjectRole(project.ID, "ops")
	require.NoError(t, err)
	assert.Equal(t, "Changed", got.Name)
}

// MySQL reports no affected rows for an update that changes nothing.
func TestUpdateRole_IdenticalValuesSucceed(t *testing.T) {
	store, _, role := setupProjectRole(t)
	require.NoError(t, store.UpdateRole(role))

	global, err := store.CreateRole(db.Role{Slug: "viewer", Name: "Viewer"})
	require.NoError(t, err)
	require.NoError(t, store.UpdateRole(global))
}

func TestUpdateRole_MissingIsNotFound(t *testing.T) {
	store := InitConfigCreateTestStore()
	assert.ErrorIs(t, store.UpdateRole(db.Role{Slug: "nope", Name: "Nope"}), db.ErrNotFound)
}

func TestDeleteRole_GlobalScopeDoesNotTouchProjectRole(t *testing.T) {
	store, project, _ := setupProjectRole(t)

	assert.ErrorIs(t, store.DeleteRole("ops", nil), db.ErrNotFound)

	_, err := store.GetProjectRole(project.ID, "ops")
	assert.NoError(t, err)
}

func TestDeleteRole_OtherProjectIsNotFound(t *testing.T) {
	store, project, _ := setupProjectRole(t)
	other := project.ID + 1

	assert.ErrorIs(t, store.DeleteRole("ops", &other), db.ErrNotFound)

	_, err := store.GetProjectRole(project.ID, "ops")
	assert.NoError(t, err)
}

func TestDeleteRole_SameScopeIsDeleted(t *testing.T) {
	store, project, _ := setupProjectRole(t)

	require.NoError(t, store.DeleteRole("ops", &project.ID))

	_, err := store.GetProjectRole(project.ID, "ops")
	assert.ErrorIs(t, err, db.ErrNotFound)

	_, err = store.CreateRole(db.Role{Slug: "viewer", Name: "Viewer"})
	require.NoError(t, err)
	require.NoError(t, store.DeleteRole("viewer", nil))
}

func TestDeleteRole_MissingIsNotFound(t *testing.T) {
	store := InitConfigCreateTestStore()
	assert.ErrorIs(t, store.DeleteRole("nope", nil), db.ErrNotFound)
}

func TestCustomRole_TemplatePermissionsIncludeBaseAndTemplateGrant(t *testing.T) {
	store, project, role := setupProjectRole(t)
	user, err := store.CreateUser(db.UserWithPwd{User: db.User{Username: "custom", Name: "Custom", Email: "custom@example.com"}, Pwd: "verystrongpassword1"})
	require.NoError(t, err)
	_, err = store.CreateProjectUser(db.ProjectUser{ProjectID: project.ID, UserID: user.ID, Role: db.ProjectUserRole(role.Slug)})
	require.NoError(t, err)
	key, err := store.CreateAccessKey(db.AccessKey{ProjectID: &project.ID, Type: db.AccessKeyNone})
	require.NoError(t, err)
	repo, err := store.CreateRepository(db.Repository{ProjectID: project.ID, Name: "repo", GitURL: "https://example.com/repo.git", GitBranch: "main", SSHKeyID: key.ID})
	require.NoError(t, err)
	tpl, err := store.CreateTemplate(db.Template{ProjectID: project.ID, RepositoryID: repo.ID, Name: "test", Playbook: "test.yml"})
	require.NoError(t, err)
	permissions, err := store.GetTemplatePermission(project.ID, tpl.ID, user.ID)
	require.NoError(t, err)
	assert.Equal(t, db.CanRunProjectTasks, permissions, "custom role can launch tasks without an additional template grant")
	_, err = store.CreateTemplateRole(db.TemplateRolePerm{ProjectID: project.ID, TemplateID: tpl.ID, RoleSlug: role.Slug, Permissions: db.CanManageProjectResources})
	require.NoError(t, err)
	permissions, err = store.GetTemplatePermission(project.ID, tpl.ID, user.ID)
	require.NoError(t, err)
	assert.Equal(t, db.CanRunProjectTasks|db.CanManageProjectResources, permissions)
	templates, err := store.GetTemplatesWithPermissions(project.ID, user.ID, db.TemplateFilter{}, db.RetrieveQueryParams{})
	require.NoError(t, err)
	require.Len(t, templates, 1)
	assert.Equal(t, permissions, *templates[0].Permissions, "list and detail must report the same additive permissions")
}
