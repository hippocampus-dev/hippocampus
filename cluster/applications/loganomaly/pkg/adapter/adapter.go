package adapter

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"loganomaly/internal/event"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	"github.com/go-playground/validator/v10"
	"golang.org/x/xerrors"
)

const (
	summaryMaxLength = 1500
	summaryFence     = "```"
)

func severity(detectionMode string) string {
	if detectionMode == event.DetectionModeImmediate {
		return "critical"
	}
	return "warning"
}

func buildGrafanaURL(grafanaBase string, grouping string) string {
	query := fmt.Sprintf(`{grouping="%s"} | json`, grouping)
	params := url.Values{}
	params.Set("schemaVersion", "1")
	params.Set("panes", fmt.Sprintf(`{"a":{"datasource":"loki","queries":[{"refId":"A","expr":"%s"}],"range":{"from":"now-1h","to":"now"}}}`, query))
	params.Set("orgId", "1")
	return fmt.Sprintf("%s/explore?%s", grafanaBase, params.Encode())
}

func fenceSummary(summary string) string {
	fenced := strings.ReplaceAll(summary, summaryFence, "'''")
	runes := []rune(fenced)
	if len(runes) > summaryMaxLength {
		fenced = fmt.Sprintf("%s...", string(runes[:summaryMaxLength]))
	}
	return fmt.Sprintf("%s\n%s\n%s", summaryFence, fenced, summaryFence)
}

func buildMessage(data event.AnomalyEvent, logsURL string) string {
	var builder strings.Builder
	if data.DetectionMode == event.DetectionModeImmediate {
		builder.WriteString(fmt.Sprintf("A fatal pattern has been detected in %s grouping.\n\n", data.Grouping))
		builder.WriteString(fmt.Sprintf("%s\n\n", fenceSummary(data.Summary)))
	} else {
		// The windowed summary already states the count, the rate and the z-score.
		builder.WriteString(fmt.Sprintf("%s has been detected in %s grouping over %s.\n\n", data.Summary, data.Grouping, data.Window))
	}
	if data.Pod != "" {
		// Suppression keys on the grouping and the normalized message, so replicas hitting the same pattern collapse onto this one.
		builder.WriteString(fmt.Sprintf("- **Example Pod**: %s\n", data.Pod))
	}
	builder.WriteString(fmt.Sprintf("- **Active Error Groupings**: %d\n", data.ActiveErrorGroupings))
	builder.WriteString(fmt.Sprintf("\n[View logs in Grafana](%s)\n", logsURL))
	return builder.String()
}

func Run(a *Args) error {
	if err := validator.New().Struct(a); err != nil {
		return xerrors.Errorf("invalid arguments: %w", err)
	}

	handle := func(ctx context.Context, e cloudevents.Event) (*cloudevents.Event, cloudevents.Result) {
		var data event.AnomalyEvent
		if err := e.DataAs(&data); err != nil {
			return nil, cloudevents.ResultACK
		}

		logsURL := buildGrafanaURL(a.GrafanaBase, data.Grouping)

		labels := map[string]string{
			"alertname":      fmt.Sprintf("loganomaly_%s", data.Grouping),
			"grouping":       data.Grouping,
			"detection_mode": data.DetectionMode,
			"severity":       severity(data.DetectionMode),
			"repository":     a.Repository,
		}
		// A windowed hash is a pure function of the grouping, which the alertname already carries.
		if data.DetectionMode == event.DetectionModeImmediate {
			labels["error_hash"] = data.ErrorHash
		}

		alert := event.AlertmanagerAlert{
			Labels: labels,
			Annotations: map[string]string{
				"message": buildMessage(data, logsURL),
			},
		}

		response := e.Clone()
		if err := response.SetData(cloudevents.ApplicationJSON, []event.AlertmanagerAlert{alert}); err != nil {
			return nil, cloudevents.ResultACK
		}

		return &response, cloudevents.ResultACK
	}

	c, err := cloudevents.NewClientHTTP()
	if err != nil {
		return xerrors.Errorf("failed to create client: %w", err)
	}

	if err := c.StartReceiver(context.Background(), handle); err != nil {
		return xerrors.Errorf("failed to start receiver: %w", err)
	}

	return nil
}
