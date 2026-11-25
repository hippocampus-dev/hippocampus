package catch

type Args struct {
	Address     string `validate:"required"`
	ImportPaths []string
	Pattern     string `validate:"required"`
	Lameduck    int    `validate:"required,gt=0"`
}

func DefaultArgs() *Args {
	return &Args{
		Address:  "0.0.0.0:8080",
		Pattern:  "**/*.proto",
		Lameduck: 1,
	}
}
