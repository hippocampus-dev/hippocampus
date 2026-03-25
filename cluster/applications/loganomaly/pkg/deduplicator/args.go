package deduplicator

import "time"

type Args struct {
	RedisAddress string        `validate:"required"`
	DedupTTL     time.Duration `validate:"gt=0"`
}

func DefaultArgs() *Args {
	return &Args{
		RedisAddress: "127.0.0.1:6379",
		DedupTTL:     24 * time.Hour,
	}
}
