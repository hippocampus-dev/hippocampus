package mcp

import (
	"armyknife/pkg/mcp/codex"

	"github.com/spf13/cobra"
	"golang.org/x/xerrors"
)

func codexCmd() *cobra.Command {
	notifyArgs := codex.DefaultArgs()

	cmd := &cobra.Command{
		Use:   "codex",
		Short: "Run MCP server for codex",
		Long:  "Starts an MCP server that provides codex features",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := codex.Run(notifyArgs); err != nil {
				return xerrors.Errorf("failed to run call.Run: %w", err)
			}
			return nil
		},
	}

	return cmd
}
