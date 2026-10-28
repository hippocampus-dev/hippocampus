package rails

import (
	"armyknife/internal/rails/command"
	"armyknife/pkg/rails/credentials_diff"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"golang.org/x/xerrors"
)

func credentialsDiffCmd() *cobra.Command {
	diffArgs := credentials_diff.DefaultArgs()

	cmd := &cobra.Command{
		Use:          "credentials:diff [FILE]",
		Short:        "",
		SilenceUsage: true,
		Args:         cobra.MaximumNArgs(1),
		PreRun: func(cmd *cobra.Command, args []string) {
			if len(args) > 0 {
				diffArgs.File = args[0]
			}

			ensureEncryptionKeyHasBeenAdded()
			ensureDiffingDriverIsConfigured()

			if diffArgs.MasterKey == "" {
				cwd, err := os.Getwd()
				if err != nil {
					return
				}
				root, err := command.Root(cwd)
				if err != nil {
					return
				}
				path := filepath.Join(*root, masterKeyFile)
				b, err := os.ReadFile(path)
				if err == nil {
					diffArgs.MasterKey = string(b)
				}
			}
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := credentials_diff.Run(diffArgs); err != nil {
				return xerrors.Errorf("failed to run diff.Run: %w", err)
			}
			return nil
		},
	}

	cmd.Flags().BoolVar(
		&diffArgs.Enroll,
		"enroll",
		diffArgs.Enroll,
		"Enroll project in credentials file diffing with `git diff`",
	)

	cmd.Flags().BoolVar(
		&diffArgs.Disenroll,
		"disenroll",
		diffArgs.Disenroll,
		"Disenroll project from credentials file diffing",
	)

	return cmd
}
