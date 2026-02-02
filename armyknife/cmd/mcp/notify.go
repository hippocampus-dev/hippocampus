package mcp

import (
	"armyknife/pkg/mcp/notify"

	"github.com/spf13/cobra"
	"golang.org/x/xerrors"
)

func notifyCmd() *cobra.Command {
	notifyArgs := notify.DefaultArgs()

	cmd := &cobra.Command{
		Use:   "notify",
		Short: "Run MCP server for desktop notifications",
		Long:  "Starts an MCP server that provides desktop notification capabilities via notify-send",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := notify.Run(notifyArgs); err != nil {
				return xerrors.Errorf("failed to run call.Run: %w", err)
			}
			return nil
		},
	}

	return cmd
}
