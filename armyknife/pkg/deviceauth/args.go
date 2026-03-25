package deviceauth

type Args struct {
	URL      string `validate:"required,url"`
	ClientID string `validate:"required"`
	Scope    string `validate:"required"`
}

func DefaultArgs() *Args {
	return &Args{
		URL:      "https://development.127.0.0.1.nip.io",
		ClientID: "armyknife",
	}
}
