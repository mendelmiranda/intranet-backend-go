package chefe

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
)

// Repository acessa o MySQL internet_novo.
type Repository interface {
	Listar(ctx context.Context) ([]ChefeFuncionario, error)
	BuscarPorID(ctx context.Context, id int64) (*ChefeFuncionario, error)
	// BuscarPorCPFChefe devolve os vínculos do chefe ordenados por nome do funcionário.
	BuscarPorCPFChefe(ctx context.Context, cpfChefe string) ([]ChefeFuncionario, error)
	BuscarPorCPFFuncionario(ctx context.Context, cpf string) (*ChefeFuncionario, error)
	ListarPorLotacao(ctx context.Context, codLotacao int) ([]ChefeFuncionario, error)
	Salvar(ctx context.Context, c ChefeFuncionario) (int64, error)
	AtualizarChefe(ctx context.Context, nomeNovo, cpfNovo, cpfAntigo string) (int64, error)
	RemoverServidor(ctx context.Context, cpfChefe, cpfFuncionario string) (int64, error)
	// CPFsExcecao devolve os CPFs de excecao_servidores (fora da avaliação).
	CPFsExcecao(ctx context.Context) ([]string, error)
	// GravarLog registra a operação em tab_log em nome do usuário (user.username).
	GravarLog(ctx context.Context, username string, operacao int, descricao string) error
}

type SQLRepository struct {
	db *sql.DB
}

func NewSQLRepository(db *sql.DB) *SQLRepository { return &SQLRepository{db: db} }

const colunas = `id, COALESCE(cpf_chefe,''), COALESCE(cpf_funcionario,''), COALESCE(nome_funcionario,''),
	COALESCE(nome_chefe,''), COALESCE(vinculo_chefe,''), COALESCE(vinculo_funcionario,''), cod_lotacao, lotacao`

type rowScanner interface{ Scan(dest ...any) error }

func scan(r rowScanner) (ChefeFuncionario, error) {
	var c ChefeFuncionario
	var cod sql.NullInt64
	var lot sql.NullString
	if err := r.Scan(&c.ID, &c.CPFChefe, &c.CPFFuncionario, &c.NomeFuncionario, &c.NomeChefe,
		&c.VinculoChefe, &c.VinculoFuncionario, &cod, &lot); err != nil {
		return c, err
	}
	if cod.Valid {
		v := int(cod.Int64)
		c.CodLotacao = &v
	}
	if lot.Valid {
		c.Lotacao = &lot.String
	}
	return c, nil
}

func (r *SQLRepository) lista(ctx context.Context, query string, args ...any) ([]ChefeFuncionario, error) {
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []ChefeFuncionario{}
	for rows.Next() {
		c, err := scan(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (r *SQLRepository) um(ctx context.Context, query string, args ...any) (*ChefeFuncionario, error) {
	c, err := scan(r.db.QueryRowContext(ctx, query, args...))
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &c, nil
}

func (r *SQLRepository) Listar(ctx context.Context) ([]ChefeFuncionario, error) {
	return r.lista(ctx, `SELECT `+colunas+` FROM chefe_funcionarios`)
}

func (r *SQLRepository) BuscarPorID(ctx context.Context, id int64) (*ChefeFuncionario, error) {
	return r.um(ctx, `SELECT `+colunas+` FROM chefe_funcionarios WHERE id = ?`, id)
}

func (r *SQLRepository) BuscarPorCPFChefe(ctx context.Context, cpf string) ([]ChefeFuncionario, error) {
	return r.lista(ctx, `SELECT `+colunas+` FROM chefe_funcionarios WHERE cpf_chefe = ? ORDER BY nome_funcionario`, cpf)
}

// BuscarPorCPFFuncionario espelha o legado, que espera no máximo um chefe por servidor.
func (r *SQLRepository) BuscarPorCPFFuncionario(ctx context.Context, cpf string) (*ChefeFuncionario, error) {
	return r.um(ctx, `SELECT `+colunas+` FROM chefe_funcionarios WHERE cpf_funcionario = ? LIMIT 1`, cpf)
}

func (r *SQLRepository) ListarPorLotacao(ctx context.Context, cod int) ([]ChefeFuncionario, error) {
	return r.lista(ctx, `SELECT `+colunas+` FROM chefe_funcionarios WHERE cod_lotacao = ? ORDER BY nome_funcionario`, cod)
}

func (r *SQLRepository) Salvar(ctx context.Context, c ChefeFuncionario) (int64, error) {
	res, err := r.db.ExecContext(ctx, `INSERT INTO chefe_funcionarios
		(cpf_chefe, cpf_funcionario, nome_funcionario, nome_chefe, vinculo_chefe, vinculo_funcionario, cod_lotacao, lotacao)
		VALUES (?,?,?,?,?,?,?,?)`,
		c.CPFChefe, c.CPFFuncionario, c.NomeFuncionario, c.NomeChefe, c.VinculoChefe, c.VinculoFuncionario,
		c.CodLotacao, c.Lotacao)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (r *SQLRepository) AtualizarChefe(ctx context.Context, nomeNovo, cpfNovo, cpfAntigo string) (int64, error) {
	res, err := r.db.ExecContext(ctx,
		`UPDATE chefe_funcionarios SET nome_chefe = ?, cpf_chefe = ? WHERE cpf_chefe = ?`, nomeNovo, cpfNovo, cpfAntigo)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

func (r *SQLRepository) RemoverServidor(ctx context.Context, cpfChefe, cpfFuncionario string) (int64, error) {
	res, err := r.db.ExecContext(ctx,
		`DELETE FROM chefe_funcionarios WHERE cpf_chefe = ? AND cpf_funcionario = ?`, cpfChefe, cpfFuncionario)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

func (r *SQLRepository) CPFsExcecao(ctx context.Context) ([]string, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT COALESCE(cpf,'') FROM excecao_servidores`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			return nil, err
		}
		out = append(out, strings.TrimSpace(s))
	}
	return out, rows.Err()
}

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

// Efetivos consulta a view da folha (MSSQL).
type Efetivos interface {
	// Efetivos devolve, entre os CPFs informados, os de servidores efetivos ativos.
	Efetivos(ctx context.Context, cpfs []string) (map[string]bool, error)
}

type SQLEfetivos struct {
	db *sql.DB
}

func NewSQLEfetivos(db *sql.DB) *SQLEfetivos { return &SQLEfetivos{db: db} }

func (e *SQLEfetivos) Efetivos(ctx context.Context, cpfs []string) (map[string]bool, error) {
	out := map[string]bool{}
	const lote = 500
	for i := 0; i < len(cpfs); i += lote {
		fim := min(i+lote, len(cpfs))
		part := cpfs[i:fim]
		ph := make([]string, len(part))
		args := make([]any, len(part))
		for j, c := range part {
			ph[j] = fmt.Sprintf("@p%d", j+1)
			args[j] = c
		}
		rows, err := e.db.QueryContext(ctx, `SELECT cpf FROM dbo.devops_servidor
			WHERE servidor = 'SERVIDORES EFETIVOS' AND ativo = 'SIM' AND cpf IN (`+strings.Join(ph, ",")+`) OPTION (MAXDOP 1)`, args...)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var cpf sql.NullString
			if err := rows.Scan(&cpf); err != nil {
				rows.Close()
				return nil, err
			}
			out[strings.TrimSpace(cpf.String)] = true
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return nil, err
		}
	}
	return out, nil
}
