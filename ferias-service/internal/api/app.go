package api

import (
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/cors"
	"github.com/gofiber/fiber/v3/middleware/recover"
	"tce.ap.gov.br/sistema-corporativo/ferias-service/internal/ferias"
	sharedstatus "tce.ap.gov.br/sistema-corporativo/shared-common/status"
)

const serviceName = "ferias-service"

type healthResponse struct {
	Status string `json:"status"`
}

var healthy = healthResponse{Status: "UP"}

type Options struct {
	Handler        *ferias.Handler
	AllowedOrigins []string
}

// New configura a aplicação HTTP do serviço de férias.
func New(opts Options) *fiber.App {
	app := fiber.New(fiber.Config{
		AppName:      serviceName,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
		BodyLimit:    1 * 1024 * 1024,
		ErrorHandler: ferias.ErrorHandler,
	})

	app.Use(recover.New())
	if len(opts.AllowedOrigins) > 0 {
		app.Use(cors.New(cors.Config{
			AllowOrigins: opts.AllowedOrigins,
			AllowMethods: []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
			AllowHeaders: []string{"Authorization", "Content-Type", "Accept", "DADOS_CLIENT"},
		}))
	}

	app.Get("/healthz", health)
	app.Get("/actuator/health", health)
	app.Get("/api/ferias/status", status)
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
