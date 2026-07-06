package main

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/google/go-cmp/cmp"
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
			"simple string",
			[]byte("+OK\r\n"),
			&RESPSimpleString{s: "OK"},
			false,
		},
		{
			"error",
			[]byte("-Error message\r\n"),
			&RESPError{s: "Error message"},
			false,
		},
		{
			"integer",
			[]byte(":1000\r\n"),
			&RESPInteger{i: 1000},
			false,
		},
		{
			"bulk string",
			[]byte("$6\r\nfoobar\r\n"),
			&RESPBulkString{s: p("foobar")},
			false,
		},
		{
			"bulk string null",
			[]byte("$-1\r\n"),
			&RESPBulkString{s: nil},
			false,
		},
		{
			"array",
			[]byte("*2\r\n$3\r\nfoo\r\n$3\r\nbar\r\n"),
			&RESPArray{a: []RESPMessage{&RESPBulkString{s: p("foo")}, &RESPBulkString{s: p("bar")}}},
			false,
		},
		{
			"array null",
			[]byte("*-1\r\n"),
			&RESPArray{a: nil},
			false,
		},
		{
			"nested array",
			[]byte("*2\r\n*2\r\n$3\r\nfoo\r\n$3\r\nbar\r\n*1\r\n:42\r\n"),
			&RESPArray{a: []RESPMessage{
				&RESPArray{a: []RESPMessage{&RESPBulkString{s: p("foo")}, &RESPBulkString{s: p("bar")}}},
				&RESPArray{a: []RESPMessage{&RESPInteger{i: 42}}},
			}},
			false,
		},
		{
			"invalid type",
			[]byte("invalid\r\n"),
			nil,
			true,
		},
		{
			"invalid bulk string length",
			[]byte("$invalid\r\n"),
			nil,
			true,
		},
		{
			"invalid array length",
			[]byte("*invalid\r\n"),
			nil,
			true,
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
					t.Errorf("unexpected error: %+v", err)
				}
				return
			}
			if wantErr {
				t.Errorf("expected error, got nil")
				return
			}
			if diff := cmp.Diff(want.String(), got.String()); diff != "" {
				t.Errorf("(-want +got):\n%s", diff)
			}
		})
	}
}

func TestEncodeCommand(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{
			"single arg",
			[]string{"PING"},
			"*1\r\n$4\r\nPING\r\n",
		},
		{
			"two args",
			[]string{"GET", "key"},
			"*2\r\n$3\r\nGET\r\n$3\r\nkey\r\n",
		},
		{
			"three args",
			[]string{"SET", "key", "value"},
			"*3\r\n$3\r\nSET\r\n$3\r\nkey\r\n$5\r\nvalue\r\n",
		},
	}
	for _, tt := range tests {
		name := tt.name
		args := tt.args
		want := tt.want
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			got := encodeCommand(args)
			if diff := cmp.Diff(want, got); diff != "" {
				t.Errorf("(-want +got):\n%s", diff)
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
			if diff := cmp.Diff(want, string(got)); diff != "" {
				t.Errorf("(-want +got):\n%s", diff)
			}
		})
	}
}

func BenchmarkEncodeCommand(b *testing.B) {
	args := []string{"SET", "key", "value"}
	for i := 0; i < b.N; i++ {
		encodeCommand(args)
	}
}

func BenchmarkRedisProtocolParser_Parse(b *testing.B) {
	input := []byte("*2\r\n$3\r\nfoo\r\n$3\r\nbar\r\n")
	parser := NewRedisProtocolParser(bytes.NewReader(input))
	for i := 0; i < b.N; i++ {
		_, _ = parser.Parse()
	}
}
