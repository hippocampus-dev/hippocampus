package proxy

import (
	"context"
	"fmt"
	"math/rand/v2"
	"net"
	"sync"
	"time"
)

const (
	udpBufferSize  = 65535
	udpIdleTimeout = 30 * time.Second
)

type udpSession struct {
	remote      *net.UDPConn
	last        time.Time
	proxyHeader []byte
}

func (m *Manager) startUDP(ctx context.Context, route Route) (net.PacketConn, error) {
	address := fmt.Sprintf("0.0.0.0:%d", route.Port)
	listenConfig := net.ListenConfig{}

	l, err := listenConfig.ListenPacket(ctx, "udp", address)
	if err != nil {
		return nil, err
	}

	go m.serveUDP(ctx, l, route)
	return l, nil
}

func (m *Manager) serveUDP(ctx context.Context, l net.PacketConn, route Route) {
	defer l.Close()

	var mu sync.Mutex
	sessions := make(map[string]*udpSession)

	go func() {
		ticker := time.NewTicker(udpIdleTimeout)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				mu.Lock()
				now := time.Now()
				for key, session := range sessions {
					if now.Sub(session.last) > udpIdleTimeout {
						session.remote.Close()
						delete(sessions, key)
					}
				}
				mu.Unlock()
			}
		}
	}()

	buf := make([]byte, udpBufferSize)
	for {
		n, clientAddr, err := l.ReadFrom(buf)
		if err != nil {
			select {
			case <-ctx.Done():
				return
			default:
				continue
			}
		}

		if len(route.Backends) == 0 {
			continue
		}

		key := clientAddr.String()
		mu.Lock()
		session, ok := sessions[key]
		if !ok {
			backend := route.Backends[rand.IntN(len(route.Backends))]
			remoteAddr, err := net.ResolveUDPAddr("udp", backendAddress(backend))
			if err != nil {
				mu.Unlock()
				continue
			}

			remote, err := net.DialUDP("udp", nil, remoteAddr)
			if err != nil {
				mu.Unlock()
				continue
			}

			var proxyHeader []byte
			if m.proxyProtocolVersion == ProtocolV2 {
				clientUDPAddr := clientAddr.(*net.UDPAddr)
				backendUDPAddr := remote.RemoteAddr().(*net.UDPAddr)
				proxyHeader = buildProxyProtocolV2Header(clientUDPAddr.IP, backendUDPAddr.IP, clientUDPAddr.Port, backendUDPAddr.Port, false)
			}

			session = &udpSession{
				remote:      remote,
				last:        time.Now(),
				proxyHeader: proxyHeader,
			}
			sessions[key] = session

			go func(clientAddr net.Addr, session *udpSession) {
				respBuf := make([]byte, udpBufferSize)
				for {
					session.remote.SetReadDeadline(time.Now().Add(udpIdleTimeout))
					n, err := session.remote.Read(respBuf)
					if err != nil {
						return
					}
					mu.Lock()
					session.last = time.Now()
					mu.Unlock()
					_, _ = l.WriteTo(respBuf[:n], clientAddr)
				}
			}(clientAddr, session)
		}
		session.last = time.Now()
		mu.Unlock()

		if len(session.proxyHeader) > 0 {
			payload := make([]byte, len(session.proxyHeader)+n)
			copy(payload, session.proxyHeader)
			copy(payload[len(session.proxyHeader):], buf[:n])
			_, _ = session.remote.Write(payload)
		} else {
			_, _ = session.remote.Write(buf[:n])
		}
	}
}
