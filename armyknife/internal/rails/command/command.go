package command

import (
	"fmt"
	"os"
	"path/filepath"

	"golang.org/x/xerrors"
)

func Root(start string) (*string, error) {
	directory, err := filepath.Abs(start)
	if err != nil {
		return nil, xerrors.Errorf("failed to get absolute path: %w", err)
	}
	for {
		if _, err := os.Stat(fmt.Sprintf("%s/.git", directory)); err == nil {
			return &directory, nil
		}
		if directory == "/" {
			return nil, xerrors.New("failed to find git repository root directory")
		}
		directory = filepath.Dir(directory)
	}
}
