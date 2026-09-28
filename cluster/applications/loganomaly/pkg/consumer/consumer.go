package consumer

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"net"
	"net/http"
	"os"
	"os/signal"
	"regexp"
	"runtime/debug"
	"strings"
	"sync"
	"syscall"
	"time"

	"loganomaly/internal/event"

	_ "k8s.io/client-go/plugin/pkg/client/auth"

	"github.com/confluentinc/confluent-kafka-go/v2/kafka"
	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/otel/attribute"
	otelprometheus "go.opentelemetry.io/otel/exporters/prometheus"
	"go.opentelemetry.io/otel/metric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"golang.org/x/xerrors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/leaderelection"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
	"k8s.io/klog/v2"
)

const (
	controllerAgentName    = "loganomaly"
	inClusterNamespacePath = "/var/run/secrets/kubernetes.io/serviceaccount/namespace"

	windowIdleTimeout = time.Hour
)

var (
	// The kubelet's systemd cgroup driver writes a pod UID with underscores
	reUUID = regexp.MustCompile(`[0-9a-fA-F]{8}[-_][0-9a-fA-F]{4}[-_][0-9a-fA-F]{4}[-_][0-9a-fA-F]{4}[-_][0-9a-fA-F]{12}`)
	reIP   = regexp.MustCompile(`\b\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}\b`)
	// yyyy/MM/dd HH:mm:ss.SSS ZZZZZ https://github.com/tikv/rfcs/blob/master/text/0018-unified-log-format.md
	// Lmmdd hh:mm:ss.uuuuuu https://github.com/kubernetes/klog/blob/main/klog.go
	reTimestamp = regexp.MustCompile(`(?:\d{4}-\d{2}-\d{2}|\d{4}/\d{2}/\d{2})[T ]\d{2}:\d{2}:\d{2}(?:[.,]\d+)?(?:Z| ?[+-]\d{2}:?\d{2})?|\d{4} \d{2}:\d{2}:\d{2}\.\d+`)
	// A panic dump's pointers differ between runs of the same crash
	// A shorter value is a constant that separates crashes, such as SIGSEGV's code and addr
	reHex = regexp.MustCompile(`\b0x[0-9a-fA-F]{4,}\b|\b[0-9a-fA-F]{16,}\b`)
	// mimir's TSDB block ULID (01M078AY9F7ZFJK6MNFY9126VJ) is not hex, so reHex leaves it whole
	reOpaqueIdentifier = regexp.MustCompile(`\b[0-9A-Za-z]{16,}\b`)
	// No trailing \b: the kernel's anon-rss:128940kB puts a unit suffix straight after the digits
	// The leading \b leaves sha256 and x86_64 intact
	reNumericID = regexp.MustCompile(`\b\d+`)
	reLongStr   = regexp.MustCompile(`"[^"]{20,}"`)

	// matchesImmediate tests a whole logged line, which ResolvedMessage in internal/event/event.go widens to the entire JSON document wherever fluentd dropped the message field
	// A bare keyword therefore also matches a metric name in an access log's URI, an --oom-bump-up-ratio argument in a container spec and the panic key in a config dump, and each benign spelling costs an immediateExclusions alternative the next spelling walks around
	// oom_kill_process is the kernel symbol the OOM killer's stack trace carries
	// System OOM encountered is the note on the SystemOOM event kubelet reports through events-logger
	// http: panic serving is what net/http logs for a handler panic it recovered
	// segfault at is what the kernel's show_signal_msg prints ahead of the faulting address
	immediatePatterns = regexp.MustCompile(`(?i)(out of memory|System OOM encountered|\bOOMKilled\b|\boom_kill_process\b|heap space|MemoryError|memory allocation of|due to memory pressure|(?:^|[^\w-])panic:\s|http: panic serving|panicked at|segfault at|SIGSEGV|SIGKILL|core dump|disk full|no space left|too many open files)`)
	// Deleting the exclusion match and re-testing, rather than dropping the line, is what keeps a line carrying both a benign keyword and a real one detected
	// An alternative that does not span the keyword immediatePatterns matches therefore suppresses nothing, while the build, the vet and the detector all stay silent
	// -XX:+CrashOnOutOfMemoryError names the condition without reporting one and ends in the OutOfMemoryError that MemoryError matches
	// A structured record carries no whitespace, so \S+ here would delete past the flag into a keyword further along the line
	// mcrouter segfaults while shutting down on SIGTERM, and a kernel line names no pod, so this covers the scale-down of every mcrouter deployment
	immediateExclusions = regexp.MustCompile(`(?i)-XX:[+-]\w+|mcrouter\[\d+\]: segfault`)

	annotationGroup = apiGroup()
)

func normalizeMessage(message string) string {
	message = reUUID.ReplaceAllString(message, "<UUID>")
	message = reIP.ReplaceAllString(message, "<IP>")
	message = reTimestamp.ReplaceAllString(message, "<TS>")
	message = reHex.ReplaceAllString(message, "<HEX>")
	message = reOpaqueIdentifier.ReplaceAllStringFunc(message, opaqueIdentifier)
	message = reNumericID.ReplaceAllString(message, "<ID>")
	message = reLongStr.ReplaceAllString(message, `"<STR>"`)
	return message
}

func opaqueIdentifier(token string) string {
	digits := 0
	for _, r := range token {
		if r >= '0' && r <= '9' {
			digits++
		}
	}
	// OAuth2AuthorizationException and Base64EncodedIdentifier reach 16 characters carrying only a version number
	if digits < 4 {
		return token
	}
	return "<ID>"
}

func errorHash(message string) string {
	h := sha256.Sum256([]byte(message))
	return hex.EncodeToString(h[:4])
}

func isErrorLevel(level string) bool {
	switch strings.ToLower(level) {
	case "error", "err", "fatal", "panic", "critical", "crit", "alert", "emerg":
		return true
	}
	return false
}

func matchesImmediate(message string) bool {
	if !immediatePatterns.MatchString(message) {
		return false
	}
	return immediatePatterns.MatchString(immediateExclusions.ReplaceAllString(message, ""))
}

func apiGroup() string {
	defaultGroup := "loganomaly.kaidotio.github.io"
	if v, ok := os.LookupEnv("VARIANT"); ok {
		return fmt.Sprintf("%s.%s", v, defaultGroup)
	}
	return defaultGroup
}

type bucket struct {
	totalCount int
	errorCount int
	timestamp  time.Time
}

type window struct {
	buckets      []bucket
	bucketSize   time.Duration
	windowSize   time.Duration
	errorCounts  []int
	startedAt    time.Time
	lastRecordAt time.Time
}

func newWindow(startedAt time.Time, bucketSize time.Duration, windowSize time.Duration) *window {
	return &window{
		buckets:      make([]bucket, 0),
		bucketSize:   bucketSize,
		windowSize:   windowSize,
		errorCounts:  make([]int, 0),
		startedAt:    startedAt,
		lastRecordAt: startedAt,
	}
}

func (w *window) addRecord(isError bool) {
	now := time.Now()
	w.lastRecordAt = now
	bucketTime := now.Truncate(w.bucketSize)

	if len(w.buckets) == 0 || !w.buckets[len(w.buckets)-1].timestamp.Equal(bucketTime) {
		w.buckets = append(w.buckets, bucket{timestamp: bucketTime})
	}

	b := &w.buckets[len(w.buckets)-1]
	b.totalCount++
	if isError {
		b.errorCount++
	}

	w.pruneOldBuckets(now)
}

func (w *window) pruneOldBuckets(now time.Time) {
	cutoff := now.Add(-w.windowSize)
	i := 0
	for i < len(w.buckets) && w.buckets[i].timestamp.Before(cutoff) {
		i++
	}
	if i > 0 {
		w.buckets = w.buckets[i:]
	}
}

func (w *window) errorRate() float64 {
	var total, errors int
	for _, b := range w.buckets {
		total += b.totalCount
		errors += b.errorCount
	}
	if total == 0 {
		return 0
	}
	return float64(errors) / float64(total)
}

func (w *window) errorCount() int {
	var errors int
	for _, b := range w.buckets {
		errors += b.errorCount
	}
	return errors
}

type detector struct {
	mu              sync.Mutex
	windows         map[string]*window
	dedupMap        map[string]time.Time
	args            *Args
	producer        *kafka.Producer
	detectionsTotal metric.Int64Counter
	suppressions    metric.Int64Counter
	activeGroupings metric.Int64UpDownCounter
	excludedTotal   metric.Int64Counter
	excludePatterns sync.Map
}

func (d *detector) excludePattern(record event.LogRecord, key string, pattern string) *regexp.Regexp {
	if cached, ok := d.excludePatterns.Load(pattern); ok {
		compiled, _ := cached.(*regexp.Regexp)
		return compiled
	}

	compiled, err := regexp.Compile(pattern)
	if err != nil {
		// A typo must leave the grouping detected rather than silence it
		klog.Errorf("failed to compile %s on %s: %v", key, record.ResolvedPod(), err)
		compiled = nil
	}
	d.excludePatterns.Store(pattern, compiled)

	return compiled
}

func (d *detector) excludedBy(record event.LogRecord, annotations map[string]string, key string, regexKey string, message string) bool {
	if annotations[key] == "true" {
		return true
	}

	pattern := annotations[regexKey]
	if pattern == "" {
		return false
	}
	compiled := d.excludePattern(record, regexKey, pattern)

	return compiled != nil && compiled.MatchString(message)
}

// Every pod carries the annotations ArgoCD sets, so the keys are built only where ours are present
func annotated(annotations map[string]string) bool {
	for key := range annotations {
		if strings.HasPrefix(key, annotationGroup) {
			return true
		}
	}
	return false
}

func (d *detector) excluded(record event.LogRecord, message string) bool {
	container := record.ResolvedContainer()
	if container == "" {
		return false
	}

	annotations := record.ResolvedAnnotations()
	if !annotated(annotations[0]) && !annotated(annotations[1]) {
		return false
	}

	excludeKey := fmt.Sprintf("%s/%s.exclude", annotationGroup, container)
	excludeRegexKey := fmt.Sprintf("%s/%s.exclude-regex", annotationGroup, container)
	for _, scope := range annotations {
		if d.excludedBy(record, scope, excludeKey, excludeRegexKey, message) {
			return true
		}
	}

	return false
}

func newDetector(a *Args, p *kafka.Producer, detectionsTotal metric.Int64Counter, suppressions metric.Int64Counter, activeGroupings metric.Int64UpDownCounter, excludedTotal metric.Int64Counter) *detector {
	return &detector{
		windows:         make(map[string]*window),
		dedupMap:        make(map[string]time.Time),
		args:            a,
		producer:        p,
		detectionsTotal: detectionsTotal,
		suppressions:    suppressions,
		activeGroupings: activeGroupings,
		excludedTotal:   excludedTotal,
	}
}

func (d *detector) handleRecord(record event.LogRecord) {
	message := record.ResolvedMessage()
	if d.excluded(record, message) {
		d.excludedTotal.Add(context.Background(), 1)
		return
	}

	isError := isErrorLevel(record.ResolvedLevel())

	d.mu.Lock()
	defer d.mu.Unlock()

	w, ok := d.windows[record.Grouping]
	if !ok {
		w = newWindow(time.Now(), 10*time.Second, 5*time.Minute)
		d.windows[record.Grouping] = w
		d.activeGroupings.Add(context.Background(), 1)
	}
	w.addRecord(isError)

	if matchesImmediate(message) {
		// reLongStr can collapse the quoted string the matched keyword sits in, so the hash takes the normalized form and the summary keeps the raw line
		normalized := normalizeMessage(message)
		hash := errorHash(normalized)
		dedupKey := fmt.Sprintf("immediate:%s:%s", record.Grouping, hash)

		modeAttribute := metric.WithAttributes(attribute.Key("mode").String(event.DetectionModeImmediate))
		if !d.trySuppress(dedupKey) {
			d.emit(event.AnomalyEvent{
				Grouping:      record.Grouping,
				ErrorHash:     hash,
				Count:         1,
				Window:        w.windowSize.String(),
				DetectionMode: event.DetectionModeImmediate,
				Summary:       message,
				Pod:           record.ResolvedPod(),
			})
			d.detectionsTotal.Add(context.Background(), 1, modeAttribute)
		} else {
			d.suppressions.Add(context.Background(), 1, modeAttribute)
		}
	}
}

func (d *detector) evaluate() {
	d.mu.Lock()
	defer d.mu.Unlock()

	now := time.Now()
	for _, w := range d.windows {
		w.pruneOldBuckets(now)
	}

	for grouping, w := range d.windows {
		// A window still filling yields counts proportional to its age
		if now.Sub(w.startedAt) < w.windowSize {
			continue
		}

		count := w.errorCount()
		zScore := d.calculateZScore(w, count)

		w.errorCounts = append(w.errorCounts, count)
		if len(w.errorCounts) > 30 {
			w.errorCounts = w.errorCounts[len(w.errorCounts)-30:]
		}

		if zScore > d.args.ZScoreThreshold {
			dedupKey := fmt.Sprintf("windowed:%s", grouping)
			modeAttribute := metric.WithAttributes(attribute.Key("mode").String(event.DetectionModeWindowed))
			if !d.trySuppress(dedupKey) {
				d.emit(event.AnomalyEvent{
					Grouping:      grouping,
					ErrorHash:     errorHash(fmt.Sprintf("zscore:%s", grouping)),
					Count:         count,
					Window:        w.windowSize.String(),
					DetectionMode: event.DetectionModeWindowed,
					ZScore:        math.Round(zScore*100) / 100,
					Summary:       fmt.Sprintf("error count anomaly: %d (rate: %.2f, z-score: %.2f)", count, w.errorRate(), zScore),
				})
				d.detectionsTotal.Add(context.Background(), 1, modeAttribute)
			} else {
				d.suppressions.Add(context.Background(), 1, modeAttribute)
			}
		}
	}

	d.pruneStaleEntries()
}

func (d *detector) calculateZScore(w *window, count int) float64 {
	if len(w.errorCounts) < d.args.MinSamples {
		return 0
	}

	n := float64(len(w.errorCounts))
	var sum float64
	for _, c := range w.errorCounts {
		sum += float64(c)
	}
	mean := sum / n

	var sumSquaredDeviation float64
	for _, c := range w.errorCounts {
		deviation := float64(c) - mean
		sumSquaredDeviation += deviation * deviation
	}

	// The observed spread absorbs overdispersion from batched lines and gaps
	// Floored at the Poisson term: a flat or zero baseline is rounding noise
	standardDeviation := max(math.Sqrt(sumSquaredDeviation/n), math.Sqrt(mean+1))

	return (float64(count) - mean) / standardDeviation
}

func (d *detector) emit(e event.AnomalyEvent) {
	bytes, err := json.Marshal(e)
	if err != nil {
		klog.Errorf("failed to marshal event: %v", err)
		return
	}

	topic := d.args.OutputTopic
	if err := d.producer.Produce(&kafka.Message{
		TopicPartition: kafka.TopicPartition{Topic: &topic, Partition: kafka.PartitionAny},
		Value:          bytes,
	}, nil); err != nil {
		klog.Errorf("failed to produce event: %v", err)
	}
}

func (d *detector) trySuppress(key string) bool {
	if t, ok := d.dedupMap[key]; ok {
		if time.Since(t) < d.args.SuppressionDuration {
			return true
		}
	}
	d.dedupMap[key] = time.Now()
	return false
}

func (d *detector) pruneStaleEntries() {
	for key, t := range d.dedupMap {
		if time.Since(t) > d.args.SuppressionDuration {
			delete(d.dedupMap, key)
		}
	}
	// An empty window is still the baseline the grouping is measured against
	for grouping, w := range d.windows {
		if time.Since(w.lastRecordAt) > windowIdleTimeout {
			delete(d.windows, grouping)
			d.activeGroupings.Add(context.Background(), -1)
		}
	}
}

func Run(a *Args) error {
	if err := validator.New().Struct(a); err != nil {
		return xerrors.Errorf("invalid arguments: %w", err)
	}

	klog.InitFlags(nil)

	exporter, err := otelprometheus.New()
	if err != nil {
		return xerrors.Errorf("failed to create exporter: %w", err)
	}
	meter := sdkmetric.NewMeterProvider(sdkmetric.WithReader(exporter)).Meter("loganomaly")
	detectionsTotal, err := meter.Int64Counter("loganomaly_detections_total")
	if err != nil {
		return xerrors.Errorf("failed to create counter: %w", err)
	}
	suppressionsTotal, err := meter.Int64Counter("loganomaly_suppressions_total")
	if err != nil {
		return xerrors.Errorf("failed to create counter: %w", err)
	}
	recordsConsumedTotal, err := meter.Int64Counter("loganomaly_records_consumed_total")
	if err != nil {
		return xerrors.Errorf("failed to create counter: %w", err)
	}
	activeGroupings, err := meter.Int64UpDownCounter("loganomaly_active_groupings")
	if err != nil {
		return xerrors.Errorf("failed to create gauge: %w", err)
	}
	excludedTotal, err := meter.Int64Counter("loganomaly_excluded_total")
	if err != nil {
		return xerrors.Errorf("failed to create counter: %w", err)
	}

	mux := http.NewServeMux()
	mux.Handle("GET /metrics", promhttp.Handler())
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(http.StatusText(http.StatusOK)))
	})

	metricsListener, err := net.Listen("tcp", a.MetricsAddress)
	if err != nil {
		return xerrors.Errorf("failed to listen: %w", err)
	}
	go func() {
		defer func() {
			if err := recover(); err != nil {
				log.Printf("panic: %+v\n%s", err, debug.Stack())
			}
		}()
		if err := http.Serve(metricsListener, mux); err != nil {
			log.Fatalf("failed to serve metrics: %+v", err)
		}
	}()

	stopCh := make(chan struct{}, 1)

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGTERM)
	go func() {
		<-quit
		close(stopCh)
	}()

	kubeConfig, err := rest.InClusterConfig()
	if err != nil {
		return xerrors.Errorf("failed to create kubernetes config: %w", err)
	}
	clientset, err := kubernetes.NewForConfig(kubeConfig)
	if err != nil {
		return xerrors.Errorf("failed to create kubernetes client: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	id := uuid.New().String()
	lock := &resourcelock.LeaseLock{
		LeaseMeta: metav1.ObjectMeta{
			Name: controllerAgentName,
			Namespace: func() string {
				namespace, err := os.ReadFile(inClusterNamespacePath)
				if err != nil {
					klog.Fatalf("failed to find leader election namespace: %+v", err)
				}
				return string(namespace)
			}(),
		},
		Client: clientset.CoordinationV1(),
		LockConfig: resourcelock.ResourceLockConfig{
			Identity: id,
		},
	}

	leaderelection.RunOrDie(ctx, leaderelection.LeaderElectionConfig{
		Lock:            lock,
		ReleaseOnCancel: true,
		LeaseDuration:   60 * time.Second,
		RenewDeadline:   15 * time.Second,
		RetryPeriod:     5 * time.Second,
		Callbacks: leaderelection.LeaderCallbacks{
			OnStartedLeading: func(ctx context.Context) {
				if err := consume(stopCh, a, detectionsTotal, suppressionsTotal, recordsConsumedTotal, activeGroupings, excludedTotal); err != nil {
					klog.Fatalf("failed to run: %s", err.Error())
					return
				}
			},
			OnStoppedLeading: func() {
				klog.Infof("leader lost: %s", id)
				os.Exit(0)
			},
			OnNewLeader: func(identity string) {
				if identity == id {
					return
				}
				klog.Infof("new leader elected: %s", identity)
			},
		},
	})

	return nil
}

func consume(stopCh <-chan struct{}, a *Args, detectionsTotal metric.Int64Counter, suppressionsTotal metric.Int64Counter, recordsConsumedTotal metric.Int64Counter, activeGroupings metric.Int64UpDownCounter, excludedTotal metric.Int64Counter) error {
	c, err := kafka.NewConsumer(&kafka.ConfigMap{
		"bootstrap.servers":          a.BootstrapServers,
		"group.id":                   controllerAgentName,
		"auto.offset.reset":          "latest",
		"enable.auto.commit":         true,
		"queued.max.messages.kbytes": 16384,
		"queued.min.messages":        10000,
	})
	if err != nil {
		return fmt.Errorf("failed to create consumer: %w", err)
	}
	defer c.Close()

	if err := c.Subscribe(a.InputTopic, nil); err != nil {
		return fmt.Errorf("failed to subscribe: %w", err)
	}

	p, err := kafka.NewProducer(&kafka.ConfigMap{
		"bootstrap.servers": a.BootstrapServers,
		// Shares the C heap with the consumer queue; the default is 8x limits.memory
		"queue.buffering.max.kbytes": 8192,
	})
	if err != nil {
		return fmt.Errorf("failed to create producer: %w", err)
	}
	defer p.Close()

	go func() {
		for e := range p.Events() {
			if m, ok := e.(*kafka.Message); ok && m.TopicPartition.Error != nil {
				klog.Errorf("delivery failed: %v", m.TopicPartition.Error)
			}
		}
	}()

	d := newDetector(a, p, detectionsTotal, suppressionsTotal, activeGroupings, excludedTotal)

	ticker := time.NewTicker(a.EvaluationInterval)
	defer ticker.Stop()

	go func() {
		for {
			select {
			case <-stopCh:
				return
			case <-ticker.C:
				d.evaluate()
			}
		}
	}()

	for {
		msg, err := c.ReadMessage(time.Second)
		if err != nil {
			if kafkaErr, ok := err.(kafka.Error); ok && kafkaErr.IsTimeout() {
				select {
				case <-stopCh:
					return nil
				default:
				}
				continue
			}
			klog.Errorf("consumer error: %v", err)
			continue
		}

		var record event.LogRecord
		if err := json.Unmarshal(msg.Value, &record); err != nil {
			continue
		}

		if record.Grouping == "" {
			continue
		}

		d.handleRecord(record)
		recordsConsumedTotal.Add(context.Background(), 1)
	}
}
