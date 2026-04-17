package cli

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/spiderai/spider/internal/client"
)

// NewExecCmd 返回 exec 子命令。
func NewExecCmd(url *string, defaultTimeout int) *cobra.Command {
	var timeoutSec int
	cmd := &cobra.Command{
		Use:   "exec <host> <command>",
		Short: "在远程主机上执行命令",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			c := client.New(*url)
			host, err := c.GetHost(args[0])
			if err != nil {
				return fmt.Errorf("主机不存在: %s", args[0])
			}
			result, err := c.Exec(&client.ExecRequest{
				HostID:         host.ID,
				Command:        args[1],
				TimeoutSeconds: timeoutSec,
			})
			if err != nil {
				return err
			}
			if result.Error != "" {
				return fmt.Errorf("%s", result.Error)
			}
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
	cmd.Flags().IntVar(&timeoutSec, "timeout", defaultTimeout, "超时秒数")
	return cmd
}

// NewPingCmd 返回 ping 子命令。
func NewPingCmd(url *string) *cobra.Command {
	return &cobra.Command{
		Use:   "ping <host>",
		Short: "测试与主机的 SSH 连通性",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c := client.New(*url)
			host, err := c.GetHost(args[0])
			if err != nil {
				return fmt.Errorf("主机不存在: %s", args[0])
			}
			result, err := c.PingHost(host.ID)
			if err != nil {
				return err
			}
			out := map[string]any{
				"host":       host.Name,
				"connected":  result.Connected,
				"latency_ms": result.LatencyMs,
			}
			if result.Error != "" {
				out["error"] = result.Error
			}
			data, _ := json.MarshalIndent(out, "", "  ")
			fmt.Println(string(data))
			return nil
		},
	}
}

// NewHistoryCmd 返回 history 子命令。
func NewHistoryCmd(url *string) *cobra.Command {
	var hostIDOrName string
	var limit int
	cmd := &cobra.Command{
		Use:   "history",
		Short: "查看命令执行历史",
		RunE: func(cmd *cobra.Command, args []string) error {
			c := client.New(*url)
			hostID := ""
			if hostIDOrName != "" {
				host, err := c.GetHost(hostIDOrName)
				if err != nil {
					return fmt.Errorf("主机不存在: %s", hostIDOrName)
				}
				hostID = host.ID
			}
			logs, err := c.ListLogs(hostID, limit)
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
