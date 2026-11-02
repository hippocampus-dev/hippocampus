package credentials_edit

import (
	"armyknife/internal/rails/command"
	"os"
	"path/filepath"
)

type Args struct {
	File      string `validate:"required"`
	MasterKey string `validate:"required"`
}

func DefaultArgs() *Args {
	a := &Args{
		File:      "config/credentials.yml.enc",
		MasterKey: os.Getenv("RAILS_MASTER_KEY"),
	}
	cwd, err := os.Getwd()
	if err != nil {
		return a
	}
	root, err := command.Root(cwd)
	if err != nil {
		return a
	}
	a.File = filepath.Join(*root, a.File)
	return a
}
