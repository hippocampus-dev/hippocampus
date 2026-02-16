//go:build linux

package notify

import (
	"fmt"
	"os/exec"

	"golang.org/x/xerrors"
)

func (h *Handler) sendPlatformNotification(summary string, body string, urgency *string, expireTime *uint) error {
	var args []string

	if urgency != nil {
		args = append(args, "-u", *urgency)
	}

	if expireTime != nil {
		args = append(args, "-t", fmt.Sprintf("%d", *expireTime*1000))
	}

	args = append(args, "--", summary, body)

	cmd := exec.Command("notify-send", args...)
	output, err := cmd.CombinedOutput()

	if err != nil {
		return xerrors.Errorf("notify-send failed: %w, output: %s", err, string(output))
	}

	return nil
}
