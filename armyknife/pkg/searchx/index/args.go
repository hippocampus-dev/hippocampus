package index

type Args struct {
	LSPServers              map[string]string `validate:"required,min=1"`
	Output                  string            `validate:"required"`
	Dimension               int               `validate:"required,min=1"`
	EmbeddingModel          string            `validate:"required"`
	AuthorizationListenPort uint              `validate:"max=65535"`
}

func DefaultArgs() *Args {
	return &Args{
		Output:                  "symbols.db",
		Dimension:               768,
		EmbeddingModel:          "embeddinggemma:300m",
		AuthorizationListenPort: 0,
	}
}
