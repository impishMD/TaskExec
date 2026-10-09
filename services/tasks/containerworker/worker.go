package containerworker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"time"

	"github.com/impishMD/taskexec/pkg/ssh"
	"github.com/impishMD/taskexec/pkg/task_logger"
	"github.com/impishMD/taskexec/services/tasks"
	"github.com/impishMD/taskexec/util"
)

// Run consumes exactly one task, then accepts confirmation/stop commands until
// it finishes. Losing the runner connection cancels the task as well.
func Run(ctx context.Context, input io.ReadCloser, output io.Writer) error {
	defer input.Close()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	go func() { <-ctx.Done(); _ = input.Close() }()
	decoder := json.NewDecoder(input)
	var req Request
	if err := decoder.Decode(&req); err != nil {
		return errors.New("cannot read container task request")
	}
	if req.Version != ProtocolVersion {
		return fmt.Errorf("unsupported container task protocol %d", req.Version)
	}
	req.Hydrate()
	// The outer provider has already selected and started this image.
	req.Template.ExecutorImage = nil
	// A worker has no server/runner credentials or database. Tool paths and SSH
	// policy come from its own image; only explicitly forwarded task env is merged.
	util.ConfigInit("", true)
	dir, err := os.MkdirTemp("", "taskexec-worker-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	util.Config.TmpPath = dir
	util.Config.Dirs.Secrets = dir
	if util.Config.EnvVars == nil {
		util.Config.EnvVars = map[string]string{}
	}
	for k, v := range req.EnvVars {
		util.Config.EnvVars[k] = v
	}
	util.Config.WebHost = req.WebRoot
	if req.WebRoot != "" {
		util.WebHostURL, err = url.Parse(req.WebRoot)
		if err != nil {
			return errors.New("invalid worker web root")
		}
	}
	executor, err := tasks.NewLocalExecutorProvider(&ssh.KeyInstaller{}).NewExecutor(req.Task, req.Template, req.Inventory, req.Repository, req.Environment, req.JWT, req.HostConfigs)
	if err != nil {
		return err
	}
	sink := &logger{encoder: json.NewEncoder(output)}
	executor.SetLogger(sink)
	go func() {
		for {
			var control Control
			if decoder.Decode(&control) != nil {
				cancel()
				return
			}
			switch control.Status {
			case task_logger.TaskConfirmed, task_logger.TaskRejected:
				executor.SetStatus(control.Status)
			case task_logger.TaskStoppingStatus:
				cancel()
				return
			}
		}
	}()
	go func() { <-ctx.Done(); executor.Kill() }()
	result := make(chan error, 1)
	go func() { result <- executor.Run(req.Username, req.IncomingVersion, req.Alias) }()
	select {
	case err = <-result:
	case <-ctx.Done():
		executor.Kill()
		// Preparation can be inside an external git/galaxy process. The worker
		// exits even if that process ignores cancellation; Docker reaps its tree.
		select {
		case err = <-result:
		case <-time.After(2 * time.Second):
			return ctx.Err()
		}
	}
	sink.mu.Lock()
	finalStatus := sink.status
	sink.mu.Unlock()
	event := Event{Type: "result", Status: finalStatus}
	if ctx.Err() != nil {
		event.Error = "container task stopped"
	} else if err != nil {
		event.Error = err.Error()
	} else if finalStatus == task_logger.TaskFailStatus {
		event.Error = "task failed or was rejected"
	}
	sink.emit(event)
	sink.mu.Lock()
	writeErr := sink.err
	sink.mu.Unlock()
	return writeErr
}
