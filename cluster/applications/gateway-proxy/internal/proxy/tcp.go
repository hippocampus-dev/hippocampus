package proxy

import (
	"context"
	"fmt"
	"io"
	"math/rand/v2"
	"net"
	"time"
)

const tcpConnectTimeout = 10 * time.Second

func (m *Manager) startTCP(ctx context.Context, route Route) (net.Listener, error) {
	address := fmt.Sprintf("0.0.0.0:%d", route.Port)
	listenConfig := net.ListenConfig{}

	l, err := listenConfig.Listen(ctx, "tcp", address)
	if err != nil {
		return nil, err
	}

	go m.serveTCP(ctx, l, route)
	return l, nil
}

func (m *Manager) serveTCP(ctx context.Context, l net.Listener, route Route) {
	defer l.Close()

	for {
		conn, err := l.Accept()
		if err != nil {
			select {
			case <-ctx.Done():
				return
			default:
				continue
			}
		}
		go func() {
			defer conn.Close()

			if len(route.Backends) == 0 {
				return
			}

			backend := route.Backends[rand.IntN(len(route.Backends))]
			remote, err := net.DialTimeout("tcp", backendAddress(backend), tcpConnectTimeout)
			if err != nil {
				return
			}
			defer remote.Close()

			if header := m.buildTCPProxyProtocolHeader(conn, remote); header != nil {
				if _, err := remote.Write(header); err != nil {
					return
				}
			}

			c := make(chan struct{}, 2)

			f := func(c chan struct{}, dst io.Writer, src io.Reader) {
				_, _ = io.Copy(dst, src)
				c <- struct{}{}
			}

			go f(c, remote, conn)
			go f(c, conn, remote)

			select {
			case <-c:
			case <-ctx.Done():
			}
		}()
	}
}

func (m *Manager) buildTCPProxyProtocolHeader(client net.Conn, backend net.Conn) []byte {
	clientAddr := client.RemoteAddr().(*net.TCPAddr)
	backendAddr := backend.RemoteAddr().(*net.TCPAddr)

	switch m.proxyProtocolVersion {
	case ProtocolV1:
		srcIsIPv4 := clientAddr.IP.To4() != nil
		dstIsIPv4 := backendAddr.IP.To4() != nil
		if srcIsIPv4 != dstIsIPv4 {
			return nil
		}
		family := "TCP4"
		if !srcIsIPv4 {
			family = "TCP6"
		}
		return []byte(fmt.Sprintf("PROXY %s %s %s %d %d\r\n",
			family,
			clientAddr.IP.String(),
			backendAddr.IP.String(),
			clientAddr.Port,
			backendAddr.Port,
		))
	case ProtocolV2:
		return buildProxyProtocolV2Header(clientAddr.IP, backendAddr.IP, clientAddr.Port, backendAddr.Port, true)
	default:
		return nil
	}
}
