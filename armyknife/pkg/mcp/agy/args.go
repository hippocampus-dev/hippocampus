package agy

type Args struct {
	Address string
	Model   string
}

func DefaultArgs() *Args {
	return &Args{}
}
