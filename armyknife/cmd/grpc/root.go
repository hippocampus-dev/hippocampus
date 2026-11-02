package grpc

import (
	"github.com/spf13/cobra"
)

func GetRootCmd(args []string) *cobra.Command {
	cmd := &cobra.Command{
		Use:          "grpc",
		Short:        "gRPC utilities",
		SilenceUsage: true,
	}

	cmd.AddCommand(callCmd())
	cmd.AddCommand(catchCmd())

	return cmd
}
