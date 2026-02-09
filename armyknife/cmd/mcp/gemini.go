package mcp

import (
	"armyknife/pkg/mcp/gemini"

	"github.com/spf13/cobra"
	"golang.org/x/xerrors"
)

func geminiCmd() *cobra.Command {
	notifyArgs := gemini.DefaultArgs()

	cmd := &cobra.Command{
		Use:   "gemini",
		Short: "Run MCP server for gemini",
		Long:  "Starts an MCP server that provides gemini features",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := gemini.Run(notifyArgs); err != nil {
				return xerrors.Errorf("failed to run call.Run: %w", err)
			}
			return nil
		},
	}

	return cmd
}
