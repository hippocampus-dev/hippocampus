package cmd

import (
	"armyknife/pkg/completion"

	"github.com/spf13/cobra"
	"golang.org/x/xerrors"
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
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := completion.Run(completionArgs); err != nil {
				return xerrors.Errorf("failed to run completion.Run: %w", err)
			}
			return nil
		},
	}

	return cmd
}
