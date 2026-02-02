package registry

import (
	"github.com/spf13/cobra"
)

func GetRootCmd(args []string) *cobra.Command {
	cmd := &cobra.Command{
		Use:          "registry",
		Short:        "Observability registry management for Kubernetes manifests",
		SilenceUsage: true,
	}

	cmd.AddCommand(generateCmd())

	return cmd
}
