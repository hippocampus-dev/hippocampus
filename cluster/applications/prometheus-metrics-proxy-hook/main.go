package main

import (
	"os"
	"prometheus-metrics-proxy-hook/cmd"
)

func main() {
	if err := cmd.GetRootCmd(os.Args[1:]).Execute(); err != nil {
		os.Exit(1)
	}
}
