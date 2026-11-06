package retry

import (
	"context"
	"net/http"
	"strconv"
	"time"
)

type Transport struct {
	Base          http.RoundTripper
	RetryStrategy Strategy
	RetryOn       *On
}

type contextKey string

const retryCountContextKey contextKey = "retryCountKey"

func getRetryCount(ctx context.Context) uint {
	v := ctx.Value(retryCountContextKey)

	i, ok := v.(uint)
	if !ok {
		return 0
	}

	return i
}

func setRetryCount(ctx context.Context, retryCount uint) context.Context {
	return context.WithValue(ctx, retryCountContextKey, retryCount)
}

func (t *Transport) RoundTrip(request *http.Request) (*http.Response, error) {
	retryCount := getRetryCount(request.Context())
	sleep, exceeded := t.retryStrategy().Sleep(retryCount)

	response, err := t.base().RoundTrip(request)
	if err != nil {
		if !exceeded && t.RetryOn != nil && t.RetryOn.CheckError(err) {
			timer := time.NewTimer(sleep)
			select {
			case <-request.Context().Done():
				timer.Stop()
				return nil, request.Context().Err()
			case <-timer.C:
			}
			return t.RoundTrip(request.WithContext(setRetryCount(request.Context(), retryCount+1)))
		}
		return nil, err
	}
	if !exceeded && t.RetryOn != nil && t.RetryOn.CheckResponse(response) {
		if h := response.Header.Get("Retry-After"); h != "" {
			if s, err := strconv.Atoi(h); err == nil {
				if s < 0 {
					s = 0
				}
				sleep = time.Duration(s) * time.Second
			} else if d, err := http.ParseTime(h); err == nil {
				if delta := d.Sub(time.Now()); delta > 0 {
					sleep = delta
				} else {
					sleep = 0
				}
			}
		}
		timer := time.NewTimer(sleep)
		select {
		case <-request.Context().Done():
			timer.Stop()
			return nil, request.Context().Err()
		case <-timer.C:
		}
		return t.RoundTrip(request.WithContext(setRetryCount(request.Context(), retryCount+1)))
	}
	return response, nil
}

func (t *Transport) base() http.RoundTripper {
	if t.Base != nil {
		return t.Base
	}
	return http.DefaultTransport
}

func (t *Transport) retryStrategy() Strategy {
	if t.RetryStrategy != nil {
		return t.RetryStrategy
	}
	return NewNever()
}

func (t *Transport) CancelRequest(request *http.Request) {
	type canceler interface {
		CancelRequest(*http.Request)
	}
	if cr, ok := t.base().(canceler); ok {
		cr.CancelRequest(request)
	}
}
