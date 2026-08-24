package retry

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"strconv"
	"time"

	"golang.org/x/xerrors"
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
	if request.Body != nil && request.Body != http.NoBody && request.GetBody == nil {
		body, err := io.ReadAll(request.Body)
		if err != nil {
			return nil, xerrors.Errorf("retry: failed to buffer request body: %w", err)
		}
		_ = request.Body.Close()
		request.Body = io.NopCloser(bytes.NewBuffer(body))
		request.GetBody = func() (io.ReadCloser, error) {
			return io.NopCloser(bytes.NewBuffer(body)), nil
		}
	}

	retryCount := getRetryCount(request.Context())
	sleep, exceeded := t.retryStrategy().Sleep(retryCount)

	response, err := t.base().RoundTrip(request)
	if err != nil {
		if !exceeded && t.RetryOn != nil && t.RetryOn.CheckError(err) {
			return t.retry(request, sleep, retryCount)
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
		_, _ = io.Copy(io.Discard, response.Body)
		_ = response.Body.Close()
		return t.retry(request, sleep, retryCount)
	}
	return response, nil
}

func (t *Transport) retry(request *http.Request, sleep time.Duration, retryCount uint) (*http.Response, error) {
	timer := time.NewTimer(sleep)
	select {
	case <-request.Context().Done():
		timer.Stop()
		return nil, request.Context().Err()
	case <-timer.C:
	}
	if request.GetBody != nil {
		body, err := request.GetBody()
		if err != nil {
			return nil, xerrors.Errorf("retry: failed to rewind request body: %w", err)
		}
		request.Body = body
	}
	return t.RoundTrip(request.WithContext(setRetryCount(request.Context(), retryCount+1)))
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
