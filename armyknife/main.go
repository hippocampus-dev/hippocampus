package main

import (
	"armyknife/cmd"
	"os"
)

func main() {
	if err := cmd.GetRootCmd(os.Args[1:]).Execute(); err != nil {
		os.Exit(1)
	}
}
