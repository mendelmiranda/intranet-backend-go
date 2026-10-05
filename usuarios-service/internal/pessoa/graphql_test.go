package pessoa

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type fixedToken string

func (token fixedToken) ServiceToken(context.Context) (string, error) {
	return string(token), nil
}

func TestNomeEncaminhaTokenECpf(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer token-de-servico" {
			t.Fatalf("authorization: %s", r.Header.Get("Authorization"))
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(body), `rhPessoa(cpf: \"11122233396\")`) && !strings.Contains(string(body), `rhPessoa(cpf: "11122233396")`) {
			t.Fatalf("query inesperada: %s", body)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"data":{"rhPessoa":{"nome":"Ana Lima"}}}`)
	}))
	t.Cleanup(server.Close)

	cadastro, err := NewClient(server.URL, fixedToken("token-de-servico")).Buscar(context.Background(), "111.222.333-96")
	if err != nil {
		t.Fatal(err)
	}
	if cadastro.Nome != "Ana Lima" {
		t.Fatalf("nome %q", cadastro.Nome)
	}
}

func TestNomeRepassaErroDaAPI(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `{"errors":[{"message":"Token inválido"}]}`)
	}))
	t.Cleanup(server.Close)

	_, err := NewClient(server.URL, fixedToken("token")).Buscar(context.Background(), "11122233396")
	var apiErr *Error
	if err == nil || !errors.As(err, &apiErr) || apiErr.Message != "Token inválido" {
		t.Fatalf("erro inesperado: %v", err)
	}
}
