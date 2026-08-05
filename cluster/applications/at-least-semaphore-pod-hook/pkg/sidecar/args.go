package sidecar

import (
	"at-least-semaphore-pod-hook/internal/lock"
)

type Args struct {
	QueueName                     string `validate:"required"`
	QueueValue                    string `validate:"required"`
	TerminationGracePeriodSeconds int    `validate:"required"`
	*lock.Args
}

func DefaultArgs() *Args {
	return &Args{
		Args:                          lock.DefaultArgs(),
		TerminationGracePeriodSeconds: 10,
	}
}
