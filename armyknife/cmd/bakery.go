package cmd

import (
	"armyknife/pkg/bakery"
	"log"

	"github.com/spf13/cobra"
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
		Run: func(cmd *cobra.Command, args []string) {
			if err := bakery.Run(bakeryArgs); err != nil {
				log.Fatalf("Failed to run bakery.Run: %+v", err)
			}
		},
	}

	cmd.Flags().StringVarP(
		&bakeryArgs.URL,
		"url",
		"",
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
