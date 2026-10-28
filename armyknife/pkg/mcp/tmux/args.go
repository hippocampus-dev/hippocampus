package tmux

type Args struct {
	Address string
}

func DefaultArgs() *Args {
	return &Args{}
}
