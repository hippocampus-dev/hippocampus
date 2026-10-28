package oauth

import (
	"armyknife/pkg/oauth/pkce"

	"github.com/spf13/cobra"
	"golang.org/x/xerrors"
)

func pkceCmd() *cobra.Command {
	pkceArgs := pkce.DefaultArgs()

	cmd := &cobra.Command{
		Use:          "pkce",
		Short:        "Acquire an OAuth token via authorization code grant with PKCE (RFC 7636)",
		SilenceUsage: true,
		Args:         cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := pkce.Run(pkceArgs); err != nil {
				return xerrors.Errorf("failed to run pkce.Run: %w", err)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(
		&pkceArgs.URL,
		"url",
		pkceArgs.URL,
		"Authorization server URL",
	)

	cmd.Flags().StringVar(
		&pkceArgs.ClientID,
		"client-id",
		pkceArgs.ClientID,
		"OAuth client ID",
	)

	cmd.Flags().StringVar(
		&pkceArgs.Scope,
		"scope",
		pkceArgs.Scope,
		"OAuth scope to request",
	)

	cmd.Flags().UintVar(
		&pkceArgs.ListenPort,
		"listen-port",
		pkceArgs.ListenPort,
		"Local port for OAuth callback (0 = random)",
	)

	return cmd
}
