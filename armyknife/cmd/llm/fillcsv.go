package llm

import (
	"armyknife/pkg/llm/fillcsv"

	"github.com/spf13/cobra"
	"golang.org/x/xerrors"
)

func fillcsvCmd() *cobra.Command {
	fillcsvArgs := fillcsv.DefaultArgs()

	cmd := &cobra.Command{
		Use:          "fillcsv CSV FROM TO",
		Short:        "fill csv columns using LLM",
		Example:      "fillcsv input.csv question,answer paraphrases --prompt-file prompts/paraphrases.txt > output.csv",
		SilenceUsage: true,
		Args:         cobra.ExactArgs(3),
		PreRun: func(cmd *cobra.Command, args []string) {
			fillcsvArgs.CSV = args[0]
			fillcsvArgs.From = args[1]
			fillcsvArgs.To = args[2]
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := fillcsv.Run(fillcsvArgs); err != nil {
				return xerrors.Errorf("failed to run fillcsv.Run: %w", err)
			}
			return nil
		},
	}

	cmd.Flags().UintVarP(
		&fillcsvArgs.Concurrency,
		"concurrency",
		"c",
		fillcsvArgs.Concurrency,
		"Number of concurrent requests",
	)

	cmd.Flags().StringVar(
		&fillcsvArgs.PromptFile,
		"prompt-file",
		"",
		"File containing prompts for LLM",
	)

	cmd.Flags().BoolVarP(
		&fillcsvArgs.Overwrite,
		"overwrite",
		"o",
		false,
		"Overwrite the original text with the generated text",
	)

	cmd.Flags().StringVar(
		&fillcsvArgs.AppendBy,
		"append-by",
		"",
		"Delimiter to append the generated text to the original text",
	)

	cmd.Flags().StringVarP(
		&fillcsvArgs.Model,
		"model",
		"m",
		fillcsvArgs.Model,
		"Model name for OpenAI Chat Completions",
	)

	cmd.Flags().UintVarP(
		&fillcsvArgs.AuthorizationListenPort,
		"authorization-listen-port",
		"p",
		fillcsvArgs.AuthorizationListenPort,
		"Port number for authorization server",
	)

	cmd.Flags().BoolVarP(
		&fillcsvArgs.ExcludeUnresolvedResults,
		"exclude-unresolved-results",
		"",
		false,
		"Exclude unresolved results from the output",
	)

	return cmd
}
