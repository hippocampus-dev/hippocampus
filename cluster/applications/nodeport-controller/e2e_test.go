package main

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/metrics/server"

	"nodeport-controller/internal/controllers"
)

func waitForCondition[T client.Object](t *testing.T, environment *testEnvironment, object T, condition func(T) bool) {
	const maxAttempts = 10
	const waitInterval = 100 * time.Millisecond

	key := client.ObjectKeyFromObject(object)
	for attempt := 0; attempt < maxAttempts; attempt++ {
		if err := environment.kubeClient.Get(environment.ctx, key, object); err != nil {
			t.Fatalf("failed to get %T: %+v", object, err)
		}

		if condition(object) {
			return
		}

		time.Sleep(waitInterval)
	}
	t.Fatalf("timeout waiting for condition on %s/%s after %v", key.Namespace, key.Name, time.Duration(maxAttempts)*waitInterval)
}

type testEnvironment struct {
	ctx        context.Context
	kubeClient client.Client
}

func createTestNamespace(t *testing.T, environment *testEnvironment) *corev1.Namespace {
	timestamp := time.Now().UnixNano()
	namespaceName := fmt.Sprintf("e2e-test-%d", timestamp)

	namespace := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: namespaceName,
		},
	}
	if err := environment.kubeClient.Create(environment.ctx, namespace); err != nil {
		t.Fatalf("failed to create test namespace: %+v", err)
	}
	t.Cleanup(func() {
		deletePolicy := metav1.DeletePropagationForeground
		deleteOptions := client.DeleteOptions{
			PropagationPolicy: &deletePolicy,
		}
		if err := environment.kubeClient.Delete(environment.ctx, namespace, &deleteOptions); err != nil {
			t.Logf("failed to delete test namespace: %+v", err)
		}
	})

	return namespace
}

func createTestNodes(t *testing.T, environment *testEnvironment, ips []string) []*corev1.Node {
	timestamp := time.Now().UnixNano()
	var nodes []*corev1.Node

	for i, ip := range ips {
		nodeName := fmt.Sprintf("e2e-test-%d-%d", i, timestamp)
		node := &corev1.Node{
			ObjectMeta: metav1.ObjectMeta{
				Name: nodeName,
			},
			Status: corev1.NodeStatus{
				Addresses: []corev1.NodeAddress{
					{
						Type:    corev1.NodeInternalIP,
						Address: ip,
					},
				},
			},
		}
		if err := environment.kubeClient.Create(environment.ctx, node); err != nil {
			t.Fatalf("failed to create node %s: %+v", nodeName, err)
		}
		nodes = append(nodes, node)
		t.Cleanup(func() {
			node := &corev1.Node{}
			if err := environment.kubeClient.Get(environment.ctx, types.NamespacedName{Name: nodeName}, node); err == nil {
				if err := environment.kubeClient.Delete(environment.ctx, node); err != nil {
					t.Logf("failed to cleanup node %s: %+v", nodeName, err)
				}
			}
		})
	}

	return nodes
}

func createNodePortService(t *testing.T, environment *testEnvironment, namespace string) *corev1.Service {
	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "e2e-test-nodeport-controller",
			Namespace: namespace,
		},
		Spec: corev1.ServiceSpec{
			Type: corev1.ServiceTypeNodePort,
			Selector: map[string]string{
				"app.kubernetes.io/name": "e2e-test-nodeport-controller",
			},
			Ports: []corev1.ServicePort{
				{
					Port:       80,
					TargetPort: intstr.FromInt32(8080),
				},
			},
		},
	}
	if err := environment.kubeClient.Create(environment.ctx, service); err != nil {
		t.Fatalf("failed to create service: %+v", err)
	}
	return service
}

func waitForServiceAnnotation(t *testing.T, environment *testEnvironment, name string, namespace string, annotationKey string, predicate func(string) bool) {
	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}

	waitForCondition(t, environment, service, func(service *corev1.Service) bool {
		annotation := service.Annotations[annotationKey]
		t.Logf("annotation: %s", annotation)
		return predicate(annotation)
	})
}

func TestE2E(t *testing.T) {
	logf.SetLogger(zap.New(zap.UseDevMode(true)))

	ctx := context.Background()
	environment := &envtest.Environment{
		UseExistingCluster: ptr.To(true),
	}

	configuration, err := environment.Start()
	if err != nil {
		t.Fatalf("unable to start environment: %+v", err)
	}
	t.Cleanup(func() {
		if err := environment.Stop(); err != nil {
			t.Errorf("unable to stop environment: %+v", err)
		}
	})

	kubeClient, err := client.New(configuration, client.Options{Scheme: clientgoscheme.Scheme})
	if err != nil {
		t.Fatalf("failed to create kubeClient: %+v", err)
	}

	m, err := ctrl.NewManager(configuration, ctrl.Options{
		Scheme: clientgoscheme.Scheme,
		Metrics: server.Options{
			BindAddress: "0",
		},
		Client: client.Options{
			Cache: &client.CacheOptions{
				DisableFor: []client.Object{},
			},
		},
	})
	if err != nil {
		t.Fatalf("unable to create manager: %+v", err)
	}

	if err := (&controllers.NodePortController{
		Client:   m.GetClient(),
		Scheme:   m.GetScheme(),
		Log:      ctrl.Log.WithName("controllers").WithName("NodePort"),
		Recorder: m.GetEventRecorderFor("nodeport-controller"),
	}).SetupWithManager(m); err != nil {
		t.Fatalf("unable to create controller: %+v", err)
	}

	ctx, cancel := context.WithCancel(ctx)
	t.Cleanup(cancel)

	go func() {
		if err := m.Start(ctx); err != nil {
			t.Logf("problem running manager: %+v", err)
		}
	}()

	if !m.GetCache().WaitForCacheSync(ctx) {
		t.Fatalf("failed to wait for cache sync")
	}

	e := &testEnvironment{
		ctx:        ctx,
		kubeClient: kubeClient,
	}

	tests := []struct {
		name string
		in   func(*testing.T, *testEnvironment) *corev1.Namespace
		want func(*testing.T, *testEnvironment, *corev1.Namespace)
	}{
		{
			name: "Two nodes with NodePort service",
			in: func(t *testing.T, environment *testEnvironment) *corev1.Namespace {
				namespace := createTestNamespace(t, environment)
				createTestNodes(t, environment, []string{
					"10.0.0.1",
					"10.0.0.2",
				})
				createNodePortService(t, environment, namespace.Name)
				return namespace
			},
			want: func(t *testing.T, environment *testEnvironment, namespace *corev1.Namespace) {
				waitForServiceAnnotation(t, environment, "e2e-test-nodeport-controller", namespace.Name,
					"nodeport-controller.kaidotio.github.io/nodes", func(annotation string) bool {
						return strings.Contains(annotation, "10.0.0.1") && strings.Contains(annotation, "10.0.0.2")
					})
			},
		},
		{
			name: "Three nodes with NodePort service",
			in: func(t *testing.T, environment *testEnvironment) *corev1.Namespace {
				namespace := createTestNamespace(t, environment)
				createTestNodes(t, environment, []string{
					"10.0.0.1",
					"10.0.0.2",
					"10.0.0.3",
				})
				createNodePortService(t, environment, namespace.Name)
				return namespace
			},
			want: func(t *testing.T, environment *testEnvironment, namespace *corev1.Namespace) {
				waitForServiceAnnotation(t, environment, "e2e-test-nodeport-controller", namespace.Name,
					"nodeport-controller.kaidotio.github.io/nodes", func(annotation string) bool {
						return strings.Contains(annotation, "10.0.0.1") &&
							strings.Contains(annotation, "10.0.0.2") &&
							strings.Contains(annotation, "10.0.0.3")
					})
			},
		},
		{
			name: "Single node with NodePort service",
			in: func(t *testing.T, environment *testEnvironment) *corev1.Namespace {
				namespace := createTestNamespace(t, environment)
				createTestNodes(t, environment, []string{
					"192.168.1.1",
				})
				createNodePortService(t, environment, namespace.Name)
				return namespace
			},
			want: func(t *testing.T, environment *testEnvironment, namespace *corev1.Namespace) {
				waitForServiceAnnotation(t, environment, "e2e-test-nodeport-controller", namespace.Name,
					"nodeport-controller.kaidotio.github.io/nodes", func(annotation string) bool {
						return strings.Contains(annotation, "192.168.1.1")
					})
			},
		},
		{
			name: "Node deletion updates service",
			in: func(t *testing.T, environment *testEnvironment) *corev1.Namespace {
				namespace := createTestNamespace(t, environment)
				nodes := createTestNodes(t, environment, []string{
					"10.0.0.1",
					"10.0.0.2",
					"10.0.0.3",
				})
				service := createNodePortService(t, environment, namespace.Name)
				waitForServiceAnnotation(t, environment, service.Name, namespace.Name,
					"nodeport-controller.kaidotio.github.io/nodes", func(annotation string) bool {
						return strings.Contains(annotation, "10.0.0.1") &&
							strings.Contains(annotation, "10.0.0.2") &&
							strings.Contains(annotation, "10.0.0.3")
					})

				if err := environment.kubeClient.Delete(environment.ctx, nodes[1]); err != nil {
					t.Fatalf("failed to delete node %s: %+v", nodes[1].Name, err)
				}

				time.Sleep(2 * time.Second)

				serviceToUpdate := &corev1.Service{}
				if err := environment.kubeClient.Get(environment.ctx, types.NamespacedName{Name: service.Name, Namespace: namespace.Name}, serviceToUpdate); err != nil {
					t.Fatalf("failed to get service: %+v", err)
				}
				if serviceToUpdate.Annotations == nil {
					serviceToUpdate.Annotations = make(map[string]string)
				}
				serviceToUpdate.Annotations["trigger-reconcile"] = fmt.Sprintf("%d", time.Now().UnixNano())
				if err := environment.kubeClient.Update(environment.ctx, serviceToUpdate); err != nil {
					t.Fatalf("failed to update service: %+v", err)
				}
				return namespace
			},
			want: func(t *testing.T, environment *testEnvironment, namespace *corev1.Namespace) {
				waitForServiceAnnotation(t, environment, "e2e-test-nodeport-controller", namespace.Name,
					"nodeport-controller.kaidotio.github.io/nodes", func(annotation string) bool {
						return strings.Contains(annotation, "10.0.0.1") &&
							strings.Contains(annotation, "10.0.0.3") &&
							!strings.Contains(annotation, "10.0.0.2")
					})
			},
		},
	}

	for _, tt := range tests {
		name := tt.name
		setup := tt.in
		verify := tt.want
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			namespace := setup(t, e)
			verify(t, e, namespace)
		})
	}
}
