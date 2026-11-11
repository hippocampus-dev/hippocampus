package adapter

type Args struct {
	GrafanaBase string `validate:"required,url"`
	Repository  string `validate:"required,contains=/,startsnotwith=/,endsnotwith=/,excludes=//"`
}

func DefaultArgs() *Args {
	return &Args{
		GrafanaBase: "https://grafana.kaidotio.dev",
		Repository:  "hippocampus-dev/hippocampus",
	}
}
