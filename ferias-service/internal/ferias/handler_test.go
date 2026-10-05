package ferias

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/golang-jwt/jwt/v5"
)

var chave = []byte("segredo")

func token(t *testing.T, roles string) string {
	t.Helper()
	s, err := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub": "maria", "roles": roles, "exp": time.Now().Add(time.Hour).Unix(),
	}).SignedString(chave)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func chamar(t *testing.T, repo *fakeRepo, roles, method, path, body string) (*http.Response, string) {
	t.Helper()
	app := fiber.New(fiber.Config{ErrorHandler: ErrorHandler})
	NewHandler(NewService(repo, fakeFolha{servidor: servidorOK()}), chave).Register(app)
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if roles != "" {
		req.Header.Set("Authorization", "Bearer "+token(t, roles))
	}
	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp, string(b)
}

func TestSemTokenResponde401(t *testing.T) {
	resp, _ := chamar(t, &fakeRepo{}, "", http.MethodGet, "/api/programacao/2026/111", "")
	if resp.StatusCode != 401 {
		t.Fatalf("status %d", resp.StatusCode)
	}
}

func TestRolesPorRota(t *testing.T) {
	casos := []struct {
		roles, method, path string
		status              int
	}{
		{"ROLE_DASHBOARD", http.MethodPost, "/api/programacao/restantes", 403},
		{"ROLE_DASHBOARD", http.MethodPost, "/api/programacao-observacao", 403},
		{"ROLE_DASHBOARD", http.MethodPut, "/api/periodo-aquisitivo/update", 403},
		{"ROLE_OUTRA", http.MethodGet, "/api/programacao/ano/2026", 403},
		{"ROLE_DASHBOARD_NORMAL", http.MethodGet, "/api/programacao/ano/abc", 400},
	}
	for _, c := range casos {
		resp, body := chamar(t, &fakeRepo{}, c.roles, c.method, c.path, `{}`)
		if resp.StatusCode != c.status {
			t.Errorf("%s %s com %s: status %d (%s), esperado %d", c.method, c.path, c.roles, resp.StatusCode, body, c.status)
		}
	}
}

func TestErroDeNegocioViraHttpResponse400(t *testing.T) {
	repo := &fakeRepo{existentes: []Programacao{{ID: 1, Periodo: "P", DataInicio: dia("2026-07-01"), DataFim: dia("2026-07-20")}}}
	resp, body := chamar(t, repo, "ROLE_DASHBOARD", http.MethodPost, "/api/programacao",
		`{"periodo":"S","cpf":"111","ano":2026,"dataInicio":"2026-12-01","dataFim":"2026-12-15"}`)
	if resp.StatusCode != 400 || !strings.Contains(body, `"message":"TOTAL DE DIAS MAIOR QUE 30."`) ||
		!strings.Contains(body, `"httpStatus":"BAD_REQUEST"`) {
		t.Fatalf("status %d body %s", resp.StatusCode, body)
	}
}

func TestPostCriaComLocation(t *testing.T) {
	resp, _ := chamar(t, &fakeRepo{}, "ROLE_RH_AVISO_FERIAS", http.MethodPost, "/api/programacao",
		`{"periodo":"P","cpf":"111","ano":2026,"dataInicio":1782864000000,"dataFim":1783641600000}`)
	if resp.StatusCode != 201 || !strings.HasSuffix(resp.Header.Get("Location"), "/api/programacao/99") {
		t.Fatalf("status %d loc %s", resp.StatusCode, resp.Header.Get("Location"))
	}
}

func TestCorpoInvalidoResponde400(t *testing.T) {
	resp, _ := chamar(t, &fakeRepo{}, "ROLE_DASHBOARD", http.MethodPost, "/api/programacao", `{"dataInicio":"ontem"}`)
	if resp.StatusCode != 400 {
		t.Fatalf("status %d", resp.StatusCode)
	}
}

func TestDeleteResponde204(t *testing.T) {
	paga := "N"
	p := Programacao{ID: 1, Periodo: "S", FeriasPaga: &paga, DataInicio: dia("2026-12-01")}
	repo := &fakeRepo{porID: map[int64]*Programacao{1: &p}}
	resp, _ := chamar(t, repo, "ROLE_DASHBOARD", http.MethodDelete, "/api/programacao/remover/1", "")
	if resp.StatusCode != 204 || len(repo.excluidos) != 1 {
		t.Fatalf("status %d", resp.StatusCode)
	}
}

func TestComprovanteDoAvisoEntregaPDF(t *testing.T) {
	repo := &fakeRepo{porID: map[int64]*Programacao{8: progBanco()}, existentes: []Programacao{*progBanco()}}
	resp, body := chamar(t, repo, "ROLE_RH_AVISO_FERIAS", http.MethodGet, "/api/programacao/comprovante/8", "")
	if resp.StatusCode != 200 || resp.Header.Get("Content-Type") != "application/pdf" || !strings.HasPrefix(body, "%PDF") ||
		!strings.Contains(resp.Header.Get("Content-Disposition"), "AVISO_DE_FERIAS_2026.pdf") {
		t.Fatalf("status %d %v", resp.StatusCode, resp.Header)
	}
	resp, _ = chamar(t, &fakeRepo{}, "ROLE_RH_AVISO_FERIAS", http.MethodGet, "/api/programacao/comprovante/8", "")
	if resp.StatusCode != 400 {
		t.Fatalf("inexistente: status %d", resp.StatusCode)
	}
	resp, _ = chamar(t, repo, "ROLE_DASHBOARD", http.MethodGet, "/api/programacao/comprovante/8", "")
	if resp.StatusCode != 403 {
		t.Fatalf("sem RH: status %d", resp.StatusCode)
	}
}

func TestPagamentoSemDataPagamentoResponde400(t *testing.T) {
	resp, _ := chamar(t, &fakeRepo{}, "ROLE_RH_AVISO_FERIAS", http.MethodPut, "/api/pagamento/programacao/gerar-comprovante/1",
		`{"id":1}`)
	if resp.StatusCode != 400 {
		t.Fatalf("status %d", resp.StatusCode)
	}
}

func TestRelatorioPDF(t *testing.T) {
	pdf, err := gerarRelatorioPDF([]Programacao{{NomeFuncionario: "João da Conceição", Matricula: 1, Ano: 2026, Periodo: "P",
		DataInicio: dia("2026-07-01"), DataFim: dia("2026-07-10"), Autorizado: "S"}})
	if err != nil || !strings.HasPrefix(string(pdf), "%PDF") {
		t.Fatalf("pdf inválido: %v", err)
	}
}
