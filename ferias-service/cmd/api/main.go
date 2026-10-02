package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gofiber/fiber/v3"
	"tce.ap.gov.br/sistema-corporativo/ferias-service/internal/api"
)

const (
	serviceName = "ferias-service"
	defaultPort = "8082"
)

func main() {
	log.SetFlags(log.Ldate | log.Ltime | log.Lmicroseconds | log.LUTC)

	application := api.New()
	port := environmentOrDefault("PORT", defaultPort)
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

func environmentOrDefault(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}
