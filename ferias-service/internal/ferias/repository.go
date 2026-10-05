package ferias

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
)

// Operações de log (OperacaoLogEnum do legado).
const (
	OperacaoCadastrando = 1
	OperacaoEditando    = 2
	OperacaoExcluindo   = 3
)

// VinculoChefe é uma linha de chefe_funcionarios usada na tela de autorização.
type VinculoChefe struct {
	NomeChefe, CPFChefe, CPFFuncionario, NomeFuncionario string
}

// Repository acessa o MySQL internet_novo.
type Repository interface {
	ProgramacoesDoAno(ctx context.Context, ano int, cpf string) ([]Programacao, error)
	ProximaProgramacao(ctx context.Context, cpf string) ([]Programacao, error)
	ProgramacoesPorCPF(ctx context.Context, cpf string) ([]Programacao, error)
	PeriodosAnteriores(ctx context.Context, cpf string) ([]Programacao, error)
	PorAno(ctx context.Context, ano int) ([]Programacao, error)
	PorID(ctx context.Context, id int64) (*Programacao, error)
	Inserir(ctx context.Context, p Programacao) (int64, error)
	Atualizar(ctx context.Context, p Programacao) error
	Excluir(ctx context.Context, id int64) error
	Consulta(ctx context.Context, f Parametros) ([]Programacao, error)
	Chart(ctx context.Context, ano, mes int, periodo string) ([]Programacao, error)
	QuantidadePorMes(ctx context.Context, ano int) ([]QuantidadePorMes, error)
	ProgramacoesUltimoPeriodoAquisitivo(ctx context.Context) ([]Programacao, error)
	DefinirAtualizada(ctx context.Context, id int64, valor int) error
	ReverterPagamento(ctx context.Context, id int64) error

	InserirHistorico(ctx context.Context, p Programacao) error
	HistoricoPorCPF(ctx context.Context, cpf string) ([]Historico, error)

	VinculosDoChefe(ctx context.Context, cpfChefe string) ([]VinculoChefe, error)
	ProgramacoesDeCPFs(ctx context.Context, cpfs []string, ano int) ([]Programacao, error)

	PeriodosAquisitivos(ctx context.Context) ([]PeriodoAquisitivo, error)
	UltimoPeriodoAquisitivo(ctx context.Context) (*PeriodoAquisitivo, error)
	SalvarPeriodoAquisitivo(ctx context.Context, p PeriodoAquisitivo) error

	Observacoes(ctx context.Context) ([]Observacao, error)
	ObservacaoPorID(ctx context.Context, id int64) (*Observacao, error)
	ObservacoesDoPeriodo(ctx context.Context, idProgramacao int64) ([]Observacao, error)
	InserirObservacao(ctx context.Context, o Observacao) (int64, error)
	RemoverObservacao(ctx context.Context, id int64) error

	Alteracoes(ctx context.Context) ([]Alteracao, error)
	InserirAlteracao(ctx context.Context, a Alteracao) (int64, error)
	AtualizarAlteracao(ctx context.Context, a Alteracao) error

	// InserirDocumento registra o comprovante na área do servidor (tabela aviso_ferias, tipo 1 = aviso de férias).
	InserirDocumento(ctx context.Context, nome, cpf, arquivo string, quando time.Time) error

	GravarLog(ctx context.Context, username string, operacao int, descricao string) error
}

type SQLRepository struct{ db *sql.DB }

func NewSQLRepository(db *sql.DB) *SQLRepository { return &SQLRepository{db: db} }

type rowScanner interface{ Scan(dest ...any) error }

const colunasProgramacao = `id, periodo, data_inicio, data_fim, ferias_paga, data_pagamento,
	COALESCE(nome_funcionario,''), COALESCE(cpf,''), ano, COALESCE(autorizado,''), cod_lotacao, matricula,
	dt_admissao, COALESCE(atualizada,0), COALESCE(membro,''), COALESCE(abono_pecuniario,''),
	COALESCE(antecipar_decimo,''), datareg`

func scanProgramacao(r rowScanner) (Programacao, error) {
	var p Programacao
	var ini, fim, pag, adm sql.NullTime
	var paga, reg sql.NullString
	if err := r.Scan(&p.ID, &p.Periodo, &ini, &fim, &paga, &pag, &p.NomeFuncionario, &p.CPF, &p.Ano,
		&p.Autorizado, &p.CodLotacao, &p.Matricula, &adm, &p.Atualizada, &p.Membro, &p.AbonoPecuniario,
		&p.AnteciparDecimo, &reg); err != nil {
		return p, err
	}
	if ini.Valid {
		p.DataInicio = novaData(ini.Time)
	}
	if fim.Valid {
		p.DataFim = novaData(fim.Time)
	}
	if pag.Valid { // coluna date: guarda a data exata, sem conversão de fuso ao regravar
		p.DataPagamento = &Data{Momento{T: pag.Time.UTC(), Dia: true}}
	}
	if adm.Valid {
		p.DtAdmissao = admissao(adm.Time)
	}
	if paga.Valid {
		p.FeriasPaga = &paga.String
	}
	if reg.Valid {
		p.Datareg = &reg.String
	}
	return p, nil
}

func (r *SQLRepository) programacoes(ctx context.Context, query string, args ...any) ([]Programacao, error) {
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Programacao{}
	for rows.Next() {
		p, err := scanProgramacao(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (r *SQLRepository) ProgramacoesDoAno(ctx context.Context, ano int, cpf string) ([]Programacao, error) {
	return r.programacoes(ctx, `SELECT `+colunasProgramacao+` FROM programacao_ferias WHERE ano = ? AND cpf = ? ORDER BY periodo`, ano, cpf)
}

func (r *SQLRepository) ProximaProgramacao(ctx context.Context, cpf string) ([]Programacao, error) {
	return r.programacoes(ctx, `SELECT `+colunasProgramacao+` FROM programacao_ferias WHERE cpf = ? AND data_inicio > CURDATE() ORDER BY data_inicio LIMIT 1`, cpf)
}

func (r *SQLRepository) ProgramacoesPorCPF(ctx context.Context, cpf string) ([]Programacao, error) {
	return r.programacoes(ctx, `SELECT `+colunasProgramacao+` FROM programacao_ferias WHERE cpf = ? ORDER BY periodo`, cpf)
}

func (r *SQLRepository) PeriodosAnteriores(ctx context.Context, cpf string) ([]Programacao, error) {
	return r.programacoes(ctx, `SELECT `+colunasProgramacao+` FROM programacao_ferias WHERE cpf = ? ORDER BY ano DESC`, cpf)
}

func (r *SQLRepository) PorAno(ctx context.Context, ano int) ([]Programacao, error) {
	return r.programacoes(ctx, `SELECT `+colunasProgramacao+` FROM programacao_ferias WHERE ano = ? ORDER BY nome_funcionario DESC`, ano)
}

func (r *SQLRepository) PorID(ctx context.Context, id int64) (*Programacao, error) {
	p, err := scanProgramacao(r.db.QueryRowContext(ctx, `SELECT `+colunasProgramacao+` FROM programacao_ferias WHERE id = ?`, id))
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &p, nil
}

func wallDataHora(d *Data) any {
	if d == nil {
		return nil
	}
	return d.Wall().Format("2006-01-02 15:04:05")
}

func diaData(d *Data) any {
	if d == nil {
		return nil
	}
	return d.DiaBanco()
}

func valoresProgramacao(p Programacao) []any {
	var adm any
	if p.DtAdmissao != nil {
		adm = p.DtAdmissao.DiaBanco()
	}
	return []any{p.Periodo, wallDataHora(p.DataInicio), wallDataHora(p.DataFim), p.FeriasPaga, diaData(p.DataPagamento),
		p.NomeFuncionario, p.CPF, p.Ano, p.Autorizado, p.CodLotacao, p.Matricula, adm, p.Atualizada, p.Membro,
		p.AbonoPecuniario, p.AnteciparDecimo, p.Datareg}
}

func (r *SQLRepository) Inserir(ctx context.Context, p Programacao) (int64, error) {
	res, err := r.db.ExecContext(ctx, `INSERT INTO programacao_ferias
		(periodo, data_inicio, data_fim, ferias_paga, data_pagamento, nome_funcionario, cpf, ano, autorizado,
		 cod_lotacao, matricula, dt_admissao, atualizada, membro, abono_pecuniario, antecipar_decimo, datareg)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`, valoresProgramacao(p)...)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (r *SQLRepository) Atualizar(ctx context.Context, p Programacao) error {
	args := append(valoresProgramacao(p), p.ID)
	_, err := r.db.ExecContext(ctx, `UPDATE programacao_ferias SET periodo=?, data_inicio=?, data_fim=?, ferias_paga=?,
		data_pagamento=?, nome_funcionario=?, cpf=?, ano=?, autorizado=?, cod_lotacao=?, matricula=?, dt_admissao=?,
		atualizada=?, membro=?, abono_pecuniario=?, antecipar_decimo=?, datareg=? WHERE id=?`, args...)
	return err
}

func (r *SQLRepository) Excluir(ctx context.Context, id int64) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM programacao_ferias WHERE id = ?`, id)
	return err
}

// Consulta monta o filtro do RH (RelatorioQueryService) com parâmetros vinculados.
func (r *SQLRepository) Consulta(ctx context.Context, f Parametros) ([]Programacao, error) {
	where := []string{"id > 0"}
	var args []any
	add := func(cond string, v any) { where = append(where, cond); args = append(args, v) }
	if f.TipoServidor != "" {
		add("membro = ?", f.TipoServidor)
	}
	if f.Periodo != "" {
		add("periodo = ?", f.Periodo)
	}
	if f.Mes > 0 {
		add("MONTH(data_inicio) = ?", f.Mes)
	}
	if f.Ano > 0 {
		add("ano = ?", f.Ano)
	}
	if f.AbonoPecuniario != "" {
		add("abono_pecuniario = ?", f.AbonoPecuniario)
	}
	if f.AnteciparDecimo != "" {
		add("antecipar_decimo = ?", f.AnteciparDecimo)
	}
	if f.FeriasPaga != "" {
		add("ferias_paga = ?", f.FeriasPaga)
	}
	if f.CPF != "" {
		add("cpf = ?", f.CPF)
	}
	if f.Lotacao > 0 {
		add("cod_lotacao = ?", f.Lotacao)
	}
	return r.programacoes(ctx, `SELECT `+colunasProgramacao+` FROM programacao_ferias WHERE `+strings.Join(where, " AND ")+
		` ORDER BY ano DESC, periodo ASC, nome_funcionario`, args...)
}

func (r *SQLRepository) Chart(ctx context.Context, ano, mes int, periodo string) ([]Programacao, error) {
	return r.programacoes(ctx, `SELECT `+colunasProgramacao+` FROM programacao_ferias WHERE ano = ? AND MONTH(data_inicio) = ? AND periodo = ? ORDER BY nome_funcionario`, ano, mes, periodo)
}

func (r *SQLRepository) QuantidadePorMes(ctx context.Context, ano int) ([]QuantidadePorMes, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT periodo, MONTH(data_inicio) AS mes, COUNT(*) AS quantidade, ano
		FROM programacao_ferias WHERE ano = ? GROUP BY MONTH(data_inicio), periodo, ano`, ano)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []QuantidadePorMes{}
	for rows.Next() {
		var q QuantidadePorMes
		if err := rows.Scan(&q.Periodo, &q.Mes, &q.Quantidade, &q.Ano); err != nil {
			return nil, err
		}
		out = append(out, q)
	}
	return out, rows.Err()
}

func (r *SQLRepository) ProgramacoesUltimoPeriodoAquisitivo(ctx context.Context) ([]Programacao, error) {
	return r.programacoes(ctx, `SELECT `+colunasProgramacao+` FROM programacao_ferias
		WHERE ano = (SELECT ano FROM periodo_aquisitivo ORDER BY ano DESC LIMIT 1)`)
}

func (r *SQLRepository) DefinirAtualizada(ctx context.Context, id int64, valor int) error {
	_, err := r.db.ExecContext(ctx, `UPDATE programacao_ferias SET atualizada = ? WHERE id = ?`, valor, id)
	return err
}

func (r *SQLRepository) ReverterPagamento(ctx context.Context, id int64) error {
	_, err := r.db.ExecContext(ctx, `UPDATE programacao_ferias SET ferias_paga = 'N', data_pagamento = NULL WHERE id = ?`, id)
	return err
}

// --- histórico ---

func (r *SQLRepository) InserirHistorico(ctx context.Context, p Programacao) error {
	var adm any
	if p.DtAdmissao != nil {
		adm = p.DtAdmissao.DiaBanco()
	}
	reg := ""
	if p.Datareg != nil {
		reg = *p.Datareg
	}
	_, err := r.db.ExecContext(ctx, `INSERT INTO programacao_ferias_historico
		(id_programacao_ferias, periodo, data_inicio, data_fim, ferias_paga, data_pagamento, nome_funcionario, cpf, ano,
		 autorizado, cod_lotacao, matricula, dt_admissao, atualizada, membro, abono_pecuniario, antecipar_decimo, datareg)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		p.ID, p.Periodo, wallDataHora(p.DataInicio), wallDataHora(p.DataFim), p.FeriasPaga, wallDataHora(p.DataPagamento),
		p.NomeFuncionario, p.CPF, p.Ano, p.Autorizado, p.CodLotacao, p.Matricula, adm, p.Atualizada, p.Membro,
		p.AbonoPecuniario, p.AnteciparDecimo, reg)
	return err
}

func (r *SQLRepository) HistoricoPorCPF(ctx context.Context, cpf string) ([]Historico, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id, id_programacao_ferias, periodo, data_inicio, data_fim, ferias_paga,
		data_pagamento, COALESCE(nome_funcionario,''), COALESCE(cpf,''), ano, COALESCE(autorizado,''), cod_lotacao,
		matricula, dt_admissao, COALESCE(atualizada,0), COALESCE(membro,''), COALESCE(abono_pecuniario,''),
		COALESCE(antecipar_decimo,''), COALESCE(datareg,'')
		FROM programacao_ferias_historico WHERE cpf = ? ORDER BY id DESC`, cpf)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Historico{}
	for rows.Next() {
		var h Historico
		var ini, fim, pag, adm sql.NullTime
		var paga sql.NullString
		if err := rows.Scan(&h.ID, &h.IDProgramacaoFerias, &h.Periodo, &ini, &fim, &paga, &pag, &h.NomeFuncionario,
			&h.CPF, &h.Ano, &h.Autorizado, &h.CodLotacao, &h.Matricula, &adm, &h.Atualizada, &h.Membro,
			&h.AbonoPecuniario, &h.AnteciparDecimo, &h.Datareg); err != nil {
			return nil, err
		}
		if ini.Valid {
			h.DataInicio = novaDataHora(ini.Time)
		}
		if fim.Valid {
			h.DataFim = novaDataHora(fim.Time)
		}
		if pag.Valid {
			h.DataPagamento = novaDataHora(pag.Time)
		}
		if adm.Valid {
			h.DtAdmissao = admissao(adm.Time)
		}
		if paga.Valid {
			h.FeriasPaga = &paga.String
		}
		out = append(out, h)
	}
	return out, rows.Err()
}

// --- autorização pelo chefe ---

func (r *SQLRepository) VinculosDoChefe(ctx context.Context, cpfChefe string) ([]VinculoChefe, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT COALESCE(nome_chefe,''), COALESCE(cpf_chefe,''), COALESCE(cpf_funcionario,''),
		COALESCE(nome_funcionario,'') FROM chefe_funcionarios WHERE cpf_chefe = ? ORDER BY nome_funcionario`, cpfChefe)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []VinculoChefe
	for rows.Next() {
		var v VinculoChefe
		if err := rows.Scan(&v.NomeChefe, &v.CPFChefe, &v.CPFFuncionario, &v.NomeFuncionario); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

func (r *SQLRepository) ProgramacoesDeCPFs(ctx context.Context, cpfs []string, ano int) ([]Programacao, error) {
	if len(cpfs) == 0 {
		return nil, nil
	}
	ph := strings.TrimSuffix(strings.Repeat("?,", len(cpfs)), ",")
	args := make([]any, 0, len(cpfs)+1)
	for _, c := range cpfs {
		args = append(args, c)
	}
	args = append(args, ano)
	return r.programacoes(ctx, `SELECT `+colunasProgramacao+` FROM programacao_ferias WHERE cpf IN (`+ph+`) AND ano = ? ORDER BY id`, args...)
}

// --- período aquisitivo ---

func scanPeriodo(r rowScanner) (PeriodoAquisitivo, error) {
	var p PeriodoAquisitivo
	var a, b, c, d sql.NullTime
	if err := r.Scan(&p.Ano, &a, &b, &c, &d); err != nil {
		return p, err
	}
	conv := func(t sql.NullTime) *DataHora {
		if t.Valid {
			return novaDataHora(t.Time)
		}
		return nil
	}
	p.PrazoInicial, p.PrazoFinal, p.CalendarioInicial, p.CalendarioFinal = conv(a), conv(b), conv(c), conv(d)
	return p, nil
}

const colunasPeriodo = `ano, prazo_inicial, prazo_final, calendario_inicial, calendario_final`

func (r *SQLRepository) PeriodosAquisitivos(ctx context.Context) ([]PeriodoAquisitivo, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT `+colunasPeriodo+` FROM periodo_aquisitivo ORDER BY ano DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []PeriodoAquisitivo{}
	for rows.Next() {
		p, err := scanPeriodo(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (r *SQLRepository) UltimoPeriodoAquisitivo(ctx context.Context) (*PeriodoAquisitivo, error) {
	p, err := scanPeriodo(r.db.QueryRowContext(ctx, `SELECT `+colunasPeriodo+` FROM periodo_aquisitivo ORDER BY ano DESC LIMIT 1`))
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &p, nil
}

func (r *SQLRepository) SalvarPeriodoAquisitivo(ctx context.Context, p PeriodoAquisitivo) error {
	w := func(d *DataHora) any {
		if d == nil {
			return nil
		}
		return d.Wall().Format("2006-01-02 15:04:05")
	}
	_, err := r.db.ExecContext(ctx, `INSERT INTO periodo_aquisitivo (`+colunasPeriodo+`) VALUES (?,?,?,?,?)
		ON DUPLICATE KEY UPDATE prazo_inicial=VALUES(prazo_inicial), prazo_final=VALUES(prazo_final),
		calendario_inicial=VALUES(calendario_inicial), calendario_final=VALUES(calendario_final)`,
		p.Ano, w(p.PrazoInicial), w(p.PrazoFinal), w(p.CalendarioInicial), w(p.CalendarioFinal))
	return err
}

// --- observações ---

const colunasObservacao = `id, id_programacao_ferias, COALESCE(texto,''), datareg, COALESCE(username,''), COALESCE(excecao_periodo_ferias,'')`

func (r *SQLRepository) observacoes(ctx context.Context, query string, args ...any) ([]Observacao, error) {
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Observacao{}
	for rows.Next() {
		var o Observacao
		var reg sql.NullTime
		if err := rows.Scan(&o.ID, &o.ProgramacaoFeriasID, &o.Texto, &reg, &o.Username, &o.ExcecaoPeriodoFerias); err != nil {
			return nil, err
		}
		if reg.Valid {
			o.Datareg = &Instante{Momento{T: reg.Time.UTC()}}
		}
		out = append(out, o)
	}
	return out, rows.Err()
}

func (r *SQLRepository) Observacoes(ctx context.Context) ([]Observacao, error) {
	return r.observacoes(ctx, `SELECT `+colunasObservacao+` FROM programacao_ferias_observacao`)
}

func (r *SQLRepository) ObservacaoPorID(ctx context.Context, id int64) (*Observacao, error) {
	l, err := r.observacoes(ctx, `SELECT `+colunasObservacao+` FROM programacao_ferias_observacao WHERE id = ?`, id)
	if err != nil || len(l) == 0 {
		return nil, err
	}
	return &l[0], nil
}

func (r *SQLRepository) ObservacoesDoPeriodo(ctx context.Context, id int64) ([]Observacao, error) {
	return r.observacoes(ctx, `SELECT `+colunasObservacao+` FROM programacao_ferias_observacao WHERE id_programacao_ferias = ? ORDER BY id DESC`, id)
}

func (r *SQLRepository) InserirObservacao(ctx context.Context, o Observacao) (int64, error) {
	res, err := r.db.ExecContext(ctx, `INSERT INTO programacao_ferias_observacao
		(id_programacao_ferias, texto, datareg, username, excecao_periodo_ferias) VALUES (?,?,?,?,?)`,
		o.ProgramacaoFeriasID, o.Texto, time.Now().UTC().Format("2006-01-02 15:04:05"), o.Username, o.ExcecaoPeriodoFerias)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (r *SQLRepository) RemoverObservacao(ctx context.Context, id int64) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM programacao_ferias_observacao WHERE id = ?`, id)
	return err
}

// --- alteração de férias ---

func dataColuna(d *DataHora) any {
	if d == nil {
		return nil
	}
	return d.DiaBanco()
}

func (r *SQLRepository) Alteracoes(ctx context.Context) ([]Alteracao, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id, datareg, COALESCE(nome,''), periodo, data_anterior_inicio, data_anterior_fim,
		data_nova_inicio, data_nova_fim, COALESCE(dar_ciencia,''), COALESCE(ano,0), COALESCE(cpf,''), COALESCE(username,'')
		FROM alteracao_ferias`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Alteracao{}
	for rows.Next() {
		var a Alteracao
		var reg, ai, af, ni, nf sql.NullTime
		if err := rows.Scan(&a.ID, &reg, &a.Nome, &a.Periodo, &ai, &af, &ni, &nf, &a.DarCiencia, &a.Ano, &a.CPF, &a.Username); err != nil {
			return nil, err
		}
		if reg.Valid {
			a.Datareg = reg.Time.Format("2006-01-02 15:04")
		}
		conv := func(t sql.NullTime) *DataHora {
			if !t.Valid {
				return nil
			}
			return &DataHora{Momento{T: time.Date(t.Time.Year(), t.Time.Month(), t.Time.Day(), 0, 0, 0, 0, belem).UTC()}}
		}
		a.DataAnteriorInicio, a.DataAnteriorFim, a.DataNovaInicio, a.DataNovaFim = conv(ai), conv(af), conv(ni), conv(nf)
		out = append(out, a)
	}
	return out, rows.Err()
}

func (r *SQLRepository) InserirAlteracao(ctx context.Context, a Alteracao) (int64, error) {
	reg := a.Datareg
	if reg == "" {
		reg = time.Now().In(belem).Format("2006-01-02 15:04:05")
	}
	res, err := r.db.ExecContext(ctx, `INSERT INTO alteracao_ferias (datareg, nome, periodo, data_anterior_inicio, data_anterior_fim,
		data_nova_inicio, data_nova_fim, dar_ciencia, ano, cpf, username) VALUES (?,?,?,?,?,?,?,?,?,?,?)`,
		reg, a.Nome, a.Periodo, dataColuna(a.DataAnteriorInicio), dataColuna(a.DataAnteriorFim),
		dataColuna(a.DataNovaInicio), dataColuna(a.DataNovaFim), a.DarCiencia, a.Ano, a.CPF, a.Username)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (r *SQLRepository) AtualizarAlteracao(ctx context.Context, a Alteracao) error {
	reg := a.Datareg
	if len(reg) == 16 {
		reg += ":00"
	}
	_, err := r.db.ExecContext(ctx, `UPDATE alteracao_ferias SET datareg=?, nome=?, periodo=?, data_anterior_inicio=?,
		data_anterior_fim=?, data_nova_inicio=?, data_nova_fim=?, dar_ciencia=?, ano=?, cpf=?, username=? WHERE id=?`,
		reg, a.Nome, a.Periodo, dataColuna(a.DataAnteriorInicio), dataColuna(a.DataAnteriorFim),
		dataColuna(a.DataNovaInicio), dataColuna(a.DataNovaFim), a.DarCiencia, a.Ano, a.CPF, a.Username, a.ID)
	return err
}

// --- auditoria ---

func (r *SQLRepository) GravarLog(ctx context.Context, username string, operacao int, descricao string) error {
	res, err := r.db.ExecContext(ctx, "INSERT INTO tab_log (data_hora, descricao, id_operacao_log, id_usuario) "+
		"SELECT ?, ?, ?, id FROM `user` WHERE username = ? LIMIT 1",
		time.Now().UTC(), descricao, operacao, username)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("usuário %q não encontrado para o log", username)
	}
	return nil
}

func (r *SQLRepository) InserirDocumento(ctx context.Context, nome, cpf, arquivo string, quando time.Time) error {
	_, err := r.db.ExecContext(ctx, `INSERT INTO aviso_ferias (nome_funcionario, cpf, data_ferias, data_reg, arquivo, id_tipo, observacao)
		VALUES (?,?,?,?,?,1,'')`, nome, cpf, quando.In(belem).Format("2006-01-02"), quando.UTC().Format("2006-01-02 15:04:05"), arquivo)
	return err
}
