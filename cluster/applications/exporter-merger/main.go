package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"runtime/debug"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	dto "github.com/prometheus/client_model/go"
	"github.com/prometheus/common/expfmt"
	"github.com/prometheus/common/model"
)

func p[T any](v T) *T {
	return &v
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
	var port string
	var terminationGracePeriod time.Duration
	var lameduck time.Duration
	var keepAlive bool
	var url string

	flag.StringVar(&port, "port", envOrDefaultValue("MERGER_PORT", "8080"), "")
	flag.DurationVar(&terminationGracePeriod, "termination-grace-period", envOrDefaultValue("TERMINATION_GRACE_PERIOD", 10*time.Second), "The duration the application needs to terminate gracefully")
	flag.DurationVar(&lameduck, "lameduck", envOrDefaultValue("LAMEDUCK", 1*time.Second), "A period that explicitly asks clients to stop sending requests, although the backend task is listening on that port and can provide the service")
	flag.BoolVar(&keepAlive, "http-keepalive", envOrDefaultValue("HTTP_KEEPALIVE", true), "")
	flag.StringVar(&url, "url", envOrDefaultValue("MERGER_URLS", ""), "URL to scrape. Can be speficied multiple times. (ENV:MERGER_URLS,space-seperated)")
	flag.Parse()

	http.DefaultTransport.(*http.Transport).MaxIdleConnsPerHost = http.DefaultTransport.(*http.Transport).MaxIdleConns

	var urls []string
	if url != "" {
		for _, u := range strings.Split(url, " ") {
			u = strings.TrimSpace(u)
			if u != "" {
				urls = append(urls, u)
			}
		}
	}

	mux := http.NewServeMux()

	mux.HandleFunc("GET /metrics", func(w http.ResponseWriter, r *http.Request) {
		merged := make(map[string]*dto.MetricFamily)
		mutex := &sync.Mutex{}
		wg := &sync.WaitGroup{}

		for _, u := range urls {
			wg.Go(func() {
				request, err := http.NewRequestWithContext(r.Context(), http.MethodGet, u, nil)
				if err != nil {
					return
				}
				response, err := http.DefaultClient.Do(request)
				if err != nil {
					return
				}
				defer func() {
					_ = response.Body.Close()
				}()

				if response.StatusCode >= 400 {
					return
				}

				parser := expfmt.NewTextParser(model.UTF8Validation)
				families, err := parser.TextToMetricFamilies(response.Body)
				if err != nil {
					return
				}

				for _, metric := range families {
					for _, m := range metric.Metric {
						m.Label = append(m.Label, &dto.LabelPair{
							Name:  p("merger_url"),
							Value: p(u),
						})
					}
				}

				mutex.Lock()
				for name, family := range families {
					if existing, ok := merged[name]; ok {
						existing.Metric = append(existing.Metric, family.Metric...)
					} else {
						merged[name] = family
					}
				}
				mutex.Unlock()
			})
		}

		wg.Wait()

		names := make([]string, 0, len(merged))
		for name := range merged {
			names = append(names, name)
		}
		sort.Strings(names)

		w.Header().Set("Content-Type", string(expfmt.NewFormat(expfmt.TypeTextPlain)))
		encoder := expfmt.NewEncoder(w, expfmt.NewFormat(expfmt.TypeTextPlain))
		for _, name := range names {
			_ = encoder.Encode(merged[name])
		}
	})

	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(http.StatusText(http.StatusOK)))
	})

	listener, err := net.Listen("tcp", fmt.Sprintf("0.0.0.0:%s", port))
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
