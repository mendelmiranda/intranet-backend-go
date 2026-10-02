package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gofiber/fiber/v3"
	"tce.ap.gov.br/sistema-corporativo/servidor-service/internal/api"
	"tce.ap.gov.br/sistema-corporativo/servidor-service/internal/config"
	"tce.ap.gov.br/sistema-corporativo/servidor-service/internal/platform"
	"tce.ap.gov.br/sistema-corporativo/servidor-service/internal/servidor"
)

const serviceName = "servidor-service"

func main() {
	log.SetFlags(log.Ldate | log.Ltime | log.Lmicroseconds | log.LUTC)

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("configuração inválida: %v", err)
	}

	folha, err := platform.Open(cfg.MSSQLDSN)
	if err != nil {
		log.Fatalf("banco da folha (MSSQL_DSN): %v", err)
	}
	defer folha.Close()

	application := api.New(api.Options{
		Handler: servidor.NewHandler(servidor.NewSQLRepository(folha)),
	})
	listenErrors := make(chan error, 1)

	go func() {
		log.Printf("%s iniciado na porta %s", serviceName, cfg.Port)
		listenErrors <- application.Listen(":"+cfg.Port, fiber.ListenConfig{
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
