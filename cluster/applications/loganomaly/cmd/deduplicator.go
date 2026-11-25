package cmd

import (
	"loganomaly/pkg/deduplicator"

	"github.com/spf13/cobra"
	"golang.org/x/xerrors"
)

func deduplicatorCmd() *cobra.Command {
	deduplicatorArgs := deduplicator.DefaultArgs()

	cmd := &cobra.Command{
		Use:          "deduplicator",
		SilenceUsage: true,
		Args:         cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := deduplicator.Run(deduplicatorArgs); err != nil {
				return xerrors.Errorf("failed to run deduplicator.Run: %w", err)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(
		&deduplicatorArgs.RedisAddress,
		"redis-address",
		deduplicatorArgs.RedisAddress,
		"Redis server address",
	)

	cmd.Flags().DurationVar(
		&deduplicatorArgs.DedupTTL,
		"dedup-ttl",
		deduplicatorArgs.DedupTTL,
		"Deduplication TTL",
	)

	return cmd
}
