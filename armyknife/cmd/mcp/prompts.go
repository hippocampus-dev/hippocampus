package mcp

import (
	"armyknife/pkg/mcp/prompts"

	"github.com/spf13/cobra"
)

func promptsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "prompts",
		Short: "MCP server for managing prompts and prompt templates",
		RunE: func(cmd *cobra.Command, args []string) error {
			return prompts.Run(&prompts.Args{})
		},
	}

	return cmd
}
