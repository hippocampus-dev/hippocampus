package progress

import (
	"fmt"
	"os"
	"strings"
	"sync"
)

type Bar struct {
	max     int
	current int
	mutex   sync.Mutex
}

func NewBar(max int) *Bar {
	return &Bar{max: max}
}

func (b *Bar) Increment(n int) {
	b.mutex.Lock()
	defer b.mutex.Unlock()
	b.current += n

	if b.max == 0 {
		return
	}

	const barWidth = 50
	progress := float64(b.current) / float64(b.max)
	blocks := int(progress * float64(barWidth))
	bar := strings.Repeat("=", blocks) + strings.Repeat(" ", barWidth-blocks)

	_, _ = fmt.Fprintf(os.Stderr, "\r[%s] %.2f%% (%d/%d)", bar, progress*100, b.current, b.max)
	if b.current == b.max {
		_, _ = fmt.Fprintf(os.Stderr, "\n")
	}
}
