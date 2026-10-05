package auth

import (
	"context"
	"errors"
	"log"
	"net/http"

	"github.com/gofiber/fiber/v3"
)

type Handler struct {
	keycloak *Keycloak
}

func NewHandler(keycloak *Keycloak) *Handler {
	return &Handler{keycloak: keycloak}
}

// Authorize devolve o endereço do Keycloak com o redirect_uri cadastrado no client.
func (h *Handler) Authorize(c fiber.Ctx) error {
	state := c.Query("state")
	challenge := c.Query("code_challenge")
	if state == "" || challenge == "" {
		return problem(c, http.StatusBadRequest, "BAD_REQUEST", "Estado da autenticação ausente.")
	}
	return c.Redirect().Status(fiber.StatusFound).To(h.keycloak.AuthorizeURL(state, challenge))
}

type tokenRequest struct {
	Code         string `json:"code"`
	CodeVerifier string `json:"codeVerifier"`
}

type loginResponse struct {
	AccessToken string `json:"accessToken"`
	ExpiresIn   int    `json:"expiresIn"`
	TokenType   string `json:"tokenType"`
	CPF         string `json:"cpf"`
	Nome        string `json:"nome,omitempty"`
}

// Token troca o código de autorização pelo access token no Keycloak.
func (h *Handler) Token(c fiber.Ctx) error {
	var body tokenRequest
	if err := c.Bind().JSON(&body); err != nil || body.Code == "" || body.CodeVerifier == "" {
		return problem(c, http.StatusBadRequest, "BAD_REQUEST", "Código de autorização ausente.")
	}

	ctx, cancel := context.WithTimeout(c.Context(), requestTimeout)
	defer cancel()

	session, err := h.keycloak.Exchange(ctx, body.Code, body.CodeVerifier)
	if err != nil {
		var authErr *Error
		if errors.As(err, &authErr) {
			return problem(c, authErr.Status, authErr.Code, authErr.Message)
		}
		log.Printf("falha inesperada na troca do código: %v", err)
		return problem(c, http.StatusBadGateway, "BAD_GATEWAY", "Não foi possível autenticar no Keycloak.")
	}

	return c.JSON(loginResponse{
		AccessToken: session.AccessToken,
		ExpiresIn:   session.ExpiresIn,
		TokenType:   session.TokenType,
		CPF:         session.CPF,
		Nome:        session.Nome,
	})
}

func problem(c fiber.Ctx, status int, code, message string) error {
	return c.Status(status).JSON(fiber.Map{
		"status":  status,
		"error":   code,
		"message": message,
	})
}
