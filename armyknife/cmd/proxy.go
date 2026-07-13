package cmd

import (
	"armyknife/pkg/proxy"

	"github.com/spf13/cobra"
	"golang.org/x/xerrors"
)

func proxyCmd() *cobra.Command {
	proxyArgs := proxy.DefaultArgs()

	cmd := &cobra.Command{
		Use:          "proxy TARGET",
		Short:        "Proxy to a target",
		SilenceUsage: true,
		Args:         cobra.ExactArgs(1),
		PreRun: func(cmd *cobra.Command, args []string) {
			proxyArgs.Target = args[0]
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := proxy.Run(proxyArgs); err != nil {
				return xerrors.Errorf("failed to run proxy.Run: %w", err)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(
		&proxyArgs.Address,
		"address",
		proxyArgs.Address,
		"Address",
	)

	cmd.Flags().IntVar(
		&proxyArgs.MaxConnections,
		"max-connections",
		proxyArgs.MaxConnections,
		"Max connections",
	)

	cmd.Flags().StringVar(
		&proxyArgs.Mode,
		"mode",
		proxyArgs.Mode,
		"Mode",
	)

	return cmd
}
