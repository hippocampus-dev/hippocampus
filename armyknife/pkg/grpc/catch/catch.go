package catch

import (
	"armyknife/armyknifepb"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"runtime/debug"
	"strings"
	"syscall"
	"time"

	"google.golang.org/grpc/status"

	"google.golang.org/grpc/codes"

	"github.com/go-playground/validator/v10"
	"github.com/mattn/go-zglob"

	"github.com/jhump/protoreflect/desc"

	"github.com/jhump/protoreflect/dynamic"

	"github.com/jhump/protoreflect/desc/protoparse"
	"golang.org/x/xerrors"
	"google.golang.org/grpc"
)

type wildcard struct{}

type rpcbinServer struct{}

func newRpcbinServer() *rpcbinServer {
	return &rpcbinServer{}
}

func (s *rpcbinServer) Delay(ctx context.Context, message *armyknifepb.DelayMessage) (*armyknifepb.DelayResponse, error) {
	if err := message.Validate(); err != nil {
		return nil, xerrors.Errorf("validation error: %w", err)
	}
	time.Sleep(time.Duration(message.Delay) * time.Second)
	return &armyknifepb.DelayResponse{}, nil
}

func (s *rpcbinServer) Status(ctx context.Context, message *armyknifepb.StatusMessage) (*armyknifepb.StatusResponse, error) {
	if err := message.Validate(); err != nil {
		return nil, xerrors.Errorf("validation error: %w", err)
	}
	b, err := json.Marshal(message.Code)
	if err != nil {
		return nil, xerrors.Errorf("failed to marshal message.Code: %w", err)
	}
	code := codes.OK
	if err := code.UnmarshalJSON(b); err != nil {
		return nil, xerrors.Errorf("failed to unmarshal message.Code: %w", err)
	}
	return &armyknifepb.StatusResponse{}, status.Errorf(code, message.Code.String())
}

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

	listener, err := net.Listen("tcp", a.Address)
	if err != nil {
		return xerrors.Errorf("could not listen %s: %w", a.Address, err)
	}
	server := grpc.NewServer()
	armyknifepb.RegisterRpcbinServer(server, newRpcbinServer())
	var endpoints []string
	for _, fd := range fds {
		for _, service := range fd.GetServices() {
			info := server.GetServiceInfo()
			if len(info[service.GetFullyQualifiedName()].Methods) != 0 {
				continue
			}
			sd := grpc.ServiceDesc{ServiceName: service.GetFullyQualifiedName(), HandlerType: (*interface{})(nil)}
			for _, method := range service.GetMethods() {
				sd.Methods = append(sd.Methods, grpc.MethodDesc{MethodName: method.GetName(), Handler: func(method *desc.MethodDescriptor) func(interface{}, context.Context, func(interface{}) error, grpc.UnaryServerInterceptor) (interface{}, error) {
					return func(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
						in := dynamic.NewMessage(method.GetInputType())
						out := dynamic.NewMessage(method.GetOutputType())
						if err := dec(in); err != nil {
							return nil, xerrors.Errorf("failed to decode at %s: %w", method.GetName(), err)
						}
						log.Printf("%+v", in)
						return out, nil
					}
				}(method)})
			}
			server.RegisterService(&sd, &wildcard{})
		}
	}
	for serviceFQN, serviceInfo := range server.GetServiceInfo() {
		for _, method := range serviceInfo.Methods {
			endpoints = append(endpoints, fmt.Sprintf("/%s/%s", serviceFQN, method.Name))
		}
	}
	go func() {
		defer func() {
			if err := recover(); err != nil {
				log.Printf("panic: %+v", err)
				log.Printf("%s", debug.Stack())
			}
		}()
		log.Printf("Listen on http://%s [%s]", a.Address, strings.Join(endpoints, ", "))
		if err := server.Serve(listener); err != nil {
			log.Fatalf("Failed to listen: %+v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGTERM)
	<-quit
	log.Printf("Attempt to shutdown instance...")

	server.GracefulStop()
	log.Printf("Server has been shutdown")
	return nil
}
