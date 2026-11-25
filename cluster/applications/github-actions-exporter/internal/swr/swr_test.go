package swr

import (
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
)

func TestGet(t *testing.T) {
	type in struct {
		setup       func(t *testing.T, c *Cache[string])
		key         string
		fetchValue  string
		staleAfter  time.Duration
		expireAfter time.Duration
	}

	tests := []struct {
		name      string
		in        in
		wantValue string
	}{
		{
			"cache miss returns fetched value",
			in{
				setup:       func(t *testing.T, c *Cache[string]) {},
				key:         "key",
				fetchValue:  "value",
				staleAfter:  60 * time.Second,
				expireAfter: 300 * time.Second,
			},
			"value",
		},
		{
			"fresh entry returns cached value",
			in{
				setup: func(t *testing.T, c *Cache[string]) {
					t.Helper()
					_, _ = c.Get("key", func() (FetchResult[string], error) {
						return FetchResult[string]{Value: "cached", StaleAfter: 60 * time.Second, ExpireAfter: 300 * time.Second}, nil
					})
				},
				key:         "key",
				fetchValue:  "new",
				staleAfter:  60 * time.Second,
				expireAfter: 300 * time.Second,
			},
			"cached",
		},
		{
			"expired entry returns fetched value",
			in{
				setup: func(t *testing.T, c *Cache[string]) {
					t.Helper()
					_, _ = c.Get("key", func() (FetchResult[string], error) {
						return FetchResult[string]{Value: "old", StaleAfter: 5 * time.Millisecond, ExpireAfter: 10 * time.Millisecond}, nil
					})
					time.Sleep(20 * time.Millisecond)
				},
				key:         "key",
				fetchValue:  "refreshed",
				staleAfter:  5 * time.Millisecond,
				expireAfter: 10 * time.Millisecond,
			},
			"refreshed",
		},
		{
			"different keys are independent",
			in{
				setup: func(t *testing.T, c *Cache[string]) {
					t.Helper()
					_, _ = c.Get("a", func() (FetchResult[string], error) {
						return FetchResult[string]{Value: "value_a", StaleAfter: 60 * time.Second, ExpireAfter: 300 * time.Second}, nil
					})
				},
				key:         "b",
				fetchValue:  "value_b",
				staleAfter:  60 * time.Second,
				expireAfter: 300 * time.Second,
			},
			"value_b",
		},
	}
	for _, tt := range tests {
		name := tt.name
		in := tt.in
		wantValue := tt.wantValue
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			cache := New[string]()
			in.setup(t, cache)

			got, err := cache.Get(in.key, func() (FetchResult[string], error) {
				return FetchResult[string]{Value: in.fetchValue, StaleAfter: in.staleAfter, ExpireAfter: in.expireAfter}, nil
			})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if diff := cmp.Diff(wantValue, got); diff != "" {
				t.Errorf("(-want +got):\n%s", diff)
			}
		})
	}
}

func TestStaleReturnsOldValueAndRefreshes(t *testing.T) {
	t.Parallel()

	cache := New[string]()

	_, _ = cache.Get("key", func() (FetchResult[string], error) {
		return FetchResult[string]{Value: "old", StaleAfter: 10 * time.Millisecond, ExpireAfter: 300 * time.Second}, nil
	})

	time.Sleep(20 * time.Millisecond)

	got, err := cache.Get("key", func() (FetchResult[string], error) {
		return FetchResult[string]{Value: "new", StaleAfter: 60 * time.Second, ExpireAfter: 300 * time.Second}, nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if diff := cmp.Diff("old", got); diff != "" {
		t.Errorf("(-want +got):\n%s", diff)
	}

	time.Sleep(50 * time.Millisecond)

	got, err = cache.Get("key", func() (FetchResult[string], error) {
		return FetchResult[string]{Value: "should_not_call", StaleAfter: 60 * time.Second, ExpireAfter: 300 * time.Second}, nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if diff := cmp.Diff("new", got); diff != "" {
		t.Errorf("(-want +got):\n%s", diff)
	}
}

func TestConcurrentStaleDeduplication(t *testing.T) {
	t.Parallel()

	cache := New[string]()

	_, _ = cache.Get("key", func() (FetchResult[string], error) {
		return FetchResult[string]{Value: "old", StaleAfter: 10 * time.Millisecond, ExpireAfter: 300 * time.Second}, nil
	})

	time.Sleep(20 * time.Millisecond)

	var callCount atomic.Int64
	var wg sync.WaitGroup

	for range 5 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			got, err := cache.Get("key", func() (FetchResult[string], error) {
				callCount.Add(1)
				time.Sleep(50 * time.Millisecond)
				return FetchResult[string]{Value: "new", StaleAfter: 10 * time.Millisecond, ExpireAfter: 300 * time.Second}, nil
			})
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if diff := cmp.Diff("old", got); diff != "" {
				t.Errorf("(-want +got):\n%s", diff)
			}
		}()
	}

	wg.Wait()
	time.Sleep(100 * time.Millisecond)

	if diff := cmp.Diff(int64(1), callCount.Load()); diff != "" {
		t.Errorf("call count (-want +got):\n%s", diff)
	}
}

func TestExpiredErrorNotCached(t *testing.T) {
	t.Parallel()

	cache := New[string]()

	_, err := cache.Get("key", func() (FetchResult[string], error) {
		return FetchResult[string]{}, errors.New("fetch failed")
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	got, err := cache.Get("key", func() (FetchResult[string], error) {
		return FetchResult[string]{Value: "recovered", StaleAfter: 60 * time.Second, ExpireAfter: 300 * time.Second}, nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if diff := cmp.Diff("recovered", got); diff != "" {
		t.Errorf("(-want +got):\n%s", diff)
	}
}

func TestStaleBackgroundErrorPreservesValue(t *testing.T) {
	t.Parallel()

	cache := New[string]()

	_, _ = cache.Get("key", func() (FetchResult[string], error) {
		return FetchResult[string]{Value: "good", StaleAfter: 10 * time.Millisecond, ExpireAfter: 300 * time.Second}, nil
	})

	time.Sleep(20 * time.Millisecond)

	got, err := cache.Get("key", func() (FetchResult[string], error) {
		return FetchResult[string]{}, errors.New("background failure")
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if diff := cmp.Diff("good", got); diff != "" {
		t.Errorf("(-want +got):\n%s", diff)
	}

	time.Sleep(50 * time.Millisecond)

	got, err = cache.Get("key", func() (FetchResult[string], error) {
		return FetchResult[string]{Value: "should_not_call", StaleAfter: 60 * time.Second, ExpireAfter: 300 * time.Second}, nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if diff := cmp.Diff("good", got); diff != "" {
		t.Errorf("(-want +got):\n%s", diff)
	}
}
