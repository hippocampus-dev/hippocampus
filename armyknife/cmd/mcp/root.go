package mcp

import (
	"github.com/spf13/cobra"
)

func GetRootCmd(args []string) *cobra.Command {
	cmd := &cobra.Command{
		Use:          "mcp",
		Short:        "MCP (Model Context Protocol) utilities",
		SilenceUsage: true,
	}

	cmd.AddCommand(agyCmd())
	cmd.AddCommand(callCmd())
	cmd.AddCommand(claudeCmd())
	cmd.AddCommand(codexCmd())
	cmd.AddCommand(computerCmd())
	cmd.AddCommand(geminiCmd())
	cmd.AddCommand(lspCmd())
	cmd.AddCommand(notifyCmd())
	cmd.AddCommand(promptsCmd())
	cmd.AddCommand(tmuxCmd())

	return cmd
}
