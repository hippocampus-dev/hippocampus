package registry

import (
	"os"

	"golang.org/x/xerrors"
	"gopkg.in/yaml.v3"
)

func ReadRegistry(path string) (*Registry, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, xerrors.Errorf("failed to read registry file: %w", err)
	}

	var r Registry
	if err := yaml.Unmarshal(data, &r); err != nil {
		return nil, xerrors.Errorf("failed to unmarshal registry: %w", err)
	}

	return &r, nil
}
