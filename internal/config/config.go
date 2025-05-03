package config

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/viper"
)

// Config holds all configuration for the application.
type Config struct {
	App       AppConfig       `mapstructure:"app"`
	Server    ServerConfig    `mapstructure:"server"`
	Logger    LoggerConfig    `mapstructure:"logger"`
	Checker   CheckerConfig   `mapstructure:"checker"`
	Cache     CacheConfig     `mapstructure:"cache"`
	Chainlist ChainlistConfig `mapstructure:"chainlist"`
}

// AppConfig holds application-level configuration.
type AppConfig struct {
	Name    string `mapstructure:"name"`
	Version string `mapstructure:"version"`
}

// ServerConfig holds HTTP server configuration.
type ServerConfig struct {
	Port string `mapstructure:"port"`
}

// LoggerConfig holds logging configuration.
type LoggerConfig struct {
	Level    string `mapstructure:"level"`
	Encoding string `mapstructure:"encoding"`
}

// CheckerConfig holds settings related to the RPC checking process.
type CheckerConfig struct {
	CheckInterval time.Duration `mapstructure:"check_interval"`
	CheckTimeout  time.Duration `mapstructure:"check_timeout"`
	MaxWorkers    int           `mapstructure:"max_workers"`
	CacheTTL      time.Duration `mapstructure:"cache_ttl"`
	RunOnStartup  bool          `mapstructure:"run_on_startup"`
}

// CacheConfig holds settings for the caching layer.
type CacheConfig struct {
	DefaultExpiration time.Duration `mapstructure:"default_expiration"`
	CleanupInterval   time.Duration `mapstructure:"cleanup_interval"`
}

// ChainlistConfig holds configuration for the Chainlist data source.
type ChainlistConfig struct {
	URL string `mapstructure:"url"`
}

// Load reads configuration from file and environment variables.
func Load(configPath string) (*Config, error) {
	v := viper.New()

	v.SetDefault("app.name", "chainlist-parser")
	v.SetDefault("app.version", "1.0.0")
	v.SetDefault("server.port", "8080")
	v.SetDefault("logger.level", "info")
	v.SetDefault("logger.encoding", "json")
	v.SetDefault("checker.check_interval", "15m")
	v.SetDefault("checker.check_timeout", "5s")
	v.SetDefault("checker.max_workers", 20)
	v.SetDefault("checker.cache_ttl", "30m")
	v.SetDefault("checker.run_on_startup", true)
	v.SetDefault("cache.default_expiration", "30m")
	v.SetDefault("cache.cleanup_interval", "1h")
	v.SetDefault("chainlist.url", "https://chainid.network/chains.json")

	v.SetConfigName("config")
	v.SetConfigType("yaml")
	v.AddConfigPath(configPath)
	v.AddConfigPath(".")

	if err := v.ReadInConfig(); err != nil {
		var configFileNotFoundError viper.ConfigFileNotFoundError
		if errors.As(err, &configFileNotFoundError) {
			fmt.Printf("Warning: Config file not found in %s or '.', using defaults/env vars\n", configPath)
		}
	}

	v.SetEnvPrefix("CHAINLIST_PARSER")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	return &cfg, nil
}

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
