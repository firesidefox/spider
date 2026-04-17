package main

import (
	"context"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	apipkg "github.com/spiderai/spider/internal/api"
	mcppkg "github.com/spiderai/spider/internal/mcp"
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

	root := &cobra.Command{
		Use:          "spider",
		Short:        "Spider — 智能运维平台 MCP Server",
		SilenceUsage: true,
		// 无子命令时直接启动服务
		RunE: func(cmd *cobra.Command, args []string) error {
			return serve(cfgFile, addr, dataDir)
		},
	}

	root.PersistentFlags().StringVar(&cfgFile, "config", "", "配置文件路径（默认 ~/.spider/config.yaml）")
	root.PersistentFlags().StringVar(&addr, "addr", "", "监听地址（覆盖配置，如 :9090）")
	root.PersistentFlags().StringVar(&dataDir, "data-dir", "", "数据目录（覆盖配置和 SPIDER_DATA_DIR）")

	root.AddCommand(newServeCmd(&cfgFile, &addr, &dataDir))
	root.AddCommand(newVersionCmd())

	return root
}

func newServeCmd(cfgFile, addr, dataDir *string) *cobra.Command {
	return &cobra.Command{
		Use:   "serve",
		Short: "启动 Spider MCP Server（默认行为）",
		RunE: func(cmd *cobra.Command, args []string) error {
			return serve(*cfgFile, *addr, *dataDir)
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

func serve(cfgFile, addrOverride, dataDirOverride string) error {
	cfg, err := config.Load(cfgFile)
	if err != nil {
		return fmt.Errorf("加载配置失败: %w", err)
	}
	if addrOverride != "" {
		cfg.SSE.Addr = addrOverride
		cfg.SSE.BaseURL = "http://localhost" + addrOverride
	}
	if dataDirOverride != "" {
		cfg.DataDir = dataDirOverride
	}

	if err := cfg.EnsureDataDir(); err != nil {
		return fmt.Errorf("初始化数据目录失败: %w", err)
	}

	cm, err := crypto.NewManager(cfg.DataDir)
	if err != nil {
		return fmt.Errorf("初始化加密模块失败: %w", err)
	}

	database, err := db.Open(cfg.DataDir)
	if err != nil {
		return fmt.Errorf("打开数据库失败: %w", err)
	}
	defer database.Close()

	hs := store.NewHostStore(database, cm)
	ls := store.NewLogStore(database)

	pool := sshpkg.NewPool(time.Duration(cfg.SSH.PoolTTL) * time.Second)
	pool.StartCleanup()
	defer pool.Close()

	app := &mcppkg.App{
		HostStore: hs,
		LogStore:  ls,
		Pool:      pool,
		Config:    cfg,
		DB:        database,
	}

	mcpHandler := mcppkg.NewHTTPHandler(app)

	mux := http.NewServeMux()
	mux.Handle("/mcp", mcpHandler)
	mux.HandleFunc("/install.sh", apipkg.InstallScriptHandler(app.Config.SSE.BaseURL))
	mux.HandleFunc("/server-install.sh", apipkg.ServerInstallScriptHandler(app.Config.SSE.BaseURL))
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"ok"}`))
	})
	mux.Handle("/api/", apipkg.NewRouter(app))

	sub, err := fs.Sub(webFS, "dist")
	if err != nil {
		return fmt.Errorf("加载 web 资源失败: %w", err)
	}
	mux.Handle("/", http.FileServer(http.FS(sub)))

	srv := &http.Server{Addr: cfg.SSE.Addr, Handler: mux}

	errCh := make(chan error, 1)
	go func() {
		fmt.Fprintf(os.Stderr, "Spider %s listening on %s\n", version, cfg.SSE.Addr)
		fmt.Fprintf(os.Stderr, "MCP endpoint:  %s/mcp\n", cfg.SSE.BaseURL)
		fmt.Fprintf(os.Stderr, "Web dashboard: %s\n", cfg.SSE.BaseURL)
		errCh <- srv.ListenAndServe()
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	select {
	case err := <-errCh:
		return err
	case <-quit:
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return srv.Shutdown(ctx)
	}
}
