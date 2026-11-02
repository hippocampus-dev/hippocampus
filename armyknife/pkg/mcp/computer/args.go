package computer

import "os"

type Args struct {
	DisplayName string
	Address     string
}

func DefaultArgs() *Args {
	return &Args{
		DisplayName: os.Getenv("DISPLAY"),
	}
}
