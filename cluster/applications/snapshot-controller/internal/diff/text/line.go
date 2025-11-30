package text

import (
	"bytes"
	"fmt"
)

type LineDiff struct{}

func NewLineDiff() *LineDiff {
	return &LineDiff{}
}

func (h *LineDiff) Calculate(baseline []byte, target []byte) (*DiffResult, error) {
	beforeLines := h.splitLines(baseline)
	afterLines := h.splitLines(target)

	lcs := h.calculateLCS(beforeLines, afterLines)
	diff, addedCount, removedCount := h.generateDiff(beforeLines, afterLines, lcs)

	totalLines := len(beforeLines) + len(afterLines)

	diffAmount := 0.0
	if totalLines > 0 {
		diffAmount = float64(addedCount+removedCount) / float64(totalLines)
		if diffAmount > 1.0 {
			diffAmount = 1.0
		}
	}

	return &DiffResult{
		Diff:       diff,
		DiffAmount: diffAmount,
	}, nil
}

func (h *LineDiff) splitLines(data []byte) [][]byte {
	if len(data) == 0 {
		return [][]byte{}
	}
	return bytes.Split(data, []byte("\n"))
}

func (h *LineDiff) calculateLCS(before, after [][]byte) [][]int {
	m, n := len(before), len(after)
	lcs := make([][]int, m+1)
	for i := range lcs {
		lcs[i] = make([]int, n+1)
	}

	for i := 1; i <= m; i++ {
		for j := 1; j <= n; j++ {
			if bytes.Equal(before[i-1], after[j-1]) {
				lcs[i][j] = lcs[i-1][j-1] + 1
			} else {
				lcs[i][j] = max(lcs[i-1][j], lcs[i][j-1])
			}
		}
	}

	return lcs
}

func (h *LineDiff) generateDiff(before, after [][]byte, lcs [][]int) ([]byte, int, int) {
	var result bytes.Buffer
	i, j := len(before), len(after)

	var lines [][]byte
	addedCount := 0
	removedCount := 0

	for i > 0 || j > 0 {
		if i > 0 && j > 0 && bytes.Equal(before[i-1], after[j-1]) {
			lines = append([][]byte{[]byte(fmt.Sprintf("  %s", before[i-1]))}, lines...)
			i--
			j--
		} else if j > 0 && (i == 0 || lcs[i][j-1] >= lcs[i-1][j]) {
			lines = append([][]byte{[]byte(fmt.Sprintf("+ %s", after[j-1]))}, lines...)
			j--
			addedCount++
		} else if i > 0 && (j == 0 || lcs[i][j-1] < lcs[i-1][j]) {
			lines = append([][]byte{[]byte(fmt.Sprintf("- %s", before[i-1]))}, lines...)
			i--
			removedCount++
		}
	}

	for i, line := range lines {
		if i > 0 {
			result.WriteByte('\n')
		}
		result.Write(line)
	}

	return result.Bytes(), addedCount, removedCount
}
