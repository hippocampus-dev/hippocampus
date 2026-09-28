package main

import (
	"bytes"
	"context"
	"net"
	"net/http"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func p[T any](v T) *T {
	return &v
}

type recordingWriter struct {
	bytes.Buffer
}

func (w *recordingWriter) SetWriteDeadline(deadline time.Time) error {
	return nil
}

func TestHTTPProtocolParser_Forward(t *testing.T) {
	tests := []struct {
		name  string
		input []byte
		want  []byte
	}{
		{
			name:  "request without body",
			input: []byte("GET / HTTP/1.1\r\nHost: example.com\r\n\r\n"),
			want:  []byte("GET / HTTP/1.1\r\nHost: example.com\r\n\r\n"),
		},
		{
			name:  "request with content length",
			input: []byte("POST / HTTP/1.1\r\nHost: example.com\r\nContent-Length: 5\r\n\r\nhello"),
			want:  []byte("POST / HTTP/1.1\r\nHost: example.com\r\nContent-Length: 5\r\n\r\nhello"),
		},
		{
			name:  "chunked request with trailer",
			input: []byte("POST / HTTP/1.1\r\nHost: example.com\r\nTransfer-Encoding: chunked\r\nTrailer: Checksum\r\n\r\n5\r\nhello\r\n0\r\nChecksum: 1\r\n\r\n"),
			want:  []byte("POST / HTTP/1.1\r\nHost: example.com\r\nTransfer-Encoding: chunked\r\nTrailer: Checksum\r\n\r\n5\r\nhello\r\n0\r\nChecksum: 1\r\n\r\n"),
		},
		{
			name:  "pipelined requests",
			input: []byte("GET /1 HTTP/1.1\r\nHost: example.com\r\n\r\nGET /2 HTTP/1.1\r\nHost: example.com\r\n\r\n"),
			want:  []byte("GET /1 HTTP/1.1\r\nHost: example.com\r\n\r\n"),
		},
	}
	for _, tt := range tests {
		name := tt.name
		input := tt.input
		want := tt.want
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			parser := NewHTTPProtocolParser(bytes.NewReader(input))
			request, err := parser.ParseRequest()
			if err != nil {
				t.Errorf("HTTPProtocolParser.ParseRequest() error = %+v", err)
				return
			}
			writer := &recordingWriter{}
			if err := parser.Forward(request.Body, writer, 0); err != nil {
				t.Errorf("HTTPProtocolParser.Forward() error = %+v", err)
				return
			}
			if got := writer.Bytes(); !bytes.Equal(got, want) {
				t.Errorf("HTTPProtocolParser.Forward() wrote %q, want %q", got, want)
			}
		})
	}
}

func TestHTTPProtocolParser_ParseResponse(t *testing.T) {
	tests := []struct {
		name          string
		requestMethod string
		input         []byte
		want          []byte
	}{
		{
			name:          "response with content length",
			requestMethod: http.MethodGet,
			input:         []byte("HTTP/1.1 200 OK\r\nContent-Length: 5\r\n\r\nhello"),
			want:          []byte("HTTP/1.1 200 OK\r\nContent-Length: 5\r\n\r\nhello"),
		},
		{
			name:          "response to HEAD carries no body",
			requestMethod: http.MethodHead,
			input:         []byte("HTTP/1.1 200 OK\r\nContent-Length: 5\r\n\r\nHTTP/1.1 200 OK\r\nContent-Length: 0\r\n\r\n"),
			want:          []byte("HTTP/1.1 200 OK\r\nContent-Length: 5\r\n\r\n"),
		},
		{
			name:          "no content",
			requestMethod: http.MethodGet,
			input:         []byte("HTTP/1.1 204 No Content\r\n\r\n"),
			want:          []byte("HTTP/1.1 204 No Content\r\n\r\n"),
		},
		{
			name:          "informational response",
			requestMethod: http.MethodPost,
			input:         []byte("HTTP/1.1 100 Continue\r\n\r\nHTTP/1.1 200 OK\r\nContent-Length: 0\r\n\r\n"),
			want:          []byte("HTTP/1.1 100 Continue\r\n\r\n"),
		},
	}
	for _, tt := range tests {
		name := tt.name
		requestMethod := tt.requestMethod
		input := tt.input
		want := tt.want
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			parser := NewHTTPProtocolParser(bytes.NewReader(input))
			response, err := parser.ParseResponse(&http.Request{Method: requestMethod})
			if err != nil {
				t.Errorf("HTTPProtocolParser.ParseResponse() error = %+v", err)
				return
			}
			writer := &recordingWriter{}
			if err := parser.Forward(response.Body, writer, 0); err != nil {
				t.Errorf("HTTPProtocolParser.Forward() error = %+v", err)
				return
			}
			if got := writer.Bytes(); !bytes.Equal(got, want) {
				t.Errorf("HTTPProtocolParser.Forward() wrote %q, want %q", got, want)
			}
		})
	}
}

func TestRedisProtocolParser_Parse(t *testing.T) {
	tests := []struct {
		name    string
		input   []byte
		want    RESPMessage
		wantErr bool
	}{
		{
			name:    "simple string",
			input:   []byte("+OK\r\n"),
			want:    &RESPSimpleString{s: "OK"},
			wantErr: false,
		},
		{
			name:    "error",
			input:   []byte("-Error message\r\n"),
			want:    &RESPError{s: "Error message"},
			wantErr: false,
		},
		{
			name:    "integer",
			input:   []byte(":1000\r\n"),
			want:    &RESPInteger{i: 1000},
			wantErr: false,
		},
		{
			name:    "bulk string",
			input:   []byte("$6\r\nfoobar\r\n"),
			want:    &RESPBulkString{s: p("foobar")},
			wantErr: false,
		},
		{
			name:    "bulk string empty",
			input:   []byte("$0\r\n\r\n"),
			want:    &RESPBulkString{s: p("")},
			wantErr: false,
		},
		{
			name:    "bulk string null",
			input:   []byte("$-1\r\n"),
			want:    &RESPBulkString{s: nil},
			wantErr: false,
		},
		{
			name:    "bulk string shorter than its length",
			input:   []byte("$6\r\nfoo\r\n"),
			want:    nil,
			wantErr: true,
		},
		{
			name:    "array",
			input:   []byte("*2\r\n$3\r\nfoo\r\n$3\r\nbar\r\n"),
			want:    &RESPArray{a: []RESPMessage{&RESPBulkString{s: p("foo")}, &RESPBulkString{s: p("bar")}}},
			wantErr: false,
		},
		{
			name:    "array null",
			input:   []byte("*-1\r\n"),
			want:    &RESPArray{a: nil},
			wantErr: false,
		},
		{
			name:    "invalid type",
			input:   []byte("invalid\r\n"),
			want:    nil,
			wantErr: true,
		},
		{
			name:    "invalid bulk string length",
			input:   []byte("$invalid\r\n"),
			want:    nil,
			wantErr: true,
		},
		{
			name:    "invalid array length",
			input:   []byte("*invalid\r\n"),
			want:    nil,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		name := tt.name
		input := tt.input
		want := tt.want
		wantErr := tt.wantErr
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			parser := NewRedisProtocolParser(bytes.NewReader(input))
			got, err := parser.Parse()
			if err != nil {
				if !wantErr {
					t.Errorf("RedisProtocolParser.Parse() error = %+v", err)
				}
				return
			}
			if wantErr {
				t.Errorf("RedisProtocolParser.Parse() error = nil, want error")
				return
			}
			if got.String() != want.String() {
				t.Errorf("RedisProtocolParser.Parse() = %s, want %s", got.String(), want.String())
			}
		})
	}
}

func BenchmarkRedisProtocolParser_Parse(b *testing.B) {
	input := []byte("*2\r\n$3\r\nfoo\r\n$3\r\nbar\r\n")
	for i := 0; i < b.N; i++ {
		parser := NewRedisProtocolParser(bytes.NewReader(input))
		_, _ = parser.Parse()
	}
}

func TestUpstreamRelay_complete(t *testing.T) {
	tests := []struct {
		name       string
		dispatched int
		forwarded  int
		wantDirty  bool
	}{
		{
			name:       "response relayed for a forwarded request",
			dispatched: 1,
			forwarded:  1,
			wantDirty:  false,
		},
		{
			name:       "response relayed before the request was forwarded",
			dispatched: 1,
			forwarded:  0,
			wantDirty:  true,
		},
		{
			name:       "message relayed with no request behind it",
			dispatched: 0,
			forwarded:  0,
			wantDirty:  true,
		},
	}
	for _, tt := range tests {
		name := tt.name
		dispatched := tt.dispatched
		forwarded := tt.forwarded
		wantDirty := tt.wantDirty
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			connection := &Connection{}
			relay := NewUpstreamRelay[struct{}](connection)
			for i := 0; i < dispatched; i++ {
				relay.dispatch(struct{}{})
			}
			for i := 0; i < forwarded; i++ {
				relay.markForwarded()
			}
			relay.complete()
			if connection.dirty != wantDirty {
				t.Errorf("Connection.dirty = %t, want %t", connection.dirty, wantDirty)
			}
		})
	}
}

func TestUpstreamRelay_drain(t *testing.T) {
	tests := []struct {
		name          string
		dispatched    bool
		buffered      bool
		completeAfter time.Duration
		closeAfter    time.Duration
		timeout       time.Duration
		wantTimedOut  bool
	}{
		{
			name:          "no exchange in flight",
			dispatched:    false,
			buffered:      false,
			completeAfter: 0,
			closeAfter:    0,
			timeout:       time.Second,
			wantTimedOut:  false,
		},
		{
			name:          "response relayed before the timeout",
			dispatched:    true,
			buffered:      false,
			completeAfter: 10 * time.Millisecond,
			closeAfter:    0,
			timeout:       time.Second,
			wantTimedOut:  false,
		},
		{
			name:          "response never relayed",
			dispatched:    true,
			buffered:      false,
			completeAfter: 0,
			closeAfter:    0,
			timeout:       50 * time.Millisecond,
			wantTimedOut:  true,
		},
		{
			name:          "request read but not dispatched",
			dispatched:    false,
			buffered:      true,
			completeAfter: 0,
			closeAfter:    0,
			timeout:       50 * time.Millisecond,
			wantTimedOut:  true,
		},
		{
			name:          "client gone before the timeout",
			dispatched:    true,
			buffered:      false,
			completeAfter: 0,
			closeAfter:    10 * time.Millisecond,
			timeout:       time.Second,
			wantTimedOut:  false,
		},
	}
	for _, tt := range tests {
		name := tt.name
		dispatched := tt.dispatched
		buffered := tt.buffered
		completeAfter := tt.completeAfter
		closeAfter := tt.closeAfter
		timeout := tt.timeout
		wantTimedOut := tt.wantTimedOut
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			relay := NewUpstreamRelay[*http.Request](&Connection{})
			if dispatched {
				relay.dispatch(&http.Request{})
			}
			if buffered {
				relay.markBuffered(true)
			}
			if completeAfter > 0 {
				go func() {
					time.Sleep(completeAfter)
					relay.complete()
				}()
			}
			if closeAfter > 0 {
				go func() {
					time.Sleep(closeAfter)
					relay.close()
				}()
			}
			start := time.Now()
			relay.drain(timeout)
			if timedOut := time.Since(start) >= timeout; timedOut != wantTimedOut {
				t.Errorf("UpstreamRelay.drain() timed out = %t, want %t", timedOut, wantTimedOut)
			}
		})
	}
}

func TestConnectionPool_Get(t *testing.T) {
	mockServer, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Error(err)
	}
	defer mockServer.Close()

	go func() {
		for {
			conn, err := mockServer.Accept()
			if err != nil {
				return
			}

			go func() {
				defer conn.Close()

				buf := make([]byte, 1024)
				for {
					if _, err := conn.Read(buf); err != nil {
						break
					}
				}
			}()
		}
	}()

	poolOption := &ConnectionPoolOption{
		MaxConnections:     3,
		MinIdleConnections: 0,
		MaxIdleConnections: 2,
		Jitter: func(duration time.Duration) time.Duration {
			return duration
		},
		Dialer: func(ctx context.Context) (net.Conn, error) {
			return net.Dial("tcp", mockServer.Addr().String())
		},
		ConnectionPoolStrategy: FIFO,
	}

	pool := NewConnectionPool(poolOption)

	conn, err := pool.Get(t.Context(), "")
	if err != nil {
		t.Error(err)
	}

	if conn == nil {
		t.Error("Connection is nil")
	}

	if _, err := conn.Write([]byte("1")); err != nil {
		t.Error(err)
	}
}

func TestConnectionPool_ReUseConnection(t *testing.T) {
	mockServer, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Error(err)
	}
	defer mockServer.Close()

	go func() {
		for {
			conn, err := mockServer.Accept()
			if err != nil {
				return
			}

			go func() {
				defer conn.Close()

				buf := make([]byte, 1024)
				for {
					if _, err := conn.Read(buf); err != nil {
						break
					}
				}
			}()
		}
	}()

	poolOption := &ConnectionPoolOption{
		MaxConnections:     3,
		MinIdleConnections: 0,
		MaxIdleConnections: 2,
		Jitter: func(duration time.Duration) time.Duration {
			return duration
		},
		Dialer: func(ctx context.Context) (net.Conn, error) {
			return net.Dial("tcp", mockServer.Addr().String())
		},
		ConnectionPoolStrategy: FIFO,
	}

	pool := NewConnectionPool(poolOption)

	conn1, err := pool.Get(t.Context(), "")
	if err != nil {
		t.Error(err)
	}

	if conn1 == nil {
		t.Error("conn1 is nil")
	}

	if _, err := conn1.Write([]byte("1")); err != nil {
		t.Error(err)
	}

	if err := pool.Put(t.Context(), conn1); err != nil {
		t.Error(err)
	}

	if pool.IdleConnections() != 1 {
		t.Error("expected 1 idle connection")
	}

	conn2, err := pool.Get(t.Context(), "")
	if err != nil {
		t.Error(err)
	}

	if conn2 == nil {
		t.Error("conn2 is nil")
	}

	if _, err := conn2.Write([]byte("1")); err != nil {
		t.Error(err)
	}

	if conn1.LocalAddr() != conn2.LocalAddr() {
		t.Error("conn1 and conn2 are not the same")
	}

	if pool.IdleConnections() != 0 {
		t.Error("expected 0 idle connection")
	}
}

func TestConnectionPool_DirtyConnection(t *testing.T) {
	mockServer, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Error(err)
	}
	defer mockServer.Close()

	go func() {
		for {
			conn, err := mockServer.Accept()
			if err != nil {
				return
			}

			go func() {
				defer conn.Close()

				buf := make([]byte, 1024)
				for {
					if _, err := conn.Read(buf); err != nil {
						break
					}
				}
			}()
		}
	}()

	poolOption := &ConnectionPoolOption{
		MaxConnections:     3,
		MinIdleConnections: 0,
		MaxIdleConnections: 2,
		Jitter: func(duration time.Duration) time.Duration {
			return duration
		},
		Dialer: func(ctx context.Context) (net.Conn, error) {
			return net.Dial("tcp", mockServer.Addr().String())
		},
		ConnectionPoolStrategy: FIFO,
	}

	pool := NewConnectionPool(poolOption)

	conn, err := pool.Get(t.Context(), "")
	if err != nil {
		t.Error(err)
	}

	if conn == nil {
		t.Error("Connection is nil")
	}

	conn.markDirty()

	if err := pool.Put(t.Context(), conn); err != nil {
		t.Error(err)
	}

	if pool.IdleConnections() != 0 {
		t.Error("expected 0 idle connection")
	}

	if pool.Connections() != 0 {
		t.Error("expected 0 connection")
	}
}

func TestConnectionPool_NoIdle(t *testing.T) {
	mockServer, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Error(err)
	}
	defer mockServer.Close()

	go func() {
		for {
			conn, err := mockServer.Accept()
			if err != nil {
				return
			}

			go func() {
				defer conn.Close()

				buf := make([]byte, 1024)
				for {
					if _, err := conn.Read(buf); err != nil {
						break
					}
				}
			}()
		}
	}()

	poolOption := &ConnectionPoolOption{
		MaxConnections:     3,
		MinIdleConnections: 0,
		MaxIdleConnections: 0,
		Jitter: func(duration time.Duration) time.Duration {
			return duration
		},
		Dialer: func(ctx context.Context) (net.Conn, error) {
			return net.Dial("tcp", mockServer.Addr().String())
		},
		ConnectionPoolStrategy: FIFO,
	}

	pool := NewConnectionPool(poolOption)

	conn1, err := pool.Get(t.Context(), "")
	if err != nil {
		t.Error(err)
	}

	if conn1 == nil {
		t.Error("conn1 is nil")
	}

	if _, err := conn1.Write([]byte("1")); err != nil {
		t.Error(err)
	}

	if err := pool.Put(t.Context(), conn1); err != nil {
		t.Error(err)
	}

	if pool.IdleConnections() != 0 {
		t.Error("expected 0 idle connection")
	}

	conn2, err := pool.Get(t.Context(), "")
	if err != nil {
		t.Error(err)
	}

	if conn2 == nil {
		t.Error("conn2 is nil")
	}

	if _, err := conn2.Write([]byte("1")); err != nil {
		t.Error(err)
	}

	if conn1.LocalAddr() == conn2.LocalAddr() {
		t.Error("conn1 and conn2 are the same")
	}
}

func TestConnectionPool_MinIdleConnections(t *testing.T) {
	mockServer, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Error(err)
	}
	defer mockServer.Close()

	go func() {
		for {
			conn, err := mockServer.Accept()
			if err != nil {
				return
			}

			go func() {
				defer conn.Close()

				buf := make([]byte, 1024)
				for {
					if _, err := conn.Read(buf); err != nil {
						break
					}
				}
			}()
		}
	}()

	poolOption := &ConnectionPoolOption{
		MaxConnections:     3,
		MinIdleConnections: 2,
		MaxIdleConnections: 2,
		Jitter: func(duration time.Duration) time.Duration {
			return duration
		},
		Dialer: func(ctx context.Context) (net.Conn, error) {
			return net.Dial("tcp", mockServer.Addr().String())
		},
		ConnectionPoolStrategy: FIFO,
	}

	pool := NewConnectionPool(poolOption)

	time.Sleep(10 * time.Millisecond)

	if pool.IdleConnections() != 2 {
		t.Error("expected 2 idle connection")
	}

	conn1, err := pool.Get(t.Context(), "")
	if err != nil {
		t.Error(err)
	}

	if conn1 == nil {
		t.Error("conn1 is nil")
	}

	if _, err := conn1.Write([]byte("1")); err != nil {
		t.Error(err)
	}

	time.Sleep(10 * time.Millisecond)

	if pool.IdleConnections() != 2 {
		t.Error("expected 2 idle connection")
	}

	conn2, err := pool.Get(t.Context(), "")
	if err != nil {
		t.Error(err)
	}

	if _, err := conn2.Write([]byte("1")); err != nil {
		t.Error(err)
	}

	if conn2 == nil {
		t.Error("conn2 is nil")
	}

	time.Sleep(10 * time.Millisecond)

	if pool.IdleConnections() != 2 {
		t.Error("expected 2 idle connection")
	}
}

func TestConnectionPool_MaxIdleConnections(t *testing.T) {
	mockServer, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Error(err)
	}
	defer mockServer.Close()

	go func() {
		for {
			conn, err := mockServer.Accept()
			if err != nil {
				return
			}

			go func() {
				defer conn.Close()

				buf := make([]byte, 1024)
				for {
					if _, err := conn.Read(buf); err != nil {
						break
					}
				}
			}()
		}
	}()

	poolOption := &ConnectionPoolOption{
		MaxConnections:     3,
		MinIdleConnections: 0,
		MaxIdleConnections: 2,
		Jitter: func(duration time.Duration) time.Duration {
			return duration
		},
		Dialer: func(ctx context.Context) (net.Conn, error) {
			return net.Dial("tcp", mockServer.Addr().String())
		},
		ConnectionPoolStrategy: FIFO,
	}

	pool := NewConnectionPool(poolOption)

	conn1, err := pool.Get(t.Context(), "")
	if err != nil {
		t.Error(err)
	}

	if conn1 == nil {
		t.Error("conn1 is nil")
	}

	if _, err := conn1.Write([]byte("1")); err != nil {
		t.Error(err)
	}

	conn2, err := pool.Get(t.Context(), "")
	if err != nil {
		t.Error(err)
	}

	if conn2 == nil {
		t.Error("conn2 is nil")
	}

	if _, err := conn2.Write([]byte("1")); err != nil {
		t.Error(err)
	}

	conn3, err := pool.Get(t.Context(), "")
	if err != nil {
		t.Error(err)
	}

	if conn3 == nil {
		t.Error("conn3 is nil")
	}

	if _, err := conn3.Write([]byte("1")); err != nil {
		t.Error(err)
	}

	if err := pool.Put(t.Context(), conn1); err != nil {
		t.Error(err)
	}

	if pool.IdleConnections() != 1 {
		t.Error("expected 1 idle connection")
	}

	if err := pool.Put(t.Context(), conn2); err != nil {
		t.Error(err)
	}

	if pool.IdleConnections() != 2 {
		t.Error("expected 2 idle connection")
	}

	if err := pool.Put(t.Context(), conn3); err != nil {
		t.Error(err)
	}

	if pool.IdleConnections() != 2 {
		t.Error("expected 2 idle connection")
	}
}

func TestConnectionPool_MaxConnections(t *testing.T) {
	mockServer, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Error(err)
	}
	defer mockServer.Close()

	go func() {
		for {
			conn, err := mockServer.Accept()
			if err != nil {
				return
			}

			go func() {
				defer conn.Close()

				buf := make([]byte, 1024)
				for {
					if _, err := conn.Read(buf); err != nil {
						break
					}
				}
			}()
		}
	}()

	poolOption := &ConnectionPoolOption{
		MaxConnections:     3,
		MinIdleConnections: 0,
		MaxIdleConnections: 2,
		Jitter: func(duration time.Duration) time.Duration {
			return duration
		},
		Dialer: func(ctx context.Context) (net.Conn, error) {
			return net.Dial("tcp", mockServer.Addr().String())
		},
		ConnectionPoolStrategy: FIFO,
	}

	pool := NewConnectionPool(poolOption)

	conn1, err := pool.Get(t.Context(), "")
	if err != nil {
		t.Error(err)
	}

	if conn1 == nil {
		t.Error("conn1 is nil")
	}

	if _, err := conn1.Write([]byte("1")); err != nil {
		t.Error(err)
	}

	conn2, err := pool.Get(t.Context(), "")
	if err != nil {
		t.Error(err)
	}

	if conn2 == nil {
		t.Error("conn2 is nil")
	}

	if _, err := conn2.Write([]byte("1")); err != nil {
		t.Error(err)
	}

	conn3, err := pool.Get(t.Context(), "")
	if err != nil {
		t.Error(err)
	}

	if conn3 == nil {
		t.Error("conn3 is not nil")
	}

	if _, err := conn3.Write([]byte("1")); err != nil {
		t.Error(err)
	}

	if pool.Connections() != 3 {
		t.Error("expected 3 connections")
	}

	if _, err := pool.Get(t.Context(), ""); err == nil {
		t.Error("expected connection pool is full")
	}
}

func TestConnection_MaxIdleTime(t *testing.T) {
	mockServer, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Error(err)
	}
	defer mockServer.Close()

	go func() {
		for {
			conn, err := mockServer.Accept()
			if err != nil {
				return
			}

			go func() {
				defer conn.Close()

				buf := make([]byte, 1024)
				for {
					if _, err := conn.Read(buf); err != nil {
						break
					}
				}
			}()
		}
	}()

	poolOption := &ConnectionPoolOption{
		MaxConnections:     3,
		MinIdleConnections: 0,
		MaxIdleConnections: 2,
		MaxIdleTime:        10 * time.Millisecond,
		Jitter: func(duration time.Duration) time.Duration {
			return duration
		},
		Dialer: func(ctx context.Context) (net.Conn, error) {
			return net.Dial("tcp", mockServer.Addr().String())
		},
		ConnectionPoolStrategy: FIFO,
	}

	pool := NewConnectionPool(poolOption)

	conn1, err := pool.Get(t.Context(), "")
	if err != nil {
		t.Error(err)
	}

	if conn1 == nil {
		t.Error("conn1 is nil")
	}

	if _, err := conn1.Write([]byte("1")); err != nil {
		t.Error(err)
	}

	err = pool.Put(t.Context(), conn1)
	if err != nil {
		t.Error(err)
	}

	if pool.IdleConnections() != 1 {
		t.Error("expected 1 idle connection")
	}

	time.Sleep(20 * time.Millisecond)

	conn2, err := pool.Get(t.Context(), "")
	if err != nil {
		t.Error(err)
	}

	if conn2 == nil {
		t.Error("conn2 is nil")
	}

	if _, err := conn2.Write([]byte("1")); err != nil {
		t.Error(err)
	}

	if pool.IdleConnections() != 0 {
		t.Error("expected 0 idle connection")
	}
}

func TestConnection_MaxLifetime(t *testing.T) {
	mockServer, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Error(err)
	}
	defer mockServer.Close()

	go func() {
		for {
			conn, err := mockServer.Accept()
			if err != nil {
				return
			}

			go func() {
				defer conn.Close()

				buf := make([]byte, 1024)
				for {
					if _, err := conn.Read(buf); err != nil {
						break
					}
				}
			}()
		}
	}()

	poolOption := &ConnectionPoolOption{
		MaxConnections:     3,
		MinIdleConnections: 0,
		MaxIdleConnections: 2,
		MaxLifetime:        10 * time.Millisecond,
		Jitter: func(duration time.Duration) time.Duration {
			return duration
		},
		Dialer: func(ctx context.Context) (net.Conn, error) {
			return net.Dial("tcp", mockServer.Addr().String())
		},
		ConnectionPoolStrategy: FIFO,
	}

	pool := NewConnectionPool(poolOption)

	conn1, err := pool.Get(t.Context(), "")
	if err != nil {
		t.Error(err)
	}

	if conn1 == nil {
		t.Error("conn1 is nil")
	}

	if _, err := conn1.Write([]byte("1")); err != nil {
		t.Error(err)
	}

	time.Sleep(10 * time.Millisecond)

	if err := pool.Put(t.Context(), conn1); err != nil {
		t.Error(err)
	}

	conn2, err := pool.Get(t.Context(), "")
	if err != nil {
		t.Error(err)
	}

	if conn2 == nil {
		t.Error("conn2 is nil")
	}

	if _, err := conn2.Write([]byte("1")); err != nil {
		t.Error(err)
	}

	if pool.IdleConnections() != 0 {
		t.Error("expected 0 idle connection")
	}
}

func TestConnection_Topology(t *testing.T) {
	mockServer1Called := uint64(0)
	mockServer1, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Error(err)
	}
	defer mockServer1.Close()

	go func() {
		for {
			conn, err := mockServer1.Accept()
			if err != nil {
				return
			}

			go func() {
				defer conn.Close()

				buf := make([]byte, 1024)
				for {
					n, err := conn.Read(buf)
					if err != nil {
						break
					}
					atomic.AddUint64(&mockServer1Called, 1)
					if _, err := conn.Write(buf[:n]); err != nil {
						break
					}
				}
			}()
		}
	}()

	mockServer2Called := uint64(0)
	mockServer2, err := net.Listen("tcp", "127.0.0.2:0")
	if err != nil {
		t.Error(err)
	}
	defer mockServer2.Close()

	go func() {
		for {
			conn, err := mockServer2.Accept()
			if err != nil {
				return
			}

			go func() {
				defer conn.Close()

				buf := make([]byte, 1024)
				for {
					n, err := conn.Read(buf)
					if err != nil {
						break
					}
					atomic.AddUint64(&mockServer2Called, 1)
					if _, err := conn.Write(buf[:n]); err != nil {
						break
					}
				}
			}()
		}
	}()

	var topologyList []Topology
	topologyList = append(topologyList, Topology{
		Name: "default",
		CIDR: net.IPNet{
			IP:   mockServer1.Addr().(*net.TCPAddr).IP,
			Mask: net.CIDRMask(32, 32),
		},
	})

	dialCount := uint64(0)
	poolOption := &ConnectionPoolOption{
		MaxConnections:         3,
		MinIdleConnections:     2,
		MaxIdleConnections:     2,
		TopologyList:           topologyList,
		ConnectionPoolStrategy: FIFO,
		Dialer: func(ctx context.Context) (net.Conn, error) {
			if atomic.AddUint64(&dialCount, 1)%2 == 0 {
				return net.Dial("tcp", mockServer1.Addr().String())
			} else {
				return net.Dial("tcp", mockServer2.Addr().String())
			}
		},
	}

	pool := NewConnectionPool(poolOption)

	time.Sleep(10 * time.Millisecond)

	if pool.IdleConnections() != 2 {
		t.Error("expected 2 idle connection")
	}

	conn1, err := pool.Get(t.Context(), "default")
	if err != nil {
		t.Error(err)
	}

	if conn1 == nil {
		t.Error("conn1 is nil")
	}

	if _, err := conn1.Write([]byte("1")); err != nil {
		t.Error(err)
	}

	buf := make([]byte, 1024)
	if _, err := conn1.Read(buf); err != nil {
		t.Error(err)
	}

	if err := pool.Put(t.Context(), conn1); err != nil {
		t.Error(err)
	}

	if pool.IdleConnections() != 2 {
		t.Error("expected 2 idle connection")
	}

	if atomic.LoadUint64(&mockServer1Called) == 0 {
		t.Error("expected mockServer1 is called")
	}

	if atomic.LoadUint64(&mockServer2Called) > 0 {
		t.Error("expected mockServer2 is not called")
	}
}

func TestConnection_FallbackToKnownTopology(t *testing.T) {
	mockServer, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Error(err)
	}
	defer mockServer.Close()

	go func() {
		for {
			conn, err := mockServer.Accept()
			if err != nil {
				return
			}

			go func() {
				defer conn.Close()

				buf := make([]byte, 1024)
				for {
					if _, err := conn.Read(buf); err != nil {
						break
					}
				}
			}()
		}
	}()

	var topologyList []Topology
	topologyList = append(topologyList, Topology{
		Name: "default",
		CIDR: net.IPNet{
			IP:   mockServer.Addr().(*net.TCPAddr).IP,
			Mask: net.CIDRMask(32, 32),
		},
	})

	poolOption := &ConnectionPoolOption{
		MaxConnections:     3,
		MinIdleConnections: 2,
		MaxIdleConnections: 2,
		TopologyList:       topologyList,
		Jitter: func(duration time.Duration) time.Duration {
			return duration
		},
		Dialer: func(ctx context.Context) (net.Conn, error) {
			return net.Dial("tcp", mockServer.Addr().String())
		},
		ConnectionPoolStrategy: FIFO,
	}

	pool := NewConnectionPool(poolOption)

	time.Sleep(10 * time.Millisecond)

	if pool.IdleConnections() != 2 {
		t.Error("expected 2 idle connection")
	}

	conn1, err := pool.Get(t.Context(), "dummy")
	if err != nil {
		t.Error(err)
	}

	if conn1 == nil {
		t.Error("conn1 is nil")
	}

	if _, err := conn1.Write([]byte("1")); err != nil {
		t.Error(err)
	}

	if err := pool.Put(t.Context(), conn1); err != nil {
		t.Error(err)
	}

	if pool.IdleConnections() != 2 {
		t.Error("expected 2 idle connection")
	}
}

func TestConnection_UnknownTopology(t *testing.T) {
	mockServer, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Error(err)
	}
	defer mockServer.Close()

	go func() {
		for {
			conn, err := mockServer.Accept()
			if err != nil {
				return
			}

			go func() {
				defer conn.Close()

				buf := make([]byte, 1024)
				for {
					if _, err := conn.Read(buf); err != nil {
						break
					}
				}
			}()
		}
	}()

	var topologyList []Topology
	topologyList = append(topologyList, Topology{
		Name: "unknown",
		CIDR: net.IPNet{
			IP:   net.ParseIP("1.1.1.1"),
			Mask: net.CIDRMask(32, 32),
		},
	})

	poolOption := &ConnectionPoolOption{
		MaxConnections:     3,
		MinIdleConnections: 2,
		MaxIdleConnections: 2,
		TopologyList:       topologyList,
		Jitter: func(duration time.Duration) time.Duration {
			return duration
		},
		Dialer: func(ctx context.Context) (net.Conn, error) {
			return net.Dial("tcp", mockServer.Addr().String())
		},
		ConnectionPoolStrategy: FIFO,
	}

	pool := NewConnectionPool(poolOption)

	time.Sleep(10 * time.Millisecond)

	if pool.IdleConnections() != 2 {
		t.Error("expected 2 idle connection")
	}

	conn1, err := pool.Get(t.Context(), "dummy")
	if err != nil {
		t.Error(err)
	}

	if conn1 == nil {
		t.Error("conn1 is nil")
	}

	if _, err := conn1.Write([]byte("1")); err != nil {
		t.Error(err)
	}

	if err := pool.Put(t.Context(), conn1); err != nil {
		t.Error(err)
	}

	if pool.IdleConnections() != 2 {
		t.Error("expected 2 idle connection")
	}
}

func TestConnection_Concurrency(t *testing.T) {
	mockServer, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Error(err)
	}
	defer mockServer.Close()

	go func() {
		for {
			conn, err := mockServer.Accept()
			if err != nil {
				return
			}

			go func() {
				defer conn.Close()

				buf := make([]byte, 1024)
				for {
					if _, err := conn.Read(buf); err != nil {
						break
					}
				}
			}()
		}
	}()

	var topologyList []Topology
	topologyList = append(topologyList, Topology{
		Name: "default",
		CIDR: net.IPNet{
			IP:   mockServer.Addr().(*net.TCPAddr).IP,
			Mask: net.CIDRMask(32, 32),
		},
	})

	poolOption := &ConnectionPoolOption{
		MaxConnections:     100,
		MinIdleConnections: 2,
		MaxIdleConnections: 5,
		TopologyList:       topologyList,
		Jitter: func(duration time.Duration) time.Duration {
			return duration
		},
		Dialer: func(ctx context.Context) (net.Conn, error) {
			return net.Dial("tcp", mockServer.Addr().String())
		},
		ConnectionPoolStrategy: FIFO,
	}

	pool := NewConnectionPool(poolOption)

	time.Sleep(10 * time.Millisecond)

	if pool.IdleConnections() != 2 {
		t.Error("expected 2 idle connection")
	}

	wg := sync.WaitGroup{}
	for i := 0; i < 100; i++ {
		wg.Go(func() {
			conn, err := pool.Get(t.Context(), "default")
			if err != nil {
				t.Error(err)
			}

			if conn == nil {
				t.Error("conn is nil")
			}

			if _, err := conn.Write([]byte("1")); err != nil {
				t.Error(err)
			}

			if err := pool.Put(t.Context(), conn); err != nil {
				t.Error(err)
			}
		})
	}

	wg.Wait()

	if pool.IdleConnections() > 5 {
		t.Error("expected <= 5 idle connection")
	}

	if pool.Connections() != pool.IdleConnections() {
		t.Error("expected all connections are idle")
	}
}
