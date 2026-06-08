package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"time"

	cloudevents "github.com/cloudevents/sdk-go/v2"
)

func envOrDefaultValue[T any](key string, defaultValue T) T {
	value, exists := os.LookupEnv(key)
	if !exists {
		return defaultValue
	}

	switch any(defaultValue).(type) {
	case string:
		return any(value).(T)
	case int:
		if intValue, err := strconv.Atoi(value); err == nil {
			return any(intValue).(T)
		}
	case int64:
		if intValue, err := strconv.ParseInt(value, 10, 64); err == nil {
			return any(intValue).(T)
		}
	case uint:
		if uintValue, err := strconv.ParseUint(value, 10, 0); err == nil {
			return any(uint(uintValue)).(T)
		}
	case uint64:
		if uintValue, err := strconv.ParseUint(value, 10, 64); err == nil {
			return any(uintValue).(T)
		}
	case float64:
		if floatValue, err := strconv.ParseFloat(value, 64); err == nil {
			return any(floatValue).(T)
		}
	case bool:
		if boolValue, err := strconv.ParseBool(value); err == nil {
			return any(boolValue).(T)
		}
	case time.Duration:
		if durationValue, err := time.ParseDuration(value); err == nil {
			return any(durationValue).(T)
		}
	}

	return defaultValue
}

type AlertmanagerAlert struct {
	Labels      map[string]string `json:"labels"`
	Annotations map[string]string `json:"annotations"`
	StartsAt    time.Time         `json:"startsAt,omitempty"`
}

func main() {
	alertmanagerURL := envOrDefaultValue("ALERTMANAGER_URL", "http://mimir-alertmanager.mimir.svc.cluster.local:3100/alertmanager/api/v1/alerts")

	handle := func(event cloudevents.Event) cloudevents.Result {
		var alerts []AlertmanagerAlert
		if err := event.DataAs(&alerts); err != nil {
			var alert AlertmanagerAlert
			if err := event.DataAs(&alert); err != nil {
				return cloudevents.ResultACK
			}
			alerts = []AlertmanagerAlert{alert}
		}

		if len(alerts) == 0 {
			return cloudevents.ResultACK
		}

		payload, err := json.Marshal(alerts)
		if err != nil {
			return cloudevents.ResultACK
		}

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		request, err := http.NewRequestWithContext(ctx, http.MethodPost, alertmanagerURL, bytes.NewReader(payload))
		if err != nil {
			return cloudevents.ResultACK
		}
		request.Header.Set("Content-Type", "application/json")

		response, err := http.DefaultClient.Do(request)
		if err != nil {
			return fmt.Errorf("failed to post alert: %w", err)
		}
		defer func() {
			_, _ = io.Copy(io.Discard, response.Body)
			_ = response.Body.Close()
		}()

		if response.StatusCode >= 500 {
			return fmt.Errorf("alertmanager returned status %d", response.StatusCode)
		}

		if response.StatusCode >= 400 {
			responseBody, _ := io.ReadAll(response.Body)
			slog.Error("alertmanager returned client error", "status", response.StatusCode, "body", string(responseBody))
			return cloudevents.ResultACK
		}

		return cloudevents.ResultACK
	}

	c, err := cloudevents.NewClientHTTP()
	if err != nil {
		log.Fatalf("failed to create client: %+v", err)
	}

	if err := c.StartReceiver(context.Background(), handle); err != nil {
		log.Fatalf("failed to start receiver: %+v", err)
	}
}
