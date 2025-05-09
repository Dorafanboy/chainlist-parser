package config

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"gopkg.in/yaml.v3"
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
	Name    string `yaml:"name"`
	Version string `yaml:"version"`
}

// ServerConfig holds HTTP server configuration.
type ServerConfig struct {
	Port string `yaml:"port"`
}

// LoggerConfig holds logging configuration.
type LoggerConfig struct {
	Level    string `yaml:"level"`
	Encoding string `yaml:"encoding"`
}

// CheckerConfig holds settings related to the RPC checking process.
type CheckerConfig struct {
	CheckInterval time.Duration `yaml:"check_interval"`
	CheckTimeout  time.Duration `yaml:"check_timeout"`
	MaxWorkers    int           `yaml:"max_workers"`
	CacheTTL      time.Duration `yaml:"cache_ttl"`
	RunOnStartup  bool          `yaml:"run_on_startup"`
}

// CacheConfig holds settings for the caching layer.
type CacheConfig struct {
	DefaultExpiration time.Duration `yaml:"default_expiration"`
	CleanupInterval   time.Duration `yaml:"cleanup_interval"`
}

// ChainlistConfig holds configuration for the Chainlist data source.
type ChainlistConfig struct {
	URL string `yaml:"url"`
}

// rawCheckerConfig is used for unmarshalling YAML string durations for CheckerConfig.
type rawCheckerConfig struct {
	CheckInterval string `yaml:"check_interval"`
	CheckTimeout  string `yaml:"check_timeout"`
	MaxWorkers    int    `yaml:"max_workers"`
	CacheTTL      string `yaml:"cache_ttl"`
	RunOnStartup  bool   `yaml:"run_on_startup"`
}

// rawCacheConfig is used for unmarshalling YAML string durations for CacheConfig.
type rawCacheConfig struct {
	DefaultExpiration string `yaml:"default_expiration"`
	CleanupInterval   string `yaml:"cleanup_interval"`
}

// rawConfig is the top-level struct for unmarshalling the raw YAML data.
type rawConfig struct {
	App       AppConfig        `yaml:"app"`
	Server    ServerConfig     `yaml:"server"`
	Logger    LoggerConfig     `yaml:"logger"`
	Checker   rawCheckerConfig `yaml:"checker"`
	Cache     rawCacheConfig   `yaml:"cache"`
	Chainlist ChainlistConfig  `yaml:"chainlist"`
}

// LoadFromYAML reads configuration from a YAML file.
func LoadFromYAML(filePath string) (*Config, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file %s: %w", filePath, err)
	}

	var rawCfg rawConfig
	err = yaml.Unmarshal(data, &rawCfg)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal yaml data from %s: %w", filePath, err)
	}

	cfg := &Config{
		App:       rawCfg.App,
		Server:    rawCfg.Server,
		Logger:    rawCfg.Logger,
		Chainlist: rawCfg.Chainlist,
	}

	checkerInterval, err := time.ParseDuration(rawCfg.Checker.CheckInterval)
	if err != nil {
		return nil, fmt.Errorf(
			"failed to parse checker.check_interval '%s': %w", rawCfg.Checker.CheckInterval, err,
		)
	}
	checkerTimeout, err := time.ParseDuration(rawCfg.Checker.CheckTimeout)
	if err != nil {
		return nil, fmt.Errorf(
			"failed to parse checker.check_timeout '%s': %w", rawCfg.Checker.CheckTimeout, err,
		)
	}
	checkerCacheTTL, err := time.ParseDuration(rawCfg.Checker.CacheTTL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse checker.cache_ttl '%s': %w", rawCfg.Checker.CacheTTL, err)
	}
	cfg.Checker = CheckerConfig{
		CheckInterval: checkerInterval,
		CheckTimeout:  checkerTimeout,
		MaxWorkers:    rawCfg.Checker.MaxWorkers,
		CacheTTL:      checkerCacheTTL,
		RunOnStartup:  rawCfg.Checker.RunOnStartup,
	}

	cacheDefExp, err := time.ParseDuration(rawCfg.Cache.DefaultExpiration)
	if err != nil {
		return nil, fmt.Errorf(
			"failed to parse cache.default_expiration '%s': %w", rawCfg.Cache.DefaultExpiration, err,
		)
	}
	cacheCleanInt, err := time.ParseDuration(rawCfg.Cache.CleanupInterval)
	if err != nil {
		return nil, fmt.Errorf(
			"failed to parse cache.cleanup_interval '%s': %w", rawCfg.Cache.CleanupInterval, err,
		)
	}
	cfg.Cache = CacheConfig{
		DefaultExpiration: cacheDefExp,
		CleanupInterval:   cacheCleanInt,
	}

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("configuration validation failed: %w", err)
	}

	return cfg, nil
}

// GetTimeout returns the configured check timeout duration.
func (c CheckerConfig) GetTimeout() time.Duration {
	return c.CheckTimeout
}

// GetCheckInterval returns the configured check interval duration.
func (c CheckerConfig) GetCheckInterval() time.Duration {
	return c.CheckInterval
}

// GetCacheTTL returns the configured cache TTL duration.
func (c CheckerConfig) GetCacheTTL() time.Duration {
	return c.CacheTTL
}

// GetDefaultExpiration returns the configured default cache expiration.
func (c CacheConfig) GetDefaultExpiration() time.Duration {
	return c.DefaultExpiration
}

// GetCleanupInterval returns the configured cache cleanup interval.
func (c CacheConfig) GetCleanupInterval() time.Duration {
	return c.CleanupInterval
}

// Validate checks the configuration values for basic correctness.
func (c *Config) Validate() error {
	if c.Server.Port == "" {
		return fmt.Errorf("server.port must be set")
	}
	if _, err := strconv.Atoi(c.Server.Port); err != nil {
		return fmt.Errorf("server.port must be a valid number: %w", err)
	}

	if c.Logger.Level == "" {
		return fmt.Errorf("logger.level must be set")
	}

	if c.Chainlist.URL == "" {
		return fmt.Errorf("chainlist.url must be set")
	}

	if c.Checker.CheckInterval <= 0 {
		return fmt.Errorf("checker.check_interval must be positive")
	}
	if c.Checker.CheckTimeout <= 0 {
		return fmt.Errorf("checker.check_timeout must be positive")
	}
	if c.Checker.MaxWorkers <= 0 {
		return fmt.Errorf("checker.max_workers must be positive")
	}
	if c.Checker.CacheTTL <= 0 {
		return fmt.Errorf("checker.cache_ttl must be positive")
	}

	if c.Cache.DefaultExpiration <= 0 {
		return fmt.Errorf("cache.default_expiration must be positive")
	}
	if c.Cache.CleanupInterval <= 0 {
		return fmt.Errorf("cache.cleanup_interval must be positive")
	}

	return nil
}
