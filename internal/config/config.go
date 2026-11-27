// Package config loads application settings from a JSON file and environment variables.
package config

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// Config holds the runtime settings for the serve command.
type Config struct {
	DBHost     string `json:"db_host"`
	DBPort     int    `json:"db_port"`
	DBUser     string `json:"db_user"`
	DBPassword string `json:"db_password"`
	DBName     string `json:"db_name"`
	HTTPAddr   string `json:"http_addr"`
	APIToken   string `json:"api_token"`
}

// Default returns only non-sensitive settings. Database credentials and the
// host must be supplied through config.json or environment variables.
func Default() Config {
	return Config{
		DBPort:   3306,
		HTTPAddr: ":8080",
	}
}

// Load reads an optional JSON file, then applies environment variables.
// Environment variables take precedence over values from the file.
func Load(path string) (Config, error) {
	cfg := Default()

	if strings.TrimSpace(path) != "" {
		data, err := os.ReadFile(path)
		if err != nil {
			return Config{}, fmt.Errorf("read config %q: %w", path, err)
		}
		if err := json.Unmarshal(data, &cfg); err != nil {
			return Config{}, fmt.Errorf("parse config %q: %w", path, err)
		}
	}

	applyEnv(&cfg)

	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func applyEnv(cfg *Config) {
	setString("DB_HOST", &cfg.DBHost)
	setString("DB_USER", &cfg.DBUser)
	setString("DB_PASSWORD", &cfg.DBPassword)
	setString("DB_NAME", &cfg.DBName)
	setString("HTTP_ADDR", &cfg.HTTPAddr)
	setString("API_TOKEN", &cfg.APIToken)

	if value, ok := os.LookupEnv("DB_PORT"); ok {
		port, err := strconv.Atoi(strings.TrimSpace(value))
		if err != nil {
			// Let Validate report a consistent error message.
			cfg.DBPort = 0
			return
		}
		cfg.DBPort = port
	}
}

func setString(key string, dst *string) {
	if value, ok := os.LookupEnv(key); ok {
		*dst = value
	}
}

// Validate checks that the config is safe to use before opening connections.
func (c Config) Validate() error {
	switch {
	case strings.TrimSpace(c.DBHost) == "":
		return fmt.Errorf("DB_HOST is required")
	case c.DBPort < 1 || c.DBPort > 65535:
		return fmt.Errorf("DB_PORT must be between 1 and 65535, got %d", c.DBPort)
	case strings.TrimSpace(c.DBUser) == "":
		return fmt.Errorf("DB_USER is required")
	case c.DBPassword == "":
		return fmt.Errorf("DB_PASSWORD is required")
	case strings.TrimSpace(c.DBName) == "":
		return fmt.Errorf("DB_NAME is required")
	case strings.TrimSpace(c.HTTPAddr) == "":
		return fmt.Errorf("HTTP_ADDR is required")
	case strings.TrimSpace(c.APIToken) == "":
		return fmt.Errorf("API_TOKEN is required (used for the X-API-Key header)")
	}
	return nil
}
