package query

import (
	"armyknife/internal/bakery"
	"armyknife/internal/lsp"
	"armyknife/internal/openai"
	"armyknife/internal/vector/sqlite"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"text/tabwriter"

	"github.com/go-playground/validator/v10"
	"golang.org/x/xerrors"
)

func Run(args *Args) error {
	if err := validator.New().Struct(args); err != nil {
		return xerrors.Errorf("validation error: %w", err)
	}

	vectorStore, err := sqlite.NewVec(args.Database, args.Dimension)
	if err != nil {
		return xerrors.Errorf("failed to open vector database: %w", err)
	}
	defer vectorStore.Close()

	bakeryClient := bakery.NewClient("https://bakery.kaidotio.dev/callback", args.AuthorizationListenPort)

	requestBody := openai.EmbeddingsRequestBody{
		Input:          args.Query,
		Model:          args.EmbeddingModel,
		Dimensions:     &args.Dimension,
		EncodingFormat: "float",
	}

	requestBodyBytes, err := json.Marshal(requestBody)
	if err != nil {
		return xerrors.Errorf("failed to marshal request body: %w", err)
	}

	request, err := openai.CreateHTTPRequest(context.Background(), bakeryClient, "/embeddings", bytes.NewReader(requestBodyBytes))
	if err != nil {
		return xerrors.Errorf("failed to create http request: %w", err)
	}

	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return xerrors.Errorf("failed to do http request: %w", err)
	}
	defer func() {
		_ = response.Body.Close()
	}()

	if response.StatusCode >= 400 {
		var errorResponseBody openai.ErrorResponseBody
		if err := json.NewDecoder(response.Body).Decode(&errorResponseBody); err != nil {
			return xerrors.Errorf("failed to decode error response body: %w", err)
		}
		return xerrors.Errorf("API error: %s", errorResponseBody.Error.Message)
	}

	var embeddingsResponse openai.EmbeddingsResponseBody
	if err := json.NewDecoder(response.Body).Decode(&embeddingsResponse); err != nil {
		return xerrors.Errorf("failed to decode response body: %w", err)
	}

	if len(embeddingsResponse.Data) == 0 {
		return xerrors.Errorf("no embeddings returned for query")
	}

	queryVector := embeddingsResponse.Data[0].Embedding

	log.Printf("Searching for: %s (limit: %d)", args.Query, args.Limit)

	results, err := vectorStore.Search(context.Background(), queryVector, args.Limit)
	if err != nil {
		return xerrors.Errorf("failed to search: %w", err)
	}

	if len(results) == 0 {
		fmt.Println("No results found.")
		return nil
	}

	writer := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(writer, "SYMBOL\tKIND\tFILE\tLINE\tSIMILARITY")
	fmt.Fprintln(writer, "------\t----\t----\t----\t----------")

	for _, result := range results {
		symbolName := result.Metadata["symbol_name"]
		symbolKindStr := result.Metadata["symbol_kind"]
		fileURI := result.Metadata["file_uri"]
		startLine := result.Metadata["start_line"]

		filePath := strings.TrimPrefix(fileURI, "file://")

		symbolKindInt, err := strconv.Atoi(symbolKindStr)
		if err != nil {
			symbolKindInt = 0
		}
		symbolKind := lsp.SymbolKind(symbolKindInt).String()

		fmt.Fprintf(writer, "%s\t%s\t%s\t%s\t%.4f\n",
			symbolName,
			symbolKind,
			filePath,
			startLine,
			result.Similarity)
	}

	writer.Flush()

	fmt.Printf("\nFound %d results.\n", len(results))

	if args.Limit == 1 && len(results) > 0 {
		codeContent := results[0].Metadata["code_content"]
		if codeContent != "" {
			fmt.Println("\nCode snippet:")
			fmt.Println(strings.Repeat("-", 80))
			fmt.Println(codeContent)
			fmt.Println(strings.Repeat("-", 80))
		}
	}

	return nil
}
