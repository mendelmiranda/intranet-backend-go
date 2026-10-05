package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	sharedstatus "tce.ap.gov.br/sistema-corporativo/shared-common/status"
)

func TestStatusEndpoint(t *testing.T) {
	t.Parallel()

	app := New(Options{})
	request := httptest.NewRequest(http.MethodGet, "/api/usuarios/status", nil)
	response, err := app.Test(request)
	if err != nil {
		t.Fatalf("erro ao executar requisição: %v", err)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		t.Fatalf("status esperado %d; recebido %d", http.StatusOK, response.StatusCode)
	}

	var payload sharedstatus.Response
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		t.Fatalf("erro ao decodificar resposta: %v", err)
	}
	if payload.Service != serviceName || payload.Status != sharedstatus.Active {
		t.Fatalf("resposta inesperada: %+v", payload)
	}
}

func BenchmarkStatusEndpoint(b *testing.B) {
	app := New(Options{})
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		request := httptest.NewRequest(http.MethodGet, "/api/usuarios/status", nil)
		response, err := app.Test(request)
		if err != nil {
			b.Fatalf("erro ao executar requisição: %v", err)
		}
		response.Body.Close()
	}
}
