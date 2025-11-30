package lsp

type Args struct {
	LSPServers []string `validate:"required,min=1"`
}

func DefaultArgs() *Args {
	return &Args{
		LSPServers: []string{"localhost:6008"},
	}
}
