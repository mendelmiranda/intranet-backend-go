module tce.ap.gov.br/sistema-corporativo/servidor-service

go 1.25.0

require (
	github.com/gofiber/fiber/v3 v3.5.0
	tce.ap.gov.br/sistema-corporativo/shared-common v0.0.0
)

replace tce.ap.gov.br/sistema-corporativo/shared-common => ../shared-common
