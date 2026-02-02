package registry

import (
	"fmt"
	"path/filepath"

	"armyknife/internal/registry"

	"github.com/go-playground/validator/v10"
	"golang.org/x/xerrors"
)

func Run(a *Args) error {
	absPath, err := filepath.Abs(a.ManifestPath)
	if err != nil {
		return xerrors.Errorf("failed to resolve path: %w", err)
	}
	a.ManifestPath = absPath

	if err := validator.New().Struct(a); err != nil {
		return xerrors.Errorf("validation error: %w", err)
	}

	var grafanaConfig *registry.GrafanaConfig
	if a.GrafanaURL != "" {
		grafanaConfig = &registry.GrafanaConfig{
			GrafanaURL:             a.GrafanaURL,
			LokiDatasourceUID:      a.LokiDatasourceUID,
			TempoDatasourceUID:     a.TempoDatasourceUID,
			PyroscopeDatasourceUID: a.PyroscopeDatasourceUID,
		}
	}

	parser := registry.NewParser(a.ManifestPath, grafanaConfig)
	r, err := parser.Parse()
	if err != nil {
		return xerrors.Errorf("failed to parse manifest: %w", err)
	}

	outputPath := a.OutputPath
	if outputPath == "" {
		outputPath = filepath.Join(a.ManifestPath, ".registry.yaml")
	}

	if err := registry.WriteRegistry(r, outputPath); err != nil {
		return xerrors.Errorf("failed to write registry: %w", err)
	}

	fmt.Printf("Generated: %s\n", outputPath)
	return nil
}
