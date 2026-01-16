package config

import (
	"fmt"
	"os"
	"time"

	"github.com/knadh/koanf/parsers/yaml"
	"github.com/knadh/koanf/providers/env"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/v2"
)

type Config struct {
	Server   ServerConfig
	Database DatabaseConfig
	Log      LogConfig
}

type ServerConfig struct {
	Host         string
	Port         string
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
}

type DatabaseConfig struct {
	URL             string
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
}

type LogConfig struct {
	Level  string
	Format string
}

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
			Host:         k.String("SERVER_HOST"),
			Port:         k.String("SERVER_PORT"),
			ReadTimeout:  k.Duration("SERVER_READ_TIMEOUT"),
			WriteTimeout: k.Duration("SERVER_WRITE_TIMEOUT"),
		},
		Database: DatabaseConfig{
			URL:             k.String("DATABASE_URL"),
			MaxOpenConns:    k.Int("DATABASE_MAX_OPEN_CONNS"),
			MaxIdleConns:    k.Int("DATABASE_MAX_IDLE_CONNS"),
			ConnMaxLifetime: k.Duration("DATABASE_CONN_MAX_LIFETIME"),
		},
		Log: LogConfig{
			Level:  k.String("LOG_LEVEL"),
			Format: k.String("LOG_FORMAT"),
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

	if cfg.Log.Level == "" {
		cfg.Log.Level = "info"
	}
	if cfg.Log.Format == "" {
		cfg.Log.Format = "json"
	}
}
