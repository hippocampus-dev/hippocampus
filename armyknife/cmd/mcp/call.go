package mcp

import (
	"armyknife/pkg/mcp/call"

	"github.com/spf13/cobra"
	"golang.org/x/xerrors"
)

func callCmd() *cobra.Command {
	callArgs := call.DefaultArgs()

	cmd := &cobra.Command{
		Use:          "call URL TOOL [ARGUMENTS]",
		Short:        "Call a tool on an MCP HTTP+SSE server and print its text result",
		SilenceUsage: true,
		Args:         cobra.RangeArgs(2, 3),
		PreRun: func(cmd *cobra.Command, args []string) {
			callArgs.URL = args[0]
			callArgs.Tool = args[1]
			if len(args) > 2 {
				callArgs.Arguments = args[2]
			}
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := call.Run(callArgs); err != nil {
				return xerrors.Errorf("failed to run call.Run: %w", err)
			}
			return nil
		},
	}

	return cmd
}
