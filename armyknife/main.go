package main

import (
	"armyknife/cmd"
	"log"
	"os"
)

func main() {
	log.SetFlags(0)

	if err := cmd.GetRootCmd(os.Args[1:]).Execute(); err != nil {
		os.Exit(1)
	}
}
