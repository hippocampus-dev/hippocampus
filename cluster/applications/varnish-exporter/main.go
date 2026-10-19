package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"runtime/debug"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"

	dto "github.com/prometheus/client_model/go"
	"github.com/prometheus/common/expfmt"
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

type counter struct {
	Description string  `json:"description"`
	Flag        string  `json:"flag"`
	Value       float64 `json:"value"`
}

func metricFamilies(counters map[string]counter) []*dto.MetricFamily {
	keys := make([]string, 0, len(counters))
	for key := range counters {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	families := make(map[string]*dto.MetricFamily)
	names := make([]string, 0, len(counters))
	for _, key := range keys {
		c := counters[key]
		fields := strings.Split(key, ".")
		name := "varnish_" + strings.ToLower(fields[0]) + "_" + strings.ToLower(fields[len(fields)-1])

		family, exists := families[name]
		if !exists {
			metricType := dto.MetricType_GAUGE
			if c.Flag == "c" {
				metricType = dto.MetricType_COUNTER
			}
			family = &dto.MetricFamily{
				Name: p(name),
				Help: p(c.Description),
				Type: p(metricType),
			}
			families[name] = family
			names = append(names, name)
		}

		metric := &dto.Metric{}
		if ident := strings.Join(fields[1:len(fields)-1], "."); ident != "" {
			metric.Label = []*dto.LabelPair{
				{
					Name:  p("ident"),
					Value: p(ident),
				},
			}
		}
		if family.GetType() == dto.MetricType_COUNTER {
			metric.Counter = &dto.Counter{Value: p(c.Value)}
		} else {
			metric.Gauge = &dto.Gauge{Value: p(c.Value)}
		}
		family.Metric = append(family.Metric, metric)
	}

	sort.Strings(names)

	ordered := make([]*dto.MetricFamily, 0, len(names))
	for _, name := range names {
		ordered = append(ordered, families[name])
	}

	return ordered
}

func main() {
	var address string
	var terminationGracePeriod time.Duration
	var lameduck time.Duration
	var keepAlive bool

	flag.StringVar(&address, "address", envOrDefaultValue("ADDRESS", "0.0.0.0:9131"), "HTTP server address")
	flag.DurationVar(&terminationGracePeriod, "termination-grace-period", envOrDefaultValue("TERMINATION_GRACE_PERIOD", 10*time.Second), "The duration the application needs to terminate gracefully")
	flag.DurationVar(&lameduck, "lameduck", envOrDefaultValue("LAMEDUCK", 1*time.Second), "A period that explicitly asks clients to stop sending requests, although the backend task is listening on that port and can provide the service")
	flag.BoolVar(&keepAlive, "http-keepalive", envOrDefaultValue("HTTP_KEEPALIVE", true), "Enable HTTP keep-alive")
	flag.Parse()

	mux := http.NewServeMux()

	mux.HandleFunc("GET /metrics", func(w http.ResponseWriter, r *http.Request) {
		command := exec.CommandContext(r.Context(), "varnishstat", "-1", "-j")
		command.Stderr = os.Stderr
		output, err := command.Output()
		if err != nil {
			log.Printf("failed to execute varnishstat: %+v", err)
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}

		var statistics struct {
			Counters map[string]counter `json:"counters"`
		}
		if err := json.Unmarshal(output, &statistics); err != nil {
			log.Printf("failed to unmarshal varnishstat output: %+v", err)
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", string(expfmt.NewFormat(expfmt.TypeTextPlain)))
		encoder := expfmt.NewEncoder(w, expfmt.NewFormat(expfmt.TypeTextPlain))
		for _, family := range metricFamilies(statistics.Counters) {
			_ = encoder.Encode(family)
		}
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
