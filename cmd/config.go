package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/mitchellh/go-homedir"
	"github.com/zquestz/go-ucl"
)

// Config stores all the application configuration.
type Config struct {
	DisplayVersion   bool   `json:"-"`
	Port             int    `json:"port,string"`
	StatsPort        int    `json:"stats_port,string"`
	WebSocketPort    int    `json:"websocket_port,string"`
	Cert             string `json:"cert"`
	Key              string `json:"key"`
	PoolSize         int    `json:"pool_size,string"`
	Debug            bool   `json:"debug,string"`
	AutoCert         string `json:"auto_cert"`
	BindIP           string `json:"bind_ip"`
	Tor              bool   `json:"tor,string"`
	TorBindIP        string `json:"tor_bind_ip"`
	TorPort          int    `json:"tor_port,string"`
	TorStatsPort     int    `json:"tor_stats_port,string"`
	TorWebSocketPort int    `json:"tor_websocket_port,string"`
}

// Load reads the configuration from ~/.cashshuffle/config and loads it into the Config struct.
// The config is in UCL format.
func (c *Config) Load() error {
	conf, err := c.loadConfig()
	if err != nil {
		return err
	}

	// There are cases when we don't have a configuration.
	if conf != nil {
		err = c.applyConf(conf)
		if err != nil {
			return err
		}
	}

	return nil
}

func (c *Config) configDir() (string, error) {
	h, err := homedir.Dir()
	if err != nil {
		return "", err
	}

	return filepath.Join(h, ".cashshuffle"), nil
}

func (c *Config) loadConfig() ([]byte, error) {
	configDir, err := c.configDir()
	if err != nil {
		return nil, err
	}

	f, err := os.Open(filepath.Join(configDir, "config"))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}

		return nil, err
	}
	defer f.Close()

	ucl.Ucldebug = false
	data, err := ucl.NewParser(f).Ucl()
	if err != nil {
		return nil, err
	}

	conf, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}

	return conf, nil
}

func (c *Config) applyConf(conf []byte) error {
	return json.Unmarshal(conf, c)
}
