package searchx

import (
	"github.com/spf13/cobra"
)

func GetRootCmd(args []string) *cobra.Command {
	cmd := &cobra.Command{
		Use:          "searchx",
		Short:        "LSP-based symbol indexing and search utilities",
		SilenceUsage: true,
	}

	cmd.AddCommand(indexCmd())
	cmd.AddCommand(queryCmd())

	return cmd
}
