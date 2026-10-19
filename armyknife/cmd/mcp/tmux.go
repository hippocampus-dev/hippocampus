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
		Short: "Run MCP server for tmux pane title, window name and completion signalling",
		Long:  "Starts an MCP server that provides tmux pane title and window name control along with completion signalling. Serves over stdio by default, or over HTTP+SSE with --address.",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := tmux.Run(tmuxArgs); err != nil {
				return xerrors.Errorf("failed to run tmux.Run: %w", err)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(
		&tmuxArgs.Address,
		"address",
		tmuxArgs.Address,
		"Serve over HTTP+SSE on this address instead of stdio",
	)

	return cmd
}
