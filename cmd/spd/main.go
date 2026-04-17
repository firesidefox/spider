package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/spiderai/spider/internal/cli"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "错误:", err)
		os.Exit(1)
	}
}

func run() error {
	var serverURL string

	root := &cobra.Command{
		Use:          "spd",
		Short:        "spd — Spider 运维管理工具",
		SilenceUsage: true,
	}
	root.PersistentFlags().StringVar(&serverURL, "url", "http://localhost:8000", "Spider 服务地址")

	root.AddCommand(
		cli.NewHostCmd(&serverURL),
		cli.NewExecCmd(&serverURL, 30),
		cli.NewPingCmd(&serverURL),
		cli.NewHistoryCmd(&serverURL),
		cli.NewMCPCmd(&serverURL),
	)

	return root.Execute()
}
