package capture

import (
	"context"
)

type CaptureResult struct {
	Screenshot []byte
	HTML       []byte
}

type CaptureOptions struct {
	MaskSelectors []string
	Headers       map[string]string
}

func NewCaptureOptions() CaptureOptions {
	return CaptureOptions{
		MaskSelectors: make([]string, 0),
		Headers:       make(map[string]string),
	}
}

type Capturer interface {
	Capture(ctx context.Context, url string, captureOptions CaptureOptions) (*CaptureResult, error)
}
