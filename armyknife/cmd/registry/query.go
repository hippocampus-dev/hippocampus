package registry

import (
	"armyknife/pkg/registry/query"

	"github.com/spf13/cobra"
	"golang.org/x/xerrors"
)

func queryCmd() *cobra.Command {
	queryArgs := query.DefaultArgs()

	cmd := &cobra.Command{
		Use:          "query DIRECTORY",
		Short:        "Query observability data from Grafana using .registry.yaml",
		SilenceUsage: true,
		Args:         cobra.ExactArgs(1),
		PreRun: func(cmd *cobra.Command, args []string) {
			queryArgs.Directory = args[0]
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := query.Run(queryArgs); err != nil {
				return xerrors.Errorf("failed to run query.Run: %w", err)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(
		&queryArgs.GrafanaURL,
		"grafana-url",
		queryArgs.GrafanaURL,
		"Grafana base URL",
	)
	cmd.Flags().UintVar(
		&queryArgs.AuthorizationListenPort,
		"authorization-listen-port",
		queryArgs.AuthorizationListenPort,
		"Port to listen for authorization callback",
	)
	cmd.Flags().StringVar(
		&queryArgs.PrometheusDatasourceUID,
		"prometheus-datasource-uid",
		queryArgs.PrometheusDatasourceUID,
		"Prometheus datasource UID",
	)
	cmd.Flags().StringVar(
		&queryArgs.LokiDatasourceUID,
		"loki-datasource-uid",
		queryArgs.LokiDatasourceUID,
		"Loki datasource UID",
	)
	cmd.Flags().StringVar(
		&queryArgs.TempoDatasourceUID,
		"tempo-datasource-uid",
		queryArgs.TempoDatasourceUID,
		"Tempo datasource UID",
	)
	cmd.Flags().StringVar(
		&queryArgs.PyroscopeDatasourceUID,
		"pyroscope-datasource-uid",
		queryArgs.PyroscopeDatasourceUID,
		"Pyroscope datasource UID",
	)
	cmd.Flags().DurationVar(
		&queryArgs.From,
		"from",
		queryArgs.From,
		"Duration lookback from now",
	)
	cmd.Flags().DurationVar(
		&queryArgs.To,
		"to",
		queryArgs.To,
		"Duration offset from now for end time",
	)
	cmd.Flags().DurationVar(
		&queryArgs.Step,
		"step",
		queryArgs.Step,
		"Prometheus query step interval",
	)
	cmd.Flags().StringSliceVar(
		&queryArgs.Signals,
		"signals",
		queryArgs.Signals,
		"Signal types to query (metrics,logs,traces,profiling)",
	)

	return cmd
}
