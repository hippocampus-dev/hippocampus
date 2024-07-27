package cmd

import (
	"armyknife/pkg/serve"
	"log"

	"github.com/spf13/cobra"
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
		Run: func(cmd *cobra.Command, args []string) {
			if err := serve.Run(serveArgs); err != nil {
				log.Fatalf("Failed to run serve.Run: %+v", err)
			}
		},
	}

	cmd.Flags().StringVarP(
		&serveArgs.Address,
		"address",
		"",
		serveArgs.Address,
		"Address",
	)

	cmd.Flags().IntVarP(
		&serveArgs.MaxConnections,
		"max-connections",
		"",
		serveArgs.MaxConnections,
		"Max connections",
	)

	return cmd
}
