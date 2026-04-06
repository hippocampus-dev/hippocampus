package image

import "image"

type DiffResult struct {
	Image      image.Image
	DiffRatio float64
}

type Differ interface {
	Calculate(baseline image.Image, target image.Image) *DiffResult
}
