package auth

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"
)

const redirectURI = "http://localhost:3000/intranet/login/callback"

func TestAuthorizeUsaRedirectCadastrado(t *testing.T) {
	t.Parallel()

	app := novoApp(t, NewKeycloak("https://keycloak.test/realms/intranet", "intranet", "segredo", redirectURI))
	response := executar(t, app, httptest.NewRequest(http.MethodGet, "/api/auth/authorize?state=estado&code_challenge=desafio", nil))
	if response.StatusCode != http.StatusFound {
		t.Fatalf("status %d", response.StatusCode)
	}

	location, err := url.Parse(response.Header.Get("Location"))
	if err != nil {
		t.Fatal(err)
	}
	query := location.Query()
	if location.Host != "keycloak.test" || query.Get("client_id") != "intranet" || query.Get("redirect_uri") != redirectURI || query.Get("code_challenge") != "desafio" {
		t.Fatalf("location inesperada: %s", location)
	}
}

func TestTokenTrocaCodigoPorSessao(t *testing.T) {
	t.Parallel()

	token := fakeJWT(t, map[string]any{"preferred_username": "111.222.333-96", "name": "Ana Lima"})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Fatal(err)
		}
		if r.Form.Get("grant_type") != "authorization_code" || r.Form.Get("code") != "codigo" || r.Form.Get("code_verifier") != "verificador" || r.Form.Get("redirect_uri") != redirectURI {
			t.Fatalf("troca inesperada: %v", r.Form)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"access_token": token,
			"expires_in":   90,
			"token_type":   "Bearer",
		})
	}))
	t.Cleanup(server.Close)

	app := novoApp(t, NewKeycloak(server.URL, "intranet", "segredo", redirectURI))
	request := httptest.NewRequest(http.MethodPost, "/api/auth/token", strings.NewReader(`{"code":"codigo","codeVerifier":"verificador"}`))
	request.Header.Set("Content-Type", "application/json")
	response := executar(t, app, request)
	if response.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(response.Body)
		t.Fatalf("status %d: %s", response.StatusCode, body)
	}

	var payload loginResponse
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		t.Fatal(err)
	}
	if payload.CPF != "11122233396" || payload.Nome != "Ana Lima" || payload.AccessToken != token {
		t.Fatalf("payload inesperado: %+v", payload)
	}
}

func novoApp(t *testing.T, keycloak *Keycloak) *fiber.App {
	t.Helper()
	handler := NewHandler(keycloak)
	app := fiber.New()
	app.Get("/api/auth/authorize", handler.Authorize)
	app.Post("/api/auth/token", handler.Token)
	return app
}

func executar(t *testing.T, app *fiber.App, request *http.Request) *http.Response {
	t.Helper()
	response, err := app.Test(request)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { response.Body.Close() })
	return response
}
