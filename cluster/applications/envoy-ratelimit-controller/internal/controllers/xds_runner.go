package controllers

import (
	"context"
	"fmt"

	v1 "envoy-ratelimit-controller/api/v1"
	"envoy-ratelimit-controller/internal/ratelimit"
	"envoy-ratelimit-controller/internal/xds"

	"golang.org/x/xerrors"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type XDSRunner struct {
	Cache              cache.Cache
	Server             *xds.Server
	IstioRootNamespace string
}

func (x *XDSRunner) NeedLeaderElection() bool {
	return true
}

func (x *XDSRunner) Start(ctx context.Context) error {
	if !x.Cache.WaitForCacheSync(ctx) {
		return xerrors.New("failed to sync cache")
	}

	var rateLimits v1.RateLimitList
	if err := x.Cache.List(ctx, &rateLimits); err != nil {
		return xerrors.Errorf("failed to list RateLimits: %w", err)
	}

	for i := range rateLimits.Items {
		rateLimit := &rateLimits.Items[i]
		if rateLimit.Namespace == x.IstioRootNamespace {
			continue
		}
		domain := fmt.Sprintf("%s.%s", rateLimit.Namespace, rateLimit.Name)
		// No EnvoyFilter dry-run here, since a transient webhook failure would otherwise drop a valid config from the first snapshot
		config, err := ratelimit.GenerateConfig(domain, rateLimit.Spec.Descriptors)
		if err != nil {
			continue
		}
		if err := x.Server.Set(ctx, client.ObjectKeyFromObject(rateLimit), config); err != nil {
			return err
		}
	}

	if err := x.Server.Initialize(ctx); err != nil {
		return err
	}

	return x.Server.Start(ctx)
}
