package fdpass

import (
	"context"
	"net"
	"os"
	"path/filepath"
	"time"

	"golang.org/x/sys/unix"
	"golang.org/x/xerrors"
)

const SocketName = "fuse.sock"

func Listen(socketPath string) (net.Listener, error) {
	if err := os.MkdirAll(filepath.Dir(socketPath), 0o750); err != nil {
		return nil, xerrors.Errorf("failed to create socket directory: %w", err)
	}

	_ = os.Remove(socketPath)

	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		return nil, xerrors.Errorf("failed to listen on %s: %w", socketPath, err)
	}

	if err := os.Chmod(socketPath, 0o666); err != nil {
		_ = listener.Close()
		return nil, xerrors.Errorf("failed to chmod socket %s: %w", socketPath, err)
	}

	return listener, nil
}

func Serve(ctx context.Context, listener net.Listener, fd int, timeout time.Duration, beforeSend func() error) error {
	connChan := make(chan net.Conn, 1)
	errChan := make(chan error, 1)

	defer func() {
		select {
		case conn := <-connChan:
			_ = conn.Close()
		default:
		}
	}()

	go func() {
		conn, err := listener.Accept()
		if err != nil {
			errChan <- err
			return
		}
		connChan <- conn
	}()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(timeout):
		return xerrors.Errorf("timed out waiting for connection on %s after %s", listener.Addr(), timeout)
	case err := <-errChan:
		return xerrors.Errorf("failed to accept connection on %s: %w", listener.Addr(), err)
	case conn := <-connChan:
		defer func() {
			_ = conn.Close()
		}()
		if beforeSend != nil {
			if err := beforeSend(); err != nil {
				return xerrors.Errorf("beforeSend hook failed: %w", err)
			}
		}
		if err := send(conn, fd); err != nil {
			return xerrors.Errorf("failed to send fd over %s: %w", listener.Addr(), err)
		}
		return nil
	}
}

func Receive(socketPath string) (int, error) {
	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		return -1, xerrors.Errorf("failed to connect to %s: %w", socketPath, err)
	}
	defer func() {
		_ = conn.Close()
	}()

	fd, err := receive(conn)
	if err != nil {
		return -1, xerrors.Errorf("failed to receive fd: %w", err)
	}

	return fd, nil
}

func send(conn net.Conn, fd int) error {
	unixConn, ok := conn.(*net.UnixConn)
	if !ok {
		return xerrors.New("connection is not a Unix socket")
	}

	file, err := unixConn.File()
	if err != nil {
		return xerrors.Errorf("failed to get file from connection: %w", err)
	}
	defer func() {
		_ = file.Close()
	}()

	rights := unix.UnixRights(fd)
	return unix.Sendmsg(int(file.Fd()), []byte{0}, rights, nil, 0)
}

func receive(conn net.Conn) (int, error) {
	unixConn, ok := conn.(*net.UnixConn)
	if !ok {
		return -1, xerrors.New("connection is not a Unix socket")
	}

	file, err := unixConn.File()
	if err != nil {
		return -1, xerrors.Errorf("failed to get file from connection: %w", err)
	}
	defer func() {
		_ = file.Close()
	}()

	buf := make([]byte, 1)
	oob := make([]byte, unix.CmsgSpace(4))

	_, oobn, _, _, err := unix.Recvmsg(int(file.Fd()), buf, oob, 0)
	if err != nil {
		return -1, xerrors.Errorf("failed to receive message: %w", err)
	}

	if oobn == 0 {
		return -1, xerrors.New("no control message received")
	}

	messages, err := unix.ParseSocketControlMessage(oob[:oobn])
	if err != nil {
		return -1, xerrors.Errorf("failed to parse control message: %w", err)
	}

	resultFd := -1
	for _, message := range messages {
		fds, err := unix.ParseUnixRights(&message)
		if err != nil {
			continue
		}
		for _, fd := range fds {
			if resultFd == -1 {
				resultFd = fd
			} else {
				_ = unix.Close(fd)
			}
		}
	}

	if resultFd == -1 {
		return -1, xerrors.New("no file descriptor in control message")
	}

	return resultFd, nil
}
