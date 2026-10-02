package api

import (
	"time"

	"github.com/gofiber/fiber/v3"
	sharedstatus "tce.ap.gov.br/sistema-corporativo/shared-common/status"
)

const serviceName = "ferias-service"

type healthResponse struct {
	Status string `json:"status"`
}

var healthy = healthResponse{Status: "UP"}

// New configura a aplicação HTTP do serviço de férias.
func New() *fiber.App {
	app := fiber.New(fiber.Config{
		AppName:      serviceName,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
		BodyLimit:    1 * 1024 * 1024,
	})

	app.Get("/healthz", health)
	app.Get("/actuator/health", health)
	app.Get("/api/ferias/status", status)

	return app
}

func health(c fiber.Ctx) error {
	return c.JSON(healthy)
}

func status(c fiber.Ctx) error {
	return c.JSON(sharedstatus.New(serviceName))
}
