package completion

type Args struct {
	Shell string `validate:"required,oneof=fish"`
}

func DefaultArgs() *Args {
	return &Args{}
}
