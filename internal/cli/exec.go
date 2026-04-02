package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/spf13/cobra"
	"github.com/spiderai/spider/internal/config"
	"github.com/spiderai/spider/internal/models"
	sshpkg "github.com/spiderai/spider/internal/ssh"
	"github.com/spiderai/spider/internal/store"
)

// NewExecCmd 返回 exec 子命令。
func NewExecCmd(hs *store.HostStore, ls *store.LogStore, pool *sshpkg.Pool, cfg *config.Config) *cobra.Command {
	var timeoutSec int
	cmd := &cobra.Command{
		Use:   "exec <host> <command>",
		Short: "在远程主机上执行命令",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			host, err := hs.GetByIDOrName(args[0])
			if err != nil {
				return fmt.Errorf("主机不存在: %s", args[0])
			}
			command := args[1]

			timeout := time.Duration(timeoutSec) * time.Second
			ctx, cancel := context.WithTimeout(context.Background(), timeout)
			defer cancel()

			client, err := pool.Get(host, hs)
			if err != nil {
				return fmt.Errorf("建立 SSH 连接失败: %w", err)
			}
			defer pool.Release(host.ID)

			result, err := client.Execute(ctx, command)
			if err != nil {
				return fmt.Errorf("执行命令失败: %w", err)
			}

			// 记录日志
			_ = ls.Save(&models.ExecutionLog{
				HostID:      host.ID,
				Command:     command,
				Stdout:      result.Stdout,
				Stderr:      result.Stderr,
				ExitCode:    result.ExitCode,
				DurationMs:  result.Duration.Milliseconds(),
				TriggeredBy: "cli",
			})

			if result.Stdout != "" {
				fmt.Print(result.Stdout)
			}
			if result.Stderr != "" {
				fmt.Fprint(cmd.ErrOrStderr(), result.Stderr)
			}
			if result.ExitCode != 0 {
				return fmt.Errorf("命令退出码: %d", result.ExitCode)
			}
			return nil
		},
	}
	cmd.Flags().IntVar(&timeoutSec, "timeout", cfg.SSH.DefaultTimeout, "超时秒数")
	return cmd
}

// NewPingCmd 返回 ping 子命令。
func NewPingCmd(hs *store.HostStore) *cobra.Command {
	return &cobra.Command{
		Use:   "ping <host>",
		Short: "测试与主机的 SSH 连通性",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			host, err := hs.GetByIDOrName(args[0])
			if err != nil {
				return fmt.Errorf("主机不存在: %s", args[0])
			}

			latency, err := sshpkg.CheckConnectivity(host, hs)
			result := map[string]any{
				"host":       host.Name,
				"connected":  err == nil,
				"latency_ms": latency.Milliseconds(),
			}
			if err != nil {
				result["error"] = err.Error()
			}
			data, _ := json.MarshalIndent(result, "", "  ")
			fmt.Println(string(data))
			return nil
		},
	}
}

// NewHistoryCmd 返回 history 子命令。
func NewHistoryCmd(hs *store.HostStore, ls *store.LogStore) *cobra.Command {
	var hostIDOrName string
	var limit int
	cmd := &cobra.Command{
		Use:   "history",
		Short: "查看命令执行历史",
		RunE: func(cmd *cobra.Command, args []string) error {
			hostID := ""
			if hostIDOrName != "" {
				host, err := hs.GetByIDOrName(hostIDOrName)
				if err != nil {
					return fmt.Errorf("主机不存在: %s", hostIDOrName)
				}
				hostID = host.ID
			}
			logs, err := ls.List(hostID, limit, 0)
			if err != nil {
				return err
			}
			if len(logs) == 0 {
				fmt.Println("暂无执行历史")
				return nil
			}
			data, _ := json.MarshalIndent(logs, "", "  ")
			fmt.Println(string(data))
			return nil
		},
	}
	cmd.Flags().StringVar(&hostIDOrName, "host", "", "按主机过滤")
	cmd.Flags().IntVar(&limit, "n", 20, "返回条数")
	return cmd
}
