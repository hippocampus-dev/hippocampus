package main

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"flag"
	"hash/fnv"
	"log"
	"math"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"runtime/debug"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

const prefix = "/_oauth"

// oauth2-proxy forwards the authenticated identity here through the ingress gateway's ext-authz filter
const userHeader = "X-Auth-Request-User"

const codeTTL = 1 * time.Minute
const tokenTTL = 5 * time.Minute

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

type grant struct {
	username  string
	expiresAt time.Time
}

type store struct {
	mutex  sync.Mutex
	grants map[string]grant
}

func newStore() *store {
	return &store{grants: make(map[string]grant)}
}

func (s *store) put(key string, value grant) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	now := time.Now()
	for existing, g := range s.grants {
		if now.After(g.expiresAt) {
			delete(s.grants, existing)
		}
	}
	s.grants[key] = value
}

func (s *store) get(key string, consume bool) (grant, bool) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	value, exists := s.grants[key]
	if !exists {
		return grant{}, false
	}
	if consume {
		delete(s.grants, key)
	}
	if time.Now().After(value.expiresAt) {
		return grant{}, false
	}

	return value, true
}

func newSecret() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}

	return base64.RawURLEncoding.EncodeToString(b), nil
}

// Mattermost rejects a GitLab user whose id is 0, and stores it as the immutable AuthData of the account
func identifier(username string) int64 {
	h := fnv.New64a()
	_, _ = h.Write([]byte(username))
	value := int64(h.Sum64() & math.MaxInt64)
	if value == 0 {
		return 1
	}

	return value
}

type accessResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
}

type userResponse struct {
	Id       int64  `json:"id"`
	Username string `json:"username"`
	Login    string `json:"login"`
	Email    string `json:"email"`
	Name     string `json:"name"`
}

func main() {
	var address string
	var clientId string
	var clientSecret string
	var redirectURI string
	var emailDomain string
	var terminationGracePeriod time.Duration
	var lameduck time.Duration
	var keepAlive bool
	flag.StringVar(&address, "address", envOrDefaultValue("ADDRESS", "0.0.0.0:8080"), "HTTP server address")
	flag.StringVar(&clientId, "client-id", envOrDefaultValue("CLIENT_ID", ""), "OAuth client id the consumer presents")
	flag.StringVar(&clientSecret, "client-secret", envOrDefaultValue("CLIENT_SECRET", ""), "OAuth client secret the consumer presents")
	flag.StringVar(&redirectURI, "redirect-uri", envOrDefaultValue("REDIRECT_URI", ""), "The only redirect_uri an authorization request may ask for")
	flag.StringVar(&emailDomain, "email-domain", envOrDefaultValue("EMAIL_DOMAIN", "localhost"), "Domain appended to the authenticated user name to form an e-mail address")

	flag.DurationVar(&terminationGracePeriod, "termination-grace-period", envOrDefaultValue("TERMINATION_GRACE_PERIOD", 10*time.Second), "The duration the application needs to terminate gracefully")
	flag.DurationVar(&lameduck, "lameduck", envOrDefaultValue("LAMEDUCK", 1*time.Second), "A period that explicitly asks clients to stop sending requests, although the backend task is listening on that port and can provide the service")
	flag.BoolVar(&keepAlive, "http-keepalive", envOrDefaultValue("HTTP_KEEPALIVE", true), "Enable HTTP keep-alive")
	flag.Parse()

	if clientId == "" || clientSecret == "" || redirectURI == "" {
		log.Fatal("client-id, client-secret and redirect-uri are required")
	}

	redirect, err := url.Parse(redirectURI)
	if err != nil {
		log.Fatalf("failed to parse redirect-uri: %+v", err)
	}

	codes := newStore()
	tokens := newStore()

	mux := http.NewServeMux()
	mux.HandleFunc("GET "+prefix+"/authorize", func(w http.ResponseWriter, r *http.Request) {
		username := r.Header.Get(userHeader)
		if username == "" {
			http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
			return
		}

		query := r.URL.Query()
		if query.Get("client_id") != clientId || query.Get("response_type") != "code" || query.Get("redirect_uri") != redirectURI {
			http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
			return
		}

		code, err := newSecret()
		if err != nil {
			log.Printf("%+v", err)
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
		codes.put(code, grant{username: username, expiresAt: time.Now().Add(codeTTL)})

		location := *redirect
		parameters := location.Query()
		parameters.Set("code", code)
		if state := query.Get("state"); state != "" {
			parameters.Set("state", state)
		}
		location.RawQuery = parameters.Encode()

		http.Redirect(w, r, location.String(), http.StatusFound)
	})

	mux.HandleFunc("POST "+prefix+"/token", func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
			return
		}

		if r.PostFormValue("client_id") != clientId || subtle.ConstantTimeCompare([]byte(r.PostFormValue("client_secret")), []byte(clientSecret)) != 1 {
			http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
			return
		}

		if r.PostFormValue("grant_type") != "authorization_code" || r.PostFormValue("redirect_uri") != redirectURI {
			http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
			return
		}

		value, exists := codes.get(r.PostFormValue("code"), true)
		if !exists {
			http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
			return
		}

		token, err := newSecret()
		if err != nil {
			log.Printf("%+v", err)
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
		tokens.put(token, grant{username: value.username, expiresAt: time.Now().Add(tokenTTL)})

		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(accessResponse{AccessToken: token, TokenType: "bearer"})
	})

	mux.HandleFunc("GET "+prefix+"/user", func(w http.ResponseWriter, r *http.Request) {
		token, found := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
		if !found {
			http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
			return
		}

		value, exists := tokens.get(token, false)
		if !exists {
			http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
			return
		}

		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(userResponse{
			Id:       identifier(value.username),
			Username: value.username,
			Login:    value.username,
			Email:    value.username + "@" + emailDomain,
			Name:     value.username,
		})
	})

	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(http.StatusText(http.StatusOK)))
	})

	listener, err := net.Listen("tcp", address)
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
