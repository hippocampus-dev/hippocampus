package rails

import (
	"armyknife/internal/rails/command"
	"armyknife/internal/rails/generators"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/spf13/cobra"
)

const masterKeyFile = "config/master.key"

func GetRootCmd(args []string) *cobra.Command {
	cmd := &cobra.Command{
		Use:          "rails",
		Short:        "Rails compatible utilities",
		SilenceUsage: true,
	}

	cmd.AddCommand(credentialsDiffCmd())
	cmd.AddCommand(credentialsEditCmd())
	cmd.AddCommand(credentialsShowCmd())

	return cmd
}

func ensureEncryptionKeyHasBeenAdded() {
	cwd, err := os.Getwd()
	if err != nil {
		return
	}
	root, err := command.Root(cwd)
	if err != nil {
		return
	}
	path := filepath.Join(*root, masterKeyFile)
	if _, err := os.Stat(path); err != nil {
		generator := generators.NewEncryptionKeyFileGenerator()
		if err := generator.AddKeyFile(masterKeyFile); err != nil {
			return
		}
		if err := generator.IgnoreKeyFile(masterKeyFile); err != nil {
			return
		}
	}
}

func ensureDiffingDriverIsConfigured() {
	if !diffingDriverConfigured() {
		configureDiffingDriver()
	}
}

func diffingDriverConfigured() bool {
	if _, err := exec.Command("git", "config", "--get", "diff.armyknife_rails_credentials.textconv").Output(); err != nil {
		return false
	}
	return true
}

func configureDiffingDriver() {
	_ = exec.Command("git", "config", "diff.armyknife_rails_credentials.textconv", "armyknife rails credentials:diff").Run()
}
