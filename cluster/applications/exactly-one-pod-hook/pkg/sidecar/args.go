package sidecar

import (
	"exactly-one-pod-hook/internal/lock"
)

type Args struct {
	UnlockKey                     string `validate:"required"`
	UnlockValue                   string `validate:"required"`
	TerminationGracePeriodSeconds int    `validate:"required"`
	*lock.Args
}

func DefaultArgs() *Args {
	return &Args{
		Args:                          lock.DefaultArgs(),
		TerminationGracePeriodSeconds: 10,
	}
}
