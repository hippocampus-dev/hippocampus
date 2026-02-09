package registry

type Args struct {
	ManifestPath           string `validate:"required,dir"`
	OutputPath             string
	GrafanaURL             string
	LokiDatasourceUID      string
	TempoDatasourceUID     string
	PyroscopeDatasourceUID string
}

func DefaultArgs() *Args {
	return &Args{}
}
