package controllers

import (
	"context"
	"fmt"

	"gateway-proxy/internal/proxy"

	"github.com/go-logr/logr"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	toolscache "k8s.io/client-go/tools/cache"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
	gatewayv1alpha2 "sigs.k8s.io/gateway-api/apis/v1alpha2"
)

type ProxyRunner struct {
	Cache        cache.Cache
	Resolver     *RouteResolver
	ProxyManager *proxy.Manager
	Log          logr.Logger
	trigger      chan struct{}
}

func (p *ProxyRunner) NeedLeaderElection() bool {
	return false
}

func (p *ProxyRunner) Start(ctx context.Context) error {
	p.trigger = make(chan struct{}, 1)

	if !p.Cache.WaitForCacheSync(ctx) {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return fmt.Errorf("failed to sync informer caches")
	}

	for _, object := range []client.Object{
		&gatewayv1.Gateway{},
		&gatewayv1.GatewayClass{},
		&gatewayv1.ListenerSet{},
		&gatewayv1alpha2.TCPRoute{},
		&gatewayv1alpha2.UDPRoute{},
		&gatewayv1.ReferenceGrant{},
	} {
		informer, err := p.Cache.GetInformer(ctx, object)
		if err != nil {
			return fmt.Errorf("failed to get informer for %T: %w", object, err)
		}
		if _, err := informer.AddEventHandler(toolscache.ResourceEventHandlerFuncs{
			AddFunc: func(_ interface{}) { p.enqueue() },
			UpdateFunc: func(oldObject interface{}, newObject interface{}) {
				oldMeta, ok1 := oldObject.(metav1.Object)
				newMeta, ok2 := newObject.(metav1.Object)
				if !ok1 || !ok2 || oldMeta.GetGeneration() != newMeta.GetGeneration() {
					p.enqueue()
				}
			},
			DeleteFunc: func(_ interface{}) { p.enqueue() },
		}); err != nil {
			return fmt.Errorf("failed to add event handler for %T: %w", object, err)
		}
	}

	p.enqueue()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-p.trigger:
			p.reconcile(ctx)
		}
	}
}

func (p *ProxyRunner) enqueue() {
	select {
	case p.trigger <- struct{}{}:
	default:
	}
}

func (p *ProxyRunner) reconcile(ctx context.Context) {
	routes, _, _, err := p.Resolver.resolveAllRoutes(ctx)
	if err != nil {
		p.Log.Error(err, "failed to resolve routes")
		return
	}
	if err := p.ProxyManager.Update(routes); err != nil {
		p.Log.Error(err, "failed to update proxy listeners")
	}
}
