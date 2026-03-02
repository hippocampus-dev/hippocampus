package handler

import (
	"errors"
	"time"

	"github.com/google/go-github/v68/github"
	"golang.org/x/xerrors"
	"k8s.io/client-go/kubernetes"
)

type Alert struct {
	Status       string            `json:"status"`
	Labels       map[string]string `json:"labels"`
	Annotations  map[string]string `json:"annotations"`
	StartsAt     time.Time         `json:"startsAt"`
	EndsAt       time.Time         `json:"endsAt"`
	GeneratorURL string            `json:"generatorURL"`
}

type AlertManagerRequest struct {
	Receiver          string            `json:"receiver"`
	Status            string            `json:"status"`
	Alerts            []Alert           `json:"alerts"`
	GroupLabels       map[string]string `json:"groupLabels"`
	CommonLabels      map[string]string `json:"commonLabels"`
	CommonAnnotations map[string]string `json:"commonAnnotations"`
	ExternalURL       string            `json:"externalURL"`
	Version           string            `json:"version"`
	GroupKey          string            `json:"groupKey"`
}

type Dispatcher struct {
	kubernetes   kubernetes.Interface
	gitHubClient *github.Client
}

func NewDispatcher(kubernetes kubernetes.Interface, gitHubClient *github.Client) *Dispatcher {
	return &Dispatcher{
		kubernetes:   kubernetes,
		gitHubClient: gitHubClient,
	}
}

func (d *Dispatcher) Handle(request *AlertManagerRequest) error {
	h, err := d.Dispatch(request)
	if err != nil {
		var notFoundError *NotFoundError
		switch {
		case errors.As(err, &notFoundError):
			return nil
		default:
			return err
		}
	}
	return h.Call(request)
}

func (d *Dispatcher) Dispatch(request *AlertManagerRequest) (Handler, error) {
	if request.Status == "firing" {
		alertname, ok := request.CommonLabels["alertname"]
		if !ok {
			return nil, xerrors.New("alertname label is not found")
		}

		severity := request.CommonLabels["severity"]
		if severity == "critical" {
			return NewCriticalAlertHandler(d.gitHubClient), nil
		}

		switch alertname {
		case "RunOutContainerMemory":
			return NewRunOutContainerMemoryHandler(d.kubernetes, time.Millisecond*100, time.Minute*5), nil
		default:
			return nil, NewNotFoundError(xerrors.Errorf("no handler was found for alertname: %s", alertname))
		}
	}
	return nil, NewNotFoundError(xerrors.Errorf("no handler was found for request.Status: %s", request.Status))
}
