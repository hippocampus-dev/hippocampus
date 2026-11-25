package completion

import (
	"embed"
	"errors"
	"fmt"
	"strings"

	"github.com/go-playground/validator/v10"
	"golang.org/x/xerrors"
)

//go:embed templates/*
var templates embed.FS

func Run(a *Args) error {
	if err := validator.New().Struct(a); err != nil {
		var errs validator.ValidationErrors
		errors.As(err, &errs)
		var messages []string
		for _, e := range errs {
			if e.ActualTag() == "oneof" {
				messages = append(messages, fmt.Sprintf("%s must be one of these [%s]", e.Field(), e.Param()))
			}
		}
		if len(messages) > 0 {
			err = xerrors.Errorf("%s: %w", strings.Join(messages, ", "), err)
		}
		return xerrors.Errorf("validation error: %w", err)
	}

	switch a.Shell {
	case "fish":
		b, err := templates.ReadFile("templates/armyknife.fish")
		if err != nil {
			return xerrors.Errorf("failed to find %s completion file: %w", a.Shell, err)
		}
		fmt.Print(string(b))
	}
	return nil
}
