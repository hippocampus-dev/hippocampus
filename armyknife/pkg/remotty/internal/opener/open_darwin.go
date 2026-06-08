//go:build darwin

package opener

import (
	"os/exec"

	"golang.org/x/xerrors"
)

func Open(u string) error {
	cmd := exec.Command("open", u)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return xerrors.Errorf("open failed: %w, output: %s", err, string(output))
	}
	return nil
}
