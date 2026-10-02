package servidor

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
)

// Repository lê o cadastro completo na view dbo.devops_servidor.
type Repository interface {
	Buscar(ctx context.Context, filtro Filtro) ([]Servidor, error)
	Listar(ctx context.Context, ativo string) ([]Servidor, error)
}

type SQLRepository struct {
	db *sql.DB
}

func NewSQLRepository(db *sql.DB) *SQLRepository {
	return &SQLRepository{db: db}
}

const colunasServidor = `nome, matricula, cgm, dt_admissao, cod_cargo, cargo, cod_funcao, funcao,
	servidor, cod_lotacao, lotacao, classe, regime, cod_previdencia, previdencia, pis, nis, cpf,
	rg, dt_exp_rg, orgao_exp_rg, dt_nascimento, sexo, [natural], nacionalidade, est_civil,
	endereco, numero, logradouro, bairro, municipio, uf, cep, celular, caixapostal, pai, mae,
	hora_mensal, hora_semanal, dt_ult_alteracao, hora_ult_alteracao, dt_cadastro, ano, mes,
	salario_base, agencia, dv_agencia, conta, dv_conta, cod, banco, email, [login], quadro,
	vinculo_rais, ativo, matricula_sistema`

func (r *SQLRepository) Buscar(ctx context.Context, filtro Filtro) ([]Servidor, error) {
	if filtro.vazio() {
		return nil, fmt.Errorf("filtro vazio")
	}

	args := make([]any, 0, 3)
	conds := make([]string, 0, 3)
	if filtro.CPF != "" {
		args = append(args, filtro.CPF)
		conds = append(conds, fmt.Sprintf(
			`REPLACE(REPLACE(REPLACE(ISNULL(cpf, ''), '.', ''), '-', ''), ' ', '') = @p%d`, len(args)))
	}
	if filtro.Matricula != nil {
		args = append(args, *filtro.Matricula)
		conds = append(conds, fmt.Sprintf(`matricula = @p%d`, len(args)))
	}
	if filtro.Nome != "" {
		args = append(args, "%"+escapeLike(filtro.Nome)+"%")
		conds = append(conds, fmt.Sprintf(
			`nome COLLATE Latin1_General_CI_AI LIKE @p%d ESCAPE '\'`, len(args)))
	}

	return r.consultar(ctx, conds, args)
}

// Listar devolve todos os servidores. ativo vazio não filtra; SIM e NAO restringem a coluna ativo.
func (r *SQLRepository) Listar(ctx context.Context, ativo string) ([]Servidor, error) {
	if ativo == "" {
		return r.consultar(ctx, nil, nil)
	}
	return r.consultar(ctx, []string{`ativo = @p1`}, []any{ativo})
}

func (r *SQLRepository) consultar(ctx context.Context, conds []string, args []any) ([]Servidor, error) {
	query := `SELECT ` + colunasServidor + ` FROM dbo.devops_servidor`
	if len(conds) > 0 {
		query += ` WHERE ` + strings.Join(conds, " AND ")
	}
	query += ` ORDER BY nome, matricula`

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	servidores := make([]Servidor, 0, 4)
	for rows.Next() {
		item, err := scanServidor(rows)
		if err != nil {
			return nil, err
		}
		servidores = append(servidores, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return servidores, nil
}

type scanner interface {
	Scan(dest ...any) error
}

func scanServidor(row scanner) (Servidor, error) {
	var (
		nome, cargo, codFuncao, funcao, tipoServidor  sql.NullString
		lotacao, classe, regime, pis, nis, cpf        sql.NullString
		rg, orgaoExpRG, sexo, natural, nacionalidade  sql.NullString
		estCivil, endereco, logradouro, bairro        sql.NullString
		municipio, uf, celular, pai, mae              sql.NullString
		dvAgencia, conta, dvConta, banco, email       sql.NullString
		login, quadro, vinculoRais                    sql.NullString
		previdencia, caixaPostal                      string
		dtUltAlteracao, horaUltAlteracao, ativo       string
		cgm, codPrevidencia                           int
		matricula, codCargo, codLotacao, numero       sql.NullInt64
		cep, horaMensal, horaSemanal, ano, mes        sql.NullInt64
		agencia, cod, matriculaSistema                sql.NullInt64
		salarioBase                                   sql.NullFloat64
		dtAdmissao, dtExpRG, dtNascimento, dtCadastro sql.NullTime
	)

	err := row.Scan(
		&nome, &matricula, &cgm, &dtAdmissao, &codCargo, &cargo, &codFuncao, &funcao,
		&tipoServidor, &codLotacao, &lotacao, &classe, &regime, &codPrevidencia, &previdencia, &pis, &nis, &cpf,
		&rg, &dtExpRG, &orgaoExpRG, &dtNascimento, &sexo, &natural, &nacionalidade, &estCivil,
		&endereco, &numero, &logradouro, &bairro, &municipio, &uf, &cep, &celular, &caixaPostal, &pai, &mae,
		&horaMensal, &horaSemanal, &dtUltAlteracao, &horaUltAlteracao, &dtCadastro, &ano, &mes,
		&salarioBase, &agencia, &dvAgencia, &conta, &dvConta, &cod, &banco, &email, &login, &quadro,
		&vinculoRais, &ativo, &matriculaSistema,
	)
	if err != nil {
		return Servidor{}, err
	}

	return Servidor{
		Nome:             text(nome),
		Matricula:        integer(matricula),
		CGM:              cgm,
		DtAdmissao:       data(dtAdmissao),
		CodCargo:         integer(codCargo),
		Cargo:            text(cargo),
		CodFuncao:        text(codFuncao),
		Funcao:           text(funcao),
		Servidor:         text(tipoServidor),
		CodLotacao:       integer(codLotacao),
		Lotacao:          text(lotacao),
		Classe:           text(classe),
		Regime:           text(regime),
		CodPrevidencia:   codPrevidencia,
		Previdencia:      previdencia,
		PIS:              text(pis),
		NIS:              text(nis),
		CPF:              text(cpf),
		RG:               text(rg),
		DtExpRG:          data(dtExpRG),
		OrgaoExpRG:       text(orgaoExpRG),
		DtNascimento:     data(dtNascimento),
		Sexo:             text(sexo),
		Natural:          text(natural),
		Nacionalidade:    text(nacionalidade),
		EstCivil:         text(estCivil),
		Endereco:         text(endereco),
		Numero:           integer(numero),
		Logradouro:       text(logradouro),
		Bairro:           text(bairro),
		Municipio:        text(municipio),
		UF:               text(uf),
		CEP:              integer(cep),
		Celular:          text(celular),
		CaixaPostal:      caixaPostal,
		Pai:              text(pai),
		Mae:              text(mae),
		HoraMensal:       integer(horaMensal),
		HoraSemanal:      integer(horaSemanal),
		DtUltAlteracao:   dtUltAlteracao,
		HoraUltAlteracao: horaUltAlteracao,
		DtCadastro:       data(dtCadastro),
		Ano:              integer(ano),
		Mes:              integer(mes),
		SalarioBase:      real(salarioBase),
		Agencia:          integer(agencia),
		DvAgencia:        text(dvAgencia),
		Conta:            text(conta),
		DvConta:          text(dvConta),
		Cod:              integer(cod),
		Banco:            text(banco),
		Email:            text(email),
		Login:            text(login),
		Quadro:           text(quadro),
		VinculoRais:      text(vinculoRais),
		Ativo:            ativo,
		MatriculaSistema: integer(matriculaSistema),
	}, nil
}

func text(v sql.NullString) *string {
	if !v.Valid {
		return nil
	}
	s := v.String
	return &s
}

func integer(v sql.NullInt64) *int {
	if !v.Valid {
		return nil
	}
	n := int(v.Int64)
	return &n
}

func real(v sql.NullFloat64) *float64 {
	if !v.Valid {
		return nil
	}
	n := v.Float64
	return &n
}

func data(v sql.NullTime) *string {
	if !v.Valid {
		return nil
	}
	s := formatarData(v.Time)
	return &s
}

func formatarData(t time.Time) string {
	if t.Hour() == 0 && t.Minute() == 0 && t.Second() == 0 {
		return t.Format("2006-01-02")
	}
	return t.Format("2006-01-02T15:04:05")
}

func escapeLike(value string) string {
	replacer := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)
	return replacer.Replace(value)
}
