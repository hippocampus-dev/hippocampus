package index

import (
	"armyknife/internal/bakery"
	"armyknife/internal/lsp"
	"armyknife/internal/openai"
	"armyknife/internal/vector"
	"armyknife/internal/vector/sqlite"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"hash/fnv"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/denormal/go-gitignore"
	"github.com/go-playground/validator/v10"
	"golang.org/x/sync/errgroup"
	"golang.org/x/xerrors"
)

var ErrAlreadyIndexed = errors.New("symbol already indexed")

type SymbolDocument struct {
	ID        string
	Name      string
	Kind      lsp.SymbolKind
	FileURI   string
	Range     lsp.Range
	Code      string
	Source    string
	Container string
}

func Run(args *Args) error {
	if err := validator.New().Struct(args); err != nil {
		return xerrors.Errorf("validation error: %w", err)
	}

	vectorStore, err := sqlite.NewVec(args.Output, args.Dimension)
	if err != nil {
		return xerrors.Errorf("failed to create vector store: %w", err)
	}
	defer vectorStore.Close()

	log.Printf("Starting symbol indexing from %d LSP servers", len(args.LSPServers))

	eg, ctx := errgroup.WithContext(context.Background())

	bakeryClient := bakery.NewClient("https://bakery.kaidotio.dev/callback", args.AuthorizationListenPort)

	for _, serverAddr := range args.LSPServers {
		addr := serverAddr
		eg.Go(func() error {
			if err := indexFromLSPServer(ctx, addr, vectorStore, args.Dimension, args.EmbeddingModel, bakeryClient); err != nil {
				return xerrors.Errorf("failed to index from server %s: %w", addr, err)
			}
			return nil
		})
	}

	return eg.Wait()
}

func indexFromLSPServer(ctx context.Context, source string, vectorStore vector.Store, vectorDimension int, embeddingModel string, bakeryClient *bakery.Client) error {
	startTime := time.Now().Unix()

	client, err := lsp.NewClient(source)
	if err != nil {
		return xerrors.Errorf("failed to create LSP client: %w", err)
	}
	defer client.Close()

	cwd, err := os.Getwd()
	if err != nil {
		return xerrors.Errorf("failed to get current working directory: %w", err)
	}

	workspaceURI := fmt.Sprintf("file://%s", cwd)
	if _, err := client.Initialize(workspaceURI); err != nil {
		return xerrors.Errorf("failed to initialize LSP client: %w", err)
	}

	files, err := getProjectFiles(cwd)
	if err != nil {
		return xerrors.Errorf("failed to get project files: %w", err)
	}

	log.Printf("Found %d files to index from source %s", len(files), source)

	totalSymbols := 0
	skippedSymbols := 0
	for _, filePath := range files {
		fileURI := fmt.Sprintf("file://%s", filePath)

		symbols, err := client.TextDocumentDocumentSymbol(fileURI)
		if err != nil {
			continue
		}

		for _, symbol := range symbols {
			switch symbol.Kind {
			case lsp.SymbolKindFunction, lsp.SymbolKindMethod, lsp.SymbolKindClass,
				lsp.SymbolKindInterface, lsp.SymbolKindStruct, lsp.SymbolKindEnum,
				lsp.SymbolKindConstructor:
			default:
				continue
			}

			code, err := lsp.GetCodeFromRange(symbol.Location.URI, symbol.Location.Range)
			if err != nil {
				log.Printf("Warning: failed to extract code for symbol %s: %v", symbol.Name, err)
				continue
			}

			h := fnv.New64a()
			_, _ = h.Write([]byte(source))
			_, _ = h.Write([]byte("|"))
			_, _ = h.Write([]byte(symbol.Location.URI))
			_, _ = h.Write([]byte("|"))
			_, _ = h.Write([]byte(symbol.ContainerName))
			_, _ = h.Write([]byte("|"))
			_, _ = h.Write([]byte(symbol.Name))
			_, _ = h.Write([]byte("|"))
			_, _ = h.Write([]byte(fmt.Sprintf("%d", symbol.Kind)))
			_, _ = h.Write([]byte("|"))
			_, _ = h.Write([]byte(code))
			symbolID := fmt.Sprintf("%016x", h.Sum64())

			symbolDocument := SymbolDocument{
				ID:        symbolID,
				Name:      symbol.Name,
				Kind:      symbol.Kind,
				FileURI:   symbol.Location.URI,
				Range:     symbol.Location.Range,
				Code:      code,
				Source:    source,
				Container: symbol.ContainerName,
			}

			if err := indexSymbol(ctx, vectorStore, symbolDocument, vectorDimension, embeddingModel, bakeryClient); err != nil {
				if errors.Is(err, ErrAlreadyIndexed) {
					skippedSymbols++
				} else {
					log.Printf("Warning: failed to index symbol %s: %v", symbol.Name, err)
				}
				continue
			}

			totalSymbols++
		}
	}

	log.Printf("Indexed %d symbols from source %s (skipped %d unchanged)", totalSymbols, source, skippedSymbols)

	if err := vectorStore.DeleteOldEntries(ctx, source, startTime); err != nil {
		log.Printf("Warning: failed to delete old entries: %v", err)
	} else {
		log.Printf("Cleaned up old symbols for source %s", source)
	}

	return client.Shutdown()
}

func indexSymbol(ctx context.Context, vectorStore vector.Store, symbolDocument SymbolDocument, vectorDimension int, embeddingModel string, bakeryClient *bakery.Client) error {
	exists, err := vectorStore.Exists(ctx, symbolDocument.ID)
	if err != nil {
		return xerrors.Errorf("failed to check existing document: %w", err)
	}

	if exists {
		log.Printf("Skipping symbol %s (ID: %s) - already indexed", symbolDocument.Name, symbolDocument.ID)
		return ErrAlreadyIndexed
	}

	requestBody := openai.EmbeddingsRequestBody{
		Input:          symbolDocument.Code,
		Model:          embeddingModel,
		Dimensions:     &vectorDimension,
		EncodingFormat: "float",
	}

	requestBodyBytes, err := json.Marshal(requestBody)
	if err != nil {
		return xerrors.Errorf("failed to marshal request body: %w", err)
	}

	request, err := openai.CreateHTTPRequest(ctx, bakeryClient, "/embeddings", bytes.NewReader(requestBodyBytes))
	if err != nil {
		return xerrors.Errorf("failed to create http request: %w", err)
	}

	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return xerrors.Errorf("failed to do http request: %w", err)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
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
		return xerrors.Errorf("no embeddings returned")
	}

	embeddingVector := embeddingsResponse.Data[0].Embedding

	metadata := map[string]string{
		"symbol_name": symbolDocument.Name,
		"symbol_kind": fmt.Sprintf("%d", symbolDocument.Kind),
		"file_uri":    symbolDocument.FileURI,
		"source":      symbolDocument.Source,
		"container":   symbolDocument.Container,
		"code_length": fmt.Sprintf("%d", len(symbolDocument.Code)),
		"start_line":  fmt.Sprintf("%d", symbolDocument.Range.Start.Line),
		"end_line":    fmt.Sprintf("%d", symbolDocument.Range.End.Line),
		"code":        symbolDocument.Code,
	}

	return vectorStore.Index(ctx, symbolDocument.ID, embeddingVector, metadata, symbolDocument.Source)
}

func getProjectFiles(rootDir string) ([]string, error) {
	var files []string

	ignore, _ := gitignore.NewRepository(rootDir)

	err := filepath.Walk(rootDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(rootDir, path)
		if err != nil {
			relPath = path
		}

		if ignore != nil {
			match := ignore.Relative(relPath, info.IsDir())
			if match != nil && match.Ignore() {
				if info.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
		}

		if info.IsDir() {
			if info.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}

		absPath, err := filepath.Abs(path)
		if err != nil {
			return err
		}
		files = append(files, absPath)

		return nil
	})

	if err != nil {
		return nil, xerrors.Errorf("failed to walk directory: %w", err)
	}

	return files, nil
}
