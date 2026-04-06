//go:build windows

package remotty

import (
	"os/exec"

	"golang.org/x/xerrors"
)

func open(u string) error {
	cmd := exec.Command("rundll32", "url.dll,FileProtocolHandler", u)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return xerrors.Errorf("rundll32 failed: %w, output: %s", err, string(output))
	}
	return nil
}
