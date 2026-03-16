package cmd

import (
	"armyknife/cmd/grpc"
	"armyknife/cmd/llm"
	"armyknife/cmd/mcp"
	"armyknife/cmd/rails"
	"armyknife/cmd/registry"
	"armyknife/cmd/s3"
	"armyknife/cmd/searchx"
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
	cmd.AddCommand(mcp.GetRootCmd(args))
	cmd.AddCommand(s3.GetRootCmd(args))
	cmd.AddCommand(rails.GetRootCmd(args))
	cmd.AddCommand(searchx.GetRootCmd(args))
	cmd.AddCommand(registry.GetRootCmd(args))
	cmd.AddCommand(bakeryCmd())
	cmd.AddCommand(completionCmd())
	cmd.AddCommand(echoCmd())
	cmd.AddCommand(egosearchCmd())
	cmd.AddCommand(fallbackCmd())
	cmd.AddCommand(proxyCmd())
	cmd.AddCommand(selfupdateCmd())
	cmd.AddCommand(serveCmd())

	return cmd
}
