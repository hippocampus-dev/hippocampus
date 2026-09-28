package cmd

import (
	"exactly-one-pod-hook/pkg/sidecar"

	"github.com/spf13/cobra"
	"golang.org/x/xerrors"
)

func sidecarCmd() *cobra.Command {
	sidecarArgs := sidecar.DefaultArgs()

	cmd := &cobra.Command{
		Use:          "sidecar UNLOCK_KEY UNLOCK_VALUE",
		Short:        "Start the sidecar to unlock the key when receiving SIGTERM",
		SilenceUsage: true,
		Args:         cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			sidecarArgs.UnlockKey = args[0]
			sidecarArgs.UnlockValue = args[1]
			sidecarArgs.Args = Args
			if err := sidecar.Run(sidecarArgs); err != nil {
				return xerrors.Errorf("failed to run sidecar.Run: %w", err)
			}
			return nil
		},
	}

	cmd.Flags().IntVar(
		&sidecarArgs.TerminationGracePeriodSeconds,
		"termination-grace-period-seconds",
		sidecarArgs.TerminationGracePeriodSeconds,
		"The duration in seconds the application needs to terminate gracefully",
	)

	return cmd
}
