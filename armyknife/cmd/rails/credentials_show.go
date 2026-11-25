package rails

import (
	"armyknife/internal/rails/command"
	"armyknife/pkg/rails/credentials_show"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"golang.org/x/xerrors"
)

func credentialsShowCmd() *cobra.Command {
	showArgs := credentials_show.DefaultArgs()

	cmd := &cobra.Command{
		Use:          "credentials:show [FILE]",
		Short:        "",
		SilenceUsage: true,
		Args:         cobra.MaximumNArgs(1),
		PreRun: func(cmd *cobra.Command, args []string) {
			if len(args) > 0 {
				showArgs.File = args[0]
			}

			ensureEncryptionKeyHasBeenAdded()
			ensureDiffingDriverIsConfigured()

			if showArgs.MasterKey == "" {
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
					showArgs.MasterKey = string(b)
				}
			}
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := credentials_show.Run(showArgs); err != nil {
				return xerrors.Errorf("failed to run show.Run: %w", err)
			}
			return nil
		},
	}

	return cmd
}
