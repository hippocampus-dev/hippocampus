package cmd

import (
	"exactly-one-pod-hook/internal/lock"
	"github.com/spf13/cobra"
)

var Args = lock.DefaultArgs()

func GetRootCmd(args []string) *cobra.Command {
	cmd := &cobra.Command{
		Use:          "exactly-one-pod-hook",
		Short:        "",
		SilenceUsage: true,
	}

	cmd.SetArgs(args)

	cmd.AddCommand(sidecarCmd())
	cmd.AddCommand(webhookCmd())

	cmd.PersistentFlags().StringVar(
		&Args.LockMode,
		"lock-mode",
		Args.LockMode,
		"The mode of the lock",
	)

	cmd.PersistentFlags().StringSliceVar(
		&Args.RedisAddresses,
		"redis-addresses",
		Args.RedisAddresses,
		"The addresses of the redis servers",
	)

	cmd.PersistentFlags().StringSliceVar(
		&Args.EtcdEndpoints,
		"etcd-endpoints",
		Args.EtcdEndpoints,
		"The endpoints of the etcd servers",
	)

	return cmd
}
