package api

import (
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/cors"
	"github.com/gofiber/fiber/v3/middleware/recover"
	"tce.ap.gov.br/sistema-corporativo/contracheque-service/internal/contracheque"
	sharedstatus "tce.ap.gov.br/sistema-corporativo/shared-common/status"
)

const ServiceName = "contracheque-service"

type healthResponse struct {
	Status string `json:"status"`
}

type Options struct {
	Handler        *contracheque.Handler
	AllowedOrigins []string
}

// New configura a aplicação HTTP do serviço de contracheque.
func New(opts Options) *fiber.App {
	app := fiber.New(fiber.Config{
		AppName:      ServiceName,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
		BodyLimit:    1 * 1024 * 1024,
	})

	app.Use(recover.New())
	if len(opts.AllowedOrigins) > 0 {
		app.Use(cors.New(cors.Config{
			AllowOrigins: opts.AllowedOrigins,
			AllowMethods: []string{"GET", "POST", "OPTIONS"},
			AllowHeaders: []string{"Authorization", "Content-Type", "Accept"},
		}))
	}

	app.Get("/healthz", health)
	app.Get("/actuator/health", health)
	app.Get("/api/contracheque/status", status)

	opts.Handler.RegisterPublic(app)
	opts.Handler.RegisterProtected(app.Group("/api/contra-cheque"))

	return app
}

func health(c fiber.Ctx) error { return c.JSON(healthResponse{Status: "UP"}) }

func status(c fiber.Ctx) error { return c.JSON(sharedstatus.New(ServiceName)) }
