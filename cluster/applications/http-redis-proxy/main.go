package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"runtime/debug"
	"strconv"
	"strings"
	"syscall"
	"time"

	"golang.org/x/xerrors"
)

// https://redis.io/docs/reference/protocol-spec/
const (
	RESPTypeSimpleString = '+'
	RESPTypeError        = '-'
	RESPTypeInteger      = ':'
	RESPTypeBulkString   = '$'
	RESPTypeArray        = '*'
)

var (
	RESPDelimiter = []byte{'\r', '\n'}
)

type RESPSimpleString struct {
	s string
}

func (s *RESPSimpleString) String() string {
	var buffer bytes.Buffer
	buffer.WriteByte(RESPTypeSimpleString)
	buffer.WriteString(s.s)
	buffer.Write(RESPDelimiter)
	return buffer.String()
}

func (s *RESPSimpleString) MarshalJSON() ([]byte, error) {
	return json.Marshal(s.s)
}

type RESPError struct {
	s string
}

func (s *RESPError) String() string {
	var buffer bytes.Buffer
	buffer.WriteByte(RESPTypeError)
	buffer.WriteString(s.s)
	buffer.Write(RESPDelimiter)
	return buffer.String()
}

func (s *RESPError) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Error string `json:"error"`
	}{Error: s.s})
}

type RESPInteger struct {
	i int
}

func (i *RESPInteger) String() string {
	var buffer bytes.Buffer
	buffer.WriteByte(RESPTypeInteger)
	buffer.WriteString(strconv.Itoa(i.i))
	buffer.Write(RESPDelimiter)
	return buffer.String()
}

func (i *RESPInteger) MarshalJSON() ([]byte, error) {
	return json.Marshal(i.i)
}

type RESPBulkString struct {
	s *string
}

func (s *RESPBulkString) String() string {
	var buffer bytes.Buffer
	buffer.WriteByte(RESPTypeBulkString)
	if s.s == nil {
		buffer.WriteString("-1")
		buffer.Write(RESPDelimiter)
		return buffer.String()
	}
	buffer.WriteString(strconv.Itoa(len(*s.s)))
	buffer.Write(RESPDelimiter)
	buffer.WriteString(*s.s)
	buffer.Write(RESPDelimiter)
	return buffer.String()
}

func (s *RESPBulkString) MarshalJSON() ([]byte, error) {
	if s.s == nil {
		return []byte("null"), nil
	}
	return json.Marshal(*s.s)
}

type RESPArray struct {
	a []RESPMessage
}

func (a *RESPArray) String() string {
	var buffer bytes.Buffer
	buffer.WriteByte(RESPTypeArray)
	if a.a == nil {
		buffer.WriteString("-1")
		buffer.Write(RESPDelimiter)
		return buffer.String()
	}
	buffer.WriteString(strconv.Itoa(len(a.a)))
	buffer.Write(RESPDelimiter)
	for _, m := range a.a {
		buffer.WriteString(m.String())
	}
	return buffer.String()
}

func (a *RESPArray) MarshalJSON() ([]byte, error) {
	if a.a == nil {
		return []byte("null"), nil
	}
	return json.Marshal(a.a)
}

type RESPMessage interface {
	String() string
	json.Marshaler
}

type RedisProtocolParser struct {
	r *bufio.Reader
}

func NewRedisProtocolParser(r io.Reader) *RedisProtocolParser {
	return &RedisProtocolParser{r: bufio.NewReader(r)}
}

func (p *RedisProtocolParser) Parse() (RESPMessage, error) {
	line, _, err := p.r.ReadLine()
	if err != nil {
		return nil, err
	}

	switch line[0] {
	case RESPTypeSimpleString:
		s := string(line[1:])
		return &RESPSimpleString{s: s}, nil
	case RESPTypeError:
		s := string(line[1:])
		return &RESPError{s: s}, nil
	case RESPTypeInteger:
		i, err := strconv.Atoi(string(line[1:]))
		if err != nil {
			return nil, err
		}
		return &RESPInteger{i: i}, nil
	case RESPTypeBulkString:
		length, err := strconv.Atoi(string(line[1:]))
		if err != nil {
			return nil, err
		}
		if length == -1 {
			return &RESPBulkString{s: nil}, nil
		}

		b := make([]byte, length+len(RESPDelimiter))
		_, err = io.ReadFull(p.r, b)
		if err != nil {
			return nil, err
		}

		s := string(b[:length])
		return &RESPBulkString{s: &s}, nil
	case RESPTypeArray:
		length, err := strconv.Atoi(string(line[1:]))
		if err != nil {
			return nil, err
		}
		if length == -1 {
			return &RESPArray{a: nil}, nil
		}

		var a []RESPMessage
		for i := 0; i < length; i++ {
			m, err := p.Parse()
			if err != nil {
				return nil, err
			}
			a = append(a, m)
		}

		return &RESPArray{a: a}, nil
	default:
		return nil, xerrors.New("invalid RESP message type")
	}
}

func encodeCommand(args []string) string {
	var buffer bytes.Buffer
	buffer.WriteByte(RESPTypeArray)
	buffer.WriteString(strconv.Itoa(len(args)))
	buffer.Write(RESPDelimiter)
	for _, arg := range args {
		buffer.WriteByte(RESPTypeBulkString)
		buffer.WriteString(strconv.Itoa(len(arg)))
		buffer.Write(RESPDelimiter)
		buffer.WriteString(arg)
		buffer.Write(RESPDelimiter)
	}
	return buffer.String()
}

func executeRedis(remoteAddress string, connectTimeout time.Duration, args []string) (RESPMessage, error) {
	remote, err := net.DialTimeout("tcp", remoteAddress, connectTimeout)
	if err != nil {
		return nil, xerrors.Errorf("failed to connect to redis: %w", err)
	}
	defer remote.Close()

	command := encodeCommand(args)
	if _, err := io.Copy(remote, strings.NewReader(command)); err != nil {
		return nil, xerrors.Errorf("failed to send command to redis: %w", err)
	}

	parser := NewRedisProtocolParser(remote)
	message, err := parser.Parse()
	if err != nil {
		return nil, xerrors.Errorf("failed to parse redis response: %w", err)
	}

	return message, nil
}

func envOrDefaultValue[T any](key string, defaultValue T) T {
	value, exists := os.LookupEnv(key)
	if !exists {
		return defaultValue
	}

	switch any(defaultValue).(type) {
	case string:
		return any(value).(T)
	case int:
		if intValue, err := strconv.Atoi(value); err == nil {
			return any(intValue).(T)
		}
	case int64:
		if intValue, err := strconv.ParseInt(value, 10, 64); err == nil {
			return any(intValue).(T)
		}
	case uint:
		if uintValue, err := strconv.ParseUint(value, 10, 0); err == nil {
			return any(uint(uintValue)).(T)
		}
	case uint64:
		if uintValue, err := strconv.ParseUint(value, 10, 64); err == nil {
			return any(uintValue).(T)
		}
	case float64:
		if floatValue, err := strconv.ParseFloat(value, 64); err == nil {
			return any(floatValue).(T)
		}
	case bool:
		if boolValue, err := strconv.ParseBool(value); err == nil {
			return any(boolValue).(T)
		}
	case time.Duration:
		if durationValue, err := time.ParseDuration(value); err == nil {
			return any(durationValue).(T)
		}
	}

	return defaultValue
}

func main() {
	var localAddress string
	var remoteAddress string
	var connectTimeout time.Duration
	var terminationGracePeriod time.Duration
	var lameduck time.Duration
	var keepAlive bool
	flag.StringVar(&localAddress, "local-address", envOrDefaultValue("LOCAL_ADDRESS", "0.0.0.0:8080"), "")
	flag.StringVar(&remoteAddress, "remote-address", envOrDefaultValue("REMOTE_ADDRESS", "127.0.0.1:6379"), "")
	flag.DurationVar(&connectTimeout, "connect-timeout", envOrDefaultValue("CONNECT_TIMEOUT", 10*time.Second), "TCP connection timeout")
	flag.DurationVar(&terminationGracePeriod, "termination-grace-period", envOrDefaultValue("TERMINATION_GRACE_PERIOD", 10*time.Second), "The duration the application needs to terminate gracefully")
	flag.DurationVar(&lameduck, "lameduck", envOrDefaultValue("LAMEDUCK", 1*time.Second), "A period that explicitly asks clients to stop sending requests, although the backend task is listening on that port and can provide the service")
	flag.BoolVar(&keepAlive, "http-keepalive", envOrDefaultValue("HTTP_KEEPALIVE", true), "")
	flag.Parse()

	mux := http.NewServeMux()

	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(http.StatusText(http.StatusOK)))
	})

	mux.HandleFunc("POST /", func(w http.ResponseWriter, r *http.Request) {
		var args []string
		if err := json.NewDecoder(r.Body).Decode(&args); err != nil {
			http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
			return
		}

		if len(args) == 0 {
			http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
			return
		}

		message, err := executeRedis(remoteAddress, connectTimeout, args)
		if err != nil {
			log.Printf("redis error: %+v", err)
			http.Error(w, http.StatusText(http.StatusBadGateway), http.StatusBadGateway)
			return
		}

		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(message)
	})

	mux.HandleFunc("GET /{key}/{field}", func(w http.ResponseWriter, r *http.Request) {
		key := r.PathValue("key")
		field := r.PathValue("field")

		message, err := executeRedis(remoteAddress, connectTimeout, []string{"HGET", key, field})
		if err != nil {
			log.Printf("redis error: %+v", err)
			http.Error(w, http.StatusText(http.StatusBadGateway), http.StatusBadGateway)
			return
		}

		if v, ok := message.(*RESPBulkString); ok && v.s == nil {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(message)
	})

	// Go routes HEAD to GET handlers automatically
	mux.HandleFunc("GET /{key}", func(w http.ResponseWriter, r *http.Request) {
		key := r.PathValue("key")

		if r.Method == http.MethodHead {
			message, err := executeRedis(remoteAddress, connectTimeout, []string{"EXISTS", key})
			if err != nil {
				log.Printf("redis error: %+v", err)
				http.Error(w, http.StatusText(http.StatusBadGateway), http.StatusBadGateway)
				return
			}
			if v, ok := message.(*RESPInteger); ok && v.i == 0 {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			w.WriteHeader(http.StatusOK)
			return
		}

		message, err := executeRedis(remoteAddress, connectTimeout, []string{"GET", key})
		if err != nil {
			log.Printf("redis error: %+v", err)
			http.Error(w, http.StatusText(http.StatusBadGateway), http.StatusBadGateway)
			return
		}

		if v, ok := message.(*RESPBulkString); ok && v.s == nil {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(message)
	})

	mux.HandleFunc("PUT /{key}/{field}", func(w http.ResponseWriter, r *http.Request) {
		key := r.PathValue("key")
		field := r.PathValue("field")

		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
			return
		}

		value := strings.TrimSpace(string(body))
		if value == "" {
			http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
			return
		}

		message, err := executeRedis(remoteAddress, connectTimeout, []string{"HSET", key, field, value})
		if err != nil {
			log.Printf("redis error: %+v", err)
			http.Error(w, http.StatusText(http.StatusBadGateway), http.StatusBadGateway)
			return
		}

		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(message)
	})

	mux.HandleFunc("PUT /{key}", func(w http.ResponseWriter, r *http.Request) {
		key := r.PathValue("key")

		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
			return
		}

		value := strings.TrimSpace(string(body))
		if value == "" {
			http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
			return
		}

		args := []string{"SET", key, value}

		query := r.URL.Query()
		if ex := query.Get("ex"); ex != "" {
			args = append(args, "EX", ex)
		} else if px := query.Get("px"); px != "" {
			args = append(args, "PX", px)
		}
		if _, ok := query["nx"]; ok {
			args = append(args, "NX")
		} else if _, ok := query["xx"]; ok {
			args = append(args, "XX")
		}

		message, err := executeRedis(remoteAddress, connectTimeout, args)
		if err != nil {
			log.Printf("redis error: %+v", err)
			http.Error(w, http.StatusText(http.StatusBadGateway), http.StatusBadGateway)
			return
		}

		// SET with NX returns nil when key already exists
		if v, ok := message.(*RESPBulkString); ok && v.s == nil {
			w.WriteHeader(http.StatusConflict)
			return
		}

		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(message)
	})

	mux.HandleFunc("DELETE /{key}/{field}", func(w http.ResponseWriter, r *http.Request) {
		key := r.PathValue("key")
		field := r.PathValue("field")

		message, err := executeRedis(remoteAddress, connectTimeout, []string{"HDEL", key, field})
		if err != nil {
			log.Printf("redis error: %+v", err)
			http.Error(w, http.StatusText(http.StatusBadGateway), http.StatusBadGateway)
			return
		}

		if v, ok := message.(*RESPInteger); ok && v.i == 0 {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		w.WriteHeader(http.StatusNoContent)
	})

	mux.HandleFunc("DELETE /{key}", func(w http.ResponseWriter, r *http.Request) {
		key := r.PathValue("key")

		message, err := executeRedis(remoteAddress, connectTimeout, []string{"DEL", key})
		if err != nil {
			log.Printf("redis error: %+v", err)
			http.Error(w, http.StatusText(http.StatusBadGateway), http.StatusBadGateway)
			return
		}

		if v, ok := message.(*RESPInteger); ok && v.i == 0 {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		w.WriteHeader(http.StatusNoContent)
	})

	mux.HandleFunc("PURGE /", func(w http.ResponseWriter, r *http.Request) {
		message, err := executeRedis(remoteAddress, connectTimeout, []string{"FLUSHALL"})
		if err != nil {
			log.Printf("redis error: %+v", err)
			http.Error(w, http.StatusText(http.StatusBadGateway), http.StatusBadGateway)
			return
		}

		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(message)
	})

	listener, err := net.Listen("tcp", localAddress)
	if err != nil {
		log.Fatalf("failed to listen: %+v", err)
	}

	server := &http.Server{
		Handler: mux,
	}
	server.SetKeepAlivesEnabled(keepAlive)

	go func() {
		defer func() {
			if err := recover(); err != nil {
				log.Printf("panic: %+v\n%s", err, debug.Stack())
			}
		}()
		if err := server.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("failed to listen: %+v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGTERM)
	<-quit
	time.Sleep(lameduck)

	ctx, cancel := context.WithTimeout(context.Background(), terminationGracePeriod)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Fatalf("failed to shutdown: %+v", err)
	}
}
