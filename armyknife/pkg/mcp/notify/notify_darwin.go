//go:build darwin

package notify

import (
	"fmt"
	"os/exec"
	"strings"

	"golang.org/x/xerrors"
)

// escapeAppleScript escapes strings for safe inclusion in AppleScript
func escapeAppleScript(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	return s
}

func (h *Handler) sendPlatformNotification(summary string, body string, urgency *string, expireTime *uint) error {
	safeSummary := escapeAppleScript(summary)
	safeBody := escapeAppleScript(body)

	script := fmt.Sprintf(`display notification "%s" with title "%s"`, safeBody, safeSummary)

	if urgency != nil && *urgency == "critical" {
		script += " sound name \"Blow\""
	}

	cmd := exec.Command("osascript", "-e", script)
	output, err := cmd.CombinedOutput()

	if err != nil {
		return xerrors.Errorf("osascript failed: %w, output: %s", err, string(output))
	}

	return nil
}
