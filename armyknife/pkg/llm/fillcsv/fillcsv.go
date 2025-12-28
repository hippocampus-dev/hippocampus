package fillcsv

import (
	"armyknife/internal/bakery"
	"armyknife/internal/jsonschema"
	"armyknife/internal/openai"
	"armyknife/internal/progress"
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/go-playground/validator/v10"
	"golang.org/x/xerrors"
)

type StructuredResult struct {
	Result   string `json:"result" jsonschema_description:"The generated result"`
	Resolved bool   `json:"resolved" jsonschema_description:"Whether the generated result is resolved user task or not"`
}

func promptByOpenAI(ctx context.Context, bakeryClient *bakery.Client, model string, prompt string) (*StructuredResult, error) {
	generator := jsonschema.NewGenerator()
	schema, err := generator.Generate(StructuredResult{})
	if err != nil {
		return nil, xerrors.Errorf("failed to generate JSON schema: %w", err)
	}

	body := openai.ChatCompletionRequestBody{
		Model: model,
		Messages: []openai.ChatMessage{
			{
				Role:    openai.User,
				Content: prompt,
			},
		},
		N: 1,
		ResponseFormat: &openai.ResponseFormat{
			Type: "json_schema",
			JSONSchema: &openai.JSONSchema{
				Name:        "structured_result",
				Description: "Response with result and resolved status",
				Schema:      schema,
				Strict:      true,
			},
		},
	}
	b, err := json.Marshal(body)
	if err != nil {
		return nil, xerrors.Errorf("failed to marshal request body: %w", err)
	}

	request, err := openai.CreateHTTPRequest(ctx, bakeryClient, "/chat/completions", bytes.NewReader(b))
	if err != nil {
		return nil, xerrors.Errorf("failed to create openai http request: %w", err)
	}

	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return nil, xerrors.Errorf("failed to request: %w", err)
	}
	defer func() {
		_ = response.Body.Close()
	}()

	if response.StatusCode != http.StatusOK {
		var r openai.ErrorResponseBody
		if err := json.NewDecoder(response.Body).Decode(&r); err != nil {
			return nil, xerrors.Errorf("failed to decode response body: %w", err)
		}
		return nil, xerrors.Errorf("failed to request: %s", r.Error.Message)
	}

	var r openai.ChatCompletionResponseBody
	if err := json.NewDecoder(response.Body).Decode(&r); err != nil {
		return nil, xerrors.Errorf("failed to decode response body: %w", err)
	}

	var llmResponse StructuredResult
	if err := json.Unmarshal([]byte(r.Choices[0].Message.Content), &llmResponse); err != nil {
		return nil, xerrors.Errorf("failed to parse LLM JSON response: %w", err)
	}

	return &llmResponse, nil
}

func Run(a *Args) error {
	http.DefaultTransport.(*http.Transport).MaxIdleConnsPerHost = http.DefaultTransport.(*http.Transport).MaxIdleConns

	if err := validator.New().Struct(a); err != nil {
		return xerrors.Errorf("validation error: %w", err)
	}

	ctx := context.Background()

	bakeryClient := bakery.NewClient("https://bakery.minikube.127.0.0.1.nip.io/callback", a.AuthorizationListenPort)

	f, err := os.Open(a.CSV)
	if err != nil {
		log.Fatalf("failed to open file: %+v", err)
	}
	defer f.Close()

	from := strings.Split(a.From, ",")
	to := a.To

	var promptTemplate string
	if a.PromptFile != "" {
		b, err := os.ReadFile(a.PromptFile)
		if err != nil {
			log.Fatalf("failed to read promptTemplate file: %+v", err)
		}
		promptTemplate = string(b)
	} else {
		promptTemplate = fmt.Sprintf("You are an AI assistant. Given `%s`, your task is to generate `%s`.", strings.Join(from, " "), to)
		for _, f := range from {
			promptTemplate += "\n"
			promptTemplate += f + ": %s"
		}
		promptTemplate += "\n"
		promptTemplate += to + ":"
	}

	reader := csv.NewReader(f)
	records, err := reader.ReadAll()
	if err != nil {
		log.Fatalf("failed to read csv: %+v", err)
	}
	headers := records[0]

	progressBar := progress.NewBar(len(records) - 1)
	progressBar.Increment(0)

	semaphore := make(chan struct{}, a.Concurrency)

	results := make(chan map[string]string, len(records)-1)
	for _, record := range records[1:] {
		r := make(map[string]string)
		for i, field := range record {
			r[headers[i]] = field
		}

		if !a.Force && r[to] != "" {
			results <- r
			progressBar.Increment(1)
			continue
		}

		semaphore <- struct{}{}
		go func(r map[string]string) {
			defer func() {
				<-semaphore
			}()

			parameters := make([]any, 0, len(r))
			for _, f := range from {
				parameters = append(parameters, r[f])
			}
			prompt := fmt.Sprintf(promptTemplate, parameters...)

			llmResponse, err := promptByOpenAI(ctx, bakeryClient, a.Model, prompt)
			if err != nil {
				log.Printf("failed to prompt: %+v", err)
				results <- r
				progressBar.Increment(1)
				return
			}

			if a.ExcludeUnresolvedResults && !llmResponse.Resolved {
				log.Printf("LLM returned unresolved for record: %v", r)
				results <- r
				progressBar.Increment(1)
				return
			}

			if a.AppendBy == "" {
				r[to] = llmResponse.Result
			} else {
				r[to] = strings.Join([]string{r[to], llmResponse.Result}, a.AppendBy)
			}

			results <- r
			progressBar.Increment(1)
		}(r)
	}

	var output bytes.Buffer
	writer := csv.NewWriter(&output)
	_ = writer.Write(headers)
	for i := 0; i < len(records)-1; i++ {
		r := <-results
		var line []string
		for _, header := range headers {
			line = append(line, r[header])
		}
		_ = writer.Write(line)
	}
	writer.Flush()

	_, _ = os.Stdout.Write(output.Bytes())

	return nil
}
