package storage

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"golang.org/x/xerrors"
)

type s3Storage struct {
	client *s3.Client
	config S3Config
}

type S3Config struct {
	Bucket string
}

func NewS3Storage(ctx context.Context, s S3Config) (Storage, error) {
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
		return nil, xerrors.Errorf("failed to load AWS config: %w", err)
	}
	s3Client := s3.NewFromConfig(c, func(o *s3.Options) {
		o.UsePathStyle = true
	})

	return &s3Storage{
		client: s3Client,
		config: s,
	}, nil
}

func (s *s3Storage) Put(ctx context.Context, key string, data []byte) (string, error) {
	contentType := http.DetectContentType(data)

	if _, err := s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(s.config.Bucket),
		Key:         aws.String(key),
		Body:        bytes.NewReader(data),
		ContentType: aws.String(contentType),
	}); err != nil {
		return "", xerrors.Errorf("failed to upload to S3: %w", err)
	}

	return fmt.Sprintf("s3://%s/%s", s.config.Bucket, key), nil
}

func (s *s3Storage) Get(ctx context.Context, url string) ([]byte, error) {
	key := strings.TrimPrefix(url, fmt.Sprintf("s3://%s/", s.config.Bucket))

	result, err := s.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.config.Bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, xerrors.Errorf("failed to download from S3: %w", err)
	}
	defer result.Body.Close()

	var buffer bytes.Buffer
	_, err = buffer.ReadFrom(result.Body)
	if err != nil {
		return nil, xerrors.Errorf("failed to read S3 object: %w", err)
	}

	return buffer.Bytes(), nil
}
