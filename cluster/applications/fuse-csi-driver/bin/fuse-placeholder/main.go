package main

import (
	"context"
	"encoding/binary"
	"errors"
	"flag"
	"fmt"
	"fuse-csi-driver/internal/fdpass"
	"io"
	"log"
	"math"
	"os"
	"os/signal"
	"path/filepath"
	"runtime/debug"
	"syscall"
	"time"

	"golang.org/x/sys/unix"
)

// https://elixir.bootlin.com/linux/v6.10.6/source/include/uapi/linux/fuse.h
const (
	fuseKernelVersion      = 7
	fuseKernelMinorVersion = 28
	fuseMinorVersionMin    = 12

	fuseRootID = 1

	fuseInHeaderSize  = 40
	fuseOutHeaderSize = 16
	fuseInitOutSize   = 64
	fuseAttrOutSize   = 104
	fuseStatfsOutSize = 80

	maxWrite    = 128 * 1024
	readBufSize = maxWrite + 4096
)

// https://elixir.bootlin.com/linux/v6.10.6/source/include/uapi/linux/fuse.h#L999
func putOutHeader(buf []byte, length int, errno int32, unique uint64) {
	binary.LittleEndian.PutUint32(buf[0:4], uint32(length))
	binary.LittleEndian.PutUint32(buf[4:8], uint32(errno))
	binary.LittleEndian.PutUint64(buf[8:16], unique)
}

func writeError(fd int, unique uint64, errno unix.Errno) error {
	var buf [fuseOutHeaderSize]byte
	putOutHeader(buf[:], fuseOutHeaderSize, -int32(errno), unique)
	_, err := unix.Write(fd, buf[:])
	return err
}

func writeResponse(fd int, unique uint64, payload []byte) error {
	total := fuseOutHeaderSize + len(payload)
	buf := make([]byte, total)
	putOutHeader(buf[:fuseOutHeaderSize], total, 0, unique)
	copy(buf[fuseOutHeaderSize:], payload)
	_, err := unix.Write(fd, buf)
	return err
}

// https://elixir.bootlin.com/linux/v6.10.6/source/include/uapi/linux/fuse.h#L585
const (
	opLookup      = uint32(1)
	opForget      = uint32(2)
	opGetattr     = uint32(3)
	opStatfs      = uint32(17)
	opFlush       = uint32(25)
	opInit        = uint32(26)
	opOpendir     = uint32(27)
	opReaddir     = uint32(28)
	opReleasedir  = uint32(29)
	opAccess      = uint32(34)
	opInterrupt   = uint32(36)
	opDestroy     = uint32(38)
	opBatchForget = uint32(42)
)

// https://elixir.bootlin.com/linux/v6.10.6/source/include/uapi/linux/fuse.h#L987
type fuseInHeader struct {
	length uint32
	opcode uint32
	unique uint64
	nodeID uint64
	uid    uint32
	gid    uint32
	pid    uint32
}

func parseFuseInHeader(buf []byte) fuseInHeader {
	return fuseInHeader{
		length: binary.LittleEndian.Uint32(buf[0:4]),
		opcode: binary.LittleEndian.Uint32(buf[4:8]),
		unique: binary.LittleEndian.Uint64(buf[8:16]),
		nodeID: binary.LittleEndian.Uint64(buf[16:24]),
		uid:    binary.LittleEndian.Uint32(buf[24:28]),
		gid:    binary.LittleEndian.Uint32(buf[28:32]),
		pid:    binary.LittleEndian.Uint32(buf[32:36]),
	}
}

// https://elixir.bootlin.com/linux/v6.10.6/source/include/uapi/linux/fuse.h#L883
func fuseInit(fd int) error {
	buf := make([]byte, readBufSize)
	n, err := unix.Read(fd, buf)
	if err != nil {
		return err
	}
	if n < fuseInHeaderSize+8 {
		return errors.New("short FUSE_INIT request")
	}

	header := parseFuseInHeader(buf[:fuseInHeaderSize])
	if header.opcode != opInit {
		return errors.New("first request was not FUSE_INIT")
	}

	payload := buf[fuseInHeaderSize:n]
	kernelMajor := binary.LittleEndian.Uint32(payload[0:4])
	kernelMinor := binary.LittleEndian.Uint32(payload[4:8])
	if kernelMajor != fuseKernelVersion || kernelMinor < fuseMinorVersionMin {
		_ = writeError(fd, header.unique, unix.EIO)
		return fmt.Errorf("unsupported kernel FUSE version %d.%d", kernelMajor, kernelMinor)
	}

	respondMinor := kernelMinor
	if respondMinor > fuseKernelMinorVersion {
		respondMinor = fuseKernelMinorVersion
	}

	var out [fuseInitOutSize]byte
	binary.LittleEndian.PutUint32(out[0:4], fuseKernelVersion)
	binary.LittleEndian.PutUint32(out[4:8], respondMinor)
	binary.LittleEndian.PutUint32(out[8:12], 0)
	binary.LittleEndian.PutUint32(out[12:16], 0)
	binary.LittleEndian.PutUint16(out[16:18], 12)
	binary.LittleEndian.PutUint16(out[18:20], 9)
	binary.LittleEndian.PutUint32(out[20:24], maxWrite)
	binary.LittleEndian.PutUint32(out[24:28], 0)
	binary.LittleEndian.PutUint16(out[28:30], uint16(maxWrite/4096))
	binary.LittleEndian.PutUint16(out[30:32], 0)

	return writeResponse(fd, header.unique, out[:])
}

func fuseDispatch(fd int, header fuseInHeader) error {
	switch header.opcode {
	case opInit:
		return writeError(fd, header.unique, unix.EIO)

	case opGetattr:
		if header.nodeID != fuseRootID {
			return writeError(fd, header.unique, unix.ENOENT)
		}
		// https://elixir.bootlin.com/linux/v6.10.6/source/include/uapi/linux/fuse.h#L694
		// https://elixir.bootlin.com/linux/v6.10.6/source/include/uapi/linux/fuse.h#L263
		var out [fuseAttrOutSize]byte
		binary.LittleEndian.PutUint64(out[0:8], 1)
		binary.LittleEndian.PutUint32(out[8:12], 0)
		binary.LittleEndian.PutUint32(out[12:16], 0)

		binary.LittleEndian.PutUint64(out[16:24], fuseRootID)
		binary.LittleEndian.PutUint64(out[24:32], 0)
		binary.LittleEndian.PutUint64(out[32:40], 0)
		binary.LittleEndian.PutUint64(out[40:48], 0)
		binary.LittleEndian.PutUint64(out[48:56], 0)
		binary.LittleEndian.PutUint64(out[56:64], 0)
		binary.LittleEndian.PutUint32(out[64:68], 0)
		binary.LittleEndian.PutUint32(out[68:72], 0)
		binary.LittleEndian.PutUint32(out[72:76], 0)
		binary.LittleEndian.PutUint32(out[76:80], unix.S_IFDIR|0o555)
		binary.LittleEndian.PutUint32(out[80:84], 2)
		binary.LittleEndian.PutUint32(out[84:88], 0)
		binary.LittleEndian.PutUint32(out[88:92], 0)
		binary.LittleEndian.PutUint32(out[92:96], 0)
		binary.LittleEndian.PutUint32(out[96:100], 4096)
		binary.LittleEndian.PutUint32(out[100:104], 0)
		return writeResponse(fd, header.unique, out[:])

	case opLookup:
		return writeError(fd, header.unique, unix.ENOENT)

	case opStatfs:
		// https://elixir.bootlin.com/linux/v6.10.6/source/include/uapi/linux/fuse.h#L825
		// https://elixir.bootlin.com/linux/v6.10.6/source/include/uapi/linux/fuse.h#L316
		var out [fuseStatfsOutSize]byte
		binary.LittleEndian.PutUint32(out[40:44], 4096)
		binary.LittleEndian.PutUint32(out[44:48], 255)
		binary.LittleEndian.PutUint32(out[48:52], 4096)
		return writeResponse(fd, header.unique, out[:])

	case opOpendir:
		// https://elixir.bootlin.com/linux/v6.10.6/source/include/uapi/linux/fuse.h#L776
		var out [16]byte
		return writeResponse(fd, header.unique, out[:])

	case opReaddir:
		return writeResponse(fd, header.unique, nil)

	case opReleasedir:
		return writeResponse(fd, header.unique, nil)

	case opAccess:
		return writeResponse(fd, header.unique, nil)

	case opFlush:
		return writeResponse(fd, header.unique, nil)

	case opForget, opBatchForget, opInterrupt:
		return nil

	case opDestroy:
		_ = writeResponse(fd, header.unique, nil)
		return io.EOF

	default:
		return writeError(fd, header.unique, unix.ENOSYS)
	}
}

func fuseLoop(fd int, stopFd int) error {
	buf := make([]byte, readBufSize)
	pollFds := []unix.PollFd{
		{Fd: int32(fd), Events: unix.POLLIN},
		{Fd: int32(stopFd), Events: unix.POLLIN},
	}
	for {
		_, err := unix.Poll(pollFds, -1)
		if err != nil {
			if errors.Is(err, unix.EINTR) {
				continue
			}
			return err
		}
		if pollFds[1].Revents != 0 {
			return nil
		}
		if pollFds[0].Revents&(unix.POLLERR|unix.POLLHUP|unix.POLLNVAL) != 0 {
			return nil
		}
		if pollFds[0].Revents&unix.POLLIN == 0 {
			continue
		}

		n, err := unix.Read(fd, buf)
		if err != nil {
			if errors.Is(err, unix.EINTR) || errors.Is(err, unix.EAGAIN) {
				continue
			}
			if errors.Is(err, unix.EBADF) || errors.Is(err, unix.ENODEV) || errors.Is(err, io.EOF) {
				return nil
			}
			return err
		}
		if n == 0 {
			return nil
		}
		if n < fuseInHeaderSize {
			continue
		}
		header := parseFuseInHeader(buf[:fuseInHeaderSize])
		if err := fuseDispatch(fd, header); err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			return err
		}
	}
}

func main() {
	var handoffPath string
	flag.StringVar(&handoffPath, "handoff", "/run/sock/handoff/fuse.sock", "Handoff socket path where the main container can take over the FUSE fd")
	flag.Parse()

	args := flag.Args()
	if len(args) != 1 {
		log.Fatal("Usage: fuse-placeholder [--handoff path] DIRECTORY")
	}

	socketPath := filepath.Join(args[0], fdpass.SocketName)
	fuseFd, err := fdpass.Receive(socketPath)
	if err != nil {
		log.Fatalf("failed to receive fd from %s: %+v", socketPath, err)
	}

	if err := fuseInit(fuseFd); err != nil {
		log.Fatalf("FUSE_INIT failed: %+v", err)
	}

	stopFds := make([]int, 2)
	if err := unix.Pipe(stopFds); err != nil {
		log.Fatalf("failed to create stop pipe: %+v", err)
	}
	stopR := stopFds[0]
	stopW := stopFds[1]

	done := make(chan struct{})
	go func() {
		defer close(done)
		defer func() {
			if r := recover(); r != nil {
				log.Printf("panic: %+v\n%s", r, debug.Stack())
			}
		}()
		if err := fuseLoop(fuseFd, stopR); err != nil {
			log.Printf("FUSE serve loop exited: %+v", err)
		}
	}()

	handoffListener, err := fdpass.Listen(handoffPath)
	if err != nil {
		log.Fatalf("failed to listen on handoff socket %s: %+v", handoffPath, err)
	}
	err = fdpass.Serve(context.Background(), handoffListener, fuseFd, time.Duration(math.MaxInt64), func() error {
		_, _ = unix.Write(stopW, []byte{0})
		<-done
		_ = unix.Close(stopW)
		_ = unix.Close(stopR)
		return nil
	})
	_ = handoffListener.Close()
	if err != nil {
		log.Fatalf("failed to hand off fd via %s: %+v", handoffPath, err)
	}

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGTERM, syscall.SIGINT)
	<-quit
}
