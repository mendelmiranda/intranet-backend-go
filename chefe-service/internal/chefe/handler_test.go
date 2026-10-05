package chefe

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

type fakeRepo struct {
	porChefe  map[string][]ChefeFuncionario
	excecao   []string
	removidos [][2]string
	salvo     *ChefeFuncionario
	atualizou []string
}

func (f *fakeRepo) Listar(context.Context) ([]ChefeFuncionario, error) { return nil, nil }
func (f *fakeRepo) BuscarPorID(context.Context, int64) (*ChefeFuncionario, error) {
	return nil, nil
}
func (f *fakeRepo) BuscarPorCPFChefe(_ context.Context, cpf string) ([]ChefeFuncionario, error) {
	return f.porChefe[cpf], nil
}
func (f *fakeRepo) BuscarPorCPFFuncionario(context.Context, string) (*ChefeFuncionario, error) {
	return nil, nil
}
func (f *fakeRepo) ListarPorLotacao(context.Context, int) ([]ChefeFuncionario, error) {
	return nil, nil
}
func (f *fakeRepo) Salvar(_ context.Context, c ChefeFuncionario) (int64, error) {
	f.salvo = &c
	return 7, nil
}
func (f *fakeRepo) AtualizarChefe(_ context.Context, n, novo, antigo string) (int64, error) {
	f.atualizou = []string{n, novo, antigo}
	return 1, nil
}
func (f *fakeRepo) RemoverServidor(_ context.Context, a, b string) (int64, error) {
	f.removidos = append(f.removidos, [2]string{a, b})
	return 1, nil
}
func (f *fakeRepo) CPFsExcecao(context.Context) ([]string, error)        { return f.excecao, nil }
func (f *fakeRepo) GravarLog(context.Context, string, int, string) error { return nil }

type fakeEfetivos map[string]bool

func (f fakeEfetivos) Efetivos(_ context.Context, cpfs []string) (map[string]bool, error) {
	out := map[string]bool{}
	for _, c := range cpfs {
		if f[c] {
			out[c] = true
		}
	}
	return out, nil
}

func chamar(t *testing.T, repo *fakeRepo, ef fakeEfetivos, method, path, body string) (*http.Response, []byte) {
	t.Helper()
	app := fiber.New()
	NewHandler(NewService(repo, ef)).Register(app.Group("/api/chefe"))
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp, b
}

func vinculos() *fakeRepo {
	return &fakeRepo{
		porChefe: map[string][]ChefeFuncionario{
			"111": {
				{ID: 1, CPFChefe: "111", CPFFuncionario: "222", NomeFuncionario: "Ana"},
				{ID: 2, CPFChefe: "111", CPFFuncionario: "333", NomeFuncionario: "Bia"},
				{ID: 3, CPFChefe: "111", CPFFuncionario: "444", NomeFuncionario: "Caio"},
				{ID: 4, CPFChefe: "111", CPFFuncionario: "555", NomeFuncionario: "Davi"},
			},
			"333": {{ID: 9, CPFChefe: "333", CPFFuncionario: "666"}},
		},
		excecao: []string{"444"},
	}
}

func nomes(t *testing.T, body []byte) []string {
	t.Helper()
	var lista []ChefeFuncionario
	if err := json.Unmarshal(body, &lista); err != nil {
		t.Fatal(err, string(body))
	}
	var out []string
	for _, c := range lista {
		out = append(out, c.NomeFuncionario)
	}
	return out
}

func TestAvaliacaoFiltraEfetivosEExcecoes(t *testing.T) {
	ef := fakeEfetivos{"222": true, "333": true, "444": true}
	_, body := chamar(t, vinculos(), ef, http.MethodGet, "/api/chefe/cpf-chefe/111/avaliacao", "")
	if got := strings.Join(nomes(t, body), ","); got != "Ana,Bia" {
		t.Fatalf("got %s", got)
	}
}

func TestAvaliacaoComParesRemoveChefes(t *testing.T) {
	ef := fakeEfetivos{"222": true, "333": true}
	_, body := chamar(t, vinculos(), ef, http.MethodGet, "/api/chefe/cpf-chefe/111/avaliacao/pares", "")
	if got := strings.Join(nomes(t, body), ","); got != "Ana" {
		t.Fatalf("got %s", got)
	}
}

func TestServidoresDoChefeVazioEhArray(t *testing.T) {
	_, body := chamar(t, vinculos(), nil, http.MethodGet, "/api/chefe/cpf-chefe/999", "")
	if strings.TrimSpace(string(body)) != "[]" {
		t.Fatalf("got %s", body)
	}
}

func TestChefeDoFuncionarioAusenteResponde200Null(t *testing.T) {
	resp, body := chamar(t, vinculos(), nil, http.MethodGet, "/api/chefe/cpf-funcionario/000", "")
	if resp.StatusCode != 200 || strings.TrimSpace(string(body)) != "null" {
		t.Fatalf("status %d body %s", resp.StatusCode, body)
	}
}

func TestEfetivo(t *testing.T) {
	_, body := chamar(t, vinculos(), fakeEfetivos{"111": true}, http.MethodGet, "/api/chefe/efetivo/cpf/111", "")
	if strings.TrimSpace(string(body)) != "true" {
		t.Fatalf("got %s", body)
	}
}

func TestNovoResponde201ComLocation(t *testing.T) {
	repo := vinculos()
	resp, _ := chamar(t, repo, nil, http.MethodPost, "/api/chefe",
		`{"cpfChefe":"1","cpfFuncionario":"2","nomeFuncionario":"X","codLotacao":5}`)
	if resp.StatusCode != 201 || !strings.HasSuffix(resp.Header.Get("Location"), "/api/chefe/7") {
		t.Fatalf("status %d loc %s", resp.StatusCode, resp.Header.Get("Location"))
	}
	if repo.salvo == nil || *repo.salvo.CodLotacao != 5 {
		t.Fatalf("salvo %+v", repo.salvo)
	}
}

func TestNovoSemCPFResponde400(t *testing.T) {
	resp, _ := chamar(t, vinculos(), nil, http.MethodPost, "/api/chefe", `{"cpfChefe":"1"}`)
	if resp.StatusCode != 400 {
		t.Fatalf("status %d", resp.StatusCode)
	}
}

func TestAtualizarChefe(t *testing.T) {
	repo := vinculos()
	resp, _ := chamar(t, repo, nil, http.MethodPut, "/api/chefe",
		`{"cpfChefeAntigo":"111","cpfChefeNovo":"777","nomeChefeNovo":"Novo"}`)
	if resp.StatusCode != 201 || strings.Join(repo.atualizou, "|") != "Novo|777|111" {
		t.Fatalf("status %d %v", resp.StatusCode, repo.atualizou)
	}
	resp, _ = chamar(t, repo, nil, http.MethodPut, "/api/chefe",
		`{"cpfChefeAntigo":"000","cpfChefeNovo":"777","nomeChefeNovo":"Novo"}`)
	if resp.StatusCode != 404 {
		t.Fatalf("status %d", resp.StatusCode)
	}
}

func TestRemoverServidor(t *testing.T) {
	repo := vinculos()
	resp, _ := chamar(t, repo, nil, http.MethodDelete, "/api/chefe/remover-servidor/111/222", "")
	if resp.StatusCode != 204 || len(repo.removidos) != 1 || repo.removidos[0] != [2]string{"111", "222"} {
		t.Fatalf("status %d %v", resp.StatusCode, repo.removidos)
	}
}
