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

type SnapshotReconciler struct {
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

func (r *SnapshotReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	snapshot := &ssV1.Snapshot{}
	if err := r.Get(ctx, req.NamespacedName, snapshot); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	if snapshot.Status.ObservedGeneration >= snapshot.Generation {
		return ctrl.Result{}, nil
	}

	snapshot.Status.ObservedGeneration = snapshot.Generation
	if err := r.Status().Update(ctx, snapshot); err != nil {
		return ctrl.Result{}, err
	}

	if r.Distributed {
		if err := r.createJob(ctx, snapshot); err != nil {
			return ctrl.Result{}, err
		}
	} else {
		if err := r.processSnapshot(ctx, snapshot); err != nil {
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

func (r *SnapshotReconciler) processSnapshot(ctx context.Context, snapshot *ssV1.Snapshot) error {
	var baselineResult *capture.CaptureResult
	var targetResult *capture.CaptureResult

	captureOptions := capture.CaptureOptions{
		MaskSelectors: snapshot.Spec.MaskSelectors,
		Headers:       snapshot.Spec.Headers,
	}

	{
		eg, ctx := errgroup.WithContext(ctx)

		eg.Go(func() error {
			result, err := r.Capturer.Capture(ctx, snapshot.Spec.Baseline, captureOptions)
			if err != nil {
				return xerrors.Errorf("failed to capture baseline screenshot: %w", err)
			}
			baselineResult = result
			return nil
		})

		eg.Go(func() error {
			result, err := r.Capturer.Capture(ctx, snapshot.Spec.Target, captureOptions)
			if err != nil {
				return xerrors.Errorf("failed to capture target screenshot: %w", err)
			}
			targetResult = result
			return nil
		})

		if err := eg.Wait(); err != nil {
			return err
		}
	}

	diffImage, diffAmount, err := r.generateDiff(baselineResult.Screenshot, targetResult.Screenshot, snapshot.Spec.ScreenshotDiffFormat)
	if err != nil {
		return xerrors.Errorf("failed to generate diff: %w", err)
	}

	htmlDiff, htmlDiffAmount, err := r.generateHTMLDiff(baselineResult.HTML, targetResult.HTML, snapshot.Spec.HTMLDiffFormat)
	if err != nil {
		return xerrors.Errorf("failed to generate HTML diff: %w", err)
	}
	var baselineURL string
	var targetURL string
	var baselineHTMLURL string
	var targetHTMLURL string
	var diffURL string
	var htmlDiffURL string

	{
		eg, ctx := errgroup.WithContext(ctx)

		eg.Go(func() error {
			imageURL, htmlURL, err := r.uploadCapture(ctx, snapshot.Spec.Baseline, baselineResult)
			if err != nil {
				return err
			}
			baselineURL = imageURL
			baselineHTMLURL = htmlURL
			return nil
		})

		eg.Go(func() error {
			imageURL, htmlURL, err := r.uploadCapture(ctx, snapshot.Spec.Target, targetResult)
			if err != nil {
				return err
			}
			targetURL = imageURL
			targetHTMLURL = htmlURL
			return nil
		})

		eg.Go(func() error {
			timestamp := time.Now().Format("20060102150405")

			h := sha256.New()
			h.Write([]byte(snapshot.Spec.Baseline + snapshot.Spec.Target))
			hash := fmt.Sprintf("%x", h.Sum(nil))[:16]

			diffKey := fmt.Sprintf("Snapshot/diff/%s/%s.jpeg", hash, timestamp)

			url, err := r.Storage.Put(ctx, diffKey, diffImage)
			if err != nil {
				return xerrors.Errorf("failed to upload diff image: %w", err)
			}
			diffURL = url
			return nil
		})

		eg.Go(func() error {
			timestamp := time.Now().Format("20060102150405")

			h := sha256.New()
			h.Write([]byte(snapshot.Spec.Baseline + snapshot.Spec.Target))
			hash := fmt.Sprintf("%x", h.Sum(nil))[:16]

			htmlDiffKey := fmt.Sprintf("Snapshot/diff/%s/%s.txt", hash, timestamp)

			url, err := r.Storage.Put(ctx, htmlDiffKey, htmlDiff)
			if err != nil {
				return xerrors.Errorf("failed to upload HTML diff: %w", err)
			}
			htmlDiffURL = url
			return nil
		})

		if err := eg.Wait(); err != nil {
			return err
		}
	}

	if err := r.updateSnapshotStatus(ctx, snapshot, baselineURL, targetURL, baselineHTMLURL, targetHTMLURL, diffURL, diffAmount, htmlDiffURL, htmlDiffAmount); err != nil {
		return err
	}
	r.Recorder.Eventf(snapshot, coreV1.EventTypeNormal, "SnapshotCompleted", "Snapshot completed successfully: %q (screenshot difference: %.2f%%, HTML difference: %.2f%%)", snapshot.Name, diffAmount*100, htmlDiffAmount*100)

	return nil
}

func (r *SnapshotReconciler) uploadCapture(ctx context.Context, url string, result *capture.CaptureResult) (string, string, error) {
	var imageURL string
	var htmlURL string
	{
		eg, ctx := errgroup.WithContext(ctx)

		timestamp := time.Now().Format("20060102150405")

		h := sha256.New()
		h.Write([]byte(url))
		urlHash := fmt.Sprintf("%x", h.Sum(nil))[:16]

		baseKey := fmt.Sprintf("Snapshot/capture/%s/%s", urlHash, timestamp)

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

		if err := eg.Wait(); err != nil {
			return "", "", err
		}
	}

	return imageURL, htmlURL, nil
}

func (r *SnapshotReconciler) updateSnapshotStatus(ctx context.Context, snapshot *ssV1.Snapshot, baselineURL string, targetURL string, baselineHTMLURL string, targetHTMLURL string, diffURL string, diffAmount float64, htmlDiffURL string, htmlDiffAmount float64) error {
	now := metaV1.Now()
	snapshot.Status.BaselineURL = baselineURL
	snapshot.Status.TargetURL = targetURL
	snapshot.Status.BaselineHTMLURL = baselineHTMLURL
	snapshot.Status.TargetHTMLURL = targetHTMLURL
	snapshot.Status.ScreenshotDiffURL = diffURL
	snapshot.Status.ScreenshotDiffAmount = diffAmount
	snapshot.Status.HTMLDiffURL = htmlDiffURL
	snapshot.Status.HTMLDiffAmount = htmlDiffAmount
	snapshot.Status.LastSnapshotTime = &now

	if err := r.Status().Update(ctx, snapshot); err != nil {
		return xerrors.Errorf("failed to update snapshot status: %w", err)
	}
	return nil
}

func (r *SnapshotReconciler) generateDiff(baselineData []byte, targetData []byte, format string) ([]byte, float64, error) {
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

func (r *SnapshotReconciler) generateHTMLDiff(baselineHTML []byte, targetHTML []byte, format string) ([]byte, float64, error) {
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

func (r *SnapshotReconciler) createJob(ctx context.Context, snapshot *ssV1.Snapshot) error {
	jobName := fmt.Sprintf("snapshot-%s-%d", snapshot.Name, time.Now().Unix())

	args := []string{
		snapshot.Spec.Baseline,
		snapshot.Spec.Target,
		"--screenshot-diff-format", snapshot.Spec.ScreenshotDiffFormat,
		"--html-diff-format", snapshot.Spec.HTMLDiffFormat,
		"--callback", fmt.Sprintf("http://%s/api/%s/%s/%s/%s/%s/artifacts", r.DistributedCallbackHost, snapshot.Namespace, ssV1.GroupVersion.Group, ssV1.GroupVersion.Version, "snapshot", snapshot.Name),
	}

	if len(snapshot.Spec.MaskSelectors) > 0 {
		args = append(args, "--mask-selectors", strings.Join(snapshot.Spec.MaskSelectors, ","))
	}

	for key, value := range snapshot.Spec.Headers {
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

	job := &batchV1.Job{
		ObjectMeta: metaV1.ObjectMeta{
			Name:      jobName,
			Namespace: snapshot.Namespace,
		},
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
	}

	if err := controllerutil.SetControllerReference(snapshot, job, r.Scheme); err != nil {
		return xerrors.Errorf("failed to set controller reference: %w", err)
	}

	if err := r.Create(ctx, job); err != nil {
		if apierrors.IsAlreadyExists(err) {
			r.Log.Info("Job already exists", "job", jobName)
			return nil
		}
		return xerrors.Errorf("failed to create job: %w", err)
	}

	r.Recorder.Eventf(snapshot, coreV1.EventTypeNormal, "JobCreated", "Created job %s for snapshot", jobName)
	return nil
}

func (r *SnapshotReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&ssV1.Snapshot{}).
		WithEventFilter(predicate.GenerationChangedPredicate{}).
		WithOptions(controller.Options{MaxConcurrentReconciles: 1}).
		Complete(r)
}
