package ferias

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// belem é o fuso do legado (a JVM roda em America/Belem).
var belem = func() *time.Location {
	if loc, err := time.LoadLocation("America/Belem"); err == nil {
		return loc
	}
	return time.FixedZone("-03", -3*3600)
}()

// Momento é um instante lido do JSON ou do banco. O banco guarda o "relógio" em UTC
// (serverTimezone=UTC no legado); Dia indica que a entrada era só uma data (yyyy-MM-dd).
type Momento struct {
	T   time.Time
	Dia bool
}

// Wall é o valor gravado em colunas datetime (relógio UTC).
func (m Momento) Wall() time.Time { return m.T.UTC() }

// DiaBanco é o valor gravado em colunas date: a data local de Belém para instantes
// (como o Hibernate/JDBC do legado) ou a própria data quando a entrada era só yyyy-MM-dd.
func (m Momento) DiaBanco() string {
	if m.Dia {
		return m.T.UTC().Format("2006-01-02")
	}
	return m.T.In(belem).Format("2006-01-02")
}

// Dia devolve a data (parte yyyy-MM-dd do relógio UTC), usada nas contas de dias.
func (m Momento) Data() time.Time {
	y, mo, d := m.T.UTC().Date()
	return time.Date(y, mo, d, 0, 0, 0, 0, time.UTC)
}

var layouts = []string{
	"2006-01-02T15:04:05.000Z07:00",
	"2006-01-02T15:04:05Z07:00",
	"2006-01-02T15:04:05.000",
	"2006-01-02T15:04:05",
	"2006-01-02 15:04:05",
	"2006-01-02 15:04",
}

func parseMomento(raw []byte) (*Momento, error) {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 || string(raw) == "null" {
		return nil, nil
	}
	if raw[0] != '"' {
		var ms int64
		if err := json.Unmarshal(raw, &ms); err != nil {
			return nil, fmt.Errorf("data inválida: %s", raw)
		}
		return &Momento{T: time.UnixMilli(ms).UTC()}, nil
	}
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		return nil, err
	}
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, nil
	}
	if t, err := time.Parse("2006-01-02", s); err == nil {
		return &Momento{T: t, Dia: true}, nil
	}
	for _, l := range layouts {
		if t, err := time.Parse(l, s); err == nil {
			return &Momento{T: t.UTC()}, nil
		}
	}
	return nil, fmt.Errorf("data inválida: %q", s)
}

// Data serializa como yyyy-MM-dd (JsonFormat das datas de ProgramacaoFerias).
type Data struct{ Momento }

func (d Data) MarshalJSON() ([]byte, error) { return json.Marshal(d.T.UTC().Format("2006-01-02")) }
func (d *Data) UnmarshalJSON(b []byte) error {
	m, err := parseMomento(b)
	if err != nil || m == nil {
		return err
	}
	d.Momento = *m
	return nil
}

// DataHora serializa como yyyy-MM-dd HH:mm:ss (histórico e período aquisitivo).
type DataHora struct{ Momento }

func (d DataHora) MarshalJSON() ([]byte, error) {
	return json.Marshal(d.T.UTC().Format("2006-01-02 15:04:05"))
}
func (d *DataHora) UnmarshalJSON(b []byte) error {
	m, err := parseMomento(b)
	if err != nil || m == nil {
		return err
	}
	d.Momento = *m
	return nil
}

// Instante serializa como o Jackson padrão do Spring Boot para java.util.Date.
type Instante struct{ Momento }

func (d Instante) MarshalJSON() ([]byte, error) {
	return json.Marshal(d.T.UTC().Format("2006-01-02T15:04:05.000") + "+00:00")
}
func (d *Instante) UnmarshalJSON(b []byte) error {
	m, err := parseMomento(b)
	if err != nil || m == nil {
		return err
	}
	d.Momento = *m
	return nil
}

func novaData(t time.Time) *Data         { return &Data{Momento{T: t.UTC()}} }
func novaDataHora(t time.Time) *DataHora { return &DataHora{Momento{T: t.UTC()}} }

// admissao converte uma coluna date em instante: meia-noite de Belém, como o Hibernate do legado.
func admissao(t time.Time) *Instante {
	return &Instante{Momento{T: time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, belem).UTC()}}
}

// totalDias espelha totalDeDiasEntreDatas: dias corridos (inclusive); 0 se faltar data ou fim < início.
func totalDias(inicio, fim *Data) int {
	if inicio == nil || fim == nil {
		return 0
	}
	a, b := inicio.Data(), fim.Data()
	if b.Before(a) {
		return 0
	}
	return int(b.Sub(a).Hours()/24+0.5) + 1
}
