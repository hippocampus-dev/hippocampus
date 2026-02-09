package viewer

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/ktr0731/go-fuzzyfinder"
	"github.com/mitchellh/go-wordwrap"
	"golang.org/x/xerrors"
)

func handleBody(body []byte) string {
	var data any
	if err := json.Unmarshal(body, &data); err != nil {
		return string(body)
	}

	b, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return string(body)
	}
	return string(b)
}

func Run(a *Args) error {
	ctx := context.Background()

	var optsFunc []func(*config.LoadOptions) error

	s3EndpointUrl, ok := os.LookupEnv("S3_ENDPOINT_URL")
	if ok {
		resolver := aws.EndpointResolverWithOptionsFunc(func(service, region string, options ...interface{}) (aws.Endpoint, error) {
			return aws.Endpoint{
				URL:               s3EndpointUrl,
				HostnameImmutable: true,
			}, nil
		})
		optsFunc = append(optsFunc, config.WithEndpointResolverWithOptions(resolver))
	}

	c, err := config.LoadDefaultConfig(ctx, optsFunc...)
	if err != nil {
		return xerrors.Errorf("failed to load configuration: %w", err)
	}

	s3Client := s3.NewFromConfig(c, func(o *s3.Options) {
		o.UsePathStyle = true
	})

	response, err := s3Client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
		Bucket: &a.S3Bucket,
		Prefix: &a.S3Prefix,
	})
	if err != nil {
		return xerrors.Errorf("failed to list objects: %w", err)
	}

	cache := make(map[string]string)

	indexes, err := fuzzyfinder.FindMulti(response.Contents, func(i int) string {
		return *response.Contents[i].Key
	}, fuzzyfinder.WithPreviewWindow(func(i, width, height int) string {
		if i == -1 {
			return ""
		}

		if v, ok := cache[*response.Contents[i].Key]; ok {
			return v
		}

		object, err := s3Client.GetObject(ctx, &s3.GetObjectInput{
			Bucket: &a.S3Bucket,
			Key:    response.Contents[i].Key,
		})
		if err != nil {
			return ""
		}
		defer func() {
			_ = object.Body.Close()
		}()

		body, err := io.ReadAll(object.Body)
		if err != nil {
			return ""
		}

		retval := wordwrap.WrapString(handleBody(body), uint(width/4))
		cache[*response.Contents[i].Key] = retval
		return retval
	}))
	if err != nil {
		return xerrors.Errorf("failed to find object: %w", err)
	}

	for _, i := range indexes {
		fmt.Println(filepath.Join(a.S3Bucket, *response.Contents[i].Key))
	}

	return nil
}
