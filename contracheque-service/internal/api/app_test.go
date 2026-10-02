package api

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"tce.ap.gov.br/sistema-corporativo/contracheque-service/internal/contracheque"
)

type fakeRepo struct {
	rubricas    []contracheque.Rubrica
	decimo      []contracheque.Rubrica
	header      *contracheque.Header
	linhas      []contracheque.Linha
	meses       []contracheque.MesDisponivel
	mesesQtd    int
	consultaMes int
	consultaAno int
	consultaMat int
	salvo       string
	ver         *contracheque.Verificacao
}

func (f *fakeRepo) Consulta(_ context.Context, mes, ano, matricula int) ([]contracheque.Rubrica, error) {
	f.consultaMes, f.consultaAno, f.consultaMat = mes, ano, matricula
	return f.rubricas, nil
}
func (f *fakeRepo) ConsultaDecimo(context.Context, int, int, int) ([]contracheque.Rubrica, error) {
	return f.decimo, nil
}
func (f *fakeRepo) MesesDoAno(context.Context, int) ([]contracheque.MesDisponivel, error) {
	f.mesesQtd++
	return f.meses, nil
}
func (f *fakeRepo) Intranet(context.Context, int, int, int) (*contracheque.Header, []contracheque.Linha, error) {
	return f.header, f.linhas, nil
}
func (f *fakeRepo) SalvarCodigo(_ context.Context, _, _, _ int, codigo string) error {
	f.salvo = codigo
	return nil
}
func (f *fakeRepo) VerificarCodigo(context.Context, string) (*contracheque.Verificacao, error) {
	return f.ver, nil
}

func str(v string) *string { return &v }

func novoApp(repo *fakeRepo) *fiber.App {
	svc := contracheque.NewService(repo, "http://verifica/resposta", time.UTC, time.Minute)
	return New(Options{Handler: contracheque.NewHandler(svc, time.UTC)})
}

func chamar(t *testing.T, app *fiber.App, method, path, body string) (*http.Response, []byte) {
	t.Helper()
	req := httptest.NewRequest(method, path, bytes.NewBufferString(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	return resp, data
}

func TestDoMesUsaOrdemMesAnoMatricula(t *testing.T) {
	repo := &fakeRepo{rubricas: []contracheque.Rubrica{{Rubrica: 1, Valor: 10, ContraCheque: contracheque.NovoContraCheque()}}}
	resp, body := chamar(t, novoApp(repo), "GET", "/api/contra-cheque/3/2026/918", "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status %d: %s", resp.StatusCode, body)
	}
	if repo.consultaMes != 3 || repo.consultaAno != 2026 || repo.consultaMat != 918 {
		t.Fatalf("parâmetros %d/%d/%d", repo.consultaMes, repo.consultaAno, repo.consultaMat)
	}
	if strings.Contains(string(body), "contraCheque") {
		t.Fatalf("resposta inclui contraCheque: %s", body)
	}
}

func TestAcessoSemToken(t *testing.T) {
	resp, _ := chamar(t, novoApp(&fakeRepo{}), "GET", "/api/contra-cheque/informacao", "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status %d, esperado %d", resp.StatusCode, http.StatusOK)
	}
}

func TestIntranetFormatoPair(t *testing.T) {
	repo := &fakeRepo{
		header: &contracheque.Header{Nome: "FULANO", Cargo: "AUDITOR", Matricula: 10, CPF: str("12345678901")},
		linhas: []contracheque.Linha{{Rubrica: 1, Descricao: "SALARIO", Valor: 1500.5, TipoEvento: "P", Quantidade: "30"}},
	}
	resp, body := chamar(t, novoApp(repo), "GET", "/api/contra-cheque/intranet/mes/3/ano/2024/matricula/10", "")
	if resp.StatusCode != 200 {
		t.Fatalf("status %d: %s", resp.StatusCode, body)
	}
	var out struct {
		First  map[string]any   `json:"first"`
		Second []map[string]any `json:"second"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		t.Fatal(err)
	}
	if out.First["nome"] != "FULANO" || len(out.Second) != 1 || out.Second[0]["tipoEvento"] != "P" || out.Second[0]["valor"] != 1500.5 {
		t.Fatalf("resposta inesperada: %s", body)
	}

	// Sem dados: first nulo e second vazio, com status 200 (como o legado).
	resp, body = chamar(t, novoApp(&fakeRepo{}), "GET", "/api/contra-cheque/intranet/mes/3/ano/2024/matricula/10", "")
	if resp.StatusCode != 200 || string(body) != `{"first":null,"second":[]}` {
		t.Fatalf("sem dados: %d %s", resp.StatusCode, body)
	}
}

func TestPesquisarOrdenaDedupa(t *testing.T) {
	novo := func(rub int, tipo string, valor float64) contracheque.Rubrica {
		return contracheque.Rubrica{Rubrica: rub, Descricao: str("D"), TipoEvento: str(tipo), Valor: valor, ContraCheque: contracheque.NovoContraCheque()}
	}
	repo := &fakeRepo{rubricas: []contracheque.Rubrica{novo(1, "D", 10), novo(2, "P", 20), novo(2, "P", 20), novo(3, "P", 30)}}
	// o frontend envia os campos numéricos como texto
	resp, body := chamar(t, novoApp(repo), "POST", "/api/contra-cheque", `{"mes":"3","ano":"2024","matriculaSistema":"55"}`)
	if resp.StatusCode != 200 {
		t.Fatalf("status %d: %s", resp.StatusCode, body)
	}
	var out []map[string]any
	if err := json.Unmarshal(body, &out); err != nil {
		t.Fatal(err)
	}
	if len(out) != 3 || out[0]["rubrica"] != 2.0 || out[1]["rubrica"] != 3.0 || out[2]["rubrica"] != 1.0 {
		t.Fatalf("ordem/dedupe incorretos: %s", body)
	}
}

func TestDecimoVazioRetorna400(t *testing.T) {
	resp, body := chamar(t, novoApp(&fakeRepo{}), "POST", "/api/contra-cheque/decimo", `{"mes":11,"ano":2024,"matriculaSistema":1}`)
	var out map[string]any
	_ = json.Unmarshal(body, &out)
	if resp.StatusCode != 400 || out["message"] != "CONTRA CHEQUE/DECIMO NÃO LOCALIZADO" ||
		out["httpStatus"] != "BAD_REQUEST" || out["reason"] != "BAD REQUEST" || out["httpStatusCode"] != float64(400) {
		t.Fatalf("%d %s", resp.StatusCode, body)
	}
}

func TestMesesComCache(t *testing.T) {
	repo := &fakeRepo{meses: []contracheque.MesDisponivel{{Codigo: 3, Mes: "Março"}}}
	app := novoApp(repo)
	for i := 0; i < 3; i++ {
		resp, body := chamar(t, app, "GET", "/api/contra-cheque/meses-ecidade/2024/10", "")
		if resp.StatusCode != 200 || string(body) != `[{"codigo":3,"mes":"Março"}]` {
			t.Fatalf("%d %s", resp.StatusCode, body)
		}
	}
	if repo.mesesQtd != 1 {
		t.Fatalf("esperava 1 consulta ao banco, houve %d", repo.mesesQtd)
	}
}

func TestVerificarCodigo(t *testing.T) {
	repo := &fakeRepo{ver: &contracheque.Verificacao{Codigo: "A", Matricula: 7, Mes: 3, Ano: 2024, DataReg: time.Date(2024, 4, 1, 12, 0, 0, 0, time.UTC)}}
	resp, body := chamar(t, novoApp(repo), "GET", "/verifica-contracheque/A.B", "")
	want := `{"valido":true,"mensagem":"Contracheque autêntico","matricula":7,"mes":3,"ano":2024,"dataGeracao":"2024-04-01T12:00:00.000+00:00"}`
	if resp.StatusCode != 200 || string(body) != want {
		t.Fatalf("%d %s", resp.StatusCode, body)
	}
	resp, body = chamar(t, novoApp(&fakeRepo{}), "GET", "/verifica-contracheque/X", "")
	if resp.StatusCode != 404 || !strings.Contains(string(body), `"valido":false`) {
		t.Fatalf("%d %s", resp.StatusCode, body)
	}
}

func TestGerarPDF(t *testing.T) {
	adm := &contracheque.Data{Time: time.Date(2010, 5, 3, 0, 0, 0, 0, time.UTC), SoData: true}
	repo := &fakeRepo{
		header: &contracheque.Header{Nome: "FULANO DE TAL DA SILVA SAURO", Cargo: "AUDITOR DE CONTROLE EXTERNO", Matricula: 10,
			CPF: str("12345678901"), Lotacao: str("DIRETORIA DE TECNOLOGIA DA INFORMAÇÃO E COMUNICAÇÃO"), Admissao: adm,
			Banco: str("BANCO DO BRASIL"), Agencia: str("1234-5"), Conta: str("99999-0")},
	}
	for i := 0; i < 60; i++ { // força quebra de página
		repo.linhas = append(repo.linhas, contracheque.Linha{Rubrica: i, Descricao: "RUBRICA AÇÃO Nº " + string(rune('A'+i%26)), Valor: 1234.56, TipoEvento: "P", Quantidade: "30"})
	}
	repo.linhas = append(repo.linhas,
		contracheque.Linha{Rubrica: 900, Descricao: "IMPOSTO DE RENDA", Valor: 500, TipoEvento: "D"},
		contracheque.Linha{Rubrica: 3120, Descricao: "BASE IRRF (FOLHA)", Valor: 1000, TipoEvento: "F"})

	resp, body := chamar(t, novoApp(repo), "POST", "/api/contra-cheque/gerar", `{"mes":"03","ano":2024,"matricula":10}`)
	if resp.StatusCode != 200 || resp.Header.Get("Content-Type") != "application/pdf" || !bytes.HasPrefix(body, []byte("%PDF")) {
		t.Fatalf("%d %s %.40s", resp.StatusCode, resp.Header.Get("Content-Type"), body)
	}
	if got := resp.Header.Get("Content-Disposition"); got != `inline; filename="contra-cheque-10-03-2024.pdf"` {
		t.Fatalf("content-disposition: %s", got)
	}
	if repo.salvo == "" || strings.Count(repo.salvo, ".") != 4 {
		t.Fatalf("código de verificação não gravado/formatado: %q", repo.salvo)
	}
	if out := os.Getenv("PDF_OUT"); out != "" {
		_ = os.WriteFile(out, body, 0o644)
	}

	// Sem contracheque na competência: 404 (o frontend trata esse caso).
	resp, _ = chamar(t, novoApp(&fakeRepo{}), "POST", "/api/contra-cheque/gerar", `{"mes":3,"ano":2024,"matricula":10}`)
	if resp.StatusCode != 404 {
		t.Fatalf("esperado 404, veio %d", resp.StatusCode)
	}
}

func TestRotasEstaticasNaoCapturadasPorParametrizada(t *testing.T) {
	repo := &fakeRepo{meses: []contracheque.MesDisponivel{}}
	resp, _ := chamar(t, novoApp(repo), "GET", "/api/contra-cheque/meses-ecidade/2024/1", "")
	if resp.StatusCode != 200 || repo.mesesQtd != 1 {
		t.Fatalf("meses-ecidade roteada incorretamente: %d", resp.StatusCode)
	}
}
