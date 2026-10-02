package api

import (
	"time"

	"github.com/gofiber/fiber/v3"
	"tce.ap.gov.br/sistema-corporativo/servidor-service/internal/servidor"
	sharedstatus "tce.ap.gov.br/sistema-corporativo/shared-common/status"
)

const serviceName = "servidor-service"

type healthResponse struct {
	Status string `json:"status"`
}

var healthy = healthResponse{Status: "UP"}

type Options struct {
	Handler *servidor.Handler
}

// New configura a aplicação HTTP do serviço de servidores.
func New(opts Options) *fiber.App {
	app := fiber.New(fiber.Config{
		AppName:      serviceName,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
		BodyLimit:    1 * 1024 * 1024,
	})

	app.Get("/healthz", health)
	app.Get("/actuator/health", health)
	app.Get("/api/servidores/status", status)
	if opts.Handler != nil {
		opts.Handler.Register(app)
	}

	return app
}

func health(c fiber.Ctx) error {
	return c.JSON(healthy)
}

func status(c fiber.Ctx) error {
	return c.JSON(sharedstatus.New(serviceName))
}
