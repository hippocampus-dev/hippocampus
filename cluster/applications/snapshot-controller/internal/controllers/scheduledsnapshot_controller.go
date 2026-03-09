package controllers

import (
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"image/jpeg"
	"os"
	ssV1 "snapshot-controller/api/v1"
	"snapshot-controller/internal/capture"
	diffimage "snapshot-controller/internal/diff/image"
	difftext "snapshot-controller/internal/diff/text"
	"snapshot-controller/internal/storage"
	"strings"
	"time"

	"github.com/go-logr/logr"
	"github.com/robfig/cron/v3"
	"golang.org/x/sync/errgroup"
	"golang.org/x/xerrors"
	batchV1 "k8s.io/api/batch/v1"
	coreV1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metaV1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

type ScheduledSnapshotReconciler struct {
	client.Client
	Log      logr.Logger
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
	Capturer capture.Capturer
	Storage  storage.Storage

	Distributed             bool
	DistributedCallbackHost string
	DistributedWorkerImage  string
}

func (r *ScheduledSnapshotReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	scheduledSnapshot := &ssV1.ScheduledSnapshot{}
	if err := r.Get(ctx, req.NamespacedName, scheduledSnapshot); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	if r.Distributed {
		if err := r.createOrUpdateCronJob(ctx, scheduledSnapshot); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	schedule, err := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow).Parse(scheduledSnapshot.Spec.Schedule)
	if err != nil {
		return ctrl.Result{}, err
	}

	nextRun := schedule.Next(time.Now().Add(-1 * time.Minute))
	if scheduledSnapshot.Status.LastSnapshotTime != nil {
		nextRun = schedule.Next(scheduledSnapshot.Status.LastSnapshotTime.Time)
	}
	now := time.Now()

	if now.Before(nextRun) {
		requeueAfter := nextRun.Sub(now)
		return ctrl.Result{RequeueAfter: requeueAfter}, nil
	}

	if err := r.processSnapshot(ctx, scheduledSnapshot); err != nil {
		return ctrl.Result{}, err
	}

	nextRun = schedule.Next(now)
	return ctrl.Result{RequeueAfter: nextRun.Sub(now)}, nil
}

func (r *ScheduledSnapshotReconciler) processSnapshot(ctx context.Context, scheduledSnapshot *ssV1.ScheduledSnapshot) error {
	captureOptions := capture.CaptureOptions{
		MaskSelectors: scheduledSnapshot.Spec.MaskSelectors,
		Headers:       scheduledSnapshot.Spec.Headers,
	}

	result, err := r.Capturer.Capture(ctx, scheduledSnapshot.Spec.Target, captureOptions)
	if err != nil {
		return xerrors.Errorf("failed to download screenshot: %w", err)
	}

	var diffImage []byte
	var diffAmount float64
	var htmlDiff []byte
	var htmlDiffAmount float64
	if scheduledSnapshot.Status.BaselineURL != "" {
		baselineData, err := r.Storage.Get(ctx, scheduledSnapshot.Status.BaselineURL)
		if err != nil {
			return xerrors.Errorf("failed to generate diff: %w", err)
		}

		diffImage, diffAmount, err = r.generateDiff(baselineData, result.Screenshot, scheduledSnapshot.Spec.ScreenshotDiffFormat)
		if err != nil {
			return xerrors.Errorf("failed to generate diff: %w", err)
		}
	}

	if scheduledSnapshot.Status.BaselineHTMLURL != "" {
		baselineHTMLData, err := r.Storage.Get(ctx, scheduledSnapshot.Status.BaselineHTMLURL)
		if err != nil {
			return xerrors.Errorf("failed to download baseline HTML: %w", err)
		}

		htmlDiff, htmlDiffAmount, err = r.generateHTMLDiff(baselineHTMLData, result.HTML, scheduledSnapshot.Spec.HTMLDiffFormat)
		if err != nil {
			return xerrors.Errorf("failed to generate HTML diff: %w", err)
		}
	}

	var imageURL string
	var htmlURL string
	var diffURL string
	var htmlDiffURL string

	{
		eg, ctx := errgroup.WithContext(ctx)

		timestamp := time.Now().Format("20060102150405")

		h := sha256.New()
		h.Write([]byte(scheduledSnapshot.Spec.Target))
		urlHash := fmt.Sprintf("%x", h.Sum(nil))[:16]

		baseKey := fmt.Sprintf("ScheduledSnapshot/capture/%s/%s", urlHash, timestamp)

		eg.Go(func() error {
			imageKey := baseKey + ".jpeg"
			path, err := r.Storage.Put(ctx, imageKey, result.Screenshot)
			if err != nil {
				return xerrors.Errorf("failed to upload screenshot: %w", err)
			}
			imageURL = path
			return nil
		})

		eg.Go(func() error {
			htmlKey := baseKey + ".html"
			path, err := r.Storage.Put(ctx, htmlKey, result.HTML)
			if err != nil {
				return xerrors.Errorf("failed to upload HTML: %w", err)
			}
			htmlURL = path
			return nil
		})

		if diffImage != nil {
			eg.Go(func() error {
				h := sha256.New()
				h.Write([]byte(scheduledSnapshot.Status.BaselineURL + scheduledSnapshot.Spec.Target))
				hash := fmt.Sprintf("%x", h.Sum(nil))[:16]

				diffKey := fmt.Sprintf("Snapshot/diff/%s/%s.jpeg", hash, timestamp)

				url, err := r.Storage.Put(ctx, diffKey, diffImage)
				if err != nil {
					return xerrors.Errorf("failed to upload diff image: %w", err)
				}
				diffURL = url
				return nil
			})
		}

		if htmlDiff != nil {
			eg.Go(func() error {
				h := sha256.New()
				h.Write([]byte(scheduledSnapshot.Status.BaselineURL + scheduledSnapshot.Spec.Target))
				hash := fmt.Sprintf("%x", h.Sum(nil))[:16]

				htmlDiffKey := fmt.Sprintf("Snapshot/diff/%s/%s.txt", hash, timestamp)

				url, err := r.Storage.Put(ctx, htmlDiffKey, htmlDiff)
				if err != nil {
					return xerrors.Errorf("failed to upload HTML diff: %w", err)
				}
				htmlDiffURL = url
				return nil
			})
		}

		if err := eg.Wait(); err != nil {
			return err
		}
	}

	if err := r.updateScheduledSnapshotStatus(ctx, scheduledSnapshot, imageURL, htmlURL, diffURL, diffAmount, htmlDiffURL, htmlDiffAmount); err != nil {
		return err
	}
	r.Recorder.Eventf(scheduledSnapshot, coreV1.EventTypeNormal, "SnapshotCompleted", "Scheduled snapshot completed successfully: %q (screenshot difference: %.2f%%, HTML difference: %.2f%%)", scheduledSnapshot.Name, diffAmount*100, htmlDiffAmount*100)

	return nil
}

func (r *ScheduledSnapshotReconciler) generateDiff(baselineData []byte, targetData []byte, format string) ([]byte, float64, error) {
	baselineImage, err := jpeg.Decode(bytes.NewReader(baselineData))
	if err != nil {
		return nil, 0.0, xerrors.Errorf("failed to decode baseline image: %w", err)
	}

	targetImage, err := jpeg.Decode(bytes.NewReader(targetData))
	if err != nil {
		return nil, 0.0, xerrors.Errorf("failed to decode target image: %w", err)
	}

	var differ diffimage.Differ
	switch format {
	case "rectangle":
		differ = diffimage.NewRectangleDiff()
	case "pixel":
		differ = diffimage.NewPixelDiff(0.1)
	default:
		return nil, 0.0, xerrors.Errorf("unknown diff format: %s", format)
	}

	diffResult := differ.Calculate(baselineImage, targetImage)

	var buffer bytes.Buffer
	err = jpeg.Encode(&buffer, diffResult.Image, &jpeg.Options{Quality: 90})
	if err != nil {
		return nil, 0.0, xerrors.Errorf("failed to encode diff image: %w", err)
	}

	return buffer.Bytes(), diffResult.DiffAmount, nil
}

func (r *ScheduledSnapshotReconciler) generateHTMLDiff(baselineHTML []byte, targetHTML []byte, format string) ([]byte, float64, error) {
	var differ difftext.Differ
	switch format {
	case "line":
		differ = difftext.NewLineDiff()
	default:
		return nil, 0.0, xerrors.Errorf("unknown HTML diff format: %s", format)
	}

	diffResult, err := differ.Calculate(baselineHTML, targetHTML)

	if err != nil {
		return nil, 0.0, xerrors.Errorf("failed to calculate HTML diff: %w", err)
	}
	return diffResult.Diff, diffResult.DiffAmount, nil
}

func (r *ScheduledSnapshotReconciler) updateScheduledSnapshotStatus(ctx context.Context, scheduledSnapshot *ssV1.ScheduledSnapshot, imageURL string, htmlURL string, diffURL string, diffAmount float64, htmlDiffURL string, htmlDiffAmount float64) error {
	now := metaV1.Now()

	if scheduledSnapshot.Status.TargetURL != "" {
		scheduledSnapshot.Status.BaselineURL = scheduledSnapshot.Status.TargetURL
	}
	if scheduledSnapshot.Status.TargetHTMLURL != "" {
		scheduledSnapshot.Status.BaselineHTMLURL = scheduledSnapshot.Status.TargetHTMLURL
	}

	scheduledSnapshot.Status.TargetURL = imageURL
	scheduledSnapshot.Status.TargetHTMLURL = htmlURL
	scheduledSnapshot.Status.ScreenshotDiffURL = diffURL
	scheduledSnapshot.Status.ScreenshotDiffAmount = diffAmount
	scheduledSnapshot.Status.HTMLDiffURL = htmlDiffURL
	scheduledSnapshot.Status.HTMLDiffAmount = htmlDiffAmount
	scheduledSnapshot.Status.LastSnapshotTime = &now

	if err := r.Status().Update(ctx, scheduledSnapshot); err != nil {
		return xerrors.Errorf("failed to update scheduled snapshot status: %w", err)
	}
	return nil
}

func (r *ScheduledSnapshotReconciler) createOrUpdateCronJob(ctx context.Context, scheduledSnapshot *ssV1.ScheduledSnapshot) error {
	cronJobName := fmt.Sprintf("snapshot-%s", scheduledSnapshot.Name)

	args := []string{
		scheduledSnapshot.Spec.Target,
		scheduledSnapshot.Spec.Target,
		"--screenshot-diff-format", scheduledSnapshot.Spec.ScreenshotDiffFormat,
		"--html-diff-format", scheduledSnapshot.Spec.HTMLDiffFormat,
		"--callback-url", fmt.Sprintf("http://%s/api/%s/%s/%s/%s/%s/artifacts", r.DistributedCallbackHost, scheduledSnapshot.Namespace, ssV1.GroupVersion.Group, ssV1.GroupVersion.Version, "scheduledsnapshot", scheduledSnapshot.Name),
	}

	if len(scheduledSnapshot.Spec.MaskSelectors) > 0 {
		args = append(args, "--mask-selectors", strings.Join(scheduledSnapshot.Spec.MaskSelectors, ","))
	}

	for key, value := range scheduledSnapshot.Spec.Headers {
		args = append(args, "-H", fmt.Sprintf("%s: %s", key, value))
	}

	envVars := []coreV1.EnvVar{
		{
			Name:  "STORAGE_BACKEND",
			Value: "s3",
		},
		{
			Name:  "S3_BUCKET",
			Value: os.Getenv("S3_BUCKET"),
		},
		{
			Name:  "S3_ENDPOINT",
			Value: os.Getenv("S3_ENDPOINT"),
		},
		{
			Name:  "S3_REGION",
			Value: os.Getenv("S3_REGION"),
		},
		{
			Name:  "AWS_ACCESS_KEY_ID",
			Value: os.Getenv("AWS_ACCESS_KEY_ID"),
		},
		{
			Name:  "AWS_SECRET_ACCESS_KEY",
			Value: os.Getenv("AWS_SECRET_ACCESS_KEY"),
		},
		{
			Name:  "CHROME_DEVTOOLS_PROTOCOL_URL",
			Value: os.Getenv("CHROME_DEVTOOLS_PROTOCOL_URL"),
		},
	}

	cronJob := &batchV1.CronJob{
		ObjectMeta: metaV1.ObjectMeta{
			Name:      cronJobName,
			Namespace: scheduledSnapshot.Namespace,
		},
		Spec: batchV1.CronJobSpec{
			Schedule: scheduledSnapshot.Spec.Schedule,
			JobTemplate: batchV1.JobTemplateSpec{
				Spec: batchV1.JobSpec{
					Template: coreV1.PodTemplateSpec{
						Spec: coreV1.PodSpec{
							RestartPolicy: coreV1.RestartPolicyNever,
							Containers: []coreV1.Container{
								{
									Name:  "worker",
									Image: r.DistributedWorkerImage,
									Args:  args,
									Env:   envVars,
								},
							},
						},
					},
				},
			},
		},
	}

	if err := controllerutil.SetControllerReference(scheduledSnapshot, cronJob, r.Scheme); err != nil {
		return xerrors.Errorf("failed to set controller reference: %w", err)
	}

	existingCronJob := &batchV1.CronJob{}
	err := r.Get(ctx, client.ObjectKey{Name: cronJobName, Namespace: scheduledSnapshot.Namespace}, existingCronJob)
	if err != nil {
		if apierrors.IsNotFound(err) {
			if err := r.Create(ctx, cronJob); err != nil {
				return xerrors.Errorf("failed to create cronjob: %w", err)
			}
			r.Recorder.Eventf(scheduledSnapshot, coreV1.EventTypeNormal, "CronJobCreated", "Created CronJob %s", cronJobName)
		} else {
			return xerrors.Errorf("failed to get existing cronjob: %w", err)
		}
	} else {
		existingCronJob.Spec = cronJob.Spec
		if err := r.Update(ctx, existingCronJob); err != nil {
			return xerrors.Errorf("failed to update cronjob: %w", err)
		}
		r.Recorder.Eventf(scheduledSnapshot, coreV1.EventTypeNormal, "CronJobUpdated", "Updated CronJob %s", cronJobName)
	}

	return nil
}

func (r *ScheduledSnapshotReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&ssV1.ScheduledSnapshot{}).
		WithEventFilter(predicate.GenerationChangedPredicate{}).
		WithOptions(controller.Options{MaxConcurrentReconciles: 1}).
		Complete(r)
}
