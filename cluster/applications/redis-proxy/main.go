package main

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"flag"
	"io"
	"log"
	"math"
	"net"
	"net/http"
	"os"
	"os/signal"
	"runtime/debug"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/prometheus"
	api "go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/sdk/metric"
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

type RESPMessage interface {
	String() string
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
		return nil, errors.New("invalid RESP message type")
	}
}

var nowFunc = time.Now

type Connection struct {
	conn       net.Conn
	createdAt  time.Time
	returnedAt time.Time
}

func NewConnection(conn net.Conn) *Connection {
	return &Connection{
		conn:       conn,
		createdAt:  nowFunc(),
		returnedAt: nowFunc(),
	}
}

func (c *Connection) Read(b []byte) (int, error) {
	return c.conn.Read(b)
}

func (c *Connection) Write(b []byte) (int, error) {
	return c.conn.Write(b)
}

func (c *Connection) Close() error {
	return c.conn.Close()
}

func (c *Connection) Cancel() error {
	return c.conn.SetReadDeadline(nowFunc())
}

func (c *Connection) isHealthy() bool {
	// A zero time value disables the deadline.
	_ = c.conn.SetReadDeadline(time.Time{})

	syscallConnection, ok := c.conn.(syscall.Conn)
	if !ok {
		return false
	}

	rawConnection, err := syscallConnection.SyscallConn()
	if err != nil {
		return false
	}

	healthy := false

	if err := rawConnection.Read(func(fd uintptr) bool {
		b := make([]byte, 1)
		// If read succeeds, it's either EOF or unexpected read; only EAGAIN/EWOULDBLOCK is considered healthy.
		_, err := syscall.Read(int(fd), b)
		if errors.Is(err, syscall.EAGAIN) || errors.Is(err, syscall.EWOULDBLOCK) {
			healthy = true
		}

		return true
	}); err != nil {
		return false
	}

	return healthy
}

type ConnectionPoolStrategy int

const (
	FIFO ConnectionPoolStrategy = iota
	LIFO
)

type ConnectionPoolOption struct {
	MaxConnections         uint
	MaxIdleConnections     uint
	MinIdleConnections     uint
	MaxIdleTime            time.Duration
	MaxLifetime            time.Duration
	Dialer                 func(context.Context) (net.Conn, error)
	ConnectionPoolStrategy ConnectionPoolStrategy
}

type ConnectionPool struct {
	option *ConnectionPoolOption

	connectionMutex sync.Mutex
	connections     []*Connection
	idleConnections []*Connection
}

func NewConnectionPool(option *ConnectionPoolOption) *ConnectionPool {
	p := &ConnectionPool{
		option:          option,
		connections:     make([]*Connection, 0),
		idleConnections: make([]*Connection, 0, option.MaxIdleConnections),
	}

	go p.prepareIdleConnection(context.Background())

	return p
}

func (p *ConnectionPool) IdleConnections() int {
	return len(p.idleConnections)
}

func (p *ConnectionPool) prepareIdleConnection(ctx context.Context) {
	p.connectionMutex.Lock()
	required := int(p.option.MinIdleConnections) - len(p.idleConnections)
	p.connectionMutex.Unlock() // unlock before creating new connections

	for i := 0; i < required; i++ {
		_, _ = p.newIdleConnection(ctx)
	}
}

func (p *ConnectionPool) newConnection(ctx context.Context) (*Connection, error) {
	if p.option.MaxConnections > 0 && len(p.connections) >= int(p.option.MaxConnections) {
		return nil, errors.New("connection pool is full")
	}

	conn, err := p.option.Dialer(ctx)
	if err != nil {
		return nil, xerrors.Errorf("failed to dial: %w", err)
	}

	connection := NewConnection(conn)

	p.connections = append(p.connections, connection)

	return connection, nil
}

func (p *ConnectionPool) newIdleConnection(ctx context.Context) (*Connection, error) {
	if p.option.MaxIdleConnections > 0 && len(p.idleConnections) >= int(p.option.MaxIdleConnections) {
		return nil, errors.New("idle connection pool is full")
	}

	conn, err := p.option.Dialer(ctx)
	if err != nil {
		return nil, xerrors.Errorf("failed to dial: %w", err)
	}

	connection := NewConnection(conn)

	p.connections = append(p.connections, connection)
	p.idleConnections = append(p.idleConnections, connection)

	return connection, nil
}

func (p *ConnectionPool) Get(ctx context.Context) (*Connection, error) {
	p.connectionMutex.Lock()
	defer p.connectionMutex.Unlock()

	for {
		connection, err := p.getIdleConnection(ctx)
		if err != nil {
			return nil, xerrors.Errorf("failed to get idle connection: %w", err)
		}

		if connection == nil {
			break
		}

		now := nowFunc()
		if !connection.isHealthy() ||
			(p.option.MaxIdleTime > 0 && now.Sub(connection.returnedAt) > p.option.MaxIdleTime) ||
			(p.option.MaxLifetime > 0 && now.Sub(connection.createdAt) > p.option.MaxLifetime) {
			_ = connection.Close()
			p.removeConnection(ctx, connection)
			continue
		}

		return connection, nil
	}

	connection, err := p.newConnection(ctx)
	if err != nil {
		return nil, xerrors.Errorf("failed to create connection: %w", err)
	}

	return connection, nil
}

func (p *ConnectionPool) getIdleConnection(ctx context.Context) (*Connection, error) {
	if len(p.idleConnections) == 0 {
		return nil, nil
	}

	switch p.option.ConnectionPoolStrategy {
	case FIFO:
		connection := p.idleConnections[0]
		p.idleConnections = p.idleConnections[1:]

		go p.prepareIdleConnection(ctx)

		return connection, nil
	case LIFO:
		connection := p.idleConnections[len(p.idleConnections)-1]
		p.idleConnections = p.idleConnections[:len(p.idleConnections)-1]

		go p.prepareIdleConnection(ctx)

		return connection, nil
	default:
		return nil, errors.New("invalid connection pool strategy")
	}
}

func (p *ConnectionPool) Put(ctx context.Context, connection *Connection) error {
	p.connectionMutex.Lock()
	defer p.connectionMutex.Unlock()

	if len(p.idleConnections) < int(p.option.MaxIdleConnections) {
		connection.returnedAt = nowFunc()
		p.idleConnections = append(p.idleConnections, connection)
	} else {
		_ = connection.Close()
		p.removeConnection(ctx, connection)
	}

	return nil
}

func (p *ConnectionPool) removeConnection(ctx context.Context, connection *Connection) {
	for i, c := range p.connections {
		if c == connection {
			p.connections = append(p.connections[:i], p.connections[i+1:]...)
			go p.prepareIdleConnection(ctx)
		}
	}
}

func main() {
	var localAddress string
	var remoteAddress string
	var monitorAddress string
	var readerRouting bool
	var readerRemoteAddress string
	var connectTimeout int
	var maxConnections int
	var maxIdleConnections int
	var minIdleConnections int
	var maxIdleTimeSeconds int
	var maxLifetimeSeconds int
	var terminationGracePeriodSeconds int
	var lameduck int
	var keepalive bool
	flag.StringVar(&localAddress, "local-address", "127.0.0.1:16379", "")
	flag.StringVar(&remoteAddress, "remote-address", "127.0.0.1:6379", "")
	flag.StringVar(&monitorAddress, "monitor-address", "127.0.0.1:8080", "")
	flag.BoolVar(&readerRouting, "reader-routing", false, "Route read commands to --reader-remote-address.")
	flag.StringVar(&readerRemoteAddress, "reader-remote-address", remoteAddress, "")
	flag.IntVar(&connectTimeout, "connect-timeout", 10000, "TCP connection timeout milliseconds. Default is 10s.")
	flag.IntVar(&maxConnections, "max-connections", math.MaxInt32, "Maximum number of TCP connections to a remote address. Default 2^32-1.")
	flag.IntVar(&maxIdleConnections, "max-idle-connections", 0, "Maximum number of idle TCP connections to a remote address. Default 0.")
	flag.IntVar(&minIdleConnections, "min-idle-connections", 0, "Minimum number of idle TCP connections to a remote address. Default 0.")
	flag.IntVar(&maxIdleTimeSeconds, "max-idle-time-seconds", 0, "Maximum idle time of TCP connections to a remote address. Default 0.")
	flag.IntVar(&maxLifetimeSeconds, "max-lifetime-seconds", 0, "Maximum lifetime of TCP connections to a remote address. Default 0.")

	flag.IntVar(&terminationGracePeriodSeconds, "termination-grace-period-seconds", 10, "The duration in seconds the application needs to terminate gracefully")
	flag.IntVar(&lameduck, "lameduck", 1, "A period that explicitly asks clients to stop sending requests, although the backend task is listening on that port and can provide the service")
	flag.BoolVar(&keepalive, "http-keepalive", true, "")
	flag.Parse()

	exporter, err := prometheus.New()
	if err != nil {
		log.Fatalf("failed to create exporter: %+v", err)
	}
	meter := metric.NewMeterProvider(metric.WithReader(exporter)).Meter("redis-proxy")
	counter, err := meter.Int64UpDownCounter("tcp_connections")
	if err != nil {
		log.Fatalf("failed to create counter: %+v", err)
	}
	gauge, err := meter.Int64ObservableGauge("tcp_idle_connections")
	if err != nil {
		log.Fatalf("failed to create gauge: %+v", err)
	}

	opt := api.WithAttributes(
		attribute.Key("upstream").String(remoteAddress),
	)
	readerOpt := api.WithAttributes(
		attribute.Key("upstream").String(readerRemoteAddress),
	)

	router := http.NewServeMux()
	router.Handle("/metrics", promhttp.Handler())

	monitorListener, err := net.Listen("tcp", monitorAddress)
	if err != nil {
		log.Fatalf("failed to listen: %+v", err)
	}

	monitorServer := &http.Server{
		Handler: router,
	}
	monitorServer.SetKeepAlivesEnabled(keepalive)

	tcpAddr, err := net.ResolveTCPAddr("tcp", localAddress)
	if err != nil {
		log.Fatalf("failed to resolve IP address: %+v", err)
	}
	listener, err := net.ListenTCP("tcp", tcpAddr)
	if err != nil {
		log.Fatalf("failed to listen: %+v", err)
	}

	connectionPool := NewConnectionPool(&ConnectionPoolOption{
		MaxConnections:     uint(maxConnections),
		MaxIdleConnections: uint(maxIdleConnections),
		MinIdleConnections: uint(minIdleConnections),
		MaxIdleTime:        time.Duration(maxIdleTimeSeconds) * time.Second,
		MaxLifetime:        time.Duration(maxLifetimeSeconds) * time.Second,
		Dialer: func(ctx context.Context) (net.Conn, error) {
			return net.DialTimeout("tcp", remoteAddress, time.Duration(connectTimeout)*time.Millisecond)
		},
		ConnectionPoolStrategy: FIFO,
	})

	var readerConnectionPool *ConnectionPool
	if readerRouting {
		readerConnectionPool = NewConnectionPool(&ConnectionPoolOption{
			MaxConnections:     uint(maxConnections),
			MaxIdleConnections: uint(maxIdleConnections),
			MinIdleConnections: uint(minIdleConnections),
			MaxIdleTime:        time.Duration(maxIdleTimeSeconds) * time.Second,
			MaxLifetime:        time.Duration(maxLifetimeSeconds) * time.Second,
			Dialer: func(ctx context.Context) (net.Conn, error) {
				return net.DialTimeout("tcp", readerRemoteAddress, time.Duration(connectTimeout)*time.Millisecond)
			},
			ConnectionPoolStrategy: FIFO,
		})
	}

	if _, err := meter.RegisterCallback(func(_ context.Context, o api.Observer) error {
		o.ObserveInt64(gauge, int64(connectionPool.IdleConnections()), opt)
		if readerRouting {
			o.ObserveInt64(gauge, int64(readerConnectionPool.IdleConnections()), readerOpt)
		}
		return nil
	}, gauge); err != nil {
		log.Fatalf("failed to register callback: %+v", err)
	}

	semaphore := make(chan struct{}, maxConnections)
	shutdown := make(chan struct{}, 1)
	wg := sync.WaitGroup{}

	go func() {
		defer func() {
			if err := recover(); err != nil {
				log.Printf("panic: %+v\n%s", err, debug.Stack())
			}
		}()

		if err := monitorServer.Serve(monitorListener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("failed to serve: %+v", err)
		}
	}()

	go func() {
		ctx := context.Background()
		for {
			local, err := listener.AcceptTCP()
			if err != nil {
				select {
				case <-shutdown:
					return
				default:
					continue
				}
			}
			semaphore <- struct{}{}
			wg.Add(1)
			go func() {
				defer func() {
					<-semaphore
					wg.Done()
				}()

				defer local.Close()

				counter.Add(ctx, 1, opt)
				defer counter.Add(ctx, -1, opt)

				remote, err := connectionPool.Get(ctx)
				if err != nil {
					return
				}
				defer connectionPool.Put(ctx, remote)
				defer remote.Cancel()

				f := func(c chan struct{}, dst io.Writer, src io.Reader) {
					_, _ = io.Copy(dst, src)
					c <- struct{}{}
				}
				if readerRouting {
					counter.Add(ctx, 1, readerOpt)
					defer counter.Add(ctx, -1, readerOpt)

					readerRemote, err := readerConnectionPool.Get(ctx)
					if err != nil {
						return
					}
					defer readerConnectionPool.Put(ctx, readerRemote)
					defer readerRemote.Cancel()

					c := make(chan struct{}, 3)

					routing := func(c chan struct{}, writer io.Writer, reader io.Writer, src io.Reader) {
						var buffer bytes.Buffer

						parser := NewRedisProtocolParser(io.TeeReader(src, &buffer))
						for {
							message, err := parser.Parse()
							if err != nil {
								if err == io.EOF {
									break
								}
								return
							}

							if isReadCommandMessage(message) {
								_, _ = io.Copy(reader, bytes.NewReader(buffer.Bytes()))
							} else {
								_, _ = io.Copy(writer, bytes.NewReader(buffer.Bytes()))
							}

							buffer.Reset()
						}

						c <- struct{}{}
					}

					go routing(c, remote, readerRemote, local)
					go f(c, local, remote)
					go f(c, local, readerRemote)

					select {
					case <-c:
					case <-shutdown:
						local.CloseWrite()
					}
				} else {
					c := make(chan struct{}, 2)

					go f(c, remote, local)
					go f(c, local, remote)

					select {
					case <-c:
					case <-shutdown:
						local.CloseWrite()
					}
				}
			}()
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGTERM)
	<-quit
	time.Sleep(time.Duration(lameduck) * time.Second)

	close(shutdown)
	listener.Close()

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(terminationGracePeriodSeconds)*time.Second)
	defer cancel()

	if err := monitorServer.Shutdown(ctx); err != nil {
		log.Fatalf("failed to shutdown: %+v", err)
	}

	wg.Wait()
}

func isReadCommandMessage(message RESPMessage) bool {
	switch message.(type) {
	case *RESPArray:
		a := message.(*RESPArray)
		if a.a == nil || len(a.a) == 0 {
			return false
		}

		command := a.a[0]
		switch command.(type) {
		case *RESPBulkString:
			s := command.(*RESPBulkString)
			if s.s == nil {
				return false
			}

			switch strings.ToUpper(*s.s) {
			case "EXISTS", "EXPIRETIME", "KEYS", "SCAN",
				// https://redis.io/docs/latest/commands/?group=string
				"GET", "GETRANGE", "MGET", "STRLEN",
				// https://redis.io/docs/latest/commands/?group=hash
				"HEXISTS", "HGET", "HGETALL", "HKEYS", "HLEN", "HMGET", "HSCAN", "HSTRLEN", "HVALS",
				// https://redis.io/docs/latest/commands/?group=list
				"LINDEX", "LLEN", "LRANGE",
				// https://redis.io/docs/latest/commands/?group=set
				"SCARD", "SINTER", "SINTERCARD", "SISMEMBER", "SMEMBERS", "SMISMEMBER", "SRANDMEMBER", "SSCAN", "SUNION",
				// https://redis.io/docs/latest/commands/?group=sorted-set
				"ZCARD", "ZCOUNT", "ZDIFF", "ZINTER", "ZINTERCARD", "ZLEXCOUNT", "ZMSCORE", "ZRANDMEMBER", "ZRANGE", "ZRANGEBYLEX", "ZRANGEBYSCORE", "ZRANK", "ZREVRANGE", "ZREVRANGEBYLEX", "ZREVRANGEBYSCORE", "ZREVRANK", "ZSCAN", "ZSCORE", "ZUNION",
				// https://redis.io/docs/latest/commands/?group=search
				"FT.SEARCH":
				return true
			}
		}
	}
	return false
}
