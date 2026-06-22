//go:build !linux && !darwin && !windows

package notify

import (
	"runtime"

	"golang.org/x/xerrors"
)

func (h *Handler) sendPlatformNotification(summary string, body string, urgency *string, expireTime *uint) error {
	return xerrors.Errorf("notifications are not supported on %s", runtime.GOOS)
}
