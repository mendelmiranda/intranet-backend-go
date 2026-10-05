package chefe

import (
	"context"
	"fmt"
	"log"
	"sort"
	"strings"
)

// ErrChefeNaoEncontrado indica que não há vínculos para o CPF do chefe informado.
var ErrChefeNaoEncontrado = fmt.Errorf("chefe não encontrado")

type Service struct {
	repo     Repository
	efetivos Efetivos
}

func NewService(repo Repository, efetivos Efetivos) *Service {
	return &Service{repo: repo, efetivos: efetivos}
}

// PorCPFChefe devolve os vínculos do chefe sem filtros (usado por férias).
func (s *Service) PorCPFChefe(ctx context.Context, cpf string) ([]ChefeFuncionario, error) {
	lista, err := s.repo.BuscarPorCPFChefe(ctx, cpf)
	if lista == nil {
		lista = []ChefeFuncionario{}
	}
	return lista, err
}

// PorCPF espelha findByCpf do legado: CPF aparado e ordenação por nome do funcionário.
func (s *Service) PorCPF(ctx context.Context, cpf string) ([]ChefeFuncionario, error) {
	lista, err := s.repo.BuscarPorCPFChefe(ctx, strings.TrimSpace(cpf))
	if err != nil {
		return nil, err
	}
	if lista == nil {
		lista = []ChefeFuncionario{}
	}
	sort.SliceStable(lista, func(i, j int) bool { return lista[i].NomeFuncionario < lista[j].NomeFuncionario })
	return lista, nil
}

// NaAvaliacao lista os servidores efetivos do chefe, sem os de excecao_servidores.
// Com semPares, remove também os servidores que são chefes de alguém.
func (s *Service) NaAvaliacao(ctx context.Context, cpfChefe string, semPares bool) ([]ChefeFuncionario, error) {
	lista, err := s.repo.BuscarPorCPFChefe(ctx, cpfChefe)
	if err != nil {
		return nil, err
	}
	cpfs := make([]string, len(lista))
	for i, c := range lista {
		cpfs[i] = strings.TrimSpace(c.CPFFuncionario)
	}
	efetivos, err := s.efetivos.Efetivos(ctx, cpfs)
	if err != nil {
		return nil, err
	}
	excecao, err := s.repo.CPFsExcecao(ctx)
	if err != nil {
		return nil, err
	}
	fora := map[string]bool{}
	for _, c := range excecao {
		fora[c] = true
	}

	out := []ChefeFuncionario{}
	for _, c := range lista {
		cpf := strings.TrimSpace(c.CPFFuncionario)
		if !efetivos[cpf] || fora[cpf] {
			continue
		}
		if semPares {
			vinculos, err := s.repo.BuscarPorCPFChefe(ctx, cpf)
			if err != nil {
				return nil, err
			}
			if len(vinculos) > 0 {
				continue
			}
		}
		out = append(out, c)
	}
	return out, nil
}

func (s *Service) Efetivo(ctx context.Context, cpf string) (bool, error) {
	m, err := s.efetivos.Efetivos(ctx, []string{strings.TrimSpace(cpf)})
	return m[strings.TrimSpace(cpf)], err
}

func (s *Service) Salvar(ctx context.Context, usuario string, c ChefeFuncionario) (int64, error) {
	s.log(ctx, usuario, OperacaoCadastrando, c.String())
	return s.repo.Salvar(ctx, c)
}

func (s *Service) AtualizarChefe(ctx context.Context, usuario string, d AtualizaChefeDTO) error {
	atuais, err := s.repo.BuscarPorCPFChefe(ctx, strings.TrimSpace(d.CPFChefeAntigo))
	if err != nil {
		return err
	}
	if len(atuais) == 0 {
		return ErrChefeNaoEncontrado
	}
	s.log(ctx, usuario, OperacaoEditando, atuais[0].String())
	_, err = s.repo.AtualizarChefe(ctx, d.NomeChefeNovo, d.CPFChefeNovo, d.CPFChefeAntigo)
	return err
}

func (s *Service) RemoverServidor(ctx context.Context, cpfChefe, cpfFuncionario string) error {
	_, err := s.repo.RemoverServidor(ctx, cpfChefe, cpfFuncionario)
	return err
}

// log é best-effort: falha na auditoria não impede a operação.
func (s *Service) log(ctx context.Context, usuario string, operacao int, descricao string) {
	if err := s.repo.GravarLog(ctx, usuario, operacao, descricao); err != nil {
		log.Printf("log de auditoria do chefe: %v", err)
	}
}

func (c ChefeFuncionario) String() string {
	return fmt.Sprintf("ChefeFuncionario(id=%d, cpfChefe='%s', cpfFuncionario='%s', nomeFuncionario='%s', "+
		"nomeChefe='%s', vinculoChefe='%s', vinculoFuncionario='%s', codLotacao=%s, lotacao='%s')",
		c.ID, c.CPFChefe, c.CPFFuncionario, c.NomeFuncionario, c.NomeChefe, c.VinculoChefe,
		c.VinculoFuncionario, intStr(c.CodLotacao), strPtr(c.Lotacao))
}

func intStr(p *int) string {
	if p == nil {
		return "null"
	}
	return fmt.Sprint(*p)
}

func strPtr(p *string) string {
	if p == nil {
		return "null"
	}
	return *p
}
