package proxy

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"dedicated-container-ingress-controller/internal/factory"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"runtime/debug"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
	"golang.org/x/sync/singleflight"
	"golang.org/x/xerrors"
	corev1 "k8s.io/api/core/v1"
	ctrl "sigs.k8s.io/controller-runtime"
)

type Args struct {
	Address         string
	CookieSecretKey string
	CookieMaxAge    int
	PodsLimit       int64
	RedisClient     *redis.Client
	Factory         *factory.DedicatedContainerFactory
}

type Server struct {
	server       *http.Server
	redisClient  *redis.Client
	factory      *factory.DedicatedContainerFactory
	transport    *http.Transport
	secretKey    []byte
	cookieMaxAge int
	podsLimit    int64
	group        singleflight.Group
}

func NewServer(a *Args) *Server {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.MaxIdleConnsPerHost = transport.MaxIdleConns

	s := &Server{
		redisClient:  a.RedisClient,
		factory:      a.Factory,
		transport:    transport,
		secretKey:    []byte(a.CookieSecretKey),
		cookieMaxAge: a.CookieMaxAge,
		podsLimit:    a.PodsLimit,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(http.StatusText(http.StatusOK)))
	})
	mux.HandleFunc("/", s.handleProxy)

	s.server = &http.Server{
		Addr:    a.Address,
		Handler: mux,
	}
	return s
}

func (s *Server) Start() error {
	proxyLogger := ctrl.Log.WithName("proxy")

	listener, err := net.Listen("tcp", s.server.Addr)
	if err != nil {
		return xerrors.Errorf("failed to listen: %w", err)
	}

	go func() {
		defer func() {
			if err := recover(); err != nil {
				proxyLogger.Error(fmt.Errorf("panic: %v", err), string(debug.Stack()))
			}
		}()
		if err := s.server.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			proxyLogger.Error(err, "failed to serve")
		}
	}()

	return nil
}

func (s *Server) Shutdown(ctx context.Context) error {
	return s.server.Shutdown(ctx)
}

type sessionData struct {
	Identifier string `json:"identifier"`
	Host       string `json:"host"`
}

func (s *Server) handleProxy(w http.ResponseWriter, r *http.Request) {
	proxyLogger := ctrl.Log.WithName("proxy")

	key := trimHost(r.Host)

	session, err := s.getSession(r, key)
	if err != nil {
		proxyLogger.Error(err, "failed to get session")
	}

	var identifier string
	var host string
	reachable := false
	if session != nil {
		identifier = session.Identifier
		host = session.Host
		reachable = checkReachable(host)
	}

	if session == nil || !reachable {
		count, err := s.podsCount(r.Context())
		if err != nil {
			proxyLogger.Error(err, "failed to count pods")
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
		if count >= s.podsLimit {
			http.Error(w, http.StatusText(http.StatusServiceUnavailable), http.StatusServiceUnavailable)
			return
		}
		if !s.factory.HasEntry(key) {
			http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
			return
		}
		result, err, _ := s.group.Do(key, func() (interface{}, error) {
			ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			defer cancel()
			return s.factory.Create(ctx, key)
		})
		if err != nil {
			proxyLogger.Error(err, "failed to create pod")
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
		pod := result.(*corev1.Pod)
		identifier = fmt.Sprintf("%s/%s", pod.Name, pod.Namespace)
		host = fmt.Sprintf("%s:%d", pod.Status.PodIP, getHTTPPort(pod))
		if err := s.saveSession(w, key, &sessionData{
			Identifier: identifier,
			Host:       host,
		}); err != nil {
			proxyLogger.Error(err, "failed to save session")
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
	}

	proxy := httputil.NewSingleHostReverseProxy(&url.URL{
		Scheme: "http",
		Host:   host,
	})
	proxy.Transport = s.transport
	proxy.ServeHTTP(w, r)

	if err := s.updateTimestamp(context.Background(), identifier); err != nil {
		proxyLogger.Error(err, "failed to update timestamp")
	}
}

func (s *Server) podsCount(ctx context.Context) (int64, error) {
	count, err := s.redisClient.ZCard(ctx, "pods").Result()
	if err != nil {
		return 0, xerrors.Errorf("failed to ZCARD: %w", err)
	}
	return count, nil
}

func (s *Server) updateTimestamp(ctx context.Context, identifier string) error {
	if err := s.redisClient.ZAdd(ctx, "pods", redis.Z{
		Score:  float64(time.Now().Unix()),
		Member: identifier,
	}).Err(); err != nil {
		return xerrors.Errorf("failed to ZADD: %w", err)
	}
	return nil
}

func (s *Server) getSession(r *http.Request, name string) (*sessionData, error) {
	cookie, err := r.Cookie(name)
	if err != nil {
		return nil, nil
	}
	parts := strings.SplitN(cookie.Value, ".", 2)
	if len(parts) != 2 {
		return nil, nil
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return nil, nil
	}
	signature, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, nil
	}
	mac := hmac.New(sha256.New, s.secretKey)
	mac.Write(payload)
	if !hmac.Equal(signature, mac.Sum(nil)) {
		return nil, nil
	}
	var data sessionData
	if err := json.Unmarshal(payload, &data); err != nil {
		return nil, nil
	}
	return &data, nil
}

func (s *Server) saveSession(w http.ResponseWriter, name string, data *sessionData) error {
	payload, err := json.Marshal(data)
	if err != nil {
		return xerrors.Errorf("failed to marshal session: %w", err)
	}
	mac := hmac.New(sha256.New, s.secretKey)
	mac.Write(payload)
	value := base64.RawURLEncoding.EncodeToString(payload) + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	http.SetCookie(w, &http.Cookie{
		Name:     name,
		Value:    value,
		Path:     "/",
		MaxAge:   s.cookieMaxAge,
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
	})
	return nil
}

func getHTTPPort(pod *corev1.Pod) int32 {
	for _, container := range pod.Spec.Containers {
		for _, port := range container.Ports {
			if port.Name == "http" {
				return port.ContainerPort
			}
		}
	}
	return 80
}

func checkReachable(host string) bool {
	conn, err := net.DialTimeout("tcp", host, time.Second)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}

func trimHost(host string) string {
	if idx := strings.IndexByte(host, ':'); idx > 0 {
		host = host[:idx]
	}
	return host
}
