package image

import (
	"image"
	"image/color"
	"image/draw"
	"runtime"
	"sync"
)

type Rectangle struct {
	X      int
	Y      int
	Width  int
	Height int
}

type RectangleDiff struct{}

func NewRectangleDiff() *RectangleDiff {
	return &RectangleDiff{}
}

func (r *RectangleDiff) Calculate(baseline image.Image, target image.Image) *DiffResult {
	if baseline == target {
		return &DiffResult{
			Image:      baseline,
			DiffAmount: 0.0,
		}
	}

	rectangles := r.findRectangles(baseline, target)

	bounds := target.Bounds()
	result := image.NewRGBA(bounds)
	draw.Draw(result, bounds, target, bounds.Min, draw.Src)

	rectColor := color.RGBA{R: 255, A: 255} // Red color for rectangles

	for _, rect := range rectangles {
		for thickness := 0; thickness < 3; thickness++ {
			for x := rect.X - thickness; x < rect.X+rect.Width+thickness; x++ {
				if x >= 0 && x < bounds.Max.X {
					if rect.Y-thickness >= 0 {
						result.Set(x, rect.Y-thickness, rectColor)
					}
					if rect.Y+rect.Height+thickness < bounds.Max.Y {
						result.Set(x, rect.Y+rect.Height+thickness, rectColor)
					}
				}
			}

			for y := rect.Y - thickness; y < rect.Y+rect.Height+thickness; y++ {
				if y >= 0 && y < bounds.Max.Y {
					if rect.X-thickness >= 0 {
						result.Set(rect.X-thickness, y, rectColor)
					}
					if rect.X+rect.Width+thickness < bounds.Max.X {
						result.Set(rect.X+rect.Width+thickness, y, rectColor)
					}
				}
			}
		}
	}

	diffAmount := r.calculateDiffAmount(baseline, target, rectangles)

	return &DiffResult{
		Image:      result,
		DiffAmount: diffAmount,
	}
}

func (r *RectangleDiff) findRectangles(baseline image.Image, target image.Image) []Rectangle {
	bounds := baseline.Bounds()
	targetBounds := target.Bounds()

	minX := bounds.Min.X
	if targetBounds.Min.X < minX {
		minX = targetBounds.Min.X
	}
	minY := bounds.Min.Y
	if targetBounds.Min.Y < minY {
		minY = targetBounds.Min.Y
	}
	maxX := bounds.Max.X
	if targetBounds.Max.X > maxX {
		maxX = targetBounds.Max.X
	}
	maxY := bounds.Max.Y
	if targetBounds.Max.Y > maxY {
		maxY = targetBounds.Max.Y
	}

	height := maxY - minY
	width := maxX - minX
	diffMap := make([][]bool, height)
	for i := range diffMap {
		diffMap[i] = make([]bool, width)
	}

	baselineRGBA, baselineIsRGBA := baseline.(*image.RGBA)
	targetRGBA, targetIsRGBA := target.(*image.RGBA)
	baselineYCbCr, baselineIsYCbCr := baseline.(*image.YCbCr)
	targetYCbCr, targetIsYCbCr := target.(*image.YCbCr)

	// Use GOMAXPROCS instead of runtime.NumCPU() to consider cgroup.
	// https://tip.golang.org/doc/go1.25#container-aware-gomaxprocs
	numWorkers := runtime.GOMAXPROCS(0)

	rowsPerWorker := height / numWorkers
	var wg sync.WaitGroup
	wg.Add(numWorkers)

	if baselineIsRGBA && targetIsRGBA {
		for i := 0; i < numWorkers; i++ {
			startY := i * rowsPerWorker
			endY := startY + rowsPerWorker
			if i == numWorkers-1 {
				endY = height
			}

			go func(startY, endY int) {
				defer wg.Done()
				r.processRGBA(baselineRGBA, targetRGBA, diffMap, bounds, targetBounds, minX, maxX, minY, startY+minY, endY+minY)
			}(startY, endY)
		}
	} else if baselineIsYCbCr && targetIsYCbCr {
		for i := 0; i < numWorkers; i++ {
			startY := i * rowsPerWorker
			endY := startY + rowsPerWorker
			if i == numWorkers-1 {
				endY = height
			}

			go func(startY, endY int) {
				defer wg.Done()
				r.processYCbCr(baselineYCbCr, targetYCbCr, diffMap, bounds, targetBounds, minX, maxX, minY, startY+minY, endY+minY)
			}(startY, endY)
		}
	} else {
		for i := 0; i < numWorkers; i++ {
			startY := i * rowsPerWorker
			endY := startY + rowsPerWorker
			if i == numWorkers-1 {
				endY = height
			}

			go func(startY, endY int) {
				defer wg.Done()
				r.processGeneric(baseline, target, diffMap, bounds, targetBounds, minX, maxX, minY, startY+minY, endY+minY)
			}(startY, endY)
		}
	}

	wg.Wait()

	visited := make([][]bool, height)
	for i := range visited {
		visited[i] = make([]bool, width)
	}

	var rectangles []Rectangle
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			if diffMap[y][x] && !visited[y][x] {
				rect := r.findBoundingBox(diffMap, visited, x, y, width, height, minX, minY)
				if rect.Width > 2 && rect.Height > 2 {
					rectangles = append(rectangles, rect)
				}
			}
		}
	}

	return r.mergeRectangles(rectangles)
}

func (r *RectangleDiff) findBoundingBox(diffMap [][]bool, visited [][]bool, startX int, startY int, width int, height int, offsetX int, offsetY int) Rectangle {
	minX := startX
	minY := startY
	maxRectX := startX
	maxRectY := startY

	queue := []struct {
		x int
		y int
	}{{startX, startY}}
	visited[startY][startX] = true

	for len(queue) > 0 {
		point := queue[0]
		queue = queue[1:]

		if point.x < minX {
			minX = point.x
		}
		if point.x > maxRectX {
			maxRectX = point.x
		}
		if point.y < minY {
			minY = point.y
		}
		if point.y > maxRectY {
			maxRectY = point.y
		}

		// Check 8 neighbors
		for dy := -1; dy <= 1; dy++ {
			for dx := -1; dx <= 1; dx++ {
				if dx == 0 && dy == 0 {
					continue
				}

				nx := point.x + dx
				ny := point.y + dy
				if nx >= 0 && nx < width && ny >= 0 && ny < height &&
					diffMap[ny][nx] && !visited[ny][nx] {
					visited[ny][nx] = true
					queue = append(queue, struct {
						x int
						y int
					}{nx, ny})
				}
			}
		}
	}

	return Rectangle{
		X:      minX + offsetX,
		Y:      minY + offsetY,
		Width:  maxRectX - minX + 1,
		Height: maxRectY - minY + 1,
	}
}

func (r *RectangleDiff) mergeRectangles(rects []Rectangle) []Rectangle {
	if len(rects) <= 1 {
		return rects
	}

	merged := make([]Rectangle, 0)
	used := make([]bool, len(rects))

	for i := 0; i < len(rects); i++ {
		if used[i] {
			continue
		}

		current := rects[i]
		mergedAny := true

		for mergedAny {
			mergedAny = false
			for j := i + 1; j < len(rects); j++ {
				if used[j] {
					continue
				}

				if r.rectanglesOverlap(current, rects[j]) || r.rectanglesClose(current, rects[j], 10) {
					current = r.combineRectangles(current, rects[j])
					used[j] = true
					mergedAny = true
				}
			}
		}

		merged = append(merged, current)
	}

	return merged
}

func (r *RectangleDiff) rectanglesOverlap(r1 Rectangle, r2 Rectangle) bool {
	return !(r1.X+r1.Width <= r2.X || r2.X+r2.Width <= r1.X ||
		r1.Y+r1.Height <= r2.Y || r2.Y+r2.Height <= r1.Y)
}

func (r *RectangleDiff) rectanglesClose(r1 Rectangle, r2 Rectangle, threshold int) bool {
	r1Expanded := Rectangle{
		X:      r1.X - threshold,
		Y:      r1.Y - threshold,
		Width:  r1.Width + 2*threshold,
		Height: r1.Height + 2*threshold,
	}

	r2Expanded := Rectangle{
		X:      r2.X - threshold,
		Y:      r2.Y - threshold,
		Width:  r2.Width + 2*threshold,
		Height: r2.Height + 2*threshold,
	}

	return r.rectanglesOverlap(r1Expanded, r2Expanded)
}

func (r *RectangleDiff) combineRectangles(r1 Rectangle, r2 Rectangle) Rectangle {
	minX := r1.X
	if r2.X < minX {
		minX = r2.X
	}

	minY := r1.Y
	if r2.Y < minY {
		minY = r2.Y
	}

	maxX := r1.X + r1.Width
	if r2.X+r2.Width > maxX {
		maxX = r2.X + r2.Width
	}

	maxY := r1.Y + r1.Height
	if r2.Y+r2.Height > maxY {
		maxY = r2.Y + r2.Height
	}

	return Rectangle{
		X:      minX,
		Y:      minY,
		Width:  maxX - minX,
		Height: maxY - minY,
	}
}

func (r *RectangleDiff) calculateDiffAmount(baseline image.Image, target image.Image, rectangles []Rectangle) float64 {
	totalDiffArea := 0
	for _, rect := range rectangles {
		totalDiffArea += rect.Width * rect.Height
	}

	bounds := baseline.Bounds()
	targetBounds := target.Bounds()

	minX := bounds.Min.X
	if targetBounds.Min.X < minX {
		minX = targetBounds.Min.X
	}
	minY := bounds.Min.Y
	if targetBounds.Min.Y < minY {
		minY = targetBounds.Min.Y
	}
	maxX := bounds.Max.X
	if targetBounds.Max.X > maxX {
		maxX = targetBounds.Max.X
	}
	maxY := bounds.Max.Y
	if targetBounds.Max.Y > maxY {
		maxY = targetBounds.Max.Y
	}

	totalArea := (maxX - minX) * (maxY - minY)

	if totalArea == 0 {
		return 0.0
	}

	return float64(totalDiffArea) / float64(totalArea)
}

func (r *RectangleDiff) processRGBA(baseline *image.RGBA, target *image.RGBA, diffMap [][]bool, bounds image.Rectangle, targetBounds image.Rectangle, minX int, maxX int, minY int, startY int, endY int) {
	for y := startY; y < endY; y++ {
		baselineRowStart := -1
		targetRowStart := -1

		if y >= bounds.Min.Y && y < bounds.Max.Y {
			baselineRowStart = baseline.PixOffset(bounds.Min.X, y)
		}
		if y >= targetBounds.Min.Y && y < targetBounds.Max.Y {
			targetRowStart = target.PixOffset(targetBounds.Min.X, y)
		}

		for x := minX; x < maxX; x++ {
			var br uint8 = 255
			var bg uint8 = 255
			var bb uint8 = 255
			var ba uint8 = 255
			var tr uint8 = 255
			var tg uint8 = 255
			var tb uint8 = 255
			var ta uint8 = 255

			if x >= bounds.Min.X && x < bounds.Max.X && baselineRowStart >= 0 {
				offset := baselineRowStart + (x-bounds.Min.X)*4
				if offset >= 0 && offset+3 < len(baseline.Pix) {
					br = baseline.Pix[offset]
					bg = baseline.Pix[offset+1]
					bb = baseline.Pix[offset+2]
					ba = baseline.Pix[offset+3]
				}
			}

			if x >= targetBounds.Min.X && x < targetBounds.Max.X && targetRowStart >= 0 {
				offset := targetRowStart + (x-targetBounds.Min.X)*4
				if offset >= 0 && offset+3 < len(target.Pix) {
					tr = target.Pix[offset]
					tg = target.Pix[offset+1]
					tb = target.Pix[offset+2]
					ta = target.Pix[offset+3]
				}
			}

			if br != tr || bg != tg || bb != tb || ba != ta {
				diffMap[y][x] = true
			}
		}
	}
}

func (r *RectangleDiff) processYCbCr(baseline *image.YCbCr, target *image.YCbCr, diffMap [][]bool, bounds image.Rectangle, targetBounds image.Rectangle, minX int, maxX int, minY int, startY int, endY int) {
	for y := startY; y < endY; y++ {
		for x := minX; x < maxX; x++ {
			var by uint8
			var bcb uint8
			var bcr uint8
			var ty uint8
			var tcb uint8
			var tcr uint8
			baselineInBounds := false
			targetInBounds := false

			if x < bounds.Max.X && y < bounds.Max.Y && x >= bounds.Min.X && y >= bounds.Min.Y {
				yOffset := baseline.YOffset(x, y)
				if yOffset >= 0 && yOffset < len(baseline.Y) {
					by = baseline.Y[yOffset]
					cOffset := baseline.COffset(x, y)
					if cOffset >= 0 && cOffset < len(baseline.Cb) {
						bcb = baseline.Cb[cOffset]
						bcr = baseline.Cr[cOffset]
						baselineInBounds = true
					}
				}
			}

			if x < targetBounds.Max.X && y < targetBounds.Max.Y && x >= targetBounds.Min.X && y >= targetBounds.Min.Y {
				yOffset := target.YOffset(x, y)
				if yOffset >= 0 && yOffset < len(target.Y) {
					ty = target.Y[yOffset]
					cOffset := target.COffset(x, y)
					if cOffset >= 0 && cOffset < len(target.Cb) {
						tcb = target.Cb[cOffset]
						tcr = target.Cr[cOffset]
						targetInBounds = true
					}
				}
			}

			if baselineInBounds && targetInBounds {
				if by != ty || bcb != tcb || bcr != tcr {
					diffMap[y][x] = true
				}
			} else if baselineInBounds != targetInBounds {
				diffMap[y][x] = true
			}
		}
	}
}

func (r *RectangleDiff) processGeneric(baseline image.Image, target image.Image, diffMap [][]bool, bounds image.Rectangle, targetBounds image.Rectangle, minX int, maxX int, minY int, startY int, endY int) {
	for y := startY; y < endY; y++ {
		for x := minX; x < maxX; x++ {
			baselineColor := color.RGBA{R: 255, G: 255, B: 255, A: 255}
			targetColor := color.RGBA{R: 255, G: 255, B: 255, A: 255}

			if x < bounds.Max.X && y < bounds.Max.Y {
				baselineColor = color.RGBAModel.Convert(baseline.At(x, y)).(color.RGBA)
			}

			if x < targetBounds.Max.X && y < targetBounds.Max.Y {
				targetColor = color.RGBAModel.Convert(target.At(x, y)).(color.RGBA)
			}

			if !colorsEqual(baselineColor, targetColor) {
				diffMap[y][x] = true
			}
		}
	}
}

func (r *RectangleDiff) ycbcrToRGBA(y uint8, cb uint8, cr uint8) (uint8, uint8, uint8, uint8) {
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
	rValue := (yy + crToR*cr1) >> 16
	gValue := (yy - cbToG*cb1 - crToG*cr1) >> 16
	bValue := (yy + cbToB*cb1) >> 16

	// Clamp values to [0, 255]
	if rValue < 0 {
		rValue = 0
	} else if rValue > 255 {
		rValue = 255
	}
	if gValue < 0 {
		gValue = 0
	} else if gValue > 255 {
		gValue = 255
	}
	if bValue < 0 {
		bValue = 0
	} else if bValue > 255 {
		bValue = 255
	}

	return uint8(rValue), uint8(gValue), uint8(bValue), 255
}
