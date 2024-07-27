package cmd

import (
	"armyknife/pkg/echo"
	"log"

	"github.com/spf13/cobra"
)

func echoCmd() *cobra.Command {
	echoArgs := echo.DefaultArgs()

	cmd := &cobra.Command{
		Use:          "echo",
		Short:        "Start echo server",
		SilenceUsage: true,
		Args:         cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			if err := echo.Run(echoArgs); err != nil {
				log.Fatalf("Failed to run echo.Run: %+v", err)
			}
		},
	}

	cmd.Flags().StringVarP(
		&echoArgs.Address,
		"address",
		"",
		echoArgs.Address,
		"Address",
	)

	cmd.Flags().IntVarP(
		&echoArgs.MaxConnections,
		"max-connections",
		"",
		echoArgs.MaxConnections,
		"Max connections",
	)

	cmd.Flags().IntVarP(
		&echoArgs.TerminationGracePeriodSeconds,
		"termination-grace-period-seconds",
		"",
		echoArgs.TerminationGracePeriodSeconds,
		"Termination grace period seconds",
	)

	cmd.Flags().IntVarP(
		&echoArgs.Lameduck,
		"lameduck",
		"",
		echoArgs.Lameduck,
		"Lameduck",
	)

	cmd.Flags().BoolVarP(
		&echoArgs.Keepalive,
		"keepalive",
		"",
		echoArgs.Keepalive,
		"Keepalive",
	)

	return cmd
}
