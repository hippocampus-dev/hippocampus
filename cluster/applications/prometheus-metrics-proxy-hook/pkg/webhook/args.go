package webhook

type Args struct {
	Host                    string `validate:"required,ip"`
	Port                    int    `validate:"required,gt=0,lte=65535"`
	CertDir                 string `validate:"required,dir"`
	MetricsAddr             string `validate:"required,tcp_addr"`
	EnableHTTP2             bool
	SecureMetrics           bool
	ProbeAddr               string `validate:"required,tcp_addr"`
	SidecarImage            string `validate:"required"`
	EnableSidecarContainers bool
}

func DefaultArgs() *Args {
	return &Args{
		Host:                    "0.0.0.0",
		Port:                    9443,
		CertDir:                 "/var/k8s-webhook-server/serving-certs",
		MetricsAddr:             "0.0.0.0:8080",
		EnableHTTP2:             false,
		SecureMetrics:           false,
		ProbeAddr:               "0.0.0.0:8081",
		SidecarImage:            "ghcr.io/kaidotio/hippocampus/prometheus-metrics-proxy-hook:main",
		EnableSidecarContainers: false,
	}
}
