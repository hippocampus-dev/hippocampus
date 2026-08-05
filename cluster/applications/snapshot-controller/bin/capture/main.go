package main

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"snapshot-controller/internal/capture"
	"snapshot-controller/internal/storage"
	"strconv"
	"strings"
	"time"

	"golang.org/x/sync/errgroup"
)

type SnapshotResult struct {
	ScreenshotPath string `json:"screenshotPath"`
	HTMLPath       string `json:"htmlPath"`
}

type headers []string

func (h *headers) String() string {
	return strings.Join(*h, ", ")
}

func (h *headers) Set(value string) error {
	*h = append(*h, value)
	return nil
}

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

func main() {
	var directory string
	var format string
	var maskSelectors string
	var delay time.Duration
	var viewportWidth int
	var viewportHeight int
	var chromeDevtoolsProtocolURL string
	var headers headers
	flag.StringVar(&directory, "directory", envOrDefaultValue("DIRECTORY", "/tmp"), "Output directory")
	flag.StringVar(&format, "format", envOrDefaultValue("FORMAT", "jpeg"), "Output format (jpeg or png)")
	flag.StringVar(&maskSelectors, "mask-selectors", envOrDefaultValue("MASK_SELECTORS", ""), "Comma-separated list of CSS selectors to mask during capture")
	flag.DurationVar(&delay, "delay", envOrDefaultValue("DELAY", 3*time.Second), "Delay before capturing")
	flag.IntVar(&viewportWidth, "viewport-width", envOrDefaultValue("VIEWPORT_WIDTH", 1920), "Viewport width in pixels")
	flag.IntVar(&viewportHeight, "viewport-height", envOrDefaultValue("VIEWPORT_HEIGHT", 1080), "Viewport height in pixels")
	flag.StringVar(&chromeDevtoolsProtocolURL, "chrome-devtools-protocol-url", envOrDefaultValue("CHROME_DEVTOOLS_PROTOCOL_URL", ""), "Connect to existing browser via Chrome DevTools Protocol URL (e.g., http://localhost:9222)")
	flag.Var(&headers, "H", "Add HTTP header (can be used multiple times, e.g., -H 'Accept: text/html' -H 'Authorization: Bearer token')")

	flag.Parse()

	args := flag.Args()
	if len(args) == 0 {
		log.Fatalf("url not specified")
	}
	url := args[0]

	ctx := context.Background()

	s, err := storage.NewFileStorage(ctx, storage.FileConfig{
		Directory: directory,
	})
	if err != nil {
		log.Fatalf("Failed to create storage backend: %v", err)
	}

	config := capture.DefaultPlaywrightConfig()
	if format != "" {
		config.Format = format
	}
	if delay > 0 {
		config.Delay = delay
	}
	if chromeDevtoolsProtocolURL != "" {
		config.ChromeDevtoolsProtocolURL = chromeDevtoolsProtocolURL
	}
	if display := os.Getenv("DISPLAY"); display != "" {
		config.Headless = false
	}
	if viewportWidth > 0 {
		config.ViewportWidth = viewportWidth
	}
	if viewportHeight > 0 {
		config.ViewportHeight = viewportHeight
	}

	capturer, err := capture.NewPlaywrightCapturer(ctx, config)
	if err != nil {
		log.Fatalf("Failed to create capturer: %v", err)
	}

	captureOptions := capture.NewCaptureOptions()
	if maskSelectors != "" {
		captureOptions.MaskSelectors = strings.Split(maskSelectors, ",")
		for i := range captureOptions.MaskSelectors {
			captureOptions.MaskSelectors[i] = strings.TrimSpace(captureOptions.MaskSelectors[i])
		}
	}
	if len(headers) > 0 {
		for _, header := range headers {
			parts := strings.SplitN(header, ":", 2)
			if len(parts) == 2 {
				key := strings.TrimSpace(parts[0])
				value := strings.TrimSpace(parts[1])
				captureOptions.Headers[key] = value
			}
		}
	}
	if headersEnvironmentVariable := os.Getenv("HEADERS"); headersEnvironmentVariable != "" {
		var m map[string]string
		if err := json.Unmarshal([]byte(headersEnvironmentVariable), &m); err == nil {
			for key, value := range m {
				captureOptions.Headers[key] = value
			}
		}
	}

	result, err := capturer.Capture(ctx, url, captureOptions)
	if err != nil {
		log.Fatalf("Failed to capture screenshot: %v", err)
	}

	timestamp := time.Now().Format("20060102150405")

	h := sha256.New()
	h.Write([]byte(url))
	urlHash := fmt.Sprintf("%x", h.Sum(nil))[:16]

	baseKey := fmt.Sprintf("Snapshot/capture/%s/%s", urlHash, timestamp)

	var imagePath string
	var htmlPath string

	{
		eg, ctx := errgroup.WithContext(ctx)

		eg.Go(func() error {
			imageKey := fmt.Sprintf("%s.%s", baseKey, config.Format)
			path, err := s.Put(ctx, imageKey, result.Screenshot)
			if err != nil {
				return err
			}
			imagePath = path
			return nil
		})

		eg.Go(func() error {
			htmlKey := fmt.Sprintf("%s.html", baseKey)
			path, err := s.Put(ctx, htmlKey, result.HTML)
			if err != nil {
				return err
			}
			htmlPath = path
			return nil
		})

		if err := eg.Wait(); err != nil {
			log.Fatalf("Failed to upload: %v", err)
		}
	}

	if err := json.NewEncoder(os.Stdout).Encode(SnapshotResult{
		ScreenshotPath: imagePath,
		HTMLPath:       htmlPath,
	}); err != nil {
		log.Fatalf("Failed to encode result: %v", err)
	}
}
