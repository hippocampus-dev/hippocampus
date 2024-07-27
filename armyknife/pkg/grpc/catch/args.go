package catch

type Args struct {
	Address     string   `validate:"required"`
	ImportPaths []string `validate:"omitempty"`
	Pattern     string   `validate:"required"`
}

func DefaultArgs() *Args {
	return &Args{
		Address: "0.0.0.0:8080",
		Pattern: "**/*.proto",
	}
}
