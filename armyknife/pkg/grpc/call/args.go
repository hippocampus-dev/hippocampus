package call

type Args struct {
	Address     string   `validate:"required"`
	Endpoint    string   `validate:"required"`
	Body        string   `validate:"required,json"`
	ImportPaths []string `validate:"omitempty"`
	Pattern     string   `validate:"required"`
}

func DefaultArgs() *Args {
	return &Args{
		Pattern: "**/*.proto",
	}
}
