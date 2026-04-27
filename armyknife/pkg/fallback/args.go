package fallback

import "math"

type Args struct {
	Address                       string `validate:"required,tcp_addr"`
	MaxConnections                int    `validate:"required,gt=0"`
	TerminationGracePeriodSeconds int    `validate:"required,gt=0"`
	Lameduck                      int    `validate:"required,gt=0"`
	Keepalive                     bool   `validate:"required"`
	Target                        string `validate:"required,url"`
	Fallback                      string `validate:"required,url"`
}

func DefaultArgs() *Args {
	return &Args{
		Address:                       "0.0.0.0:8080",
		MaxConnections:                math.MaxInt32,
		TerminationGracePeriodSeconds: 10,
		Lameduck:                      1,
		Keepalive:                     true,
	}
}
