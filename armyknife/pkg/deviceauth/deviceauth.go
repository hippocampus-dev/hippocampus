package deviceauth

import (
	"armyknife/internal/deviceauth"
	"fmt"

	"github.com/go-playground/validator/v10"
	"golang.org/x/xerrors"
)

func Run(a *Args) error {
	if err := validator.New().Struct(a); err != nil {
		return xerrors.Errorf("validation error: %w", err)
	}

	client := deviceauth.NewClient(a.URL, a.ClientID)
	token, err := client.GetToken(a.Scope)
	if err != nil {
		return xerrors.Errorf("failed to get token: %w", err)
	}

	fmt.Println(token)

	return nil
}
