package mcp

import (
	"armyknife/pkg/mcp/tmux"

	"github.com/spf13/cobra"
	"golang.org/x/xerrors"
)

func tmuxCmd() *cobra.Command {
	tmuxArgs := tmux.DefaultArgs()

	cmd := &cobra.Command{
		Use:   "tmux",
		Short: "Run MCP server for tmux control",
		Long:  "Starts an MCP server that provides tmux control capabilities",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := tmux.Run(tmuxArgs); err != nil {
				return xerrors.Errorf("failed to run tmux.Run: %w", err)
			}
			return nil
		},
	}

	return cmd
}
