package pessoa

import (
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"
)

func TestConsultarUsaCpfDoToken(t *testing.T) {
	t.Parallel()

	graphql := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `{"data":{"rhPessoa":{"nome":"Ana Lima"}}}`)
	}))
	t.Cleanup(graphql.Close)

	app := fiber.New()
	app.Get("/api/usuarios/pessoa", NewHandler(NewClient(graphql.URL, fixedToken("token-de-servico"))).Consultar)

	token := "e30." + base64.RawURLEncoding.EncodeToString([]byte(`{"cpf":"11122233396"}`)) + ".assinatura"
	request := httptest.NewRequest(http.MethodGet, "/api/usuarios/pessoa", nil)
	request.Header.Set("Authorization", "Bearer "+token)

	response, err := app.Test(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		t.Fatalf("status %d", response.StatusCode)
	}
	var payload struct {
		Nome string `json:"nome"`
	}
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		t.Fatal(err)
	}
	if payload.Nome != "Ana Lima" {
		t.Fatalf("nome %q", payload.Nome)
	}
}

func TestConsultarUsaCpfDaQueryQuandoTokenNaoTem(t *testing.T) {
	t.Parallel()

	graphql := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		if !json.Valid(body) {
			t.Fatalf("corpo inválido: %s", body)
		}
		_, _ = io.WriteString(w, `{"data":{"rhPessoa":{"nome":"Ana Lima"}}}`)
	}))
	t.Cleanup(graphql.Close)

	app := fiber.New()
	app.Get("/api/usuarios/pessoa", NewHandler(NewClient(graphql.URL, fixedToken("token-de-servico"))).Consultar)

	request := httptest.NewRequest(http.MethodGet, "/api/usuarios/pessoa?cpf=11122233396", nil)
	request.Header.Set("Authorization", "Bearer token-sem-cpf")

	response, err := app.Test(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(response.Body)
		t.Fatalf("status %d: %s", response.StatusCode, body)
	}
}

func TestConsultarPrefereCpfDaQuery(t *testing.T) {
	t.Parallel()

	graphql := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		if !strings.Contains(string(body), "99988877766") {
			t.Fatalf("cpf da query não foi consultado: %s", body)
		}
		_, _ = io.WriteString(w, `{"data":{"rhPessoa":{"nome":"Outra Pessoa","cpf":"99988877766"}}}`)
	}))
	t.Cleanup(graphql.Close)

	app := fiber.New()
	app.Get("/api/usuarios/pessoa", NewHandler(NewClient(graphql.URL, fixedToken("token-de-servico"))).Consultar)

	token := "e30." + base64.RawURLEncoding.EncodeToString([]byte(`{"cpf":"11122233396"}`)) + ".assinatura"
	request := httptest.NewRequest(http.MethodGet, "/api/usuarios/pessoa?cpf=99988877766", nil)
	request.Header.Set("Authorization", "Bearer "+token)

	response, err := app.Test(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(response.Body)
		t.Fatalf("status %d: %s", response.StatusCode, body)
	}
}
