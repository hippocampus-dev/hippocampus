package generate

type Args struct {
	ManifestPath           string `validate:"required,dir"`
	OutputPath             string
	GrafanaURL             string
	LokiDatasourceUID      string
	TempoDatasourceUID     string
	PyroscopeDatasourceUID string
}

func DefaultArgs() *Args {
	return &Args{
		GrafanaURL:             "https://grafana.minikube.127.0.0.1.nip.io",
		LokiDatasourceUID:      "loki",
		TempoDatasourceUID:     "tempo",
		PyroscopeDatasourceUID: "pyroscope",
	}
}
