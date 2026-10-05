package proxy

import (
	"testing"

	"github.com/go-logr/logr"
)

func TestManagerUpdateAndStop(t *testing.T) {
	t.Parallel()

	m := NewManager(logr.Discard(), ProtocolDisabled)

	if m.Ready() {
		t.Error("expected Ready() to be false before first Update")
	}

	port1 := freePort(t)
	port2 := freePort(t)

	if err := m.Update([]Route{
		{Port: int32(port1), Protocol: ProtocolTCP, Backends: []Backend{{Address: "127.0.0.1", Port: 9999}}},
		{Port: int32(port2), Protocol: ProtocolUDP, Backends: []Backend{{Address: "127.0.0.1", Port: 9999}}},
	}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !m.Ready() {
		t.Error("expected Ready() to be true after Update")
	}

	m.mu.Lock()
	if got := len(m.listeners); got != 2 {
		t.Errorf("got %d listeners, want 2", got)
	}
	m.mu.Unlock()

	if err := m.Update([]Route{
		{Port: int32(port1), Protocol: ProtocolTCP, Backends: []Backend{{Address: "127.0.0.1", Port: 9999}}},
	}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	m.mu.Lock()
	if got := len(m.listeners); got != 1 {
		t.Errorf("got %d listeners, want 1", got)
	}
	m.mu.Unlock()

	m.Stop()

	m.mu.Lock()
	if got := len(m.listeners); got != 0 {
		t.Errorf("got %d listeners, want 0", got)
	}
	m.mu.Unlock()
}
