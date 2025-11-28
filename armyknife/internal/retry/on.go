package retry

import (
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"

	"golang.org/x/xerrors"
)

type On struct {
	_5xx           bool
	gatewayError   bool
	connectFailure bool
	retriable4xx   bool
	statusCodes    []int
}

func NewDefaultRetryOn() *On {
	return &On{
		_5xx:           false,
		gatewayError:   true,
		connectFailure: true,
		retriable4xx:   true,
		statusCodes:    []int{},
	}
}

func NewRetryOnFromString(s string) (*On, error) {
	o := &On{}
	for _, s := range strings.Split(s, ",") {
		switch s {
		case "5xx":
			o._5xx = true
		case "gateway-error":
			o.gatewayError = true
		case "connect-failure":
			o.connectFailure = true
		case "retriable-4xx":
			o.retriable4xx = true
		default:
			statusCode, err := strconv.Atoi(s)
			if err != nil {
				return nil, xerrors.Errorf("invalid retryOn: %s", s)
			}
			o.statusCodes = append(o.statusCodes, statusCode)
		}
	}
	return o, nil
}

// copy from https://github.com/envoyproxy/envoy/blob/70d6ec1df6384118cf2fa2f02c0041edb76b2377/source/common/router/retry_state_impl.cc#L387
func (o *On) CheckResponse(response *http.Response) bool {
	if (o._5xx && response.StatusCode >= 500 && response.StatusCode < 600) ||
		(o.gatewayError && response.StatusCode >= 502 && response.StatusCode < 505) ||
		(o.retriable4xx && response.StatusCode == 409) {
		return true
	}

	for _, i := range o.statusCodes {
		if i == response.StatusCode {
			return true
		}
	}

	return false
}

func (o *On) CheckError(err error) bool {
	type temporary interface{ Temporary() bool }
	var terr temporary
	if (errors.As(err, &terr) && terr.Temporary()) || errors.Is(err, io.EOF) {
		// ref https://www.envoyproxy.io/docs/envoy/latest/configuration/http/http_filters/router_filter#:~:text=Envoy%20will%20attempt%20a%20retry%20if%20the%20upstream%20server%20responds%20with%20any%205xx%20response%20code%2C%20or%20does%20not%20respond%20at%20all%20(disconnect/reset/read%20timeout).%20(Includes%20connect%2Dfailure%20and%20refused%2Dstream)
		if o.connectFailure || o._5xx {
			return true
		}
	}
	return false
}
