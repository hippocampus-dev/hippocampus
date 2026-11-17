package cmd

import (
	"armyknife/pkg/egosearch"

	"github.com/spf13/cobra"
	"golang.org/x/xerrors"
)

func egosearchCmd() *cobra.Command {
	egosearchArgs := egosearch.DefaultArgs()

	cmd := &cobra.Command{
		Use:          "egosearch",
		Short:        "",
		Example:      "egosearch | xargs -r -L1 tail -f",
		SilenceUsage: true,
		Args:         cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := egosearch.Run(egosearchArgs); err != nil {
				return xerrors.Errorf("failed to run egosearch.Run: %w", err)
			}
			return nil
		},
	}

	cmd.PersistentFlags().StringSliceVarP(
		&egosearchArgs.Keywords,
		"keywords",
		"k",
		egosearchArgs.Keywords,
		"",
	)

	cmd.PersistentFlags().StringVarP(
		&egosearchArgs.InvertMatch,
		"invert-match",
		"v",
		egosearchArgs.InvertMatch,
		"",
	)

	cmd.PersistentFlags().DurationVar(
		&egosearchArgs.Interval,
		"interval",
		egosearchArgs.Interval,
		"",
	)

	cmd.PersistentFlags().IntVarP(
		&egosearchArgs.Count,
		"count",
		"c",
		egosearchArgs.Count,
		"",
	)

	cmd.PersistentFlags().StringVarP(
		&egosearchArgs.SlackToken,
		"token",
		"",
		egosearchArgs.SlackToken,
		"",
	)

	if f := cmd.PersistentFlags().Lookup("token"); f != nil {
		f.DefValue = "********"
	}

	return cmd
}
