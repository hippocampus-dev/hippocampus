//go:build windows

package notify

import (
	"html"
	"os/exec"

	"golang.org/x/xerrors"
)

func (h *Handler) sendPlatformNotification(summary string, body string, urgency *string, expireTime *uint) error {
	safeSummary := html.EscapeString(summary)
	safeBody := html.EscapeString(body)

	powershellScript := `
[Windows.UI.Notifications.ToastNotificationManager, Windows.UI.Notifications, ContentType = WindowsRuntime] | Out-Null
[Windows.UI.Notifications.ToastNotification, Windows.UI.Notifications, ContentType = WindowsRuntime] | Out-Null
[Windows.Data.Xml.Dom.XmlDocument, Windows.Data.Xml.Dom.XmlDocument, ContentType = WindowsRuntime] | Out-Null

$summary = @'
` + safeSummary + `
'@

$body = @'
` + safeBody + `
'@

$template = @"
<toast>
    <visual>
        <binding template="ToastGeneric">
            <text>$summary</text>
            <text>$body</text>
        </binding>
    </visual>
</toast>
"@

$xml = New-Object Windows.Data.Xml.Dom.XmlDocument
$xml.LoadXml($template)
$toast = New-Object Windows.UI.Notifications.ToastNotification $xml
[Windows.UI.Notifications.ToastNotificationManager]::CreateToastNotifier("armyknife").Show($toast)
`

	cmd := exec.Command("powershell", "-ExecutionPolicy", "Bypass", "-Command", powershellScript)
	output, err := cmd.CombinedOutput()

	if err != nil {
		return xerrors.Errorf("powershell notification failed: %w, output: %s", err, string(output))
	}

	return nil
}
