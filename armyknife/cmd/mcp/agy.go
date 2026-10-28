package mcp

import (
	"armyknife/pkg/mcp/agy"

	"github.com/spf13/cobra"
	"golang.org/x/xerrors"
)

func agyCmd() *cobra.Command {
	agyArgs := agy.DefaultArgs()

	cmd := &cobra.Command{
		Use:          "agy",
		Short:        "Run MCP server for agy",
		Long:         "Starts an MCP server that provides agy features",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := agy.Run(agyArgs); err != nil {
				return xerrors.Errorf("failed to run agy.Run: %w", err)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(
		&agyArgs.Address,
		"address",
		agyArgs.Address,
		"Serve over HTTP+SSE on this address instead of stdio",
	)

	return cmd
}
