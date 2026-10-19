package generators

import (
	"armyknife/internal/rails/command"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/xerrors"
)

const gitignore = ".gitignore"

type EncryptionKeyFileGenerator struct{}

func NewEncryptionKeyFileGenerator() *EncryptionKeyFileGenerator {
	return &EncryptionKeyFileGenerator{}
}

func (g *EncryptionKeyFileGenerator) AddKeyFile(keyFile string) error {
	root, err := command.Root(keyFile)
	if err != nil {
		return xerrors.Errorf("failed to search repository root directory: %w", err)
	}

	path := filepath.Join(*root, keyFile)
	if _, err := os.Stat(path); err == nil {
		return nil
	}

	key := make([]byte, 16)
	if _, err := rand.Read(key); err != nil {
		return xerrors.Errorf("failed to generate random key: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return xerrors.Errorf("failed to create directory: %w", err)
	}

	if err := os.WriteFile(path, []byte(hex.EncodeToString(key)), 0600); err != nil {
		return xerrors.Errorf("failed to write to file: %w", err)
	}
	return nil
}

func (g *EncryptionKeyFileGenerator) IgnoreKeyFile(keyFile string) error {
	root, err := command.Root(keyFile)
	if err != nil {
		return xerrors.Errorf("failed to search repository root directory: %w", err)
	}

	gitignoreEntry := fmt.Sprintf("\n/%s", keyFile)

	path := filepath.Join(*root, gitignore)
	if _, err := os.Stat(path); err == nil {
		b, err := os.ReadFile(path)
		if err != nil {
			return xerrors.Errorf("failed to read file: %w", err)
		}

		if !strings.Contains(string(b), gitignoreEntry) {
			b = append(b, []byte(gitignoreEntry)...)
			if err := os.WriteFile(path, b, 0644); err != nil {
				return xerrors.Errorf("failed to write to file: %w", err)
			}
		}
	} else {
		if err := os.WriteFile(path, []byte(gitignoreEntry), 0644); err != nil {
			return xerrors.Errorf("failed to write to file: %w", err)
		}
	}

	return nil
}
