// Package config provides application configuration management.
package config

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/knadh/koanf/parsers/yaml"
	"github.com/knadh/koanf/providers/env"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/v2"
)

// Config represents the application configuration.
type Config struct {
	Server   ServerConfig
	Database DatabaseConfig
	Log      LogConfig
	JWT      JWTConfig
	CORS     CORSConfig
	Cookie   CookieConfig
}

// CORSConfig contains CORS settings.
type CORSConfig struct {
	AllowedOrigins []string
}

// CookieConfig contains cookie settings for authentication.
type CookieConfig struct {
	Secure bool   // Set Secure flag on cookies (true in production)
	Domain string // Cookie domain (empty = current domain)
}

// ServerConfig contains HTTP server settings.
type ServerConfig struct {
	Host              string
	Port              string
	ReadTimeout       time.Duration
	ReadHeaderTimeout time.Duration
	WriteTimeout      time.Duration
	IdleTimeout       time.Duration
}

// DatabaseConfig contains database connection settings.
type DatabaseConfig struct {
	URL             string
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
	ConnectTimeout  time.Duration
	ConnectAttempts int
}

// LogConfig contains logging settings.
type LogConfig struct {
	Level  string
	Format string
}

// JWTConfig contains JWT authentication settings.
type JWTConfig struct {
	SecretKey            string
	AccessTokenDuration  time.Duration
	RefreshTokenDuration time.Duration
}

// Load loads configuration from config.yaml and environment variables.
func Load() (*Config, error) {
	k := koanf.New(".")

	if err := k.Load(file.Provider("config.yaml"), yaml.Parser()); err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("load config file: %w", err)
	}

	if err := k.Load(env.Provider("", ".", func(s string) string {
		return s
	}), nil); err != nil {
		return nil, fmt.Errorf("load env variables: %w", err)
	}

	cfg := &Config{
		Server: ServerConfig{
			Host:              k.String("SERVER_HOST"),
			Port:              k.String("SERVER_PORT"),
			ReadTimeout:       k.Duration("SERVER_READ_TIMEOUT"),
			ReadHeaderTimeout: k.Duration("SERVER_READ_HEADER_TIMEOUT"),
			WriteTimeout:      k.Duration("SERVER_WRITE_TIMEOUT"),
			IdleTimeout:       k.Duration("SERVER_IDLE_TIMEOUT"),
		},
		Database: DatabaseConfig{
			URL:             k.String("DATABASE_URL"),
			MaxOpenConns:    k.Int("DATABASE_MAX_OPEN_CONNS"),
			MaxIdleConns:    k.Int("DATABASE_MAX_IDLE_CONNS"),
			ConnMaxLifetime: k.Duration("DATABASE_CONN_MAX_LIFETIME"),
			ConnectTimeout:  k.Duration("DATABASE_CONNECT_TIMEOUT"),
			ConnectAttempts: k.Int("DATABASE_CONNECT_ATTEMPTS"),
		},
		Log: LogConfig{
			Level:  k.String("LOG_LEVEL"),
			Format: k.String("LOG_FORMAT"),
		},
		JWT: JWTConfig{
			SecretKey:            k.String("JWT_SECRET_KEY"),
			AccessTokenDuration:  k.Duration("JWT_ACCESS_TOKEN_DURATION"),
			RefreshTokenDuration: k.Duration("JWT_REFRESH_TOKEN_DURATION"),
		},
		CORS: CORSConfig{
			AllowedOrigins: parseOrigins(k.String("CORS_ALLOWED_ORIGINS")),
		},
		Cookie: CookieConfig{
			Secure: k.Bool("COOKIE_SECURE"),
			Domain: k.String("COOKIE_DOMAIN"),
		},
	}

	setDefaults(cfg)

	return cfg, nil
}

func setDefaults(cfg *Config) {
	if cfg.Server.Host == "" {
		cfg.Server.Host = "0.0.0.0"
	}
	if cfg.Server.Port == "" {
		cfg.Server.Port = "8080"
	}
	if cfg.Server.ReadTimeout == 0 {
		cfg.Server.ReadTimeout = 15 * time.Second
	}
	if cfg.Server.WriteTimeout == 0 {
		cfg.Server.WriteTimeout = 15 * time.Second
	}
	if cfg.Server.ReadHeaderTimeout == 0 {
		cfg.Server.ReadHeaderTimeout = 5 * time.Second
	}
	if cfg.Server.IdleTimeout == 0 {
		cfg.Server.IdleTimeout = 60 * time.Second
	}

	if cfg.Database.URL == "" {
		cfg.Database.URL = "postgres://statuspage:statuspage@localhost:5432/statuspage?sslmode=disable"
	}
	if cfg.Database.MaxOpenConns == 0 {
		cfg.Database.MaxOpenConns = 25
	}
	if cfg.Database.MaxIdleConns == 0 {
		cfg.Database.MaxIdleConns = 5
	}
	if cfg.Database.ConnMaxLifetime == 0 {
		cfg.Database.ConnMaxLifetime = 5 * time.Minute
	}
	if cfg.Database.ConnectTimeout == 0 {
		cfg.Database.ConnectTimeout = 30 * time.Second
	}
	if cfg.Database.ConnectAttempts == 0 {
		cfg.Database.ConnectAttempts = 5
	}

	if cfg.Log.Level == "" {
		cfg.Log.Level = "info"
	}
	if cfg.Log.Format == "" {
		cfg.Log.Format = "json"
	}

	if cfg.JWT.SecretKey == "" {
		cfg.JWT.SecretKey = "change-me-in-production"
	}
	if cfg.JWT.AccessTokenDuration == 0 {
		cfg.JWT.AccessTokenDuration = 15 * time.Minute
	}
	if cfg.JWT.RefreshTokenDuration == 0 {
		cfg.JWT.RefreshTokenDuration = 168 * time.Hour
	}

	if len(cfg.CORS.AllowedOrigins) == 0 {
		cfg.CORS.AllowedOrigins = []string{"http://localhost:3000"}
	}
}

func parseOrigins(origins string) []string {
	if origins == "" {
		return nil
	}
	parts := strings.Split(origins, ",")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		trimmed := strings.TrimSpace(p)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}
