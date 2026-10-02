// Package contracheque implementa o módulo de contracheque do S3i, espelhando o
// contrato JSON do backend Spring Boot (pacote br.gov.ap.tce.s3i.contracheque).
package contracheque

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Rubrica números especiais usados para compor os totais do ContraCheque.
const (
	rubricaLiquido   = 1013
	rubricaProventos = 1155
	rubricaDescontos = 1156
	rubricaBaseIRRF  = 3120
)

// NaoLocalizadoError corresponde a ContraChequeNaoLocalizadoException (HTTP 400).
type NaoLocalizadoError struct{ Message string }

func (e *NaoLocalizadoError) Error() string { return e.Message }

// Data serializa como o Jackson do Spring Boot (datas como texto, não timestamp):
// java.sql.Date -> "2006-01-02"; java.util.Date/Timestamp -> ISO-8601 em UTC.
type Data struct {
	Time   time.Time
	SoData bool
}

func (d Data) MarshalJSON() ([]byte, error) {
	if d.SoData {
		return json.Marshal(d.Time.Format("2006-01-02"))
	}
	return json.Marshal(d.Time.UTC().Format("2006-01-02T15:04:05.000+00:00"))
}

// ContraCheque é o cabeçalho agregado devolvido dentro de cada Rubrica.
type ContraCheque struct {
	Mes            int      `json:"mes"`
	Ano            int      `json:"ano"`
	Matricula      int      `json:"matricula"`
	MatriculaSist  int      `json:"matriculaSistema"`
	Nome           *string  `json:"nome"`
	CPF            *string  `json:"cpf"`
	Lotacao        *string  `json:"lotacao"`
	Padrao         *string  `json:"padrao"`
	Cargo          *string  `json:"cargo"`
	Admissao       *Data    `json:"admissao"`
	Funcao         *string  `json:"funcao"`
	Agencia        *string  `json:"agencia"`
	Conta          *string  `json:"conta"`
	Banco          *string  `json:"banco"`
	TotalLiquido   *float64 `json:"totalLiquido"`
	TotalProventos *float64 `json:"totalProventos"`
	TotalDescontos *float64 `json:"totalDescontos"`
	BaseIRRF       *float64 `json:"baseIRRF"`
}

// NovoContraCheque replica os valores padrão do bean Kotlin (strings vazias,
// totais 0.0 e admissão = agora).
func NovoContraCheque() ContraCheque {
	vazio := func() *string { s := ""; return &s }
	zero := func() *float64 { f := 0.0; return &f }
	return ContraCheque{
		Nome: vazio(), CPF: vazio(), Lotacao: vazio(), Padrao: vazio(), Cargo: vazio(),
		Funcao: vazio(), Agencia: vazio(), Conta: vazio(), Banco: vazio(),
		Admissao:     &Data{Time: time.Now()},
		TotalLiquido: zero(), TotalProventos: zero(), TotalDescontos: zero(), BaseIRRF: zero(),
	}
}

// Rubrica é uma linha do contracheque. O cabeçalho agregado fica só em memória
// e não entra no JSON da consulta.
type Rubrica struct {
	Rubrica             int          `json:"rubrica"`
	Descricao           *string      `json:"descricao"`
	Quantidade          *string      `json:"quantidade"`
	TipoEvento          *string      `json:"tipoEvento"`
	TipoEventoDescricao *string      `json:"tipoEventoDescricao"`
	Valor               float64      `json:"valor"`
	ContraCheque        ContraCheque `json:"-"`
}

// chave de igualdade do data class Kotlin (o corpo `contraCheque` fica de fora).
type rubricaKey struct {
	rubrica                                    int
	descricao, quantidade, tipo, tipoDescricao string
	descNil, qtdNil, tipoNil, tipoDescNil      bool
	valor                                      float64
}

func (r Rubrica) key() rubricaKey {
	str := func(s *string) (string, bool) {
		if s == nil {
			return "", true
		}
		return *s, false
	}
	k := rubricaKey{rubrica: r.Rubrica, valor: r.Valor}
	k.descricao, k.descNil = str(r.Descricao)
	k.quantidade, k.qtdNil = str(r.Quantidade)
	k.tipo, k.tipoNil = str(r.TipoEvento)
	k.tipoDescricao, k.tipoDescNil = str(r.TipoEventoDescricao)
	return k
}

type MesDisponivel struct {
	Codigo int    `json:"codigo"`
	Mes    string `json:"mes"`
}

type Header struct {
	Nome      string  `json:"nome"`
	Cargo     string  `json:"cargo"`
	Matricula int     `json:"matricula"`
	CPF       *string `json:"cpf"`
	Padrao    *string `json:"padrao"`
	Funcao    *string `json:"funcao"`
	Lotacao   *string `json:"lotacao"`
	Admissao  *Data   `json:"admissao"`
	Agencia   *string `json:"agencia"`
	Conta     *string `json:"conta"`
	Banco     *string `json:"banco"`
}

type Linha struct {
	Rubrica    int     `json:"rubrica"`
	Descricao  string  `json:"descricao"`
	Valor      float64 `json:"valor"`
	TipoEvento string  `json:"tipoEvento"`
	Quantidade string  `json:"quantidade"`
}

// Par reproduz a serialização do kotlin.Pair ({"first": ..., "second": ...}).
type Par struct {
	First  *Header `json:"first"`
	Second []Linha `json:"second"`
}

type Verificacao struct {
	Codigo    string
	Matricula int
	Mes       int
	Ano       int
	DataReg   time.Time
}

// Inteiro aceita número ou texto numérico no JSON, como o Jackson faz para `Int`
// (o frontend envia "mes" e "matriculaSistema" ora como número, ora como string).
type Inteiro int

func (i *Inteiro) UnmarshalJSON(b []byte) error {
	s := strings.TrimSpace(string(bytes.Trim(bytes.TrimSpace(b), `"`)))
	if s == "" || s == "null" {
		*i = 0
		return nil
	}
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		f, ferr := strconv.ParseFloat(s, 64)
		if ferr != nil {
			return fmt.Errorf("valor inteiro inválido: %q", s)
		}
		n = int64(f)
	}
	*i = Inteiro(n)
	return nil
}

// ConsultaRequest cobre os corpos de POST /contra-cheque, /decimo e /gerar.
type ConsultaRequest struct {
	Mes              Inteiro `json:"mes"`
	Ano              Inteiro `json:"ano"`
	Matricula        Inteiro `json:"matricula"`
	MatriculaSistema Inteiro `json:"matriculaSistema"`
}
