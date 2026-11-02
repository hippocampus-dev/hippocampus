package rails

import (
	"armyknife/internal/rails/command"
	"armyknife/pkg/rails/credentials_edit"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"golang.org/x/xerrors"
)

func credentialsEditCmd() *cobra.Command {
	editArgs := credentials_edit.DefaultArgs()

	cmd := &cobra.Command{
		Use:          "credentials:edit [FILE]",
		Short:        "",
		SilenceUsage: true,
		Args:         cobra.MaximumNArgs(1),
		PreRun: func(cmd *cobra.Command, args []string) {
			if len(args) > 0 {
				editArgs.File = args[0]
			}

			ensureEncryptionKeyHasBeenAdded()
			ensureDiffingDriverIsConfigured()

			if editArgs.MasterKey == "" {
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
					editArgs.MasterKey = string(b)
				}
			}
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := credentials_edit.Run(editArgs); err != nil {
				return xerrors.Errorf("failed to run edit.Run: %w", err)
			}
			return nil
		},
	}

	return cmd
}
