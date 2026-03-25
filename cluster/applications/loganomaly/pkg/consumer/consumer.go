package consumer

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"os/signal"
	"regexp"
	"strings"
	"sync"
	"syscall"
	"time"

	"loganomaly/internal/event"

	_ "k8s.io/client-go/plugin/pkg/client/auth"

	"github.com/confluentinc/confluent-kafka-go/v2/kafka"
	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
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
)

var (
	reUUID      = regexp.MustCompile(`[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}`)
	reIP        = regexp.MustCompile(`\b\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}\b`)
	reTimestamp = regexp.MustCompile(`\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}[^\s]*`)
	reHex       = regexp.MustCompile(`\b[0-9a-fA-F]{16,}\b`)
	reNumericID = regexp.MustCompile(`\b\d{4,}\b`)
	reLongStr   = regexp.MustCompile(`"[^"]{20,}"`)

	immediatePatterns = regexp.MustCompile(`(?i)(OutOfMemory|OOM|heap space|panic|segfault|SIGSEGV|SIGKILL|core dump|disk full|no space left|too many open files)`)
)

func normalizeMessage(message string) string {
	message = reUUID.ReplaceAllString(message, "<UUID>")
	message = reIP.ReplaceAllString(message, "<IP>")
	message = reTimestamp.ReplaceAllString(message, "<TS>")
	message = reHex.ReplaceAllString(message, "<HEX>")
	message = reNumericID.ReplaceAllString(message, "<ID>")
	message = reLongStr.ReplaceAllString(message, `"<STR>"`)
	return message
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

type bucket struct {
	totalCount int
	errorCount int
	timestamp  time.Time
}

type window struct {
	buckets    []bucket
	bucketSize time.Duration
	windowSize time.Duration
	errorRates []float64
}

func newWindow(bucketSize time.Duration, windowSize time.Duration) *window {
	return &window{
		buckets:    make([]bucket, 0),
		bucketSize: bucketSize,
		windowSize: windowSize,
		errorRates: make([]float64, 0),
	}
}

func (w *window) addRecord(isError bool) {
	now := time.Now()
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
	mu       sync.Mutex
	windows  map[string]*window
	dedupMap map[string]time.Time
	args     *Args
	producer *kafka.Producer
}

func newDetector(a *Args, p *kafka.Producer) *detector {
	return &detector{
		windows:  make(map[string]*window),
		dedupMap: make(map[string]time.Time),
		args:     a,
		producer: p,
	}
}

func (d *detector) handleRecord(record event.LogRecord) {
	isError := isErrorLevel(record.Level)

	d.mu.Lock()
	defer d.mu.Unlock()

	w, ok := d.windows[record.Grouping]
	if !ok {
		w = newWindow(10*time.Second, 5*time.Minute)
		d.windows[record.Grouping] = w
	}
	w.addRecord(isError)

	if isError && immediatePatterns.MatchString(record.Message) {
		normalized := normalizeMessage(record.Message)
		hash := errorHash(normalized)
		dedupKey := fmt.Sprintf("immediate:%s:%s", record.Grouping, hash)

		if !d.trySuppress(dedupKey) {
			d.emit(event.AnomalyEvent{
				Grouping:      record.Grouping,
				ErrorHash:     hash,
				Count:         1,
				Window:        w.windowSize.String(),
				DetectionMode: event.DetectionModeImmediate,
				BlastRadius:   d.blastRadius(),
				Summary:       normalized,
			})
		}
	}
}

func (d *detector) evaluate() {
	d.mu.Lock()
	defer d.mu.Unlock()

	radius := d.blastRadius()
	for grouping, w := range d.windows {
		rate := w.errorRate()
		if rate == 0 {
			continue
		}

		zScore := d.calculateZScore(w, rate)

		w.errorRates = append(w.errorRates, rate)
		if len(w.errorRates) > 30 {
			w.errorRates = w.errorRates[len(w.errorRates)-30:]
		}

		if zScore > d.args.ZScoreThreshold {
			dedupKey := fmt.Sprintf("windowed:%s", grouping)
			if !d.trySuppress(dedupKey) {
				d.emit(event.AnomalyEvent{
					Grouping:      grouping,
					ErrorHash:     errorHash(fmt.Sprintf("zscore:%s", grouping)),
					Count:         w.errorCount(),
					Window:        w.windowSize.String(),
					DetectionMode: event.DetectionModeWindowed,
					ZScore:        math.Round(zScore*100) / 100,
					BlastRadius:   radius,
					Summary:       fmt.Sprintf("error rate anomaly: %.2f (z-score: %.2f)", rate, zScore),
				})
			}
		}
	}

	d.pruneStaleEntries()
}

func (d *detector) calculateZScore(w *window, rate float64) float64 {
	if len(w.errorRates) < d.args.MinSamples {
		return 0
	}

	var sum, sumSq float64
	for _, r := range w.errorRates {
		sum += r
		sumSq += r * r
	}
	n := float64(len(w.errorRates))
	mean := sum / n
	variance := sumSq/n - mean*mean
	if variance < 0 {
		variance = 0
	}
	stddev := math.Sqrt(variance)

	if stddev > 0 {
		return (rate - mean) / stddev
	}
	return 0
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

func (d *detector) blastRadius() int {
	count := 0
	for _, w := range d.windows {
		if w.errorCount() > 0 {
			count++
		}
	}
	return count
}

func (d *detector) pruneStaleEntries() {
	for key, t := range d.dedupMap {
		if time.Since(t) > d.args.SuppressionDuration {
			delete(d.dedupMap, key)
		}
	}
	for grouping, w := range d.windows {
		if len(w.buckets) == 0 {
			delete(d.windows, grouping)
		}
	}
}

func Run(a *Args) error {
	if err := validator.New().Struct(a); err != nil {
		return xerrors.Errorf("invalid arguments: %w", err)
	}

	klog.InitFlags(nil)

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
				if err := consume(stopCh, a); err != nil {
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

func consume(stopCh <-chan struct{}, a *Args) error {
	c, err := kafka.NewConsumer(&kafka.ConfigMap{
		"bootstrap.servers":  a.BootstrapServers,
		"group.id":           controllerAgentName,
		"auto.offset.reset":  "latest",
		"enable.auto.commit": true,
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

	d := newDetector(a, p)

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
	}
}
