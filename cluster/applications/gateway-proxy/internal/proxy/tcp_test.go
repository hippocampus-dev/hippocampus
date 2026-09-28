package proxy

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/go-logr/logr"
)

func TestTCPProxy(t *testing.T) {
	t.Parallel()

	backend, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to start backend: %v", err)
	}
	defer backend.Close()

	go func() {
		for {
			conn, err := backend.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				defer c.Close()
				buf := make([]byte, 1024)
				n, err := c.Read(buf)
				if err != nil {
					return
				}
				_, _ = c.Write(buf[:n])
			}(conn)
		}
	}()

	backendAddr := backend.Addr().(*net.TCPAddr)

	m := NewManager(logr.Discard(), ProtocolDisabled)
	defer m.Stop()

	proxyPort := freePort(t)
	if err := m.Update([]Route{
		{
			Port:     int32(proxyPort),
			Protocol: ProtocolTCP,
			Backends: []Backend{
				{Address: "127.0.0.1", Port: int32(backendAddr.Port)},
			},
		},
	}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", proxyPort), 2*time.Second)
	if err != nil {
		t.Fatalf("failed to connect to proxy: %v", err)
	}
	defer conn.Close()

	message := "hello"
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

func TestTCPProxyWithProxyProtocolV1(t *testing.T) {
	t.Parallel()

	headerCh := make(chan string, 1)

	backend, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to start backend: %v", err)
	}
	defer backend.Close()

	go func() {
		for {
			conn, err := backend.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				defer c.Close()
				reader := bufio.NewReader(c)
				line, err := reader.ReadString('\n')
				if err != nil {
					return
				}
				headerCh <- line

				buf := make([]byte, 1024)
				n, err := reader.Read(buf)
				if err != nil {
					return
				}
				_, _ = c.Write(buf[:n])
			}(conn)
		}
	}()

	backendAddr := backend.Addr().(*net.TCPAddr)

	m := NewManager(logr.Discard(), ProtocolV1)
	defer m.Stop()

	proxyPort := freePort(t)
	if err := m.Update([]Route{
		{
			Port:     int32(proxyPort),
			Protocol: ProtocolTCP,
			Backends: []Backend{
				{Address: "127.0.0.1", Port: int32(backendAddr.Port)},
			},
		},
	}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", proxyPort), 2*time.Second)
	if err != nil {
		t.Fatalf("failed to connect to proxy: %v", err)
	}
	defer conn.Close()

	message := "hello"
	_, err = conn.Write([]byte(message))
	if err != nil {
		t.Fatalf("failed to write: %v", err)
	}

	select {
	case header := <-headerCh:
		if !strings.HasPrefix(header, "PROXY TCP4 127.0.0.1 127.0.0.1 ") {
			t.Errorf("unexpected PROXY protocol header: %q", header)
		}
		if !strings.HasSuffix(header, "\r\n") {
			t.Errorf("PROXY protocol header missing CRLF: %q", header)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for PROXY protocol header")
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

func TestTCPProxyWithProxyProtocolV2(t *testing.T) {
	t.Parallel()

	headerCh := make(chan []byte, 1)

	backend, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to start backend: %v", err)
	}
	defer backend.Close()

	go func() {
		for {
			conn, err := backend.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				defer c.Close()
				// v2 header: 16 bytes fixed + 12 bytes IPv4 addresses = 28 bytes
				header := make([]byte, 28)
				if _, err := io.ReadFull(c, header); err != nil {
					return
				}
				headerCh <- header

				buf := make([]byte, 1024)
				n, err := c.Read(buf)
				if err != nil {
					return
				}
				_, _ = c.Write(buf[:n])
			}(conn)
		}
	}()

	backendAddr := backend.Addr().(*net.TCPAddr)

	m := NewManager(logr.Discard(), ProtocolV2)
	defer m.Stop()

	proxyPort := freePort(t)
	if err := m.Update([]Route{
		{
			Port:     int32(proxyPort),
			Protocol: ProtocolTCP,
			Backends: []Backend{
				{Address: "127.0.0.1", Port: int32(backendAddr.Port)},
			},
		},
	}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", proxyPort), 2*time.Second)
	if err != nil {
		t.Fatalf("failed to connect to proxy: %v", err)
	}
	defer conn.Close()

	message := "hello"
	_, err = conn.Write([]byte(message))
	if err != nil {
		t.Fatalf("failed to write: %v", err)
	}

	select {
	case header := <-headerCh:
		if !bytes.Equal(header[0:12], proxyProtocolV2Signature[:]) {
			t.Errorf("unexpected v2 signature: %x", header[0:12])
		}
		if header[12] != proxyProtocolV2VersionCommand {
			t.Errorf("unexpected version/command byte: %x, want %x", header[12], proxyProtocolV2VersionCommand)
		}
		if header[13] != proxyProtocolV2FamilyTCP4 {
			t.Errorf("unexpected family byte: %x, want %x", header[13], proxyProtocolV2FamilyTCP4)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for PROXY protocol v2 header")
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

func freePort(t *testing.T) int {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to get free port: %v", err)
	}
	port := l.Addr().(*net.TCPAddr).Port
	l.Close()
	return port
}
