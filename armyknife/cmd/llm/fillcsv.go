package llm

import (
	"armyknife/pkg/llm/fillcsv"
	"log"

	"github.com/spf13/cobra"
)

func fillcsvCmd() *cobra.Command {
	fillcsvArgs := fillcsv.DefaultArgs()

	cmd := &cobra.Command{
		Use:          "fillcsv CSV FROM TO",
		Short:        "fill csv columns using LLM",
		Example:      "fillcsv input.csv question,answer paraphrasing --prompt-file prompts/paraphrasing.txt > output.csv",
		SilenceUsage: true,
		Args:         cobra.ExactArgs(3),
		PreRun: func(cmd *cobra.Command, args []string) {
			fillcsvArgs.CSV = args[0]
			fillcsvArgs.From = args[1]
			fillcsvArgs.To = args[2]
		},
		Run: func(cmd *cobra.Command, args []string) {
			log.SetFlags(0)

			if err := fillcsv.Run(fillcsvArgs); err != nil {
				log.Fatalf("Failed to run fillcsv.Run: %+v", err)
			}
		},
	}

	cmd.Flags().UintVarP(
		&fillcsvArgs.Concurrency,
		"concurrency",
		"c",
		fillcsvArgs.Concurrency,
		"Number of concurrent requests",
	)

	cmd.Flags().StringVarP(
		&fillcsvArgs.PromptFile,
		"prompt-file",
		"",
		"",
		"File containing prompts for LLM",
	)

	cmd.Flags().BoolVarP(
		&fillcsvArgs.Override,
		"override",
		"o",
		false,
		"Override the original text with the generated text",
	)

	cmd.Flags().StringVarP(
		&fillcsvArgs.AppendBy,
		"append-by",
		"",
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

	return cmd
}
