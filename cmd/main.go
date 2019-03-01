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
	appName                 = "cashshuffle"
	version                 = "0.6.1"
	defaultPort             = 1337
	defaultWebSocketPort    = 1338
	defaultTorPort          = 1339
	defaultTorWebSocketPort = 1340
	defaultStatsPort        = 8080
	defaultTorStatsPort     = 8081
	defaultPoolSize         = 5
	defaultTorBindIP        = "127.0.0.1"
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

	if config.WebSocketPort == 0 {
		config.WebSocketPort = defaultWebSocketPort
	}

	if config.StatsPort == 0 {
		config.StatsPort = defaultStatsPort
	}

	if config.TorBindIP == "" {
		config.TorBindIP = defaultTorBindIP
	}

	if config.TorPort == 0 {
		config.TorPort = defaultTorPort
	}

	if config.TorWebSocketPort == 0 {
		config.TorWebSocketPort = defaultTorWebSocketPort
	}

	if config.TorStatsPort == 0 {
		config.TorStatsPort = defaultTorStatsPort
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
		&config.WebSocketPort, "websocket-port", "w", config.WebSocketPort, "websocket port")
	MainCmd.PersistentFlags().IntVarP(
		&config.StatsPort, "stats-port", "z", config.StatsPort, "stats server port")
	MainCmd.PersistentFlags().IntVarP(
		&config.PoolSize, "pool-size", "s", config.PoolSize, "pool size")
	MainCmd.PersistentFlags().BoolVarP(
		&config.Debug, "debug", "d", config.Debug, "debug mode")
	MainCmd.PersistentFlags().StringVarP(
		&config.AutoCert, "auto-cert", "a", config.AutoCert, "register hostname with LetsEncrypt")
	MainCmd.PersistentFlags().StringVarP(
		&config.BindIP, "bind-ip", "b", config.BindIP, "IP address to bind to")
	MainCmd.PersistentFlags().BoolVarP(
		&config.Tor, "tor", "t", config.Tor, "enable secondary listener for tor connections")
	MainCmd.PersistentFlags().StringVarP(
		&config.TorBindIP, "tor-bind-ip", "", config.TorBindIP, "IP address to bind to for tor")
	MainCmd.PersistentFlags().IntVarP(
		&config.TorPort, "tor-port", "", config.TorPort, "tor server port")
	MainCmd.PersistentFlags().IntVarP(
		&config.TorWebSocketPort, "tor-websocket-port", "", config.TorWebSocketPort, "tor websocket port")
	MainCmd.PersistentFlags().IntVarP(
		&config.TorStatsPort, "tor-stats-port", "", config.TorStatsPort, "tor stats server port")
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

	t := server.NewTracker(config.PoolSize, config.Port, config.WebSocketPort, config.TorPort, config.TorWebSocketPort)

	m, err := getLetsEncryptManager()
	if err != nil {
		return err
	}

	// enable stats if port specified
	if config.StatsPort > 0 {
		go server.StartStatsServer(config.BindIP, config.StatsPort, config.Cert, config.Key, t, m, false)
	}

	if config.Tor && config.TorStatsPort > 0 {
		go server.StartStatsServer(config.TorBindIP, config.TorStatsPort, "", "", t, nil, true)
	}

	// enable websocket port if specified.
	if config.WebSocketPort > 0 {
		go server.StartWebsocket(config.BindIP, config.WebSocketPort, config.Cert, config.Key, config.Debug, t, m, false)
	}

	if config.Tor && config.TorWebSocketPort > 0 {
		go server.StartWebsocket(config.TorBindIP, config.TorWebSocketPort, "", "", config.Debug, t, nil, true)
	}

	// enable tor server if specified.
	if config.Tor {
		go server.Start(config.TorBindIP, config.TorPort, "", "", config.Debug, t, nil, true)
	}

	return server.Start(config.BindIP, config.Port, config.Cert, config.Key, config.Debug, t, m, false)
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
