package cmd

import (
	"armyknife/pkg/completion"
	"log"

	"github.com/spf13/cobra"
)

func completionCmd() *cobra.Command {
	completionArgs := completion.DefaultArgs()

	cmd := &cobra.Command{
		Use:          "completion SHELL",
		Short:        "Show a version",
		SilenceUsage: true,
		Args:         cobra.ExactArgs(1),
		PreRun: func(cmd *cobra.Command, args []string) {
			completionArgs.Shell = args[0]
		},
		Run: func(cmd *cobra.Command, args []string) {
			log.SetFlags(0)

			if err := completion.Run(completionArgs); err != nil {
				log.Fatalf("Failed to run completion.Run: %+v", err)
			}
		},
	}

	return cmd
}
