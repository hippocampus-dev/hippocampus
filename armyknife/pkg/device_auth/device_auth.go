package device_auth

import (
	"armyknife/internal/device_auth"
	"fmt"

	"github.com/go-playground/validator/v10"
	"golang.org/x/xerrors"
)

func Run(a *Args) error {
	if err := validator.New().Struct(a); err != nil {
		return xerrors.Errorf("validation error: %w", err)
	}

	client := device_auth.NewClient(a.URL)
	token, err := client.GetToken(a.Scope)
	if err != nil {
		return xerrors.Errorf("failed to get token: %w", err)
	}

	fmt.Println(token)

	return nil
}
