package image

import "image"

type DiffResult struct {
	Image      image.Image
	DiffAmount float64
}

type Differ interface {
	Calculate(baseline image.Image, target image.Image) *DiffResult
}
