package proxy

import (
	"context"
	"fmt"
	"io"
	"net"
	"sync"
	"sync/atomic"

	"github.com/go-logr/logr"
)

type Protocol string

const (
	ProtocolTCP Protocol = "TCP"
	ProtocolUDP Protocol = "UDP"
)

type ProtocolVersion string

const (
	ProtocolDisabled ProtocolVersion = ""
	ProtocolV1       ProtocolVersion = "1"
	ProtocolV2       ProtocolVersion = "2"
)

type Route struct {
	Port     int32
	Protocol Protocol
	Backends []Backend
}

type Backend struct {
	Address string
	Port    int32
}

type routeKey struct {
	Port     int32
	Protocol Protocol
}

type listener struct {
	cancel context.CancelFunc
	closer io.Closer
	route  Route
}

type Manager struct {
	mu                   sync.Mutex
	listeners            map[routeKey]*listener
	log                  logr.Logger
	ready                atomic.Bool
	proxyProtocolVersion ProtocolVersion
}

func NewManager(log logr.Logger, proxyProtocolVersion ProtocolVersion) *Manager {
	return &Manager{
		listeners:            make(map[routeKey]*listener),
		log:                  log,
		proxyProtocolVersion: proxyProtocolVersion,
	}
}

func (m *Manager) Update(routes []Route) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	desired := make(map[routeKey]Route)
	for _, r := range routes {
		desired[routeKey{Port: r.Port, Protocol: r.Protocol}] = r
	}

	for key, l := range m.listeners {
		if _, ok := desired[key]; !ok {
			l.cancel()
			l.closer.Close()
			delete(m.listeners, key)
		}
	}

	for key, route := range desired {
		if existing, ok := m.listeners[key]; ok {
			if routeEqual(existing.route, route) {
				continue
			}
			existing.cancel()
			existing.closer.Close()
			delete(m.listeners, key)
		}

		ctx, cancel := context.WithCancel(context.Background())
		closer, err := m.start(ctx, route)
		if err != nil {
			m.log.Error(err, "failed to listen", "port", key.Port, "protocol", route.Protocol)
			cancel()
			m.ready.Store(false)
			return fmt.Errorf("failed to bind port %d/%s: %w", key.Port, route.Protocol, err)
		}
		m.listeners[key] = &listener{
			cancel: cancel,
			closer: closer,
			route:  route,
		}
	}
	m.ready.Store(true)
	return nil
}

func (m *Manager) Stop() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for key, l := range m.listeners {
		l.cancel()
		l.closer.Close()
		delete(m.listeners, key)
	}
}

func (m *Manager) Ready() bool {
	return m.ready.Load()
}

func (m *Manager) start(ctx context.Context, route Route) (io.Closer, error) {
	switch route.Protocol {
	case ProtocolTCP:
		return m.startTCP(ctx, route)
	case ProtocolUDP:
		return m.startUDP(ctx, route)
	default:
		return nil, fmt.Errorf("unsupported protocol: %s", route.Protocol)
	}
}

func backendAddress(b Backend) string {
	return net.JoinHostPort(b.Address, fmt.Sprintf("%d", b.Port))
}

func routeEqual(a Route, b Route) bool {
	if a.Port != b.Port || a.Protocol != b.Protocol || len(a.Backends) != len(b.Backends) {
		return false
	}
	for i := range a.Backends {
		if a.Backends[i] != b.Backends[i] {
			return false
		}
	}
	return true
}
