package main

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"runtime/debug"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	v4 "github.com/aws/aws-sdk-go-v2/aws/signer/v4"
	awsHTTP "github.com/aws/aws-sdk-go-v2/aws/transport/http"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	smithyendpoints "github.com/aws/smithy-go/endpoints"
)

func envOrDefaultValue[T any](key string, defaultValue T) T {
	value, exists := os.LookupEnv(key)
	if !exists {
		return defaultValue
	}

	switch any(defaultValue).(type) {
	case string:
		return any(value).(T)
	case int:
		if intValue, err := strconv.Atoi(value); err == nil {
			return any(intValue).(T)
		}
	case int64:
		if intValue, err := strconv.ParseInt(value, 10, 64); err == nil {
			return any(intValue).(T)
		}
	case uint:
		if uintValue, err := strconv.ParseUint(value, 10, 0); err == nil {
			return any(uint(uintValue)).(T)
		}
	case uint64:
		if uintValue, err := strconv.ParseUint(value, 10, 64); err == nil {
			return any(uintValue).(T)
		}
	case float64:
		if floatValue, err := strconv.ParseFloat(value, 64); err == nil {
			return any(floatValue).(T)
		}
	case bool:
		if boolValue, err := strconv.ParseBool(value); err == nil {
			return any(boolValue).(T)
		}
	case time.Duration:
		if durationValue, err := time.ParseDuration(value); err == nil {
			return any(durationValue).(T)
		}
	}

	return defaultValue
}

type resolver struct{}

func (*resolver) ResolveEndpoint(ctx context.Context, params s3.EndpointParameters) (smithyendpoints.Endpoint, error) {
	s3EndpointUrl, ok := os.LookupEnv("S3_ENDPOINT_URL")
	if ok {
		u, err := url.Parse(s3EndpointUrl)
		if err != nil {
			return smithyendpoints.Endpoint{}, err
		}
		return smithyendpoints.Endpoint{
			URI: *u,
		}, nil
	}

	return s3.NewDefaultEndpointResolverV2().ResolveEndpoint(ctx, params)
}

func main() {
	var address string
	var terminationGracePeriod time.Duration
	var lameduck time.Duration
	var keepAlive bool
	flag.StringVar(&address, "address", envOrDefaultValue("ADDRESS", "0.0.0.0:8080"), "")

	flag.DurationVar(&terminationGracePeriod, "termination-grace-period", envOrDefaultValue("TERMINATION_GRACE_PERIOD", 10*time.Second), "The duration the application needs to terminate gracefully")
	flag.DurationVar(&lameduck, "lameduck", envOrDefaultValue("LAMEDUCK", 1*time.Second), "A period that explicitly asks clients to stop sending requests, although the backend task is listening on that port and can provide the service")
	flag.BoolVar(&keepAlive, "http-keepalive", envOrDefaultValue("HTTP_KEEPALIVE", true), "")
	flag.Parse()

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

	ctx := context.Background()
	c, err := config.LoadDefaultConfig(ctx, optsFunc...)
	if err != nil {
		log.Fatalf("failed to load configuration, %+v", err)
	}
	s3Client := s3.NewFromConfig(c, func(o *s3.Options) {
		//o.EndpointResolverV2 = &resolver{}
		o.UsePathStyle = true
	})

	bucket := os.Getenv("S3_BUCKET")

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		key := strings.TrimPrefix(r.URL.Path, "/")

		if key == "" {
			http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
			return
		}

		switch r.Method {
		case http.MethodGet:
			output, err := s3Client.GetObject(r.Context(), &s3.GetObjectInput{
				Bucket: &bucket,
				Key:    &key,
			})
			if err != nil {
				var awsErr *awsHTTP.ResponseError
				if errors.As(err, &awsErr) {
					if awsErr.HTTPStatusCode() == http.StatusNotFound {
						http.Error(w, http.StatusText(awsErr.HTTPStatusCode()), awsErr.HTTPStatusCode())
						return
					}
				}
				log.Printf("%+v", err)
				http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
				return
			}

			defer func() {
				_ = output.Body.Close()
			}()
			response, err := io.ReadAll(output.Body)
			if err != nil {
				log.Printf("%+v", err)
				http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
				return
			}

			w.Header().Set("Content-Type", *output.ContentType)
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(response)
		case http.MethodPost:
			defer func() {
				_ = r.Body.Close()
			}()
			body, err := io.ReadAll(r.Body)
			if err != nil {
				log.Printf("%+v", err)
				http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
				return
			}

			contentType := http.DetectContentType(body)
			if _, err := s3Client.PutObject(r.Context(), &s3.PutObjectInput{
				Bucket:        &bucket,
				Key:           &key,
				Body:          bytes.NewReader(body),
				ContentLength: aws.Int64(int64(len(body))),
				ContentType:   &contentType,
			}, s3.WithAPIOptions(v4.SwapComputePayloadSHA256ForUnsignedPayloadMiddleware)); err != nil {
				log.Printf("%+v", err)
				http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
				return
			}

			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(http.StatusText(http.StatusOK)))
		case http.MethodDelete:
			if _, err := s3Client.DeleteObject(r.Context(), &s3.DeleteObjectInput{
				Bucket: &bucket,
				Key:    &key,
			}); err != nil {
				log.Printf("%+v", err)
				http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
				return
			}

			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(http.StatusText(http.StatusOK)))
		}
	})

	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(http.StatusText(http.StatusOK)))
	})

	listener, err := net.Listen("tcp", address)
	if err != nil {
		log.Fatalf("failed to listen: %+v", err)
	}

	server := &http.Server{
		Handler: mux,
	}
	server.SetKeepAlivesEnabled(keepAlive)

	go func() {
		defer func() {
			if err := recover(); err != nil {
				log.Printf("panic: %+v\n%s", err, debug.Stack())
			}
		}()
		if err := server.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("failed to listen: %+v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGTERM)
	<-quit
	time.Sleep(lameduck)

	ctx, cancel := context.WithTimeout(context.Background(), terminationGracePeriod)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Fatalf("failed to shutdown: %+v", err)
	}
}
