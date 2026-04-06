package bakery

import (
	"armyknife/internal/bakery"
	"fmt"

	"github.com/go-playground/validator/v10"
	"golang.org/x/xerrors"
)

func Run(a *Args) error {
	if err := validator.New().Struct(a); err != nil {
		return xerrors.Errorf("validation error: %w", err)
	}

	client := bakery.NewClient(a.URL, a.ListenPort)
	v, err := client.GetValue(a.CookieName)
	if err != nil {
		return xerrors.Errorf("failed to get value: %w", err)
	}

	fmt.Println(v)

	return nil
}
