package cmd

import (
	"armyknife/pkg/echo"

	"github.com/spf13/cobra"
	"golang.org/x/xerrors"
)

func echoCmd() *cobra.Command {
	echoArgs := echo.DefaultArgs()

	cmd := &cobra.Command{
		Use:          "echo",
		Short:        "Start echo server",
		SilenceUsage: true,
		Args:         cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := echo.Run(echoArgs); err != nil {
				return xerrors.Errorf("failed to run echo.Run: %w", err)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(
		&echoArgs.Address,
		"address",
		echoArgs.Address,
		"Address",
	)

	cmd.Flags().IntVar(
		&echoArgs.MaxConnections,
		"max-connections",
		echoArgs.MaxConnections,
		"Max connections",
	)

	cmd.Flags().IntVar(
		&echoArgs.TerminationGracePeriodSeconds,
		"termination-grace-period-seconds",
		echoArgs.TerminationGracePeriodSeconds,
		"Termination grace period seconds",
	)

	cmd.Flags().IntVar(
		&echoArgs.Lameduck,
		"lameduck",
		echoArgs.Lameduck,
		"Lameduck",
	)

	cmd.Flags().BoolVar(
		&echoArgs.Keepalive,
		"keepalive",
		echoArgs.Keepalive,
		"Keepalive",
	)

	return cmd
}
