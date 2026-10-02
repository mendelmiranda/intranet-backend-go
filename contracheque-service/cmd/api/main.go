package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"
	_ "time/tzdata"

	"github.com/gofiber/fiber/v3"
	"tce.ap.gov.br/sistema-corporativo/contracheque-service/internal/api"
	"tce.ap.gov.br/sistema-corporativo/contracheque-service/internal/config"
	"tce.ap.gov.br/sistema-corporativo/contracheque-service/internal/contracheque"
	"tce.ap.gov.br/sistema-corporativo/contracheque-service/internal/platform"
)

func main() {
	log.SetFlags(log.Ldate | log.Ltime | log.Lmicroseconds | log.LUTC)

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("configuração inválida: %v", err)
	}

	folha, err := platform.Open("sqlserver", cfg.MSSQLDSN)
	if err != nil {
		log.Fatalf("banco da folha (MSSQL_DSN): %v", err)
	}
	defer folha.Close()
	mysql, err := platform.Open("mysql", cfg.MySQLDSN)
	if err != nil {
		log.Fatalf("banco internet_novo (MYSQL_DSN): %v", err)
	}
	defer mysql.Close()

	repo := contracheque.NewSQLRepository(folha, mysql)
	svc := contracheque.NewService(repo, cfg.VerificaURL, cfg.Location, cfg.CacheTTL)
	application := api.New(api.Options{
		Handler:        contracheque.NewHandler(svc, cfg.Location),
		AllowedOrigins: cfg.AllowedOrigins,
	})

	listenErrors := make(chan error, 1)
	go func() {
		log.Printf("%s iniciado na porta %s", api.ServiceName, cfg.Port)
		listenErrors <- application.Listen(":"+cfg.Port, fiber.ListenConfig{DisableStartupMessage: true})
	}()

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(signals)

	select {
	case received := <-signals:
		log.Printf("sinal %s recebido; encerrando %s", received, api.ServiceName)
	case err := <-listenErrors:
		if err != nil {
			log.Fatalf("falha ao executar %s: %v", api.ServiceName, err)
		}
		return
	}

	shutdownContext, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := application.ShutdownWithContext(shutdownContext); err != nil {
		log.Fatalf("falha no encerramento de %s: %v", api.ServiceName, err)
	}
	log.Printf("%s encerrado", api.ServiceName)
}
