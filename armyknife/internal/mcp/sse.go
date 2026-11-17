package mcp

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"sync"
	"time"

	"golang.org/x/xerrors"
)

func (s *Server) ServeSSE(address string) error {
	sessions := &sync.Map{}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /sse", func(w http.ResponseWriter, r *http.Request) {
		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")

		id, err := randomID()
		if err != nil {
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
		out := make(chan []byte, 256)
		sessions.Store(id, out)
		defer sessions.Delete(id)

		fmt.Fprintf(w, "event: endpoint\ndata: /messages?sessionId=%s\n\n", id)
		flusher.Flush()

		for {
			select {
			case data := <-out:
				fmt.Fprintf(w, "event: message\ndata: %s\n\n", data)
				flusher.Flush()
			case <-r.Context().Done():
				return
			}
		}
	})

	mux.HandleFunc("POST /messages", func(w http.ResponseWriter, r *http.Request) {
		id := r.URL.Query().Get("sessionId")
		outAny, ok := sessions.Load(id)
		if !ok {
			http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
			return
		}
		out := outAny.(chan []byte)

		var request JSONRPCRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
			return
		}

		if request.ID != nil {
			responseBytes, err := json.Marshal(s.dispatch(request))
			if err != nil {
				http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
				return
			}
			select {
			case out <- responseBytes:
			case <-r.Context().Done():
			}
		}

		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte(http.StatusText(http.StatusAccepted)))
	})

	server := &http.Server{
		Addr:              address,
		Handler:           allowLocal(mux),
		ReadHeaderTimeout: 10 * time.Second,
	}
	return server.ListenAndServe()
}

// https://modelcontextprotocol.io/specification/2025-06-18/basic/transports#security-warning
func allowLocal(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if origin := r.Header.Get("Origin"); origin != "" && !isLoopbackOrigin(origin) {
			http.Error(w, http.StatusText(http.StatusForbidden), http.StatusForbidden)
			return
		}
		if !isLoopbackHost(r.Host) {
			http.Error(w, http.StatusText(http.StatusForbidden), http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func isLoopbackHost(hostport string) bool {
	host := hostport
	if h, _, err := net.SplitHostPort(hostport); err == nil {
		host = h
	}
	switch host {
	case "127.0.0.1", "localhost", "::1", "[::1]":
		return true
	}
	return false
}

func isLoopbackOrigin(origin string) bool {
	parsed, err := url.Parse(origin)
	if err != nil {
		return false
	}
	return isLoopbackHost(parsed.Host)
}

func randomID() (string, error) {
	buffer := make([]byte, 16)
	if _, err := rand.Read(buffer); err != nil {
		return "", xerrors.Errorf("failed to read random: %w", err)
	}
	return hex.EncodeToString(buffer), nil
}
