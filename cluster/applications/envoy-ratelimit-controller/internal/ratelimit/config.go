package ratelimit

import (
	"bytes"
	"encoding/json"
	"strings"

	rlsConfV3 "github.com/envoyproxy/go-control-plane/ratelimit/config/ratelimit/v3"
	"golang.org/x/xerrors"
	"k8s.io/apimachinery/pkg/runtime"
)

type descriptor struct {
	Key            string       `json:"key"`
	Value          string       `json:"value,omitempty"`
	RateLimit      *rateLimit   `json:"rate_limit,omitempty"`
	Descriptors    []descriptor `json:"descriptors,omitempty"`
	ShadowMode     bool         `json:"shadow_mode,omitempty"`
	DetailedMetric bool         `json:"detailed_metric,omitempty"`
}

type rateLimit struct {
	RequestsPerUnit uint32    `json:"requests_per_unit,omitempty"`
	Unit            string    `json:"unit,omitempty"`
	Unlimited       bool      `json:"unlimited,omitempty"`
	Name            string    `json:"name,omitempty"`
	Replaces        []replace `json:"replaces,omitempty"`
}

type replace struct {
	Name string `json:"name"`
}

// Other units panic at request time: https://github.com/envoyproxy/ratelimit/blob/91484c59/src/utils/utilities.go
var units = map[string]rlsConfV3.RateLimitUnit{
	"SECOND": rlsConfV3.RateLimitUnit_SECOND,
	"MINUTE": rlsConfV3.RateLimitUnit_MINUTE,
	"HOUR":   rlsConfV3.RateLimitUnit_HOUR,
	"DAY":    rlsConfV3.RateLimitUnit_DAY,
}

func GenerateConfig(domain string, rawDescriptors []runtime.RawExtension) (*rlsConfV3.RateLimitConfig, error) {
	descriptors := make([]descriptor, 0, len(rawDescriptors))
	for _, raw := range rawDescriptors {
		decoder := json.NewDecoder(bytes.NewReader(raw.Raw))
		// The ratelimit image is pinned to 91484c59 by .github/workflows/99_mirror-ratelimit.yaml: https://github.com/envoyproxy/ratelimit/blob/91484c59/src/config/config_impl.go
		decoder.DisallowUnknownFields()
		var d descriptor
		if err := decoder.Decode(&d); err != nil {
			return nil, xerrors.Errorf("failed to decode descriptor: %w", err)
		}
		descriptors = append(descriptors, d)
	}
	// One invalid config fails the whole update: https://github.com/envoyproxy/ratelimit/blob/91484c59/src/provider/xds_grpc_sotw_provider.go
	if err := validateDescriptors(descriptors); err != nil {
		return nil, err
	}
	return &rlsConfV3.RateLimitConfig{
		Name:        domain,
		Domain:      domain,
		Descriptors: convertDescriptors(descriptors),
	}, nil
}

func validateDescriptors(descriptors []descriptor) error {
	seen := make(map[string]struct{}, len(descriptors))
	for _, d := range descriptors {
		if d.Key == "" {
			return xerrors.New("descriptor has empty key")
		}
		compositeKey := d.Key
		if d.Value != "" {
			compositeKey += "_" + d.Value
		}
		if _, ok := seen[compositeKey]; ok {
			return xerrors.Errorf("duplicate descriptor composite key '%s'", compositeKey)
		}
		seen[compositeKey] = struct{}{}

		if d.RateLimit != nil {
			_, validUnit := units[strings.ToUpper(d.RateLimit.Unit)]
			if d.RateLimit.Unlimited && validUnit {
				return xerrors.Errorf("descriptor '%s' should not specify rate limit unit when unlimited", compositeKey)
			}
			if !d.RateLimit.Unlimited && !validUnit {
				return xerrors.Errorf("descriptor '%s' has invalid rate limit unit '%s'", compositeKey, d.RateLimit.Unit)
			}
			for _, r := range d.RateLimit.Replaces {
				if r.Name == "" {
					return xerrors.Errorf("descriptor '%s' should not have an empty replaces entry", compositeKey)
				}
				if r.Name == d.RateLimit.Name {
					return xerrors.Errorf("descriptor '%s' replaces should not contain name of same descriptor", compositeKey)
				}
			}
		}

		if err := validateDescriptors(d.Descriptors); err != nil {
			return err
		}
	}
	return nil
}

func convertDescriptors(descriptors []descriptor) []*rlsConfV3.RateLimitDescriptor {
	result := make([]*rlsConfV3.RateLimitDescriptor, 0, len(descriptors))
	for _, d := range descriptors {
		converted := &rlsConfV3.RateLimitDescriptor{
			Key:            d.Key,
			Value:          d.Value,
			Descriptors:    convertDescriptors(d.Descriptors),
			ShadowMode:     d.ShadowMode,
			DetailedMetric: d.DetailedMetric,
		}
		if d.RateLimit != nil {
			replaces := make([]*rlsConfV3.RateLimitReplace, 0, len(d.RateLimit.Replaces))
			for _, r := range d.RateLimit.Replaces {
				replaces = append(replaces, &rlsConfV3.RateLimitReplace{Name: r.Name})
			}
			converted.RateLimit = &rlsConfV3.RateLimitPolicy{
				Unit:            units[strings.ToUpper(d.RateLimit.Unit)],
				RequestsPerUnit: d.RateLimit.RequestsPerUnit,
				Unlimited:       d.RateLimit.Unlimited,
				Name:            d.RateLimit.Name,
				Replaces:        replaces,
			}
		}
		result = append(result, converted)
	}
	return result
}
