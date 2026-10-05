package ferias

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
)

// Armazenamento guarda os comprovantes gerados.
type Armazenamento interface {
	Salvar(cpf, arquivo string, pdf []byte) error
}

// ArmazenamentoFS grava em {Dir}/{cpf}/{arquivo}, a estrutura lida pelo módulo de documentos do Spring.
type ArmazenamentoFS struct{ Dir string }

var (
	cpfSeguro     = regexp.MustCompile(`^\d{11}$`)
	arquivoSeguro = regexp.MustCompile(`^/[A-Za-z0-9_.-]+\.pdf$`)
)

func (a ArmazenamentoFS) Salvar(cpf, arquivo string, pdf []byte) error {
	if !cpfSeguro.MatchString(cpf) || !arquivoSeguro.MatchString(arquivo) {
		return fmt.Errorf("destino de comprovante inválido: %q %q", cpf, arquivo)
	}
	dir := filepath.Join(a.Dir, cpf)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, arquivo[1:]), pdf, 0o640)
}
