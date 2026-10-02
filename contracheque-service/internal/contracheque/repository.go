package contracheque

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

// Repository abstrai o acesso a dados (ContraChequeRepositoryJDBC).
type Repository interface {
	Consulta(ctx context.Context, mes, ano, matricula int) ([]Rubrica, error)
	ConsultaDecimo(ctx context.Context, mes, ano, matricula int) ([]Rubrica, error)
	MesesDoAno(ctx context.Context, ano int) ([]MesDisponivel, error)
	Intranet(ctx context.Context, mes, ano, matricula int) (*Header, []Linha, error)
	SalvarCodigo(ctx context.Context, matricula, mes, ano int, codigo string) error
	VerificarCodigo(ctx context.Context, codigo string) (*Verificacao, error)
}

// SQLRepository usa o SQL Server da folha (GP0001_TCEAP) e o MySQL internet_novo.
type SQLRepository struct {
	folha *sql.DB
	mysql *sql.DB
}

func NewSQLRepository(folha, mysql *sql.DB) *SQLRepository {
	return &SQLRepository{folha: folha, mysql: mysql}
}

const colunasRubrica = `rubrica, descricao, quantidade, tipoevento, tipo_evento_descricao, valor,
	mes, ano, matricula, matricula_sistema, nome, cpf, lotacao, padrao, cargo, admissao,
	funcao, agencia, conta, banco`

func (r *SQLRepository) Consulta(ctx context.Context, mes, ano, matricula int) ([]Rubrica, error) {
	// A view devops_contracheque junta o cadastro do servidor (dezenas de tabelas)
	// e leva cerca de 500 ms. As linhas do contracheque estão em FICHAFINANCEIRA,
	// com a matrícula interna = matrícula pública * 10 + dígito. A competência
	// usada é a habilitada com inOpcaoFiltragem = 'T'.
	competencia := time.Date(ano, time.Month(mes), 1, 0, 0, 0, 0, time.UTC)
	query := fmt.Sprintf(`SELECT f.cdVerba, UPPER(v.DsVerba), f.vlComplemento, v.TpCategoria,
		CASE v.TpCategoria
			WHEN 'P' THEN 'Proventos'
			WHEN 'D' THEN 'Desconto'
			WHEN 'F' THEN 'Folha'
			ELSE v.TpCategoria
		END,
		f.vlMensal
		FROM GP0001_TCEAP.dbo.FICHAFINANCEIRA f
		JOIN GP0001_TCEAP.dbo.VERBA v ON v.CdVerba = f.cdVerba
		WHERE f.cdMatricula BETWEEN @p1 AND @p2
		  AND f.dtCompetencia = @p3
		  AND f.tpCalculo IN (
			SELECT h.tpCalculo FROM GP0001_TCEAP.dbo.HABILITACAOCALCULO h
			WHERE h.dtCompetencia = @p3 AND h.inOpcaoFiltragem = 'T'
		  )
		  AND (v.TpCategoria IN ('P', 'D') OR f.cdVerba IN (%d, %d, %d, %d))`,
		rubricaLiquido, rubricaProventos, rubricaDescontos, rubricaBaseIRRF)

	rows, err := r.folha.QueryContext(ctx, query, matricula*10, matricula*10+9, competencia)
	if err != nil {
		return nil, &NaoLocalizadoError{"OCORREU UM ERRO AO TENTAR CONSULTAR O CONTRA CHEQUE: " + err.Error()}
	}
	defer rows.Close()

	rubricas := make([]Rubrica, 0, 16)
	for rows.Next() {
		var (
			codigo                      sql.NullInt64
			descricao, quantidade, tipo sql.NullString
			tipoDescricao               sql.NullString
			valor                       sql.NullFloat64
		)
		if err := rows.Scan(&codigo, &descricao, &quantidade, &tipo, &tipoDescricao, &valor); err != nil {
			return nil, &NaoLocalizadoError{"OCORREU UM ERRO AO TENTAR CONSULTAR O CONTRA CHEQUE: " + err.Error()}
		}
		rubricas = append(rubricas, Rubrica{
			Rubrica:             int(codigo.Int64),
			Descricao:           strPtr(descricao),
			Quantidade:          strPtr(quantidade),
			TipoEvento:          strPtr(tipo),
			TipoEventoDescricao: strPtr(tipoDescricao),
			Valor:               valor.Float64,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, &NaoLocalizadoError{"OCORREU UM ERRO AO TENTAR CONSULTAR O CONTRA CHEQUE: " + err.Error()}
	}
	return rubricas, nil
}

func (r *SQLRepository) ConsultaDecimo(ctx context.Context, mes, ano, matricula int) ([]Rubrica, error) {
	query := "SELECT " + colunasRubrica + " FROM GP0001_TCEAP.dbo.devops_decimo " +
		"WHERE mes = @p1 AND ano = @p2 AND matricula = @p3 OPTION(MAXDOP 1)"
	rubricas, err := r.consultaRubricas(ctx, query, mes, ano, matricula, false)
	if err != nil {
		return nil, &NaoLocalizadoError{"OCORREU UM ERRO AO TENTAR CONSULTAR O CONTRA CHEQUE/DECIMO: " + err.Error()}
	}
	return rubricas, nil
}

// consultaRubricas lê as linhas e preenche o ContraCheque da primeira rubrica
// (e, no contracheque mensal, os totais a partir das rubricas 1013/1155/1156/3120).
func (r *SQLRepository) consultaRubricas(ctx context.Context, query string, mes, ano, matricula int, comTotais bool) ([]Rubrica, error) {
	rows, err := r.folha.QueryContext(ctx, query, mes, ano, matricula)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var rubricas []Rubrica
	for rows.Next() {
		var (
			rubrica, rMes, rAno, rMatricula, rMatSistema                     sql.NullInt64
			descricao, quantidade, tipo, tipoDesc                            sql.NullString
			valor                                                            sql.NullFloat64
			nome, cpf, lotacao, padrao, cargo, funcao, agencia, conta, banco sql.NullString
			admissao                                                         sql.NullTime
		)
		if err := rows.Scan(&rubrica, &descricao, &quantidade, &tipo, &tipoDesc, &valor,
			&rMes, &rAno, &rMatricula, &rMatSistema, &nome, &cpf, &lotacao, &padrao, &cargo,
			&admissao, &funcao, &agencia, &conta, &banco); err != nil {
			return nil, err
		}

		item := Rubrica{
			Rubrica:             int(rubrica.Int64),
			Descricao:           strPtr(descricao),
			Quantidade:          strPtr(quantidade),
			TipoEvento:          strPtr(tipo),
			TipoEventoDescricao: strPtr(tipoDesc),
			Valor:               valor.Float64,
			ContraCheque:        NovoContraCheque(),
		}
		rubricas = append(rubricas, item)

		cc := &rubricas[0].ContraCheque
		cc.Mes, cc.Ano = int(rMes.Int64), int(rAno.Int64)
		cc.Matricula, cc.MatriculaSist = int(rMatricula.Int64), int(rMatSistema.Int64)
		cc.Nome, cc.CPF, cc.Lotacao, cc.Padrao, cc.Cargo = strPtr(nome), strPtr(cpf), strPtr(lotacao), strPtr(padrao), strPtr(cargo)
		cc.Admissao = dataSQL(admissao)
		cc.Funcao, cc.Agencia, cc.Conta, cc.Banco = strPtr(funcao), strPtr(agencia), strPtr(conta), strPtr(banco)

		if comTotais {
			v := valor.Float64
			switch int(rubrica.Int64) {
			case rubricaLiquido:
				cc.TotalLiquido = &v
			case rubricaProventos:
				cc.TotalProventos = &v
			case rubricaDescontos:
				cc.TotalDescontos = &v
			case rubricaBaseIRRF:
				cc.BaseIRRF = &v
			}
		}
	}
	return rubricas, rows.Err()
}

const nomesMeses = `CASE
	WHEN MONTH(dtCompetencia) = 1  THEN 'Janeiro'
	WHEN MONTH(dtCompetencia) = 2  THEN 'Fevereiro'
	WHEN MONTH(dtCompetencia) = 3  THEN 'Março'
	WHEN MONTH(dtCompetencia) = 4  THEN 'Abril'
	WHEN MONTH(dtCompetencia) = 5  THEN 'Maio'
	WHEN MONTH(dtCompetencia) = 6  THEN 'Junho'
	WHEN MONTH(dtCompetencia) = 7  THEN 'Julho'
	WHEN MONTH(dtCompetencia) = 8  THEN 'Agosto'
	WHEN MONTH(dtCompetencia) = 9  THEN 'Setembro'
	WHEN MONTH(dtCompetencia) = 10 THEN 'Outubro'
	WHEN MONTH(dtCompetencia) = 11 THEN 'Novembro'
	WHEN MONTH(dtCompetencia) = 12 THEN 'Dezembro' END`

func (r *SQLRepository) MesesDoAno(ctx context.Context, ano int) ([]MesDisponivel, error) {
	query := "SELECT MONTH(dtCompetencia) AS mes, " + nomesMeses + " AS descr " +
		"FROM HABILITACAOCALCULO h WHERE YEAR(dtCompetencia) = @p1 AND inOpcaoFiltragem = 'T' " +
		"ORDER BY MONTH(dtCompetencia) DESC OPTION(MAXDOP 1)"

	rows, err := r.folha.QueryContext(ctx, query, ano)
	if err != nil {
		return nil, &NaoLocalizadoError{"NAO FOI POSSIVEL CARREGAR OS MESES: " + err.Error()}
	}
	defer rows.Close()

	lista := []MesDisponivel{}
	for rows.Next() {
		var codigo int
		var descr sql.NullString
		if err := rows.Scan(&codigo, &descr); err != nil {
			return nil, &NaoLocalizadoError{"NAO FOI POSSIVEL CARREGAR OS MESES: " + err.Error()}
		}
		lista = append(lista, MesDisponivel{Codigo: codigo, Mes: descr.String})
	}
	if err := rows.Err(); err != nil {
		return nil, &NaoLocalizadoError{"NAO FOI POSSIVEL CARREGAR OS MESES: " + err.Error()}
	}

	// Regra do legado: em 2020 a competência de janeiro é adicionada ao final da
	// lista quando faltam meses. O Java usa lista.add(11, ...), que falha com
	// IndexOutOfBounds quando há menos de 11 itens.
	if len(lista) < 12 && ano == 2020 {
		if len(lista) < 11 {
			return nil, &NaoLocalizadoError{fmt.Sprintf("NAO FOI POSSIVEL CARREGAR OS MESES: Index: 11, Size: %d", len(lista))}
		}
		lista = append(lista[:11], append([]MesDisponivel{{Codigo: 1, Mes: "Janeiro"}}, lista[11:]...)...)
	}
	return lista, nil
}

func (r *SQLRepository) Intranet(ctx context.Context, mes, ano, matricula int) (*Header, []Linha, error) {
	const query = `SELECT nome, cargo, cpf, lotacao, matricula, rubrica, descricao, valor, tipoevento,
		quantidade, padrao, funcao, admissao, agencia, conta, banco
		FROM devops_contracheque WHERE matricula = @p1 AND mes = @p2 AND ano = @p3 ORDER BY tipoevento`

	rows, err := r.folha.QueryContext(ctx, query, matricula, mes, ano)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()

	var header *Header
	linhas := []Linha{}
	for rows.Next() {
		var (
			nome, cargo, cpf, lotacao, descricao, tipo, quantidade sql.NullString
			padrao, funcao, agencia, conta, banco                  sql.NullString
			mat, rubrica                                           sql.NullInt64
			valor                                                  sql.NullFloat64
			admissao                                               sql.NullTime
		)
		if err := rows.Scan(&nome, &cargo, &cpf, &lotacao, &mat, &rubrica, &descricao, &valor, &tipo,
			&quantidade, &padrao, &funcao, &admissao, &agencia, &conta, &banco); err != nil {
			return nil, nil, err
		}
		if header == nil {
			header = &Header{
				Nome: nome.String, Cargo: cargo.String, Matricula: int(mat.Int64),
				CPF: strPtr(cpf), Padrao: strPtr(padrao), Funcao: strPtr(funcao), Lotacao: strPtr(lotacao),
				Admissao: dataSQL(admissao), Agencia: strPtr(agencia), Conta: strPtr(conta), Banco: strPtr(banco),
			}
		}
		linhas = append(linhas, Linha{
			Rubrica: int(rubrica.Int64), Descricao: descricao.String, Valor: valor.Float64,
			TipoEvento: tipo.String, Quantidade: quantidade.String,
		})
	}
	return header, linhas, rows.Err()
}

// SalvarCodigo atualiza o código da matrícula e, se não houver registro, insere
// um novo (mesmo algoritmo do legado: um único código vigente por matrícula).
func (r *SQLRepository) SalvarCodigo(ctx context.Context, matricula, mes, ano int, codigo string) error {
	res, err := r.mysql.ExecContext(ctx,
		"UPDATE `codigo_verificacao` SET codigo = ?, mes = ?, ano = ?, datareg = NOW() WHERE matricula = ?",
		codigo, mes, ano, matricula)
	if err != nil {
		return err
	}
	if n, err := res.RowsAffected(); err == nil && n > 0 {
		return nil
	}
	// RowsAffected do MySQL é 0 se nada mudou; como datareg = NOW() sempre muda, 0 significa ausência.
	_, err = r.mysql.ExecContext(ctx,
		"INSERT INTO `codigo_verificacao` (codigo, matricula, mes, ano, datareg) VALUES (?, ?, ?, ?, NOW())",
		codigo, matricula, mes, ano)
	return err
}

func (r *SQLRepository) VerificarCodigo(ctx context.Context, codigo string) (*Verificacao, error) {
	var v Verificacao
	err := r.mysql.QueryRowContext(ctx,
		"SELECT codigo, matricula, mes, ano, datareg FROM codigo_verificacao WHERE codigo = ?",
		trimSpace(codigo)).Scan(&v.Codigo, &v.Matricula, &v.Mes, &v.Ano, &v.DataReg)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &v, nil
}

func strPtr(s sql.NullString) *string {
	if !s.Valid {
		return nil
	}
	return &s.String
}

func dataSQL(t sql.NullTime) *Data {
	if !t.Valid {
		return nil
	}
	return &Data{Time: t.Time, SoData: true}
}

func trimSpace(s string) string { return strings.TrimSpace(s) }
