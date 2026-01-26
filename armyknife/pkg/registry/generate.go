package registry

import (
	"fmt"
	"path/filepath"

	"armyknife/internal/registry"

	"github.com/go-playground/validator/v10"
	"golang.org/x/xerrors"
	"gopkg.in/yaml.v3"
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

	parser := registry.NewParser(a.ManifestPath)
	reg, err := parser.Parse()
	if err != nil {
		return xerrors.Errorf("failed to parse manifest: %w", err)
	}

	if a.Stdout {
		data, err := yaml.Marshal(reg)
		if err != nil {
			return xerrors.Errorf("failed to marshal registry: %w", err)
		}
		fmt.Print(string(data))
		return nil
	}

	outputPath := a.OutputPath
	if outputPath == "" {
		outputPath = filepath.Join(a.ManifestPath, ".registry.yaml")
	}

	if err := registry.WriteRegistry(reg, outputPath); err != nil {
		return xerrors.Errorf("failed to write registry: %w", err)
	}

	fmt.Printf("Generated: %s\n", outputPath)
	return nil
}
