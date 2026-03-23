package cmd

import (
	"loganomaly/pkg/consumer"

	"github.com/spf13/cobra"
	"golang.org/x/xerrors"
)

func consumerCmd() *cobra.Command {
	consumerArgs := consumer.DefaultArgs()

	cmd := &cobra.Command{
		Use:          "consumer",
		SilenceUsage: true,
		Args:         cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := consumer.Run(consumerArgs); err != nil {
				return xerrors.Errorf("failed to run consumer.Run: %w", err)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(
		&consumerArgs.BootstrapServers,
		"bootstrap-servers",
		consumerArgs.BootstrapServers,
		"Kafka bootstrap servers",
	)

	cmd.Flags().StringVar(
		&consumerArgs.InputTopic,
		"input-topic",
		consumerArgs.InputTopic,
		"Input Kafka topic",
	)

	cmd.Flags().StringVar(
		&consumerArgs.OutputTopic,
		"output-topic",
		consumerArgs.OutputTopic,
		"Output Kafka topic",
	)

	cmd.Flags().Float64Var(
		&consumerArgs.ZScoreThreshold,
		"z-score-threshold",
		consumerArgs.ZScoreThreshold,
		"Z-score threshold for anomaly detection",
	)

	cmd.Flags().DurationVar(
		&consumerArgs.EvaluationInterval,
		"evaluation-interval",
		consumerArgs.EvaluationInterval,
		"Window evaluation interval",
	)

	cmd.Flags().DurationVar(
		&consumerArgs.SuppressionDuration,
		"suppression-duration",
		consumerArgs.SuppressionDuration,
		"Immediate-fire dedup suppression duration",
	)

	return cmd
}
