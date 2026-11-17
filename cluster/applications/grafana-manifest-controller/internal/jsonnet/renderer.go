package jsonnet

import (
	"path/filepath"
	"strings"

	"github.com/google/go-jsonnet"
	"golang.org/x/xerrors"
)

// SafeImporter wraps FileImporter and restricts imports to allowed paths
type SafeImporter struct {
	allowedPaths []string
	fileImporter *jsonnet.FileImporter
}

// Import implements jsonnet.Importer interface with path restriction
func (i *SafeImporter) Import(importedFrom string, importedPath string) (jsonnet.Contents, string, error) {
	contents, foundAt, err := i.fileImporter.Import(importedFrom, importedPath)
	if err != nil {
		return contents, foundAt, err
	}

	absPath, err := filepath.Abs(foundAt)
	if err != nil {
		return jsonnet.Contents{}, "", xerrors.Errorf("failed to resolve absolute path: %w", err)
	}

	for _, allowed := range i.allowedPaths {
		absAllowed, err := filepath.Abs(allowed)
		if err != nil {
			continue
		}
		if strings.HasPrefix(absPath, absAllowed+string(filepath.Separator)) || absPath == absAllowed {
			return contents, foundAt, nil
		}
	}

	return jsonnet.Contents{}, "", xerrors.Errorf("import path %q is outside allowed directories", importedPath)
}

// Renderer renders Jsonnet to JSON
type Renderer struct {
	vm *jsonnet.VM
}

// NewRenderer creates a new Jsonnet renderer with library paths for imports
func NewRenderer(libraryPaths []string) *Renderer {
	vm := jsonnet.MakeVM()
	vm.Importer(&SafeImporter{
		allowedPaths: libraryPaths,
		fileImporter: &jsonnet.FileImporter{
			JPaths: libraryPaths,
		},
	})
	return &Renderer{vm: vm}
}

// Render evaluates Jsonnet code and returns JSON
func (r *Renderer) Render(source string) ([]byte, error) {
	if source == "" {
		return nil, xerrors.New("empty jsonnet source")
	}

	result, err := r.vm.EvaluateAnonymousSnippet("dashboard.jsonnet", source)
	if err != nil {
		return nil, xerrors.Errorf("failed to evaluate jsonnet: %w", err)
	}

	return []byte(result), nil
}
