package credentials_show

import (
	"armyknife/internal/rails_message_encryptor"
	"fmt"
	"os"

	"github.com/go-playground/validator/v10"
	"golang.org/x/xerrors"
)

func Run(a *Args) error {
	if err := validator.New().Struct(a); err != nil {
		return xerrors.Errorf("validation error: %w", err)
	}

	b, err := os.ReadFile(a.File)
	if err != nil {
		return xerrors.Errorf("failed to read file: %w", err)
	}

	encryptor := rails_message_encryptor.NewRailsMessageEncryptor(a.MasterKey, &rails_message_encryptor.Options{})
	s, err := encryptor.Decrypt(string(b))
	if err != nil {
		return xerrors.Errorf("failed to decrypt: %w", err)
	}

	fmt.Print(*s)

	return nil
}
