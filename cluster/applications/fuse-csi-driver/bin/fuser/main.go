package main

import (
	"flag"
	"fmt"
	"fuse-csi-driver/internal/fdpass"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
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

func main() {
	var placeholder string
	flag.StringVar(&placeholder, "I", envOrDefaultValue("PLACEHOLDER", "{}"), "Placeholder in command args to replace with /dev/fd/N")
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
