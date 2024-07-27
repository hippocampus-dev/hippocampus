package grpc

import (
	"armyknife/pkg/grpc/call"
	"log"

	"github.com/spf13/cobra"
)

func callCmd() *cobra.Command {
	callArgs := call.DefaultArgs()

	cmd := &cobra.Command{
		Use:          "call ADDRESS ENDPOINT BODY",
		Short:        "",
		SilenceUsage: true,
		Args:         cobra.ExactArgs(3),
		PreRun: func(cmd *cobra.Command, args []string) {
			callArgs.Address = args[0]
			callArgs.Endpoint = args[1]
			callArgs.Body = args[2]
		},
		Run: func(cmd *cobra.Command, args []string) {
			log.SetFlags(0)

			if err := call.Run(callArgs); err != nil {
				log.Fatalf("Failed to run call.Run: %+v", err)
			}
		},
	}

	cmd.Flags().StringVarP(
		&callArgs.Pattern,
		"pattern",
		"p",
		callArgs.Pattern,
		"Glob pattern for source proto files",
	)

	cmd.Flags().StringSliceVarP(
		&callArgs.ImportPaths,
		"imports-paths",
		"I",
		callArgs.ImportPaths,
		"Import paths for reading proto files",
	)

	return cmd
}
