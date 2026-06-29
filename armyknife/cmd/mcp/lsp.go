package mcp

import (
	"armyknife/pkg/mcp/lsp"

	"github.com/spf13/cobra"
	"golang.org/x/xerrors"
)

func lspCmd() *cobra.Command {
	lspArgs := lsp.DefaultArgs()

	cmd := &cobra.Command{
		Use:          "lsp LSP_SERVER_ADDRESSES...",
		Short:        "Run MCP server for LSP operations",
		Long:         "Starts an MCP server that provides LSP functionality including workspace symbols and code extraction",
		SilenceUsage: true,
		Args:         cobra.MinimumNArgs(1),
		PreRun: func(cmd *cobra.Command, args []string) {
			lspArgs.LSPServers = args
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := lsp.Run(lspArgs); err != nil {
				return xerrors.Errorf("failed to run lsp.Run: %w", err)
			}
			return nil
		},
	}

	return cmd
}
