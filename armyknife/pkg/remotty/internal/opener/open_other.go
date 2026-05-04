//go:build !linux && !darwin && !windows

package opener

import (
	"runtime"

	"golang.org/x/xerrors"
)

func Open(u string) error {
	return xerrors.Errorf("open is not supported on %s", runtime.GOOS)
}
