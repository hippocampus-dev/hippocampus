package grpc

import (
	"armyknife/pkg/grpc/catch"
	"log"

	"github.com/spf13/cobra"
)

func catchCmd() *cobra.Command {
	catchArgs := catch.DefaultArgs()

	cmd := &cobra.Command{
		Use:          "catch",
		Short:        "",
		SilenceUsage: true,
		Args:         cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			if err := catch.Run(catchArgs); err != nil {
				log.Fatalf("Failed to run catch.Run: %+v", err)
			}
		},
	}

	cmd.Flags().StringVarP(
		&catchArgs.Address,
		"address",
		"",
		catchArgs.Address,
		"Address to use a gRPC server",
	)

	cmd.Flags().StringVarP(
		&catchArgs.Pattern,
		"pattern",
		"p",
		catchArgs.Pattern,
		"Glob pattern for source proto files",
	)

	cmd.Flags().StringSliceVarP(
		&catchArgs.ImportPaths,
		"imports-paths",
		"I",
		catchArgs.ImportPaths,
		"Import paths for reading proto files",
	)

	return cmd
}
