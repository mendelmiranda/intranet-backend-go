package ferias

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"
)

type fakeRepo struct {
	Repository // métodos não usados entram em pânico, denunciando uso inesperado

	existentes  []Programacao
	porID       map[int64]*Programacao
	inseridas   []Programacao
	atualizadas []Programacao
	historico   []Programacao
	excluidos   []int64
	ultimoAno   int
	documentos  []string
	observacoes []Observacao
	revertidos  []int64
	historicos  []Historico
}

func (f *fakeRepo) ProgramacoesDoAno(context.Context, int, string) ([]Programacao, error) {
	return f.existentes, nil
}
func (f *fakeRepo) PorID(_ context.Context, id int64) (*Programacao, error) { return f.porID[id], nil }
func (f *fakeRepo) Inserir(_ context.Context, p Programacao) (int64, error) {
	f.inseridas = append(f.inseridas, p)
	return 99, nil
}
func (f *fakeRepo) Atualizar(_ context.Context, p Programacao) error {
	f.atualizadas = append(f.atualizadas, p)
	return nil
}
func (f *fakeRepo) HistoricoPorCPF(context.Context, string) ([]Historico, error) {
	return f.historicos, nil
}
func (f *fakeRepo) InserirDocumento(_ context.Context, _, cpf, arquivo string, _ time.Time) error {
	f.documentos = append(f.documentos, cpf+arquivo)
	return nil
}
func (f *fakeRepo) InserirObservacao(_ context.Context, o Observacao) (int64, error) {
	f.observacoes = append(f.observacoes, o)
	return 55, nil
}
func (f *fakeRepo) ReverterPagamento(_ context.Context, id int64) error {
	f.revertidos = append(f.revertidos, id)
	return nil
}
func (f *fakeRepo) InserirHistorico(_ context.Context, p Programacao) error {
	f.historico = append(f.historico, p)
	return nil
}
func (f *fakeRepo) Excluir(_ context.Context, id int64) error {
	f.excluidos = append(f.excluidos, id)
	return nil
}
func (f *fakeRepo) UltimoPeriodoAquisitivo(context.Context) (*PeriodoAquisitivo, error) {
	return &PeriodoAquisitivo{Ano: f.ultimoAno}, nil
}
func (f *fakeRepo) GravarLog(context.Context, string, int, string) error { return nil }

type fakeFolha struct {
	Folha
	servidor *ServidorFolha
}

func (f fakeFolha) PorCPF(context.Context, string) (*ServidorFolha, error) { return f.servidor, nil }

func servidorOK() *ServidorFolha {
	nome, lot, mat := "Ana Souza", 7, 1234
	return &ServidorFolha{Nome: &nome, CodLotacao: &lot, Matricula: &mat}
}

func dia(s string) *Data {
	t, _ := time.Parse("2006-01-02", s)
	return &Data{Momento{T: t, Dia: true}}
}

func prog(periodo, ini, fim string) Programacao {
	return Programacao{Periodo: periodo, CPF: "111", Ano: 2026, DataInicio: dia(ini), DataFim: dia(fim)}
}

func novoServico(r *fakeRepo) *Service {
	s := NewService(r, fakeFolha{servidor: servidorOK()})
	s.agora = func() time.Time { return time.Date(2026, 5, 1, 12, 0, 0, 0, belem) }
	return s
}

func TestTotalDiasContaInclusivo(t *testing.T) {
	if got := totalDias(dia("2026-07-01"), dia("2026-07-20")); got != 20 {
		t.Fatalf("got %d", got)
	}
	if totalDias(dia("2026-07-20"), dia("2026-07-01")) != 0 || totalDias(nil, dia("2026-07-01")) != 0 {
		t.Fatal("fim anterior ou data ausente deve dar 0")
	}
}

func TestValidaPeriodo(t *testing.T) {
	casos := []struct {
		nome string
		p    Programacao
		erro string
	}{
		{"primeiro 30 dias", prog("P", "2026-07-01", "2026-07-30"), ""},
		{"primeiro 31 dias", prog("P", "2026-07-01", "2026-07-31"), "Total de dias maior que trinta."},
		{"segundo 10 dias", prog("S", "2026-12-01", "2026-12-10"), ""},
		{"segundo 9 dias", prog("S", "2026-12-01", "2026-12-09"), "TOTAL DE DIAS MENOR QUE DEZ."},
		{"segundo 21 dias", prog("S", "2026-12-01", "2026-12-21"), "Total de dias(21) inválido."},
		{"periodo desconhecido", prog("X", "2026-12-01", "2026-12-10"), "Total de dias(10) inválido."},
	}
	for _, c := range casos {
		err := validaPeriodo(c.p)
		if c.erro == "" && err != nil || c.erro != "" && (err == nil || err.Error() != c.erro) {
			t.Errorf("%s: got %v, want %q", c.nome, err, c.erro)
		}
	}
}

func TestServidorComumSegueRegraDeDias(t *testing.T) {
	repo := &fakeRepo{existentes: []Programacao{{ID: 1, Periodo: "P", DataInicio: dia("2026-07-01"), DataFim: dia("2026-07-20")}}}
	// segundo período de 15 dias + 20 existentes = 35 > 30
	_, err := novoServico(repo).Salvar(context.Background(), Chamador{Usuario: "u"}, prog("S", "2026-12-01", "2026-12-15"))
	if err == nil || err.Error() != "TOTAL DE DIAS MAIOR QUE 30." {
		t.Fatalf("got %v", err)
	}
	// segundo de 10 dias cabe (30) e é gravado com os dados do servidor
	salva, err := novoServico(repo).Salvar(context.Background(), Chamador{Usuario: "u"}, prog("S", "2026-12-01", "2026-12-10"))
	if err != nil || salva.ID != 99 || salva.NomeFuncionario != "Ana Souza" || salva.CodLotacao != 7 || salva.Matricula != 1234 {
		t.Fatalf("%+v %v", salva, err)
	}
	if salva.Datareg == nil || *salva.Datareg != "01/05/2026 12:00" {
		t.Fatalf("datareg %v", salva.Datareg)
	}
}

func TestSegundoNaoPodeComecarAntesDoPrimeiro(t *testing.T) {
	repo := &fakeRepo{existentes: []Programacao{{ID: 1, Periodo: "P", DataInicio: dia("2026-09-01"), DataFim: dia("2026-09-10")}}}
	_, err := novoServico(repo).Salvar(context.Background(), Chamador{}, prog("S", "2026-07-01", "2026-07-10"))
	if err == nil || !strings.Contains(err.Error(), "SEGUNDO PERÍODO NÃO PODE SER MENOR") {
		t.Fatalf("got %v", err)
	}
}

func TestEdicaoNaoConflitaComSiMesma(t *testing.T) {
	repo := &fakeRepo{existentes: []Programacao{{ID: 5, Periodo: "P", DataInicio: dia("2026-07-01"), DataFim: dia("2026-07-20")}}}
	p := prog("P", "2026-08-01", "2026-08-20")
	p.ID = 5
	if _, err := novoServico(repo).Salvar(context.Background(), Chamador{}, p); err != nil {
		t.Fatal(err)
	}
	if len(repo.atualizadas) != 1 || len(repo.inseridas) != 0 {
		t.Fatalf("deveria atualizar: %+v", repo)
	}
}

func TestMembroEAdminPulamRegraDeDias(t *testing.T) {
	repo := &fakeRepo{}
	for _, cx := range []Chamador{{Membro: true}, {Admin: true}} {
		if _, err := novoServico(repo).Salvar(context.Background(), cx, prog("P", "2026-07-01", "2026-08-30")); err != nil {
			t.Fatalf("%+v: %v", cx, err)
		}
	}
}

func TestAnoZeroUsaUltimoPeriodoAquisitivo(t *testing.T) {
	repo := &fakeRepo{ultimoAno: 2026}
	p := prog("P", "2026-07-01", "2026-07-10")
	p.Ano = 0
	salva, err := novoServico(repo).Salvar(context.Background(), Chamador{}, p)
	if err != nil || salva.Ano != 2026 {
		t.Fatalf("%+v %v", salva, err)
	}
}

func TestServidorNaoLocalizado(t *testing.T) {
	s := NewService(&fakeRepo{}, fakeFolha{})
	_, err := s.Salvar(context.Background(), Chamador{Admin: true}, prog("P", "2026-07-01", "2026-07-10"))
	var neg ErroNegocio
	if !errors.As(err, &neg) {
		t.Fatalf("got %v", err)
	}
}

func TestRemoverRespeitaPrazoDoPrimeiroPeriodo(t *testing.T) {
	paga := func(v string) *string { return &v }
	casos := []struct {
		nome   string
		p      Programacao
		remove bool
	}{
		{"P paga", Programacao{ID: 1, Periodo: "P", FeriasPaga: paga("S"), DataInicio: dia("2026-12-01")}, false},
		{"P não paga a 30 dias", Programacao{ID: 1, Periodo: "P", FeriasPaga: paga("N"), DataInicio: dia("2026-05-31")}, false},
		{"P não paga a 61 dias", Programacao{ID: 1, Periodo: "P", FeriasPaga: paga("N"), DataInicio: dia("2026-07-01")}, true},
		{"S sempre pode", Programacao{ID: 1, Periodo: "S", FeriasPaga: paga("S"), DataInicio: dia("2026-05-02")}, true},
	}
	for _, c := range casos {
		p := c.p
		repo := &fakeRepo{porID: map[int64]*Programacao{1: &p}}
		err := novoServico(repo).Remover(context.Background(), Chamador{}, 1)
		if c.remove && (err != nil || len(repo.excluidos) != 1 || len(repo.historico) != 1) {
			t.Errorf("%s: deveria remover e guardar histórico: %v", c.nome, err)
		}
		if !c.remove && (err == nil || len(repo.excluidos) != 0) {
			t.Errorf("%s: deveria bloquear", c.nome)
		}
	}
}

func TestAutorizacaoSoAntesDoInicio(t *testing.T) {
	repo := &fakeRepo{}
	s := novoServico(repo)
	futuro := prog("P", "2026-07-01", "2026-07-10")
	futuro.ID = 3
	if _, err := s.AtualizarAutorizacao(context.Background(), Chamador{}, futuro); err != nil || len(repo.atualizadas) != 1 {
		t.Fatalf("deveria gravar: %v", err)
	}
	passado := prog("P", "2026-04-01", "2026-04-10")
	passado.ID = 4
	salva, err := s.AtualizarAutorizacao(context.Background(), Chamador{}, passado)
	if err != nil || len(repo.atualizadas) != 1 || salva.ID != 4 {
		t.Fatalf("não deveria gravar: %v", err)
	}
}

type fakeDocs struct{ salvos map[string][]byte }

func (f *fakeDocs) Salvar(cpf, arquivo string, pdf []byte) error {
	if f.salvos == nil {
		f.salvos = map[string][]byte{}
	}
	f.salvos[cpf+arquivo] = pdf
	return nil
}

func progBanco() *Programacao {
	pg := "N"
	return &Programacao{ID: 8, Periodo: "P", CPF: "12345678909", Ano: 2026, NomeFuncionario: "Ana Souza", Matricula: 1234,
		DataInicio: dia("2026-07-01"), DataFim: dia("2026-07-10"), FeriasPaga: &pg, Autorizado: "S",
		DtAdmissao: admissao(time.Date(2011, 8, 1, 0, 0, 0, 0, time.UTC))}
}

func TestRegistrarPagamentoMarcaComoPagaEmiteComprovante(t *testing.T) {
	repo := &fakeRepo{porID: map[int64]*Programacao{8: progBanco()}, existentes: []Programacao{*progBanco()}}
	docs := &fakeDocs{}
	s := novoServico(repo)
	s.ComArmazenamento(docs)
	p := Programacao{ID: 8, Periodo: "P", CPF: "12345678909", Ano: 2026, DataPagamento: dia("2026-05-01"),
		Datas: &Datas{DataInicio: dia("2026-07-01"), DataFim: dia("2026-07-10")}}
	salva, ok, err := s.RegistrarPagamento(context.Background(), Chamador{}, p)
	if err != nil || !ok || salva.FeriasPaga == nil || *salva.FeriasPaga != "S" || salva.DataInicio.DiaBanco() != "2026-07-01" {
		t.Fatalf("%+v ok=%v err=%v", salva, ok, err)
	}
	pdf := docs.salvos["12345678909/AVISO_DE_FERIAS_2026.pdf"]
	if !strings.HasPrefix(string(pdf), "%PDF") {
		t.Fatalf("comprovante não gravado: %v", docs.salvos)
	}
	if len(repo.documentos) != 1 || repo.documentos[0] != "12345678909/AVISO_DE_FERIAS_2026.pdf" {
		t.Fatalf("documento não registrado: %v", repo.documentos)
	}
	_, ok, _ = s.RegistrarPagamento(context.Background(), Chamador{}, Programacao{ID: 8})
	if ok {
		t.Fatal("sem datas deve falhar")
	}
}

func TestObservacaoDeReversaoEmiteComprovanteEVoltaParaNaoPaga(t *testing.T) {
	repo := &fakeRepo{porID: map[int64]*Programacao{8: progBanco()}}
	docs := &fakeDocs{}
	s := novoServico(repo)
	s.ComArmazenamento(docs)
	id, err := s.SalvarObservacao(context.Background(), Chamador{Usuario: "rh"}, Observacao{ProgramacaoFeriasID: 8, Texto: "erro de data"})
	if err != nil || id != 55 || len(repo.revertidos) != 1 || repo.observacoes[0].Username != "rh" {
		t.Fatalf("id=%d err=%v %+v", id, err, repo)
	}
	if len(docs.salvos) != 1 || len(repo.documentos) != 1 || !strings.Contains(repo.documentos[0], "/INFORMACAO_PAGAMENTO_") {
		t.Fatalf("comprovante: %v %v", docs.salvos, repo.documentos)
	}
}

func TestObservacaoDeAlteracaoUsaUltimoHistoricoComoPeriodoAnterior(t *testing.T) {
	antigo := &DataHora{Momento{T: time.Date(2026, 6, 1, 3, 0, 0, 0, time.UTC)}}
	repo := &fakeRepo{porID: map[int64]*Programacao{8: progBanco()},
		historicos: []Historico{{IDProgramacaoFerias: 8, Ano: 2026, DataInicio: antigo, DataFim: antigo}}}
	s := novoServico(repo)
	d, arquivo, err := s.dadosObservacao(context.Background(), *progBanco(),
		Observacao{ExcecaoPeriodoFerias: "SIM", Texto: "x", DataInicio: dia("2026-08-03"), DataFim: dia("2026-08-12"), Ano: 2026})
	if err != nil || d.Tipo != ComprovanteAlteracao || fmtData(d.Anterior.Inicio) != "2026-06-01" ||
		fmtData(d.Novo.Inicio) != "2026-08-03" || !strings.HasPrefix(arquivo, "/INFORMACAO_PAGAMENTO_") {
		t.Fatalf("%+v %q %v", d, arquivo, err)
	}
}

func TestObservacaoSemProgramacaoFalha(t *testing.T) {
	_, err := novoServico(&fakeRepo{}).SalvarObservacao(context.Background(), Chamador{}, Observacao{ProgramacaoFeriasID: 1})
	if err == nil {
		t.Fatal("esperava erro")
	}
}

func TestArmazenamentoFSRejeitaCaminhosInseguros(t *testing.T) {
	a := ArmazenamentoFS{Dir: t.TempDir()}
	for _, c := range [][2]string{{"../x", "/a.pdf"}, {"12345678909", "/../a.pdf"}, {"12345678909", "a.pdf"}, {"123", "/a.pdf"}} {
		if err := a.Salvar(c[0], c[1], []byte("x")); err == nil {
			t.Errorf("deveria rejeitar %v", c)
		}
	}
	if err := a.Salvar("12345678909", "/AVISO_DE_FERIAS_2026.pdf", []byte("x")); err != nil {
		t.Fatal(err)
	}
}

func TestJSONDasDatas(t *testing.T) {
	adm := admissao(time.Date(2020, 3, 1, 0, 0, 0, 0, time.UTC))
	p := Programacao{ID: 1, DataInicio: novaData(time.Date(2026, 7, 1, 3, 0, 0, 0, time.UTC)), DtAdmissao: adm}
	b, _ := json.Marshal(p)
	for _, want := range []string{`"dataInicio":"2026-07-01"`, `"dtAdmissao":"2020-03-01T03:00:00.000+00:00"`, `"datas":null`, `"dataPagamento":null`} {
		if !strings.Contains(string(b), want) {
			t.Errorf("faltou %s em %s", want, b)
		}
	}
}

func TestEntradaDeDatasAceitaFormatosDoFrontend(t *testing.T) {
	var p Programacao
	corpo := `{"dataInicio":1782864000000,"dataFim":"2026-07-10","dataPagamento":"2026-05-01T03:00:00.000Z"}`
	if err := json.Unmarshal([]byte(corpo), &p); err != nil {
		t.Fatal(err)
	}
	if p.DataInicio.Wall().Format("2006-01-02") != "2026-07-01" || p.DataFim.DiaBanco() != "2026-07-10" ||
		p.DataPagamento.DiaBanco() != "2026-05-01" {
		t.Fatalf("%+v %+v %+v", p.DataInicio, p.DataFim, p.DataPagamento)
	}
}
