package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	cloudevents "github.com/cloudevents/sdk-go/v2"
)

var targetURL string

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

func handle(event cloudevents.Event) cloudevents.Result {
	body := event.Data()

	request, err := http.NewRequest(http.MethodPost, targetURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	if event.DataContentType() != "" {
		request.Header.Set("Content-Type", event.DataContentType())
	}

	request.Header.Set("Ce-Id", event.ID())
	request.Header.Set("Ce-Type", event.Type())
	request.Header.Set("Ce-Source", event.Source())
	if event.Subject() != "" {
		request.Header.Set("Ce-Subject", event.Subject())
	}

	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer func() {
		_ = response.Body.Close()
	}()

	if response.StatusCode >= 500 {
		responseBody, _ := io.ReadAll(response.Body)
		return fmt.Errorf("upstream returned status %d, body=%s", response.StatusCode, string(responseBody))
	}

	if response.StatusCode >= 400 {
		responseBody, _ := io.ReadAll(response.Body)
		log.Printf("upstream returned status %d, body=%s, not retrying", response.StatusCode, string(responseBody))
		return cloudevents.ResultACK
	}

	_, _ = io.Copy(io.Discard, response.Body)

	return cloudevents.ResultACK
}

func main() {
	flag.StringVar(&targetURL, "target-url", envOrDefaultValue("TARGET_URL", ""), "Target URL to forward CloudEvents to")
	flag.Parse()

	if targetURL == "" {
		log.Fatal("--target-url or TARGET_URL is required")
	}

	http.DefaultTransport.(*http.Transport).MaxIdleConnsPerHost = http.DefaultTransport.(*http.Transport).MaxIdleConns

	c, err := cloudevents.NewDefaultClient()
	if err != nil {
		log.Fatalf("failed to create client: %+v", err)
	}

	if err := c.StartReceiver(context.Background(), handle); err != nil {
		log.Fatalf("failed to start receiver: %+v", err)
	}
}
