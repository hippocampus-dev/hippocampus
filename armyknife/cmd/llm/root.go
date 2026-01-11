package llm

import (
	"github.com/spf13/cobra"
)

func GetRootCmd(args []string) *cobra.Command {
	cmd := &cobra.Command{
		Use:          "llm",
		Short:        "LLM utilities",
		SilenceUsage: true,
	}

	cmd.AddCommand(fillcsvCmd())

	return cmd
}
