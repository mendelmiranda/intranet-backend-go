// Package chefe reimplementa o módulo "chefe" do backend Spring Boot (S3i):
// vínculos entre chefes e servidores na tabela chefe_funcionarios.
package chefe

// ChefeFuncionario é um vínculo chefe → servidor (entidade ChefeFuncionario do legado).
type ChefeFuncionario struct {
	ID                 int64   `json:"id"`
	CPFChefe           string  `json:"cpfChefe"`
	CPFFuncionario     string  `json:"cpfFuncionario"`
	NomeFuncionario    string  `json:"nomeFuncionario"`
	NomeChefe          string  `json:"nomeChefe"`
	VinculoChefe       string  `json:"vinculoChefe"`
	VinculoFuncionario string  `json:"vinculoFuncionario"`
	CodLotacao         *int    `json:"codLotacao"`
	Lotacao            *string `json:"lotacao"`
}

// AtualizaChefeDTO é o corpo do PUT /api/chefe.
type AtualizaChefeDTO struct {
	CPFChefeAntigo string `json:"cpfChefeAntigo"`
	CPFChefeNovo   string `json:"cpfChefeNovo"`
	NomeChefeNovo  string `json:"nomeChefeNovo"`
}
