package cmd

import (
	"github.com/spf13/cobra"
)

func GetRootCmd(args []string) *cobra.Command {
	cmd := &cobra.Command{
		Use:          "loganomaly",
		SilenceUsage: true,
	}

	cmd.SetArgs(args)

	cmd.AddCommand(consumerCmd())
	cmd.AddCommand(deduplicatorCmd())
	cmd.AddCommand(adapterCmd())

	return cmd
}
