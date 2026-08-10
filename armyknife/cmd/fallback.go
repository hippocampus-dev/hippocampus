package cmd

import (
	"armyknife/pkg/fallback"

	"github.com/spf13/cobra"
	"golang.org/x/xerrors"
)

func fallbackCmd() *cobra.Command {
	fallbackArgs := fallback.DefaultArgs()

	cmd := &cobra.Command{
		Use:          "fallback TARGET FALLBACK",
		Short:        "Fallback to a FALLBACK if TARGET is unreachable",
		SilenceUsage: true,
		Args:         cobra.ExactArgs(2),
		PreRun: func(cmd *cobra.Command, args []string) {
			fallbackArgs.Target = args[0]
			fallbackArgs.Fallback = args[1]
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := fallback.Run(fallbackArgs); err != nil {
				return xerrors.Errorf("failed to run fallback.Run: %w", err)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(
		&fallbackArgs.Address,
		"address",
		fallbackArgs.Address,
		"Address",
	)

	cmd.Flags().IntVar(
		&fallbackArgs.MaxConnections,
		"max-connections",
		fallbackArgs.MaxConnections,
		"Max connections",
	)

	return cmd
}
