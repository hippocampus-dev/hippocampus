package mcp

import (
	"armyknife/pkg/mcp/prompts"

	"github.com/spf13/cobra"
)

func promptsCmd() *cobra.Command {
	promptsArgs := prompts.DefaultArgs()

	cmd := &cobra.Command{
		Use:   "prompts",
		Short: "MCP server for managing prompts and prompt templates",
		RunE: func(cmd *cobra.Command, args []string) error {
			return prompts.Run(promptsArgs)
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
