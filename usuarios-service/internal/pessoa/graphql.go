// Package pessoa consulta o cadastro do servidor na API GraphQL de staging.
package pessoa

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
	"unicode"
)

const requestTimeout = 15 * time.Second

const pessoaQuery = `
query {
  rhPessoa(cpf: "%s") {
    cpf
    nome
    nomeSocial
    email
    ativo
    dadosPessoais {
      dataNascimento
      sexo
      nomeMae
      nomePai
      identidade { numero orgaoEmissor uf dataExpedicao }
      pis
      celular
      emailPessoal
      endereco { logradouro numero complemento bairro municipio uf cep }
    }
    vinculos {
      matricula
      matriculaSistema
      contrato
      grupo
      tipo
      lotacao { codigo nome }
      historicoLotacao { inicio fim lotacao { codigo nome } }
      inicio
      fim
      ativo
      contaBancaria {
        agencia
        agenciaDigito
        conta
        contaDigito
        banco { codigo nome }
      }
    }
  }
}`

type Error struct {
	Status  int
	Code    string
	Message string
}

func (e *Error) Error() string { return e.Message }

// TokenSource fornece o access token de client credentials exigido pela API.
type TokenSource interface {
	ServiceToken(ctx context.Context) (string, error)
}

type Client struct {
	url    string
	http   *http.Client
	tokens TokenSource
}

func NewClient(graphqlURL string, tokens TokenSource) *Client {
	return &Client{
		url:    graphqlURL,
		http:   &http.Client{Timeout: requestTimeout},
		tokens: tokens,
	}
}

// Cadastro é o rhPessoa devolvido ao dashboard.
type Cadastro struct {
	CPF           string         `json:"cpf"`
	Nome          string         `json:"nome"`
	NomeSocial    string         `json:"nomeSocial,omitempty"`
	Email         string         `json:"email,omitempty"`
	Ativo         bool           `json:"ativo"`
	DadosPessoais *DadosPessoais `json:"dadosPessoais,omitempty"`
	Vinculos      []Vinculo      `json:"vinculos"`
}

type DadosPessoais struct {
	DataNascimento string      `json:"dataNascimento,omitempty"`
	Sexo           string      `json:"sexo,omitempty"`
	NomeMae        string      `json:"nomeMae,omitempty"`
	NomePai        string      `json:"nomePai,omitempty"`
	Identidade     *Identidade `json:"identidade,omitempty"`
	PIS            string      `json:"pis,omitempty"`
	Celular        string      `json:"celular,omitempty"`
	EmailPessoal   string      `json:"emailPessoal,omitempty"`
	Endereco       *Endereco   `json:"endereco,omitempty"`
}

type Identidade struct {
	Numero        string `json:"numero,omitempty"`
	OrgaoEmissor  string `json:"orgaoEmissor,omitempty"`
	UF            string `json:"uf,omitempty"`
	DataExpedicao string `json:"dataExpedicao,omitempty"`
}

type Endereco struct {
	Logradouro  string `json:"logradouro,omitempty"`
	Numero      string `json:"numero,omitempty"`
	Complemento string `json:"complemento,omitempty"`
	Bairro      string `json:"bairro,omitempty"`
	Municipio   string `json:"municipio,omitempty"`
	UF          string `json:"uf,omitempty"`
	CEP         string `json:"cep,omitempty"`
}

type Vinculo struct {
	Matricula        int              `json:"matricula"`
	MatriculaSistema int              `json:"matriculaSistema"`
	Contrato         int              `json:"contrato"`
	Grupo            string           `json:"grupo,omitempty"`
	Tipo             string           `json:"tipo,omitempty"`
	Lotacao          *CodigoNome      `json:"lotacao,omitempty"`
	HistoricoLotacao []PeriodoLotacao `json:"historicoLotacao,omitempty"`
	Inicio           string           `json:"inicio,omitempty"`
	Fim              string           `json:"fim,omitempty"`
	Ativo            bool             `json:"ativo"`
	ContaBancaria    *ContaBancaria   `json:"contaBancaria,omitempty"`
}

type CodigoNome struct {
	Codigo string `json:"codigo,omitempty"`
	Nome   string `json:"nome,omitempty"`
}

type PeriodoLotacao struct {
	Lotacao *CodigoNome `json:"lotacao,omitempty"`
	Inicio  string      `json:"inicio,omitempty"`
	Fim     string      `json:"fim,omitempty"`
}

type ContaBancaria struct {
	Banco         *CodigoNome `json:"banco,omitempty"`
	Agencia       string      `json:"agencia,omitempty"`
	AgenciaDigito string      `json:"agenciaDigito,omitempty"`
	Conta         string      `json:"conta,omitempty"`
	ContaDigito   string      `json:"contaDigito,omitempty"`
}

type graphQLResponse struct {
	Data struct {
		RhPessoa *Cadastro `json:"rhPessoa"`
	} `json:"data"`
	Errors []struct {
		Message string `json:"message"`
	} `json:"errors"`
}

// Buscar consulta rhPessoa com o token de serviço do Keycloak.
func (c *Client) Buscar(ctx context.Context, cpf string) (Cadastro, error) {
	cpf = onlyDigits(cpf)
	if len(cpf) != 11 {
		return Cadastro{}, &Error{Status: http.StatusBadRequest, Code: "BAD_REQUEST", Message: "CPF inválido."}
	}
	payload, err := c.post(ctx, map[string]string{
		"query": fmt.Sprintf(pessoaQuery, cpf),
	})
	if err != nil {
		return Cadastro{}, err
	}

	var parsed graphQLResponse
	if err := json.Unmarshal(payload, &parsed); err != nil {
		return Cadastro{}, &Error{Status: http.StatusBadGateway, Code: "BAD_GATEWAY", Message: "Resposta inválida da consulta."}
	}
	if err := graphQLFailure(parsed.Errors); err != nil {
		return Cadastro{}, err
	}
	if parsed.Data.RhPessoa == nil || strings.TrimSpace(parsed.Data.RhPessoa.Nome) == "" {
		return Cadastro{}, &Error{Status: http.StatusNotFound, Code: "NOT_FOUND", Message: "Nenhum cadastro encontrado para este CPF."}
	}

	cadastro := *parsed.Data.RhPessoa
	cadastro.Nome = strings.TrimSpace(cadastro.Nome)
	cadastro.NomeSocial = strings.TrimSpace(cadastro.NomeSocial)
	cadastro.Email = strings.TrimSpace(cadastro.Email)
	if cadastro.CPF == "" {
		cadastro.CPF = cpf
	}
	if cadastro.Vinculos == nil {
		cadastro.Vinculos = []Vinculo{}
	}
	return cadastro, nil
}

func (c *Client) post(ctx context.Context, body any) ([]byte, error) {
	token, err := c.tokens.ServiceToken(ctx)
	if err != nil || strings.TrimSpace(token) == "" {
		message := "Não foi possível obter o token de consulta no Keycloak."
		if err != nil && err.Error() != "" {
			message = err.Error()
		}
		return nil, &Error{Status: http.StatusBadGateway, Code: "BAD_GATEWAY", Message: message}
	}

	raw, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.url, bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	res, err := c.http.Do(req)
	if err != nil {
		return nil, &Error{Status: http.StatusBadGateway, Code: "BAD_GATEWAY", Message: "Não foi possível consultar o cadastro."}
	}
	defer res.Body.Close()

	payload, err := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if err != nil {
		return nil, &Error{Status: http.StatusBadGateway, Code: "BAD_GATEWAY", Message: "Resposta inválida da consulta."}
	}
	if res.StatusCode == http.StatusUnauthorized || res.StatusCode == http.StatusForbidden {
		return nil, &Error{Status: http.StatusBadGateway, Code: "BAD_GATEWAY", Message: "A API recusou o token de consulta."}
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return nil, &Error{Status: http.StatusBadGateway, Code: "BAD_GATEWAY", Message: "A consulta do cadastro foi recusada."}
	}
	return payload, nil
}

func graphQLFailure(errors []struct {
	Message string `json:"message"`
}) error {
	for _, item := range errors {
		if strings.TrimSpace(item.Message) != "" {
			return &Error{Status: http.StatusBadGateway, Code: "BAD_GATEWAY", Message: item.Message}
		}
	}
	return nil
}

func onlyDigits(value string) string {
	var builder strings.Builder
	for _, r := range value {
		if unicode.IsDigit(r) {
			builder.WriteRune(r)
		}
	}
	return builder.String()
}
