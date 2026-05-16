package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	apipkg "github.com/spiderai/spider/internal/api"
	"github.com/spiderai/spider/internal/agent"
	"github.com/spiderai/spider/internal/auth"
	"github.com/spiderai/spider/internal/logger"
	mcppkg "github.com/spiderai/spider/internal/mcp"
	"github.com/spiderai/spider/internal/monitor"
	"github.com/spiderai/spider/internal/permission"
	"github.com/spiderai/spider/internal/scheduler"
	sshpkg "github.com/spiderai/spider/internal/ssh"

	"github.com/spiderai/spider/internal/config"
	"github.com/spiderai/spider/internal/crypto"
	"github.com/spiderai/spider/internal/db"
	"github.com/spiderai/spider/internal/store"
)

// 由 ldflags 注入
var (
	version   = "dev"
	commit    = "unknown"
	buildTime = "unknown"
)

func main() {
	if err := newRootCmd().Execute(); err != nil {
		os.Exit(1)
	}
}

func newRootCmd() *cobra.Command {
	var cfgFile string
	var addr string
	var dataDir string
	var debug bool

	root := &cobra.Command{
		Use:          "spider",
		Short:        "Spider — 智能运维平台 MCP Server",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return serve(cfgFile, addr, dataDir, debug)
		},
	}

	root.PersistentFlags().StringVar(&cfgFile, "config", "", "配置文件路径（默认 ~/.spider/config.yaml）")
	root.PersistentFlags().StringVar(&addr, "addr", "", "监听地址（覆盖配置，如 :9090）")
	root.PersistentFlags().StringVar(&dataDir, "data-dir", "", "数据目录（覆盖配置和 SPIDER_DATA_DIR）")
	root.PersistentFlags().BoolVar(&debug, "debug", false, "启用 debug 日志级别")

	root.AddCommand(newServeCmd(&cfgFile, &addr, &dataDir, &debug))
	root.AddCommand(newVersionCmd())

	return root
}

func newServeCmd(cfgFile, addr, dataDir *string, debug *bool) *cobra.Command {
	return &cobra.Command{
		Use:   "serve",
		Short: "启动 Spider MCP Server（默认行为）",
		RunE: func(cmd *cobra.Command, args []string) error {
			return serve(*cfgFile, *addr, *dataDir, *debug)
		},
	}
}

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "显示版本信息",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("spider %s (commit: %s, built: %s)\n", version, commit, buildTime)
		},
	}
}

// spaHandler serves static files and falls back to index.html for SPA routing.
func spaHandler(fsys http.FileSystem) http.Handler {
	fileServer := http.FileServer(fsys)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f, err := fsys.Open(r.URL.Path)
		if err != nil {
			// file not found → serve index.html for SPA client-side routing
			r2 := *r
			r2.URL.Path = "/"
			fileServer.ServeHTTP(w, &r2)
			return
		}
		f.Close()
		fileServer.ServeHTTP(w, r)
	})
}

func serve(cfgFile, addrOverride, dataDirOverride string, debug bool) error {
	cfg, err := config.Load(cfgFile)
	if err != nil {
		return fmt.Errorf("加载配置失败: %w", err)
	}
	cfgPath := cfgFile
	if cfgPath == "" {
		cfgPath = config.DefaultConfigPath()
	}
	if addrOverride != "" {
		cfg.SSE.Addr = addrOverride
		host, port, err := net.SplitHostPort(addrOverride)
		if err == nil && (host == "" || host == "0.0.0.0" || host == "::") {
			cfg.SSE.BaseURL = "http://localhost:" + port
		} else {
			cfg.SSE.BaseURL = "http://" + addrOverride
		}
	}
	if dataDirOverride != "" {
		cfg.DataDir = dataDirOverride
	}

	if err := cfg.EnsureDataDir(); err != nil {
		return fmt.Errorf("初始化数据目录失败: %w", err)
	}
	if err := agent.SyncBuiltinSkills(cfg.DataDir, builtinSkillsFS); err != nil {
		return fmt.Errorf("同步内置 skills 失败: %w", err)
	}

	logFile := cfg.Log.File
	if logFile == "" {
		logFile = filepath.Join(cfg.LogsDir, "spider.log")
	}
	if debug {
		cfg.Log.Level = "debug"
	}
	if err := logger.Init(logger.Config{
		Level:      cfg.Log.Level,
		Format:     cfg.Log.Format,
		File:       logFile,
		MaxSizeMB:  cfg.Log.MaxSizeMB,
		MaxBackups: cfg.Log.MaxBackups,
		Stderr:     cfg.Log.Stderr,
	}); err != nil {
		return fmt.Errorf("初始化日志失败: %w", err)
	}
	logger.ForModule("main").Info().Str("version", version).Str("addr", cfg.SSE.Addr).Msg("spider starting")

	cm, err := crypto.NewManager(cfg.DataDir)
	if err != nil {
		return fmt.Errorf("初始化加密模块失败: %w", err)
	}

	database, err := db.Open(cfg.DataDir)
	if err != nil {
		return fmt.Errorf("打开数据库失败: %w", err)
	}
	defer database.Close()

	hs := store.NewHostStore(database)
	ls := store.NewLogStore(database)
	us := store.NewUserStore(database)
	ts := store.NewTokenStore(database)
	ks := store.NewSSHKeyStore(database, cm)
	afs := store.NewAccessFaceStore(database, cm)
	fps := store.NewFingerprintStore(database)
	ms := store.NewMemoryStore(database)

	jwtMgr, err := auth.NewJWTManager(cfg.DataDir)
	if err != nil {
		return fmt.Errorf("初始化 JWT 模块失败: %w", err)
	}

	if err := us.EnsureDefaultAdmin(); err != nil {
		return fmt.Errorf("初始化默认管理员失败: %w", err)
	}

	pool := sshpkg.NewPool(time.Duration(cfg.SSH.PoolTTL) * time.Second)
	pool.StartCleanup()
	defer pool.Close()

	shutdownCtx, shutdownCancel := context.WithCancel(context.Background())
	defer shutdownCancel()

	app := &mcppkg.App{
		HostStore:        hs,
		SSHKeyStore:      ks,
		LogStore:         ls,
		Pool:             pool,
		Config:           cfg,
		ConfigPath:       cfgPath,
		DB:               database,
		UserStore:        us,
		TokenStore:       ts,
		JWTManager:       jwtMgr,
		AccessFaceStore:  afs,
		FingerprintStore: fps,
		MemoryStore:      ms,
		ShutdownCtx:      shutdownCtx,
	}

	app.ConvStore = store.NewConversationStore(database)
	app.MsgStore = store.NewMessageStore(database)
	app.DocStore = store.NewDocumentStore(database)
	app.GroupStore = store.NewGroupStore(database)
	ps := store.NewProviderStore(database, cm)
	app.ProviderStore = ps
	app.RagConfigStore = store.NewRagConfigStore(database, cm)
	app.TodoStore = store.NewTodoStore(database)
	app.TopologyStore = store.NewTopologyStore(database)

	app.Classifier = permission.NewClassifier(nil)
	if len(cfg.Agent.Rules) > 0 {
		app.Classifier.Reload(cfg.Agent.Rules)
	}
	app.Enforcer = permission.NewEnforcer()
	app.ApprovalManager = permission.NewApprovalManager()
	app.PermissionMode = permission.Mode(cfg.Agent.PermissionMode)

	taskStore := store.NewTaskStore(database)
	taskRunStore := store.NewTaskRunStore(database)
	notifyChannelStore := store.NewNotifyChannelStore(database, cm)
	app.TaskStore = taskStore
	app.TaskRunStore = taskRunStore
	app.NotifyChannelStore = notifyChannelStore

	agentFactory, err := agent.NewFactory(
		ps, hs, afs, pool, ks, ls, app.MsgStore,
	)
	if err != nil {
		logger.ForModule("main").Warn().Err(err).Msg("agent factory not available")
	} else {
		agentFactory.Enforcer = app.Enforcer
		agentFactory.PermissionMode = app.PermissionMode
		agentFactory.SummaryStore = store.NewSummaryStore(database)
		agentFactory.CompactionCfg = cfg.Agent.Compaction
		agentFactory.MaxTurns = cfg.Agent.MaxTurns
		agentFactory.TodoStore = app.TodoStore
		agentFactory.TaskStore = taskStore
		app.AgentFactory = agentFactory
	}

	if app.AgentFactory != nil {
		exec := scheduler.NewExecutor(taskStore, taskRunStore, hs, app.AgentFactory, notifyChannelStore)
		app.Executor = exec
		// Mark any runs left in 'running' state from a previous crash as failed.
		if n, err := taskRunStore.MarkStaleRunsFailed(2 * time.Hour); err != nil {
			logger.ForModule("main").Warn().Err(err).Msg("startup sweep for stale task runs failed")
		} else if n > 0 {
			logger.ForModule("main").Info().Int64("count", n).Msg("marked stale task runs as failed")
		}
		sched := scheduler.NewScheduler(taskStore, taskRunStore, exec)
		sched.Start(shutdownCtx)
		defer sched.Stop()
		defer exec.Stop()
	}

	app.Monitor = monitor.New(
		app.HostStore,
		app.AccessFaceStore,
		func(hostID string, online bool) {
			data, _ := json.Marshal(map[string]any{
				"type": "host_status",
				"content": map[string]any{
					"host_id": hostID,
					"online":  online,
				},
			})
			app.BroadcastGlobalSSE(data)
		},
	)
	app.Monitor.Start(shutdownCtx)

	mcpHandler := mcppkg.NewHTTPHandler(app)

	mux := http.NewServeMux()
	mux.Handle("/mcp", mcpHandler)
	mux.HandleFunc("/install.sh", apipkg.InstallScriptHandler(app.Config.SSE.BaseURL))
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"ok"}`))
	})
	mux.Handle("/api/", apipkg.NewRouter(app))

	sub, err := fs.Sub(webFS, "dist")
	if err != nil {
		return fmt.Errorf("加载 web 资源失败: %w", err)
	}
	mux.Handle("/", spaHandler(http.FS(sub)))

	srv := &http.Server{Addr: cfg.SSE.Addr, Handler: mux}

	errCh := make(chan error, 1)
	go func() {
		logger.ForModule("main").Info().
			Str("addr", cfg.SSE.Addr).
			Str("mcp", cfg.SSE.BaseURL+"/mcp").
			Str("web", cfg.SSE.BaseURL).
			Msg("spider listening")
		errCh <- srv.ListenAndServe()
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	select {
	case err := <-errCh:
		return fmt.Errorf("http server: %w", err)
	case <-quit:
		shutdownCancel() // close SSE streams before HTTP shutdown
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := srv.Shutdown(ctx); err != nil && !errors.Is(err, context.DeadlineExceeded) {
			return err
		}
		return nil
	}
}
