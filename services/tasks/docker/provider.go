// Package docker implements one disposable Docker container per runner task.
package docker

import (
	"context"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"

	"github.com/distribution/reference"
	"github.com/docker/go-units"
	"github.com/google/uuid"
	"github.com/impishMD/taskexec/db"
	"github.com/impishMD/taskexec/pkg/task_logger"
	"github.com/impishMD/taskexec/services/tasks"
	"github.com/impishMD/taskexec/services/tasks/containerworker"
	"github.com/impishMD/taskexec/util"
	"github.com/moby/moby/client"
)

type Provider struct {
	client *client.Client
	config util.RunnerDockerConfig
	memory int64
}

func NewProvider(cfg util.RunnerDockerConfig) (*Provider, error) {
	if cfg.Image == "" {
		cfg.Image = "impishmd/taskexec:latest-job"
	}
	if cfg.Network == "" {
		cfg.Network = "bridge"
	}
	if cfg.PullPolicy == "" {
		cfg.PullPolicy = "if-not-present"
	}
	if cfg.CleanupGraceSeconds == 0 {
		cfg.CleanupGraceSeconds = 30
	}
	if _, err := reference.ParseNormalizedNamed(cfg.Image); err != nil {
		return nil, fmt.Errorf("invalid Docker executor image: %w", err)
	}
	if cfg.PullPolicy != "always" && cfg.PullPolicy != "if-not-present" && cfg.PullPolicy != "never" {
		return nil, fmt.Errorf("invalid Docker pull policy %q", cfg.PullPolicy)
	}
	if math.IsNaN(cfg.CPULimit) || math.IsInf(cfg.CPULimit, 0) || cfg.CPULimit < 0 || cfg.CPULimit > float64(math.MaxInt64)/1e9 {
		return nil, fmt.Errorf("invalid Docker CPU limit")
	}
	if cfg.CleanupGraceSeconds < 0 {
		return nil, fmt.Errorf("Docker cleanup grace must not be negative")
	}
	var memory int64
	if cfg.MemoryLimit != "" {
		var err error
		memory, err = units.RAMInBytes(cfg.MemoryLimit)
		if err != nil || memory < 0 {
			return nil, fmt.Errorf("invalid Docker memory limit")
		}
	}
	opts := []client.Opt{client.FromEnv, client.WithUserAgent("TaskExec/" + util.Ver)}
	if cfg.Host != "" {
		opts = append(opts, client.WithHost(cfg.Host))
	}
	if cfg.CertPath != "" {
		opts = append(opts, client.WithTLSClientConfig(filepath.Join(cfg.CertPath, "ca.pem"), filepath.Join(cfg.CertPath, "cert.pem"), filepath.Join(cfg.CertPath, "key.pem")))
	} else if cfg.TLSVerify {
		opts = append(opts, client.WithTLSClientConfig("", "", ""))
	}
	cli, err := client.New(opts...)
	if err != nil {
		return nil, err
	}
	return &Provider{client: cli, config: cfg, memory: memory}, nil
}
func (p *Provider) Close() error { return p.client.Close() }

func (p *Provider) NewExecutor(task db.Task, template db.Template, inventory db.Inventory, repository db.Repository, environment db.Environment, jwt string, hostConfigs []db.HostConfig) (tasks.Executor, error) {
	image := p.config.Image
	if override := template.NormalizedExecutorImage(); override != nil {
		image = *override
	}
	// Invalid template images fail as task errors during Prepare, rather than
	// leaving the server's assignment stuck while the runner repeatedly rejects it.
	ctx, cancel := context.WithCancel(context.Background())
	req := containerworker.Request{
		Version: containerworker.ProtocolVersion,
		Task:    task, Template: template, Inventory: inventory,
		Repository: repository, Environment: environment,
		HostConfigs: hostConfigs, JWT: jwt,
		RepositoryKey: repository.SSHKey, InventoryKey: inventory.SSHKey,
		BecomeKey: inventory.BecomeKey, InventoryRepository: inventory.Repository,
		EnvVars: map[string]string{},
	}
	if inventory.Repository != nil {
		req.InventoryRepositoryKey = inventory.Repository.SSHKey
	}
	for _, v := range template.Vaults {
		req.VaultKeys = append(req.VaultKeys, v.Vault)
	}
	for _, h := range hostConfigs {
		req.HostKeys = append(req.HostKeys, h.SSHKey)
	}
	if util.Config != nil {
		req.WebRoot = util.Config.WebHost
		for _, key := range util.Config.ForwardedEnvVars {
			if value := os.Getenv(key); value != "" {
				req.EnvVars[key] = value
			}
		}
		for key, value := range util.Config.EnvVars {
			req.EnvVars[key] = value
		}
	}
	return &Executor{
		provider: p, request: req, image: strings.TrimSpace(image),
		name: "taskexec-task-" + uuid.NewString(),
		ctx:  ctx, cancel: cancel, logger: task_logger.NopLogger{},
	}, nil
}
