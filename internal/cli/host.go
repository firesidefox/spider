package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"github.com/spiderai/spider/internal/client"
	"github.com/spiderai/spider/internal/models"
)

// NewHostCmd 返回 host 子命令组。url 在执行时解引用，确保 --url flag 已解析。
func NewHostCmd(url *string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "host",
		Short: "主机管理",
	}
	cmd.AddCommand(
		newHostAddCmd(url),
		newHostListCmd(url),
		newHostRmCmd(url),
		newHostUpdateCmd(url),
	)
	return cmd
}

func newHostAddCmd(url *string) *cobra.Command {
	var (
		ip         string
		port       int
		username   string
		authType   string
		keyFile    string
		password   string
		passphrase string
		tags       string
	)
	cmd := &cobra.Command{
		Use:   "add <name>",
		Short: "添加主机",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			credential := password
			if keyFile != "" {
				data, err := os.ReadFile(keyFile)
				if err != nil {
					return fmt.Errorf("读取私钥文件失败: %w", err)
				}
				credential = string(data)
			}
			req := &models.AddHostRequest{
				Name:       args[0],
				IP:         ip,
				Port:       port,
				Username:   username,
				AuthType:   models.AuthType(authType),
				Credential: credential,
				Passphrase: passphrase,
				Tags:       splitTags(tags),
			}
			host, err := client.New(*url).AddHost(req)
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
	cmd.Flags().StringVar(&password, "password", "", "SSH 密码")
	cmd.Flags().StringVar(&passphrase, "passphrase", "", "私钥 passphrase")
	cmd.Flags().StringVar(&tags, "tag", "", "标签（逗号分隔）")
	_ = cmd.MarkFlagRequired("ip")
	return cmd
}

func newHostListCmd(url *string) *cobra.Command {
	var tag string
	var jsonOutput bool
	cmd := &cobra.Command{
		Use:   "list",
		Short: "列出所有主机",
		RunE: func(cmd *cobra.Command, args []string) error {
			hosts, err := client.New(*url).ListHosts(tag)
			if err != nil {
				return err
			}
			if jsonOutput {
				data, _ := json.MarshalIndent(hosts, "", "  ")
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

func newHostRmCmd(url *string) *cobra.Command {
	return &cobra.Command{
		Use:   "rm <id-or-name>",
		Short: "删除主机",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c := client.New(*url)
			host, err := c.GetHost(args[0])
			if err != nil {
				return err
			}
			if err := c.DeleteHost(host.ID); err != nil {
				return err
			}
			fmt.Printf("主机 %s 已删除\n", host.Name)
			return nil
		},
	}
}

func newHostUpdateCmd(url *string) *cobra.Command {
	var (
		name       string
		ip         string
		port       int
		username   string
		authType   string
		keyFile    string
		password   string
		passphrase string
		tags       string
	)
	cmd := &cobra.Command{
		Use:   "update <id-or-name>",
		Short: "更新主机信息",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c := client.New(*url)
			host, err := c.GetHost(args[0])
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
			if cmd.Flags().Changed("tag") {
				req.Tags = splitTags(tags)
			}
			updated, err := c.UpdateHost(host.ID, req)
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
