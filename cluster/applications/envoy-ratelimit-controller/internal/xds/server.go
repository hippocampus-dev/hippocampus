package xds

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"sort"
	"strconv"
	"sync"

	v1 "envoy-ratelimit-controller/api/v1"

	coreV3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	discoveryV3 "github.com/envoyproxy/go-control-plane/envoy/service/discovery/v3"
	"github.com/envoyproxy/go-control-plane/pkg/cache/types"
	cacheV3 "github.com/envoyproxy/go-control-plane/pkg/cache/v3"
	resourceV3 "github.com/envoyproxy/go-control-plane/pkg/resource/v3"
	serverV3 "github.com/envoyproxy/go-control-plane/pkg/server/v3"
	rlsConfV3 "github.com/envoyproxy/go-control-plane/ratelimit/config/ratelimit/v3"
	"golang.org/x/xerrors"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/proto"
	metaV1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sTypes "k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/event"
)

const (
	nodeID = "envoy-ratelimit"
	// Never a RateLimit domain, since those always contain a dot
	placeholderDomain = "envoy-ratelimit-controller"
)

type nodeHash struct{}

func (nodeHash) ID(_ *coreV3.Node) string {
	// Every envoy-ratelimit replica is served the same snapshot whatever node ID it sends
	return nodeID
}

type stream struct {
	acked           int64
	rejected        string
	rejectedVersion int64
	nonce           string
	nonceVersion    int64
}

type Server struct {
	Address     string
	Events      chan event.GenericEvent
	cache       cacheV3.SnapshotCache
	mutex       sync.Mutex
	initialized bool
	version     int64
	configs     map[k8sTypes.NamespacedName]*rlsConfV3.RateLimitConfig
	included    map[k8sTypes.NamespacedName]int64
	streams     map[int64]*stream
}

func NewServer(address string) *Server {
	return &Server{
		Address:  address,
		Events:   make(chan event.GenericEvent),
		cache:    cacheV3.NewSnapshotCache(false, nodeHash{}, nil),
		configs:  make(map[k8sTypes.NamespacedName]*rlsConfV3.RateLimitConfig),
		included: make(map[k8sTypes.NamespacedName]int64),
		streams:  make(map[int64]*stream),
	}
}

func (s *Server) Set(ctx context.Context, name k8sTypes.NamespacedName, config *rlsConfV3.RateLimitConfig) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if current, ok := s.configs[name]; ok && proto.Equal(current, config) {
		return nil
	}
	s.configs[name] = config
	return s.publish(ctx, name)
}

func (s *Server) Delete(ctx context.Context, name k8sTypes.NamespacedName) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if _, ok := s.configs[name]; !ok {
		return nil
	}
	delete(s.configs, name)
	delete(s.included, name)
	return s.publish(ctx, name)
}

func (s *Server) Initialize(ctx context.Context) error {
	// Called once every RateLimit present at startup has been Set, so envoy-ratelimit never receives a partial snapshot
	s.mutex.Lock()
	defer s.mutex.Unlock()

	s.initialized = true
	return s.publish(ctx, k8sTypes.NamespacedName{})
}

func (s *Server) publish(ctx context.Context, changed k8sTypes.NamespacedName) error {
	if !s.initialized {
		return nil
	}

	names := make([]k8sTypes.NamespacedName, 0, len(s.configs))
	for name := range s.configs {
		names = append(names, name)
	}
	sort.Slice(names, func(i int, j int) bool {
		return names[i].String() < names[j].String()
	})
	// HEALTHY_WITH_AT_LEAST_ONE_CONFIG_LOADED fails every replica on a config without any domain: https://github.com/envoyproxy/ratelimit/blob/91484c59/src/service/ratelimit.go
	resources := []types.Resource{&rlsConfV3.RateLimitConfig{Name: placeholderDomain, Domain: placeholderDomain}}
	for _, name := range names {
		resources = append(resources, s.configs[name])
	}

	s.version++
	snapshot, err := cacheV3.NewSnapshot(strconv.FormatInt(s.version, 10), map[resourceV3.Type][]types.Resource{
		resourceV3.RateLimitConfigType: resources,
	})
	if err != nil {
		return xerrors.Errorf("failed to create snapshot: %w", err)
	}
	if err := s.cache.SetSnapshot(ctx, nodeID, snapshot); err != nil {
		return xerrors.Errorf("failed to set snapshot: %w", err)
	}

	for _, name := range names {
		if _, ok := s.included[name]; !ok || name == changed {
			s.included[name] = s.version
		}
	}
	return nil
}

func (s *Server) Status(name k8sTypes.NamespacedName) (metaV1.ConditionStatus, string, string) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	version, ok := s.included[name]
	if !ok {
		return metaV1.ConditionFalse, "Pending", "the config has not been published yet"
	}
	if len(s.streams) == 0 {
		return metaV1.ConditionFalse, "NoRateLimitServer", "no envoy-ratelimit is connected"
	}

	accepted := 0
	for _, st := range s.streams {
		if st.acked >= version {
			accepted++
			continue
		}
		if st.rejected != "" && st.rejectedVersion >= version {
			return metaV1.ConditionFalse, "Rejected", st.rejected
		}
	}
	if accepted < len(s.streams) {
		return metaV1.ConditionFalse, "Pending", fmt.Sprintf("accepted by %d of %d envoy-ratelimit", accepted, len(s.streams))
	}
	return metaV1.ConditionTrue, "Synced", fmt.Sprintf("accepted by %d envoy-ratelimit", accepted)
}

func (s *Server) ReadyCheck(_ *http.Request) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if !s.initialized {
		return xerrors.New("the first snapshot has not been published yet")
	}
	return nil
}

func (s *Server) notify() {
	s.mutex.Lock()
	names := make([]k8sTypes.NamespacedName, 0, len(s.included))
	for name := range s.included {
		names = append(names, name)
	}
	s.mutex.Unlock()

	go func() {
		for _, name := range names {
			s.Events <- event.GenericEvent{Object: &v1.RateLimit{ObjectMeta: metaV1.ObjectMeta{Namespace: name.Namespace, Name: name.Name}}}
		}
	}()
}

func (s *Server) Start(ctx context.Context) error {
	callbacks := serverV3.CallbackFuncs{
		StreamRequestFunc: func(id int64, request *discoveryV3.DiscoveryRequest) error {
			s.mutex.Lock()
			st, ok := s.streams[id]
			if !ok {
				st = &stream{}
				s.streams[id] = st
			}
			// VersionInfo is replayed by a reconnecting client and may come from a previous controller process, so only the nonce of a response sent on this stream identifies what was accepted or rejected
			if request.ResponseNonce != "" && request.ResponseNonce == st.nonce {
				if request.ErrorDetail != nil {
					st.rejected = request.ErrorDetail.GetMessage()
					st.rejectedVersion = st.nonceVersion
				} else {
					st.acked = st.nonceVersion
					st.rejected = ""
				}
			}
			s.mutex.Unlock()
			s.notify()
			return nil
		},
		StreamResponseFunc: func(_ context.Context, id int64, _ *discoveryV3.DiscoveryRequest, response *discoveryV3.DiscoveryResponse) {
			s.mutex.Lock()
			defer s.mutex.Unlock()
			if st, ok := s.streams[id]; ok {
				if version, err := strconv.ParseInt(response.VersionInfo, 10, 64); err == nil {
					st.nonce = response.Nonce
					st.nonceVersion = version
				}
			}
		},
		StreamClosedFunc: func(id int64, _ *coreV3.Node) {
			s.mutex.Lock()
			delete(s.streams, id)
			s.mutex.Unlock()
			s.notify()
		},
	}

	grpcServer := grpc.NewServer()
	discoveryV3.RegisterAggregatedDiscoveryServiceServer(grpcServer, serverV3.NewServer(ctx, s.cache, callbacks))

	listener, err := net.Listen("tcp", s.Address)
	if err != nil {
		return xerrors.Errorf("failed to listen on %s: %w", s.Address, err)
	}

	errCh := make(chan error, 1)
	go func() {
		errCh <- grpcServer.Serve(listener)
	}()

	select {
	case <-ctx.Done():
		grpcServer.GracefulStop()
		return nil
	case err := <-errCh:
		return xerrors.Errorf("failed to serve xDS: %w", err)
	}
}
