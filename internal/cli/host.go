package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"github.com/spiderai/spider/internal/models"
	"github.com/spiderai/spider/internal/store"
)

// NewHostCmd 返回 host 子命令组。
func NewHostCmd(hs *store.HostStore) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "host",
		Short: "主机管理",
	}
	cmd.AddCommand(
		newHostAddCmd(hs),
		newHostListCmd(hs),
		newHostRmCmd(hs),
		newHostUpdateCmd(hs),
	)
	return cmd
}

func newHostAddCmd(hs *store.HostStore) *cobra.Command {
	var (
		ip          string
		port        int
		username    string
		authType    string
		keyFile     string
		password    string
		passphrase  string
		proxyHostID string
		tags        string
	)
	cmd := &cobra.Command{
		Use:   "add <name>",
		Short: "添加主机",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			credential := password

			// 如果指定了 key 文件，读取内容
			if keyFile != "" {
				data, err := os.ReadFile(keyFile)
				if err != nil {
					return fmt.Errorf("读取私钥文件失败: %w", err)
				}
				credential = string(data)
			}

			at := models.AuthType(authType)
			req := &models.AddHostRequest{
				Name:        name,
				IP:          ip,
				Port:        port,
				Username:    username,
				AuthType:    at,
				Credential:  credential,
				Passphrase:  passphrase,
				ProxyHostID: proxyHostID,
				Tags:        splitTags(tags),
			}
			host, err := hs.Add(req)
			if err != nil {
				return err
			}
			fmt.Printf("主机添加成功: %s (ID: %s)\n", host.Name, host.ID)
			return nil
		},
	}
	cmd.Flags().StringVar(&ip, "ip", "", "主机 IP 地址（必填）")
	cmd.Flags().IntVar(&port, "port", 22, "SSH 端口")
	cmd.Flags().StringVar(&username, "user", "root", "SSH 用户名")
	cmd.Flags().StringVar(&authType, "auth", "key", "认证类型: password | key | key_password")
	cmd.Flags().StringVar(&keyFile, "key", "", "SSH 私钥文件路径")
	cmd.Flags().StringVar(&password, "password", "", "SSH 密码（auth=password 时使用）")
	cmd.Flags().StringVar(&passphrase, "passphrase", "", "私钥 passphrase（auth=key_password 时使用）")
	cmd.Flags().StringVar(&proxyHostID, "proxy", "", "跳板机主机 ID")
	cmd.Flags().StringVar(&tags, "tag", "", "标签（逗号分隔，如 prod,web）")
	_ = cmd.MarkFlagRequired("ip")
	return cmd
}

func newHostListCmd(hs *store.HostStore) *cobra.Command {
	var tag string
	var jsonOutput bool
	cmd := &cobra.Command{
		Use:   "list",
		Short: "列出所有主机",
		RunE: func(cmd *cobra.Command, args []string) error {
			hosts, err := hs.List(tag)
			if err != nil {
				return err
			}
			if jsonOutput {
				safeHosts := make([]*models.SafeHost, len(hosts))
				for i, h := range hosts {
					safeHosts[i] = h.Safe()
				}
				data, _ := json.MarshalIndent(safeHosts, "", "  ")
				fmt.Println(string(data))
				return nil
			}
			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "ID\tNAME\tIP\tPORT\tUSER\tAUTH\tTAGS")
			for _, h := range hosts {
				fmt.Fprintf(w, "%s\t%s\t%s\t%d\t%s\t%s\t%s\n",
					h.ID[:8]+"...", h.Name, h.IP, h.Port, h.Username,
					string(h.AuthType), strings.Join(h.Tags, ","),
				)
			}
			w.Flush()
			return nil
		},
	}
	cmd.Flags().StringVar(&tag, "tag", "", "按标签过滤")
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "JSON 格式输出")
	return cmd
}

func newHostRmCmd(hs *store.HostStore) *cobra.Command {
	return &cobra.Command{
		Use:   "rm <id-or-name>",
		Short: "删除主机",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			host, err := hs.GetByIDOrName(args[0])
			if err != nil {
				return err
			}
			if err := hs.Delete(host.ID); err != nil {
				return err
			}
			fmt.Printf("主机 %s 已删除\n", host.Name)
			return nil
		},
	}
}

func newHostUpdateCmd(hs *store.HostStore) *cobra.Command {
	var (
		name        string
		ip          string
		port        int
		username    string
		authType    string
		keyFile     string
		password    string
		passphrase  string
		proxyHostID string
		tags        string
	)
	cmd := &cobra.Command{
		Use:   "update <id-or-name>",
		Short: "更新主机信息",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			host, err := hs.GetByIDOrName(args[0])
			if err != nil {
				return err
			}
			req := &models.UpdateHostRequest{}
			if cmd.Flags().Changed("name") {
				req.Name = &name
			}
			if cmd.Flags().Changed("ip") {
				req.IP = &ip
			}
			if cmd.Flags().Changed("port") {
				req.Port = &port
			}
			if cmd.Flags().Changed("user") {
				req.Username = &username
			}
			if cmd.Flags().Changed("auth") {
				at := models.AuthType(authType)
				req.AuthType = &at
			}
			if cmd.Flags().Changed("key") {
				data, err := os.ReadFile(keyFile)
				if err != nil {
					return fmt.Errorf("读取私钥文件失败: %w", err)
				}
				s := string(data)
				req.Credential = &s
			}
			if cmd.Flags().Changed("password") {
				req.Credential = &password
			}
			if cmd.Flags().Changed("passphrase") {
				req.Passphrase = &passphrase
			}
			if cmd.Flags().Changed("proxy") {
				req.ProxyHostID = &proxyHostID
			}
			if cmd.Flags().Changed("tag") {
				req.Tags = splitTags(tags)
			}
			updated, err := hs.Update(host.ID, req)
			if err != nil {
				return err
			}
			fmt.Printf("主机 %s 更新成功\n", updated.Name)
			return nil
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "新名称")
	cmd.Flags().StringVar(&ip, "ip", "", "新 IP")
	cmd.Flags().IntVar(&port, "port", 0, "新端口")
	cmd.Flags().StringVar(&username, "user", "", "新用户名")
	cmd.Flags().StringVar(&authType, "auth", "", "新认证类型")
	cmd.Flags().StringVar(&keyFile, "key", "", "新私钥文件")
	cmd.Flags().StringVar(&password, "password", "", "新密码")
	cmd.Flags().StringVar(&passphrase, "passphrase", "", "新 passphrase")
	cmd.Flags().StringVar(&proxyHostID, "proxy", "", "新跳板机 ID")
	cmd.Flags().StringVar(&tags, "tag", "", "新标签（逗号分隔）")
	return cmd
}

func splitTags(s string) []string {
	if s == "" {
		return []string{}
	}
	var tags []string
	for _, t := range strings.Split(s, ",") {
		t = strings.TrimSpace(t)
		if t != "" {
			tags = append(tags, t)
		}
	}
	return tags
}
