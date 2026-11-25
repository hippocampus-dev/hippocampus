package image

import (
	"image"
	"image/color"
	"image/draw"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func createTestImage(width int, height int, c color.Color) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	draw.Draw(img, img.Bounds(), &image.Uniform{C: c}, image.Point{}, draw.Src)
	return img
}

func createHalfBlackTestImage(width int, height int) *image.RGBA {
	img := createTestImage(width, height, color.White)
	for y := 0; y < height/2; y++ {
		for x := 0; x < width; x++ {
			img.Set(x, y, color.Black)
		}
	}
	return img
}

func TestPixelDiff_Calculate(t *testing.T) {
	sameImage := createTestImage(100, 100, color.White)

	tests := []struct {
		name     string
		baseline *image.RGBA
		target   *image.RGBA
		want     float64
	}{
		{name: "NoDifference", baseline: createTestImage(100, 100, color.White), target: createTestImage(100, 100, color.White), want: 0.0},
		{name: "CompleteDifference", baseline: createTestImage(100, 100, color.White), target: createTestImage(100, 100, color.Black), want: 1.0},
		{name: "PartialDifference", baseline: createTestImage(100, 100, color.White), target: createHalfBlackTestImage(100, 100), want: 0.5},
		{name: "SameImageInstance", baseline: sameImage, target: sameImage, want: 0.0},
	}

	for _, tt := range tests {
		name := tt.name
		baseline := tt.baseline
		target := tt.target
		want := tt.want
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			got := NewPixelDiff(0.1).Calculate(baseline, target)
			if diff := cmp.Diff(want, got.DiffRatio); diff != "" {
				t.Errorf("(-want +got):\n%s", diff)
			}
		})
	}
}

func BenchmarkPixelDiff_Calculate_Small(b *testing.B) {
	pd := NewPixelDiff(0.1)
	img1 := createTestImage(1920, 1080, color.White)
	img2 := createTestImage(1920, 1080, color.White)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		pd.Calculate(img1, img2)
	}
}

func BenchmarkPixelDiff_Calculate_Large(b *testing.B) {
	pd := NewPixelDiff(0.1)
	img1 := createTestImage(3840, 2160, color.White)
	img2 := createTestImage(3840, 2160, color.White)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		pd.Calculate(img1, img2)
	}
}
