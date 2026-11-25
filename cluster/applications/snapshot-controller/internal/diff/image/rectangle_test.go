package image

import (
	"image"
	"image/color"
	"image/draw"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func createRectTestImage(width int, height int, c color.Color) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	draw.Draw(img, img.Bounds(), &image.Uniform{C: c}, image.Point{}, draw.Src)
	return img
}

func TestRectangleDiff_Calculate(t *testing.T) {
	sameImage := createRectTestImage(100, 100, color.White)

	tests := []struct {
		name          string
		baseline      *image.RGBA
		target        *image.RGBA
		wantDifferent bool
	}{
		{name: "NoDifference", baseline: createRectTestImage(100, 100, color.White), target: createRectTestImage(100, 100, color.White), wantDifferent: false},
		{name: "CompleteDifference", baseline: createRectTestImage(100, 100, color.White), target: createRectTestImage(100, 100, color.Black), wantDifferent: true},
		{name: "PartialDifference", baseline: createRectTestImage(100, 100, color.White), target: createHalfBlackTestImage(100, 100), wantDifferent: true},
		{name: "SameImageInstance", baseline: sameImage, target: sameImage, wantDifferent: false},
	}

	for _, tt := range tests {
		name := tt.name
		baseline := tt.baseline
		target := tt.target
		wantDifferent := tt.wantDifferent
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			got := NewRectangleDiff().Calculate(baseline, target)
			if diff := cmp.Diff(wantDifferent, got.DiffRatio != 0.0); diff != "" {
				t.Errorf("DiffRatio %f (-want +got different):\n%s", got.DiffRatio, diff)
			}
		})
	}
}

func BenchmarkRectangleDiff_Calculate_Small(b *testing.B) {
	rd := NewRectangleDiff()
	img1 := createRectTestImage(1920, 1080, color.White)
	img2 := createRectTestImage(1920, 1080, color.White)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rd.Calculate(img1, img2)
	}
}

func BenchmarkRectangleDiff_Calculate_Large(b *testing.B) {
	rd := NewRectangleDiff()
	img1 := createTestImage(3840, 2160, color.White)
	img2 := createTestImage(3840, 2160, color.White)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rd.Calculate(img1, img2)
	}
}
