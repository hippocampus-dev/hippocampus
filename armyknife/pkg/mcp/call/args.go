package call

type Args struct {
	URL       string `validate:"required,url"`
	Tool      string `validate:"required"`
	Arguments string `validate:"omitempty,json"`
}

func DefaultArgs() *Args {
	return &Args{}
}
