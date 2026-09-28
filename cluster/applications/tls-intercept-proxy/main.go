package main

import (
	"container/list"
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"errors"
	"flag"
	"log"
	"math/big"
	"net"
	"net/http"
	"net/http/httputil"
	"os"
	"os/signal"
	"runtime/debug"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"golang.org/x/xerrors"
)

const (
	HeaderOriginalHost   = "X-Original-Host"
	HeaderOriginalScheme = "X-Original-Scheme"
)

const MaxIssuedCertificates = 10000

type issuedCertificate struct {
	certificate *tls.Certificate
	element     *list.Element
}

type CertificateAuthority struct {
	certificateFile string
	privateKeyFile  string
	lifetime        time.Duration

	mutex       sync.Mutex
	modifiedAt  time.Time
	certificate *x509.Certificate
	privateKey  crypto.Signer
	issued      map[string]*issuedCertificate
	recency     *list.List
}

func NewCertificateAuthority(certificateFile string, privateKeyFile string, lifetime time.Duration) (*CertificateAuthority, error) {
	authority := &CertificateAuthority{
		certificateFile: certificateFile,
		privateKeyFile:  privateKeyFile,
		lifetime:        lifetime,
		issued:          map[string]*issuedCertificate{},
		recency:         list.New(),
	}

	authority.mutex.Lock()
	defer authority.mutex.Unlock()

	if err := authority.reloadLocked(); err != nil {
		return nil, err
	}

	return authority, nil
}

func (c *CertificateAuthority) reloadLocked() error {
	information, err := os.Stat(c.certificateFile)
	if err != nil {
		return xerrors.Errorf("failed to stat certificate authority certificate: %w", err)
	}
	if !information.ModTime().After(c.modifiedAt) {
		return nil
	}

	keyPair, err := tls.LoadX509KeyPair(c.certificateFile, c.privateKeyFile)
	if err != nil {
		return xerrors.Errorf("failed to load certificate authority key pair: %w", err)
	}

	certificate, err := x509.ParseCertificate(keyPair.Certificate[0])
	if err != nil {
		return xerrors.Errorf("failed to parse certificate authority certificate: %w", err)
	}

	privateKey, ok := keyPair.PrivateKey.(crypto.Signer)
	if !ok {
		return xerrors.New("certificate authority private key does not implement crypto.Signer")
	}

	c.modifiedAt = information.ModTime()
	c.certificate = certificate
	c.privateKey = privateKey
	c.recency.Init()
	clear(c.issued)

	return nil
}

func (c *CertificateAuthority) Issue(host string) (*tls.Certificate, error) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	if err := c.reloadLocked(); err != nil {
		return nil, err
	}

	if issued, ok := c.issued[host]; ok {
		if time.Now().Before(issued.certificate.Leaf.NotAfter) {
			c.recency.MoveToFront(issued.element)
			return issued.certificate, nil
		}
		c.recency.Remove(issued.element)
		delete(c.issued, host)
	}

	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, xerrors.Errorf("failed to generate private key: %w", err)
	}

	// https://github.com/golang/go/blob/go1.27.0/src/crypto/tls/generate_cert.go#L112
	serialNumber, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return nil, xerrors.Errorf("failed to generate serial number: %w", err)
	}

	notAfter := time.Now().Add(c.lifetime)
	if notAfter.After(c.certificate.NotAfter) {
		notAfter = c.certificate.NotAfter
	}

	template := &x509.Certificate{
		SerialNumber:          serialNumber,
		Subject:               pkix.Name{CommonName: host},
		NotBefore:             time.Now(),
		NotAfter:              notAfter,
		KeyUsage:              x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}
	if address := net.ParseIP(host); address != nil {
		template.IPAddresses = []net.IP{address}
	} else {
		template.DNSNames = []string{host}
	}

	derBytes, err := x509.CreateCertificate(rand.Reader, template, c.certificate, privateKey.Public(), c.privateKey)
	if err != nil {
		return nil, xerrors.Errorf("failed to create certificate: %w", err)
	}

	hostCertificate, err := x509.ParseCertificate(derBytes)
	if err != nil {
		return nil, xerrors.Errorf("failed to parse created certificate: %w", err)
	}

	certificate := &tls.Certificate{
		Certificate: [][]byte{derBytes, c.certificate.Raw},
		PrivateKey:  privateKey,
		Leaf:        hostCertificate,
	}

	// A browsing agent reaches a new host on nearly every page, so the cache has to have a ceiling
	if c.recency.Len() >= MaxIssuedCertificates {
		oldest := c.recency.Back()
		c.recency.Remove(oldest)
		delete(c.issued, oldest.Value.(string))
	}
	c.issued[host] = &issuedCertificate{certificate: certificate, element: c.recency.PushFront(host)}

	return certificate, nil
}

type SingleConnectionListener struct {
	connection net.Conn
	accepted   bool
}

func (l *SingleConnectionListener) Accept() (net.Conn, error) {
	if l.accepted {
		return nil, net.ErrClosed
	}
	l.accepted = true
	return l.connection, nil
}

func (l *SingleConnectionListener) Close() error {
	// http.Server.Serve closes the listener as soon as Accept fails, while the accepted connection is still being served
	// https://github.com/golang/go/blob/go1.27.0/src/net/http/server.go#L3528
	return nil
}

func (l *SingleConnectionListener) Addr() net.Addr {
	return l.connection.LocalAddr()
}

func canonicalHost(authority string, scheme string) string {
	host, port, err := net.SplitHostPort(authority)
	if err != nil {
		return authority
	}
	if (scheme == "https" && port != "443") || (scheme == "http" && port != "80") {
		return authority
	}
	if strings.Contains(host, ":") {
		return "[" + host + "]"
	}
	return host
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

func serve(server *http.Server, address string) *http.Server {
	listener, err := net.Listen("tcp", address)
	if err != nil {
		log.Fatalf("failed to listen: %+v", err)
	}

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

	return server
}

func main() {
	var localForwardProxyAddress string
	var localReverseProxyAddress string
	var remoteAddress string
	var remoteHost string
	var certificateAuthorityFile string
	var certificateAuthorityPrivateKeyFile string
	var certificateLifetime time.Duration
	var connectTimeout time.Duration
	var responseHeaderTimeout time.Duration
	var idleConnectionTimeout time.Duration
	var maxIdleConnectionsPerHost int
	var terminationGracePeriod time.Duration
	var lameduck time.Duration
	var keepAlive bool
	flag.StringVar(&localForwardProxyAddress, "local-forward-proxy-address", envOrDefaultValue("LOCAL_FORWARD_PROXY_ADDRESS", "0.0.0.0:3128"), "Local listen address the client sends CONNECT to")
	flag.StringVar(&localReverseProxyAddress, "local-reverse-proxy-address", envOrDefaultValue("LOCAL_REVERSE_PROXY_ADDRESS", "0.0.0.0:8080"), "Local listen address decrypted requests come back to on their way to the original host")
	flag.StringVar(&remoteAddress, "remote-address", envOrDefaultValue("REMOTE_ADDRESS", ""), "Remote address decrypted requests are forwarded to")
	flag.StringVar(&remoteHost, "remote-host", envOrDefaultValue("REMOTE_HOST", ""), "Host header sent to the remote address")
	flag.StringVar(&certificateAuthorityFile, "certificate-authority-file", envOrDefaultValue("CERTIFICATE_AUTHORITY_FILE", ""), "PEM encoded certificate authority certificate")
	flag.StringVar(&certificateAuthorityPrivateKeyFile, "certificate-authority-private-key-file", envOrDefaultValue("CERTIFICATE_AUTHORITY_PRIVATE_KEY_FILE", ""), "PEM encoded certificate authority private key")
	flag.DurationVar(&certificateLifetime, "certificate-lifetime", envOrDefaultValue("CERTIFICATE_LIFETIME", 24*time.Hour), "Lifetime of the certificates issued for intercepted hosts")
	flag.DurationVar(&connectTimeout, "connect-timeout", envOrDefaultValue("CONNECT_TIMEOUT", 10*time.Second), "TCP connection timeout")
	flag.DurationVar(&responseHeaderTimeout, "response-header-timeout", envOrDefaultValue("RESPONSE_HEADER_TIMEOUT", 30*time.Second), "Response header timeout")
	flag.DurationVar(&idleConnectionTimeout, "idle-connection-timeout", envOrDefaultValue("IDLE_CONNECTION_TIMEOUT", 90*time.Second), "Idle connection timeout")
	flag.IntVar(&maxIdleConnectionsPerHost, "max-idle-connections-per-host", envOrDefaultValue("MAX_IDLE_CONNECTIONS_PER_HOST", 32), "Maximum idle connections per host")
	flag.DurationVar(&terminationGracePeriod, "termination-grace-period", envOrDefaultValue("TERMINATION_GRACE_PERIOD", 10*time.Second), "The duration the application needs to terminate gracefully")
	flag.DurationVar(&lameduck, "lameduck", envOrDefaultValue("LAMEDUCK", 1*time.Second), "A period that explicitly asks clients to stop sending requests, although the backend task is listening on that port and can provide the service")
	flag.BoolVar(&keepAlive, "http-keepalive", envOrDefaultValue("HTTP_KEEPALIVE", true), "Enable HTTP keep-alive")
	flag.Parse()

	if remoteAddress == "" {
		log.Fatal("--remote-address or REMOTE_ADDRESS is required")
	}
	if remoteHost == "" {
		log.Fatal("--remote-host or REMOTE_HOST is required")
	}
	if certificateAuthorityFile == "" {
		log.Fatal("--certificate-authority-file or CERTIFICATE_AUTHORITY_FILE is required")
	}
	if certificateAuthorityPrivateKeyFile == "" {
		log.Fatal("--certificate-authority-private-key-file or CERTIFICATE_AUTHORITY_PRIVATE_KEY_FILE is required")
	}

	authority, err := NewCertificateAuthority(certificateAuthorityFile, certificateAuthorityPrivateKeyFile, certificateLifetime)
	if err != nil {
		log.Fatalf("failed to load certificate authority: %+v", err)
	}

	transport := &http.Transport{
		DialContext:           (&net.Dialer{Timeout: connectTimeout}).DialContext,
		TLSHandshakeTimeout:   connectTimeout,
		ResponseHeaderTimeout: responseHeaderTimeout,
		IdleConnTimeout:       idleConnectionTimeout,
		MaxIdleConnsPerHost:   maxIdleConnectionsPerHost,
	}

	remoteProxy := &httputil.ReverseProxy{
		Rewrite: func(request *httputil.ProxyRequest) {
			request.Out.URL.Scheme = "http"
			request.Out.URL.Host = remoteAddress
			request.Out.Host = remoteHost
		},
		Transport: transport,
	}

	interceptToRewrite := func(scheme string, host string) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			r.Header.Set(HeaderOriginalScheme, scheme)
			r.Header.Set(HeaderOriginalHost, host)
			remoteProxy.ServeHTTP(w, r)
		})
	}

	forwardProxyServer := &http.Server{
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodConnect {
				host := r.Host
				if name, _, err := net.SplitHostPort(host); err == nil {
					host = name
				}

				hijacker, ok := w.(http.Hijacker)
				if !ok {
					http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
					return
				}

				connection, _, err := hijacker.Hijack()
				if err != nil {
					log.Printf("failed to hijack connection: %+v", err)
					return
				}

				if _, err := connection.Write([]byte("HTTP/1.1 200 Connection Established\r\n\r\n")); err != nil {
					_ = connection.Close()
					return
				}

				// Decrypted requests are forwarded over HTTP/1.1, so h2 must not be offered to the client
				tlsConnection := tls.Server(connection, &tls.Config{
					NextProtos: []string{"http/1.1"},
					GetCertificate: func(*tls.ClientHelloInfo) (*tls.Certificate, error) {
						return authority.Issue(host)
					},
				})

				tunnel := &http.Server{
					Handler: interceptToRewrite("https", canonicalHost(r.Host, "https")),
				}
				_ = tunnel.Serve(&SingleConnectionListener{connection: tlsConnection})
				return
			}
			interceptToRewrite("http", canonicalHost(r.Host, "http")).ServeHTTP(w, r)
		}),
	}
	forwardProxyServer.SetKeepAlivesEnabled(keepAlive)

	reverseProxyServer := &http.Server{
		Handler: &httputil.ReverseProxy{
			Rewrite: func(request *httputil.ProxyRequest) {
				request.Out.URL.Scheme = request.In.Header.Get(HeaderOriginalScheme)
				request.Out.URL.Host = request.In.Header.Get(HeaderOriginalHost)
				request.Out.Host = request.Out.URL.Host
				request.Out.Header.Del(HeaderOriginalScheme)
				request.Out.Header.Del(HeaderOriginalHost)
			},
			Transport: transport,
		},
	}
	reverseProxyServer.SetKeepAlivesEnabled(keepAlive)

	servers := []*http.Server{
		serve(forwardProxyServer, localForwardProxyAddress),
		serve(reverseProxyServer, localReverseProxyAddress),
	}

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGTERM)
	<-quit
	time.Sleep(lameduck)

	ctx, cancel := context.WithTimeout(context.Background(), terminationGracePeriod)
	defer cancel()

	for _, server := range servers {
		if err := server.Shutdown(ctx); err != nil {
			log.Fatalf("failed to shutdown: %+v", err)
		}
	}
}
