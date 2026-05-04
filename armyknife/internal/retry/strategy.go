package retry

import (
	"errors"
	"math"
	"math/rand"
	"time"

	"golang.org/x/exp/constraints"
)

type Strategy interface {
	Sleep(uint) (time.Duration, bool)
}

type never struct{}

func NewNever() *never {
	return &never{}
}

func (nr *never) Sleep(n uint) (time.Duration, bool) {
	return 0, true
}

type Entropy func(int64) int64

type exponentialBackOff struct {
	base          time.Duration
	max           time.Duration
	maxRetryCount uint
	entropy       Entropy
}

func NewExponentialBackOff(base time.Duration, max time.Duration, maxRetryCount uint, entropy Entropy) *exponentialBackOff {
	return &exponentialBackOff{
		base:          base,
		max:           max,
		maxRetryCount: maxRetryCount,
		entropy:       entropy,
	}
}

func (eb *exponentialBackOff) Sleep(retryCount uint) (time.Duration, bool) {
	entropy := eb.getEntropy()
	if retryCount >= eb.maxRetryCount {
		return 0, true
	}

	if retryCount >= 63 {
		return time.Duration(entropy(min(math.MaxInt64, int64(eb.max)))), false
	}

	delay, err := checkedMulInt64(1<<retryCount, int64(eb.base))
	if err != nil {
		return time.Duration(entropy(min(math.MaxInt64, int64(eb.max)))), false
	}
	return time.Duration(entropy(min(delay, int64(eb.max)))), false
}

func (eb *exponentialBackOff) getEntropy() Entropy {
	if eb.entropy == nil {
		return rand.Int63n
	}
	return eb.entropy
}

func min[T constraints.Ordered](l T, r T) T {
	if l > r {
		return r
	}
	return l
}

var OverflowError = errors.New("overflow")

func checkedMulInt64(l int64, r int64) (int64, error) {
	if l == 0 || r == 0 {
		return l * r, nil
	}
	if l > math.MaxInt64/r {
		return 0, OverflowError
	}
	return l * r, nil
}
