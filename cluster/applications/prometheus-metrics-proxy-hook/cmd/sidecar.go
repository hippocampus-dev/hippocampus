package cmd

import (
	"prometheus-metrics-proxy-hook/pkg/sidecar"

	"github.com/spf13/cobra"
	"golang.org/x/xerrors"
)

func sidecarCmd() *cobra.Command {
	sidecarArgs := sidecar.DefaultArgs()

	cmd := &cobra.Command{
		Use:          "sidecar TARGET_URL",
		Short:        "Start the metrics proxy sidecar",
		SilenceUsage: true,
		Args:         cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			sidecarArgs.TargetURL = args[0]

			if err := sidecar.Run(sidecarArgs); err != nil {
				return xerrors.Errorf("failed to run sidecar.Run: %w", err)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(
		&sidecarArgs.Address,
		"address",
		sidecarArgs.Address,
		"Address to listen on",
	)

	cmd.Flags().DurationVar(
		&sidecarArgs.TerminationGracePeriod,
		"termination-grace-period",
		sidecarArgs.TerminationGracePeriod,
		"The duration the application needs to terminate gracefully",
	)

	cmd.Flags().DurationVar(
		&sidecarArgs.Lameduck,
		"lameduck",
		sidecarArgs.Lameduck,
		"A period that explicitly asks clients to stop sending requests",
	)

	return cmd
}
