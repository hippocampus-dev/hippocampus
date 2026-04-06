//go:build linux

package remotty

import (
	"os/exec"

	"golang.org/x/xerrors"
)

func open(u string) error {
	cmd := exec.Command("xdg-open", u)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return xerrors.Errorf("xdg-open failed: %w, output: %s", err, string(output))
	}
	return nil
}
