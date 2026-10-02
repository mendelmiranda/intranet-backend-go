package servidor

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v3"
)

type fakeRepo struct {
	filtro Filtro
	rows   []Servidor
	err    error
	called bool
}

func (f *fakeRepo) Buscar(_ context.Context, filtro Filtro) ([]Servidor, error) {
	f.called = true
	f.filtro = filtro
	return f.rows, f.err
}

func (f *fakeRepo) Listar(_ context.Context, ativo string) ([]Servidor, error) {
	f.called = true
	f.filtro.Ativo = ativo
	return f.rows, f.err
}

func chamar(t *testing.T, repo *fakeRepo, path string) (*http.Response, []byte) {
	t.Helper()
	app := fiber.New()
	NewHandler(repo).Register(app)
	req := httptest.NewRequest(http.MethodGet, path, nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	return resp, body
}

func TestDetalheExigeFiltro(t *testing.T) {
	repo := &fakeRepo{}
	resp, body := chamar(t, repo, "/api/servidores/detalhe")
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status %d, corpo %s", resp.StatusCode, body)
	}
	if repo.called {
		t.Fatal("repositório não deveria ser chamado")
	}
}

func TestDetalhePorCPF(t *testing.T) {
	nome := "Servidor Teste"
	repo := &fakeRepo{rows: []Servidor{{Nome: &nome, CGM: 1, Ativo: "SIM"}}}
	resp, body := chamar(t, repo, "/api/servidores/detalhe?cpf=123.456.789-09")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status %d, corpo %s", resp.StatusCode, body)
	}
	if repo.filtro.CPF != "12345678909" {
		t.Fatalf("cpf recebido %q", repo.filtro.CPF)
	}
	var resultado Resultado
	if err := json.Unmarshal(body, &resultado); err != nil {
		t.Fatal(err)
	}
	if resultado.Total != 1 || resultado.Servidores[0].CGM != 1 {
		t.Fatalf("resultado inesperado: %+v", resultado)
	}
}

func TestDetalhePorTermoMatricula(t *testing.T) {
	repo := &fakeRepo{rows: []Servidor{{CGM: 7, Ativo: "SIM"}}}
	resp, body := chamar(t, repo, "/api/servidores/detalhe?q=918")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status %d, corpo %s", resp.StatusCode, body)
	}
	if repo.filtro.Matricula == nil || *repo.filtro.Matricula != 918 {
		t.Fatalf("matrícula recebida %+v", repo.filtro.Matricula)
	}
}

func TestDetalheNaoEncontrado(t *testing.T) {
	repo := &fakeRepo{}
	resp, _ := chamar(t, repo, "/api/servidores/detalhe?nome=inexistente")
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status %d", resp.StatusCode)
	}
}

func TestListarPorAtivo(t *testing.T) {
	repo := &fakeRepo{rows: []Servidor{{CGM: 1, Ativo: "SIM"}, {CGM: 2, Ativo: "SIM"}}}
	resp, body := chamar(t, repo, "/api/servidores?ativo=sim")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status %d, corpo %s", resp.StatusCode, body)
	}
	if repo.filtro.Ativo != "SIM" {
		t.Fatalf("ativo recebido %q", repo.filtro.Ativo)
	}
	var resultado Resultado
	if err := json.Unmarshal(body, &resultado); err != nil {
		t.Fatal(err)
	}
	if resultado.Total != 2 {
		t.Fatalf("total %d", resultado.Total)
	}
}

func TestListarAtivoInvalido(t *testing.T) {
	repo := &fakeRepo{}
	resp, _ := chamar(t, repo, "/api/servidores?ativo=talvez")
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status %d", resp.StatusCode)
	}
	if repo.called {
		t.Fatal("repositório não deveria ser chamado")
	}
}

func TestInterpretarConsulta(t *testing.T) {
	filtro, err := interpretarConsulta("", "", "", "123.456.789-09")
	if err != nil || filtro.CPF != "12345678909" {
		t.Fatalf("cpf: %+v %v", filtro, err)
	}
	filtro, err = interpretarConsulta("", "", "", "Maria Silva")
	if err != nil || filtro.Nome != "Maria Silva" {
		t.Fatalf("nome: %+v %v", filtro, err)
	}
	filtro, err = interpretarConsulta("52998224725", "Ana", "10", "")
	if err != nil || filtro.CPF != "52998224725" || filtro.Nome != "Ana" || filtro.Matricula == nil || *filtro.Matricula != 10 {
		t.Fatalf("combinado: %+v %v", filtro, err)
	}
	if _, err := interpretarConsulta("123", "", "", ""); err == nil {
		t.Fatal("cpf curto deveria falhar")
	}
}
