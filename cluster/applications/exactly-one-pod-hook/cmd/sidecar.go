package cmd

import (
	"exactly-one-pod-hook/pkg/sidecar"
	"github.com/spf13/cobra"
	"log"
)

func sidecarCmd() *cobra.Command {
	sidecarArgs := sidecar.DefaultArgs()

	cmd := &cobra.Command{
		Use:          "sidecar UNLOCK_KEY UNLOCK_VALUE",
		Short:        "Start the sidecar to unlock the key when receiving SIGTERM",
		SilenceUsage: true,
		Args:         cobra.ExactArgs(2),
		Run: func(cmd *cobra.Command, args []string) {
			sidecarArgs.UnlockKey = args[0]
			sidecarArgs.UnlockValue = args[1]
			sidecarArgs.Args = Args
			if err := sidecar.Run(sidecarArgs); err != nil {
				log.Fatalf("Failed to run sidecar.Run: %+v", err)
			}
		},
	}

	return cmd
}
