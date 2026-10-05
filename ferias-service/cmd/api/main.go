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
	"tce.ap.gov.br/sistema-corporativo/ferias-service/internal/config"
	"tce.ap.gov.br/sistema-corporativo/ferias-service/internal/ferias"
	"tce.ap.gov.br/sistema-corporativo/ferias-service/internal/platform"
)

const serviceName = "ferias-service"

func main() {
	log.SetFlags(log.Ldate | log.Ltime | log.Lmicroseconds | log.LUTC)

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("configuração inválida: %v", err)
	}

	intranet, err := platform.Open("mysql", cfg.MySQLDSN)
	if err != nil {
		log.Fatalf("banco internet_novo (FERIAS_MYSQL_DSN): %v", err)
	}
	defer intranet.Close()

	folha, err := platform.Open("sqlserver", cfg.MSSQLDSN)
	if err != nil {
		log.Fatalf("banco da folha (MSSQL_DSN): %v", err)
	}
	defer folha.Close()

	service := ferias.NewService(ferias.NewSQLRepository(intranet), ferias.NewSQLFolha(folha))
	if cfg.DocumentosDir != "" {
		service.ComArmazenamento(ferias.ArmazenamentoFS{Dir: cfg.DocumentosDir})
	} else {
		log.Printf("FERIAS_DOCUMENTOS_DIR não informada: comprovantes não serão gravados em disco")
	}
	application := api.New(api.Options{
		Handler:        ferias.NewHandler(service, cfg.JWTSigningKey),
		AllowedOrigins: cfg.AllowedOrigins,
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
