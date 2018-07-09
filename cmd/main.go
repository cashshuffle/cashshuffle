package cmd

import (
	"fmt"
	"os"

	"github.com/cashshuffle/cashshuffle/server"

	"github.com/spf13/cobra"
)

const (
	appName         = "cashshuffle"
	version         = "0.1.1"
	defaultPort     = 8080
	defaultPoolSize = 5
)

// Stores configuration data.
var config Config

// MainCmd is the main command for Cobra.
var MainCmd = &cobra.Command{
	Use:   "cashshuffle",
	Short: "CoinShuffle server.",
	Long:  `CoinShuffle server.`,
	Run: func(cmd *cobra.Command, args []string) {
		err := performCommand(cmd, args)
		if err != nil {
			bail(err)
		}
	},
}

func init() {
	err := config.Load()
	if err != nil {
		bail(fmt.Errorf("Failed to load configuration: %s", err))
	}

	prepareFlags()
}

func bail(err error) {
	fmt.Fprintf(os.Stderr, "[Error] %s\n", err)
	os.Exit(1)
}

func prepareFlags() {
	if config.Port == 0 {
		config.Port = defaultPort
	}

	if config.PoolSize == 0 {
		config.PoolSize = defaultPoolSize
	}

	MainCmd.PersistentFlags().StringVarP(
		&config.Cert, "cert", "c", config.Cert, "path to server.crt for TLS")
	MainCmd.PersistentFlags().StringVarP(
		&config.Key, "key", "k", config.Key, "path to server.key for TLS")
	MainCmd.PersistentFlags().BoolVarP(
		&config.DisplayVersion, "version", "v", false, "display version")
	MainCmd.PersistentFlags().IntVarP(
		&config.Port, "port", "p", config.Port, "server port")
	MainCmd.PersistentFlags().IntVarP(
		&config.StatsPort, "stats-port", "z", config.StatsPort, "stats server port (default: disabled)")
	MainCmd.PersistentFlags().IntVarP(
		&config.PoolSize, "pool-size", "s", config.PoolSize, "pool size")
	MainCmd.PersistentFlags().BoolVarP(
		&config.Debug, "debug", "d", config.Debug, "debug mode")
}

// Where all the work happens.
func performCommand(cmd *cobra.Command, args []string) error {
	if config.DisplayVersion {
		fmt.Printf("%s %s\n", appName, version)
		return nil
	}

	t := server.NewTracker(config.PoolSize)

	// enable stats if port specified
	if config.StatsPort > 0 {
		go server.StartStatsServer(config.StatsPort, config.Cert, config.Key, t)
	}

	return server.Start(config.Port, config.Cert, config.Key, t, config.Debug)
}
