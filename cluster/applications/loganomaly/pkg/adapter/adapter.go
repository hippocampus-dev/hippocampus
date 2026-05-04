package adapter

import (
	"context"
	"fmt"
	"net/url"
	"strconv"

	"loganomaly/internal/event"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	"github.com/go-playground/validator/v10"
	"golang.org/x/xerrors"
)

func severity(blastRadius int) string {
	if blastRadius >= 5 {
		return "critical"
	}
	if blastRadius >= 3 {
		return "warning"
	}
	return "info"
}

func buildGrafanaURL(grafanaBase string, grouping string) string {
	query := fmt.Sprintf(`{grouping="%s"} | json`, grouping)
	params := url.Values{}
	params.Set("schemaVersion", "1")
	params.Set("panes", fmt.Sprintf(`{"a":{"datasource":"loki","queries":[{"refId":"A","expr":"%s"}],"range":{"from":"now-1h","to":"now"}}}`, query))
	params.Set("orgId", "1")
	return fmt.Sprintf("%s/explore?%s", grafanaBase, params.Encode())
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

		alert := event.AlertmanagerAlert{
			Labels: map[string]string{
				"alertname":      fmt.Sprintf("loganomaly_%s", data.Grouping),
				"grouping":       data.Grouping,
				"detection_mode": data.DetectionMode,
				"severity":       severity(data.BlastRadius),
			},
			Annotations: map[string]string{
				"summary":      data.Summary,
				"logs_url":     buildGrafanaURL(a.GrafanaBase, data.Grouping),
				"count":        strconv.Itoa(data.Count),
				"window":       data.Window,
				"z_score":      fmt.Sprintf("%.2f", data.ZScore),
				"blast_radius": strconv.Itoa(data.BlastRadius),
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
