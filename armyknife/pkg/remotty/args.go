package remotty

type Args struct {
	Server     string `validate:"required,url"`
	Remotes    []string
	Auth       string `validate:"required"`
	BakeryURL  string `validate:"required,uri"`
	CookieName string
	ListenPort uint
	Sync       string `validate:"omitempty,dir"`
	Env        []string
}

func DefaultArgs() *Args {
	return &Args{
		BakeryURL:  "https://bakery.kaidotio.dev/callback",
		CookieName: "remotty.kaidotio.dev",
		ListenPort: 0,
	}
}
