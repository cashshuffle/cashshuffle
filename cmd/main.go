package cmd

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"

	"github.com/cashshuffle/cashshuffle/server"
	"golang.org/x/crypto/acme/autocert"

	"github.com/spf13/cobra"
)

const (
	appName         = "cashshuffle"
	version         = "0.2.0"
	defaultPort     = 8080
	defaultPoolSize = 5
)

// Stores configuration data.
var config Config

// MainCmd is the main command for Cobra.
var MainCmd = &cobra.Command{
	Use:   "cashshuffle",
	Short: "CashShuffle server.",
	Long:  `CashShuffle server.`,
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
		&config.StatsPort, "stats-port", "z", config.StatsPort, "stats server port (default disabled)")
	MainCmd.PersistentFlags().IntVarP(
		&config.PoolSize, "pool-size", "s", config.PoolSize, "pool size")
	MainCmd.PersistentFlags().BoolVarP(
		&config.Debug, "debug", "d", config.Debug, "debug mode")
	MainCmd.PersistentFlags().StringVarP(
		&config.AutoCert, "auto-cert", "a", config.AutoCert, "register hostname with LetsEncrypt")
}

// Where all the work happens.
func performCommand(cmd *cobra.Command, args []string) error {
	if config.DisplayVersion {
		fmt.Printf("%s %s\n", appName, version)
		return nil
	}

	if config.AutoCert != "" && (config.Cert != "" || config.Key != "") {
		return errors.New("can't specify auto-cert and key/cert")
	}

	t := server.NewTracker(config.PoolSize)

	m, err := getLetsEncryptManager()
	if err != nil {
		return err
	}

	// enable stats if port specified
	if config.StatsPort > 0 {
		go server.StartStatsServer(config.StatsPort, config.Cert, config.Key, t, m)
	}

	return server.Start(config.Port, config.Cert, config.Key, config.Debug, t, m)
}

func getLetsEncryptManager() (*autocert.Manager, error) {
	configDir, err := config.configDir()
	if err != nil {
		return nil, err
	}

	certDir := filepath.Join(configDir, "certs")
	if _, err := os.Stat(certDir); os.IsNotExist(err) {
		err = os.MkdirAll(certDir, 0755)
		if err != nil {
			return nil, err
		}
	}

	if config.AutoCert == "" {
		return nil, nil
	}

	m := &autocert.Manager{
		Cache:      autocert.DirCache(certDir),
		Prompt:     autocert.AcceptTOS,
		HostPolicy: autocert.HostWhitelist(config.AutoCert),
	}

	go http.ListenAndServe(":http", m.HTTPHandler(nil))

	return m, nil
}
