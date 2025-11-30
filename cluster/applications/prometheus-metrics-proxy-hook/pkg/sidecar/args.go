package sidecar

import "time"

type Args struct {
	Address                string        `validate:"required"`
	TargetPort             int           `validate:"required,min=1,max=65535"`
	TerminationGracePeriod time.Duration `validate:"required"`
	Lameduck               time.Duration `validate:"required"`
}

func DefaultArgs() *Args {
	return &Args{
		Address:                "0.0.0.0:18080",
		TargetPort:             8080,
		TerminationGracePeriod: 30 * time.Second,
		Lameduck:               1 * time.Second,
	}
}
