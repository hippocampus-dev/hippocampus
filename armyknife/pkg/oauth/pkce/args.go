package pkce

type Args struct {
	URL        string `validate:"required,url"`
	ClientID   string `validate:"required"`
	Scope      string `validate:"required"`
	ListenPort uint
}

func DefaultArgs() *Args {
	return &Args{
		URL:      "https://development.127.0.0.1.nip.io",
		ClientID: "armyknife",
	}
}
