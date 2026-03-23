package cmd

import (
	"armyknife/pkg/device_auth"

	"github.com/spf13/cobra"
	"golang.org/x/xerrors"
)

func deviceAuthCmd() *cobra.Command {
	deviceAuthArgs := device_auth.DefaultArgs()

	cmd := &cobra.Command{
		Use:          "device-auth",
		Short:        "Acquire an OAuth token via device authorization grant (RFC 8628)",
		SilenceUsage: true,
		Args:         cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := device_auth.Run(deviceAuthArgs); err != nil {
				return xerrors.Errorf("failed to run device_auth.Run: %w", err)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(
		&deviceAuthArgs.URL,
		"url",
		deviceAuthArgs.URL,
		"Device flow bridge server URL",
	)

	cmd.Flags().StringVar(
		&deviceAuthArgs.Scope,
		"scope",
		deviceAuthArgs.Scope,
		"OAuth scope to request",
	)

	return cmd
}
