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
		PreRun: func(cmd *cobra.Command, args []string) {
			sidecarArgs.TargetURL = args[0]
		},
		RunE: func(cmd *cobra.Command, args []string) error {
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

	cmd.Flags().DurationVar(
		&sidecarArgs.ScrapeWaitThreshold,
		"scrape-wait-threshold",
		sidecarArgs.ScrapeWaitThreshold,
		"Wait for next scrape if time since last scrape exceeds this threshold",
	)

	cmd.Flags().BoolVar(
		&sidecarArgs.Keepalive,
		"keepalive",
		sidecarArgs.Keepalive,
		"Enable HTTP keep-alive",
	)

	return cmd
}
