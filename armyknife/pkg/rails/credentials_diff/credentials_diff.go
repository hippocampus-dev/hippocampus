package credentials_diff

import (
	"armyknife/internal/rails/command"
	"armyknife/internal/rails_message_encryptor"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-playground/validator/v10"
	"golang.org/x/xerrors"
)

const gitattributes = ".gitattributes"

func Run(a *Args) error {
	if err := validator.New().Struct(a); err != nil {
		return xerrors.Errorf("validation error: %w", err)
	}

	gitattributesEntry := fmt.Sprintf("%s diff=armyknife_rails_credentials\n", a.File)
	if a.Enroll {
		root, err := command.Root(a.File)
		if err != nil {
			return xerrors.Errorf("failed to search repository root directory: %w", err)
		}

		path := filepath.Join(*root, gitattributes)
		if _, err := os.Stat(path); err == nil {
			b, err := os.ReadFile(path)
			if err != nil {
				return xerrors.Errorf("failed to read file: %w", err)
			}

			if !strings.Contains(string(b), gitattributesEntry) {
				b = append(b, []byte(gitattributesEntry)...)
				if err := os.WriteFile(path, b, 0644); err != nil {
					return xerrors.Errorf("failed to write to file: %w", err)
				}
			}
		} else {
			if err := os.WriteFile(path, []byte(gitattributesEntry), 0644); err != nil {
				return xerrors.Errorf("failed to write to file: %w", err)
			}
		}

		return nil
	}
	if a.Disenroll {
		root, err := command.Root(a.File)
		if err != nil {
			return xerrors.Errorf("failed to search repository root directory: %w", err)
		}

		path := filepath.Join(*root, gitattributes)
		if _, err := os.Stat(path); err == nil {
			b, err := os.ReadFile(path)
			if err != nil {
				return xerrors.Errorf("failed to read file: %w", err)
			}

			if strings.Contains(string(b), gitattributesEntry) {
				b = []byte(strings.Replace(string(b), gitattributesEntry, "", -1))
				if err := os.WriteFile(path, b, 0644); err != nil {
					return xerrors.Errorf("failed to write to file: %w", err)
				}
			}
		}

		return nil
	}

	b, err := os.ReadFile(a.File)
	if err != nil {
		return xerrors.Errorf("failed to read file: %w", err)
	}

	encryptor := rails_message_encryptor.NewRailsMessageEncryptor(a.MasterKey, &rails_message_encryptor.Options{})
	s, err := encryptor.Decrypt(string(b))
	if err == nil {
		fmt.Print(*s)
	}

	return nil
}
