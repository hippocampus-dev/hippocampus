package s3

import (
	"armyknife/pkg/s3/viewer"

	"github.com/spf13/cobra"
	"golang.org/x/xerrors"
)

func viewerCmd() *cobra.Command {
	viewerArgs := viewer.DefaultArgs()

	cmd := &cobra.Command{
		Use:          "viewer S3_BUCKET S3_PREFIX",
		Short:        "View S3 objects using fuzzy finder",
		Example:      "S3_ENDPOINT_URL=http://127.0.0.1:9000 viewer my-bucket my-prefix",
		SilenceUsage: true,
		Args:         cobra.ExactArgs(2),
		PreRun: func(cmd *cobra.Command, args []string) {
			viewerArgs.S3Bucket = args[0]
			viewerArgs.S3Prefix = args[1]
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := viewer.Run(viewerArgs); err != nil {
				return xerrors.Errorf("failed to run viewer.Run: %w", err)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(
		&viewerArgs.S3EndpointURL,
		"s3-endpoint-url",
		viewerArgs.S3EndpointURL,
		"S3 endpoint URL",
	)

	return cmd
}
