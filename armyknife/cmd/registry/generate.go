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

	cmd.Flags().StringVar(
		&generateArgs.GrafanaURL,
		"grafana-url",
		"https://grafana.minikube.127.0.0.1.nip.io",
		"Grafana base URL for generating Explore links",
	)
	cmd.Flags().StringVar(
		&generateArgs.LokiDatasourceUID,
		"loki-datasource-uid",
		"loki",
		"Loki datasource UID for log Explore links",
	)
	cmd.Flags().StringVar(
		&generateArgs.TempoDatasourceUID,
		"tempo-datasource-uid",
		"tempo",
		"Tempo datasource UID for trace Explore links",
	)
	cmd.Flags().StringVar(
		&generateArgs.PyroscopeDatasourceUID,
		"pyroscope-datasource-uid",
		"pyroscope",
		"Pyroscope datasource UID for profiling Explore links",
	)

	return cmd
}
