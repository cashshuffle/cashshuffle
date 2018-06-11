package main

import (
	"fmt"
	"os"
	"os/signal"

	"github.com/cashshuffle/cashshuffle/cmd"
)

func main() {
	setupSignalHandlers()

	if err := cmd.MainCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func setupSignalHandlers() {
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)

	go interuptSignal(c)
}

func interuptSignal(c <-chan os.Signal) {
	<-c
	os.Exit(0)
}
