package mcp

import (
	"github.com/spf13/cobra"
)

func GetRootCmd(args []string) *cobra.Command {
	cmd := &cobra.Command{
		Use:          "mcp",
		Short:        "MCP (Model Context Protocol) servers",
		SilenceUsage: true,
	}

	cmd.AddCommand(claudeCmd())
	cmd.AddCommand(codexCmd())
	cmd.AddCommand(geminiCmd())
	cmd.AddCommand(lspCmd())
	cmd.AddCommand(notifyCmd())
	cmd.AddCommand(promptsCmd())
	cmd.AddCommand(tmuxCmd())

	return cmd
}
