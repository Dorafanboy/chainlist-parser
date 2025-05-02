package config

import (
	"time"

	"github.com/spf13/viper"
)

// Config stores all configuration of the application.
// The values are read by viper from a config file or environment variable.
type Config struct {
	Server    ServerConfig    `mapstructure:"server"`
	Chainlist ChainlistConfig `mapstructure:"chainlist"`
	Checker   CheckerConfig   `mapstructure:"checker"`
	Cache     CacheConfig     `mapstructure:"cache"`
	Logger    LoggerConfig    `mapstructure:"logger"`
}

// ServerConfig stores HTTP server related config
type ServerConfig struct {
	Port int `mapstructure:"port"`
}

// ChainlistConfig stores config for Chainlist data source
type ChainlistConfig struct {
	URL string `mapstructure:"url"`
}

// CheckerConfig stores config for the RPC checker
type CheckerConfig struct {
	TimeoutSeconds       int `mapstructure:"timeout_seconds"`
	CheckIntervalMinutes int `mapstructure:"check_interval_minutes"`
}

// CacheConfig stores config for the cache
type CacheConfig struct {
	TTLMinutes             int `mapstructure:"ttl_minutes"`
	CleanupIntervalMinutes int `mapstructure:"cleanup_interval_minutes"`
}

// LoggerConfig stores logger related config
type LoggerConfig struct {
	Level    string `mapstructure:"level"`
	Encoding string `mapstructure:"encoding"`
}

// LoadConfig reads configuration from file or environment variables.
func LoadConfig(path string) (config Config, err error) {
	v := viper.New()

	v.AddConfigPath(path)
	v.SetConfigName("config") // name of config file (without extension)
	v.SetConfigType("yaml")   // REQUIRED if the config file does not have the extension in the name

	v.AutomaticEnv() // read in environment variables that match

	err = v.ReadInConfig() // Find and read the config file
	if err != nil {        // Handle errors reading the config file
		// Allow config file not found error if it doesn't exist
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return
		}
	}

	// Set default values (optional, but recommended)
	// v.SetDefault("server.port", 8080)
	// ... add more defaults

	err = v.Unmarshal(&config)
	return
}

// Helper methods to get durations

func (c CheckerConfig) GetTimeout() time.Duration {
	return time.Duration(c.TimeoutSeconds) * time.Second
}

func (c CheckerConfig) GetCheckInterval() time.Duration {
	return time.Duration(c.CheckIntervalMinutes) * time.Minute
}

func (c CacheConfig) GetTTL() time.Duration {
	return time.Duration(c.TTLMinutes) * time.Minute
}

func (c CacheConfig) GetCleanupInterval() time.Duration {
	return time.Duration(c.CleanupIntervalMinutes) * time.Minute
}
