package driver

import (
	"context"
	"fmt"
	"fuse-csi-driver/internal/fdpass"
	"log"
	"net"
	"os"
	"path/filepath"
	"runtime/debug"
	"sync"
	"time"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"golang.org/x/sys/unix"
	"golang.org/x/xerrors"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

const (
	driverName           = "fuse-csi-driver.csi.kaidotio.github.io"
	driverVersion        = "0.1.0"
	fdTimeout            = 1 * time.Hour
	kubeletPodsDirectory = "/var/lib/kubelet/pods"
	csiSocketPath        = "/csi/csi.sock"
)

type mountInfo struct {
	fuseFd     int
	fdClosed   bool
	cancel     context.CancelFunc
	socketPath string
}

type Driver struct {
	csi.UnimplementedIdentityServer
	csi.UnimplementedNodeServer

	nodeID string
	server *grpc.Server

	mu     sync.Mutex
	mounts map[string]*mountInfo
}

func NewDriver(nodeID string) *Driver {
	return &Driver{
		nodeID: nodeID,
		mounts: make(map[string]*mountInfo),
	}
}

func (d *Driver) Run() error {
	_ = os.Remove(csiSocketPath)

	listener, err := net.Listen("unix", csiSocketPath)
	if err != nil {
		return xerrors.Errorf("failed to listen: %w", err)
	}

	d.server = grpc.NewServer(grpc.UnaryInterceptor(panicRecoveryInterceptor))
	csi.RegisterIdentityServer(d.server, d)
	csi.RegisterNodeServer(d.server, d)

	if err := d.server.Serve(listener); err != nil {
		return xerrors.Errorf("failed to serve: %w", err)
	}
	return nil
}

func (d *Driver) Stop() {
	if d.server != nil {
		d.server.GracefulStop()
	}
}

func (d *Driver) GetPluginInfo(_ context.Context, _ *csi.GetPluginInfoRequest) (*csi.GetPluginInfoResponse, error) {
	return &csi.GetPluginInfoResponse{
		Name:          driverName,
		VendorVersion: driverVersion,
	}, nil
}

func (d *Driver) GetPluginCapabilities(_ context.Context, _ *csi.GetPluginCapabilitiesRequest) (*csi.GetPluginCapabilitiesResponse, error) {
	return &csi.GetPluginCapabilitiesResponse{}, nil
}

func (d *Driver) Probe(_ context.Context, _ *csi.ProbeRequest) (*csi.ProbeResponse, error) {
	return &csi.ProbeResponse{Ready: &wrapperspb.BoolValue{Value: true}}, nil
}

func (d *Driver) NodePublishVolume(_ context.Context, request *csi.NodePublishVolumeRequest) (*csi.NodePublishVolumeResponse, error) {
	targetPath := request.GetTargetPath()
	if targetPath == "" {
		return nil, status.Error(codes.InvalidArgument, "target path is required")
	}

	volumeContext := request.GetVolumeContext()
	socketVolume := volumeContext["socketVolume"]
	if socketVolume == "" {
		return nil, status.Error(codes.InvalidArgument, "volumeAttribute socketVolume is required")
	}

	podUID := volumeContext["csi.storage.k8s.io/pod.uid"]
	if podUID == "" {
		return nil, status.Error(codes.InvalidArgument, "pod UID is required (podInfoOnMount must be true)")
	}

	uid := uint32(65532)
	gid := uint32(65532)

	if err := os.MkdirAll(targetPath, 0o750); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to create target path: %v", err)
	}

	fuseFd, err := openFuse()
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to open /dev/fuse: %v", err)
	}

	if err := mountFuse(targetPath, fuseFd, uid, gid); err != nil {
		_ = unix.Close(fuseFd)
		return nil, status.Errorf(codes.Internal, "failed to mount FUSE: %v", err)
	}

	socketDir := filepath.Join(kubeletPodsDirectory, podUID, "volumes", "kubernetes.io~empty-dir", socketVolume)
	socketPath := filepath.Join(socketDir, fdpass.SocketName)
	log.Printf("NodePublishVolume: fd=%d, targetPath=%s, socketPath=%s", fuseFd, targetPath, socketPath)

	listener, err := fdpass.Listen(socketPath)
	if err != nil {
		_ = unix.Close(fuseFd)
		if unmountErr := unmountFuse(targetPath); unmountErr != nil {
			log.Printf("failed to unmount %s after listen failure: %+v", targetPath, unmountErr)
		}
		return nil, status.Errorf(codes.Internal, "failed to listen on fdpass socket: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	info := &mountInfo{
		fuseFd:     fuseFd,
		cancel:     cancel,
		socketPath: socketPath,
	}

	d.mu.Lock()
	d.mounts[targetPath] = info
	d.mu.Unlock()

	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("panic: %+v\n%s", r, debug.Stack())
			}
		}()
		defer func() {
			_ = listener.Close()
		}()

		log.Printf("fdpass.Serve: starting socketPath=%s fd=%d", socketPath, fuseFd)
		if err := fdpass.Serve(ctx, listener, fuseFd, fdTimeout, nil); err != nil {
			log.Printf("fdpass.Serve failed: %+v", err)
			d.cleanup(targetPath)
			return
		}

		log.Printf("fdpass: fd sent successfully")
		d.mu.Lock()
		_, exists := d.mounts[targetPath]
		if exists {
			info.fdClosed = true
		}
		d.mu.Unlock()
		if exists {
			_ = unix.Close(fuseFd)
		}
	}()

	return &csi.NodePublishVolumeResponse{}, nil
}

func (d *Driver) NodeUnpublishVolume(_ context.Context, request *csi.NodeUnpublishVolumeRequest) (*csi.NodeUnpublishVolumeResponse, error) {
	targetPath := request.GetTargetPath()
	if targetPath == "" {
		return nil, status.Error(codes.InvalidArgument, "target path is required")
	}

	d.cleanup(targetPath)

	return &csi.NodeUnpublishVolumeResponse{}, nil
}

func (d *Driver) NodeGetInfo(_ context.Context, _ *csi.NodeGetInfoRequest) (*csi.NodeGetInfoResponse, error) {
	return &csi.NodeGetInfoResponse{
		NodeId: d.nodeID,
	}, nil
}

func (d *Driver) NodeGetCapabilities(_ context.Context, _ *csi.NodeGetCapabilitiesRequest) (*csi.NodeGetCapabilitiesResponse, error) {
	return &csi.NodeGetCapabilitiesResponse{}, nil
}

func panicRecoveryInterceptor(ctx context.Context, request any, _ *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (_ any, err error) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("panic: %+v\n%s", r, debug.Stack())
			err = status.Errorf(codes.Internal, "panic: %v", r)
		}
	}()

	return handler(ctx, request)
}

func openFuse() (int, error) {
	fd, err := unix.Open("/dev/fuse", unix.O_RDWR, 0)
	if err != nil {
		return -1, xerrors.Errorf("failed to open /dev/fuse: %w", err)
	}
	return fd, nil
}

func mountFuse(targetPath string, fd int, uid uint32, gid uint32) error {
	options := fmt.Sprintf("fd=%d,rootmode=40000,user_id=%d,group_id=%d,allow_other", fd, uid, gid)
	if err := unix.Mount("fuse", targetPath, "fuse", 0, options); err != nil {
		return xerrors.Errorf("failed to mount FUSE at %s: %w", targetPath, err)
	}
	return nil
}

func (d *Driver) cleanup(targetPath string) {
	d.mu.Lock()
	info, exists := d.mounts[targetPath]
	var fdClosed bool
	if exists {
		fdClosed = info.fdClosed
		info.fdClosed = true
		delete(d.mounts, targetPath)
	}
	d.mu.Unlock()

	if exists {
		info.cancel()
		if !fdClosed {
			_ = unix.Close(info.fuseFd)
		}
	}

	if err := unmountFuse(targetPath); err != nil {
		log.Printf("failed to unmount %s: %+v", targetPath, err)
	}
}

func unmountFuse(targetPath string) error {
	if err := unix.Unmount(targetPath, 0); err != nil {
		if detachErr := unix.Unmount(targetPath, unix.MNT_DETACH); detachErr != nil {
			return xerrors.Errorf("failed to unmount %s: %w", targetPath, detachErr)
		}
	}
	return nil
}
