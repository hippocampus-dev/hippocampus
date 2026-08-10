package handler

import (
	"context"
	"errors"
	"fmt"
	"log"
	"sort"
	"strings"
	"time"

	"github.com/google/go-github/v68/github"
	"golang.org/x/xerrors"
)

type CriticalAlertHandler struct {
	client  *github.Client
	timeout time.Duration
}

func NewCriticalAlertHandler(client *github.Client, timeout time.Duration) *CriticalAlertHandler {
	return &CriticalAlertHandler{
		client:  client,
		timeout: timeout,
	}
}

func (h *CriticalAlertHandler) Call(request *AlertManagerRequest) error {
	ctx, cancel := context.WithTimeout(context.Background(), h.timeout)
	defer cancel()

	var errs []error
	// Listing once per repository keeps the cost at repositories x pages rather than alerts x pages, and doubles as the within-batch duplicate guard.
	opened := make(map[string]map[string]int)
	for _, alert := range request.Alerts {
		if alert.Status != "firing" {
			continue
		}

		owner, repository := h.parseRepository(alert.Labels["repository"])
		if owner == "" || repository == "" {
			log.Printf("Skipped alert %s: repository label is required (format: owner/repo)", request.CommonLabels["alertname"])
			continue
		}

		slug := fmt.Sprintf("%s/%s", owner, repository)
		numbers, ok := opened[slug]
		if !ok {
			listed, err := h.listOpenIssues(ctx, owner, repository)
			if err != nil {
				errs = append(errs, xerrors.Errorf("alert %s: failed to list issues: %w", request.CommonLabels["alertname"], err))
				continue
			}
			opened[slug] = listed
			numbers = listed
		}

		severity := request.CommonLabels["severity"]
		title := h.buildTitle(request.CommonLabels["alertname"], severity, alert.Labels)

		if number, ok := numbers[title]; ok {
			log.Printf("Skipped creating issue titled %q: #%d is already open", title, number)
			continue
		}

		labels := []string{"alert"}
		if severity != "" {
			labels = append(labels, severity)
		}

		issueRequest := &github.IssueRequest{
			Title:  github.Ptr(title),
			Body:   github.Ptr(h.buildBody(request, alert)),
			Labels: &labels,
		}

		issue, _, err := h.client.Issues.Create(ctx, owner, repository, issueRequest)
		if err != nil {
			log.Printf("Failed to create issue for alert %s: %+v", request.CommonLabels["alertname"], err)
			errs = append(errs, xerrors.Errorf("alert %s: failed to create issue: %w", request.CommonLabels["alertname"], err))
			continue
		}

		numbers[title] = issue.GetNumber()

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

func (h *CriticalAlertHandler) listOpenIssues(ctx context.Context, owner string, repository string) (map[string]int, error) {
	numbers := make(map[string]int)
	options := &github.IssueListByRepoOptions{
		State:       "open",
		ListOptions: github.ListOptions{PerPage: 100},
	}

	for {
		issues, response, err := h.client.Issues.ListByRepo(ctx, owner, repository, options)
		if err != nil {
			return nil, err
		}

		for _, issue := range issues {
			if issue == nil || issue.IsPullRequest() {
				continue
			}
			numbers[issue.GetTitle()] = issue.GetNumber()
		}

		if response.NextPage == 0 {
			return numbers, nil
		}
		options.Page = response.NextPage
	}
}

func (h *CriticalAlertHandler) buildTitle(alertname string, severity string, labels map[string]string) string {
	namespace := labels["namespace"]
	pod := labels["pod"]

	prefix := strings.ToUpper(severity)
	if prefix == "" {
		prefix = "ALERT"
	}

	title := fmt.Sprintf("[%s] %s", prefix, alertname)
	if namespace != "" && pod != "" {
		title = fmt.Sprintf("%s: %s/%s", title, namespace, pod)
	} else if namespace != "" {
		title = fmt.Sprintf("%s: %s", title, namespace)
	}
	// Without this an alert whose distinct occurrences share every label the title is built from lands on one issue that the already-open guard then keeps reusing.
	if errorHash := labels["error_hash"]; errorHash != "" {
		title = fmt.Sprintf("%s (%s)", title, errorHash)
	}
	return title
}

func (h *CriticalAlertHandler) buildBody(request *AlertManagerRequest, alert Alert) string {
	var builder strings.Builder

	builder.WriteString("## Alert Details\n\n")
	builder.WriteString(fmt.Sprintf("- **Alert Name**: %s\n", request.CommonLabels["alertname"]))
	builder.WriteString(fmt.Sprintf("- **Severity**: %s\n", request.CommonLabels["severity"]))
	builder.WriteString(fmt.Sprintf("- **Started At**: %s\n", alert.StartsAt.Format("2006-01-02T15:04:05Z07:00")))

	if message := strings.TrimRight(alert.Annotations["message"], "\n"); message != "" {
		builder.WriteString(fmt.Sprintf("\n## Message\n\n%s\n", message))
	}

	builder.WriteString("\n## Labels\n\n")
	builder.WriteString("| Key | Value |\n")
	builder.WriteString("|-----|-------|\n")
	keys := make([]string, 0, len(alert.Labels))
	for key := range alert.Labels {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		builder.WriteString(fmt.Sprintf("| %s | %s |\n", key, alert.Labels[key]))
	}

	if alert.GeneratorURL != "" || request.ExternalURL != "" {
		builder.WriteString("\n## Links\n\n")
		if alert.GeneratorURL != "" {
			builder.WriteString(fmt.Sprintf("- [View in Prometheus](%s)\n", alert.GeneratorURL))
		}
		if request.ExternalURL != "" {
			builder.WriteString(fmt.Sprintf("- [Alertmanager UI](%s)\n", request.ExternalURL))
		}
	}

	return builder.String()
}
