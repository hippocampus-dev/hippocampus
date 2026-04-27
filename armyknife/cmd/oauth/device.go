package oauth

import (
	"armyknife/pkg/oauth/device"

	"github.com/spf13/cobra"
	"golang.org/x/xerrors"
)

func deviceCmd() *cobra.Command {
	deviceArgs := device.DefaultArgs()

	cmd := &cobra.Command{
		Use:          "device",
		Short:        "Acquire an OAuth token via device authorization grant (RFC 8628)",
		SilenceUsage: true,
		Args:         cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := device.Run(deviceArgs); err != nil {
				return xerrors.Errorf("failed to run device.Run: %w", err)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(
		&deviceArgs.URL,
		"url",
		deviceArgs.URL,
		"Authorization server URL",
	)

	cmd.Flags().StringVar(
		&deviceArgs.ClientID,
		"client-id",
		deviceArgs.ClientID,
		"OAuth client ID",
	)

	cmd.Flags().StringVar(
		&deviceArgs.Scope,
		"scope",
		deviceArgs.Scope,
		"OAuth scope to request",
	)

	return cmd
}
