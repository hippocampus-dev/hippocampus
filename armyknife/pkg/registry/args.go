package registry

type Args struct {
	ManifestPath string `validate:"required,dir"`
	OutputPath   string
	Stdout       bool
}

func DefaultArgs() *Args {
	return &Args{}
}
