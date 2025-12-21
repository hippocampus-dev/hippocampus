package sidecar

import "time"

type Args struct {
	Address                string        `validate:"required"`
	TargetURL              string        `validate:"required,url"`
	TerminationGracePeriod time.Duration `validate:"required"`
	Lameduck               time.Duration `validate:"required"`
}

func DefaultArgs() *Args {
	return &Args{
		Address:                "0.0.0.0:18080",
		TargetURL:              "http://127.0.0.1:8080/metrics",
		TerminationGracePeriod: 30 * time.Second,
		Lameduck:               1 * time.Second,
	}
}
