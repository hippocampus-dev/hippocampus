package main

import (
	"flag"
	"fmt"
	"fuse-csi-driver/internal/fdpass"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
)

func main() {
	var placeholder string
	flag.StringVar(&placeholder, "I", "{}", "Placeholder in command args to replace with /dev/fd/N")
	flag.Parse()

	if placeholder == "" {
		log.Fatal("placeholder (-I) must not be empty")
	}

	args := flag.Args()
	if len(args) < 2 {
		log.Fatal("Usage: fuser [-I placeholder] DIRECTORY -- command [args...]")
	}

	socketPath := filepath.Join(args[0], fdpass.SocketName)
	command := args[1:]

	fd, err := fdpass.Receive(socketPath)
	if err != nil {
		log.Fatalf("failed to receive fd: %+v", err)
	}

	fdPath := fmt.Sprintf("/dev/fd/%d", fd)
	for i, arg := range command {
		command[i] = strings.ReplaceAll(arg, placeholder, fdPath)
	}

	binary, err := exec.LookPath(command[0])
	if err != nil {
		log.Fatalf("failed to find binary %s: %+v", command[0], err)
	}

	if err := syscall.Exec(binary, command, os.Environ()); err != nil {
		log.Fatalf("failed to exec %s: %+v", binary, err)
	}
}
