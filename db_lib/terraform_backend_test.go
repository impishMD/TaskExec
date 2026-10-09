package db_lib

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/impishMD/taskexec/db"
	"github.com/impishMD/taskexec/util"
	"github.com/stretchr/testify/require"
)

func TestTerraformBackendFilenameCannotEscapeCheckout(t *testing.T) {
	for _, name := range []string{"../outside.tf", "/tmp/outside.tf", "nested/backend.tf", `..\outside.tf`, "backend.tf.json"} {
		params := db.TerraformTemplateParams{BackendFilename: name}
		require.Error(t, params.ValidateBackendFilename(), name)
	}
	for _, name := range []string{"", "backend.tf", "taskexec_override.tf"} {
		params := db.TerraformTemplateParams{BackendFilename: name}
		require.NoError(t, params.ValidateBackendFilename(), name)
	}
	previous := util.Config
	util.Config = &util.ConfigType{TmpPath: t.TempDir()}
	t.Cleanup(func() { util.Config = previous })
	app := TerraformApp{Repository: db.Repository{ProjectID: 1, ID: 1}, Template: db.Template{ID: 1}}
	require.NoError(t, os.MkdirAll(app.GetFullPath(), 0700))
	outside := filepath.Join(t.TempDir(), "outside.tf")
	require.NoError(t, os.WriteFile(outside, []byte("do not overwrite"), 0600))
	link := filepath.Join(app.GetFullPath(), "backend.tf")
	require.NoError(t, os.Symlink(outside, link))
	err := app.InstallRequirements(LocalAppInstallingArgs{TplParams: &db.TerraformTemplateParams{OverrideBackend: true}, Params: &db.TerraformTaskParams{}})
	require.ErrorContains(t, err, "symbolic link")
	app.Clear()
	content, err := os.ReadFile(outside)
	require.NoError(t, err)
	require.Equal(t, "do not overwrite", string(content))
	_, err = os.Lstat(link)
	require.NoError(t, err, "a failed preparation does not delete an existing repository file")
}
