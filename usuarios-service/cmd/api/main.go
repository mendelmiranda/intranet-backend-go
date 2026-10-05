package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gofiber/fiber/v3"
	"tce.ap.gov.br/sistema-corporativo/usuarios-service/internal/api"
	"tce.ap.gov.br/sistema-corporativo/usuarios-service/internal/auth"
	"tce.ap.gov.br/sistema-corporativo/usuarios-service/internal/config"
	"tce.ap.gov.br/sistema-corporativo/usuarios-service/internal/pessoa"
)

const serviceName = "usuarios-service"

func main() {
	log.SetFlags(log.Ldate | log.Ltime | log.Lmicroseconds | log.LUTC)

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("configuração inválida: %v", err)
	}

	keycloak := auth.NewKeycloak(cfg.Keycloak.Issuer, cfg.Keycloak.ClientID, cfg.Keycloak.ClientSecret, cfg.Keycloak.RedirectURI)
	application := api.New(api.Options{
		Auth:           auth.NewHandler(keycloak),
		Pessoa:         pessoa.NewHandler(pessoa.NewClient(cfg.GraphQLURL, keycloak)),
		AllowedOrigins: cfg.AllowedOrigins,
	})

	port := cfg.Port
	listenErrors := make(chan error, 1)

	go func() {
		log.Printf("%s iniciado na porta %s", serviceName, port)
		listenErrors <- application.Listen(":"+port, fiber.ListenConfig{
			DisableStartupMessage: true,
		})
	}()

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(signals)

	select {
	case received := <-signals:
		log.Printf("sinal %s recebido; encerrando %s", received, serviceName)
	case err := <-listenErrors:
		if err != nil {
			log.Fatalf("falha ao executar %s: %v", serviceName, err)
		}
		return
	}

	shutdownContext, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := application.ShutdownWithContext(shutdownContext); err != nil {
		log.Fatalf("falha no encerramento de %s: %v", serviceName, err)
	}

	log.Printf("%s encerrado", serviceName)
}
