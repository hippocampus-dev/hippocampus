package credentials_edit

import (
	"armyknife/internal/rails_message_encryptor"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/go-playground/validator/v10"
	"golang.org/x/xerrors"
)

func Run(a *Args) error {
	if err := validator.New().Struct(a); err != nil {
		return xerrors.Errorf("validation error: %w", err)
	}

	encryptor := rails_message_encryptor.NewRailsMessageEncryptor(a.MasterKey, &rails_message_encryptor.Options{})

	pattern := filepath.Base(a.File)
	extension := filepath.Ext(a.File)
	if extension == ".enc" {
		pattern = strings.TrimSuffix(pattern, extension)
	}
	t, err := os.CreateTemp("", fmt.Sprintf("*.%s", pattern))
	if err != nil {
		return xerrors.Errorf("failed to create tempfile: %w", err)
	}
	defer os.Remove(t.Name())

	var before string
	if _, err := os.Stat(a.File); err == nil {
		b, err := os.ReadFile(a.File)
		if err != nil {
			return xerrors.Errorf("failed to read file: %w", err)
		}

		s, err := encryptor.Decrypt(string(b))
		if err != nil {
			return xerrors.Errorf("failed to decrypt: %w", err)
		}
		before = *s
		if err := os.WriteFile(t.Name(), []byte(before), 0644); err != nil {
			return xerrors.Errorf("failed to write to tempfile: %w", err)
		}
	}

	var editor string
	editor, ok := os.LookupEnv("EDITOR")
	if !ok {
		editor = "vim"
	}
	shards := strings.Fields(editor)
	name := shards[0]
	arg := []string{t.Name()}
	if len(shards) > 1 {
		arg = append(arg, shards[1:]...)
	}
	cmd := exec.Command(name, arg...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return xerrors.Errorf("failed to edit tempfile: %w", err)
	}

	modified, err := os.ReadFile(t.Name())
	if err != nil {
		return xerrors.Errorf("failed to read tempfile: %w", err)
	}
	after := string(modified)

	if before != after {
		encrypted, err := encryptor.Encrypt(after)
		if err != nil {
			return xerrors.Errorf("failed to encrypt: %w", err)
		}

		if err := os.MkdirAll(filepath.Dir(a.File), 0755); err != nil {
			return xerrors.Errorf("failed to create directory: %w", err)
		}

		if err := os.WriteFile(a.File, []byte(*encrypted), 0644); err != nil {
			return xerrors.Errorf("failed to write to file: %w", err)
		}
	}

	fmt.Println("File encrypted and saved.")

	return nil
}
