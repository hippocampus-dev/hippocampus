package controllers

import (
	"context"
	"sync"
	"time"

	garV1 "github-actions-runner-controller/api/v1"
	"github-actions-runner-controller/internal/github"

	"github.com/go-logr/logr"
	batchV1 "k8s.io/api/batch/v1"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metaV1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	toolscache "k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/retry"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	// Nothing enqueues an assigned job whose run is still going, and the poll below is what carries it to the cancel and the rerun.
	workflowRunPollInterval = 10 * time.Second
	// Bound of this controller rather than of GitHub: an interruption repeating for the same reason would ask forever, and several interrupted runners of one run each spend one attempt.
	maxWorkflowRunAttempts = 5
	// A container the kubelet stops with SIGTERM ends at this code, which is what tells an interruption from an out-of-memory kill and from a runner that failed on its own.
	sigtermExitCode = 143
	// Short of the hour an installation token lasts, since the poll above would otherwise exchange one against the REST API on every pass for as long as a run takes.
	githubAppClientLifetime = 50 * time.Minute
)

type AssignedJobReconciler struct {
	client.Client
	Log                     logr.Logger
	Recorder                record.EventRecorder
	APIReader               client.Reader
	GitHubAppClientId       string
	GitHubAppInstallationId string
	GitHubAppPrivateKey     string

	githubAppClientMutex    sync.Mutex
	githubAppClient         *github.Client
	githubAppClientIssuedAt time.Time
}

func (r *AssignedJobReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	assignedJob := &garV1.AssignedJob{}
	if err := r.Get(ctx, req.NamespacedName, assignedJob); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	// The runner Job carries the same name and outlives its pod by the TTL buildRunnerJob sets.
	var runnerJob batchV1.Job
	runnerJobLives := true
	if err := r.APIReader.Get(ctx, req.NamespacedName, &runnerJob); apierrors.IsNotFound(err) {
		runnerJobLives = false
	} else if err != nil {
		return ctrl.Result{}, err
	}

	if !assignedJob.Spec.Interrupted {
		// A failed Job is kept until its TTL removes it, because the pod event carrying the exit code can arrive after the Job controller counted the failure.
		if runnerJobLives && runnerJob.Status.Succeeded == 0 {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, client.IgnoreNotFound(r.Delete(ctx, assignedJob))
	}

	// Nothing but the listener names the run this runner took, and it reports a job seconds after GitHub assigned it.
	if assignedJob.Spec.WorkflowRunID == 0 {
		if !runnerJobLives {
			return ctrl.Result{}, client.IgnoreNotFound(r.Delete(ctx, assignedJob))
		}
		return ctrl.Result{RequeueAfter: workflowRunPollInterval}, nil
	}

	scaleSet := &garV1.ScaleSet{}
	owner := metaV1.GetControllerOf(assignedJob)
	if owner == nil {
		return ctrl.Result{}, client.IgnoreNotFound(r.Delete(ctx, assignedJob))
	}
	if err := r.Get(ctx, client.ObjectKey{Namespace: req.Namespace, Name: owner.Name}, scaleSet); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, client.IgnoreNotFound(r.Delete(ctx, assignedJob))
		}
		return ctrl.Result{}, err
	}

	githubClient, err := r.newGitHubClient(ctx, scaleSet)
	if err != nil {
		return ctrl.Result{}, err
	}

	run, err := githubClient.GetWorkflowRun(ctx, assignedJob.Spec.Owner, assignedJob.Spec.Repo, assignedJob.Spec.WorkflowRunID)
	if err != nil {
		return ctrl.Result{}, err
	}
	if run == nil {
		return ctrl.Result{}, client.IgnoreNotFound(r.Delete(ctx, assignedJob))
	}
	if run.RunAttempt >= maxWorkflowRunAttempts {
		r.Recorder.Eventf(scaleSet, v1.EventTypeWarning, "FailedRerun", "Left workflow run %d at attempt %d, which reached the %d this controller allows", assignedJob.Spec.WorkflowRunID, run.RunAttempt, maxWorkflowRunAttempts)
		return ctrl.Result{}, client.IgnoreNotFound(r.Delete(ctx, assignedJob))
	}

	workflowJobs, err := githubClient.ListWorkflowRunJobs(ctx, assignedJob.Spec.Owner, assignedJob.Spec.Repo, assignedJob.Spec.WorkflowRunID)
	if err != nil {
		return ctrl.Result{}, err
	}
	var workflowJob *github.WorkflowJob
	for i := range workflowJobs {
		if workflowJobs[i].RunnerName == req.Name {
			workflowJob = &workflowJobs[i]
			break
		}
	}
	// An attempt that re-ran this job took the runner name with it, which leaves nothing here to address a rerun to.
	if workflowJob == nil {
		return ctrl.Result{}, client.IgnoreNotFound(r.Delete(ctx, assignedJob))
	}

	if run.Status != github.StatusCompleted {
		// Held open by something other than this job once this one has a conclusion, which the rerun another interrupted runner of this run started is, and cancelling then takes that rerun down.
		if workflowJob.Status != github.StatusInProgress {
			return ctrl.Result{RequeueAfter: workflowRunPollInterval}, nil
		}

		var assignedJobs garV1.AssignedJobList
		if err := r.List(ctx, &assignedJobs, client.InNamespace(req.Namespace)); err != nil {
			return ctrl.Result{}, err
		}
		interrupted := map[string]struct{}{}
		for i := range assignedJobs.Items {
			if assignedJobs.Items[i].Spec.Interrupted && assignedJobs.Items[i].Spec.WorkflowRunID == assignedJob.Spec.WorkflowRunID {
				interrupted[assignedJobs.Items[i].Name] = struct{}{}
			}
		}
		for i := range workflowJobs {
			if workflowJobs[i].Status != github.StatusInProgress || workflowJobs[i].RunnerName == req.Name {
				continue
			}
			// A job whose runner this controller recorded as interrupted holds nobody's work, so waiting for it would leave two interrupted runners of one run waiting for each other.
			if _, ok := interrupted[workflowJobs[i].RunnerName]; ok {
				continue
			}
			return ctrl.Result{RequeueAfter: workflowRunPollInterval}, nil
		}

		cancelled, refusal, err := githubClient.ForceCancelWorkflowRun(ctx, assignedJob.Spec.Owner, assignedJob.Spec.Repo, assignedJob.Spec.WorkflowRunID)
		if err != nil {
			return ctrl.Result{}, err
		}
		if !cancelled {
			// Carries what GitHub answered because a credential missing write on Actions and a run it will not cancel yet are both refused here, and only the first of them needs anyone's attention.
			r.Recorder.Eventf(scaleSet, v1.EventTypeWarning, "FailedCancel", "GitHub refused to cancel workflow run %d, which holds the job taken by runner %q: %s", assignedJob.Spec.WorkflowRunID, req.Name, refusal)
		}
		return ctrl.Result{RequeueAfter: workflowRunPollInterval}, nil
	}

	rerun, refusal, err := githubClient.RerunWorkflowJob(ctx, assignedJob.Spec.Owner, assignedJob.Spec.Repo, workflowJob.ID)
	if err != nil {
		return ctrl.Result{}, err
	}
	if rerun {
		r.Recorder.Eventf(scaleSet, v1.EventTypeNormal, "SuccessfulRerun", "Rerun workflow job %d of run %d taken by runner %q", workflowJob.ID, assignedJob.Spec.WorkflowRunID, req.Name)
	} else {
		// Carries what GitHub answered because a credential missing write on Actions and a job identifier a newer attempt replaced are both refused here.
		r.Recorder.Eventf(scaleSet, v1.EventTypeWarning, "FailedRerun", "GitHub refused the rerun of workflow job %d of run %d: %s", workflowJob.ID, assignedJob.Spec.WorkflowRunID, refusal)
	}
	return ctrl.Result{}, client.IgnoreNotFound(r.Delete(ctx, assignedJob))
}

func (r *AssignedJobReconciler) newGitHubClient(ctx context.Context, scaleSet *garV1.ScaleSet) (*github.Client, error) {
	if scaleSet.Spec.TokenSecretKeyRef != nil {
		var tokenSecret v1.Secret
		if err := r.Get(
			ctx,
			client.ObjectKey{
				Name:      scaleSet.Spec.TokenSecretKeyRef.Name,
				Namespace: scaleSet.Namespace,
			},
			&tokenSecret,
		); err != nil {
			return nil, err
		}
		return github.NewClientWithPersonalAccessToken(string(tokenSecret.Data[scaleSet.Spec.TokenSecretKeyRef.Key])), nil
	}

	r.githubAppClientMutex.Lock()
	defer r.githubAppClientMutex.Unlock()

	if r.githubAppClient != nil && time.Since(r.githubAppClientIssuedAt) < githubAppClientLifetime {
		return r.githubAppClient, nil
	}
	githubAppClient, err := github.NewClientWithGitHubApp(ctx, r.GitHubAppClientId, r.GitHubAppInstallationId, r.GitHubAppPrivateKey)
	if err != nil {
		return nil, err
	}
	r.githubAppClient = githubAppClient
	r.githubAppClientIssuedAt = time.Now()
	return githubAppClient, nil
}

func (r *AssignedJobReconciler) recordInterruption(ctx context.Context, object any) {
	if tombstone, ok := object.(toolscache.DeletedFinalStateUnknown); ok {
		object = tombstone.Obj
	}
	pod, ok := object.(*v1.Pod)
	if !ok || pod.Labels["app.kubernetes.io/component"] != "runner" {
		return
	}
	runnerName := pod.Labels["batch.kubernetes.io/job-name"]
	if runnerName == "" {
		return
	}
	interrupted := false
	for i := range pod.Status.ContainerStatuses {
		terminated := pod.Status.ContainerStatuses[i].State.Terminated
		if pod.Status.ContainerStatuses[i].Name == "runner" && terminated != nil && terminated.ExitCode == sigtermExitCode {
			interrupted = true
		}
	}
	if !interrupted {
		return
	}

	assignedJob := &garV1.AssignedJob{ObjectMeta: metaV1.ObjectMeta{Namespace: pod.Namespace, Name: runnerName}}
	// Created here as well as updated, since the listener creates the same object when GitHub reports the job before the pod ends, and loses the race to it through the cache this reads.
	if err := retry.OnError(retry.DefaultRetry, func(err error) bool {
		return apierrors.IsConflict(err) || apierrors.IsAlreadyExists(err)
	}, func() error {
		_, err := controllerutil.CreateOrUpdate(ctx, r.Client, assignedJob, func() error {
			assignedJob.OwnerReferences = []metaV1.OwnerReference{{
				APIVersion:         garV1.GroupVersion.String(),
				Kind:               "ScaleSet",
				Name:               pod.Labels["app.kubernetes.io/name"],
				UID:                types.UID(pod.Labels[scaleSetUIDLabel]),
				Controller:         ptr.To(true),
				BlockOwnerDeletion: ptr.To(true),
			}}
			assignedJob.Spec.Interrupted = true
			return nil
		})
		return err
	}); err != nil {
		r.Log.WithValues("assignedjob", client.ObjectKeyFromObject(assignedJob)).Error(err, "failed to record interruption")
	}
}

func (r *AssignedJobReconciler) SetupWithManager(mgr ctrl.Manager) error {
	r.APIReader = mgr.GetAPIReader()

	// Read from the event rather than from a reconcile because the exit code lives on the pod alone and the pod object is removed about a second after the container ends.
	// Both replicas run this, since an informer does not wait for the leader lease, and the write it makes is the same one either of them would make.
	informer, err := mgr.GetCache().GetInformer(context.Background(), &v1.Pod{})
	if err != nil {
		return err
	}
	if _, err := informer.AddEventHandler(toolscache.ResourceEventHandlerFuncs{
		UpdateFunc: func(_ any, object any) { r.recordInterruption(context.Background(), object) },
		DeleteFunc: func(object any) { r.recordInterruption(context.Background(), object) },
	}); err != nil {
		return err
	}

	// The runner Job reaching a terminal state is what retires an assigned job nobody interrupted, and nothing else enqueues one.
	return ctrl.NewControllerManagedBy(mgr).
		For(&garV1.AssignedJob{}).
		Watches(&batchV1.Job{}, handler.EnqueueRequestsFromMapFunc(func(_ context.Context, object client.Object) []reconcile.Request {
			return []reconcile.Request{{NamespacedName: client.ObjectKeyFromObject(object)}}
		})).
		WithOptions(controller.Options{MaxConcurrentReconciles: reconcileConcurrency}).
		Complete(r)
}
