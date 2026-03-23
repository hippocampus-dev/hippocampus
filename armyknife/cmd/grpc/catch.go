package grpc

import (
	"armyknife/pkg/grpc/catch"

	"github.com/spf13/cobra"
	"golang.org/x/xerrors"
)

func catchCmd() *cobra.Command {
	catchArgs := catch.DefaultArgs()

	cmd := &cobra.Command{
		Use:          "catch",
		Short:        "",
		SilenceUsage: true,
		Args:         cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := catch.Run(catchArgs); err != nil {
				return xerrors.Errorf("failed to run catch.Run: %w", err)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(
		&catchArgs.Address,
		"address",
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
