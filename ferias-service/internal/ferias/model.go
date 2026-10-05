// Package ferias reimplementa o módulo de férias do backend Spring Boot (S3i).
package ferias

import "fmt"

// Programacao é a entidade ProgramacaoFerias (tabela programacao_ferias).
type Programacao struct {
	ID              int64     `json:"id"`
	Periodo         string    `json:"periodo"`
	DataInicio      *Data     `json:"dataInicio"`
	DataFim         *Data     `json:"dataFim"`
	FeriasPaga      *string   `json:"feriasPaga"`
	DataPagamento   *Data     `json:"dataPagamento"`
	NomeFuncionario string    `json:"nomeFuncionario"`
	CPF             string    `json:"cpf"`
	Ano             int       `json:"ano"`
	Autorizado      string    `json:"autorizado"`
	CodLotacao      int       `json:"codLotacao"`
	Matricula       int       `json:"matricula"`
	DtAdmissao      *Instante `json:"dtAdmissao"`
	Atualizada      int       `json:"atualizada"`
	Membro          string    `json:"membro"`
	AbonoPecuniario string    `json:"abonoPecuniario"`
	AnteciparDecimo string    `json:"anteciparDecimo"`
	Datareg         *string   `json:"datareg"`
	Lotacao         string    `json:"lotacao"`
	Datas           *Datas    `json:"datas"`
}

// Datas é o ProgramacaoFeriasDatas usado no corpo do pagamento.
type Datas struct {
	DataInicio *Data `json:"dataInicio"`
	DataFim    *Data `json:"dataFim"`
}

func (p Programacao) totalDias() int { return totalDias(p.DataInicio, p.DataFim) }

func (p Programacao) String() string {
	return fmt.Sprintf("ProgramacaoFerias(id=%d, periodo='%s', dataInicio=%s, dataFim=%s, feriasPaga=%s, "+
		"dataPagamento=%s, nomeFuncionario='%s', cpf='%s', ano=%d, autorizado='%s', codLotacao=%d, matricula=%d, "+
		"atualizada=%d, membro='%s', abonoPecuniario='%s', anteciparDecimo='%s', lotacao='%s')",
		p.ID, p.Periodo, fmtData(p.DataInicio), fmtData(p.DataFim), strOuNull(p.FeriasPaga), fmtData(p.DataPagamento),
		p.NomeFuncionario, p.CPF, p.Ano, p.Autorizado, p.CodLotacao, p.Matricula, p.Atualizada, p.Membro,
		p.AbonoPecuniario, p.AnteciparDecimo, p.Lotacao)
}

func fmtData(d *Data) string {
	if d == nil {
		return "null"
	}
	return d.T.UTC().Format("2006-01-02")
}

func strOuNull(s *string) string {
	if s == nil {
		return "null"
	}
	return *s
}

// Historico é a entidade ProgramacaoFeriasHistorico (programacao_ferias_historico).
type Historico struct {
	ID                  int64     `json:"id"`
	IDProgramacaoFerias int64     `json:"idProgramacaoFerias"`
	Periodo             string    `json:"periodo"`
	DataInicio          *DataHora `json:"dataInicio"`
	DataFim             *DataHora `json:"dataFim"`
	FeriasPaga          *string   `json:"feriasPaga"`
	DataPagamento       *DataHora `json:"dataPagamento"`
	NomeFuncionario     string    `json:"nomeFuncionario"`
	CPF                 string    `json:"cpf"`
	Ano                 int       `json:"ano"`
	Autorizado          string    `json:"autorizado"`
	CodLotacao          int       `json:"codLotacao"`
	Matricula           int       `json:"matricula"`
	DtAdmissao          *Instante `json:"dtAdmissao"`
	Atualizada          int       `json:"atualizada"`
	Membro              string    `json:"membro"`
	AbonoPecuniario     string    `json:"abonoPecuniario"`
	AnteciparDecimo     string    `json:"anteciparDecimo"`
	Locacao             string    `json:"locacao"`
	Datareg             string    `json:"datareg"`
}

// Observacao é a entidade ProgramacaoFeriasObservacao (programacao_ferias_observacao).
type Observacao struct {
	ID                   int64     `json:"id"`
	ProgramacaoFeriasID  int64     `json:"programacaoFeriasId"`
	Texto                string    `json:"texto"`
	Datareg              *Instante `json:"datareg"`
	Username             string    `json:"username"`
	ExcecaoPeriodoFerias string    `json:"excecaoPeriodoFerias"`
	CPFServidor          string    `json:"cpfServidor"`
	NomeServidor         string    `json:"nomeServidor"`
	DataInicio           *Data     `json:"dataInicio"`
	DataFim              *Data     `json:"dataFim"`
	Ano                  int       `json:"ano"`
}

// Alteracao é a entidade AlteracaoFerias (alteracao_ferias).
type Alteracao struct {
	ID                 int64     `json:"id"`
	Datareg            string    `json:"datareg"` // yyyy-MM-dd HH:mm
	Nome               string    `json:"nome"`
	Periodo            string    `json:"periodo"`
	DataAnteriorInicio *DataHora `json:"dataAnteriorInicio"`
	DataAnteriorFim    *DataHora `json:"dataAnteriorFim"`
	DataNovaInicio     *DataHora `json:"dataNovaInicio"`
	DataNovaFim        *DataHora `json:"dataNovaFim"`
	DarCiencia         string    `json:"darCiencia"`
	Ano                int       `json:"ano"`
	CPF                string    `json:"cpf"`
	Username           string    `json:"username"`
}

// PeriodoAquisitivo é a entidade PeriodoAquisitivo (periodo_aquisitivo).
type PeriodoAquisitivo struct {
	Ano               int       `json:"ano"`
	PrazoInicial      *DataHora `json:"prazoInicial"`
	PrazoFinal        *DataHora `json:"prazoFinal"`
	CalendarioInicial *DataHora `json:"calendarioInicial"`
	CalendarioFinal   *DataHora `json:"calendarioFinal"`
}

// Parametros é o ProgramacaoParameters da consulta do RH.
type Parametros struct {
	Periodo         string `json:"periodo"`
	Mes             int    `json:"mes"`
	Ano             int    `json:"ano"`
	CPF             string `json:"cpf"`
	Lotacao         int    `json:"lotacao"`
	AbonoPecuniario string `json:"abonoPecuniario"`
	AnteciparDecimo string `json:"anteciparDecimo"`
	FeriasPaga      string `json:"feriasPaga"`
	TipoServidor    string `json:"tipoServidor"`
	StatusServidor  string `json:"statusServidor"`
}

// QuantidadePorMes é o QuantidadeDeServidoresDeFeriasDTO do gráfico.
type QuantidadePorMes struct {
	Periodo    string `json:"periodo"`
	Mes        int    `json:"mes"`
	Quantidade int    `json:"quantidade"`
	Ano        int    `json:"ano"`
}

// AutorizarFerias é o AutorizarFeriasDTO (nomes de propriedade como no Jackson do legado).
type AutorizarFerias struct {
	NomeChefe                *string       `json:"nome_chefe"`
	CPFChefe                 *string       `json:"cpf_chefe"`
	NomeFuncionario          *string       `json:"nome_funcionario"`
	CPFFuncionario           *string       `json:"cpf_funcionario"`
	TemProgramacao           *bool         `json:"temProgramacao"`
	ServidoresComProgramacao []Programacao `json:"servidoresComProgramacao"`
}

// ServidorFolha é o recorte do cadastro da folha (view devops_servidor) usado por férias.
// Campos nulos são omitidos, como no DevopsServidor (JsonInclude NON_NULL).
type ServidorFolha struct {
	CPF              *string   `json:"cpf,omitempty"`
	Nome             *string   `json:"nome,omitempty"`
	Matricula        *int      `json:"matricula,omitempty"`
	CGM              *int      `json:"cgm,omitempty"`
	DtAdmissao       *Instante `json:"dtAdmissao,omitempty"`
	CodCargo         *int      `json:"codCargo,omitempty"`
	Cargo            *string   `json:"cargo,omitempty"`
	CodFuncao        *string   `json:"codFuncao,omitempty"`
	Funcao           *string   `json:"funcao,omitempty"`
	Servidor         *string   `json:"servidor,omitempty"`
	CodLotacao       *int      `json:"codLotacao,omitempty"`
	Lotacao          *string   `json:"lotacao,omitempty"`
	Classe           *string   `json:"classe,omitempty"`
	Regime           *string   `json:"regime,omitempty"`
	Email            *string   `json:"email,omitempty"`
	Login            *string   `json:"login,omitempty"`
	Quadro           *string   `json:"quadro,omitempty"`
	Ativo            *string   `json:"ativo,omitempty"`
	MatriculaSistema *int      `json:"matriculaSistema,omitempty"`
}

// Erros de regra de negócio: o Spring os mapeia para 400 com HttpResponse.
type ErroNegocio string

func (e ErroNegocio) Error() string { return string(e) }
