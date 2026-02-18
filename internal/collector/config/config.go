package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Config holds all configuration for the collector process.
type Config struct {
	Collector CollectorConfig `yaml:"collector"`
	Central   CentralConfig   `yaml:"central"`
	TLS       TLSConfig       `yaml:"tls"`
	BGP       BGPConfig       `yaml:"bgp"`
	Commands  CommandsConfig  `yaml:"commands"`
}

type CollectorConfig struct {
	ID       string `yaml:"id"`
	Location string `yaml:"location"`
}

type CentralConfig struct {
	Address string `yaml:"address"`
}

type TLSConfig struct {
	Cert string `yaml:"cert"`
	Key  string `yaml:"key"`
	CA   string `yaml:"ca"`
}

type BGPConfig struct {
	LocalASN   uint32       `yaml:"local_asn"`
	RouterID   string       `yaml:"router_id"`
	ListenPort int          `yaml:"listen_port"`
	Peers      []PeerConfig `yaml:"peers"`
}

type PeerConfig struct {
	Neighbor       string `yaml:"neighbor"`
	ASN            uint32 `yaml:"asn"`
	DisplayName    string `yaml:"display_name"`
	Passive        bool   `yaml:"passive"`
	AddPathReceive bool   `yaml:"addpath_receive"`
}

type CommandsConfig struct {
	Ping        PingConfig        `yaml:"ping"`
	Traceroute  TracerouteConfig  `yaml:"traceroute"`
	Concurrency ConcurrencyConfig `yaml:"concurrency"`
}

type PingConfig struct {
	Binary           string `yaml:"binary"`
	MaxCount         uint8  `yaml:"max_count"`
	MaxTimeoutMs     uint32 `yaml:"max_timeout_ms"`
	DefaultCount     uint8  `yaml:"default_count"`
	DefaultTimeoutMs uint32 `yaml:"default_timeout_ms"`
}

type TracerouteConfig struct {
	Binary           string `yaml:"binary"`
	MaxHops          uint8  `yaml:"max_hops"`
	MaxTimeoutMs     uint32 `yaml:"max_timeout_ms"`
	DefaultMaxHops   uint8  `yaml:"default_max_hops"`
	DefaultTimeoutMs uint32 `yaml:"default_timeout_ms"`
}

type ConcurrencyConfig struct {
	PerCollector int `yaml:"per_collector"`
}

// Load reads a collector configuration from a YAML file and applies
// environment variable overrides.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config file: %w", err)
	}

	cfg := &Config{}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parse config file: %w", err)
	}

	applyEnvOverrides(cfg)
	return cfg, nil
}

func applyEnvOverrides(cfg *Config) {
	if v := os.Getenv("COLLECTOR_ID"); v != "" {
		cfg.Collector.ID = v
	}
	if v := os.Getenv("COLLECTOR_LOCATION"); v != "" {
		cfg.Collector.Location = v
	}
	if v := os.Getenv("COLLECTOR_CENTRAL_ADDR"); v != "" {
		cfg.Central.Address = v
	}
	if v := os.Getenv("COLLECTOR_TLS_CERT"); v != "" {
		cfg.TLS.Cert = v
	}
	if v := os.Getenv("COLLECTOR_TLS_KEY"); v != "" {
		cfg.TLS.Key = v
	}
	if v := os.Getenv("COLLECTOR_TLS_CA"); v != "" {
		cfg.TLS.CA = v
	}
}

// Defaults returns a Config with default values.
func Defaults() *Config {
	return &Config{
		Central: CentralConfig{
			Address: "localhost:9090",
		},
		BGP: BGPConfig{
			ListenPort: -1,
		},
		Commands: CommandsConfig{
			Ping: PingConfig{
				Binary:           "/usr/bin/ping",
				MaxCount:         10,
				MaxTimeoutMs:     10000,
				DefaultCount:     5,
				DefaultTimeoutMs: 5000,
			},
			Traceroute: TracerouteConfig{
				Binary:           "/usr/bin/traceroute",
				MaxHops:          64,
				MaxTimeoutMs:     10000,
				DefaultMaxHops:   30,
				DefaultTimeoutMs: 5000,
			},
			Concurrency: ConcurrencyConfig{
				PerCollector: 5,
			},
		},
	}
}
