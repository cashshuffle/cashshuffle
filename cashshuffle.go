package main

import (
	"fmt"
	"os"

	"github.com/cashshuffle/cashshuffle/cmd"
)

func main() {
	setupSignalHandlers()

	if err := cmd.MainCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
