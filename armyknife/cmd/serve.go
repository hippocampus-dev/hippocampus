package cmd

import (
	"armyknife/pkg/serve"

	"github.com/spf13/cobra"
	"golang.org/x/xerrors"
)

func serveCmd() *cobra.Command {
	serveArgs := serve.DefaultArgs()

	cmd := &cobra.Command{
		Use:          "serve DIRECTORY",
		Short:        "Serve a current directory",
		SilenceUsage: true,
		Args:         cobra.ExactArgs(1),
		PreRun: func(cmd *cobra.Command, args []string) {
			serveArgs.Directory = args[0]
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := serve.Run(serveArgs); err != nil {
				return xerrors.Errorf("failed to run serve.Run: %w", err)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(
		&serveArgs.Address,
		"address",
		serveArgs.Address,
		"Address",
	)

	cmd.Flags().IntVar(
		&serveArgs.MaxConnections,
		"max-connections",
		serveArgs.MaxConnections,
		"Max connections",
	)

	return cmd
}
