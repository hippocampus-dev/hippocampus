package selfupdate

import "os"

type Args struct {
	GitHubToken string `validate:"required"`
}

func DefaultArgs() *Args {
	return &Args{
		GitHubToken: os.Getenv("GITHUB_TOKEN"),
	}
}
