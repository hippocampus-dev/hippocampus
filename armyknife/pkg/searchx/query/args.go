package query

type Args struct {
	Query                   string `validate:"required"`
	Database                string `validate:"required"`
	Limit                   int    `validate:"required,min=1"`
	Dimension               int    `validate:"required,min=1"`
	EmbeddingModel          string `validate:"required"`
	AuthorizationListenPort uint   `validate:"max=65535"`
}

func DefaultArgs() *Args {
	return &Args{
		Database:                "symbols.db",
		Limit:                   10,
		Dimension:               768,
		EmbeddingModel:          "embeddinggemma:300m",
		AuthorizationListenPort: 0,
	}
}
