package cmd

import (
	"armyknife/pkg/remotty"

	"github.com/spf13/cobra"
	"golang.org/x/xerrors"
)

func remottyCmd() *cobra.Command {
	remottyArgs := remotty.DefaultArgs()

	cmd := &cobra.Command{
		Use:          "remotty SERVER REMOTE [REMOTE...]",
		Short:        "Connect to a remotty workspace via reverse tunnel",
		SilenceUsage: true,
		Args:         cobra.MinimumNArgs(2),
		PreRun: func(cmd *cobra.Command, args []string) {
			remottyArgs.Server = args[0]
			remottyArgs.Remotes = args[1:]
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := remotty.Run(remottyArgs); err != nil {
				return xerrors.Errorf("failed to run remotty.Run: %w", err)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(
		&remottyArgs.Auth,
		"auth",
		remottyArgs.Auth,
		"Chisel authentication credentials (user:password)",
	)

	cmd.Flags().StringVar(
		&remottyArgs.BakeryURL,
		"bakery-url",
		remottyArgs.BakeryURL,
		"Bakery callback URL",
	)

	cmd.Flags().StringVar(
		&remottyArgs.CookieName,
		"cookie-name",
		remottyArgs.CookieName,
		"Session cookie name (defaults to server hostname)",
	)

	cmd.Flags().UintVarP(
		&remottyArgs.ListenPort,
		"listen-port",
		"p",
		remottyArgs.ListenPort,
		"The port to listen on for the bakery authorization callback",
	)

	cmd.Flags().StringVar(
		&remottyArgs.Sync,
		"sync",
		remottyArgs.Sync,
		"Local directory to expose via SFTP for remote mounting",
	)

	return cmd
}
