package mcp

import (
	"armyknife/pkg/mcp/computer"

	"github.com/spf13/cobra"
	"golang.org/x/xerrors"
)

func computerCmd() *cobra.Command {
	computerArgs := computer.DefaultArgs()

	cmd := &cobra.Command{
		Use:          "computer",
		Short:        "Run MCP server for X11 desktop automation",
		Long:         "Starts an MCP server that provides Anthropic computer use compatible desktop automation via X11",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := computer.Run(computerArgs); err != nil {
				return xerrors.Errorf("failed to run computer.Run: %w", err)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(
		&computerArgs.DisplayName,
		"display",
		computerArgs.DisplayName,
		"X11 display name (e.g. :0, :1)",
	)

	return cmd
}
