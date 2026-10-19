package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
)

func main() {
	if len(os.Args) < 2 {
		os.Exit(1)
	}

	var args []string
	if len(os.Args) > 2 {
		args = os.Args[2:]
	}

	cmd := exec.Command(os.Args[1], args...)
	cmd.Stdin = os.Stdin
	cmd.Stderr = os.Stderr

	output, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			os.Stderr.Write(exitErr.Stderr)
		}
		os.Exit(1)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(output, &result); err != nil {
		return
	}

	if githubOutput := os.Getenv("GITHUB_OUTPUT"); githubOutput != "" {
		f, err := os.OpenFile(githubOutput, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			return
		}
		defer f.Close()

		for key, value := range result {
			_, _ = fmt.Fprintf(f, "%s=%v\n", key, value)
		}
	}
}
