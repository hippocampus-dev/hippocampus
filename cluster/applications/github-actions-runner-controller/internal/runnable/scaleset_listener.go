package runnable

import (
	"context"
	"fmt"
	"sync"

	garV1 "github-actions-runner-controller/api/v1"

	"github.com/actions/scaleset"
	"github.com/actions/scaleset/listener"
	"github.com/go-logr/logr"
	"github.com/prometheus/client_golang/prometheus"
	"golang.org/x/xerrors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
)

var (
	scaleSetMetricLabelNames = []string{"namespace", "name", "owner", "repo"}

	assignedJobsGauge = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "github_actions_scale_set_assigned_jobs",
			Help: "Number of jobs assigned to the scale set, waiting and running",
		},
		scaleSetMetricLabelNames,
	)
	runningJobsGauge = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "github_actions_scale_set_running_jobs",
			Help: "Number of jobs running on the scale set",
		},
		scaleSetMetricLabelNames,
	)
	desiredRunnersGauge = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "github_actions_scale_set_desired_runners",
			Help: "Number of runners the scale set is converging to",
		},
		scaleSetMetricLabelNames,
	)
)

func init() {
	metrics.Registry.MustRegister(assignedJobsGauge, runningJobsGauge, desiredRunnersGauge)
}

type listenerHandle struct {
	generation int64
	cancel     context.CancelFunc
	done       chan struct{}
	client     *scaleset.Client
	scaleSetID int
}

type ScaleSetListener struct {
	client client.Client
	log    logr.Logger

	rootContext context.Context
	stop        context.CancelFunc
	mutex       sync.Mutex
	stopped     bool
	handles     map[types.UID]*listenerHandle
	waitGroup   sync.WaitGroup
}

func NewScaleSetListener(c client.Client, log logr.Logger) *ScaleSetListener {
	// Reconcile may call Ensure before Start, so the context every poll loop derives from is created here rather than in Start.
	rootContext, stop := context.WithCancel(context.Background())
	return &ScaleSetListener{
		client:      c,
		log:         log,
		rootContext: rootContext,
		stop:        stop,
		handles:     map[types.UID]*listenerHandle{},
	}
}

func (l *ScaleSetListener) NeedLeaderElection() bool {
	return true
}

func (l *ScaleSetListener) Start(ctx context.Context) error {
	<-ctx.Done()

	l.mutex.Lock()
	l.stopped = true
	l.mutex.Unlock()

	l.stop()
	l.waitGroup.Wait()

	return nil
}

func (l *ScaleSetListener) Session(uid types.UID) (*scaleset.Client, int, bool) {
	l.mutex.Lock()
	defer l.mutex.Unlock()

	handle := l.handles[uid]
	if handle == nil {
		return nil, 0, false
	}
	return handle.client, handle.scaleSetID, true
}

func (l *ScaleSetListener) Running(uid types.UID, generation int64) bool {
	l.mutex.Lock()
	defer l.mutex.Unlock()

	handle := l.handles[uid]
	return handle != nil && handle.generation == generation
}

func (l *ScaleSetListener) Ensure(scaleSet *garV1.ScaleSet, actionsClient *scaleset.Client, scaleSetID int) {
	l.mutex.Lock()
	stopped := l.stopped
	running := l.handles[scaleSet.UID]
	l.mutex.Unlock()

	if stopped {
		return
	}

	if running != nil {
		if running.generation == scaleSet.Generation {
			return
		}
		l.Stop(scaleSet.UID)
	}

	pollContext, cancel := context.WithCancel(l.rootContext)
	done := make(chan struct{})
	l.mutex.Lock()
	// Adding to the WaitGroup after Start began waiting on it panics, so this check and the Add share one critical section.
	if l.stopped {
		l.mutex.Unlock()
		cancel()
		return
	}
	l.handles[scaleSet.UID] = &listenerHandle{
		generation: scaleSet.Generation,
		cancel:     cancel,
		done:       done,
		client:     actionsClient,
		scaleSetID: scaleSetID,
	}
	l.waitGroup.Add(1)
	l.mutex.Unlock()

	scaler := &scaleSetScaler{
		client:     l.client,
		key:        client.ObjectKeyFromObject(scaleSet),
		owner:      scaleSet.Spec.Owner,
		repo:       scaleSet.Spec.Repo,
		minRunners: scaleSet.Spec.MinRunners,
		maxRunners: scaleSet.Spec.MaxRunners,
	}
	uid := scaleSet.UID
	generation := scaleSet.Generation

	go func() {
		defer l.waitGroup.Done()
		defer close(done)
		defer cancel()

		if err := l.poll(pollContext, actionsClient, scaleSetID, scaler); err != nil && pollContext.Err() == nil {
			l.log.WithValues("scaleset", scaler.key).Error(err, "listener stopped")
		}

		l.mutex.Lock()
		if handle := l.handles[uid]; handle != nil && handle.generation == generation {
			delete(l.handles, uid)
		}
		l.mutex.Unlock()
	}()
}

func (l *ScaleSetListener) poll(ctx context.Context, actionsClient *scaleset.Client, scaleSetID int, scaler *scaleSetScaler) error {
	session, err := actionsClient.MessageSessionClient(ctx, scaleSetID, scaler.key.Name)
	if err != nil {
		return xerrors.Errorf("failed to create message session: %w", err)
	}
	defer func() {
		_ = session.Close(context.WithoutCancel(ctx))
	}()

	runner, err := listener.New(session, listener.Config{
		ScaleSetID: scaleSetID,
		MaxRunners: scaler.maxRunners,
	}, listener.WithMetricsRecorder(scaler))
	if err != nil {
		return xerrors.Errorf("failed to create listener: %w", err)
	}

	return runner.Run(ctx, scaler)
}

func (l *ScaleSetListener) Stop(uid types.UID) {
	l.mutex.Lock()
	handle := l.handles[uid]
	delete(l.handles, uid)
	l.mutex.Unlock()

	if handle != nil {
		handle.cancel()
		<-handle.done
	}
}

type scaleSetScaler struct {
	client     client.Client
	key        client.ObjectKey
	owner      string
	repo       string
	minRunners int
	maxRunners int
}

func (s *scaleSetScaler) metricLabelValues() []string {
	return []string{s.key.Namespace, s.key.Name, s.owner, s.repo}
}

func (s *scaleSetScaler) RecordStatistics(statistics *scaleset.RunnerScaleSetStatistic) {
	assignedJobsGauge.WithLabelValues(s.metricLabelValues()...).Set(float64(statistics.TotalAssignedJobs))
	runningJobsGauge.WithLabelValues(s.metricLabelValues()...).Set(float64(statistics.TotalRunningJobs))
}

func (s *scaleSetScaler) RecordDesiredRunners(count int) {
	desiredRunnersGauge.WithLabelValues(s.metricLabelValues()...).Set(float64(count))
}

func (s *scaleSetScaler) RecordJobStarted(msg *scaleset.JobStarted) {}

func (s *scaleSetScaler) RecordJobCompleted(msg *scaleset.JobCompleted) {}

func (s *scaleSetScaler) HandleJobStarted(ctx context.Context, jobInfo *scaleset.JobStarted) error {
	return nil
}

func (s *scaleSetScaler) HandleJobCompleted(ctx context.Context, jobInfo *scaleset.JobCompleted) error {
	return nil
}

func (s *scaleSetScaler) HandleDesiredRunnerCount(ctx context.Context, count int) (int, error) {
	desired := min(s.maxRunners, s.minRunners+count)

	scaleSet := &garV1.ScaleSet{}
	if err := s.client.Get(ctx, s.key, scaleSet); err != nil {
		return 0, err
	}

	// Patched rather than updated because a conflict would propagate out of runner.Run and stop the poll loop for good.
	// Patched unconditionally because this client reads through the informer cache, which can still hold the value the reconciler reset when the session started.
	patch := client.RawPatch(types.MergePatchType, fmt.Appendf(nil, `{"status":{"desiredRunners":%d}}`, desired))
	if err := s.client.Status().Patch(ctx, scaleSet, patch); err != nil {
		return 0, err
	}

	return desired, nil
}

func DeleteSeries(namespace string, name string) {
	series := prometheus.Labels{"namespace": namespace, "name": name}
	assignedJobsGauge.DeletePartialMatch(series)
	runningJobsGauge.DeletePartialMatch(series)
	desiredRunnersGauge.DeletePartialMatch(series)
}
