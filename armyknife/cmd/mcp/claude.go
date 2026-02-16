package mcp

import (
	"armyknife/pkg/mcp/claude"

	"github.com/spf13/cobra"
	"golang.org/x/xerrors"
)

func claudeCmd() *cobra.Command {
	claudeArgs := claude.DefaultArgs()

	cmd := &cobra.Command{
		Use:          "claude",
		Short:        "Run MCP server for claude",
		Long:         "Starts an MCP server that provides claude features",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := claude.Run(claudeArgs); err != nil {
				return xerrors.Errorf("failed to run claude.Run: %w", err)
			}
			return nil
		},
	}

	return cmd
}
