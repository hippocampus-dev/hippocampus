package ratelimit

import (
	"encoding/json"

	v1 "envoy-ratelimit-controller/api/v1"

	"golang.org/x/xerrors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

const (
	rateLimitCluster   = "outbound|8081||envoy-ratelimit.envoy-ratelimit.svc.cluster.local"
	rateLimitAuthority = "envoy-ratelimit.envoy-ratelimit.svc.cluster.local"
)

func GenerateEnvoyFilter(rateLimit *v1.RateLimit, domain string) (*unstructured.Unstructured, error) {
	typedConfig := map[string]interface{}{}
	if rateLimit.Spec.Filter != nil {
		if err := json.Unmarshal(rateLimit.Spec.Filter.Raw, &typedConfig); err != nil {
			return nil, xerrors.Errorf("failed to decode filter: %w", err)
		}
	}
	// The filter-level rate_limits overrides the virtual host rate_limits: https://www.envoyproxy.io/docs/envoy/latest/api-v3/extensions/filters/http/ratelimit/v3/rate_limit.proto
	for _, key := range []string{"rate_limits", "rateLimits"} {
		if _, ok := typedConfig[key]; ok {
			return nil, xerrors.Errorf("filter.%s is not supported, use rateLimits instead", key)
		}
	}
	if _, ok := typedConfig["timeout"]; !ok {
		typedConfig["timeout"] = "1s"
	}
	typedConfig["@type"] = "type.googleapis.com/envoy.extensions.filters.http.ratelimit.v3.RateLimit"
	typedConfig["domain"] = domain
	typedConfig["rate_limit_service"] = map[string]interface{}{
		"grpc_service": map[string]interface{}{
			"envoy_grpc": map[string]interface{}{
				"cluster_name": rateLimitCluster,
				"authority":    rateLimitAuthority,
			},
		},
		"transport_api_version": "V3",
	}

	httpFilterValue := map[string]interface{}{
		"name":         "envoy.filters.http.ratelimit",
		"typed_config": typedConfig,
	}

	rateLimits := make([]interface{}, 0, len(rateLimit.Spec.RateLimits))
	for _, raw := range rateLimit.Spec.RateLimits {
		var r interface{}
		if err := json.Unmarshal(raw.Raw, &r); err != nil {
			return nil, xerrors.Errorf("failed to decode rateLimits: %w", err)
		}
		rateLimits = append(rateLimits, r)
	}

	configPatches := []interface{}{
		map[string]interface{}{
			"applyTo": "HTTP_FILTER",
			"match": map[string]interface{}{
				"context": "SIDECAR_INBOUND",
				"listener": map[string]interface{}{
					"filterChain": map[string]interface{}{
						"filter": map[string]interface{}{
							"name": "envoy.filters.network.http_connection_manager",
							"subFilter": map[string]interface{}{
								"name": "envoy.filters.http.router",
							},
						},
					},
				},
			},
			"patch": map[string]interface{}{
				"operation": "INSERT_BEFORE",
				"value":     httpFilterValue,
			},
		},
		map[string]interface{}{
			"applyTo": "VIRTUAL_HOST",
			"match": map[string]interface{}{
				"context": "SIDECAR_INBOUND",
				"routeConfiguration": map[string]interface{}{
					"vhost": map[string]interface{}{
						"route": map[string]interface{}{
							"action": "ANY",
						},
					},
				},
			},
			"patch": map[string]interface{}{
				"operation": "MERGE",
				"value": map[string]interface{}{
					"rate_limits": rateLimits,
				},
			},
		},
	}

	envoyFilter := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "networking.istio.io/v1alpha3",
			"kind":       "EnvoyFilter",
			"metadata": map[string]interface{}{
				"name":      rateLimit.Name + "-ratelimit",
				"namespace": rateLimit.Namespace,
			},
			"spec": map[string]interface{}{
				"workloadSelector": map[string]interface{}{
					"labels": toStringInterfaceMap(rateLimit.Spec.WorkloadSelector),
				},
				"configPatches": configPatches,
			},
		},
	}

	return envoyFilter, nil
}

func toStringInterfaceMap(m map[string]string) map[string]interface{} {
	result := make(map[string]interface{}, len(m))
	for k, v := range m {
		result[k] = v
	}
	return result
}
