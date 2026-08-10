package bakery

type Args struct {
	CookieName string `validate:"required"`
	URL        string `validate:"required,uri"`
	ListenPort uint
}

func DefaultArgs() *Args {
	return &Args{
		URL:        "https://bakery.kaidotio.dev/callback",
		ListenPort: 0,
	}
}
