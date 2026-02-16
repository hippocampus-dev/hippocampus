package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"flag"
	"fmt"
	"image"
	_ "image/jpeg"
	"image/png"
	"log"
	"os"
	diffimage "snapshot-controller/internal/diff/image"
	difftext "snapshot-controller/internal/diff/text"
	"snapshot-controller/internal/storage"
	"strconv"
	"time"
)

type DiffOutput struct {
	DiffPath   string  `json:"diffPath"`
	DiffAmount float64 `json:"diffAmount"`
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
	flag.StringVar(&directory, "directory", envOrDefaultValue("DIRECTORY", "/tmp"), "Output directory")
	flag.StringVar(&format, "format", envOrDefaultValue("FORMAT", "pixel"), "Output format (pixel, rectangle, line, dom)")

	flag.Parse()

	args := flag.Args()
	if len(args) < 2 {
		log.Fatalf("baseline, target not specified")
	}

	ctx := context.Background()
	s, err := storage.NewFileStorage(ctx, storage.FileConfig{
		Directory: directory,
	})
	if err != nil {
		log.Fatalf("Failed to create storage backend: %v", err)
	}

	baselinePath := args[0]
	targetPath := args[1]

	timestamp := time.Now().Format("20060102150405")

	h := sha256.New()
	h.Write([]byte(baselinePath + targetPath))
	hash := fmt.Sprintf("%x", h.Sum(nil))[:16]

	var diffPath string
	var diffAmount float64
	switch format {
	case "pixel":
		baselineImage, err := loadImage(baselinePath)
		if err != nil {
			log.Fatalf("Failed to load baseline image: %v", err)
		}

		targetImage, err := loadImage(targetPath)
		if err != nil {
			log.Fatalf("Failed to load target image: %v", err)
		}

		diffResult := diffimage.NewPixelDiff(0.1).Calculate(baselineImage, targetImage)

		var buffer bytes.Buffer
		if err := png.Encode(&buffer, diffResult.Image); err != nil {
			log.Fatalf("Failed to encode diff image: %v", err)
		}

		key := fmt.Sprintf("Snapshot/diff/%s/%s.png", hash, timestamp)
		diffPath, err = s.Put(ctx, key, buffer.Bytes())
		if err != nil {
			log.Fatalf("Failed to save diff image: %v", err)
		}
		diffAmount = diffResult.DiffAmount
	case "rectangle":
		baselineImage, err := loadImage(baselinePath)
		if err != nil {
			log.Fatalf("Failed to load baseline image: %v", err)
		}

		targetImage, err := loadImage(targetPath)
		if err != nil {
			log.Fatalf("Failed to load target image: %v", err)
		}

		diffResult := diffimage.NewRectangleDiff().Calculate(baselineImage, targetImage)

		var buffer bytes.Buffer
		if err := png.Encode(&buffer, diffResult.Image); err != nil {
			log.Fatalf("Failed to encode diff image: %v", err)
		}

		key := fmt.Sprintf("Snapshot/diff/%s/%s.png", hash, timestamp)
		diffPath, err = s.Put(ctx, key, buffer.Bytes())
		if err != nil {
			log.Fatalf("Failed to save diff image: %v", err)
		}
		diffAmount = diffResult.DiffAmount
	case "line":
		baselineHTML, err := os.ReadFile(baselinePath)
		if err != nil {
			log.Fatalf("Failed to read baseline HTML file: %v", err)
		}

		targetHTML, err := os.ReadFile(targetPath)
		if err != nil {
			log.Fatalf("Failed to read target HTML file: %v", err)
		}

		diffResult, err := difftext.NewLineDiff().Calculate(baselineHTML, targetHTML)
		if err != nil {
			log.Fatalf("Failed to calculate line diff: %v", err)
		}

		var buffer bytes.Buffer
		buffer.Write(diffResult.Diff)

		key := fmt.Sprintf("Snapshot/diff/%s/%s.txt", hash, timestamp)
		diffPath, err = s.Put(ctx, key, buffer.Bytes())
		if err != nil {
			log.Fatalf("Failed to save diff image: %v", err)
		}
		diffAmount = diffResult.DiffAmount
	case "dom":
		baselineHTML, err := os.ReadFile(baselinePath)
		if err != nil {
			log.Fatalf("Failed to read baseline HTML file: %v", err)
		}

		targetHTML, err := os.ReadFile(targetPath)
		if err != nil {
			log.Fatalf("Failed to read target HTML file: %v", err)
		}

		diffResult, err := difftext.NewDOMDiff().Calculate(baselineHTML, targetHTML)
		if err != nil {
			log.Fatalf("Failed to calculate DOM diff: %v", err)
		}

		var buffer bytes.Buffer
		buffer.Write(diffResult.Diff)

		key := fmt.Sprintf("Snapshot/diff/%s/%s.txt", hash, timestamp)
		diffPath, err = s.Put(ctx, key, buffer.Bytes())
		if err != nil {
			log.Fatalf("Failed to save diff file: %v", err)
		}
		diffAmount = diffResult.DiffAmount
	default:
		log.Fatalf("Unknown diff type: %s", format)
	}

	if err := json.NewEncoder(os.Stdout).Encode(DiffOutput{
		DiffPath:   diffPath,
		DiffAmount: diffAmount,
	}); err != nil {
		log.Fatalf("Failed to encode result: %v", err)
	}
}

func loadImage(path string) (image.Image, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	i, _, err := image.Decode(file)
	if err != nil {
		return nil, err
	}

	return i, nil
}
