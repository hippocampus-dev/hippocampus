package oauth

import (
	"github.com/spf13/cobra"
)

func GetRootCmd(args []string) *cobra.Command {
	cmd := &cobra.Command{
		Use:          "oauth",
		Short:        "OAuth utilities",
		SilenceUsage: true,
	}

	cmd.AddCommand(deviceCmd())
	cmd.AddCommand(pkceCmd())

	return cmd
}
