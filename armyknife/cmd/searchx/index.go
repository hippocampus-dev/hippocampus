package searchx

import (
	"armyknife/pkg/searchx/index"
	"strings"

	"github.com/spf13/cobra"
	"golang.org/x/xerrors"
)

func indexCmd() *cobra.Command {
	indexArgs := index.DefaultArgs()
	var serverFlags []string

	cmd := &cobra.Command{
		Use:          "index --server LANGUAGE=ADDRESS...",
		Short:        "Index symbols from LSP servers into vector database",
		SilenceUsage: true,
		PreRunE: func(cmd *cobra.Command, args []string) error {
			indexArgs.LSPServers = make(map[string]string)
			for _, serverFlag := range serverFlags {
				parts := strings.SplitN(serverFlag, "=", 2)
				if len(parts) != 2 {
					return xerrors.Errorf("invalid server format %q, expected LANGUAGE=ADDRESS", serverFlag)
				}
				language := parts[0]
				address := parts[1]
				indexArgs.LSPServers[language] = address
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := index.Run(indexArgs); err != nil {
				return xerrors.Errorf("failed to run index.Run: %w", err)
			}
			return nil
		},
	}

	cmd.Flags().StringArrayVarP(
		&serverFlags,
		"server",
		"s",
		nil,
		"LSP server in LANGUAGE=ADDRESS format (e.g., python=127.0.0.1:2089)",
	)
	_ = cmd.MarkFlagRequired("server")

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
