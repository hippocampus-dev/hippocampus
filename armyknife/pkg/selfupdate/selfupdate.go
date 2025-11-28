package selfupdate

import (
	"armyknife/pkg/version"
	"bufio"
	"fmt"
	"os"

	"github.com/go-playground/validator/v10"

	"github.com/blang/semver"
	"github.com/rhysd/go-github-selfupdate/selfupdate"
	"golang.org/x/xerrors"
)

const (
	repository = "kaidotio/hippocampus"
)

func Run(a *Args) error {
	if err := validator.New().Struct(a); err != nil {
		return xerrors.Errorf("validation error: %w", err)
	}

	up, err := selfupdate.NewUpdater(selfupdate.Config{
		APIToken: a.GitHubToken,
	})
	if err != nil {
		return xerrors.Errorf("failed to create updater: %w", err)
	}
	latest, found, err := up.DetectLatest(repository)
	if err != nil {
		return xerrors.Errorf("failed to detect the latest: %w", err)
	}

	v := semver.MustParse(version.Version)
	if !found || latest.Version.LTE(v) {
		fmt.Print("Current version is the latest")
		return nil
	}

	fmt.Printf("Do you want to update to %s? (Y/n): ", latest.Version)
	input, err := bufio.NewReader(os.Stdin).ReadString('\n')
	if err != nil {
		return xerrors.Errorf("failed to read input string: %w", err)
	}
	if input != "Y\n" && input != "\n" {
		return nil
	}

	if _, err := up.UpdateSelf(v, repository); err != nil {
		return xerrors.Errorf("failed to update self: %w", err)
	}
	return nil
}
