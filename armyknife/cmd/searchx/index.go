package searchx

import (
	"armyknife/pkg/searchx/index"

	"github.com/spf13/cobra"
	"golang.org/x/xerrors"
)

func indexCmd() *cobra.Command {
	indexArgs := index.DefaultArgs()

	cmd := &cobra.Command{
		Use:          "index LSP_SERVER_ADDRESSES...",
		Short:        "Index symbols from LSP servers into vector database",
		SilenceUsage: true,
		Args:         cobra.MinimumNArgs(1),
		PreRun: func(cmd *cobra.Command, args []string) {
			indexArgs.LSPServers = args
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := index.Run(indexArgs); err != nil {
				return xerrors.Errorf("failed to run index.Run: %w", err)
			}
			return nil
		},
	}

	cmd.Flags().StringVarP(
		&indexArgs.Output,
		"database",
		"d",
		indexArgs.Output,
		"Path to vector database",
	)

	cmd.Flags().IntVar(
		&indexArgs.Dimension,
		"dimension",
		indexArgs.Dimension,
		"Dimension of the vector embeddings",
	)

	cmd.Flags().StringVar(
		&indexArgs.EmbeddingModel,
		"embedding-model",
		indexArgs.EmbeddingModel,
		"Name of the embedding model to use",
	)

	cmd.Flags().UintVar(
		&indexArgs.AuthorizationListenPort,
		"authorization-listen-port",
		indexArgs.AuthorizationListenPort,
		"Port to listen for authorization callback",
	)

	return cmd
}
