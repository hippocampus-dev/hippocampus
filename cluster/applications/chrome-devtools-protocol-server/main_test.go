package main

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestNewReverseProxyRewritesHostAndOrigin(t *testing.T) {
	t.Parallel()

	var host string
	var origin string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		host = r.Host
		origin = r.Header.Get("Origin")
	}))
	defer upstream.Close()

	remoteAddress := strings.TrimPrefix(upstream.URL, "http://")
	proxy := httptest.NewServer(newReverseProxy(remoteAddress))
	defer proxy.Close()

	request, err := http.NewRequest(http.MethodGet, proxy.URL+"/json/version", nil)
	if err != nil {
		t.Fatalf("failed to build request: %+v", err)
	}
	request.Header.Set("Origin", "http://chrome-devtools-protocol-server.example.com")

	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatalf("failed to request: %+v", err)
	}
	defer func() {
		_, _ = io.Copy(io.Discard, response.Body)
		_ = response.Body.Close()
	}()

	if host != remoteAddress {
		t.Errorf("host = %q, want %q", host, remoteAddress)
	}
	if origin != "" {
		t.Errorf("origin = %q, want empty", origin)
	}
}

func TestNewReverseProxyRewritesWebSocketDebuggerURL(t *testing.T) {
	t.Parallel()

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprintf(w, `{"webSocketDebuggerUrl":"ws://%s/devtools/browser/1"}`, r.Host)
	}))
	defer upstream.Close()

	proxy := httptest.NewServer(newReverseProxy(strings.TrimPrefix(upstream.URL, "http://")))
	defer proxy.Close()

	response, err := http.Get(proxy.URL + "/json/version")
	if err != nil {
		t.Fatalf("failed to request: %+v", err)
	}
	defer func() {
		_ = response.Body.Close()
	}()

	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatalf("failed to read body: %+v", err)
	}

	want := fmt.Sprintf(`{"webSocketDebuggerUrl":"ws://%s/devtools/browser/1"}`, strings.TrimPrefix(proxy.URL, "http://"))
	if string(body) != want {
		t.Errorf("body = %q, want %q", string(body), want)
	}
	if response.ContentLength != int64(len(want)) {
		t.Errorf("content length = %d, want %d", response.ContentLength, len(want))
	}
}

func TestNewReverseProxyProxiesWebSocketUpgrade(t *testing.T) {
	t.Parallel()

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		connection, buffer, err := w.(http.Hijacker).Hijack()
		if err != nil {
			return
		}
		defer func() {
			_ = connection.Close()
		}()

		if _, err := buffer.WriteString("HTTP/1.1 101 Switching Protocols\r\nUpgrade: websocket\r\nConnection: Upgrade\r\n\r\n"); err != nil {
			return
		}
		if err := buffer.Flush(); err != nil {
			return
		}

		line, err := buffer.ReadString('\n')
		if err != nil {
			return
		}
		if _, err := buffer.WriteString("echo:" + line); err != nil {
			return
		}
		_ = buffer.Flush()
	}))
	defer upstream.Close()

	proxy := httptest.NewServer(newReverseProxy(strings.TrimPrefix(upstream.URL, "http://")))
	defer proxy.Close()

	connection, err := net.Dial("tcp", strings.TrimPrefix(proxy.URL, "http://"))
	if err != nil {
		t.Fatalf("failed to dial: %+v", err)
	}
	defer func() {
		_ = connection.Close()
	}()

	if _, err := fmt.Fprintf(connection, "GET /devtools/browser/1 HTTP/1.1\r\nHost: %s\r\nUpgrade: websocket\r\nConnection: Upgrade\r\n\r\n", strings.TrimPrefix(proxy.URL, "http://")); err != nil {
		t.Fatalf("failed to write handshake: %+v", err)
	}

	buffer := bufio.NewReader(connection)
	status, err := buffer.ReadString('\n')
	if err != nil {
		t.Fatalf("failed to read status line: %+v", err)
	}
	if !strings.HasPrefix(status, "HTTP/1.1 101 ") {
		t.Fatalf("status line = %q, want 101", status)
	}
	for {
		line, err := buffer.ReadString('\n')
		if err != nil {
			t.Fatalf("failed to read header: %+v", err)
		}
		if line == "\r\n" {
			break
		}
	}

	if _, err := connection.Write([]byte("ping\n")); err != nil {
		t.Fatalf("failed to write payload: %+v", err)
	}

	line, err := buffer.ReadString('\n')
	if err != nil {
		t.Fatalf("failed to read payload: %+v", err)
	}
	if line != "echo:ping\n" {
		t.Errorf("payload = %q, want %q", line, "echo:ping\n")
	}
}
