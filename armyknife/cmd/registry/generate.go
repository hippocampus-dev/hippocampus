package registry

import (
	"armyknife/pkg/registry"

	"github.com/spf13/cobra"
	"golang.org/x/xerrors"
)

func generateCmd() *cobra.Command {
	generateArgs := registry.DefaultArgs()

	cmd := &cobra.Command{
		Use:          "generate",
		Short:        "Generate .registry.yaml from Kubernetes manifests",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := registry.Run(generateArgs); err != nil {
				return xerrors.Errorf("failed to run registry.Run: %w", err)
			}
			return nil
		},
	}

	cmd.Flags().StringVarP(
		&generateArgs.ManifestPath,
		"manifest",
		"m",
		"",
		"Path to manifest directory (required)",
	)
	_ = cmd.MarkFlagRequired("manifest")

	cmd.Flags().StringVarP(
		&generateArgs.OutputPath,
		"output",
		"o",
		"",
		"Output file path (default: <manifest>/.registry.yaml)",
	)

	cmd.Flags().BoolVar(
		&generateArgs.Stdout,
		"stdout",
		false,
		"Output to stdout instead of file",
	)

	return cmd
}
