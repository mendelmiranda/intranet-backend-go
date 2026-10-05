package pessoa

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"unicode"
	"unicode/utf8"
)

const pesquisaLimite = 8

const pesquisaQuery = `
query ($nome: String!) {
  rhPessoas(page: 1, pageSize: 8, filter: { nome: { contains: $nome } }) {
    totalCount
    nodes {
      cpf
      nome
      nomeSocial
      ativo
      vinculos { tipo lotacao { nome } ativo }
    }
  }
}`

// Resumo é um servidor na lista do autocomplete.
type Resumo struct {
	CPF        string `json:"cpf"`
	Nome       string `json:"nome"`
	NomeSocial string `json:"nomeSocial,omitempty"`
	Ativo      bool   `json:"ativo"`
	Tipo       string `json:"tipo,omitempty"`
	Lotacao    string `json:"lotacao,omitempty"`
}

// Pesquisa é a página curta devolvida enquanto o nome é digitado.
type Pesquisa struct {
	Total   int      `json:"total"`
	Pessoas []Resumo `json:"pessoas"`
}

type pesquisaResponse struct {
	Data struct {
		RhPessoas struct {
			TotalCount int `json:"totalCount"`
			Nodes      []struct {
				CPF        string `json:"cpf"`
				Nome       string `json:"nome"`
				NomeSocial string `json:"nomeSocial"`
				Ativo      bool   `json:"ativo"`
				Vinculos   []struct {
					Tipo    string `json:"tipo"`
					Ativo   bool   `json:"ativo"`
					Lotacao *struct {
						Nome string `json:"nome"`
					} `json:"lotacao"`
				} `json:"vinculos"`
			} `json:"nodes"`
		} `json:"rhPessoas"`
	} `json:"data"`
	Errors []struct {
		Message string `json:"message"`
	} `json:"errors"`
}

// Pesquisar lista servidores cujo nome contém o texto informado.
func (c *Client) Pesquisar(ctx context.Context, nome string) (Pesquisa, error) {
	nome, ok := nomePesquisa(nome)
	if !ok {
		return Pesquisa{}, &Error{Status: http.StatusBadRequest, Code: "BAD_REQUEST", Message: "Informe ao menos duas letras do nome."}
	}

	payload, err := c.post(ctx, map[string]any{
		"query":     pesquisaQuery,
		"variables": map[string]string{"nome": nome},
	})
	if err != nil {
		return Pesquisa{}, err
	}

	var parsed pesquisaResponse
	if err := json.Unmarshal(payload, &parsed); err != nil {
		return Pesquisa{}, &Error{Status: http.StatusBadGateway, Code: "BAD_GATEWAY", Message: "Resposta inválida da consulta."}
	}
	if err := graphQLFailure(parsed.Errors); err != nil {
		return Pesquisa{}, err
	}

	pessoas := make([]Resumo, 0, len(parsed.Data.RhPessoas.Nodes))
	for _, node := range parsed.Data.RhPessoas.Nodes {
		nome := strings.TrimSpace(node.Nome)
		if nome == "" || len(onlyDigits(node.CPF)) != 11 {
			continue
		}
		tipo, lotacao := vinculoResumo(node.Vinculos)
		pessoas = append(pessoas, Resumo{
			CPF:        onlyDigits(node.CPF),
			Nome:       nome,
			NomeSocial: strings.TrimSpace(node.NomeSocial),
			Ativo:      node.Ativo,
			Tipo:       tipo,
			Lotacao:    lotacao,
		})
	}
	return Pesquisa{Total: parsed.Data.RhPessoas.TotalCount, Pessoas: pessoas}, nil
}

func vinculoResumo(vinculos []struct {
	Tipo    string `json:"tipo"`
	Ativo   bool   `json:"ativo"`
	Lotacao *struct {
		Nome string `json:"nome"`
	} `json:"lotacao"`
}) (string, string) {
	if len(vinculos) == 0 {
		return "", ""
	}
	escolhido := vinculos[0]
	for _, vinculo := range vinculos {
		if vinculo.Ativo {
			escolhido = vinculo
			break
		}
	}
	lotacao := ""
	if escolhido.Lotacao != nil {
		lotacao = strings.TrimSpace(escolhido.Lotacao.Nome)
	}
	return strings.TrimSpace(escolhido.Tipo), lotacao
}

func nomePesquisa(value string) (string, bool) {
	value = strings.Join(strings.Fields(value), " ")
	tamanho := utf8.RuneCountInString(value)
	if tamanho < 2 || tamanho > 80 {
		return "", false
	}
	for _, r := range value {
		if unicode.IsLetter(r) || unicode.IsSpace(r) || r == '-' || r == '\'' || r == '.' {
			continue
		}
		return "", false
	}
	return value, true
}
