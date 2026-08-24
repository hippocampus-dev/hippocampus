package cmd

import (
	"armyknife/pkg/selfupdate"

	"github.com/spf13/cobra"
	"golang.org/x/xerrors"
)

func selfupdateCmd() *cobra.Command {
	selfupdateArgs := selfupdate.DefaultArgs()

	cmd := &cobra.Command{
		Use:          "selfupdate",
		Short:        "Update myself to the latest version",
		SilenceUsage: true,
		Args:         cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := selfupdate.Run(selfupdateArgs); err != nil {
				return xerrors.Errorf("failed to run selfupdate.Run: %w", err)
			}
			return nil
		},
	}

	return cmd
}
