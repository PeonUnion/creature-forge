package config

// Package config loads application configuration from a YAML file using
// spf13/viper, with environment-variable overrides. Sane defaults are applied
// when fields are absent.

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/spf13/viper"
)

// Server configures the HTTP server.
type Server struct {
	Host string `mapstructure:"host" yaml:"host"`
	Port int    `mapstructure:"port" yaml:"port"`
	Dev  bool   `mapstructure:"dev" yaml:"dev"`
}

// Data configures the data root.
type Data struct {
	// Root is the data directory containing species/ presets/ skins/
	// (default: ./data).
	Root string `mapstructure:"root" yaml:"root"`
}

// Log configures the logging package.
type Log struct {
	Level  string `mapstructure:"level" yaml:"level"`   // debug|info|warn|error
	Format string `mapstructure:"format" yaml:"format"` // console|json
	Output string `mapstructure:"output" yaml:"output"` // ""|stderr|file path
}

// Config is the root application configuration.
type Config struct {
	Server Server `mapstructure:"server" yaml:"server"`
	Data   Data   `mapstructure:"data" yaml:"data"`
	Log    Log    `mapstructure:"log" yaml:"log"`
}

// Default returns the built-in defaults.
func Default() *Config {
	return &Config{
		Server: Server{Host: "127.0.0.1", Port: 8765, Dev: false},
		Data:   Data{Root: "./data"},
		Log:    Log{Level: "info", Format: "console"},
	}
}

// Load reads config from the YAML file at path (if it exists), overlays
// environment variables (prefix "CFG_", nested keys via "__"), and falls back
// to defaults for anything missing.
//
// Env override examples:
//
//	CFG_SERVER_PORT=9000
//	CFG_DATA_ROOT=/srv/creatureforge/data
//	CFG_LOG_LEVEL=debug
func Load(path string) (*Config, error) {
	cfg := Default()

	v := viper.New()
	v.SetConfigFile(path)
	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			if _, statErr := os.Stat(path); statErr != nil && !os.IsNotExist(statErr) {
				return nil, fmt.Errorf("read config %s: %w", path, err)
			}
		}
	} else {
		if err := v.Unmarshal(cfg); err != nil {
			return nil, fmt.Errorf("parse config %s: %w", path, err)
		}
	}

	// Environment overrides (highest precedence): CFG_<KEY> with "__" for nesting.
	cfg.Server.Host = envStr("CFG_SERVER_HOST", cfg.Server.Host)
	cfg.Server.Port = envInt("CFG_SERVER_PORT", cfg.Server.Port)
	cfg.Server.Dev = envBool("CFG_SERVER_DEV", cfg.Server.Dev)
	cfg.Data.Root = envStr("CFG_DATA_ROOT", cfg.Data.Root)
	cfg.Log.Level = envStr("CFG_LOG_LEVEL", cfg.Log.Level)
	cfg.Log.Format = envStr("CFG_LOG_FORMAT", cfg.Log.Format)
	cfg.Log.Output = envStr("CFG_LOG_OUTPUT", cfg.Log.Output)
	return cfg, nil
}

func envStr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func envInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}

func envBool(key string, def bool) bool {
	if v := os.Getenv(key); v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			return b
		}
	}
	return def
}

// DataDir returns the absolute path of the data root.
func (c *Config) DataDir() (string, error) {
	if c.Data.Root == "" {
		return filepath.Abs("./data")
	}
	return filepath.Abs(c.Data.Root)
}
