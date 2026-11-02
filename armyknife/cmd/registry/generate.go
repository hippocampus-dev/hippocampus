package registry

import (
	"armyknife/pkg/registry/generate"

	"github.com/spf13/cobra"
	"golang.org/x/xerrors"
)

func generateCmd() *cobra.Command {
	generateArgs := generate.DefaultArgs()

	cmd := &cobra.Command{
		Use:          "generate DIRECTORY",
		Short:        "Generate .registry.yaml from Kubernetes manifests",
		SilenceUsage: true,
		Args:         cobra.ExactArgs(1),
		PreRun: func(cmd *cobra.Command, args []string) {
			generateArgs.ManifestPath = args[0]
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := generate.Run(generateArgs); err != nil {
				return xerrors.Errorf("failed to run generate.Run: %w", err)
			}
			return nil
		},
	}

	cmd.Flags().StringVarP(
		&generateArgs.OutputPath,
		"output",
		"o",
		"",
		"Output file path (default: <manifest>/.registry.yaml)",
	)

	cmd.Flags().StringVar(
		&generateArgs.GrafanaURL,
		"grafana-url",
		generateArgs.GrafanaURL,
		"Grafana base URL for generating Explore links",
	)
	cmd.Flags().StringVar(
		&generateArgs.LokiDatasourceUID,
		"loki-datasource-uid",
		generateArgs.LokiDatasourceUID,
		"Loki datasource UID for log Explore links",
	)
	cmd.Flags().StringVar(
		&generateArgs.TempoDatasourceUID,
		"tempo-datasource-uid",
		generateArgs.TempoDatasourceUID,
		"Tempo datasource UID for trace Explore links",
	)
	cmd.Flags().StringVar(
		&generateArgs.PyroscopeDatasourceUID,
		"pyroscope-datasource-uid",
		generateArgs.PyroscopeDatasourceUID,
		"Pyroscope datasource UID for profiling Explore links",
	)

	return cmd
}
