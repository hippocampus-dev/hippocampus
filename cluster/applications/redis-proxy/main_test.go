package main

import (
	"bytes"
	"encoding/json"
	"testing"
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
			name:    "bulk string null",
			input:   []byte("$-1\r\n"),
			want:    &RESPBulkString{s: nil},
			wantErr: false,
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
			if got.String() != want.String() {
				t.Errorf("RedisProtocolParser.Parse() = %s, want %s", got.String(), want.String())
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
	parser := NewRedisProtocolParser(bytes.NewReader(input))
	for i := 0; i < b.N; i++ {
		_, _ = parser.Parse()
	}
}
