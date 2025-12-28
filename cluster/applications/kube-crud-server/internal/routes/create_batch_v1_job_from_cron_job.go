package routes

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"

	jsonpatch "github.com/evanphx/json-patch"
	batchv1 "k8s.io/api/batch/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
	"k8s.io/client-go/kubernetes"
)

func CreateBatchV1JobFromCronJob(clientset *kubernetes.Clientset) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		namespace := r.PathValue("namespace")
		name := r.PathValue("name")
		from := r.PathValue("from")

		patchType := types.PatchType(r.Header.Get("Content-Type"))

		newJob := &batchv1.Job{}

		cronJob, err := clientset.BatchV1().CronJobs(namespace).Get(r.Context(), from, metav1.GetOptions{})
		if err != nil {
			if apierrors.IsNotFound(err) {
				http.NotFound(w, r)
				return
			}
			slog.Error(fmt.Sprintf("failed to get cronjob: %s", err))
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}

		newJob.Name = name
		newJob.Namespace = namespace
		newJob.Spec = cronJob.Spec.JobTemplate.Spec

		// Set an owner reference to the job template
		newJob.SetOwnerReferences([]metav1.OwnerReference{
			{
				APIVersion:         "v1",
				Kind:               "CronJob",
				Name:               cronJob.Name,
				UID:                cronJob.UID,
				Controller:         func(b bool) *bool { return &b }(true),
				BlockOwnerDeletion: func(b bool) *bool { return &b }(true),
			},
		})

		newJob.Spec.Selector = nil
		if newJob.Spec.Template.ObjectMeta.Labels == nil {
			newJob.Spec.Template.ObjectMeta.Labels = map[string]string{}
		}
		newJob.Spec.Template.Labels["batch.kubernetes.io/job-name"] = newJob.Name
		newJob.Spec.Template.Labels["job-name"] = newJob.Name
		delete(newJob.Spec.Template.Labels, "batch.kubernetes.io/controller-uid")
		delete(newJob.Spec.Template.Labels, "controller-uid")

		// https://kubernetes.io/docs/reference/using-api/api-concepts/#patch-and-apply
		switch patchType {
		case types.JSONPatchType:
			body, err := io.ReadAll(r.Body)
			if err != nil {
				http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
				return
			}

			before, err := json.Marshal(newJob)
			if err != nil {
				slog.Error(fmt.Sprintf("failed to marshal job: %s", err))
				http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
				return
			}

			patch, err := jsonpatch.DecodePatch(body)
			if err != nil {
				slog.Error(fmt.Sprintf("failed to decode patch: %s", err))
				http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
				return
			}

			after, err := patch.Apply(before)
			if err != nil {
				slog.Error(fmt.Sprintf("failed to apply patch: %s", err))
				http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
				return
			}

			if err := json.Unmarshal(after, newJob); err != nil {
				slog.Error(fmt.Sprintf("failed to unmarshal job: %s", err))
				http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
				return
			}
		case types.MergePatchType:
			body, err := io.ReadAll(r.Body)
			if err != nil {
				http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
				return
			}

			before, err := json.Marshal(newJob)
			if err != nil {
				slog.Error(fmt.Sprintf("failed to marshal job: %s", err))
				http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
				return
			}

			after, err := jsonpatch.MergePatch(before, body)
			if err != nil {
				slog.Error(fmt.Sprintf("failed to merge patch: %s", err))
				http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
				return
			}

			if err := json.Unmarshal(after, newJob); err != nil {
				slog.Error(fmt.Sprintf("failed to unmarshal job: %s", err))
				http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
				return
			}
		case types.StrategicMergePatchType:
			body, err := io.ReadAll(r.Body)
			if err != nil {
				http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
				return
			}

			newJob.Kind = "Job"
			newJob.APIVersion = "batch/v1"

			before, err := json.Marshal(newJob)
			if err != nil {
				slog.Error(fmt.Sprintf("failed to marshal job: %s", err))
				http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
				return
			}

			after, err := strategicpatch.StrategicMergePatch(before, body, newJob)
			if err != nil {
				slog.Error(fmt.Sprintf("failed to merge patch: %s", err))
				http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
				return
			}

			if err := runtime.DecodeInto(unstructured.UnstructuredJSONScheme, after, newJob); err != nil {
				slog.Error(fmt.Sprintf("failed to decode unstructured: %s", err))
				http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
				return
			}
		case types.ApplyPatchType:
			http.Error(w, http.StatusText(http.StatusUnsupportedMediaType), http.StatusUnsupportedMediaType)
			return
		}

		job, err := clientset.BatchV1().Jobs(namespace).Create(r.Context(), newJob, metav1.CreateOptions{})
		if err != nil {
			if apierrors.IsNotFound(err) {
				http.NotFound(w, r)
				return
			}
			slog.Error(fmt.Sprintf("failed to create resource: %s", err))
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}

		b, err := json.Marshal(job)
		if err != nil {
			slog.Error(fmt.Sprintf("failed to marshal json: %s", err))
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(b)
	}
}
