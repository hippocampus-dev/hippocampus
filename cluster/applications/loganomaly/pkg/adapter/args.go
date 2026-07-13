package adapter

type Args struct {
	GrafanaBase string `validate:"required,url"`
}

func DefaultArgs() *Args {
	return &Args{
		GrafanaBase: "https://grafana.kaidotio.dev",
	}
}
