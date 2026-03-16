package searchx

import (
	"armyknife/pkg/searchx/query"

	"github.com/spf13/cobra"
	"golang.org/x/xerrors"
)

func queryCmd() *cobra.Command {
	queryArgs := query.DefaultArgs()

	cmd := &cobra.Command{
		Use:          "query QUERY_TEXT",
		Short:        "Query indexed symbols from vector database",
		SilenceUsage: true,
		Args:         cobra.ExactArgs(1),
		PreRun: func(cmd *cobra.Command, args []string) {
			queryArgs.Query = args[0]
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := query.Run(queryArgs); err != nil {
				return xerrors.Errorf("failed to run query.Run: %w", err)
			}
			return nil
		},
	}

	cmd.Flags().StringVarP(
		&queryArgs.Database,
		"database",
		"d",
		queryArgs.Database,
		"Path to vector database",
	)

	cmd.Flags().IntVarP(
		&queryArgs.Limit,
		"limit",
		"l",
		queryArgs.Limit,
		"Maximum number of results to return",
	)

	cmd.Flags().IntVar(
		&queryArgs.Dimension,
		"dimension",
		queryArgs.Dimension,
		"Dimension of the vector embeddings",
	)

	cmd.Flags().StringVar(
		&queryArgs.EmbeddingModel,
		"embedding-model",
		queryArgs.EmbeddingModel,
		"Name of the embedding model to use",
	)

	cmd.Flags().UintVar(
		&queryArgs.AuthorizationListenPort,
		"authorization-listen-port",
		queryArgs.AuthorizationListenPort,
		"Port to listen for authorization callback",
	)

	return cmd
}
