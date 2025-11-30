package mcp

import (
	"armyknife/pkg/mcp/claude"

	"github.com/spf13/cobra"
	"golang.org/x/xerrors"
)

func claudeCmd() *cobra.Command {
	notifyArgs := claude.DefaultArgs()

	cmd := &cobra.Command{
		Use:   "claude",
		Short: "Run MCP server for claude",
		Long:  "Starts an MCP server that provides claude features",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := claude.Run(notifyArgs); err != nil {
				return xerrors.Errorf("failed to run call.Run: %w", err)
			}
			return nil
		},
	}

	return cmd
}
