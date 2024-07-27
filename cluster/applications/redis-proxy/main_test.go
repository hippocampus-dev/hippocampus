package main

import (
	"bytes"
	"testing"
)

func toP(s string) *string {
	return &s
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
			want:    &RESPBulkString{s: toP("foobar")},
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
			want:    &RESPArray{a: []RESPMessage{&RESPBulkString{s: toP("foo")}, &RESPBulkString{s: toP("bar")}}},
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
					t.Errorf("RedisProtocolParser.Parse() error = %v", err)
				}
				return
			}
			if got.String() != want.String() {
				t.Errorf("RedisProtocolParser.Parse() = %v, want %v", got.String(), want.String())
			}
		})
	}
}
