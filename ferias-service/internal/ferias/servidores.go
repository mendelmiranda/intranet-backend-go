package ferias

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

// Folha consulta o cadastro de servidores (view dbo.devops_servidor, SQL Server da folha).
type Folha interface {
	// PorCPF devolve o servidor ativo de maior matrícula, ou nil se não existir.
	PorCPF(ctx context.Context, cpf string) (*ServidorFolha, error)
	// Restantes lista servidores ativos filtrados por tipo (S/N), CPF e lotação.
	Restantes(ctx context.Context, f Parametros) ([]ServidorFolha, error)
}

type SQLFolha struct{ db *sql.DB }

func NewSQLFolha(db *sql.DB) *SQLFolha { return &SQLFolha{db: db} }

const colunasFolha = `cpf, nome, matricula, cgm, dt_admissao, cod_cargo, cargo, cod_funcao, funcao, servidor,
	cod_lotacao, lotacao, classe, regime, email, [login], quadro, ativo, matricula_sistema`

func scanFolha(r rowScanner) (ServidorFolha, error) {
	var s ServidorFolha
	var cpf, nome, cargo, codFuncao, funcao, servidor, lotacao, classe, regime, email, login, quadro, ativo sql.NullString
	var matricula, cgm, codCargo, codLotacao, matSistema sql.NullInt64
	var adm sql.NullTime
	if err := r.Scan(&cpf, &nome, &matricula, &cgm, &adm, &codCargo, &cargo, &codFuncao, &funcao, &servidor,
		&codLotacao, &lotacao, &classe, &regime, &email, &login, &quadro, &ativo, &matSistema); err != nil {
		return s, err
	}
	str := func(v sql.NullString) *string {
		if !v.Valid {
			return nil
		}
		return &v.String
	}
	num := func(v sql.NullInt64) *int {
		if !v.Valid {
			return nil
		}
		n := int(v.Int64)
		return &n
	}
	s.CPF, s.Nome, s.Cargo, s.CodFuncao, s.Funcao, s.Servidor = str(cpf), str(nome), str(cargo), str(codFuncao), str(funcao), str(servidor)
	s.Lotacao, s.Classe, s.Regime, s.Email, s.Login, s.Quadro, s.Ativo = str(lotacao), str(classe), str(regime), str(email), str(login), str(quadro), str(ativo)
	s.Matricula, s.CGM, s.CodCargo, s.CodLotacao, s.MatriculaSistema = num(matricula), num(cgm), num(codCargo), num(codLotacao), num(matSistema)
	if adm.Valid {
		s.DtAdmissao = admissao(adm.Time)
	}
	return s, nil
}

func (f *SQLFolha) PorCPF(ctx context.Context, cpf string) (*ServidorFolha, error) {
	row := f.db.QueryRowContext(ctx, `SELECT TOP 1 `+colunasFolha+` FROM dbo.devops_servidor
		WHERE cpf = @p1 AND ativo = 'SIM' ORDER BY matricula DESC OPTION (MAXDOP 1)`, strings.TrimSpace(cpf))
	s, err := scanFolha(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &s, nil
}

func (f *SQLFolha) Restantes(ctx context.Context, p Parametros) ([]ServidorFolha, error) {
	conds := []string{"ativo = 'SIM'"}
	var args []any
	switch strings.ToUpper(strings.TrimSpace(p.TipoServidor)) {
	case "S":
		conds = append(conds, "servidor LIKE 'SERVIDORES%'")
	case "N":
		conds = append(conds, "(servidor LIKE 'CONSELHEIRO%' OR servidor LIKE 'PROCURADOR%')")
	}
	if p.CPF != "" {
		args = append(args, p.CPF)
		conds = append(conds, fmt.Sprintf("cpf = @p%d", len(args)))
	}
	if p.Lotacao > 0 {
		args = append(args, p.Lotacao)
		conds = append(conds, fmt.Sprintf("cod_lotacao = @p%d", len(args)))
	}
	rows, err := f.db.QueryContext(ctx, `SELECT `+colunasFolha+` FROM dbo.devops_servidor WHERE `+
		strings.Join(conds, " AND ")+` ORDER BY nome OPTION (MAXDOP 1)`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []ServidorFolha{}
	for rows.Next() {
		s, err := scanFolha(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}
