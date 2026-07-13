package viewer

import "os"

type Args struct {
	S3Bucket      string `validate:"required"`
	S3Prefix      string `validate:"required"`
	S3EndpointURL string `validate:"url"`
}

func DefaultArgs() *Args {
	return &Args{
		S3EndpointURL: os.Getenv("S3_ENDPOINT_URL"),
	}
}
