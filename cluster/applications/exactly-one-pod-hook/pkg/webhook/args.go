package webhook

import "exactly-one-pod-hook/internal/lock"

type Args struct {
	Host                    string `validate:"required,ip"`
	Port                    int    `validate:"required,gt=0,lte=65535"`
	CertDir                 string `validate:"required,dir"`
	MetricsAddr             string `validate:"required,tcp_addr"`
	EnableHTTP2             bool   `validate:"omitempty"`
	SecureMetrics           bool   `validate:"omitempty"`
	ProbeAddr               string `validate:"required,tcp_addr"`
	SidecarImage            string `validate:"required"`
	EnableSidecarContainers bool   `validate:"omitempty"`
	*lock.Args
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
		SidecarImage:            "ghcr.io/kaidotio/hippocampus/exactly-one-pod-hook:main",
		EnableSidecarContainers: false,
		Args:                    lock.DefaultArgs(),
	}
}
