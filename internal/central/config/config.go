package config

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

// Config holds all configuration for the central server process.
type Config struct {
	API       APIConfig       `yaml:"api"`
	GRPC      GRPCConfig      `yaml:"grpc"`
	TLS       TLSConfig       `yaml:"tls"`
	RateLimit RateLimitConfig `yaml:"rate_limit"`
	Snapshots SnapshotConfig  `yaml:"snapshots"`
	Metrics   MetricsConfig   `yaml:"metrics"`
}

type APIConfig struct {
	ListenAddr   string        `yaml:"listen_addr"`
	WriteTimeout time.Duration `yaml:"write_timeout"`
	ReadTimeout  time.Duration `yaml:"read_timeout"`
}

type GRPCConfig struct {
	ListenAddr string `yaml:"listen_addr"`
}

type TLSConfig struct {
	Cert string `yaml:"cert"`
	Key  string `yaml:"key"`
	CA   string `yaml:"ca"`
}

type RateLimitConfig struct {
	RequestsPerInterval int           `yaml:"requests_per_interval"`
	Interval            time.Duration `yaml:"interval"`
	CleanupInterval     time.Duration `yaml:"cleanup_interval"`
	StaleAfter          time.Duration `yaml:"stale_after"`
}

type SnapshotConfig struct {
	Enabled   bool          `yaml:"enabled"`
	Directory string        `yaml:"directory"`
	Interval  time.Duration `yaml:"interval"`
}

type MetricsConfig struct {
	Enabled    bool   `yaml:"enabled"`
	ListenAddr string `yaml:"listen_addr"`
}

// Load reads a central configuration from a YAML file and applies
// environment variable overrides.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config file: %w", err)
	}

	cfg := Defaults()
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parse config file: %w", err)
	}

	applyEnvOverrides(cfg)
	return cfg, nil
}

func applyEnvOverrides(cfg *Config) {
	if v := os.Getenv("CENTRAL_LISTEN_ADDR"); v != "" {
		cfg.API.ListenAddr = v
	}
	if v := os.Getenv("CENTRAL_GRPC_ADDR"); v != "" {
		cfg.GRPC.ListenAddr = v
	}
	if v := os.Getenv("CENTRAL_TLS_CERT"); v != "" {
		cfg.TLS.Cert = v
	}
	if v := os.Getenv("CENTRAL_TLS_KEY"); v != "" {
		cfg.TLS.Key = v
	}
	if v := os.Getenv("CENTRAL_TLS_CA"); v != "" {
		cfg.TLS.CA = v
	}
}

// Defaults returns a Config with default values.
func Defaults() *Config {
	return &Config{
		API: APIConfig{
			ListenAddr:   ":8080",
			WriteTimeout: 120 * time.Second,
			ReadTimeout:  5 * time.Second,
		},
		GRPC: GRPCConfig{
			ListenAddr: ":9090",
		},
		RateLimit: RateLimitConfig{
			RequestsPerInterval: 1,
			Interval:            30 * time.Second,
			CleanupInterval:     1 * time.Minute,
			StaleAfter:          5 * time.Minute,
		},
		Snapshots: SnapshotConfig{
			Enabled:   false,
			Directory: "/var/lib/route-beacon/snapshots",
			Interval:  5 * time.Minute,
		},
		Metrics: MetricsConfig{
			Enabled:    false,
			ListenAddr: ":9091",
		},
	}
}
