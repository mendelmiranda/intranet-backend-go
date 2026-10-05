package pessoa

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v3"
)

func TestCargosAgrupaPaginasEFiltraNome(t *testing.T) {
	t.Parallel()

	var chamadas int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		chamadas++
		var body struct {
			Variables struct {
				After *string `json:"after"`
			} `json:"variables"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		w.Header().Set("Content-Type", "application/json")
		if body.Variables.After == nil {
			_, _ = io.WriteString(w, `{
				"data":{"rhVinculos":{
					"pageInfo":{"hasNextPage":true,"endCursor":"pagina-2"},
					"nodes":[
						{"cargo":{"codigo":"472","nome":"ASSESSOR I"}},
						{"cargo":{"codigo":"472","nome":"ASSESSOR I"}},
						{"cargo":null}
					]
				}}
			}`)
			return
		}
		if *body.Variables.After != "pagina-2" {
			t.Fatalf("cursor %q", *body.Variables.After)
		}
		_, _ = io.WriteString(w, `{
			"data":{"rhVinculos":{
				"pageInfo":{"hasNextPage":false,"endCursor":null},
				"nodes":[
					{"cargo":{"codigo":"383","nome":"AUDITOR DE CONTROLE EXTERNO"}},
					{"cargo":{"codigo":"475","nome":"ASSISTENTE I"}}
				]
			}}
		}`)
	}))
	t.Cleanup(server.Close)

	app := fiber.New()
	app.Get("/api/usuarios/cargos", NewHandler(NewClient(server.URL, fixedToken("token-de-servico"))).Cargos)

	request := httptest.NewRequest(http.MethodGet, "/api/usuarios/cargos?q=ass", nil)
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

	var payload Cargos
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		t.Fatal(err)
	}
	if chamadas != 2 || payload.Total != 2 {
		t.Fatalf("chamadas %d payload %+v", chamadas, payload)
	}
	if payload.Cargos[0].Nome != "ASSESSOR I" || payload.Cargos[0].Quantidade != 2 || payload.Cargos[1].Nome != "ASSISTENTE I" {
		t.Fatalf("ordem inesperada: %+v", payload.Cargos)
	}
}

func TestServidoresDoCargo(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Variables struct {
				Codigo string `json:"codigo"`
			} `json:"variables"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if body.Variables.Codigo != "472" {
			t.Fatalf("codigo %q", body.Variables.Codigo)
		}
		_, _ = io.WriteString(w, `{
			"data":{"rhVinculos":{
				"pageInfo":{"hasNextPage":false,"endCursor":null},
				"nodes":[
					{"matricula":10,"lotacao":{"nome":"Gabinete"},"pessoa":{"cpf":"11122233396","nome":"Ana Lima"}},
					{"matricula":11,"lotacao":{"nome":"Escola"},"pessoa":{"cpf":"99988877766","nome":"Bruno Souza"}}
				]
			}}
		}`)
	}))
	t.Cleanup(server.Close)

	app := fiber.New()
	app.Get("/api/usuarios/cargos/servidores", NewHandler(NewClient(server.URL, fixedToken("token"))).Servidores)
	request := httptest.NewRequest(http.MethodGet, "/api/usuarios/cargos/servidores?codigo=472", nil)
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
	var payload ServidoresCargo
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		t.Fatal(err)
	}
	if payload.Total != 2 || payload.Servidores[0].Nome != "Ana Lima" || payload.Servidores[1].Lotacao != "Escola" {
		t.Fatalf("servidores %+v", payload)
	}
}
