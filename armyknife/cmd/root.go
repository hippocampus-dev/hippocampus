package cmd

import (
	"armyknife/cmd/grpc"
	"armyknife/cmd/llm"
	"armyknife/pkg/version"

	"github.com/spf13/cobra"
)

func GetRootCmd(args []string) *cobra.Command {
	cmd := &cobra.Command{
		Use:          "armyknife",
		Short:        "This is Armyknife",
		SilenceUsage: true,
		Version:      version.Version,
	}

	cmd.SetArgs(args)
	cmd.AddCommand(grpc.GetRootCmd(args))
	cmd.AddCommand(llm.GetRootCmd(args))
	cmd.AddCommand(bakeryCmd())
	cmd.AddCommand(completionCmd())
	cmd.AddCommand(echoCmd())
	cmd.AddCommand(selfupdateCmd())
	cmd.AddCommand(serveCmd())

	return cmd
}
