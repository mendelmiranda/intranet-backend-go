package pessoa

import (
	"context"
	"encoding/json"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"
)

const cargosPorPagina = 100
const cargosPaginasMaximas = 20

const cargosQuery = `
query ($after: String) {
  rhVinculos(first: 100, after: $after, filter: { ativo: { eq: true } }) {
    pageInfo { hasNextPage endCursor }
    nodes {
      ... on RhVinculoServidor {
        cargo { codigo nome }
      }
    }
  }
}`

// Cargo é um cargo distinto entre os vínculos ativos.
type Cargo struct {
	Codigo     string `json:"codigo,omitempty"`
	Nome       string `json:"nome"`
	Quantidade int    `json:"quantidade"`
}

// Cargos é o catálogo devolvido ao dashboard.
type Cargos struct {
	Total  int     `json:"total"`
	Cargos []Cargo `json:"cargos"`
}

type cargosPagina struct {
	Data struct {
		RhVinculos struct {
			PageInfo struct {
				HasNextPage bool   `json:"hasNextPage"`
				EndCursor   string `json:"endCursor"`
			} `json:"pageInfo"`
			Nodes []struct {
				Cargo *struct {
					Codigo string `json:"codigo"`
					Nome   string `json:"nome"`
				} `json:"cargo"`
			} `json:"nodes"`
		} `json:"rhVinculos"`
	} `json:"data"`
	Errors []struct {
		Message string `json:"message"`
	} `json:"errors"`
}

// Cargos lista os cargos dos vínculos ativos. q filtra o nome, sem diferenciar maiúsculas.
func (c *Client) Cargos(ctx context.Context, q string) (Cargos, error) {
	q = strings.TrimSpace(q)
	if utf8.RuneCountInString(q) > 80 {
		return Cargos{}, &Error{Status: http.StatusBadRequest, Code: "BAD_REQUEST", Message: "A busca do cargo é longa demais."}
	}

	contagem := map[string]*Cargo{}
	var after any
	for pagina := 0; pagina < cargosPaginasMaximas; pagina++ {
		payload, err := c.post(ctx, map[string]any{
			"query":     cargosQuery,
			"variables": map[string]any{"after": after},
		})
		if err != nil {
			return Cargos{}, err
		}

		var parsed cargosPagina
		if err := json.Unmarshal(payload, &parsed); err != nil {
			return Cargos{}, &Error{Status: http.StatusBadGateway, Code: "BAD_GATEWAY", Message: "Resposta inválida da consulta."}
		}
		if err := graphQLFailure(parsed.Errors); err != nil {
			return Cargos{}, err
		}

		for _, node := range parsed.Data.RhVinculos.Nodes {
			if node.Cargo == nil {
				continue
			}
			nome := strings.TrimSpace(node.Cargo.Nome)
			if nome == "" {
				continue
			}
			codigo := strings.TrimSpace(node.Cargo.Codigo)
			chave := codigo + "\n" + nome
			item := contagem[chave]
			if item == nil {
				item = &Cargo{Codigo: codigo, Nome: nome}
				contagem[chave] = item
			}
			item.Quantidade++
		}

		if !parsed.Data.RhVinculos.PageInfo.HasNextPage || parsed.Data.RhVinculos.PageInfo.EndCursor == "" {
			break
		}
		after = parsed.Data.RhVinculos.PageInfo.EndCursor
	}

	cargos := make([]Cargo, 0, len(contagem))
	busca := strings.ToLower(q)
	for _, item := range contagem {
		if busca != "" && !strings.Contains(strings.ToLower(item.Nome), busca) {
			continue
		}
		cargos = append(cargos, *item)
	}
	slices.SortFunc(cargos, func(a, b Cargo) int {
		return strings.Compare(strings.ToLower(a.Nome), strings.ToLower(b.Nome))
	})
	return Cargos{Total: len(cargos), Cargos: cargos}, nil
}

const servidoresCargoQuery = `
query ($after: String, $codigo: String!) {
  rhVinculos(first: 100, after: $after, filter: { ativo: { eq: true }, cargo: { eq: $codigo } }) {
    pageInfo { hasNextPage endCursor }
    nodes {
      ... on RhVinculoServidor {
        matricula
        lotacao { nome }
        pessoa { cpf nome }
      }
    }
  }
}`

// ServidorCargo é um servidor com vínculo ativo naquele cargo.
type ServidorCargo struct {
	Nome      string `json:"nome"`
	CPF       string `json:"cpf"`
	Matricula int    `json:"matricula,omitempty"`
	Lotacao   string `json:"lotacao,omitempty"`
}

// ServidoresCargo é a lista aberta no modal do cargo.
type ServidoresCargo struct {
	Total      int             `json:"total"`
	Servidores []ServidorCargo `json:"servidores"`
}

type servidoresCargoPagina struct {
	Data struct {
		RhVinculos struct {
			PageInfo struct {
				HasNextPage bool   `json:"hasNextPage"`
				EndCursor   string `json:"endCursor"`
			} `json:"pageInfo"`
			Nodes []struct {
				Matricula int `json:"matricula"`
				Lotacao   *struct {
					Nome string `json:"nome"`
				} `json:"lotacao"`
				Pessoa *struct {
					CPF  string `json:"cpf"`
					Nome string `json:"nome"`
				} `json:"pessoa"`
			} `json:"nodes"`
		} `json:"rhVinculos"`
	} `json:"data"`
	Errors []struct {
		Message string `json:"message"`
	} `json:"errors"`
}

// ServidoresDoCargo lista quem tem vínculo ativo no código do cargo.
func (c *Client) ServidoresDoCargo(ctx context.Context, codigo string) (ServidoresCargo, error) {
	codigo = strings.TrimSpace(codigo)
	if codigo == "" || utf8.RuneCountInString(codigo) > 20 || strings.ContainsAny(codigo, " \n\t") {
		return ServidoresCargo{}, &Error{Status: http.StatusBadRequest, Code: "BAD_REQUEST", Message: "Código do cargo inválido."}
	}

	vistos := map[string]ServidorCargo{}
	var after any
	for pagina := 0; pagina < cargosPaginasMaximas; pagina++ {
		payload, err := c.post(ctx, map[string]any{
			"query": servidoresCargoQuery,
			"variables": map[string]any{
				"after":  after,
				"codigo": codigo,
			},
		})
		if err != nil {
			return ServidoresCargo{}, err
		}

		var parsed servidoresCargoPagina
		if err := json.Unmarshal(payload, &parsed); err != nil {
			return ServidoresCargo{}, &Error{Status: http.StatusBadGateway, Code: "BAD_GATEWAY", Message: "Resposta inválida da consulta."}
		}
		if err := graphQLFailure(parsed.Errors); err != nil {
			return ServidoresCargo{}, err
		}

		for _, node := range parsed.Data.RhVinculos.Nodes {
			if node.Pessoa == nil {
				continue
			}
			nome := strings.TrimSpace(node.Pessoa.Nome)
			cpf := onlyDigits(node.Pessoa.CPF)
			if nome == "" || len(cpf) != 11 {
				continue
			}
			lotacao := ""
			if node.Lotacao != nil {
				lotacao = strings.TrimSpace(node.Lotacao.Nome)
			}
			chave := cpf + "\n" + strconv.Itoa(node.Matricula)
			vistos[chave] = ServidorCargo{
				Nome:      nome,
				CPF:       cpf,
				Matricula: node.Matricula,
				Lotacao:   lotacao,
			}
		}

		if !parsed.Data.RhVinculos.PageInfo.HasNextPage || parsed.Data.RhVinculos.PageInfo.EndCursor == "" {
			break
		}
		after = parsed.Data.RhVinculos.PageInfo.EndCursor
	}

	servidores := make([]ServidorCargo, 0, len(vistos))
	for _, item := range vistos {
		servidores = append(servidores, item)
	}
	slices.SortFunc(servidores, func(a, b ServidorCargo) int {
		return strings.Compare(strings.ToLower(a.Nome), strings.ToLower(b.Nome))
	})
	return ServidoresCargo{Total: len(servidores), Servidores: servidores}, nil
}
