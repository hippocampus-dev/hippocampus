package handler

import (
	"time"

	"golang.org/x/xerrors"
	"k8s.io/client-go/kubernetes"
)

type AlertManagerRequest struct {
	Receiver string `json:"receiver"`
	Status   string `json:"status"`
	Alerts   []struct {
		Status       string            `json:"status"`
		Labels       map[string]string `json:"labels"`
		Annotations  map[string]string `json:"annotations"`
		StartsAt     time.Time         `json:"startsAt"`
		EndsAt       time.Time         `json:"endsAt"`
		GeneratorURL string            `json:"generatorURL"`
	} `json:"alerts"`
	GroupLabels       map[string]string `json:"groupLabels"`
	CommonLabels      map[string]string `json:"commonLabels"`
	CommonAnnotations map[string]string `json:"commonAnnotations"`
	ExternalURL       string            `json:"externalURL"`
	Version           string            `json:"version"`
	GroupKey          string            `json:"groupKey"`
}

func Handle(client kubernetes.Interface, request *AlertManagerRequest) error {
	h, err := Dispatch(client, request)
	if err != nil {
		switch err.(type) {
		case *NotFoundError:
			return nil
		default:
			return err
		}
	}
	return h.Call(request)
}

func Dispatch(client kubernetes.Interface, request *AlertManagerRequest) (Handler, error) {
	if request.Status == "firing" {
		alertname, ok := request.CommonLabels["alertname"]
		if !ok {
			return nil, xerrors.New("alertname label is not found")
		}
		switch alertname {
		case "RunOutContainerMemory":
			return NewRunOutContainerMemoryHandler(client, time.Millisecond*100, time.Minute*5), nil
		default:
			return nil, NewNotFoundError(xerrors.Errorf("no handler was found for alertname: %s", alertname))
		}
	}
	return nil, NewNotFoundError(xerrors.Errorf("no handler was found for request.Status: %s", request.Status))
}
