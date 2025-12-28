package bakery

type Args struct {
	CookieName string `validate:"required"`
	URL        string `validate:"required,uri"`
	ListenPort uint
}

func DefaultArgs() *Args {
	return &Args{
		URL:        "https://bakery.minikube.127.0.0.1.nip.io/callback",
		ListenPort: 0,
	}
}
