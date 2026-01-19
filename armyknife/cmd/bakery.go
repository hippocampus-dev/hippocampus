package cmd

import (
	"armyknife/pkg/bakery"

	"github.com/spf13/cobra"
	"golang.org/x/xerrors"
)

func bakeryCmd() *cobra.Command {
	bakeryArgs := bakery.DefaultArgs()

	cmd := &cobra.Command{
		Use:          "bakery COOKIE_NAME",
		Short:        "Get a cookie from the bakery",
		SilenceUsage: true,
		Args:         cobra.ExactArgs(1),
		PreRun: func(cmd *cobra.Command, args []string) {
			bakeryArgs.CookieName = args[0]
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := bakery.Run(bakeryArgs); err != nil {
				return xerrors.Errorf("failed to run bakery.Run: %w", err)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(
		&bakeryArgs.URL,
		"url",
		bakeryArgs.URL,
		"Bakery callback URL",
	)

	cmd.Flags().UintVarP(
		&bakeryArgs.ListenPort,
		"listen-port",
		"p",
		bakeryArgs.ListenPort,
		"The port to listen on for the bakery authorization callback",
	)

	return cmd
}
