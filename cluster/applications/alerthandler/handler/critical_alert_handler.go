package handler

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"

	"github.com/google/go-github/v68/github"
	"golang.org/x/xerrors"
)

type CriticalAlertHandler struct {
	client *github.Client
}

func NewCriticalAlertHandler(client *github.Client) *CriticalAlertHandler {
	return &CriticalAlertHandler{
		client: client,
	}
}

func (h *CriticalAlertHandler) Call(request *AlertManagerRequest) error {
	ctx := context.Background()

	var errs []error
	for _, alert := range request.Alerts {
		if alert.Status != "firing" {
			continue
		}

		owner, repository := h.parseRepository(alert.Labels["repository"])
		if owner == "" || repository == "" {
			errs = append(errs, xerrors.New("repository label is required (format: owner/repo)"))
			continue
		}

		title := h.buildTitle(request.CommonLabels["alertname"], alert.Labels)
		body := h.buildBody(request, alert)
		labels := []string{"alert", "critical"}

		issueRequest := &github.IssueRequest{
			Title:  github.Ptr(title),
			Body:   github.Ptr(body),
			Labels: &labels,
		}

		issue, _, err := h.client.Issues.Create(ctx, owner, repository, issueRequest)
		if err != nil {
			log.Printf("Failed to create issue for alert %s: %+v", request.CommonLabels["alertname"], err)
			errs = append(errs, xerrors.Errorf("alert %s: failed to create issue: %w", request.CommonLabels["alertname"], err))
			continue
		}

		log.Printf("Created GitHub issue #%d (%s) for alert %s", issue.GetNumber(), issue.GetHTMLURL(), request.CommonLabels["alertname"])
	}

	return errors.Join(errs...)
}

func (h *CriticalAlertHandler) parseRepository(repository string) (string, string) {
	if repository == "" {
		return "", ""
	}
	parts := strings.SplitN(repository, "/", 2)
	if len(parts) != 2 {
		return "", ""
	}
	return parts[0], parts[1]
}

func (h *CriticalAlertHandler) buildTitle(alertname string, labels map[string]string) string {
	namespace := labels["namespace"]
	pod := labels["pod"]

	if namespace != "" && pod != "" {
		return fmt.Sprintf("[CRITICAL] %s: %s/%s", alertname, namespace, pod)
	}
	if namespace != "" {
		return fmt.Sprintf("[CRITICAL] %s: %s", alertname, namespace)
	}
	return fmt.Sprintf("[CRITICAL] %s", alertname)
}

func (h *CriticalAlertHandler) buildBody(request *AlertManagerRequest, alert Alert) string {
	var builder strings.Builder

	builder.WriteString("## Alert Details\n\n")
	builder.WriteString(fmt.Sprintf("- **Alert Name**: %s\n", request.CommonLabels["alertname"]))
	builder.WriteString(fmt.Sprintf("- **Severity**: %s\n", request.CommonLabels["severity"]))
	builder.WriteString(fmt.Sprintf("- **Started At**: %s\n", alert.StartsAt.Format("2006-01-02T15:04:05Z07:00")))

	if summary, ok := alert.Annotations["summary"]; ok {
		builder.WriteString(fmt.Sprintf("- **Summary**: %s\n", summary))
	}

	if description, ok := alert.Annotations["description"]; ok {
		builder.WriteString(fmt.Sprintf("\n## Description\n\n%s\n", description))
	}

	builder.WriteString("\n## Labels\n\n")
	builder.WriteString("| Key | Value |\n")
	builder.WriteString("|-----|-------|\n")
	for key, value := range alert.Labels {
		builder.WriteString(fmt.Sprintf("| %s | %s |\n", key, value))
	}

	if alert.GeneratorURL != "" {
		builder.WriteString(fmt.Sprintf("\n## Links\n\n- [View in Prometheus](%s)\n", alert.GeneratorURL))
	}

	if request.ExternalURL != "" {
		builder.WriteString(fmt.Sprintf("- [Alertmanager UI](%s)\n", request.ExternalURL))
	}

	return builder.String()
}
