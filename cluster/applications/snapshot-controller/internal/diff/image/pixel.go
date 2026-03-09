package image

import (
	"image"
	"image/color"
	"image/draw"
	"runtime"
	"sync"
	"sync/atomic"
)

type PixelDiff struct {
	threshold float64
}

func NewPixelDiff(threshold float64) *PixelDiff {
	return &PixelDiff{
		threshold,
	}
}

func (p *PixelDiff) Calculate(baseline image.Image, target image.Image) *DiffResult {
	if baseline == target {
		return &DiffResult{
			Image:      baseline,
			DiffAmount: 0.0,
		}
	}

	bounds := p.calculateUnionBounds(baseline, target)
	diff := image.NewRGBA(bounds)
	draw.Draw(diff, bounds, &image.Uniform{C: color.White}, image.Point{}, draw.Src)

	var addedPixelCount int64
	var removedPixelCount int64
	totalPixelCount := int64((bounds.Max.Y - bounds.Min.Y) * (bounds.Max.X - bounds.Min.X))

	baselineRGBA, baselineIsRGBA := baseline.(*image.RGBA)
	targetRGBA, targetIsRGBA := target.(*image.RGBA)
	baselineNRGBA, baselineIsNRGBA := baseline.(*image.NRGBA)
	targetNRGBA, targetIsNRGBA := target.(*image.NRGBA)
	baselineRGBA64, baselineIsRGBA64 := baseline.(*image.RGBA64)
	targetRGBA64, targetIsRGBA64 := target.(*image.RGBA64)
	baselineNRGBA64, baselineIsNRGBA64 := baseline.(*image.NRGBA64)
	targetNRGBA64, targetIsNRGBA64 := target.(*image.NRGBA64)
	baselineYCbCr, baselineIsYCbCr := baseline.(*image.YCbCr)
	targetYCbCr, targetIsYCbCr := target.(*image.YCbCr)

	// Use GOMAXPROCS instead of runtime.NumCPU() to consider cgroup.
	// https://tip.golang.org/doc/go1.25#container-aware-gomaxprocs
	numWorkers := runtime.GOMAXPROCS(0)

	height := bounds.Max.Y - bounds.Min.Y
	rowsPerWorker := height / numWorkers

	var wg sync.WaitGroup
	wg.Add(numWorkers)

	if baselineIsRGBA && targetIsRGBA && diff.Bounds().Eq(bounds) {
		for i := 0; i < numWorkers; i++ {
			startY := bounds.Min.Y + i*rowsPerWorker
			endY := startY + rowsPerWorker
			if i == numWorkers-1 {
				endY = bounds.Max.Y
			}

			go func(startY int, endY int) {
				defer wg.Done()
				p.processRGBA(baselineRGBA, targetRGBA, diff, bounds.Min.X, bounds.Max.X, startY, endY, &addedPixelCount, &removedPixelCount)
			}(startY, endY)
		}
	} else if baselineIsNRGBA && targetIsNRGBA && diff.Bounds().Eq(bounds) {
		for i := 0; i < numWorkers; i++ {
			startY := bounds.Min.Y + i*rowsPerWorker
			endY := startY + rowsPerWorker
			if i == numWorkers-1 {
				endY = bounds.Max.Y
			}

			go func(startY int, endY int) {
				defer wg.Done()
				p.processNRGBA(baselineNRGBA, targetNRGBA, diff, bounds.Min.X, bounds.Max.X, startY, endY, &addedPixelCount, &removedPixelCount)
			}(startY, endY)
		}
	} else if baselineIsRGBA64 && targetIsRGBA64 && diff.Bounds().Eq(bounds) {
		for i := 0; i < numWorkers; i++ {
			startY := bounds.Min.Y + i*rowsPerWorker
			endY := startY + rowsPerWorker
			if i == numWorkers-1 {
				endY = bounds.Max.Y
			}

			go func(startY int, endY int) {
				defer wg.Done()
				p.processRGBA64(baselineRGBA64, targetRGBA64, diff, bounds.Min.X, bounds.Max.X, startY, endY, &addedPixelCount, &removedPixelCount)
			}(startY, endY)
		}
	} else if baselineIsNRGBA64 && targetIsNRGBA64 && diff.Bounds().Eq(bounds) {
		for i := 0; i < numWorkers; i++ {
			startY := bounds.Min.Y + i*rowsPerWorker
			endY := startY + rowsPerWorker
			if i == numWorkers-1 {
				endY = bounds.Max.Y
			}

			go func(startY int, endY int) {
				defer wg.Done()
				p.processNRGBA64(baselineNRGBA64, targetNRGBA64, diff, bounds.Min.X, bounds.Max.X, startY, endY, &addedPixelCount, &removedPixelCount)
			}(startY, endY)
		}
	} else if baselineIsYCbCr && targetIsYCbCr && diff.Bounds().Eq(bounds) {
		for i := 0; i < numWorkers; i++ {
			startY := bounds.Min.Y + i*rowsPerWorker
			endY := startY + rowsPerWorker
			if i == numWorkers-1 {
				endY = bounds.Max.Y
			}

			go func(startY int, endY int) {
				defer wg.Done()
				p.processYCbCr(baselineYCbCr, targetYCbCr, diff, bounds.Min.X, bounds.Max.X, startY, endY, &addedPixelCount, &removedPixelCount)
			}(startY, endY)
		}
	} else {
		for i := 0; i < numWorkers; i++ {
			startY := bounds.Min.Y + i*rowsPerWorker
			endY := startY + rowsPerWorker
			if i == numWorkers-1 {
				endY = bounds.Max.Y
			}

			go func(startY int, endY int) {
				defer wg.Done()
				p.processGeneric(baseline, target, diff, bounds.Min.X, bounds.Max.X, startY, endY, &addedPixelCount, &removedPixelCount)
			}(startY, endY)
		}
	}

	wg.Wait()

	diffAmount := 0.0
	if totalPixelCount > 0 {
		diffAmount = float64(addedPixelCount+removedPixelCount) / float64(totalPixelCount)
	}

	return &DiffResult{
		Image:      diff,
		DiffAmount: diffAmount,
	}
}

func (p *PixelDiff) processRGBA(baseline *image.RGBA, target *image.RGBA, diff *image.RGBA, minX int, maxX int, startY int, endY int, addedCount *int64, removedCount *int64) {
	var localAdded int64
	var localRemoved int64

	for y := startY; y < endY; y++ {
		baselineRowStart := baseline.PixOffset(minX, y)
		targetRowStart := target.PixOffset(minX, y)
		diffRowStart := diff.PixOffset(minX, y)

		for x := 0; x < (maxX - minX); x++ {
			baselineOffset := baselineRowStart + x*4
			targetOffset := targetRowStart + x*4
			diffOffset := diffRowStart + x*4

			if baselineOffset >= 0 && baselineOffset+3 < len(baseline.Pix) &&
				targetOffset >= 0 && targetOffset+3 < len(target.Pix) {
				br := baseline.Pix[baselineOffset]
				bg := baseline.Pix[baselineOffset+1]
				bb := baseline.Pix[baselineOffset+2]
				ba := baseline.Pix[baselineOffset+3]

				tr := target.Pix[targetOffset]
				tg := target.Pix[targetOffset+1]
				tb := target.Pix[targetOffset+2]
				ta := target.Pix[targetOffset+3]

				if br == tr && bg == tg && bb == tb && ba == ta {
					diff.Pix[diffOffset] = br
					diff.Pix[diffOffset+1] = bg
					diff.Pix[diffOffset+2] = bb
					diff.Pix[diffOffset+3] = ba
				} else {
					dr, dg, db, da := p.getDiffColor(br, bg, bb, ba, tr, tg, tb, ta)

					diff.Pix[diffOffset] = dr
					diff.Pix[diffOffset+1] = dg
					diff.Pix[diffOffset+2] = db
					diff.Pix[diffOffset+3] = da

					if dr == 255 && dg == 0 && db == 0 {
						localAdded++
					} else if dr == 0 && dg == 0 && db == 255 {
						localRemoved++
					}
				}
			} else {
				diff.Pix[diffOffset] = 255
				diff.Pix[diffOffset+1] = 255
				diff.Pix[diffOffset+2] = 255
				diff.Pix[diffOffset+3] = 255
			}
		}
	}

	atomic.AddInt64(addedCount, localAdded)
	atomic.AddInt64(removedCount, localRemoved)
}

func (p *PixelDiff) processNRGBA(baseline *image.NRGBA, target *image.NRGBA, diff *image.RGBA, minX int, maxX int, startY int, endY int, addedCount *int64, removedCount *int64) {
	var localAdded int64
	var localRemoved int64

	for y := startY; y < endY; y++ {
		baselineRowStart := baseline.PixOffset(minX, y)
		targetRowStart := target.PixOffset(minX, y)
		diffRowStart := diff.PixOffset(minX, y)

		for x := 0; x < (maxX - minX); x++ {
			baselineOffset := baselineRowStart + x*4
			targetOffset := targetRowStart + x*4
			diffOffset := diffRowStart + x*4

			if baselineOffset >= 0 && baselineOffset+3 < len(baseline.Pix) &&
				targetOffset >= 0 && targetOffset+3 < len(target.Pix) {
				br := baseline.Pix[baselineOffset]
				bg := baseline.Pix[baselineOffset+1]
				bb := baseline.Pix[baselineOffset+2]
				ba := baseline.Pix[baselineOffset+3]

				tr := target.Pix[targetOffset]
				tg := target.Pix[targetOffset+1]
				tb := target.Pix[targetOffset+2]
				ta := target.Pix[targetOffset+3]

				if br == tr && bg == tg && bb == tb && ba == ta {
					diff.Pix[diffOffset] = br
					diff.Pix[diffOffset+1] = bg
					diff.Pix[diffOffset+2] = bb
					diff.Pix[diffOffset+3] = ba
				} else {
					dr, dg, db, da := p.getDiffColor(br, bg, bb, ba, tr, tg, tb, ta)

					diff.Pix[diffOffset] = dr
					diff.Pix[diffOffset+1] = dg
					diff.Pix[diffOffset+2] = db
					diff.Pix[diffOffset+3] = da

					if dr == 255 && dg == 0 && db == 0 {
						localAdded++
					} else if dr == 0 && dg == 0 && db == 255 {
						localRemoved++
					}
				}
			} else {
				diff.Pix[diffOffset] = 255
				diff.Pix[diffOffset+1] = 255
				diff.Pix[diffOffset+2] = 255
				diff.Pix[diffOffset+3] = 255
			}
		}
	}

	atomic.AddInt64(addedCount, localAdded)
	atomic.AddInt64(removedCount, localRemoved)
}

func (p *PixelDiff) processRGBA64(baseline *image.RGBA64, target *image.RGBA64, diff *image.RGBA, minX int, maxX int, startY int, endY int, addedCount *int64, removedCount *int64) {
	var localAdded int64
	var localRemoved int64

	for y := startY; y < endY; y++ {
		baselineRowStart := baseline.PixOffset(minX, y)
		targetRowStart := target.PixOffset(minX, y)
		diffRowStart := diff.PixOffset(minX, y)

		for x := 0; x < (maxX - minX); x++ {
			baselineOffset := baselineRowStart + x*8
			targetOffset := targetRowStart + x*8
			diffOffset := diffRowStart + x*4

			if baselineOffset >= 0 && baselineOffset+7 < len(baseline.Pix) &&
				targetOffset >= 0 && targetOffset+7 < len(target.Pix) {
				br := baseline.Pix[baselineOffset+1]
				bg := baseline.Pix[baselineOffset+3]
				bb := baseline.Pix[baselineOffset+5]
				ba := baseline.Pix[baselineOffset+7]

				tr := target.Pix[targetOffset+1]
				tg := target.Pix[targetOffset+3]
				tb := target.Pix[targetOffset+5]
				ta := target.Pix[targetOffset+7]

				if br == tr && bg == tg && bb == tb && ba == ta {
					diff.Pix[diffOffset] = br
					diff.Pix[diffOffset+1] = bg
					diff.Pix[diffOffset+2] = bb
					diff.Pix[diffOffset+3] = ba
				} else {
					dr, dg, db, da := p.getDiffColor(br, bg, bb, ba, tr, tg, tb, ta)

					diff.Pix[diffOffset] = dr
					diff.Pix[diffOffset+1] = dg
					diff.Pix[diffOffset+2] = db
					diff.Pix[diffOffset+3] = da

					if dr == 255 && dg == 0 && db == 0 {
						localAdded++
					} else if dr == 0 && dg == 0 && db == 255 {
						localRemoved++
					}
				}
			} else {
				diff.Pix[diffOffset] = 255
				diff.Pix[diffOffset+1] = 255
				diff.Pix[diffOffset+2] = 255
				diff.Pix[diffOffset+3] = 255
			}
		}
	}

	atomic.AddInt64(addedCount, localAdded)
	atomic.AddInt64(removedCount, localRemoved)
}

func (p *PixelDiff) processNRGBA64(baseline *image.NRGBA64, target *image.NRGBA64, diff *image.RGBA, minX int, maxX int, startY int, endY int, addedCount *int64, removedCount *int64) {
	var localAdded int64
	var localRemoved int64

	for y := startY; y < endY; y++ {
		baselineRowStart := baseline.PixOffset(minX, y)
		targetRowStart := target.PixOffset(minX, y)
		diffRowStart := diff.PixOffset(minX, y)

		for x := 0; x < (maxX - minX); x++ {
			baselineOffset := baselineRowStart + x*8
			targetOffset := targetRowStart + x*8
			diffOffset := diffRowStart + x*4

			if baselineOffset >= 0 && baselineOffset+7 < len(baseline.Pix) &&
				targetOffset >= 0 && targetOffset+7 < len(target.Pix) {
				br := baseline.Pix[baselineOffset+1]
				bg := baseline.Pix[baselineOffset+3]
				bb := baseline.Pix[baselineOffset+5]
				ba := baseline.Pix[baselineOffset+7]

				tr := target.Pix[targetOffset+1]
				tg := target.Pix[targetOffset+3]
				tb := target.Pix[targetOffset+5]
				ta := target.Pix[targetOffset+7]

				if br == tr && bg == tg && bb == tb && ba == ta {
					diff.Pix[diffOffset] = br
					diff.Pix[diffOffset+1] = bg
					diff.Pix[diffOffset+2] = bb
					diff.Pix[diffOffset+3] = ba
				} else {
					dr, dg, db, da := p.getDiffColor(br, bg, bb, ba, tr, tg, tb, ta)

					diff.Pix[diffOffset] = dr
					diff.Pix[diffOffset+1] = dg
					diff.Pix[diffOffset+2] = db
					diff.Pix[diffOffset+3] = da

					if dr == 255 && dg == 0 && db == 0 {
						localAdded++
					} else if dr == 0 && dg == 0 && db == 255 {
						localRemoved++
					}
				}
			} else {
				diff.Pix[diffOffset] = 255
				diff.Pix[diffOffset+1] = 255
				diff.Pix[diffOffset+2] = 255
				diff.Pix[diffOffset+3] = 255
			}
		}
	}

	atomic.AddInt64(addedCount, localAdded)
	atomic.AddInt64(removedCount, localRemoved)
}

func (p *PixelDiff) processGeneric(baseline image.Image, target image.Image, diff *image.RGBA, minX int, maxX int, startY int, endY int, addedCount *int64, removedCount *int64) {
	var localAdded int64
	var localRemoved int64

	for y := startY; y < endY; y++ {
		for x := minX; x < maxX; x++ {
			baselineColor := p.getColorAt(baseline, x, y)
			targetColor := p.getColorAt(target, x, y)

			if colorsEqual(baselineColor, targetColor) {
				diff.Set(x, y, baselineColor)
			} else {
				dr, dg, db, da := p.getDiffColor(baselineColor.R, baselineColor.G, baselineColor.B, baselineColor.A, targetColor.R, targetColor.G, targetColor.B, targetColor.A)
				diffColor := color.RGBA{R: dr, G: dg, B: db, A: da}
				diff.Set(x, y, diffColor)

				if dr == 255 && dg == 0 && db == 0 {
					localAdded++
				} else if dr == 0 && dg == 0 && db == 255 {
					localRemoved++
				}
			}
		}
	}

	atomic.AddInt64(addedCount, localAdded)
	atomic.AddInt64(removedCount, localRemoved)
}

func (p *PixelDiff) calculateUnionBounds(baseline image.Image, target image.Image) image.Rectangle {
	baselineBounds := baseline.Bounds()
	targetBounds := target.Bounds()

	minX := baselineBounds.Min.X
	if targetBounds.Min.X < minX {
		minX = targetBounds.Min.X
	}

	minY := baselineBounds.Min.Y
	if targetBounds.Min.Y < minY {
		minY = targetBounds.Min.Y
	}

	maxX := baselineBounds.Max.X
	if targetBounds.Max.X > maxX {
		maxX = targetBounds.Max.X
	}

	maxY := baselineBounds.Max.Y
	if targetBounds.Max.Y > maxY {
		maxY = targetBounds.Max.Y
	}

	return image.Rect(minX, minY, maxX, maxY)
}

func (p *PixelDiff) getColorAt(img image.Image, x int, y int) color.RGBA {
	bounds := img.Bounds()
	if x < bounds.Min.X || x >= bounds.Max.X || y < bounds.Min.Y || y >= bounds.Max.Y {
		return color.RGBA{R: 255, G: 255, B: 255, A: 255}
	}
	return color.RGBAModel.Convert(img.At(x, y)).(color.RGBA)
}

func (p *PixelDiff) processYCbCr(baseline *image.YCbCr, target *image.YCbCr, diff *image.RGBA, minX int, maxX int, startY int, endY int, addedCount *int64, removedCount *int64) {
	var localAdded int64
	var localRemoved int64

	for y := startY; y < endY; y++ {
		for x := minX; x < maxX; x++ {
			baselineOffset := baseline.YOffset(x, y)
			targetOffset := target.YOffset(x, y)

			if baselineOffset >= 0 && baselineOffset < len(baseline.Y) &&
				targetOffset >= 0 && targetOffset < len(target.Y) {

				by := baseline.Y[baselineOffset]
				ty := target.Y[targetOffset]

				baseCbOffset := baseline.COffset(x, y)
				tarCbOffset := target.COffset(x, y)

				bcb := baseline.Cb[baseCbOffset]
				bcr := baseline.Cr[baseCbOffset]
				tcb := target.Cb[tarCbOffset]
				tcr := target.Cr[tarCbOffset]

				diffOffset := diff.PixOffset(x, y)

				if by == ty && bcb == tcb && bcr == tcr {
					r, g, b, a := p.ycbcrToRGBA(by, bcb, bcr)
					diff.Pix[diffOffset] = r
					diff.Pix[diffOffset+1] = g
					diff.Pix[diffOffset+2] = b
					diff.Pix[diffOffset+3] = a
				} else {
					br, bg, bb, ba := p.ycbcrToRGBA(by, bcb, bcr)
					tr, tg, tb, ta := p.ycbcrToRGBA(ty, tcb, tcr)

					if br == tr && bg == tg && bb == tb && ba == ta {
						diff.Pix[diffOffset] = br
						diff.Pix[diffOffset+1] = bg
						diff.Pix[diffOffset+2] = bb
						diff.Pix[diffOffset+3] = ba
					} else {
						dr, dg, db, da := p.getDiffColor(br, bg, bb, ba, tr, tg, tb, ta)
						diff.Pix[diffOffset] = dr
						diff.Pix[diffOffset+1] = dg
						diff.Pix[diffOffset+2] = db
						diff.Pix[diffOffset+3] = da

						if dr == 255 && dg == 0 && db == 0 {
							localAdded++
						} else if dr == 0 && dg == 0 && db == 255 {
							localRemoved++
						}
					}
				}
			} else {
				diffOffset := diff.PixOffset(x, y)
				diff.Pix[diffOffset] = 255
				diff.Pix[diffOffset+1] = 255
				diff.Pix[diffOffset+2] = 255
				diff.Pix[diffOffset+3] = 255
			}
		}
	}

	atomic.AddInt64(addedCount, localAdded)
	atomic.AddInt64(removedCount, localRemoved)
}

func (p *PixelDiff) ycbcrToRGBA(y uint8, cb uint8, cr uint8) (uint8, uint8, uint8, uint8) {
	// YCbCr to RGB conversion based on ITU-R BT.601 (JPEG standard)
	// Reference: https://www.w3.org/Graphics/JPEG/jfif3.pdf (Section 7)
	// Reference: https://en.wikipedia.org/wiki/YCbCr#JPEG_conversion
	//
	// Conversion formula:
	// R = Y + 1.402 * (Cr - 128)
	// G = Y - 0.344136 * (Cb - 128) - 0.714136 * (Cr - 128)
	// B = Y + 1.772 * (Cb - 128)
	//
	// The coefficients come from the BT.601 standard:
	// - 1.402 is derived from 2 * (1 - Kr) where Kr = 0.299
	// - 0.344136 and 0.714136 are derived from the color matrix transformation
	// - 1.772 is derived from 2 * (1 - Kb) where Kb = 0.114

	// Convert to fixed-point arithmetic for performance
	// Multiply coefficients by 65536 (2^16) and use bit shift instead of division
	const (
		// 1.402 * 65536 = 91881
		crToR = 91881
		// 0.344136 * 65536 = 22554
		cbToG = 22554
		// 0.714136 * 65536 = 46802
		crToG = 46802
		// 1.772 * 65536 = 116130
		cbToB = 116130
	)

	// YCbCr in JPEG uses the full range [0, 255] for all components
	// See JFIF specification: https://www.w3.org/Graphics/JPEG/jfif3.pdf
	yy := int32(y) * 0x10101 // Equivalent to y * 65536, but optimized
	cb1 := int32(cb) - 128   // Center Cb around 0 (JPEG stores Cb/Cr centered at 128)
	cr1 := int32(cr) - 128   // Center Cr around 0 (JPEG stores Cb/Cr centered at 128)

	// Apply the conversion formula with fixed-point arithmetic
	r := (yy + crToR*cr1) >> 16
	g := (yy - cbToG*cb1 - crToG*cr1) >> 16
	b := (yy + cbToB*cb1) >> 16

	// Clamp values to [0, 255]
	if r < 0 {
		r = 0
	} else if r > 255 {
		r = 255
	}
	if g < 0 {
		g = 0
	} else if g > 255 {
		g = 255
	}
	if b < 0 {
		b = 0
	} else if b > 255 {
		b = 255
	}

	return uint8(r), uint8(g), uint8(b), 255
}

func colorsEqual(c1 color.RGBA, c2 color.RGBA) bool {
	return c1.R == c2.R && c1.G == c2.G && c1.B == c2.B && c1.A == c2.A
}

func (p *PixelDiff) getDiffColor(br uint8, bg uint8, bb uint8, ba uint8, tr uint8, tg uint8, tb uint8, ta uint8) (uint8, uint8, uint8, uint8) {
	const (
		redColor  = 255
		blueColor = 255
	)

	baselineBrightness := int(br) + int(bg) + int(bb)
	targetBrightness := int(tr) + int(tg) + int(tb)
	normalizedDiff := float64(targetBrightness-baselineBrightness) / (255.0 * 3.0)

	if normalizedDiff > p.threshold {
		return redColor, 0, 0, 255
	} else if normalizedDiff < -p.threshold {
		return 0, 0, blueColor, 255
	} else {
		return br, bg, bb, ba
	}
}
