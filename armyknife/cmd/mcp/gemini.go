package mcp

import (
	"armyknife/pkg/mcp/gemini"

	"github.com/spf13/cobra"
	"golang.org/x/xerrors"
)

func geminiCmd() *cobra.Command {
	geminiArgs := gemini.DefaultArgs()

	cmd := &cobra.Command{
		Use:          "gemini",
		Short:        "Run MCP server for gemini",
		Long:         "Starts an MCP server that provides gemini features",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := gemini.Run(geminiArgs); err != nil {
				return xerrors.Errorf("failed to run gemini.Run: %w", err)
			}
			return nil
		},
	}

	return cmd
}
