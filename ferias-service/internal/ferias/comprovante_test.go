package ferias

import (
	"os"
	"strings"
	"testing"
	"time"
)

func dadosAviso() DadosComprovante {
	pag := "Paga em 05/10/2026"
	return DadosComprovante{
		Tipo: ComprovanteAviso, Servidor: "ANA PAULA DA CONCEIÇÃO SOUZA", Matricula: 1234, CPF: "12345678909",
		Lotacao: "DIVISÃO DE GESTÃO DO ARQUIVO", Admissao: admissao(time.Date(2011, 8, 1, 0, 0, 0, 0, time.UTC)),
		Exercicio: 2026, AbonoPecuniario: "N", AnteciparDecimo: "S", Autorizado: "S",
		Periodos: []PeriodoComprovante{
			{Rotulo: "1º período", Inicio: dia("2026-07-01"), Fim: dia("2026-07-15"), Situacao: pag},
			{Rotulo: "2º período", Inicio: dia("2026-12-01"), Fim: dia("2026-12-15"), Situacao: "Programada"},
		},
		Emissao: time.Date(2026, 10, 5, 10, 0, 0, 0, belem),
	}
}

func TestComprovanteAvisoGeraPDFComConteudo(t *testing.T) {
	pdf, err := GerarComprovante(dadosAviso())
	if err != nil || !strings.HasPrefix(string(pdf), "%PDF") {
		t.Fatalf("pdf inválido: %v", err)
	}
	if dir := os.Getenv("COMPROVANTE_AMOSTRAS"); dir != "" { // amostras para conferência visual
		_ = os.WriteFile(dir+"/aviso.pdf", pdf, 0o600)
		alt := dadosAviso()
		alt.Tipo = ComprovanteAlteracao
		alt.Anterior = &PeriodoComprovante{Rotulo: "Anterior", Inicio: dia("2026-07-01"), Fim: dia("2026-07-15"), Situacao: "2026"}
		alt.Novo = &PeriodoComprovante{Rotulo: "Novo", Inicio: dia("2026-08-03"), Fim: dia("2026-08-17"), Situacao: "2026"}
		alt.Observacao = "Alteração solicitada pela chefia imediata em razão da necessidade do serviço, conforme despacho."
		b, _ := GerarComprovante(alt)
		_ = os.WriteFile(dir+"/alteracao.pdf", b, 0o600)
		rev := dadosAviso()
		rev.Tipo = ComprovanteReversao
		rev.Periodos = nil
		rev.Observacao = "Pagamento revertido para correção da data de início do período."
		b, _ = GerarComprovante(rev)
		_ = os.WriteFile(dir+"/reversao.pdf", b, 0o600)
	}
}

func TestCodigoDeControleEstavel(t *testing.T) {
	a, b := dadosAviso(), dadosAviso()
	b.Emissao = b.Emissao.Add(48 * time.Hour)
	if a.CodigoControle() != b.CodigoControle() {
		t.Fatal("a data de emissão não deve alterar o código")
	}
	b.Periodos[0].Fim = dia("2026-07-20")
	if a.CodigoControle() == b.CodigoControle() {
		t.Fatal("alterar um período deve alterar o código")
	}
	if len(a.CodigoControle()) != 19 {
		t.Fatalf("formato: %s", a.CodigoControle())
	}
}

func TestFormatarCPFEDataExtenso(t *testing.T) {
	if formatarCPF("12345678909") != "123.456.789-09" || formatarCPF("123.456.789-09") != "123.456.789-09" {
		t.Fatal("cpf")
	}
	if got := dataExtenso(time.Date(2026, 3, 1, 12, 0, 0, 0, belem)); got != "1 de março de 2026" {
		t.Fatal(got)
	}
}
