package cmd

import (
	"github.com/spf13/cobra"
)

func GetRootCmd(args []string) *cobra.Command {
	cmd := &cobra.Command{
		Use:          "prometheus-metrics-proxy-hook",
		Short:        "",
		SilenceUsage: true,
	}

	cmd.SetArgs(args)

	cmd.AddCommand(sidecarCmd())
	cmd.AddCommand(webhookCmd())

	return cmd
}
