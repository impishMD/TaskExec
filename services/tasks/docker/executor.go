package docker

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/containerd/errdefs"
	"github.com/distribution/reference"
	"github.com/impishMD/taskexec/pkg/task_logger"
	"github.com/impishMD/taskexec/services/tasks/containerworker"
	"github.com/moby/moby/api/pkg/stdcopy"
	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/client"
)

type Executor struct {
	provider        *Provider
	request         containerworker.Request
	image, name, id string
	ctx             context.Context
	cancel          context.CancelFunc
	logger          task_logger.Logger
	prepareMu       sync.Mutex
	prepared        bool
	prepareErr      error
	createAttempted bool
	connMu          sync.Mutex
	conn            net.Conn
	killed          atomic.Bool
	cleanup         sync.Once
}

func (e *Executor) Async() bool                    { return false }
func (e *Executor) IsKilled() bool                 { return e.killed.Load() }
func (e *Executor) SetLogger(l task_logger.Logger) { e.logger = l }
func (e *Executor) Kill()                          { e.killed.Store(true); e.cancel() }
func (e *Executor) SetStatus(s task_logger.TaskStatus) {
	// Status events from the worker are already published by the runner. Only
	// explicit user decisions travel back, avoiding a status echo loop.
	if s == task_logger.TaskStoppingStatus {
		e.Kill()
		return
	}
	if s != task_logger.TaskConfirmed && s != task_logger.TaskRejected {
		return
	}
	if err := e.send(containerworker.Control{Status: s}); err != nil {
		e.logger.Log("Failed to send task confirmation to the container")
		e.cancel()
	}
}
func (e *Executor) send(v any) error {
	e.connMu.Lock()
	defer e.connMu.Unlock()
	if e.conn == nil {
		return errors.New("container channel is not connected")
	}
	_ = e.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
	err := json.NewEncoder(e.conn).Encode(v)
	_ = e.conn.SetWriteDeadline(time.Time{})
	return err
}
func (e *Executor) Prepare(username string, version *string, alias string) error {
	e.prepareMu.Lock()
	defer e.prepareMu.Unlock()
	if e.prepared {
		return e.prepareErr
	}
	e.prepared = true
	e.request.Username = username
	e.request.IncomingVersion = version
	e.request.Alias = alias
	e.prepareErr = e.prepare()
	return e.prepareErr
}
func (e *Executor) prepare() error {
	if err := e.ctx.Err(); err != nil {
		return err
	}
	if _, err := reference.ParseNormalizedNamed(e.image); err != nil {
		return fmt.Errorf("invalid executor image: %w", err)
	}
	cli, cfg := e.provider.client, e.provider.config
	_, err := cli.ImageInspect(e.ctx, e.image)
	if err != nil && !errdefs.IsNotFound(err) {
		return fmt.Errorf("inspect Docker image: %w", err)
	}
	if cfg.PullPolicy == "always" || (errdefs.IsNotFound(err) && cfg.PullPolicy == "if-not-present") {
		e.logger.Logf("Pulling task container image %s", e.image)
		stream, pullErr := cli.ImagePull(e.ctx, e.image, client.ImagePullOptions{})
		if pullErr != nil {
			return fmt.Errorf("pull task image: %w", pullErr)
		}
		defer stream.Close()
		if pullErr = stream.Wait(e.ctx); pullErr != nil {
			return fmt.Errorf("pull task image: %w", pullErr)
		}
	} else if err != nil {
		return fmt.Errorf("task image is not available locally (pull_policy=never): %w", err)
	}
	init := true
	e.createAttempted = true
	response, err := cli.ContainerCreate(e.ctx, client.ContainerCreateOptions{Name: e.name, Config: &container.Config{
		Image: e.image, Entrypoint: []string{"/usr/local/bin/taskexec"}, Cmd: []string{"task-worker"},
		OpenStdin: true, StdinOnce: true, AttachStdin: true, AttachStdout: true, AttachStderr: true,
		Labels: map[string]string{"io.taskexec.executor": "docker", "io.taskexec.task": strconv.Itoa(e.request.Task.ID), "io.taskexec.project": strconv.Itoa(e.request.Task.ProjectID)},
	}, HostConfig: &container.HostConfig{
		Init: &init, AutoRemove: true, NetworkMode: container.NetworkMode(cfg.Network), Privileged: cfg.Privileged,
		Resources: container.Resources{NanoCPUs: int64(cfg.CPULimit * 1e9), Memory: e.provider.memory},
	}})
	if err != nil {
		return fmt.Errorf("create task container: %w", err)
	}
	e.id = response.ID
	return e.ctx.Err()
}
func (e *Executor) Run(username string, version *string, alias string) (err error) {
	defer func() {
		if e.IsKilled() {
			e.logger.SetStatus(task_logger.TaskStoppedStatus)
			err = nil
		} else if err != nil {
			e.logger.Logf("Docker executor: %v", err)
		}
	}()
	defer e.Cleanup()
	if err = e.Prepare(username, version, alias); err != nil {
		return err
	}
	attach, err := e.provider.client.ContainerAttach(e.ctx, e.id, client.ContainerAttachOptions{Stream: true, Stdin: true, Stdout: true, Stderr: true})
	if err != nil {
		return fmt.Errorf("attach task container: %w", err)
	}
	defer attach.Close()
	e.connMu.Lock()
	e.conn = attach.Conn
	e.connMu.Unlock()
	defer func() { e.connMu.Lock(); e.conn = nil; e.connMu.Unlock() }()
	watchDone := make(chan struct{})
	defer close(watchDone)
	go func() {
		select {
		case <-e.ctx.Done():
			attach.Close()
		case <-watchDone:
		}
	}()
	if _, err = e.provider.client.ContainerStart(e.ctx, e.id, client.ContainerStartOptions{}); err != nil {
		return fmt.Errorf("start task container: %w", err)
	}
	out, outWriter := io.Pipe()
	defer out.Close()
	stderr, stderrWriter := io.Pipe()
	defer stderr.Close()
	copyDone := make(chan struct{})
	go func() {
		defer close(copyDone)
		_, copyErr := stdcopy.StdCopy(outWriter, stderrWriter, attach.Reader)
		_ = outWriter.CloseWithError(copyErr)
		_ = stderrWriter.CloseWithError(copyErr)
	}()
	stderrDone := make(chan struct{})
	go func() {
		defer close(stderrDone)
		defer stderr.Close()
		s := bufio.NewScanner(stderr)
		s.Buffer(make([]byte, 4096), 10*1024*1024)
		for s.Scan() {
			e.logger.Log(s.Text())
		}
		if err := s.Err(); err != nil {
			e.logger.Logf("Failed to read container stderr: %v", err)
			e.cancel()
		}
	}()
	// Closing every pipe and attachment before waiting also covers invalid output
	// from an incompatible image and a cancelled image/task startup.
	defer func() { attach.Close(); _ = out.Close(); _ = stderr.Close(); <-copyDone; <-stderrDone }()
	if err = e.send(e.request); err != nil {
		return fmt.Errorf("send container task: %w", err)
	}
	e.logger.Logf("Executing task in Docker image %s", e.image)
	scanner := bufio.NewScanner(out)
	scanner.Buffer(make([]byte, 64*1024), 11*1024*1024)
	resultReceived := false
	var resultError string
	for scanner.Scan() {
		var event containerworker.Event
		if json.Unmarshal(scanner.Bytes(), &event) != nil {
			return errors.New("invalid container worker output; use a compatible TaskExec job image")
		}
		switch event.Type {
		case "log":
			e.logger.LogWithTime(event.Time, event.Message)
		case "status":
			if event.Status.IsValid() && !event.Status.IsFinished() {
				e.logger.SetStatus(event.Status)
			}
		case "commit":
			e.logger.SetCommit(event.CommitHash, event.CommitMessage)
		case "result":
			resultReceived = true
			resultError = event.Error
		default:
			return errors.New("unknown container worker event")
		}
	}
	if err = scanner.Err(); err != nil {
		return fmt.Errorf("read container output: %w", err)
	}
	if !resultReceived {
		return errors.New("container exited without a task result; check image compatibility and memory limit")
	}
	if resultError != "" {
		return errors.New(resultError)
	}
	return nil
}
func (e *Executor) Cleanup() {
	e.cleanup.Do(func() {
		e.cancel()
		e.prepareMu.Lock()
		defer e.prepareMu.Unlock()
		if !e.createAttempted {
			return
		}
		ctx, cancel := context.WithTimeout(context.Background(), time.Duration(e.provider.config.CleanupGraceSeconds+15)*time.Second)
		defer cancel()
		target := e.id
		if target == "" {
			target = e.name
		}
		if e.IsKilled() {
			_, _ = e.provider.client.ContainerStop(ctx, target, client.ContainerStopOptions{Timeout: &e.provider.config.CleanupGraceSeconds})
		}
		for {
			_, err := e.provider.client.ContainerRemove(ctx, target, client.ContainerRemoveOptions{Force: true, RemoveVolumes: true})
			if errdefs.IsNotFound(err) {
				return
			}
			if err != nil && !errdefs.IsConflict(err) {
				e.logger.Logf("Failed to remove task container %s: %v", target, err)
				return
			}
			// AutoRemove may already be removing it; both operations are
			// asynchronous. Do not report cleanup complete until it is gone.
			_, err = e.provider.client.ContainerInspect(ctx, target, client.ContainerInspectOptions{})
			if errdefs.IsNotFound(err) {
				return
			}
			select {
			case <-ctx.Done():
				e.logger.Logf("Timed out removing task container %s", target)
				return
			case <-time.After(100 * time.Millisecond):
			}
		}
	})
}
