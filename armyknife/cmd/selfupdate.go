package cmd

import (
	"armyknife/pkg/selfupdate"
	"log"

	"github.com/spf13/cobra"
)

func selfupdateCmd() *cobra.Command {
	selfupdateArgs := selfupdate.DefaultArgs()

	cmd := &cobra.Command{
		Use:          "selfupdate",
		Short:        "Update myself to the latest version",
		SilenceUsage: true,
		Args:         cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			log.SetFlags(0)

			if err := selfupdate.Run(selfupdateArgs); err != nil {
				log.Fatalf("Failed to run serve.Run: %+v", err)
			}
		},
	}

	return cmd
}
