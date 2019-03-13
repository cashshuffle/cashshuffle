package cmd

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/cashshuffle/cashshuffle/server"

	"github.com/spf13/cobra"
	"github.com/ulule/limiter/v3"
	"github.com/ulule/limiter/v3/drivers/store/memory"
	"golang.org/x/crypto/acme/autocert"
)

const (
	appName                 = "cashshuffle"
	version                 = "0.6.9"
	defaultPort             = 1337
	defaultWebSocketPort    = 1338
	defaultTorPort          = 1339
	defaultTorWebSocketPort = 1340
	defaultStatsPort        = 8080
	defaultTorStatsPort     = 8081
	defaultPoolSize         = 5
	defaultTorBindIP        = "127.0.0.1"

	ipRateLimit    = "180-M"
	torIPRateLimit = "500-M"
)

// Stores configuration data.
var config Config

// MainCmd is the main command for Cobra.
var MainCmd = &cobra.Command{
	Use:   "cashshuffle",
	Short: "CashShuffle server.",
	Long:  `CashShuffle server.`,
	Run: func(cmd *cobra.Command, args []string) {
		errChan := performCommand(cmd, args)
		handleServerErrors(errChan)
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
func performCommand(cmd *cobra.Command, args []string) chan error {
	errChan := make(chan error)

	if config.DisplayVersion {
		fmt.Printf("%s %s\n", appName, version)
		os.Exit(0)
	}

	if config.AutoCert != "" && (config.Cert != "" || config.Key != "") {
		errChan <- errors.New("can't specify auto-cert and key/cert")
		return errChan
	}

	t := server.NewTracker(config.PoolSize, config.Port, config.WebSocketPort, config.TorPort, config.TorWebSocketPort)

	cleanupDeniedTicker := time.NewTicker(time.Minute)
	defer cleanupDeniedTicker.Stop()
	go func() {
		for range cleanupDeniedTicker.C {
			t.CleanupDeniedByIPMatch()
		}
	}()

	m, err := getLetsEncryptManager()
	if err != nil {
		errChan <- err
		return errChan
	}

	limit, torLimit, err := getLimiters()
	if err != nil {
		errChan <- err
		return errChan
	}

	// enable stats if port specified
	if config.StatsPort > 0 {
		go func() {
			errChan <- server.StartStatsServer(config.BindIP, config.StatsPort, config.Cert, config.Key, t, m, false, limit)
		}()
	}

	if config.Tor && config.TorStatsPort > 0 {
		go func() {
			errChan <- server.StartStatsServer(config.TorBindIP, config.TorStatsPort, "", "", t, nil, true, torLimit)
		}()
	}

	// enable websocket port if specified.
	if config.WebSocketPort > 0 {
		go func() {
			errChan <- server.StartWebsocket(config.BindIP, config.WebSocketPort, config.Cert, config.Key, config.Debug, t, m, false, limit)
		}()
	}

	if config.Tor && config.TorWebSocketPort > 0 {
		go func() {
			errChan <- server.StartWebsocket(config.TorBindIP, config.TorWebSocketPort, "", "", config.Debug, t, nil, true, torLimit)
		}()
	}

	// enable tor server if specified.
	if config.Tor {
		go func() {
			errChan <- server.Start(config.TorBindIP, config.TorPort, "", "", config.Debug, t, nil, true, torLimit)
		}()
	}

	go func() {
		errChan <- server.Start(config.BindIP, config.Port, config.Cert, config.Key, config.Debug, t, m, false, limit)
	}()

	return errChan
}

func getLimiters() (*limiter.Limiter, *limiter.Limiter, error) {
	var rate limiter.Rate

	rate, err := limiter.NewRateFromFormatted(ipRateLimit)
	if err != nil {
		return nil, nil, err
	}

	torRate, err := limiter.NewRateFromFormatted(torIPRateLimit)
	if err != nil {
		return nil, nil, err
	}

	limit := limiter.New(memory.NewStore(), rate, limiter.WithTrustForwardHeader(true))
	torLimit := limiter.New(memory.NewStore(), torRate)

	return limit, torLimit, nil
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

func handleServerErrors(c chan error) {
	err := <-c
	if err != nil {
		bail(err)
	}
}
