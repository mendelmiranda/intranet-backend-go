// Package config carrega a configuração do serviço a partir de variáveis de ambiente.
// Nenhuma credencial é mantida no código: tudo vem do ambiente.
package config

import (
	"encoding/base64"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Port string

	// MSSQLDSN aponta para o banco da folha (GP0001_TCEAP), ex.:
	// sqlserver://usuario:senha@host:1597?database=GP0001_TCEAP
	MSSQLDSN string
	// MySQLDSN aponta para o banco internet_novo (tabela codigo_verificacao), ex.:
	// usuario:senha@tcp(host:3306)/internet_novo?parseTime=true&loc=UTC
	MySQLDSN string

	// JWTSigningKey é a chave HS256 do legado. O jjwt 0.9 trata a string de
	// assinatura como Base64; por isso a chave é decodificada aqui.
	JWTSigningKey []byte

	// VerificaURL é o endereço (sem querystring) da página de verificação usada no QR Code.
	VerificaURL string

	AllowedOrigins []string
	CacheTTL       time.Duration
	Location       *time.Location
}

func Load() (Config, error) {
	cfg := Config{
		Port:        env("PORT", "8084"),
		MSSQLDSN:    os.Getenv("MSSQL_DSN"),
		MySQLDSN:    os.Getenv("MYSQL_DSN"),
		VerificaURL: env("CONTRACHEQUE_VERIFICA_URL", "http://10.10.3.5:3000/resposta-contra-cheque"),
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

	minutes, err := strconv.Atoi(env("CACHE_TTL_MINUTES", "5"))
	if err != nil || minutes < 0 {
		return cfg, fmt.Errorf("CACHE_TTL_MINUTES inválido")
	}
	cfg.CacheTTL = time.Duration(minutes) * time.Minute

	cfg.Location, err = time.LoadLocation(env("TIMEZONE", "America/Belem"))
	if err != nil {
		return cfg, fmt.Errorf("TIMEZONE inválido: %w", err)
	}
	return cfg, nil
}

func env(name, fallback string) string {
	if v := os.Getenv(name); v != "" {
		return v
	}
	return fallback
}
