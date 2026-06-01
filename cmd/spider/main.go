package main

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/spiderai/spider/internal/config"
	"github.com/spiderai/spider/internal/db"
	server "github.com/spiderai/spider/internal/server"
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
	root.AddCommand(newResetPasswordCmd(&cfgFile, &dataDir))

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

func newResetPasswordCmd(cfgFile, dataDir *string) *cobra.Command {
	return &cobra.Command{
		Use:   "reset-password <username> <new-password>",
		Short: "重置指定用户的密码（无需旧密码）",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			username, newPassword := args[0], args[1]
			if len(newPassword) < 8 {
				return fmt.Errorf("密码至少 8 位")
			}
			cfg, err := config.Load(*cfgFile)
			if err != nil {
				return fmt.Errorf("加载配置失败: %w", err)
			}
			if *dataDir != "" {
				cfg.DataDir = *dataDir
			}
			database, err := db.Open(cfg.DataDir)
			if err != nil {
				return fmt.Errorf("打开数据库失败: %w", err)
			}
			defer database.Close()
			us := store.NewUserStore(database)
			user, err := us.GetByUsername(username)
			if err != nil {
				return fmt.Errorf("用户不存在: %s", username)
			}
			if _, err := us.Update(user.ID, nil, nil, &newPassword); err != nil {
				return fmt.Errorf("重置密码失败: %w", err)
			}
			fmt.Printf("用户 %s 密码已重置\n", username)
			return nil
		},
	}
}


func serve(cfgFile, addrOverride, dataDirOverride string, debug bool) error {
	return server.Run(context.Background(), server.Options{
		ConfigFile: cfgFile,
		Addr:       addrOverride,
		DataDir:    dataDirOverride,
		Debug:      debug,
		Version:    version,
		WebFS:      webFS,
		SkillsFS:   builtinSkillsFS,
	})
}
