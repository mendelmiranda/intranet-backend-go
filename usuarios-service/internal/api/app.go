package api

import (
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/cors"
	sharedstatus "tce.ap.gov.br/sistema-corporativo/shared-common/status"
	"tce.ap.gov.br/sistema-corporativo/usuarios-service/internal/auth"
	"tce.ap.gov.br/sistema-corporativo/usuarios-service/internal/pessoa"
)

const serviceName = "usuarios-service"

type healthResponse struct {
	Status string `json:"status"`
}

type Options struct {
	Auth           *auth.Handler
	Pessoa         *pessoa.Handler
	AllowedOrigins []string
}

var healthy = healthResponse{Status: "UP"}

// New configura a aplicação HTTP do serviço de usuários.
func New(opts Options) *fiber.App {
	app := fiber.New(fiber.Config{
		AppName:      serviceName,
		ReadTimeout:  20 * time.Second,
		WriteTimeout: 20 * time.Second,
		IdleTimeout:  60 * time.Second,
		BodyLimit:    1 * 1024 * 1024,
	})

	if len(opts.AllowedOrigins) > 0 {
		app.Use(cors.New(cors.Config{
			AllowOrigins: opts.AllowedOrigins,
			AllowMethods: []string{"GET", "POST", "OPTIONS"},
			AllowHeaders: []string{"Authorization", "Content-Type", "Accept"},
		}))
	}

	app.Get("/healthz", health)
	app.Get("/actuator/health", health)
	app.Get("/api/usuarios/status", status)
	if opts.Auth != nil {
		app.Get("/api/auth/authorize", opts.Auth.Authorize)
		app.Post("/api/auth/token", opts.Auth.Token)
	}
	if opts.Pessoa != nil {
		app.Get("/api/usuarios/pessoa", opts.Pessoa.Consultar)
		app.Get("/api/usuarios/pessoas", opts.Pessoa.Pesquisar)
	}

	return app
}

func health(c fiber.Ctx) error {
	return c.JSON(healthy)
}

func status(c fiber.Ctx) error {
	return c.JSON(sharedstatus.New(serviceName))
}
