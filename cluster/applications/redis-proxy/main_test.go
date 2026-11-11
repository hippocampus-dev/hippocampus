package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func p[T any](v T) *T {
	return &v
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
			name:          "no reply in flight",
			dispatched:    false,
			buffered:      false,
			completeAfter: 0,
			closeAfter:    0,
			timeout:       time.Second,
			wantTimedOut:  false,
		},
		{
			name:          "reply relayed before the timeout",
			dispatched:    true,
			buffered:      false,
			completeAfter: 10 * time.Millisecond,
			closeAfter:    0,
			timeout:       time.Second,
			wantTimedOut:  false,
		},
		{
			name:          "reply never relayed",
			dispatched:    true,
			buffered:      false,
			completeAfter: 0,
			closeAfter:    0,
			timeout:       50 * time.Millisecond,
			wantTimedOut:  true,
		},
		{
			name:          "command read but not dispatched",
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
			relay := NewUpstreamRelay()
			connection := &Connection{}
			if dispatched {
				relay.dispatch(connection)
			}
			if buffered {
				relay.markBuffered(true)
			}
			if completeAfter > 0 {
				go func() {
					time.Sleep(completeAfter)
					relay.complete(connection)
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

func resp(args ...string) string {
	var buffer strings.Builder
	fmt.Fprintf(&buffer, "*%d\r\n", len(args))
	for _, arg := range args {
		fmt.Fprintf(&buffer, "$%d\r\n%s\r\n", len(arg), arg)
	}
	return buffer.String()
}

type shortConn struct {
	net.Conn
	limit int
}

func (c *shortConn) Write(b []byte) (int, error) {
	if len(b) <= c.limit {
		return c.Conn.Write(b)
	}
	n, _ := c.Conn.Write(b[:c.limit])
	// A reset arrives after the replies to what got through, which the mirror reads in the meantime.
	time.Sleep(100 * time.Millisecond)
	return n, errors.New("connection reset by peer")
}

func TestMirror(t *testing.T) {
	large := strings.Repeat("x", 2*MirrorMaxBatchBytes)
	tests := []struct {
		name         string
		dialErr      error
		queueSize    int
		block        bool
		limits       []int
		shortWrite   int
		commands     [][]string
		wantPayloads []string
		wantSent     int
		wantDropped  int
		wantUnknown  int
	}{
		{
			name:         "commands mirrored in order",
			dialErr:      nil,
			queueSize:    16,
			block:        false,
			limits:       nil,
			shortWrite:   0,
			commands:     [][]string{{"SELECT", "3"}, {"SET", "a", "1"}, {"SET", "l", large}, {"DEL", "a"}},
			wantPayloads: []string{resp("SELECT", "3") + resp("SET", "a", "1") + resp("SET", "l", large) + resp("DEL", "a")},
			wantSent:     4,
			wantDropped:  0,
			wantUnknown:  0,
		},
		{
			name:         "commands written without a reply dropped",
			dialErr:      nil,
			queueSize:    16,
			block:        true,
			limits:       []int{2},
			shortWrite:   0,
			commands:     [][]string{{"SELECT", "3"}, {"SET", "a", "1"}, {"SET", "b", "2"}, {"SET", "c", "3"}},
			wantPayloads: []string{resp("SELECT", "3") + resp("SET", "a", "1") + resp("SET", "b", "2")},
			wantSent:     2,
			wantDropped:  0,
			wantUnknown:  2,
		},
		{
			name:         "transaction whose EXEC got no reply dropped",
			dialErr:      nil,
			queueSize:    16,
			block:        true,
			limits:       []int{2},
			shortWrite:   0,
			commands:     [][]string{{"MULTI"}, {"SET", "a", "1"}, {"EXEC"}, {"SET", "z", "1"}},
			wantPayloads: []string{resp("MULTI") + resp("SET", "a", "1") + resp("EXEC")},
			wantSent:     0,
			wantDropped:  0,
			wantUnknown:  4,
		},
		{
			name:         "rejected session command not replayed",
			dialErr:      nil,
			queueSize:    16,
			block:        true,
			limits:       nil,
			shortWrite:   len(resp("SELECT", "3")) + len(resp("SELECT", "invalid")) + 3,
			commands:     [][]string{{"SELECT", "3"}, {"SELECT", "invalid"}, {"SET", "a", "1"}},
			wantPayloads: []string{resp("SELECT", "3") + resp("SELECT", "invalid"), resp("SELECT", "3") + resp("SET", "a", "1")},
			wantSent:     3,
			wantDropped:  0,
			wantUnknown:  0,
		},
		{
			name:         "session changed in a transaction replayed",
			dialErr:      nil,
			queueSize:    16,
			block:        true,
			limits:       nil,
			shortWrite:   len(resp("SELECT", "1")) + len(resp("MULTI")) + len(resp("SELECT", "2")) + len(resp("EXEC")) + 3,
			commands:     [][]string{{"SELECT", "1"}, {"MULTI"}, {"SELECT", "2"}, {"EXEC"}, {"SET", "a", "1"}},
			wantPayloads: []string{resp("SELECT", "1") + resp("MULTI") + resp("SELECT", "2") + resp("EXEC"), resp("SELECT", "2") + resp("SET", "a", "1")},
			wantSent:     5,
			wantDropped:  0,
			wantUnknown:  0,
		},
		{
			name:         "session changed after a nested MULTI replayed",
			dialErr:      nil,
			queueSize:    16,
			block:        true,
			limits:       nil,
			shortWrite:   len(resp("MULTI")) + len(resp("MULTI")) + len(resp("SELECT", "2")) + len(resp("EXEC")) + 3,
			commands:     [][]string{{"MULTI"}, {"MULTI"}, {"SELECT", "2"}, {"EXEC"}, {"SET", "a", "1"}},
			wantPayloads: []string{resp("MULTI") + resp("MULTI") + resp("SELECT", "2") + resp("EXEC"), resp("SELECT", "2") + resp("SET", "a", "1")},
			wantSent:     5,
			wantDropped:  0,
			wantUnknown:  0,
		},
		{
			name:         "latest authentication replayed first",
			dialErr:      nil,
			queueSize:    16,
			block:        true,
			limits:       nil,
			shortWrite:   len(resp("HELLO", "2", "AUTH", "u2", "p2")) + len(resp("HELLO", "2")),
			commands:     [][]string{{"AUTH", "u1", "p1"}, {"HELLO", "2", "AUTH", "u2", "p2"}, {"HELLO", "2"}, {"SET", "l", large}},
			wantPayloads: []string{resp("AUTH", "u1", "p1") + resp("HELLO", "2", "AUTH", "u2", "p2") + resp("HELLO", "2"), resp("HELLO", "2", "AUTH", "u2", "p2") + resp("HELLO", "2") + resp("SET", "l", large)},
			wantSent:     4,
			wantDropped:  0,
			wantUnknown:  0,
		},
		{
			name:         "rejected AUTH written apart from other commands",
			dialErr:      nil,
			queueSize:    16,
			block:        true,
			limits:       nil,
			shortWrite:   len(resp("AUTH", "invalid", "x")),
			commands:     [][]string{{"AUTH", "u", "p"}, {"AUTH", "invalid", "x"}, {"SET", "l", large}},
			wantPayloads: []string{resp("AUTH", "u", "p") + resp("AUTH", "invalid", "x"), resp("AUTH", "u", "p") + resp("SET", "l", large)},
			wantSent:     3,
			wantDropped:  0,
			wantUnknown:  0,
		},
		{
			name:         "rejected RESET keeps the session",
			dialErr:      nil,
			queueSize:    16,
			block:        true,
			limits:       nil,
			shortWrite:   len(resp("SELECT", "1")) + len(resp("RESET", "x")),
			commands:     [][]string{{"SELECT", "1"}, {"RESET", "x"}, {"SET", "l", large}},
			wantPayloads: []string{resp("SELECT", "1") + resp("RESET", "x"), resp("SELECT", "1") + resp("SET", "l", large)},
			wantSent:     3,
			wantDropped:  0,
			wantUnknown:  0,
		},
		{
			name:         "transaction written again counted once",
			dialErr:      nil,
			queueSize:    16,
			block:        true,
			limits:       nil,
			shortWrite:   len(resp("MULTI")) + len(resp("SET", "a", "1")) + 3,
			commands:     [][]string{{"MULTI"}, {"SET", "a", "1"}, {"SET", "l", large}, {"EXEC"}},
			wantPayloads: []string{resp("MULTI") + resp("SET", "a", "1"), resp("MULTI") + resp("SET", "a", "1") + resp("SET", "l", large) + resp("EXEC")},
			wantSent:     4,
			wantDropped:  0,
			wantUnknown:  0,
		},
		{
			name:         "rejected HELLO with AUTH written after the replies before it",
			dialErr:      nil,
			queueSize:    16,
			block:        true,
			limits:       nil,
			shortWrite:   0,
			commands:     [][]string{{"SET", "a", "1"}, {"HELLO", "2", "AUTH", "invalid", "x"}, {"SET", "b", "2"}},
			wantPayloads: []string{resp("SET", "a", "1") + resp("HELLO", "2", "AUTH", "invalid", "x") + resp("SET", "b", "2")},
			wantSent:     3,
			wantDropped:  0,
			wantUnknown:  0,
		},
		{
			name:         "client name reading AUTH not taken as authentication",
			dialErr:      nil,
			queueSize:    16,
			block:        true,
			limits:       nil,
			shortWrite:   len(resp("AUTH", "u", "p")) + len(resp("HELLO", "2", "SETNAME", "auth")),
			commands:     [][]string{{"AUTH", "u", "p"}, {"HELLO", "2", "SETNAME", "auth"}, {"SET", "l", large}},
			wantPayloads: []string{resp("AUTH", "u", "p") + resp("HELLO", "2", "SETNAME", "auth"), resp("AUTH", "u", "p") + resp("HELLO", "2", "SETNAME", "auth") + resp("SET", "l", large)},
			wantSent:     3,
			wantDropped:  0,
			wantUnknown:  0,
		},
		{
			name:         "transaction kept open by DISCARD with arguments",
			dialErr:      nil,
			queueSize:    16,
			block:        false,
			limits:       nil,
			shortWrite:   0,
			commands:     [][]string{{"MULTI"}, {"SET", "a", "1"}, {"DISCARD", "x"}, {"SET", "b", "2"}, {"EXEC"}},
			wantPayloads: []string{resp("MULTI") + resp("SET", "a", "1") + resp("DISCARD", "x") + resp("SET", "b", "2") + resp("EXEC")},
			wantSent:     5,
			wantDropped:  0,
			wantUnknown:  0,
		},
		{
			name:         "commands left unwritten by a failed write sent again",
			dialErr:      nil,
			queueSize:    16,
			block:        true,
			limits:       []int{0},
			shortWrite:   len(resp("SET", "a", "1")) + 3,
			commands:     [][]string{{"SET", "a", "1"}, {"SET", "b", "2"}, {"SET", "c", "3"}},
			wantPayloads: []string{resp("SET", "a", "1"), resp("SET", "b", "2") + resp("SET", "c", "3")},
			wantSent:     2,
			wantDropped:  0,
			wantUnknown:  1,
		},
		{
			name:         "session replayed on a new connection",
			dialErr:      nil,
			queueSize:    16,
			block:        true,
			limits:       []int{0},
			shortWrite:   len(resp("SELECT", "3")) + len(resp("SET", "a", "1")) + 3,
			commands:     [][]string{{"SELECT", "3"}, {"SET", "a", "1"}, {"SET", "b", "2"}},
			wantPayloads: []string{resp("SELECT", "3"), resp("SELECT", "3") + resp("SET", "b", "2")},
			wantSent:     1,
			wantDropped:  0,
			wantUnknown:  2,
		},
		{
			name:         "transaction left unwritten sent whole again",
			dialErr:      nil,
			queueSize:    16,
			block:        true,
			limits:       []int{0},
			shortWrite:   len(resp("SET", "a", "1")) + len(resp("MULTI")) + 3,
			commands:     [][]string{{"SET", "a", "1"}, {"MULTI"}, {"SET", "b", "2"}, {"EXEC"}},
			wantPayloads: []string{resp("SET", "a", "1"), resp("MULTI") + resp("SET", "b", "2") + resp("EXEC")},
			wantSent:     3,
			wantDropped:  0,
			wantUnknown:  1,
		},
		{
			name:         "discarded transaction never sent",
			dialErr:      nil,
			queueSize:    16,
			block:        false,
			limits:       nil,
			shortWrite:   0,
			commands:     [][]string{{"MULTI"}, {"SET", "a", "1"}, {"DISCARD"}, {"SET", "b", "2"}},
			wantPayloads: []string{resp("SET", "b", "2")},
			wantSent:     1,
			wantDropped:  0,
			wantUnknown:  0,
		},
		{
			name:         "queue full drops all but session commands",
			dialErr:      nil,
			queueSize:    2,
			block:        true,
			limits:       nil,
			shortWrite:   0,
			commands:     [][]string{{"SELECT", "1"}, {"SET", "a", "1"}, {"SET", "b", "2"}, {"SELECT", "2"}, {"MULTI"}, {"SET", "c", "3"}, {"EXEC"}},
			wantPayloads: []string{resp("SELECT", "1") + resp("SET", "a", "1") + resp("SELECT", "2")},
			wantSent:     3,
			wantDropped:  4,
			wantUnknown:  0,
		},
		{
			name:         "upstream unreachable",
			dialErr:      errors.New("connection refused"),
			queueSize:    16,
			block:        false,
			limits:       nil,
			shortWrite:   0,
			commands:     [][]string{{"SET", "a", "1"}, {"SET", "b", "2"}},
			wantPayloads: nil,
			wantSent:     0,
			wantDropped:  2,
			wantUnknown:  0,
		},
	}
	for _, tt := range tests {
		name := tt.name
		dialErr := tt.dialErr
		queueSize := tt.queueSize
		block := tt.block
		limits := tt.limits
		shortWrite := tt.shortWrite
		commands := tt.commands
		wantPayloads := tt.wantPayloads
		wantSent := tt.wantSent
		wantDropped := tt.wantDropped
		wantUnknown := tt.wantUnknown
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			listener, err := net.Listen("tcp", "127.0.0.1:0")
			if err != nil {
				t.Fatalf("net.Listen error: %+v", err)
			}
			defer listener.Close()

			// Each connection replies as Redis does, rejecting a command that holds "invalid" and leaving out the reply of a rejected AUTH or HELLO read together with an earlier command, and one given a limit stops replying and hangs up after that many.
			payloads := make([]string, len(wantPayloads))
			served := sync.WaitGroup{}
			served.Add(len(wantPayloads))
			go func() {
				for i := range wantPayloads {
					conn, err := listener.Accept()
					if err != nil {
						for range len(wantPayloads) - i {
							served.Done()
						}
						return
					}
					go func() {
						defer served.Done()
						defer conn.Close()
						var payload strings.Builder
						var transaction []string
						parser := NewRedisProtocolParser(conn)
						for n := 0; ; n++ {
							pipelined := parser.Buffered() > 0
							message, err := parser.Parse()
							if err != nil {
								break
							}
							payload.WriteString(message.String())
							if i < len(limits) && n == limits[i] {
								_ = conn.(*net.TCPConn).CloseWrite()
								_, _ = io.Copy(io.Discard, conn)
								break
							}
							a := message.(*RESPArray).a
							command := strings.ToUpper(*a[0].(*RESPBulkString).s)
							reply := "+OK\r\n"
							if strings.Contains(message.String(), "invalid") {
								reply = "-ERR invalid\r\n"
								if (command == "AUTH" || command == "HELLO") && pipelined {
									continue
								}
							}
							switch {
							case transaction != nil && command == "EXEC" && len(a) == 1:
								reply = fmt.Sprintf("*%d\r\n%s", len(transaction), strings.Join(transaction, ""))
								transaction = nil
							case transaction != nil && command == "DISCARD" && len(a) == 1:
								transaction = nil
							case transaction != nil && command == "MULTI":
								reply = "-ERR MULTI calls can not be nested\r\n"
							case transaction != nil:
								transaction = append(transaction, reply)
								reply = "+QUEUED\r\n"
							case command == "MULTI":
								transaction = []string{}
							}
							if _, err := conn.Write([]byte(reply)); err != nil {
								break
							}
						}
						payloads[i] = payload.String()
					}()
				}
			}()

			release := make(chan struct{})
			var dials int64
			var mutex sync.Mutex
			results := make(map[string]int)
			var queued int64
			wg := sync.WaitGroup{}
			mirror := NewMirror(&MirrorOption{
				Dialer: func(ctx context.Context) (net.Conn, error) {
					if block {
						<-release
					}
					if dialErr != nil {
						return nil, dialErr
					}
					conn, err := net.Dial("tcp", listener.Addr().String())
					if err == nil && shortWrite > 0 && atomic.AddInt64(&dials, 1) == 1 {
						return &shortConn{Conn: conn, limit: shortWrite}, nil
					}
					return conn, err
				},
				QueueSize:    queueSize,
				WriteTimeout: 100 * time.Millisecond,
				DrainTimeout: 300 * time.Millisecond,
				Observe: func(result string, n int) {
					mutex.Lock()
					defer mutex.Unlock()
					results[result] += n
				},
				Queued: func(delta int64) {
					atomic.AddInt64(&queued, delta)
				},
				Failed: func(command string, code string) {},
			}, &wg)

			for _, command := range commands {
				b := []byte(resp(command...))
				a := make([]RESPMessage, len(command))
				for i, arg := range command {
					a[i] = &RESPBulkString{s: p(arg)}
				}
				mirror.Send(command[0], a, b)
				// The mirror has to own its copy, as the request loop reuses the buffer.
				copy(b, bytes.Repeat([]byte{'x'}, len(b)))
			}
			close(release)
			mirror.Close()

			done := make(chan struct{})
			go func() {
				wg.Wait()
				close(done)
			}()
			timer := time.NewTimer(10 * time.Second)
			defer timer.Stop()
			select {
			case <-done:
			case <-timer.C:
				t.Fatalf("Mirror did not finish")
			}
			served.Wait()

			for i, want := range wantPayloads {
				if payloads[i] != want {
					t.Errorf("Mirror payload on connection %d = %.200q, want %.200q", i, payloads[i], want)
				}
			}
			if got := atomic.LoadInt64(&queued); got != 0 {
				t.Errorf("Mirror queued = %d after close, want 0", got)
			}
			mutex.Lock()
			defer mutex.Unlock()
			if results["sent"] != wantSent || results["dropped"] != wantDropped || results["unknown"] != wantUnknown {
				t.Errorf("Mirror sent = %d, dropped = %d, unknown = %d, want sent = %d, dropped = %d, unknown = %d", results["sent"], results["dropped"], results["unknown"], wantSent, wantDropped, wantUnknown)
			}
		})
	}
}

func TestMirrorErrorCodes(t *testing.T) {
	tests := []struct {
		name    string
		message RESPMessage
		want    []string
	}{
		{"ok", &RESPSimpleString{s: "OK"}, nil},
		{"error", &RESPError{s: "WRONGTYPE Operation against a key holding the wrong kind of value"}, []string{"WRONGTYPE"}},
		{"error without a message", &RESPError{s: "ERR"}, []string{"ERR"}},
		{"transaction", &RESPArray{a: []RESPMessage{&RESPSimpleString{s: "OK"}, &RESPError{s: "OOM command not allowed"}, &RESPInteger{i: 1}, &RESPError{s: "ERR value is not an integer"}}}, []string{"OOM", "ERR"}},
		{"array without errors", &RESPArray{a: []RESPMessage{&RESPInteger{i: 1}}}, nil},
	}
	for _, tt := range tests {
		name := tt.name
		message := tt.message
		want := tt.want
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if got := mirrorErrorCodes(message); !slices.Equal(got, want) {
				t.Errorf("mirrorErrorCodes() = %v, want %v", got, want)
			}
		})
	}
}

func TestIsMirroredCommand(t *testing.T) {
	tests := []struct {
		name    string
		command []string
		want    bool
	}{
		{"write", []string{"SET", "k", "v"}, true},
		{"write in lowercase", []string{"hset", "k", "f", "v"}, true},
		{"session state", []string{"SELECT", "1"}, true},
		{"transaction", []string{"MULTI"}, true},
		{"read", []string{"GET", "k"}, false},
		{"watch", []string{"WATCH", "k"}, false},
		{"subscription", []string{"SUBSCRIBE", "c"}, false},
		{"introspection", []string{"COMMAND"}, false},
		{"quit", []string{"QUIT"}, false},
		{"client", []string{"CLIENT", "SETNAME", "app"}, false},
		{"blocking", []string{"BLPOP", "k", "0"}, false},
		{"replication", []string{"WAIT", "1", "0"}, false},
		{"server role", []string{"SHUTDOWN"}, false},
		{"migration", []string{"MIGRATE", "host", "6379", "k", "0", "1000", "REPLACE"}, false},
		{"debug", []string{"DEBUG", "SLEEP", "1"}, false},
		{"read left out of reader routing", []string{"TTL", "k"}, false},
		{"container of reads", []string{"MEMORY", "USAGE", "k"}, false},
		{"consumer group read", []string{"XREADGROUP", "GROUP", "g", "c", "STREAMS", "s", ">"}, true},
		{"blocking consumer group read", []string{"XREADGROUP", "GROUP", "g", "c", "BLOCK", "0", "STREAMS", "s", ">"}, false},
		{"consumer group named block", []string{"XREADGROUP", "GROUP", "block", "c", "COUNT", "1", "STREAMS", "s", ">"}, true},
		{"stream named block", []string{"XREADGROUP", "GROUP", "g", "c", "STREAMS", "block", ">"}, true},
		{"consumer named block", []string{"XREADGROUP", "GROUP", "g", "block", "STREAMS", "s", ">"}, true},
		{"blocking read by a consumer named streams", []string{"XREADGROUP", "GROUP", "g", "streams", "BLOCK", "0", "STREAMS", "s", ">"}, false},
	}
	for _, tt := range tests {
		name := tt.name
		command := tt.command
		want := tt.want
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			a := make([]RESPMessage, len(command))
			for i, arg := range command {
				a[i] = &RESPBulkString{s: p(arg)}
			}
			if got := isMirroredCommand(command[0], a); got != want {
				t.Errorf("isMirroredCommand(%v) = %t, want %t", command, got, want)
			}
		})
	}
}

type resetConn struct {
	net.Conn
}

func (c *resetConn) Write(b []byte) (int, error) {
	return 0, errors.New("connection reset by peer")
}

func TestMirror_run(t *testing.T) {
	// An upstream that accepts connections only to reset them must not be dialed in a tight loop.
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("net.Listen error: %+v", err)
	}
	defer listener.Close()
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			_ = conn.Close()
		}
	}()

	var dials int64
	wg := sync.WaitGroup{}
	mirror := NewMirror(&MirrorOption{
		Dialer: func(ctx context.Context) (net.Conn, error) {
			atomic.AddInt64(&dials, 1)
			conn, err := net.Dial("tcp", listener.Addr().String())
			if err != nil {
				return nil, err
			}
			return &resetConn{Conn: conn}, nil
		},
		QueueSize:    16,
		WriteTimeout: 100 * time.Millisecond,
		DrainTimeout: 500 * time.Millisecond,
		Observe:      func(result string, n int) {},
		Queued:       func(delta int64) {},
		Failed:       func(command string, code string) {},
	}, &wg)
	mirror.Send("SET", []RESPMessage{&RESPBulkString{s: p("SET")}, &RESPBulkString{s: p("a")}, &RESPBulkString{s: p("1")}}, []byte(resp("SET", "a", "1")))
	mirror.Close()
	wg.Wait()

	if got := atomic.LoadInt64(&dials); got > 10 {
		t.Errorf("Mirror dialed %d times in 500ms, want backoff between dials", got)
	}
}

func TestMirror_serve(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("net.Listen error: %+v", err)
	}
	defer listener.Close()
	var received int64
	go func() {
		for i := 0; ; i++ {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			go func() {
				defer conn.Close()
				if i == 0 {
					_, _ = io.Copy(io.Discard, conn)
					return
				}
				parser := NewRedisProtocolParser(conn)
				for {
					message, err := parser.Parse()
					if err != nil {
						return
					}
					if *message.(*RESPArray).a[0].(*RESPBulkString).s == "SET" {
						atomic.AddInt64(&received, 1)
					}
					if _, err := conn.Write([]byte("-ERR invalid\r\n")); err != nil {
						return
					}
				}
			}()
		}
	}()

	hello := []string{"HELLO", "2", "AUTH", "u", "p"}
	release := make(chan struct{})
	var dials int64
	var mutex sync.Mutex
	var failed []string
	wg := sync.WaitGroup{}
	mirror := NewMirror(&MirrorOption{
		Dialer: func(ctx context.Context) (net.Conn, error) {
			<-release
			conn, err := net.Dial("tcp", listener.Addr().String())
			if err == nil && atomic.AddInt64(&dials, 1) == 1 {
				return &shortConn{Conn: conn, limit: len(resp(hello...))}, nil
			}
			return conn, err
		},
		QueueSize:    16,
		WriteTimeout: 100 * time.Millisecond,
		DrainTimeout: 2 * time.Second,
		Observe:      func(result string, n int) {},
		Queued:       func(delta int64) {},
		Failed: func(command string, code string) {
			time.Sleep(100 * time.Millisecond)
			mutex.Lock()
			defer mutex.Unlock()
			failed = append(failed, command)
		},
	}, &wg)
	for _, command := range [][]string{hello, {"SET", "a", "1"}} {
		a := make([]RESPMessage, len(command))
		for i, arg := range command {
			a[i] = &RESPBulkString{s: p(arg)}
		}
		mirror.Send(command[0], a, []byte(resp(command...)))
	}
	close(release)
	mirror.Close()
	wg.Wait()

	mutex.Lock()
	defer mutex.Unlock()
	if len(failed) == 0 || slices.ContainsFunc(failed, func(command string) bool { return command != "HELLO" }) {
		t.Errorf("Mirror failed commands = %v, want only HELLO", failed)
	}
	if got := atomic.LoadInt64(&received); got != 0 {
		t.Errorf("Mirror wrote SET %d times after the replayed HELLO was rejected, want 0", got)
	}
}

func TestMirror_receive(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("net.Listen error: %+v", err)
	}
	defer listener.Close()
	go func() {
		for i := 0; ; i++ {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			go func() {
				defer conn.Close()
				parser := NewRedisProtocolParser(conn)
				for {
					if _, err := parser.Parse(); err != nil {
						return
					}
					if _, err := conn.Write([]byte("+OK\r\n")); err != nil || i == 0 {
						return
					}
				}
			}()
		}
	}()

	var dials int64
	wg := sync.WaitGroup{}
	mirror := NewMirror(&MirrorOption{
		Dialer: func(ctx context.Context) (net.Conn, error) {
			conn, err := net.Dial("tcp", listener.Addr().String())
			if err != nil {
				return nil, err
			}
			if atomic.AddInt64(&dials, 1) == 1 {
				return conn, nil
			}
			return &shortConn{Conn: conn, limit: len(resp("SELECT", "1"))}, nil
		},
		QueueSize:    16,
		WriteTimeout: 100 * time.Millisecond,
		DrainTimeout: 2 * time.Second,
		Observe:      func(result string, n int) {},
		Queued:       func(delta int64) {},
		Failed:       func(command string, code string) {},
	}, &wg)
	mirror.Send("SELECT", []RESPMessage{&RESPBulkString{s: p("SELECT")}, &RESPBulkString{s: p("1")}}, []byte(resp("SELECT", "1")))
	time.Sleep(100 * time.Millisecond)
	mirror.Send("SET", []RESPMessage{&RESPBulkString{s: p("SET")}, &RESPBulkString{s: p("a")}, &RESPBulkString{s: p("1")}}, []byte(resp("SET", "a", "1")))
	mirror.Close()
	wg.Wait()

	if got := atomic.LoadInt64(&dials); got > 10 {
		t.Errorf("Mirror dialed %d times in 2s when only the replayed commands got replies, want backoff between dials", got)
	}
}

type mirrorUpstream struct {
	silent bool
	delay  time.Duration
	hangUp int
}

type mirrorStep struct {
	gap     time.Duration
	command []string
}

func TestMirror_serve_deadline(t *testing.T) {
	set := func(gap time.Duration, key string) mirrorStep {
		return mirrorStep{gap: gap, command: []string{"SET", key, "1"}}
	}
	every := func(n int, gap time.Duration) []mirrorStep {
		steps := make([]mirrorStep, n)
		for i := range steps {
			steps[i] = set(gap, strconv.Itoa(i))
		}
		return steps
	}
	silent := mirrorUpstream{silent: true, delay: 0, hangUp: 0}
	prompt := mirrorUpstream{silent: false, delay: 0, hangUp: 0}
	slow := mirrorUpstream{silent: false, delay: 80 * time.Millisecond, hangUp: 0}
	once := mirrorUpstream{silent: false, delay: 0, hangUp: 1}
	tests := []struct {
		name           string
		upstreams      []mirrorUpstream
		steps          []mirrorStep
		wait           time.Duration
		minConnections int64
		maxConnections int64
		wantSent       int
		wantLost       int
	}{
		{"silent upstream after a pause", []mirrorUpstream{silent}, []mirrorStep{set(0, "a"), set(400*time.Millisecond, "b")}, 50 * time.Millisecond, 2, 2, 0, 2},
		{"silent upstream while commands keep coming", []mirrorUpstream{silent}, every(7, 80*time.Millisecond), 50 * time.Millisecond, 2, 4, 0, 7},
		{"idle connection kept", []mirrorUpstream{prompt}, []mirrorStep{set(0, "a"), set(500*time.Millisecond, "b")}, 50 * time.Millisecond, 1, 1, 2, 0},
		{"busy connection with slow replies kept", []mirrorUpstream{{silent: false, delay: 50 * time.Millisecond, hangUp: 0}}, every(10, 50*time.Millisecond), 50 * time.Millisecond, 1, 1, 10, 0},
		{"slow replies after the client leaves", []mirrorUpstream{slow}, every(5, 0), 0, 0, 1, 5, 0},
		{"silent upstream after the client leaves", []mirrorUpstream{silent}, every(2, 0), 0, 0, 1, 0, 2},
		{"AUTH behind slow replies", []mirrorUpstream{slow}, []mirrorStep{set(0, "a"), {gap: 0, command: []string{"AUTH", "u", "p"}}, set(0, "b")}, 400 * time.Millisecond, 1, 1, 3, 0},
		{"AUTH behind a silent upstream", []mirrorUpstream{silent, prompt}, []mirrorStep{set(0, "a"), {gap: 0, command: []string{"AUTH", "u", "p"}}, set(0, "b")}, 500 * time.Millisecond, 2, 2, 2, 1},
		{"replayed session on a silent upstream", []mirrorUpstream{once, silent, prompt}, []mirrorStep{{gap: 0, command: []string{"SELECT", "1"}}, set(150*time.Millisecond, "a")}, 800 * time.Millisecond, 3, 3, 2, 0},
		{"replayed session answered slowly", []mirrorUpstream{once, slow}, []mirrorStep{{gap: 0, command: []string{"SELECT", "1"}}, set(150*time.Millisecond, "a")}, 400 * time.Millisecond, 2, 2, 2, 0},
	}
	for _, tt := range tests {
		name := tt.name
		upstreams := tt.upstreams
		steps := tt.steps
		wait := tt.wait
		minConnections := tt.minConnections
		maxConnections := tt.maxConnections
		wantSent := tt.wantSent
		wantLost := tt.wantLost
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			listener, err := net.Listen("tcp", "127.0.0.1:0")
			if err != nil {
				t.Fatalf("net.Listen error: %+v", err)
			}
			defer listener.Close()
			var accepted int64
			go func() {
				for {
					conn, err := listener.Accept()
					if err != nil {
						return
					}
					upstream := upstreams[min(int(atomic.AddInt64(&accepted, 1))-1, len(upstreams)-1)]
					go func() {
						defer conn.Close()
						parser := NewRedisProtocolParser(conn)
						for replies := 0; ; {
							if _, err := parser.Parse(); err != nil {
								return
							}
							if upstream.silent {
								continue
							}
							time.Sleep(upstream.delay)
							if _, err := conn.Write([]byte("+OK\r\n")); err != nil {
								return
							}
							if replies++; replies == upstream.hangUp {
								return
							}
						}
					}()
				}
			}()

			var mutex sync.Mutex
			results := make(map[string]int)
			wg := sync.WaitGroup{}
			mirror := NewMirror(&MirrorOption{
				Dialer: func(ctx context.Context) (net.Conn, error) {
					return net.Dial("tcp", listener.Addr().String())
				},
				QueueSize:    16,
				WriteTimeout: 200 * time.Millisecond,
				DrainTimeout: time.Second,
				Observe: func(result string, n int) {
					mutex.Lock()
					defer mutex.Unlock()
					results[result] += n
				},
				Queued: func(delta int64) {},
				Failed: func(command string, code string) {},
			}, &wg)
			for _, step := range steps {
				time.Sleep(step.gap)
				a := make([]RESPMessage, len(step.command))
				for i, arg := range step.command {
					a[i] = &RESPBulkString{s: p(arg)}
				}
				mirror.Send(step.command[0], a, []byte(resp(step.command...)))
			}
			time.Sleep(wait)
			got := atomic.LoadInt64(&accepted)
			mirror.Close()
			wg.Wait()

			if got < minConnections || got > maxConnections {
				t.Errorf("Mirror opened %d connections while the client stayed, want %d to %d", got, minConnections, maxConnections)
			}
			mutex.Lock()
			defer mutex.Unlock()
			if lost := results["unknown"] + results["dropped"]; results["sent"] != wantSent || lost != wantLost {
				t.Errorf("Mirror sent = %d, lost = %d, want sent = %d, lost = %d", results["sent"], lost, wantSent, wantLost)
			}
		})
	}
}

func TestRESPMessage_MarshalJSON(t *testing.T) {
	tests := []struct {
		name string
		in   RESPMessage
		want string
	}{
		{
			"simple string",
			&RESPSimpleString{s: "OK"},
			`"OK"`,
		},
		{
			"error",
			&RESPError{s: "ERR unknown command"},
			`{"error":"ERR unknown command"}`,
		},
		{
			"integer",
			&RESPInteger{i: 42},
			`42`,
		},
		{
			"bulk string",
			&RESPBulkString{s: p("hello")},
			`"hello"`,
		},
		{
			"bulk string null",
			&RESPBulkString{s: nil},
			`null`,
		},
		{
			"array",
			&RESPArray{a: []RESPMessage{&RESPBulkString{s: p("foo")}, &RESPBulkString{s: p("bar")}}},
			`["foo","bar"]`,
		},
		{
			"array null",
			&RESPArray{a: nil},
			`null`,
		},
	}
	for _, tt := range tests {
		name := tt.name
		in := tt.in
		want := tt.want
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			got, err := json.Marshal(in)
			if err != nil {
				t.Fatalf("json.Marshal error: %+v", err)
			}
			if string(got) != want {
				t.Errorf("want %s, got %s", want, string(got))
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
