package codex

import "os"

type Args struct {
	Address string
	Profile string
}

func DefaultArgs() *Args {
	return &Args{
		Profile: os.Getenv("CODEX_PROFILE"),
	}
}
