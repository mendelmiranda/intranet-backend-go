package pessoa

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strings"

	"github.com/gofiber/fiber/v3"
)

type Handler struct {
	client *Client
}

func NewHandler(client *Client) *Handler {
	return &Handler{client: client}
}

// Consultar lê o CPF da sessão e consulta rhPessoa com o token de serviço.
func (h *Handler) Consultar(c fiber.Ctx) error {
	token := bearerToken(c.Get("Authorization"))
	if token == "" {
		return problem(c, http.StatusUnauthorized, "UNAUTHORIZED", "Token ausente.")
	}

	cpf := onlyDigits(c.Query("cpf"))
	if len(cpf) != 11 {
		cpf = cpfFromToken(token)
	}
	if len(onlyDigits(cpf)) != 11 {
		return problem(c, http.StatusBadRequest, "BAD_REQUEST", "CPF ausente no token.")
	}

	ctx, cancel := context.WithTimeout(c.Context(), requestTimeout)
	defer cancel()

	cadastro, err := h.client.Buscar(ctx, cpf)
	if err != nil {
		var apiErr *Error
		if errors.As(err, &apiErr) {
			return problem(c, apiErr.Status, apiErr.Code, apiErr.Message)
		}
		log.Printf("falha inesperada na consulta rhPessoa: %v", err)
		return problem(c, http.StatusBadGateway, "BAD_GATEWAY", "Não foi possível consultar o cadastro.")
	}

	return c.JSON(cadastro)
}

// Pesquisar lista servidores pelo nome para o autocomplete.
func (h *Handler) Pesquisar(c fiber.Ctx) error {
	if bearerToken(c.Get("Authorization")) == "" {
		return problem(c, http.StatusUnauthorized, "UNAUTHORIZED", "Token ausente.")
	}

	ctx, cancel := context.WithTimeout(c.Context(), requestTimeout)
	defer cancel()

	resultado, err := h.client.Pesquisar(ctx, c.Query("q"))
	if err != nil {
		var apiErr *Error
		if errors.As(err, &apiErr) {
			return problem(c, apiErr.Status, apiErr.Code, apiErr.Message)
		}
		log.Printf("falha inesperada na pesquisa de servidores: %v", err)
		return problem(c, http.StatusBadGateway, "BAD_GATEWAY", "Não foi possível pesquisar os servidores.")
	}
	return c.JSON(resultado)
}

// Cargos lista os cargos distintos dos vínculos ativos.
func (h *Handler) Cargos(c fiber.Ctx) error {
	if bearerToken(c.Get("Authorization")) == "" {
		return problem(c, http.StatusUnauthorized, "UNAUTHORIZED", "Token ausente.")
	}

	ctx, cancel := context.WithTimeout(c.Context(), requestTimeout)
	defer cancel()

	resultado, err := h.client.Cargos(ctx, c.Query("q"))
	if err != nil {
		var apiErr *Error
		if errors.As(err, &apiErr) {
			return problem(c, apiErr.Status, apiErr.Code, apiErr.Message)
		}
		log.Printf("falha inesperada na consulta de cargos: %v", err)
		return problem(c, http.StatusBadGateway, "BAD_GATEWAY", "Não foi possível consultar os cargos.")
	}
	return c.JSON(resultado)
}

// Servidores lista quem ocupa o cargo informado pelo código.
func (h *Handler) Servidores(c fiber.Ctx) error {
	if bearerToken(c.Get("Authorization")) == "" {
		return problem(c, http.StatusUnauthorized, "UNAUTHORIZED", "Token ausente.")
	}

	ctx, cancel := context.WithTimeout(c.Context(), requestTimeout)
	defer cancel()

	resultado, err := h.client.ServidoresDoCargo(ctx, c.Query("codigo"))
	if err != nil {
		var apiErr *Error
		if errors.As(err, &apiErr) {
			return problem(c, apiErr.Status, apiErr.Code, apiErr.Message)
		}
		log.Printf("falha inesperada na consulta de servidores do cargo: %v", err)
		return problem(c, http.StatusBadGateway, "BAD_GATEWAY", "Não foi possível consultar os servidores do cargo.")
	}
	return c.JSON(resultado)
}

func bearerToken(header string) string {
	scheme, token, ok := strings.Cut(header, " ")
	if !ok || !strings.EqualFold(scheme, "Bearer") {
		return ""
	}
	return strings.TrimSpace(token)
}

func cpfFromToken(token string) string {
	parts := strings.Split(token, ".")
	if len(parts) < 2 {
		return ""
	}
	raw, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		raw, err = base64.URLEncoding.DecodeString(parts[1])
		if err != nil {
			return ""
		}
	}
	claims := map[string]any{}
	if err := json.Unmarshal(raw, &claims); err != nil {
		return ""
	}
	for _, key := range []string{"cpf", "CPF", "preferred_username", "username"} {
		value, _ := claims[key].(string)
		digits := onlyDigits(value)
		if len(digits) == 11 {
			return digits
		}
	}
	return ""
}

func problem(c fiber.Ctx, status int, code, message string) error {
	return c.Status(status).JSON(fiber.Map{
		"status":  status,
		"error":   code,
		"message": message,
	})
}
