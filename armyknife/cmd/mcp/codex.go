package mcp

import (
	"armyknife/pkg/mcp/codex"

	"github.com/spf13/cobra"
	"golang.org/x/xerrors"
)

func codexCmd() *cobra.Command {
	codexArgs := codex.DefaultArgs()

	cmd := &cobra.Command{
		Use:          "codex",
		Short:        "Run MCP server for codex",
		Long:         "Starts an MCP server that provides codex features",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := codex.Run(codexArgs); err != nil {
				return xerrors.Errorf("failed to run codex.Run: %w", err)
			}
			return nil
		},
	}

	return cmd
}
