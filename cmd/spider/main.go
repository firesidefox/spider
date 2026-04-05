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

	apipkg "github.com/spiderai/spider/internal/api"
	mcppkg "github.com/spiderai/spider/internal/mcp"
	sshpkg "github.com/spiderai/spider/internal/ssh"

	"github.com/spiderai/spider/internal/config"
	"github.com/spiderai/spider/internal/crypto"
	"github.com/spiderai/spider/internal/db"
	"github.com/spiderai/spider/internal/store"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "错误:", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load("")
	if err != nil {
		return fmt.Errorf("加载配置失败: %w", err)
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
	mux.Handle("/api/", apipkg.NewRouter(app))

	sub, err := fs.Sub(webFS, "web/dist")
	if err != nil {
		return fmt.Errorf("加载 web 资源失败: %w", err)
	}
	mux.Handle("/", http.FileServer(http.FS(sub)))

	srv := &http.Server{Addr: cfg.SSE.Addr, Handler: mux}

	errCh := make(chan error, 1)
	go func() {
		fmt.Fprintf(os.Stderr, "Spider 启动，监听 %s\n", cfg.SSE.Addr)
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
