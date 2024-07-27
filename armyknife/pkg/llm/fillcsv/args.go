package fillcsv

type Args struct {
	CSV                     string `validate:"required,file"`
	From                    string `validate:"required"`
	To                      string `validate:"required"`
	Concurrency             uint   `validate:"required,gt=0"`
	PromptFile              string `validate:"omitempty,file"`
	Override                bool   `validate:"omitempty"`
	AppendBy                string `validate:"omitempty"`
	Model                   string `validate:"required"`
	AuthorizationListenPort uint   `validate:"omitempty"`
}

func DefaultArgs() *Args {
	return &Args{
		Concurrency:             1,
		Model:                   "gpt-4o",
		AuthorizationListenPort: 0,
	}
}
