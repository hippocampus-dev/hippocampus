package echo

type Args struct {
	Address                       string `validate:"required,tcp_addr"`
	MaxConnections                int    `validate:"required,gt=0"`
	TerminationGracePeriodSeconds int    `validate:"required,gt=0"`
	Lameduck                      int    `validate:"required,gt=0"`
	Keepalive                     bool   `validate:"omitempty"`
}

func DefaultArgs() *Args {
	return &Args{
		Address:                       "0.0.0.0:8080",
		MaxConnections:                65536,
		TerminationGracePeriodSeconds: 10,
		Lameduck:                      1,
		Keepalive:                     true,
	}
}
