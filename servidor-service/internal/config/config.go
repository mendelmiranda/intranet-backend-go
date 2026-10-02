package config

import (
	"fmt"
	"os"
)

type Config struct {
	Port     string
	MSSQLDSN string
}

func Load() (Config, error) {
	cfg := Config{
		Port:     env("PORT", "8083"),
		MSSQLDSN: os.Getenv("MSSQL_DSN"),
	}
	if cfg.MSSQLDSN == "" {
		return cfg, fmt.Errorf("MSSQL_DSN não informada")
	}
	return cfg, nil
}

func env(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}
