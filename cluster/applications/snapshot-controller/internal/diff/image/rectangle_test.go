package image

import (
	"image"
	"image/color"
	"image/draw"
	"testing"
)

func createRectTestImage(width, height int, c color.Color) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	draw.Draw(img, img.Bounds(), &image.Uniform{C: c}, image.Point{}, draw.Src)
	return img
}

func TestRectangleDiff_Calculate(t *testing.T) {
	rd := NewRectangleDiff()

	t.Run("NoDifference", func(t *testing.T) {
		img1 := createRectTestImage(100, 100, color.White)
		img2 := createRectTestImage(100, 100, color.White)

		result := rd.Calculate(img1, img2)

		if result.DiffAmount != 0.0 {
			t.Errorf("Expected DiffAmount to be 0.0, got %f", result.DiffAmount)
		}
	})

	t.Run("CompleteDifference", func(t *testing.T) {
		img1 := createRectTestImage(100, 100, color.White)
		img2 := createRectTestImage(100, 100, color.Black)

		result := rd.Calculate(img1, img2)

		if result.DiffAmount == 0.0 {
			t.Errorf("Expected DiffAmount to be greater than 0, got %f", result.DiffAmount)
		}
	})

	t.Run("PartialDifference", func(t *testing.T) {
		img1 := createRectTestImage(100, 100, color.White)
		img2 := createRectTestImage(100, 100, color.White)

		for y := 0; y < 50; y++ {
			for x := 0; x < 100; x++ {
				img2.Set(x, y, color.Black)
			}
		}

		result := rd.Calculate(img1, img2)

		if result.DiffAmount == 0.0 {
			t.Errorf("Expected DiffAmount to be greater than 0, got %f", result.DiffAmount)
		}
	})

	t.Run("SameImageInstance", func(t *testing.T) {
		img := createRectTestImage(100, 100, color.White)

		result := rd.Calculate(img, img)

		if result.DiffAmount != 0.0 {
			t.Errorf("Expected DiffAmount to be 0.0 for same image instance, got %f", result.DiffAmount)
		}
	})
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
