package controllers

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/go-logr/logr"
	coreV1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
)

func apiGroup() string {
	defaultGroup := "nodeport-controller.kaidotio.github.io"
	if v, ok := os.LookupEnv("VARIANT"); ok {
		return fmt.Sprintf("%s.%s", v, defaultGroup)
	}
	return defaultGroup
}

type NodePortController struct {
	client.Client
	Log      logr.Logger
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

func (r *NodePortController) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	service := &v1.Service{}
	if err := r.Get(ctx, req.NamespacedName, service); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	if service.Spec.Type != v1.ServiceTypeNodePort {
		return ctrl.Result{}, nil
	}

	var nodeList v1.NodeList
	if err := r.List(ctx, &nodeList); err != nil {
		return ctrl.Result{}, err
	}

	var nodes []string
	for _, node := range nodeList.Items {
		for _, address := range node.Status.Addresses {
			if address.Type != v1.NodeInternalIP {
				continue
			}

			nodes = append(nodes, address.Address)
		}
	}

	nodesAnnotation := strings.Join(nodes, ",")
	if service.Annotations == nil {
		service.Annotations = make(map[string]string)
	}

	if service.Annotations[fmt.Sprintf("%s/nodes", apiGroup())] != nodesAnnotation {
		service.Annotations[fmt.Sprintf("%s/nodes", apiGroup())] = nodesAnnotation
		if err := r.Update(ctx, service); err != nil {
			return ctrl.Result{}, err
		}
		r.Recorder.Eventf(service, coreV1.EventTypeNormal, "SuccessfulUpdated", "Updated nodes annotation: %q", nodes)
	}

	return ctrl.Result{}, nil
}

func (r *NodePortController) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1.Service{}).
		WithOptions(controller.Options{MaxConcurrentReconciles: 1}).
		Complete(r)
}
