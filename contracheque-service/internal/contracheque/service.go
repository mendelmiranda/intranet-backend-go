package contracheque

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"sort"
	"strconv"
	"strings"
	"time"

	"tce.ap.gov.br/sistema-corporativo/contracheque-service/internal/platform"
)

// Service concentra as regras de negócio (ContraChequeServiceImpl + HeaderECards).
type Service struct {
	repo        Repository
	verificaURL string
	location    *time.Location
	mesesAno    *platform.TTLCache[[]MesDisponivel]
	mesesEcid   *platform.TTLCache[[]MesDisponivel]
}

func NewService(repo Repository, verificaURL string, location *time.Location, cacheTTL time.Duration) *Service {
	return &Service{
		repo:        repo,
		verificaURL: verificaURL,
		location:    location,
		mesesAno:    platform.NewTTLCache[[]MesDisponivel](cacheTTL),
		mesesEcid:   platform.NewTTLCache[[]MesDisponivel](cacheTTL),
	}
}

func (s *Service) Consulta(ctx context.Context, mes, ano, matricula int) ([]Rubrica, error) {
	rubricas, err := s.repo.Consulta(ctx, mes, ano, matricula)
	if err != nil {
		return nil, err
	}
	if len(rubricas) == 0 {
		return nil, &NaoLocalizadoError{"CONTRA CHEQUE NÃO LOCALIZADO"}
	}
	return rubricas, nil
}

func (s *Service) ConsultaDecimo(ctx context.Context, mes, ano, matricula int) ([]Rubrica, error) {
	rubricas, err := s.repo.ConsultaDecimo(ctx, mes, ano, matricula)
	if err != nil {
		return nil, err
	}
	if len(rubricas) == 0 {
		return nil, &NaoLocalizadoError{"CONTRA CHEQUE/DECIMO NÃO LOCALIZADO"}
	}
	return rubricas, nil
}

// OrdenadasSemRepeticao replica `sortedByDescending { tipoEvento }.toSet()`:
// ordenação estável decrescente (nulo é o menor valor) e remoção de duplicadas
// pela igualdade do data class Rubricas.
func OrdenadasSemRepeticao(rubricas []Rubrica) []Rubrica {
	ordenadas := append([]Rubrica(nil), rubricas...)
	sort.SliceStable(ordenadas, func(i, j int) bool {
		a, b := ordenadas[i].TipoEvento, ordenadas[j].TipoEvento
		switch {
		case a == nil:
			return false
		case b == nil:
			return true
		default:
			return *a > *b
		}
	})

	vistas := make(map[rubricaKey]struct{}, len(ordenadas))
	resultado := make([]Rubrica, 0, len(ordenadas))
	for _, r := range ordenadas {
		k := r.key()
		if _, ok := vistas[k]; ok {
			continue
		}
		vistas[k] = struct{}{}
		resultado = append(resultado, r)
	}
	return resultado
}

// MesesDoAno equivale a mesesDisponiveisDoAno (com cache por ano).
func (s *Service) MesesDoAno(ctx context.Context, ano int) ([]MesDisponivel, error) {
	return s.mesesCacheados(ctx, s.mesesAno, strconv.Itoa(ano), ano)
}

// MesesEcidade replica o controller legado: a matrícula faz parte da chave de
// cache, mas a consulta continua sendo a dos meses habilitados do ano.
func (s *Service) MesesEcidade(ctx context.Context, ano, matricula int) ([]MesDisponivel, error) {
	return s.mesesCacheados(ctx, s.mesesEcid, fmt.Sprintf("%d:%d", ano, matricula), ano)
}

func (s *Service) mesesCacheados(ctx context.Context, cache *platform.TTLCache[[]MesDisponivel], key string, ano int) ([]MesDisponivel, error) {
	if meses, ok := cache.Get(key); ok {
		return meses, nil
	}
	meses, err := s.repo.MesesDoAno(ctx, ano)
	if err != nil {
		return nil, err
	}
	cache.Set(key, meses)
	return meses, nil
}

func (s *Service) Intranet(ctx context.Context, mes, ano, matricula int) (Par, error) {
	header, linhas, err := s.repo.Intranet(ctx, mes, ano, matricula)
	if err != nil {
		return Par{}, err
	}
	if linhas == nil {
		linhas = []Linha{}
	}
	return Par{First: header, Second: linhas}, nil
}

// ErrSemContraCheque indica que não há dados para gerar o PDF (HTTP 404).
var ErrSemContraCheque = errors.New("contracheque não disponível para a competência")

// GerarPDF busca o contracheque, grava um novo código de verificação e monta o PDF.
func (s *Service) GerarPDF(ctx context.Context, mes, ano, matricula int) ([]byte, error) {
	header, linhas, err := s.repo.Intranet(ctx, mes, ano, matricula)
	if err != nil {
		return nil, err
	}
	if header == nil {
		return nil, ErrSemContraCheque
	}

	codigo, err := GerarCodigo()
	if err != nil {
		return nil, err
	}
	if err := s.repo.SalvarCodigo(ctx, matricula, mes, ano, codigo); err != nil {
		return nil, fmt.Errorf("salvar código de verificação: %w", err)
	}

	return renderPDF(pdfInput{
		Mes: mes, Ano: ano, Matricula: matricula,
		Header: *header, Linhas: linhas,
		Codigo: codigo, URL: s.verificaURL + "?codigo=" + codigo,
		Emissao: time.Now().In(s.location),
	})
}

// Verificar devolve nil quando o código não existe.
func (s *Service) Verificar(ctx context.Context, codigo string) (*Verificacao, error) {
	return s.repo.VerificarCodigo(ctx, codigo)
}

// GerarCodigo produz o código de verificação no formato do legado: cinco grupos
// hexadecimais maiúsculos (inteiro aleatório em [0, 0xFFFFF)) separados por ponto.
func GerarCodigo() (string, error) {
	grupos := make([]string, 5)
	limite := big.NewInt(0xFFFFF)
	for i := range grupos {
		n, err := rand.Int(rand.Reader, limite)
		if err != nil {
			return "", err
		}
		grupos[i] = strings.ToUpper(strconv.FormatInt(n.Int64(), 16))
	}
	return strings.Join(grupos, "."), nil
}
