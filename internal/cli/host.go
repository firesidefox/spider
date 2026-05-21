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
		ip    string
		notes string
		tags  string
	)
	cmd := &cobra.Command{
		Use:   "add <name>",
		Short: "添加主机",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			req := &models.AddHostRequest{
				Name:  args[0],
				IP:    ip,
				Notes: notes,
				Tags:  splitTags(tags),
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
	cmd.Flags().StringVar(&notes, "notes", "", "备注")
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
			hosts, err := client.New(*url).GetHosts(tag)
			if err != nil {
				return err
			}
			if jsonOutput {
				data, _ := json.MarshalIndent(hosts, "", "  ")
				fmt.Println(string(data))
				return nil
			}
			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "ID\tNAME\tIP\tTAGS")
			for _, h := range hosts {
				fmt.Fprintf(w, "%s\t%s\t%s\t%s\n",
					h.ID[:8]+"...", h.Name, h.IP,
					strings.Join(h.Tags, ","),
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
		name  string
		ip    string
		notes string
		tags  string
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
			if cmd.Flags().Changed("notes") {
				req.Notes = &notes
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
	cmd.Flags().StringVar(&notes, "notes", "", "新备注")
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
