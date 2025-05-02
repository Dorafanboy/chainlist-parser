package config

import (
	"time"
)

// Config holds all configuration for the application.
type Config struct {
	App       AppConfig       `yaml:"app"`
	Server    ServerConfig    `yaml:"server"`
	Logger    LoggerConfig    `yaml:"logger"`
	Checker   CheckerConfig   `yaml:"checker"`
	Cache     CacheConfig     `yaml:"cache"`
	Chainlist ChainlistConfig `yaml:"chainlist"`
}

// AppConfig holds application-level configuration.
type AppConfig struct {
	Name    string `yaml:"name" env-default:"chainlist-parser"`
	Version string `yaml:"version" env-default:"1.0.0"`
}

// ServerConfig holds HTTP server configuration.
type ServerConfig struct {
	Port string `yaml:"port" env-default:"8080"`
}

// LoggerConfig holds logging configuration.
type LoggerConfig struct {
	Level    string `yaml:"level" env-default:"info"`
	Encoding string `yaml:"encoding" env-default:"json"`
}

// CheckerConfig holds settings related to the RPC checking process.
type CheckerConfig struct {
	CheckInterval    time.Duration `yaml:"check_interval" env-default:"15m"`
	CheckTimeout     time.Duration `yaml:"check_timeout" env-default:"5s"`
	MaxWorkers       int           `yaml:"max_workers" env-default:"10"`
	CacheTTL         time.Duration `yaml:"cache_ttl" env-default:"30m"`       // Cache TTL for checked RPCs
	RunOnStartup     bool          `yaml:"run_on_startup" env-default:"true"` // Correctly added field
	UserAgent        string        `yaml:"user_agent" env-default:"chainlist-parser/1.0"`
	ChainlistDataURL string        `yaml:"chainlist_data_url" env-default:"https://chainid.network/chains.json"`
}

// CacheConfig holds settings for the caching layer.
type CacheConfig struct {
	DefaultExpiration time.Duration `yaml:"default_expiration" env-default:"30m"`
	CleanupInterval   time.Duration `yaml:"cleanup_interval" env-default:"1h"`
}

// ChainlistConfig holds configuration for the Chainlist data source.
type ChainlistConfig struct {
	URL string `yaml:"url" env-default:"https://chainid.network/chains.json"`
}

// Load reads configuration from file and environment variables.
func Load(configPath string) (*Config, error) {
	// Implementation for loading config (using Viper or similar)
	// ... (this part is assumed to exist and work)
	// For example purposes, return a default config:
	cfg := &Config{
		App: AppConfig{
			Name:    "chainlist-parser",
			Version: "1.0.0",
		},
		Server: ServerConfig{
			Port: "8080",
		},
		Logger: LoggerConfig{
			Level:    "info",
			Encoding: "json",
		},
		Checker: CheckerConfig{
			CheckInterval:    15 * time.Minute,
			CheckTimeout:     5 * time.Second,
			MaxWorkers:       10,
			CacheTTL:         30 * time.Minute,
			RunOnStartup:     true, // Default value
			UserAgent:        "chainlist-parser/1.0",
			ChainlistDataURL: "https://chainid.network/chains.json",
		},
		Cache: CacheConfig{
			DefaultExpiration: 30 * time.Minute,
			CleanupInterval:   1 * time.Hour,
		},
		Chainlist: ChainlistConfig{
			URL: "https://chainid.network/chains.json",
		},
	}
	// Add proper loading logic here (e.g., Viper)
	return cfg, nil
}

// Helper methods to get durations

func (c CheckerConfig) GetTimeout() time.Duration {
	return c.CheckTimeout
}

func (c CheckerConfig) GetCheckInterval() time.Duration {
	return c.CheckInterval
}

func (c CheckerConfig) GetCacheTTL() time.Duration {
	return c.CacheTTL
}

func (c CacheConfig) GetDefaultExpiration() time.Duration {
	return c.DefaultExpiration
}

func (c CacheConfig) GetCleanupInterval() time.Duration {
	return c.CleanupInterval
}
