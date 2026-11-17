package transfer

import (
	"archive/tar"
	"errors"
	"io"
	"io/fs"
	"net"
	"os"
	"path/filepath"
	"time"

	"golang.org/x/xerrors"
)

const dialTimeout = time.Minute

func Receive(address string, directory string) error {
	listener, err := net.Listen("tcp", address)
	if err != nil {
		return xerrors.Errorf("failed to listen: %w", err)
	}
	defer listener.Close()

	connection, err := listener.Accept()
	if err != nil {
		return xerrors.Errorf("failed to accept: %w", err)
	}
	defer connection.Close()

	root, err := os.OpenRoot(directory)
	if err != nil {
		return xerrors.Errorf("failed to open %s: %w", directory, err)
	}
	defer root.Close()

	reader := tar.NewReader(connection)
	for {
		header, err := reader.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return xerrors.Errorf("failed to read header: %w", err)
		}

		switch header.Typeflag {
		case tar.TypeDir:
			if err := root.Mkdir(header.Name, header.FileInfo().Mode().Perm()); err != nil && !errors.Is(err, fs.ErrExist) {
				return xerrors.Errorf("failed to create directory %s: %w", header.Name, err)
			}
		case tar.TypeSymlink:
			if err := root.Symlink(header.Linkname, header.Name); err != nil {
				return xerrors.Errorf("failed to create symlink %s: %w", header.Name, err)
			}
			continue
		case tar.TypeReg:
			file, err := root.OpenFile(header.Name, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, header.FileInfo().Mode().Perm())
			if err != nil {
				return xerrors.Errorf("failed to create file %s: %w", header.Name, err)
			}
			if _, err := io.Copy(file, reader); err != nil {
				file.Close()
				return xerrors.Errorf("failed to write file %s: %w", header.Name, err)
			}
			if err := file.Close(); err != nil {
				return xerrors.Errorf("failed to close file %s: %w", header.Name, err)
			}
		}

		if err := root.Chmod(header.Name, header.FileInfo().Mode().Perm()); err != nil {
			return xerrors.Errorf("failed to change mode of %s: %w", header.Name, err)
		}
		if err := root.Chtimes(header.Name, header.ModTime, header.ModTime); err != nil {
			return xerrors.Errorf("failed to change times of %s: %w", header.Name, err)
		}
	}

	if _, err := connection.Write([]byte{0}); err != nil {
		return xerrors.Errorf("failed to acknowledge: %w", err)
	}
	return nil
}

func Send(address string, directory string) error {
	var connection net.Conn
	deadline := time.Now().Add(dialTimeout)
	for {
		var err error
		connection, err = net.DialTimeout("tcp", address, dialTimeout)
		if err == nil {
			break
		}
		if time.Now().After(deadline) {
			return xerrors.Errorf("failed to dial: %w", err)
		}
		time.Sleep(time.Second)
	}
	defer connection.Close()

	writer := tar.NewWriter(connection)
	if err := filepath.WalkDir(directory, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if path == directory {
			return nil
		}

		info, err := entry.Info()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() && !info.IsDir() && info.Mode()&fs.ModeSymlink == 0 {
			return nil
		}

		var link string
		if info.Mode()&fs.ModeSymlink != 0 {
			if link, err = os.Readlink(path); err != nil {
				return err
			}
		}
		header, err := tar.FileInfoHeader(info, link)
		if err != nil {
			return err
		}
		if header.Name, err = filepath.Rel(directory, path); err != nil {
			return err
		}
		if err := writer.WriteHeader(header); err != nil {
			return err
		}

		if !info.Mode().IsRegular() {
			return nil
		}
		file, err := os.Open(path)
		if err != nil {
			return err
		}
		defer file.Close()
		_, err = io.Copy(writer, file)
		return err
	}); err != nil {
		return xerrors.Errorf("failed to walk %s: %w", directory, err)
	}
	if err := writer.Close(); err != nil {
		return xerrors.Errorf("failed to close archive: %w", err)
	}

	if _, err := io.ReadFull(connection, make([]byte, 1)); err != nil {
		return xerrors.Errorf("failed to read acknowledgement: %w", err)
	}
	return nil
}
