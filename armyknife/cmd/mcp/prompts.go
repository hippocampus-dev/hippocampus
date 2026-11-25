package mcp

import (
	"armyknife/pkg/mcp/prompts"

	"github.com/spf13/cobra"
	"golang.org/x/xerrors"
)

func promptsCmd() *cobra.Command {
	promptsArgs := prompts.DefaultArgs()

	cmd := &cobra.Command{
		Use:          "prompts",
		Short:        "MCP server for managing prompts and prompt templates",
		SilenceUsage: true,
		Args:         cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := prompts.Run(promptsArgs); err != nil {
				return xerrors.Errorf("failed to run prompts.Run: %w", err)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(
		&promptsArgs.Address,
		"address",
		promptsArgs.Address,
		"Serve over HTTP+SSE on this address instead of stdio",
	)

	return cmd
}
