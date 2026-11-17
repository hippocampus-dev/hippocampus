package proxy

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/go-logr/logr"
)

func TestUDPProxy(t *testing.T) {
	t.Parallel()

	backend, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to start backend: %v", err)
	}
	defer backend.Close()

	go func() {
		buf := make([]byte, udpBufferSize)
		for {
			n, addr, err := backend.ReadFrom(buf)
			if err != nil {
				return
			}
			_, _ = backend.WriteTo(buf[:n], addr)
		}
	}()

	backendAddr := backend.LocalAddr().(*net.UDPAddr)

	m := NewManager(logr.Discard(), ProtocolDisabled)
	defer m.Stop()

	proxyPort := freePort(t)
	if err := m.Update([]Route{
		{
			Port:     int32(proxyPort),
			Protocol: ProtocolUDP,
			Backends: []Backend{
				{Address: "127.0.0.1", Port: int32(backendAddr.Port)},
			},
		},
	}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	conn, err := net.DialTimeout("udp", fmt.Sprintf("127.0.0.1:%d", proxyPort), 2*time.Second)
	if err != nil {
		t.Fatalf("failed to connect to proxy: %v", err)
	}
	defer conn.Close()

	message := "hello-udp"
	_, err = conn.Write([]byte(message))
	if err != nil {
		t.Fatalf("failed to write: %v", err)
	}

	buf := make([]byte, 1024)
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	n, err := conn.Read(buf)
	if err != nil {
		t.Fatalf("failed to read: %v", err)
	}

	if got := string(buf[:n]); got != message {
		t.Errorf("got %q, want %q", got, message)
	}
}

func TestUDPProxyWithProxyProtocolV2(t *testing.T) {
	t.Parallel()

	receivedCh := make(chan []byte, 1)

	backend, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to start backend: %v", err)
	}
	defer backend.Close()

	go func() {
		buf := make([]byte, udpBufferSize)
		for {
			n, addr, err := backend.ReadFrom(buf)
			if err != nil {
				return
			}
			received := make([]byte, n)
			copy(received, buf[:n])
			receivedCh <- received

			// v2 IPv4 header is 28 bytes; echo back only the payload
			if n > 28 {
				_, _ = backend.WriteTo(buf[28:n], addr)
			}
		}
	}()

	backendAddr := backend.LocalAddr().(*net.UDPAddr)

	m := NewManager(logr.Discard(), ProtocolV2)
	defer m.Stop()

	proxyPort := freePort(t)
	if err := m.Update([]Route{
		{
			Port:     int32(proxyPort),
			Protocol: ProtocolUDP,
			Backends: []Backend{
				{Address: "127.0.0.1", Port: int32(backendAddr.Port)},
			},
		},
	}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	conn, err := net.DialTimeout("udp", fmt.Sprintf("127.0.0.1:%d", proxyPort), 2*time.Second)
	if err != nil {
		t.Fatalf("failed to connect to proxy: %v", err)
	}
	defer conn.Close()

	message := "hello-udp-v2"
	_, err = conn.Write([]byte(message))
	if err != nil {
		t.Fatalf("failed to write: %v", err)
	}

	select {
	case received := <-receivedCh:
		if len(received) < 28 {
			t.Fatalf("received datagram too short: %d bytes", len(received))
		}
		if !bytes.Equal(received[0:12], proxyProtocolV2Signature[:]) {
			t.Errorf("unexpected v2 signature: %x", received[0:12])
		}
		if received[12] != proxyProtocolV2VersionCommand {
			t.Errorf("unexpected version/command byte: %x, want %x", received[12], proxyProtocolV2VersionCommand)
		}
		if received[13] != proxyProtocolV2FamilyUDP4 {
			t.Errorf("unexpected family byte: %x, want %x", received[13], proxyProtocolV2FamilyUDP4)
		}
		addrLen := binary.BigEndian.Uint16(received[14:16])
		if addrLen != proxyProtocolV2IPv4AddrLen {
			t.Errorf("unexpected address length: %d, want %d", addrLen, proxyProtocolV2IPv4AddrLen)
		}
		payload := string(received[28:])
		if payload != message {
			t.Errorf("payload got %q, want %q", payload, message)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for UDP datagram with PROXY protocol v2 header")
	}

	buf := make([]byte, 1024)
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	n, err := conn.Read(buf)
	if err != nil {
		t.Fatalf("failed to read response: %v", err)
	}

	if got := string(buf[:n]); got != message {
		t.Errorf("response got %q, want %q", got, message)
	}
}
