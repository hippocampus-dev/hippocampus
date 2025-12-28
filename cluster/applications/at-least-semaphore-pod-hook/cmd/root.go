package cmd

import (
	"at-least-semaphore-pod-hook/internal/lock"

	"github.com/spf13/cobra"
)

var Args = lock.DefaultArgs()

func GetRootCmd(args []string) *cobra.Command {
	cmd := &cobra.Command{
		Use:          "at-least-semaphore-pod-hook",
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
		&Args.EtcdAddresses,
		"etcd-addresses",
		Args.EtcdAddresses,
		"The addresses of the etcd servers",
	)

	cmd.PersistentFlags().StringVar(
		&Args.QueueRedisAddress,
		"queue-redis-address",
		Args.QueueRedisAddress,
		"The address of the queue redis server",
	)

	return cmd
}
