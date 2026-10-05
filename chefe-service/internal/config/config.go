// Package config carrega a configuração do serviço a partir de variáveis de ambiente.
package config

import (
	"encoding/base64"
	"fmt"
	"os"
	"strings"
)

type Config struct {
	Port string

	// MySQLDSN aponta para o banco internet_novo (chefe_funcionarios, excecao_servidores, user, tab_log).
	MySQLDSN string
	// MSSQLDSN aponta para o banco da folha (view devops_servidor), usado para checar servidores efetivos.
	MSSQLDSN string

	// JWTSigningKey é a chave HS256 do legado (o jjwt 0.9 trata a string como Base64).
	JWTSigningKey  []byte
	AllowedOrigins []string
}

func Load() (Config, error) {
	cfg := Config{
		Port:     env("PORT", "8085"),
		MySQLDSN: firstEnv("CHEFE_MYSQL_DSN", "MYSQL_DSN"),
		MSSQLDSN: os.Getenv("MSSQL_DSN"),
	}

	if cfg.MySQLDSN == "" || cfg.MSSQLDSN == "" {
		return cfg, fmt.Errorf("CHEFE_MYSQL_DSN (ou MYSQL_DSN) e MSSQL_DSN são obrigatórias")
	}

	key := os.Getenv("JWT_SIGNING_KEY")
	if key == "" {
		return cfg, fmt.Errorf("JWT_SIGNING_KEY não informada")
	}
	if decoded, err := base64.StdEncoding.DecodeString(key); err == nil {
		cfg.JWTSigningKey = decoded
	} else if decoded, err := base64.RawStdEncoding.DecodeString(key); err == nil {
		cfg.JWTSigningKey = decoded
	} else if decoded, err := base64.URLEncoding.DecodeString(key); err == nil {
		cfg.JWTSigningKey = decoded
	} else {
		cfg.JWTSigningKey = []byte(key)
	}

	for _, origin := range strings.Split(os.Getenv("CORS_ALLOWED_ORIGINS"), ",") {
		if origin = strings.TrimSpace(origin); origin != "" {
			cfg.AllowedOrigins = append(cfg.AllowedOrigins, origin)
		}
	}
	return cfg, nil
}

func env(name, fallback string) string {
	if v := os.Getenv(name); v != "" {
		return v
	}
	return fallback
}

func firstEnv(names ...string) string {
	for _, n := range names {
		if v := os.Getenv(n); v != "" {
			return v
		}
	}
	return ""
}
