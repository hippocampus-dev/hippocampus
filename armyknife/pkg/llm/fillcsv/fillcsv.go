package fillcsv

import (
	"armyknife/internal/bakery"
	"armyknife/internal/openai"
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

func promptByOpenAI(ctx context.Context, bakeryClient *bakery.Client, prompt string) (*string, error) {
	body := openai.ChatCompletionRequestBody{
		Model: "gpt-4o",
		Messages: []openai.ChatMessage{
			{
				Role:    openai.User,
				Content: prompt,
			},
		},
		N: 1,
	}
	b, err := json.Marshal(body)
	if err != nil {
		return nil, xerrors.Errorf("failed to marshal request body: %w", err)
	}

	request, err := openai.CreateHTTPRequest(ctx, bakeryClient, bytes.NewBuffer(b))
	if err != nil {
		return nil, xerrors.Errorf("failed to create openai http request: %w", err)
	}

	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return nil, xerrors.Errorf("failed to request: %w", err)
	}
	defer response.Body.Close()

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
	return &r.Choices[0].Message.Content, nil
}

func Run(a *Args) error {
	if err := validator.New().Struct(a); err != nil {
		return xerrors.Errorf("validation error: %w", err)
	}

	ctx := context.Background()

	bakeryClient := bakery.NewClient("https://bakery.minikube.127.0.0.1.nip.io", a.AuthorizationListenPort)

	f, err := os.Open(a.CSV)
	if err != nil {
		log.Fatalf("failed to open file: %+v", err)
	}
	defer f.Close()

	from := strings.Split(os.Args[2], ",")
	to := os.Args[3]

	var prompt string
	if a.PromptFile != "" {
		b, err := os.ReadFile(a.PromptFile)
		if err != nil {
			log.Fatalf("failed to read prompt file: %+v", err)
		}
		prompt = string(b)
	} else {
		prompt = fmt.Sprintf("You are an AI assistant. Given `%s`, your task is to generate `%s`.", strings.Join(from, " "), to)
		for _, f := range from {
			prompt += "\n"
			prompt += f + ": %s"
		}
		prompt += "\n"
		prompt += to + ":"
	}

	reader := csv.NewReader(f)
	records, err := reader.ReadAll()
	if err != nil {
		log.Fatalf("failed to read csv: %+v", err)
	}
	headers := records[0]

	semaphore := make(chan struct{}, a.Concurrency)

	results := make(chan map[string]string, len(records)-1)
	for _, record := range records[1:] {
		r := make(map[string]string)
		for i, field := range record {
			r[headers[i]] = field
		}

		if !a.Override && r[to] != "" {
			results <- r
			continue
		}

		semaphore <- struct{}{}
		go func(r map[string]string) {
			defer func() {
				<-semaphore
			}()

			parameters := make([]interface{}, 0, len(r))
			for _, f := range from {
				parameters = append(parameters, r[f])
			}
			result, err := promptByOpenAI(ctx, bakeryClient, fmt.Sprintf(prompt, parameters...))
			if err != nil {
				log.Printf("failed to prompt: %+v", err)
				results <- r
				return
			}

			if a.AppendBy == "" {
				r[to] = *result
			} else {
				r[to] = strings.Join([]string{r[to], *result}, a.AppendBy)
			}

			results <- r
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
