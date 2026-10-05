package pessoa

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"
)

func TestPesquisarEnviaNomeEResumeVinculoAtivo(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer token-de-servico" {
			t.Fatalf("authorization: %s", r.Header.Get("Authorization"))
		}
		var body struct {
			Variables struct {
				Nome string `json:"nome"`
			} `json:"variables"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if body.Variables.Nome != "Ana Lima" {
			t.Fatalf("nome %q", body.Variables.Nome)
		}
		_, _ = io.WriteString(w, `{
			"data": {
				"rhPessoas": {
					"totalCount": 2,
					"nodes": [{
						"cpf": "11122233396",
						"nome": "Ana Lima",
						"ativo": true,
						"vinculos": [
							{"tipo": "COMISSIONADO", "ativo": false, "lotacao": {"nome": "Antiga"}},
							{"tipo": "EFETIVO", "ativo": true, "lotacao": {"nome": "Escola de Contas"}}
						]
					}]
				}
			}
		}`)
	}))
	t.Cleanup(server.Close)

	app := fiber.New()
	app.Get("/api/usuarios/pessoas", NewHandler(NewClient(server.URL, fixedToken("token-de-servico"))).Pesquisar)

	request := httptest.NewRequest(http.MethodGet, "/api/usuarios/pessoas?q=Ana%20Lima", nil)
	request.Header.Set("Authorization", "Bearer sessao")
	response, err := app.Test(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(response.Body)
		t.Fatalf("status %d: %s", response.StatusCode, body)
	}

	var payload Pesquisa
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		t.Fatal(err)
	}
	if payload.Total != 2 || len(payload.Pessoas) != 1 || payload.Pessoas[0].Tipo != "EFETIVO" || payload.Pessoas[0].Lotacao != "Escola de Contas" {
		t.Fatalf("pesquisa inesperada: %+v", payload)
	}
}

func TestPesquisarRecusaNomeCurto(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	app.Get("/api/usuarios/pessoas", NewHandler(NewClient("http://graphql.invalid", fixedToken("token"))).Pesquisar)
	request := httptest.NewRequest(http.MethodGet, "/api/usuarios/pessoas?q=a", nil)
	request.Header.Set("Authorization", "Bearer sessao")
	response, err := app.Test(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusBadRequest {
		t.Fatalf("status %d", response.StatusCode)
	}
}

func TestNomePesquisa(t *testing.T) {
	t.Parallel()
	nome, ok := nomePesquisa("  ana   lima ")
	if !ok || nome != "Ana Lima" && nome != "ana lima" {
		if !ok || !strings.EqualFold(nome, "ana lima") {
			t.Fatalf("nome %q ok %v", nome, ok)
		}
	}
	if _, ok := nomePesquisa("a"); ok {
		t.Fatal("nome curto aceito")
	}
	if _, ok := nomePesquisa("ana 1"); ok {
		t.Fatal("número aceito")
	}
	resultado, err := NewClient("http://graphql.invalid", fixedToken("token")).Pesquisar(context.Background(), "a")
	if err == nil || resultado.Total != 0 {
		t.Fatal(err)
	}
}
