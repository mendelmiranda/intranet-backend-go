// Package config carrega a configuração do serviço a partir de variáveis de ambiente.
// Nenhuma credencial é mantida no código.
package config

import (
	"fmt"
	"os"
	"strings"
)

type Config struct {
	Port           string
	AppURL         string
	GraphQLURL     string
	AllowedOrigins []string
	Keycloak       Keycloak
}

type Keycloak struct {
	Issuer       string
	ClientID     string
	ClientSecret string
	RedirectURI  string
}

func Load() (Config, error) {
	cfg := Config{
		Port:       env("PORT", "8081"),
		AppURL:     strings.TrimRight(env("APP_URL", "http://localhost:3000/intranet"), "/"),
		GraphQLURL: env("GRAPHQL_URL", "https://api-staging.apps.ocp4.tce.local/graphql"),
		Keycloak: Keycloak{
			Issuer:       strings.TrimRight(os.Getenv("KEYCLOAK_ISSUER"), "/"),
			ClientID:     os.Getenv("KEYCLOAK_CLIENT_ID"),
			ClientSecret: os.Getenv("KEYCLOAK_CLIENT_SECRET"),
			RedirectURI:  env("KEYCLOAK_REDIRECT_URI", "http://localhost:3000/intranet/login/callback"),
		},
	}

	for _, origin := range strings.Split(os.Getenv("CORS_ALLOWED_ORIGINS"), ",") {
		if origin = strings.TrimSpace(origin); origin != "" {
			cfg.AllowedOrigins = append(cfg.AllowedOrigins, origin)
		}
	}

	if cfg.Keycloak.Issuer == "" || cfg.Keycloak.ClientID == "" || cfg.Keycloak.ClientSecret == "" {
		return cfg, fmt.Errorf("KEYCLOAK_ISSUER, KEYCLOAK_CLIENT_ID e KEYCLOAK_CLIENT_SECRET são obrigatórios")
	}
	return cfg, nil
}

func env(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}
