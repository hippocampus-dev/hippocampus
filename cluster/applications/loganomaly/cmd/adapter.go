package cmd

import (
	"loganomaly/pkg/adapter"

	"github.com/spf13/cobra"
	"golang.org/x/xerrors"
)

func adapterCmd() *cobra.Command {
	adapterArgs := adapter.DefaultArgs()

	cmd := &cobra.Command{
		Use:          "adapter",
		SilenceUsage: true,
		Args:         cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := adapter.Run(adapterArgs); err != nil {
				return xerrors.Errorf("failed to run adapter.Run: %w", err)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(
		&adapterArgs.GrafanaBase,
		"grafana-base",
		adapterArgs.GrafanaBase,
		"Grafana base URL",
	)

	return cmd
}
