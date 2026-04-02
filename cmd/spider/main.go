package main

import (
	"fmt"
	"os"
	"time"

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

	return mcppkg.Serve(&mcppkg.App{
		HostStore: hs,
		LogStore:  ls,
		Pool:      pool,
		Config:    cfg,
		DB:        database,
	})
}

