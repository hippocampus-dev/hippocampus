package cmd

import (
	"armyknife/pkg/deviceauth"

	"github.com/spf13/cobra"
	"golang.org/x/xerrors"
)

func deviceAuthCmd() *cobra.Command {
	deviceAuthArgs := deviceauth.DefaultArgs()

	cmd := &cobra.Command{
		Use:          "deviceauth",
		Short:        "Acquire an OAuth token via device authorization grant (RFC 8628)",
		SilenceUsage: true,
		Args:         cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := deviceauth.Run(deviceAuthArgs); err != nil {
				return xerrors.Errorf("failed to run deviceauth.Run: %w", err)
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
		&deviceAuthArgs.ClientID,
		"client-id",
		deviceAuthArgs.ClientID,
		"OAuth client ID",
	)

	cmd.Flags().StringVar(
		&deviceAuthArgs.Scope,
		"scope",
		deviceAuthArgs.Scope,
		"OAuth scope to request",
	)

	return cmd
}
