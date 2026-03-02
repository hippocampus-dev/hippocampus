package main

import (
	"context"
	"errors"
	"flag"
	"io"
	"log"
	"math"
	"math/rand"
	"net"
	"net/http"
	"os"
	"os/signal"
	"runtime/debug"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/otel/attribute"
	otelprometheus "go.opentelemetry.io/otel/exporters/prometheus"
	"go.opentelemetry.io/otel/metric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"golang.org/x/xerrors"
)

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

func (c *Connection) LocalAddr() net.Addr {
	return c.conn.LocalAddr()
}

func (c *Connection) RemoteAddr() net.Addr {
	return c.conn.RemoteAddr()
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

type Topology struct {
	Name string
	CIDR net.IPNet
}

type ConnectionPoolOption struct {
	MaxConnections         uint
	MaxIdleConnections     uint
	MinIdleConnections     uint
	MaxIdleTime            time.Duration
	MaxLifetime            time.Duration
	Jitter                 func(time.Duration) time.Duration
	TopologyList           []Topology
	Dialer                 func(context.Context) (net.Conn, error)
	ConnectionPoolStrategy ConnectionPoolStrategy
}

type ConnectionPool struct {
	option *ConnectionPoolOption

	connectionMutex      sync.Mutex
	connections          []*Connection
	idleConnections      map[string][]*Connection
	idleConnectionsCount int64
}

func NewConnectionPool(option *ConnectionPoolOption) *ConnectionPool {
	p := &ConnectionPool{
		option:               option,
		connections:          make([]*Connection, 0),
		idleConnections:      make(map[string][]*Connection),
		idleConnectionsCount: 0,
	}

	go p.prepareIdleConnection(context.Background())

	return p
}

func (p *ConnectionPool) IdleConnections() int {
	return int(atomic.LoadInt64(&p.idleConnectionsCount))
}

func (p *ConnectionPool) Connections() int {
	return len(p.connections)
}

func (p *ConnectionPool) prepareIdleConnection(ctx context.Context) {
	required := int(p.option.MinIdleConnections) - p.IdleConnections()
	for i := 0; i < required; i++ {
		_, _ = p.newIdleConnection(ctx)
	}
}

func (p *ConnectionPool) newIdleConnection(ctx context.Context) (*Connection, error) {
	conn, err := p.option.Dialer(ctx)
	if err != nil {
		return nil, xerrors.Errorf("failed to dial: %w", err)
	}

	connection := NewConnection(conn)

	p.connectionMutex.Lock()
	defer p.connectionMutex.Unlock()
	if p.option.MaxIdleConnections > 0 && p.IdleConnections() >= int(p.option.MaxIdleConnections) {
		_ = connection.Close()
		return nil, xerrors.New("idle connection pool is full")
	}
	topologyName := p.topologyName(connection.RemoteAddr())

	p.connections = append(p.connections, connection)
	p.idleConnections[topologyName] = append(p.idleConnections[topologyName], connection)
	atomic.AddInt64(&p.idleConnectionsCount, 1)

	return connection, nil
}

func (p *ConnectionPool) newConnection(ctx context.Context) (*Connection, error) {
	if p.option.MaxConnections > 0 && p.Connections() >= int(p.option.MaxConnections) {
		return nil, xerrors.New("connection pool is full")
	}

	conn, err := p.option.Dialer(ctx)
	if err != nil {
		return nil, xerrors.Errorf("failed to dial: %w", err)
	}

	connection := NewConnection(conn)

	p.connections = append(p.connections, connection)

	return connection, nil
}

func (p *ConnectionPool) Get(ctx context.Context, topologyName string) (*Connection, error) {
	p.connectionMutex.Lock()
	defer p.connectionMutex.Unlock()

	defer func() {
		go p.prepareIdleConnection(ctx)
	}()

	for {
		connection, err := p.getIdleConnection(ctx, topologyName)
		if err != nil {
			return nil, xerrors.Errorf("failed to get idle connection: %w", err)
		}

		if connection == nil {
			if n := p.pickRandomIdleTopologyName(); n != nil {
				topologyName = *n
				continue
			}
			break
		}

		now := nowFunc()
		if (p.option.MaxIdleTime > 0 && now.Sub(connection.returnedAt) > p.option.Jitter(p.option.MaxIdleTime)) ||
			(p.option.MaxLifetime > 0 && now.Sub(connection.createdAt) > p.option.Jitter(p.option.MaxLifetime)) ||
			!connection.isHealthy() {
			_ = connection.Close()
			p.removeConnection(ctx, topologyName, connection)
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

func (p *ConnectionPool) getIdleConnection(ctx context.Context, topologyName string) (*Connection, error) {
	if len(p.idleConnections[topologyName]) == 0 {
		return nil, nil
	}

	switch p.option.ConnectionPoolStrategy {
	case FIFO:
		connection := p.idleConnections[topologyName][0]
		p.idleConnections[topologyName] = p.idleConnections[topologyName][1:]
		atomic.AddInt64(&p.idleConnectionsCount, -1)

		return connection, nil
	case LIFO:
		size := len(p.idleConnections[topologyName])
		connection := p.idleConnections[topologyName][size-1]
		p.idleConnections[topologyName] = p.idleConnections[topologyName][:size-1]
		atomic.AddInt64(&p.idleConnectionsCount, -1)

		return connection, nil
	default:
		return nil, xerrors.New("invalid connection pool strategy")
	}
}

func (p *ConnectionPool) Put(ctx context.Context, connection *Connection) error {
	p.connectionMutex.Lock()
	defer p.connectionMutex.Unlock()

	topologyName := p.topologyName(connection.RemoteAddr())

	if p.IdleConnections() < int(p.option.MaxIdleConnections) {
		connection.returnedAt = nowFunc()
		p.idleConnections[topologyName] = append(p.idleConnections[topologyName], connection)
		atomic.AddInt64(&p.idleConnectionsCount, 1)
	} else {
		_ = connection.Close()
		p.removeConnection(ctx, topologyName, connection)
	}

	return nil
}

func (p *ConnectionPool) removeConnection(ctx context.Context, topologyName string, connection *Connection) {
	for i, c := range p.connections {
		if c == connection {
			p.connections = append(p.connections[:i], p.connections[i+1:]...)
		}
	}
}

func (p *ConnectionPool) pickRandomIdleTopologyName() *string {
	for topologyName := range p.idleConnections {
		if len(p.idleConnections[topologyName]) > 0 {
			return &topologyName
		}
	}
	return nil
}

func (p *ConnectionPool) topologyName(addr net.Addr) string {
	for _, t := range p.option.TopologyList {
		switch addr := addr.(type) {
		case *net.TCPAddr:
			if t.CIDR.Contains(addr.IP) {
				return t.Name
			}
		case *net.UDPAddr:
			if t.CIDR.Contains(addr.IP) {
				return t.Name
			}
		}
	}
	return ""
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
	var monitorAddress string
	var connectTimeout time.Duration
	var maxConnections int
	var maxIdleConnections int
	var minIdleConnections int
	var maxIdleTime time.Duration
	var maxLifetime time.Duration
	var jitterPercentage float64
	var topologyAwareRouting bool
	var topologies string
	var ownIP string
	var terminationGracePeriod time.Duration
	var lameduck time.Duration
	var keepalive bool
	flag.StringVar(&localAddress, "local-address", envOrDefaultValue("LOCAL_ADDRESS", "127.0.0.1:18888"), "")
	flag.StringVar(&remoteAddress, "remote-address", envOrDefaultValue("REMOTE_ADDRESS", "127.0.0.1:8888"), "")
	flag.StringVar(&monitorAddress, "monitor-address", envOrDefaultValue("MONITOR_ADDRESS", "127.0.0.1:8080"), "")
	flag.DurationVar(&connectTimeout, "connect-timeout", envOrDefaultValue("CONNECT_TIMEOUT", 10*time.Second), "TCP connection timeout. Default is 10s.")
	flag.IntVar(&maxConnections, "max-connections", envOrDefaultValue("MAX_CONNECTIONS", math.MaxInt32), "Maximum number of TCP connections to a remote address. Default 2^32-1.")
	flag.IntVar(&maxIdleConnections, "max-idle-connections", envOrDefaultValue("MAX_IDLE_CONNECTIONS", 0), "Maximum number of idle TCP connections to a remote address. Default 0.")
	flag.IntVar(&minIdleConnections, "min-idle-connections", envOrDefaultValue("MIN_IDLE_CONNECTIONS", 0), "Minimum number of idle TCP connections to a remote address. Default 0.")
	flag.DurationVar(&maxIdleTime, "max-idle-time", envOrDefaultValue("MAX_IDLE_TIME", time.Duration(0)), "Maximum idle time of TCP connections to a remote address. Default 0.")
	flag.DurationVar(&maxLifetime, "max-lifetime", envOrDefaultValue("MAX_LIFETIME", time.Duration(0)), "Maximum lifetime of TCP connections to a remote address. Default 0.")
	flag.Float64Var(&jitterPercentage, "jitter-percentage", envOrDefaultValue("JITTER_PERCENTAGE", 0.1), "Jitter percentage for connection pool")
	flag.BoolVar(&topologyAwareRouting, "topology-aware-routing", envOrDefaultValue("TOPOLOGY_AWARE_ROUTING", false), "Topology-aware routing")
	flag.StringVar(&topologies, "topologies", envOrDefaultValue("TOPOLOGIES", ""), "TopologyList in the format of name1=192.168.0.0/24,name2=192.168.1.0/24")
	flag.StringVar(&ownIP, "own-ip", envOrDefaultValue("OWN_IP", ""), "Own IP address for topology-aware routing")

	flag.DurationVar(&terminationGracePeriod, "termination-grace-period", envOrDefaultValue("TERMINATION_GRACE_PERIOD", 10*time.Second), "The duration the application needs to terminate gracefully")
	flag.DurationVar(&lameduck, "lameduck", envOrDefaultValue("LAMEDUCK", 1*time.Second), "A period that explicitly asks clients to stop sending requests, although the backend task is listening on that port and can provide the service")
	flag.BoolVar(&keepalive, "http-keepalive", envOrDefaultValue("HTTP_KEEPALIVE", true), "")
	flag.Parse()

	exporter, err := otelprometheus.New()
	if err != nil {
		log.Fatalf("failed to create exporter: %+v", err)
	}
	meter := sdkmetric.NewMeterProvider(sdkmetric.WithReader(exporter)).Meter("tcp-proxy")
	tcpConnections, err := meter.Int64UpDownCounter("tcp_connections")
	if err != nil {
		log.Fatalf("failed to create gauge: %+v", err)
	}
	tcpIdleConnections, err := meter.Int64ObservableGauge("tcp_idle_connections")
	if err != nil {
		log.Fatalf("failed to create gauge: %+v", err)
	}

	opt := metric.WithAttributes(
		attribute.Key("upstream").String(remoteAddress),
	)

	mux := http.NewServeMux()
	mux.Handle("GET /metrics", promhttp.Handler())

	monitorListener, err := net.Listen("tcp", monitorAddress)
	if err != nil {
		log.Fatalf("failed to listen: %+v", err)
	}

	monitorServer := &http.Server{
		Handler: mux,
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

	var topologyList []Topology
	var topologyName string
	if topologyAwareRouting {
		for _, t := range strings.Split(topologies, ",") {
			if t == "" {
				continue
			}

			parts := strings.Split(t, "=")
			if len(parts) != 2 {
				log.Fatalf("invalid topologies: %s", topologies)
			}
			_, cidr, err := net.ParseCIDR(parts[1])
			if err != nil {
				log.Fatalf("failed to parse CIDR %s: %+v", parts[1], err)
			}
			topology := Topology{
				Name: parts[0],
				CIDR: *cidr,
			}
			topologyList = append(topologyList, topology)

			if topology.CIDR.Contains(net.ParseIP(ownIP)) {
				topologyName = topology.Name
			}
		}
	}

	connectionPool := NewConnectionPool(&ConnectionPoolOption{
		MaxConnections:     uint(maxConnections),
		MaxIdleConnections: uint(maxIdleConnections),
		MinIdleConnections: uint(minIdleConnections),
		MaxIdleTime:        maxIdleTime,
		MaxLifetime:        maxLifetime,
		Jitter: func(duration time.Duration) time.Duration {
			jitter := time.Duration(float64(duration) * jitterPercentage * (rand.Float64()*2 - 1))
			return duration + jitter
		},
		TopologyList: topologyList,
		Dialer: func(ctx context.Context) (net.Conn, error) {
			return net.DialTimeout("tcp", remoteAddress, connectTimeout)
		},
		ConnectionPoolStrategy: FIFO,
	})

	if _, err := meter.RegisterCallback(func(_ context.Context, o metric.Observer) error {
		o.ObserveInt64(tcpIdleConnections, int64(connectionPool.IdleConnections()), opt)
		return nil
	}, tcpIdleConnections); err != nil {
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
			tcpConnections.Add(ctx, 1, opt)
			go func() {
				defer func() {
					<-semaphore
					wg.Done()
					tcpConnections.Add(ctx, -1, opt)
				}()

				defer local.Close()

				remote, err := connectionPool.Get(ctx, topologyName)
				if err != nil {
					return
				}
				defer connectionPool.Put(ctx, remote)
				defer remote.Cancel()

				c := make(chan struct{}, 2)

				f := func(c chan struct{}, dst io.Writer, src io.Reader) {
					_, _ = io.Copy(dst, src)
					c <- struct{}{}
				}
				go f(c, remote, local)
				go f(c, local, remote)

				select {
				case <-c:
				case <-shutdown:
					local.CloseWrite()
				}
			}()
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGTERM)
	<-quit
	time.Sleep(lameduck)

	close(shutdown)
	listener.Close()

	ctx, cancel := context.WithTimeout(context.Background(), terminationGracePeriod)
	defer cancel()

	if err := monitorServer.Shutdown(ctx); err != nil {
		log.Fatalf("failed to shutdown: %+v", err)
	}

	wg.Wait()
}
