//go:build !linux && !darwin && !windows

package remotty

import (
	"runtime"

	"golang.org/x/xerrors"
)

func open(u string) error {
	return xerrors.Errorf("open is not supported on %s", runtime.GOOS)
}
