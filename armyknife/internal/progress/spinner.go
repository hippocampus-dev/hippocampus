package progress

import (
	"fmt"
	"os"
	"sync"
)

type Spinner struct {
	chars []rune
	index int
	mutex sync.Mutex
}

func NewSpinner() *Spinner {
	return &Spinner{chars: []rune{'-', '\\', '|', '/'}}
}

func (s *Spinner) Spin() {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	_, _ = fmt.Fprintf(os.Stderr, "\r%c", s.chars[s.index])
	s.index = (s.index + 1) % len(s.chars)
}
