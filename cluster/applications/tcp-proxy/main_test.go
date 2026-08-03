package main

import (
	"context"
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

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

	if pool.IdleConnections() != 1 {
		t.Error("expected 1 idle connection")
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

	if pool.IdleConnections() != 1 {
		t.Error("expected 1 idle connection")
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

	if pool.IdleConnections() != 1 {
		t.Error("expected 1 idle connection")
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
