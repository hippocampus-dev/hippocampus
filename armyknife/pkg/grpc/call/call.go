package call

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/go-playground/validator/v10"
	"github.com/mattn/go-zglob"

	"github.com/jhump/protoreflect/desc/protoparse"
	"github.com/jhump/protoreflect/dynamic"
	"golang.org/x/xerrors"
	"google.golang.org/grpc"
)

func Run(a *Args) error {
	if err := validator.New().Struct(a); err != nil {
		return xerrors.Errorf("validation error: %w", err)
	}

	p := &protoparse.Parser{}
	if a.ImportPaths != nil {
		p.ImportPaths = append(p.ImportPaths, a.ImportPaths...)
	}
	files, err := zglob.Glob(a.Pattern)
	if err != nil {
		return xerrors.Errorf("failed to glob files for %s: %w", a.Pattern, err)
	}
	fds, err := p.ParseFiles(files...)
	if err != nil {
		return xerrors.Errorf("failed to parse files: %w", err)
	}
	for _, fd := range fds {
		fds = append(fds, fd.GetDependencies()...)
	}

	conn, err := grpc.Dial(a.Address, grpc.WithInsecure())
	if err != nil {
		return xerrors.Errorf("failed to dial to %s: %w", a.Address, err)
	}
	defer conn.Close()

	var availableEndpoints []string
	for _, fd := range fds {
		for _, service := range fd.GetServices() {
			for _, method := range service.GetMethods() {
				s := strings.Split(method.GetFullyQualifiedName(), ".")
				endpoint := fmt.Sprintf("/%s/%s", strings.Join(s[:len(s)-1], "."), s[len(s)-1])
				if endpoint == a.Endpoint {
					in := dynamic.NewMessage(method.GetInputType())
					out := dynamic.NewMessage(method.GetOutputType())
					if err := in.UnmarshalJSON([]byte(a.Body)); err != nil {
						return xerrors.Errorf("failed to unmarshal: %+v, available: %+v", err, in.GetKnownFields())
					}
					if err := conn.Invoke(context.Background(), endpoint, in, out); err != nil {
						return xerrors.Errorf("failed to invoke %s: %w", endpoint, err)
					}
					log.Printf("%+v", out)
					return nil
				}
				availableEndpoints = append(availableEndpoints, endpoint)
			}
		}
	}

	return xerrors.Errorf("%s is not included in the list of available %s", a.Endpoint, strings.Join(availableEndpoints, ", "))
}
