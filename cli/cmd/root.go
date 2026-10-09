package cmd

import (
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/gorilla/handlers"
	"github.com/impishMD/taskexec/api"
	"github.com/impishMD/taskexec/api/helpers"
	"github.com/impishMD/taskexec/api/sockets"
	"github.com/impishMD/taskexec/db"
	"github.com/impishMD/taskexec/db/factory"
	"github.com/impishMD/taskexec/pkg/debuglog"
	"github.com/impishMD/taskexec/pkg/metrics"
	"github.com/impishMD/taskexec/services/audit"
	"github.com/impishMD/taskexec/services/schedules"
	"github.com/impishMD/taskexec/services/server"
	"github.com/impishMD/taskexec/services/tasks"
	"github.com/impishMD/taskexec/util"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var persistentFlags struct {
	configPath  string
	noConfig    bool
	logLevel    string
	debugFilter string
}

var rootCmd = &cobra.Command{
	Use:   "taskexec",
	Short: "TaskExec runs Ansible, Terraform, OpenTofu and scripts",
	Long: `TaskExec runs Ansible, Terraform, OpenTofu and scripts.
Source code is available at https://github.com/impishMD/TaskExec.
Complete documentation is available at https://github.com/impishMD/TaskExec`,
	Run: func(cmd *cobra.Command, args []string) {
		_ = cmd.Help()
		os.Exit(0)
	},

	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		str := persistentFlags.logLevel
		if str == "" {
			str = os.Getenv("TASKEXEC_LOG_LEVEL")
		}

		if str != "" {
			lvl, err := log.ParseLevel(str)
			if err != nil {
				log.Panic(err)
			}

			fmt.Println("Log level set to", lvl)
			log.SetLevel(lvl)
		}

		initDebugFilter()
	},
}

// initDebugFilter installs a Node.js-`debug`-style namespace filter for DEBUG
// logs, driven by the --debug-filter flag or TASKEXEC_DEBUG_FILTER env var.
// The filter only narrows DEBUG-level output and only takes effect when the log
// level is already DEBUG; otherwise there are no debug entries to filter and the
// logger is left untouched.
func initDebugFilter() {
	spec, filter := configuredDebugFilter()
	if filter == nil {
		return
	}

	log.SetFormatter(debuglog.NewFilteringFormatter(
		log.StandardLogger().Formatter,
		filter,
	))

	fmt.Println("Debug filter active:", spec)
}

func configuredDebugFilter() (string, *debuglog.Filter) {
	spec := persistentFlags.debugFilter
	if spec == "" {
		spec = os.Getenv("TASKEXEC_DEBUG_FILTER")
	}

	if spec == "" || log.GetLevel() < log.DebugLevel {
		return "", nil
	}

	return spec, debuglog.Parse(spec)
}

// registerPersistentFlags is guarded by a sync.Once because both Execute and
// RootCommand need the flags present, and pflag panics on a duplicate name.
var registerPersistentFlags = sync.OnceFunc(func() {
	rootCmd.PersistentFlags().StringVar(&persistentFlags.logLevel, "log-level", "", "Log level: DEBUG, INFO, WARN, ERROR, FATAL, PANIC")
	rootCmd.PersistentFlags().StringVar(&persistentFlags.debugFilter, "debug-filter", "", "Debug namespace filter (only with DEBUG level), e.g. 'runner,task_*' or '*,-db'")
	rootCmd.PersistentFlags().StringVar(&persistentFlags.configPath, "config", "", "Configuration file path")
	rootCmd.PersistentFlags().BoolVar(&persistentFlags.noConfig, "no-config", false, "Don't use configuration file")
})

func Execute() {
	registerPersistentFlags()
	if err := rootCmd.Execute(); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// watchEncryptionKeyReload enables key rotation without restarting the server:
//   - a SIGHUP forces an immediate reload;
//   - a background poller applies changes to the encryption-keys file (and the
//     key files it references) automatically. The poller runs only when a keys
//     file is configured and the poll interval is positive.
func watchEncryptionKeyReload() {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGHUP)
	go func() {
		for range sigCh {
			if err := util.ReloadEncryptionKeys(); err != nil {
				log.WithError(err).Error("failed to reload encryption keys")
			} else {
				log.Info("encryption keys reloaded (SIGHUP)")
			}
		}
	}()

	interval := util.Config.EncryptionKeysPollInterval()
	if util.Config.EncryptionKeysFile() == "" || interval <= 0 {
		return
	}

	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for range ticker.C {
			changed, err := util.ReloadEncryptionKeysIfChanged()
			if err != nil {
				log.WithError(err).Error("failed to reload encryption keys")
			} else if changed {
				log.Info("encryption keys reloaded (file changed)")
			}
		}
	}()
}

func runService() {
	store := createStore("root")
	defer store.Close()
	if err := util.Config.ValidateServerCapabilities(); err != nil {
		log.WithError(err).Fatal("unsupported server configuration")
	}

	watchEncryptionKeyReload()

	jwtSigner, jwtErr := util.InitJWTSignerFromStore(store)
	if jwtErr != nil {
		log.WithError(jwtErr).Warning("failed to initialise JWT signer")
	}

	initSyslog(util.Config.Syslog)

	// Initialize HA node identity before any component that uses it.
	util.InitHANodeID()

	appMetrics := metrics.NewMetrics()

	auditService, auditErr := audit.StartService(
		store,
		util.Config.Audit,
		util.HANodeID(),
		nil,
	)
	if auditErr != nil {
		log.WithError(auditErr).Fatal("failed to start the audit log")
	}
	defer auditService.Stop()

	state := tasks.NewMemoryTaskStateStore()
	terraformStore := store
	ansibleTaskRepo := store
	workflowStore := store

	projectService := server.NewProjectService(store, store)
	encryptionService := server.NewAccessKeyEncryptionService(store, store, store, store)
	accessKeyInstallationService := server.NewAccessKeyInstallationService(encryptionService)
	integrationService := server.NewIntegrationService(store, encryptionService)
	inventoryService := server.NewInventoryService(
		store,
		store,
		store,
		encryptionService,
	)
	accessKeyService := server.NewAccessKeyService(store, encryptionService, store, store)
	secretStorageService := server.NewSecretStorageService(store, store, accessKeyService, encryptionService)
	environmentService := server.NewEnvironmentService(store, encryptionService, store)
	runnerService := server.NewRunnerService(store)

	taskPool := tasks.CreateTaskPool(
		store,
		state,
		ansibleTaskRepo,
		inventoryService,
		encryptionService,
		accessKeyInstallationService,
		jwtSigner,
		appMetrics,
	)
	taskPool.SetAuditRecorder(auditService.Recorder())
	taskPool.SetWorkflowRepo(workflowStore)

	// The workflow service orchestrates workflow runs and launches each node's
	// task through the pool; the pool calls back into it when a workflow task
	// finishes. Wire the cycle: pool first, then service (with the pool as its
	// enqueuer), then inject the service back into the pool.
	workflowService := server.NewWorkflowService(workflowStore, store, &taskPool, auditService.Recorder())
	taskPool.SetWorkflowService(workflowService)

	schedulePool := schedules.CreateSchedulePool(
		store,
		&taskPool,
		accessKeyInstallationService,
		encryptionService,
	)

	defer schedulePool.Destroy()
	defer taskPool.Stop()

	util.Config.PrintDbInfo()

	port := util.Config.Port

	if !strings.HasPrefix(port, ":") {
		port = ":" + port
	}

	fmt.Printf("Tmp Path (projects home) %v\n", util.Config.TmpPath)
	fmt.Printf("TaskExec %v\n", util.Version())
	fmt.Printf("Interface %v\n", util.Config.Interface)
	fmt.Printf("Port %v\n", util.Config.Port)

	// Start the WebSocket hub before the broadcaster so that h.broadcast
	// channel is being consumed when LocalBroadcast is called.
	go sockets.StartWS()

	taskPool.LogRunnerStateSnapshot()
	go schedulePool.Run()
	go taskPool.Run()
	if err := taskPool.RecoverWorkflowTasks(); err != nil {
		log.WithError(err).Fatal("cannot recover interrupted workflow tasks")
	}
	// Timers and approvals progress without an open browser, including after restart.
	workflowReconciler := server.NewWorkflowReconciler(workflowStore, workflowService)
	workflowReconciler.Start()
	defer workflowReconciler.Stop()


	route := api.Route(
		store,
		terraformStore,
		workflowStore,
		ansibleTaskRepo,
		&taskPool,
		projectService,
		integrationService,
		encryptionService,
		accessKeyInstallationService,
		secretStorageService,
		accessKeyService,
		environmentService,
		jwtSigner,
		runnerService,
		workflowService,
		appMetrics,
	)

	route.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			r = helpers.SetContextValue(r, "store", store)
			r = helpers.SetContextValue(r, "schedule_pool", schedulePool)
			r = helpers.SetContextValue(r, "task_pool", &taskPool)
			r = helpers.SetContextValue(r, "audit", auditService.Recorder())

			next.ServeHTTP(w, r)
		})
	})

	var router http.Handler = route

	router = handlers.ProxyHeaders(router)
	// Outside ProxyHeaders, which trusts X-Forwarded-For blindly.
	router = auditService.Wrap(router)
	http.Handle("/", router)

	fmt.Println("Server is running")

	var err error
	if util.Config.TLS.Enabled {

		if util.Config.TLS.HTTPRedirectPort != nil && util.Config.TLS.HTTPRedirectAddr != "" {
			panic("You can't use both HTTP redirect address and port at the same time")
		}

		var httpRedirectAddr string

		if util.Config.TLS.HTTPRedirectPort != nil {
			httpRedirectAddr = fmt.Sprintf(":%d", *util.Config.TLS.HTTPRedirectPort)
		} else if util.Config.TLS.HTTPRedirectAddr != "" {
			httpRedirectAddr = util.Config.TLS.HTTPRedirectAddr
		}

		if httpRedirectAddr != "" {

			go func() {

				err = http.ListenAndServe(httpRedirectAddr, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					target := "https://"

					if util.Config.WebHost != "" {
						webHost, err2 := url.Parse(util.Config.WebHost)
						if err2 != nil {
							log.Panic(err2)
						}
						target += webHost.Host + r.URL.Path
					} else {
						hostParts := strings.Split(r.Host, ":")
						host := hostParts[0]
						target += host + port + r.URL.Path
					}

					if len(r.URL.RawQuery) > 0 {
						target += "?" + r.URL.RawQuery
					}

					if r.Method != "GET" && r.Method != "HEAD" && r.Method != "OPTIONS" {
						http.Error(w, "http requests forbidden", http.StatusForbidden)
						return
					}

					http.Redirect(w, r, target, http.StatusTemporaryRedirect)
				}))
				if err != nil {
					log.Panic(err)
				}
			}()
		}

		err = http.ListenAndServeTLS(util.Config.Interface+port, util.Config.TLS.CertFile, util.Config.TLS.KeyFile, cropTrailingSlashMiddleware(router))

		if err != nil {
			log.Panic(err)
		}

	} else {
		err = http.ListenAndServe(util.Config.Interface+port, cropTrailingSlashMiddleware(router))
	}

	if err != nil {
		log.WithError(err).Panic("Error starting server")
	}
}

func createStoreWithMigrationVersion(token string, undoTo *string, applyTo *string) db.Store {
	util.ConfigInit(persistentFlags.configPath, persistentFlags.noConfig)

	store := factory.CreateStore()

	store.Connect()

	var err error
	if undoTo != nil {
		err = db.Rollback(store, *undoTo)
	} else {
		err = db.Migrate(store, applyTo)
	}

	if err != nil {
		panic(err)
	}

	err = db.FillConfigFromDB(store)

	if err != nil {
		panic(err)
	}

	util.LookupDefaultApps()

	return store
}

func createStore(token string) db.Store {
	return createStoreWithMigrationVersion(token, nil, nil)
}
