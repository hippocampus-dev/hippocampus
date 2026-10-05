package handler

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/google/go-github/v68/github"
	"golang.org/x/xerrors"
)

// buildTitle carries alertname, severity, namespace and pod, recordOccurrence carries error_hash, and the issue is opened in the repository the label names
var renderedElsewhere = map[string]struct{}{
	"alertname":  {},
	"severity":   {},
	"namespace":  {},
	"pod":        {},
	"error_hash": {},
	"repository": {},
}

type CriticalAlertHandler struct {
	client           *github.Client
	timeout          time.Duration
	grafanaAlertsURL string
}

type openIssue struct {
	number int
	labels map[string]struct{}
}

func NewCriticalAlertHandler(client *github.Client, timeout time.Duration, grafanaAlertsURL string) *CriticalAlertHandler {
	return &CriticalAlertHandler{
		client:           client,
		timeout:          timeout,
		grafanaAlertsURL: grafanaAlertsURL,
	}
}

func (h *CriticalAlertHandler) Call(request *AlertManagerRequest) error {
	ctx, cancel := context.WithTimeout(context.Background(), h.timeout)
	defer cancel()

	var errs []error
	// Per-repository listing costs repositories x pages, and guards duplicates
	opened := make(map[string]map[string]openIssue)
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
		issues, ok := opened[slug]
		if !ok {
			listed, err := h.listOpenIssues(ctx, owner, repository)
			if err != nil {
				errs = append(errs, xerrors.Errorf("alert %s: failed to list issues: %w", request.CommonLabels["alertname"], err))
				continue
			}
			opened[slug] = listed
			issues = listed
		}

		severity := request.CommonLabels["severity"]
		title := h.buildTitle(request.CommonLabels["alertname"], severity, alert.Labels)
		occurrence := h.buildOccurrenceLabel(alert.Labels["error_hash"])

		if existing, ok := issues[title]; ok {
			if err := h.recordOccurrence(ctx, owner, repository, existing, occurrence, alert); err != nil {
				errs = append(errs, xerrors.Errorf("alert %s: %w", request.CommonLabels["alertname"], err))
			}
			continue
		}

		labels := []string{"alert"}
		if severity != "" {
			labels = append(labels, severity)
		}
		if occurrence != "" {
			labels = append(labels, occurrence)
		}

		issueRequest := &github.IssueRequest{
			Title:  github.Ptr(title),
			Body:   github.Ptr(h.buildBody(alert)),
			Labels: &labels,
		}

		issue, _, err := h.client.Issues.Create(ctx, owner, repository, issueRequest)
		if err != nil {
			log.Printf("Failed to create issue for alert %s: %+v", request.CommonLabels["alertname"], err)
			errs = append(errs, xerrors.Errorf("alert %s: failed to create issue: %w", request.CommonLabels["alertname"], err))
			continue
		}

		// Reading them back tells apart the labels the create accepted from the ones it asked for
		issues[title] = openIssue{number: issue.GetNumber(), labels: h.buildLabelSet(issue.Labels)}

		log.Printf("Created GitHub issue #%d (%s) for alert %s", issue.GetNumber(), issue.GetHTMLURL(), request.CommonLabels["alertname"])
	}

	return errors.Join(errs...)
}

func (h *CriticalAlertHandler) recordOccurrence(ctx context.Context, owner string, repository string, issue openIssue, occurrence string, alert Alert) error {
	// Alertmanager resends a firing alert, and only the occurrence label tells that resend from a new one
	if occurrence == "" {
		log.Printf("Skipped commenting on #%d: the alert carries no error_hash", issue.number)
		return nil
	}
	if _, ok := issue.labels[occurrence]; ok {
		log.Printf("Skipped commenting on #%d: %s is already recorded", issue.number, occurrence)
		return nil
	}

	comment := &github.IssueComment{Body: github.Ptr(h.buildBody(alert))}
	if _, _, err := h.client.Issues.CreateComment(ctx, owner, repository, issue.number, comment); err != nil {
		return xerrors.Errorf("failed to comment on issue #%d: %w", issue.number, err)
	}
	issue.labels[occurrence] = struct{}{}

	// main.go answers a returned error with a 500 and Alertmanager resends the same batch, so returning here would repost the comment on every retry
	// Past GitHub's cap of 100 labels per issue every further add answers 422 Validation Failed, and each occurrence beyond it reposts its comment at every notification
	// A failed label therefore costs only the guard against the next notification
	if _, _, err := h.client.Issues.AddLabelsToIssue(ctx, owner, repository, issue.number, []string{occurrence}); err != nil {
		log.Printf("Failed to label issue #%d with %s: %+v", issue.number, occurrence, err)
	}

	log.Printf("Commented on GitHub issue #%d for %s", issue.number, occurrence)

	return nil
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

func (h *CriticalAlertHandler) listOpenIssues(ctx context.Context, owner string, repository string) (map[string]openIssue, error) {
	opened := make(map[string]openIssue)
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
			// The list-issues endpoint returns pull requests too, and one sharing a title takes every comment and label meant for the issue
			if issue == nil || issue.IsPullRequest() {
				continue
			}
			opened[issue.GetTitle()] = openIssue{number: issue.GetNumber(), labels: h.buildLabelSet(issue.Labels)}
		}

		if response.NextPage == 0 {
			return opened, nil
		}
		options.Page = response.NextPage
	}
}

func (h *CriticalAlertHandler) buildTitle(alertname string, severity string, labels map[string]string) string {
	// The title is the only key listOpenIssues matches on, so a value that varies between occurrences of the same alert belongs on a label instead
	// Each notification then opens another issue, which nothing reports because opening one is the success path
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
	return title
}

func (h *CriticalAlertHandler) buildLabelSet(labels []*github.Label) map[string]struct{} {
	set := make(map[string]struct{}, len(labels))
	for _, label := range labels {
		set[label.GetName()] = struct{}{}
	}
	return set
}

func (h *CriticalAlertHandler) buildOccurrenceLabel(errorHash string) string {
	if errorHash == "" {
		return ""
	}
	return fmt.Sprintf("error_hash-%s", errorHash)
}

// Grafana reads the alert groups filter from queryString, and the page lists every group of the Alertmanager without it
func (h *CriticalAlertHandler) buildAlertsURL(alertname string) string {
	if h.grafanaAlertsURL == "" || alertname == "" {
		return h.grafanaAlertsURL
	}

	parsed, err := url.Parse(h.grafanaAlertsURL)
	if err != nil {
		return h.grafanaAlertsURL
	}
	query := parsed.Query()
	query.Set("queryString", fmt.Sprintf("alertname=%q", alertname))
	parsed.RawQuery = query.Encode()

	return parsed.String()
}

func (h *CriticalAlertHandler) buildBody(alert Alert) string {
	var builder strings.Builder

	builder.WriteString("## Alert Details\n\n")
	builder.WriteString(fmt.Sprintf("- **Started At**: %s\n", alert.StartsAt.Format("2006-01-02T15:04:05Z07:00")))

	if message := strings.TrimRight(alert.Annotations["message"], "\n"); message != "" {
		builder.WriteString(fmt.Sprintf("\n## Message\n\n%s\n", message))
	}

	keys := make([]string, 0, len(alert.Labels))
	for key := range alert.Labels {
		if _, ok := renderedElsewhere[key]; ok {
			continue
		}
		keys = append(keys, key)
	}
	sort.Strings(keys)
	if len(keys) > 0 {
		builder.WriteString("\n## Labels\n\n")
		builder.WriteString("| Key | Value |\n")
		builder.WriteString("|-----|-------|\n")
		for _, key := range keys {
			builder.WriteString(fmt.Sprintf("| %s | %s |\n", key, alert.Labels[key]))
		}
	}

	// The payload's externalURL is the Alertmanager's own address, which nothing outside the cluster resolves
	alertsURL := h.buildAlertsURL(alert.Labels["alertname"])
	if alert.GeneratorURL != "" || alertsURL != "" {
		builder.WriteString("\n## Links\n\n")
		if alert.GeneratorURL != "" {
			builder.WriteString(fmt.Sprintf("- [View in Prometheus](%s)\n", alert.GeneratorURL))
		}
		if alertsURL != "" {
			builder.WriteString(fmt.Sprintf("- [View in Grafana](%s)\n", alertsURL))
		}
	}

	return builder.String()
}
