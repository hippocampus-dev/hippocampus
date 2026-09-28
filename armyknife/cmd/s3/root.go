package s3

import (
	"github.com/spf13/cobra"
)

func GetRootCmd(args []string) *cobra.Command {
	cmd := &cobra.Command{
		Use:          "s3",
		Short:        "S3 utilities",
		SilenceUsage: true,
	}

	cmd.AddCommand(viewerCmd())

	return cmd
}
