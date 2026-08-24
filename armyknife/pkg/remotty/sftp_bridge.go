package remotty

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"errors"
	"log"
	"net"
	"runtime/debug"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
	"golang.org/x/xerrors"
)

type sftpBridge struct {
	listener      net.Listener
	configuration *ssh.ServerConfig
	directory     string
}

func newSFTPBridge(directory string, clientPublicKey ssh.PublicKey) (*sftpBridge, error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, xerrors.Errorf("failed to generate host key: %w", err)
	}

	signer, err := ssh.NewSignerFromKey(key)
	if err != nil {
		return nil, xerrors.Errorf("failed to create signer: %w", err)
	}

	authorizedKey := clientPublicKey.Marshal()
	configuration := &ssh.ServerConfig{
		PublicKeyCallback: func(conn ssh.ConnMetadata, key ssh.PublicKey) (*ssh.Permissions, error) {
			if bytes.Equal(key.Marshal(), authorizedKey) {
				return &ssh.Permissions{}, nil
			}
			return nil, xerrors.Errorf("unknown public key for %s", conn.RemoteAddr())
		},
	}
	configuration.AddHostKey(signer)

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, xerrors.Errorf("failed to create SFTP listener: %w", err)
	}

	s := &sftpBridge{
		listener:      listener,
		configuration: configuration,
		directory:     directory,
	}

	go func() {
		for {
			conn, err := s.listener.Accept()
			if err != nil {
				if errors.Is(err, net.ErrClosed) {
					return
				}
				continue
			}
			go s.handleConnection(conn)
		}
	}()

	return s, nil
}

func (s *sftpBridge) Addr() net.Addr {
	return s.listener.Addr()
}

func (s *sftpBridge) Close() error {
	return s.listener.Close()
}

func (s *sftpBridge) handleConnection(conn net.Conn) {
	defer func() {
		if err := recover(); err != nil {
			log.Printf("panic: %+v\n%s", err, debug.Stack())
		}
	}()
	defer func() {
		_ = conn.Close()
	}()

	sshConn, channels, requests, err := ssh.NewServerConn(conn, s.configuration)
	if err != nil {
		return
	}
	defer func() {
		_ = sshConn.Close()
	}()

	go ssh.DiscardRequests(requests)

	for newChannel := range channels {
		if newChannel.ChannelType() != "session" {
			_ = newChannel.Reject(ssh.UnknownChannelType, "unknown channel type")
			continue
		}

		channel, channelRequests, err := newChannel.Accept()
		if err != nil {
			continue
		}

		go func() {
			for request := range channelRequests {
				if request.Type == "subsystem" && len(request.Payload) > 4 && string(request.Payload[4:]) == "sftp" {
					_ = request.Reply(true, nil)
				} else {
					_ = request.Reply(false, nil)
				}
			}
		}()

		server, err := sftp.NewServer(channel, sftp.WithServerWorkingDirectory(s.directory))
		if err != nil {
			_ = channel.Close()
			continue
		}

		go func() {
			defer func() {
				_ = channel.Close()
			}()
			if err := server.Serve(); err != nil {
				if !errors.Is(err, sftp.ErrSSHFxConnectionLost) {
					log.Printf("sftp server error: %+v", err)
				}
			}
		}()
	}
}
