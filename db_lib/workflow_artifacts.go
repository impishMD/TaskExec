package db_lib

import (
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/impishMD/taskexec/db"
)

//go:embed ansible/taskexec_workflow.py
var workflowCallback []byte

// The callback is additional to the configured stdout callback. Its directory
// is task-specific and removed after Ansible exits, including error exits.
func (t *AnsibleApp) runWithWorkflowArtifacts(args LocalAppRunningArgs) error {
	dir, err := os.MkdirTemp("", "taskexec-workflow-callback-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	if err = os.WriteFile(filepath.Join(dir, "taskexec_workflow.py"), workflowCallback, 0600); err != nil {
		return err
	}
	file := filepath.Join(dir, "artifacts.json")
	callbackPath := dir
	for _, entry := range args.EnvironmentVars {
		if value, ok := strings.CutPrefix(entry, "ANSIBLE_CALLBACK_PLUGINS="); ok && value != "" {
			callbackPath += string(os.PathListSeparator) + value
		}
	}
	args.EnvironmentVars = append(args.EnvironmentVars, "ANSIBLE_CALLBACK_PLUGINS="+callbackPath, "TASKEXEC_WORKFLOW_ARTIFACTS_FILE="+file)
	runErr := t.Playbook.RunPlaybook(args.CliArgs["default"], args.EnvironmentVars, args.Inputs, args.StopCh)
	f, err := os.Open(file)
	if errors.Is(err, os.ErrNotExist) {
		return runErr
	}
	if err != nil {
		return errors.Join(runErr, err)
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, db.MaxWorkflowArtifactsBytes+1))
	if err != nil {
		return errors.Join(runErr, err)
	}
	var values map[string]any
	if len(data) > db.MaxWorkflowArtifactsBytes || json.Unmarshal(data, &values) != nil || values == nil {
		return errors.Join(runErr, fmt.Errorf("workflow artifacts must be a JSON object up to %d bytes", db.MaxWorkflowArtifactsBytes))
	}
	compact, err := json.Marshal(values)
	if err != nil {
		return errors.Join(runErr, err)
	}
	// Travels through the same ordered log channel for local and remote tasks.
	// Public set_stats values are visible to project members, just like task logs.
	t.Logger.Log(db.WorkflowArtifactsMarker + string(compact))
	return runErr
}
