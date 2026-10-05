package ferias

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Chamador identifica quem executa a operação (token JWT validado).
type Chamador struct {
	Usuario string
	Admin   bool // ROLE_RH_AVISO_FERIAS
	Membro  bool // ROLE_MEMBROS
}

type Service struct {
	repo  Repository
	folha Folha
	docs  Armazenamento
	agora func() time.Time
}

// ComArmazenamento define onde os comprovantes em PDF são guardados.
func (s *Service) ComArmazenamento(a Armazenamento) { s.docs = a }

func NewService(repo Repository, folha Folha) *Service {
	return &Service{repo: repo, folha: folha, agora: time.Now}
}

func (s *Service) log(ctx context.Context, usuario string, operacao int, descricao string) {
	if err := s.repo.GravarLog(ctx, usuario, operacao, descricao); err != nil {
		log.Printf("log de auditoria de férias: %v", err)
	}
}

func (s *Service) dataReg() *string {
	v := s.agora().In(belem).Format("02/01/2006 15:04")
	return &v
}

// --- consultas ---

// PeriodosDoAno: ano 0 devolve a próxima programação; ano 1, todas do servidor; demais, as do ano.
func (s *Service) PeriodosDoAno(ctx context.Context, ano int, cpf string) ([]Programacao, error) {
	switch ano {
	case 0:
		return s.repo.ProximaProgramacao(ctx, cpf)
	case 1:
		return s.repo.ProgramacoesPorCPF(ctx, cpf)
	}
	return s.repo.ProgramacoesDoAno(ctx, ano, cpf)
}

func (s *Service) PeriodosAnteriores(ctx context.Context, cpf string) ([]Programacao, error) {
	lista, err := s.repo.PeriodosAnteriores(ctx, cpf)
	if err != nil {
		return nil, err
	}
	if len(lista) == 0 {
		return nil, ErroNegocio("PERÍODOS NÃO LOCALIZADOS")
	}
	return lista, nil
}

// PorAno devolve até 150 programações do ano, por nome decrescente.
func (s *Service) PorAno(ctx context.Context, ano int) ([]Programacao, error) {
	lista, err := s.repo.PorAno(ctx, ano)
	if err != nil {
		return nil, err
	}
	if len(lista) == 0 {
		return nil, ErroNegocio("PERÍODOS NÃO LOCALIZADOS")
	}
	if len(lista) > 150 {
		lista = lista[:150]
	}
	return lista, nil
}

// Consulta é a pesquisa do RH. O legado calcula um filtro por statusServidor (ativo/inativo na folha),
// mas descarta o resultado e devolve a lista sem ele; mantemos o mesmo comportamento.
func (s *Service) Consulta(ctx context.Context, f Parametros) ([]Programacao, error) {
	lista, err := s.repo.Consulta(ctx, f)
	if err != nil {
		return nil, err
	}
	if len(lista) == 0 {
		return nil, ErroNegocio("PROGRAMACAO NAO ENCONTRADA.")
	}
	return lista, nil
}

// Restantes lista os servidores ativos que ainda não têm programação.
func (s *Service) Restantes(ctx context.Context, f Parametros) ([]ServidorFolha, error) {
	var programados []Programacao
	var err error
	if f.Ano > 0 {
		programados, err = s.repo.PorAno(ctx, f.Ano)
	} else {
		programados, err = s.repo.ProgramacoesUltimoPeriodoAquisitivo(ctx)
	}
	if err != nil {
		return nil, err
	}
	ativos, err := s.folha.Restantes(ctx, f)
	if err != nil {
		return nil, err
	}
	feitos := make(map[string]bool, len(programados))
	for _, p := range programados {
		feitos[strings.TrimSpace(p.CPF)] = true
	}
	out := make([]ServidorFolha, 0, len(ativos))
	for _, a := range ativos {
		if a.CPF != nil && feitos[strings.TrimSpace(*a.CPF)] {
			continue
		}
		out = append(out, a)
	}
	return out, nil
}

// ServidoresDoChefe monta a tela de autorização: os servidores do chefe com suas programações no ano.
func (s *Service) ServidoresDoChefe(ctx context.Context, cpfChefe string, ano int) (AutorizarFerias, error) {
	dto := AutorizarFerias{ServidoresComProgramacao: []Programacao{}}
	vinculos, err := s.repo.VinculosDoChefe(ctx, cpfChefe)
	if err != nil {
		return dto, err
	}
	cpfs := make([]string, len(vinculos))
	for i, v := range vinculos {
		cpfs[i] = v.CPFFuncionario
	}
	progs, err := s.repo.ProgramacoesDeCPFs(ctx, cpfs, ano)
	if err != nil {
		return dto, err
	}
	porCPF := map[string][]Programacao{}
	for _, p := range progs {
		porCPF[strings.TrimSpace(p.CPF)] = append(porCPF[strings.TrimSpace(p.CPF)], p)
	}
	for _, v := range vinculos {
		nome, cpf := v.NomeChefe, v.CPFChefe
		dto.NomeChefe, dto.CPFChefe = &nome, &cpf
		if l := porCPF[strings.TrimSpace(v.CPFFuncionario)]; len(l) > 0 {
			dto.ServidoresComProgramacao = append(dto.ServidoresComProgramacao, l...)
		} else {
			vazio := ""
			dto.ServidoresComProgramacao = append(dto.ServidoresComProgramacao, Programacao{
				NomeFuncionario: v.NomeFuncionario, CPF: v.CPFFuncionario, Ano: ano, FeriasPaga: &vazio, Datareg: &vazio})
		}
	}
	return dto, nil
}

// --- gravação ---

func validarCorpo(p Programacao) error {
	if strings.TrimSpace(p.Periodo) == "" {
		return ErroNegocio("Período deve estar preenchido.")
	}
	if strings.TrimSpace(p.CPF) == "" {
		return ErroNegocio("CPF deve estar preenchido.")
	}
	if p.DataInicio == nil || p.DataFim == nil {
		return ErroNegocio("Data de início e data de fim devem estar preenchidas.")
	}
	return nil
}

// Salvar cria ou altera uma programação (ProgramacaoFeriasServiceImpl.save).
// O RH grava sem as regras de dias; os demais perfis passam por elas.
func (s *Service) Salvar(ctx context.Context, cx Chamador, p Programacao) (Programacao, error) {
	if err := validarCorpo(p); err != nil {
		return Programacao{}, err
	}
	servidor, err := s.folha.PorCPF(ctx, p.CPF)
	if err != nil {
		return Programacao{}, err
	}

	if cx.Admin {
		if p.ID > 0 {
			p.Datareg = s.dataReg()
			s.log(ctx, cx.Usuario, OperacaoEditando, "EDIT. PROGRAMACAO ADMIN: "+p.String())
			if err := s.salvarHistoricoAdm(ctx, cx, p.ID); err != nil {
				return Programacao{}, err
			}
			if err := s.registrarAlteracao(ctx, cx, p); err != nil {
				return Programacao{}, err
			}
			return p, s.repo.Atualizar(ctx, p)
		}
		if err := preencherServidor(&p, servidor); err != nil {
			return Programacao{}, err
		}
		p.Datareg = s.dataReg()
		s.log(ctx, cx.Usuario, OperacaoCadastrando, "PROGRAMACAO ADMIN: "+p.String())
		return s.gravar(ctx, cx, p)
	}

	if !cx.Membro {
		if err := s.regraDeDias(ctx, p); err != nil {
			return Programacao{}, err
		}
	}
	ano, err := s.anoDoPeriodoAquisitivo(ctx, p.Ano)
	if err != nil {
		return Programacao{}, err
	}
	p.Ano = ano
	if err := preencherServidor(&p, servidor); err != nil {
		return Programacao{}, err
	}
	p.Datareg = s.dataReg()
	s.log(ctx, cx.Usuario, OperacaoCadastrando, "PROGRAMACAO SERVIDORES: "+p.String())
	return s.gravar(ctx, cx, p)
}

func (s *Service) gravar(ctx context.Context, cx Chamador, p Programacao) (Programacao, error) {
	if err := s.registrarAlteracao(ctx, cx, p); err != nil {
		return Programacao{}, err
	}
	if p.ID > 0 {
		return p, s.repo.Atualizar(ctx, p)
	}
	id, err := s.repo.Inserir(ctx, p)
	p.ID = id
	return p, err
}

func preencherServidor(p *Programacao, servidor *ServidorFolha) error {
	if servidor == nil || servidor.CodLotacao == nil {
		return ErroNegocio("Servidor não localizado.")
	}
	if servidor.Nome != nil {
		p.NomeFuncionario = *servidor.Nome
	} else {
		p.NomeFuncionario = ""
	}
	p.Matricula = 0
	if servidor.Matricula != nil {
		p.Matricula = *servidor.Matricula
	}
	p.DtAdmissao = servidor.DtAdmissao
	p.CodLotacao = *servidor.CodLotacao
	return nil
}

func (s *Service) anoDoPeriodoAquisitivo(ctx context.Context, ano int) (int, error) {
	if ano != 0 {
		return ano, nil
	}
	ult, err := s.repo.UltimoPeriodoAquisitivo(ctx)
	if err != nil {
		return 0, err
	}
	if ult == nil {
		return 0, ErroNegocio("PERIODO AQUISITIVO NAO ENCONTRADO")
	}
	return ult.Ano, nil
}

// regraDeDias aplica as regras de períodos de férias dos servidores comuns.
func (s *Service) regraDeDias(ctx context.Context, p Programacao) error {
	existentes, err := s.repo.ProgramacoesDoAno(ctx, p.Ano, p.CPF)
	if err != nil {
		return err
	}
	dias := 0
	for _, e := range existentes {
		if p.ID > 0 && e.ID == p.ID { // a própria programação em edição não conta contra si
			continue
		}
		if err := verificaPeriodos(p, e); err != nil {
			return err
		}
		dias += e.totalDias()
	}
	if dias+p.totalDias() > 30 {
		return ErroNegocio("TOTAL DE DIAS MAIOR QUE 30.")
	}
	return validaPeriodo(p)
}

func verificaPeriodos(p, outra Programacao) error {
	if outra.DataInicio == nil {
		return nil
	}
	if p.Periodo == "P" && p.DataInicio.T.After(outra.DataInicio.T) {
		return ErroNegocio("PRIMEIRO PERÍODO NÃO PODE SER MAIOR QUE O SEGUNDO.")
	}
	if p.Periodo == "S" && p.DataInicio.T.Before(outra.DataInicio.T) {
		return ErroNegocio("SEGUNDO PERÍODO NÃO PODE SER MENOR QUE O PRIMEIRO.")
	}
	return nil
}

// validaPeriodo espelha PrimeiroPeriodo/SegundoPeriodo: primeiro período de 1 a 30 dias;
// segundo de 10 a 20 dias.
func validaPeriodo(p Programacao) error {
	dias := p.totalDias()
	switch p.Periodo {
	case "P":
		if dias < 1 || dias > 30 {
			return ErroNegocio("Total de dias maior que trinta.")
		}
		return nil
	case "S":
		if dias < 10 {
			return ErroNegocio("TOTAL DE DIAS MENOR QUE DEZ.")
		}
		if dias > 20 {
			return ErroNegocio(fmt.Sprintf("Total de dias(%d) inválido.", dias))
		}
		return nil
	}
	return ErroNegocio(fmt.Sprintf("Total de dias(%d) inválido.", dias))
}

func (s *Service) salvarHistoricoAdm(ctx context.Context, cx Chamador, id int64) error {
	atual, err := s.repo.PorID(ctx, id)
	if err != nil || atual == nil {
		return err
	}
	s.log(ctx, cx.Usuario, OperacaoCadastrando, "HISTORICO: "+atual.String())
	return s.repo.InserirHistorico(ctx, *atual)
}

// registrarAlteracao grava em alteracao_ferias a mudança em relação ao último histórico do ano.
func (s *Service) registrarAlteracao(ctx context.Context, cx Chamador, p Programacao) error {
	hist, err := s.repo.HistoricoPorCPF(ctx, p.CPF)
	if err != nil {
		return err
	}
	for _, h := range hist { // já ordenado por id decrescente
		if h.Ano != p.Ano {
			continue
		}
		dia := func(d *DataHora) *DataHora {
			if d == nil {
				return nil
			}
			return &DataHora{Momento{T: d.Data(), Dia: true}}
		}
		diaP := func(d *Data) *DataHora {
			if d == nil {
				return nil
			}
			return &DataHora{Momento{T: d.Data(), Dia: true}}
		}
		a := Alteracao{
			Nome: p.NomeFuncionario, Periodo: p.Periodo,
			DataAnteriorInicio: dia(h.DataInicio), DataAnteriorFim: dia(h.DataFim),
			DataNovaInicio: diaP(p.DataInicio), DataNovaFim: diaP(p.DataFim),
			DarCiencia: "N", Ano: p.Ano, CPF: p.CPF, Username: cx.Usuario,
		}
		s.log(ctx, cx.Usuario, OperacaoCadastrando, fmt.Sprintf("PROGRAMACAO ADMIN: ALTERACAO %s %s", p.NomeFuncionario, p.Periodo))
		_, err := s.repo.InserirAlteracao(ctx, a)
		return err
	}
	return nil
}

// Remover guarda a programação no histórico e a exclui, respeitando o prazo do primeiro período.
func (s *Service) Remover(ctx context.Context, cx Chamador, id int64) error {
	p, err := s.repo.PorID(ctx, id)
	if err != nil || p == nil {
		return err
	}
	paga := ""
	if p.FeriasPaga != nil {
		paga = *p.FeriasPaga
	}
	if p.Periodo == "P" && paga == "S" {
		return ErroNegocio("NÃO É POSSÍVEL ALTERAR O PRIMEIRO PERÍODO.")
	}
	if p.Periodo == "P" && paga == "N" && p.DataInicio != nil {
		y, m, d := s.agora().In(belem).Date()
		hoje := time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
		if dias := p.DataInicio.Data().Sub(hoje).Hours() / 24; dias < 50 {
			return ErroNegocio("NÃO É POSSÍVEL ALTERAR O PRIMEIRO PERÍODO.")
		}
	}
	s.log(ctx, cx.Usuario, OperacaoCadastrando, "HISTORICO: "+p.String())
	if err := s.repo.InserirHistorico(ctx, *p); err != nil {
		return err
	}
	s.log(ctx, cx.Usuario, OperacaoExcluindo, p.String())
	return s.repo.Excluir(ctx, id)
}

// AtualizarAutorizacao grava a autorização do chefe enquanto o período ainda não começou.
// Se já começou, nada é gravado e devolve apenas o id (como o legado).
func (s *Service) AtualizarAutorizacao(ctx context.Context, cx Chamador, p Programacao) (Programacao, error) {
	if p.DataInicio == nil {
		return Programacao{}, ErroNegocio("Data de início deve estar preenchida.")
	}
	if p.DataInicio.T.Before(s.agora()) {
		return Programacao{ID: p.ID}, nil
	}
	s.log(ctx, cx.Usuario, OperacaoCadastrando, "AUTORIZACAO DE FERIAS: "+p.String())
	return p, s.repo.Atualizar(ctx, p)
}

// Atualizar grava a programação como veio (PUT /programacao/atualizar do legado: save direto, só RH).
func (s *Service) Atualizar(ctx context.Context, cx Chamador, p Programacao) (Programacao, error) {
	return s.Salvar(ctx, cx, p)
}

// DesfazerPagamento grava a programação com os dados de pagamento revertidos pelo RH.
func (s *Service) DesfazerPagamento(ctx context.Context, cx Chamador, p Programacao) (Programacao, error) {
	s.log(ctx, cx.Usuario, OperacaoEditando, "DESFAZER PAGAMENTO: "+p.String())
	return p, s.repo.Atualizar(ctx, p)
}

// RegistrarPagamento marca as férias como pagas e emite o aviso de férias em PDF.
// ok=false equivale ao id 0 do legado (400).
func (s *Service) RegistrarPagamento(ctx context.Context, cx Chamador, p Programacao) (Programacao, bool, error) {
	if p.Datas != nil {
		p.DataInicio, p.DataFim = p.Datas.DataInicio, p.Datas.DataFim
	} else {
		p.DataInicio, p.DataFim = nil, nil
	}
	if p.DataInicio == nil || p.DataFim == nil || p.ID == 0 {
		return Programacao{}, false, nil
	}
	paga := "S"
	p.FeriasPaga = &paga

	atual, err := s.repo.PorID(ctx, p.ID)
	if err != nil || atual == nil {
		return Programacao{}, false, err
	}
	// o comprovante usa os dados cadastrais do banco e as datas/pagamento informados
	emitido := *atual
	emitido.DataInicio, emitido.DataFim, emitido.DataPagamento, emitido.FeriasPaga = p.DataInicio, p.DataFim, p.DataPagamento, p.FeriasPaga
	dados, err := s.dadosAviso(ctx, emitido)
	if err != nil {
		return Programacao{}, false, err
	}
	arquivo := fmt.Sprintf("/AVISO_DE_FERIAS_%d.pdf", s.agora().In(belem).Year())
	if err := s.emitir(ctx, dados, atual.CPF, atual.NomeFuncionario, arquivo); err != nil {
		return Programacao{}, false, err
	}

	s.log(ctx, cx.Usuario, OperacaoCadastrando, "REALIZAR PAGAMENTO: "+p.String())
	if err := s.repo.Atualizar(ctx, p); err != nil {
		return Programacao{}, false, err
	}
	s.registrarDocumento(ctx, atual.NomeFuncionario, atual.CPF, arquivo)
	return p, true, nil
}

// ComprovanteDoAviso gera sob demanda o aviso de férias da programação (sem gravar em disco).
func (s *Service) ComprovanteDoAviso(ctx context.Context, id int64) ([]byte, string, error) {
	p, err := s.repo.PorID(ctx, id)
	if err != nil {
		return nil, "", err
	}
	if p == nil {
		return nil, "", ErroNegocio("Programação não encontrada.")
	}
	dados, err := s.dadosAviso(ctx, *p)
	if err != nil {
		return nil, "", err
	}
	pdf, err := GerarComprovante(dados)
	return pdf, fmt.Sprintf("AVISO_DE_FERIAS_%d.pdf", p.Ano), err
}

func rotuloPeriodo(p string) string {
	if p == "P" {
		return "1º período"
	}
	return "2º período"
}

func (s *Service) lotacaoDe(ctx context.Context, cpf string) string {
	srv, err := s.folha.PorCPF(ctx, cpf)
	if err != nil || srv == nil || srv.Lotacao == nil {
		return ""
	}
	return *srv.Lotacao
}

// dadosAviso monta o aviso com todos os períodos do exercício; o período informado substitui o gravado.
func (s *Service) dadosAviso(ctx context.Context, p Programacao) (DadosComprovante, error) {
	todos, err := s.repo.ProgramacoesDoAno(ctx, p.Ano, p.CPF)
	if err != nil {
		return DadosComprovante{}, err
	}
	achou := false
	for i := range todos {
		if todos[i].ID == p.ID {
			todos[i], achou = p, true
		}
	}
	if !achou {
		todos = append(todos, p)
	}
	sort.SliceStable(todos, func(i, j int) bool { return todos[i].Periodo < todos[j].Periodo })

	d := DadosComprovante{
		Tipo: ComprovanteAviso, Servidor: p.NomeFuncionario, Matricula: p.Matricula, CPF: p.CPF,
		Lotacao: s.lotacaoDe(ctx, p.CPF), Admissao: p.DtAdmissao, Exercicio: p.Ano,
		AbonoPecuniario: p.AbonoPecuniario, AnteciparDecimo: p.AnteciparDecimo, Autorizado: p.Autorizado,
		Emissao: s.agora(),
	}
	for _, t := range todos {
		situacao := "Programada"
		if t.FeriasPaga != nil && *t.FeriasPaga == "S" && t.DataPagamento != nil {
			situacao = "Paga em " + dataBR(t.DataPagamento)
		} else if t.FeriasPaga != nil && *t.FeriasPaga == "S" {
			situacao = "Paga"
		}
		d.Periodos = append(d.Periodos, PeriodoComprovante{
			Rotulo: rotuloPeriodo(t.Periodo), Inicio: t.DataInicio, Fim: t.DataFim, Situacao: situacao})
	}
	return d, nil
}

// emitir gera o PDF e o guarda (se houver armazenamento configurado).
func (s *Service) emitir(ctx context.Context, d DadosComprovante, cpf, nome, arquivo string) error {
	pdf, err := GerarComprovante(d)
	if err != nil {
		return fmt.Errorf("gerar comprovante: %w", err)
	}
	if s.docs == nil {
		return nil
	}
	return s.docs.Salvar(strings.TrimSpace(cpf), arquivo, pdf)
}

// registrarDocumento coloca o comprovante na área do servidor. Só faz sentido se o arquivo foi gravado.
func (s *Service) registrarDocumento(ctx context.Context, nome, cpf, arquivo string) {
	if s.docs == nil {
		return
	}
	if err := s.repo.InserirDocumento(ctx, nome, strings.TrimSpace(cpf), arquivo, s.agora()); err != nil {
		log.Printf("registro do documento %s: %v", arquivo, err)
	}
}

func (s *Service) Bloquear(ctx context.Context, cx Chamador, lista []Programacao, valor int) error {
	for _, p := range lista {
		s.log(ctx, cx.Usuario, OperacaoCadastrando, fmt.Sprintf("BLOQUEAR/DESBLOQUEAR: %s (atualizada=%d)", p.String(), valor))
		if err := s.repo.DefinirAtualizada(ctx, p.ID, valor); err != nil {
			return err
		}
	}
	return nil
}

// --- observações ---

// SalvarObservacao emite o comprovante (alteração de período ou reversão de pagamento), registra a
// observação e volta a programação para "não paga".
func (s *Service) SalvarObservacao(ctx context.Context, cx Chamador, o Observacao) (int64, error) {
	if o.ProgramacaoFeriasID <= 0 {
		return 0, ErroNegocio("Programação não informada.")
	}
	p, err := s.repo.PorID(ctx, o.ProgramacaoFeriasID)
	if err != nil {
		return 0, err
	}
	if p == nil {
		return 0, ErroNegocio("Programação não encontrada.")
	}
	if strings.TrimSpace(o.Username) == "" {
		o.Username = cx.Usuario
	}

	dados, arquivo, err := s.dadosObservacao(ctx, *p, o)
	if err != nil {
		return 0, err
	}
	if err := s.emitir(ctx, dados, p.CPF, p.NomeFuncionario, arquivo); err != nil {
		return 0, err
	}
	s.registrarDocumento(ctx, p.NomeFuncionario, p.CPF, arquivo)

	s.log(ctx, cx.Usuario, OperacaoCadastrando, fmt.Sprintf("OBSERVACAO: programacao=%d texto=%q excecao=%q", o.ProgramacaoFeriasID, o.Texto, o.ExcecaoPeriodoFerias))
	id, err := s.repo.InserirObservacao(ctx, o)
	if err != nil {
		return 0, err
	}
	return id, s.repo.ReverterPagamento(ctx, o.ProgramacaoFeriasID)
}

func (s *Service) dadosObservacao(ctx context.Context, p Programacao, o Observacao) (DadosComprovante, string, error) {
	nome, err := nomeAleatorio()
	if err != nil {
		return DadosComprovante{}, "", err
	}
	arquivo := "/INFORMACAO_PAGAMENTO_" + nome + ".pdf"
	d := DadosComprovante{
		Servidor: p.NomeFuncionario, Matricula: p.Matricula, CPF: p.CPF, Lotacao: s.lotacaoDe(ctx, p.CPF),
		Admissao: p.DtAdmissao, Exercicio: p.Ano, Observacao: o.Texto, Emissao: s.agora(),
	}

	if strings.EqualFold(strings.TrimSpace(o.ExcecaoPeriodoFerias), "SIM") {
		// o período anterior é o último histórico desta programação (a edição já foi gravada)
		anterior := PeriodoComprovante{Rotulo: "Período anterior", Inicio: p.DataInicio, Fim: p.DataFim, Situacao: strconv.Itoa(p.Ano)}
		hist, err := s.repo.HistoricoPorCPF(ctx, p.CPF)
		if err != nil {
			return d, "", err
		}
		for _, h := range hist {
			if h.IDProgramacaoFerias == p.ID {
				anterior.Inicio, anterior.Fim, anterior.Situacao = diaDe(h.DataInicio), diaDe(h.DataFim), strconv.Itoa(h.Ano)
				break
			}
		}
		novo := PeriodoComprovante{Rotulo: "Novo período", Inicio: o.DataInicio, Fim: o.DataFim, Situacao: strconv.Itoa(o.Ano)}
		if novo.Inicio == nil || novo.Fim == nil {
			novo.Inicio, novo.Fim = p.DataInicio, p.DataFim
		}
		if o.Ano == 0 {
			novo.Situacao = strconv.Itoa(p.Ano)
		} else {
			d.Exercicio = o.Ano
		}
		d.Tipo, d.Anterior, d.Novo = ComprovanteAlteracao, &anterior, &novo
		return d, arquivo, nil
	}

	d.Tipo = ComprovanteReversao
	d.Periodos = []PeriodoComprovante{{Rotulo: rotuloPeriodo(p.Periodo), Inicio: p.DataInicio, Fim: p.DataFim, Situacao: "Pagamento revertido"}}
	return d, arquivo, nil
}

func diaDe(d *DataHora) *Data {
	if d == nil {
		return nil
	}
	return &Data{Momento{T: d.Data(), Dia: true}}
}

func nomeAleatorio() (string, error) {
	b := make([]byte, 6)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// --- período aquisitivo ---

func (s *Service) PeriodoDoAno(ctx context.Context, ano int) ([]PeriodoAquisitivo, error) {
	todos, err := s.repo.PeriodosAquisitivos(ctx)
	if err != nil {
		return nil, err
	}
	out := []PeriodoAquisitivo{}
	for _, p := range todos {
		if p.Ano == ano {
			out = append(out, p)
		}
	}
	if len(out) == 0 {
		return nil, ErroNegocio("PERIODO AQUISITIVO NAO ENCONTRADO")
	}
	return out, nil
}

func (s *Service) SalvarPeriodoAquisitivo(ctx context.Context, cx Chamador, p PeriodoAquisitivo) error {
	if p.Ano <= 0 {
		return ErroNegocio("Ano deve estar preenchido.")
	}
	s.log(ctx, cx.Usuario, OperacaoCadastrando, fmt.Sprintf("PeriodoAquisitivo(ano=%d)", p.Ano))
	return s.repo.SalvarPeriodoAquisitivo(ctx, p)
}

// --- alteração de férias ---

func (s *Service) NovaAlteracao(ctx context.Context, cx Chamador, p Programacao) error {
	if err := validarCorpo(p); err != nil {
		return err
	}
	return s.registrarAlteracao(ctx, cx, p)
}
