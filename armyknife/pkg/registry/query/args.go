package query

import "time"

type Args struct {
	Directory               string `validate:"required,dir"`
	GrafanaURL              string `validate:"required,url"`
	AuthorizationListenPort uint   `validate:"max=65535"`
	PrometheusDatasourceUID string
	LokiDatasourceUID       string
	TempoDatasourceUID      string
	PyroscopeDatasourceUID  string
	From                    time.Duration
	To                      time.Duration
	Step                    time.Duration
	Signals                 []string `validate:"dive,oneof=metrics logs traces profiling"`
}

func DefaultArgs() *Args {
	return &Args{
		GrafanaURL:              "https://grafana.minikube.127.0.0.1.nip.io",
		PrometheusDatasourceUID: "prometheus",
		LokiDatasourceUID:       "loki",
		TempoDatasourceUID:      "tempo",
		PyroscopeDatasourceUID:  "pyroscope",
		From:                    30 * time.Minute,
		Step:                    30 * time.Second,
		AuthorizationListenPort: 0,
	}
}
