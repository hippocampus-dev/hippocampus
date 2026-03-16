package egosearch

import (
	"os"
	"time"
)

type Args struct {
	Keywords    []string
	InvertMatch string
	Interval    time.Duration
	Count       int
	SlackToken  string
}

func DefaultArgs() *Args {
	return &Args{
		Interval:   5 * time.Minute,
		Count:      10,
		SlackToken: os.Getenv("SLACK_TOKEN"),
	}
}
