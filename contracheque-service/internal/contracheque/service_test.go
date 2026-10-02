package contracheque

import (
	"regexp"
	"testing"
)

func TestGerarCodigoFormato(t *testing.T) {
	re := regexp.MustCompile(`^([0-9A-F]{1,5}\.){4}[0-9A-F]{1,5}$`)
	for i := 0; i < 200; i++ {
		c, err := GerarCodigo()
		if err != nil || !re.MatchString(c) {
			t.Fatalf("código inválido %q (%v)", c, err)
		}
	}
}

func TestBRL(t *testing.T) {
	cases := map[int64]string{0: "R$ 0,00", 5: "R$ 0,05", 123456: "R$ 1.234,56", 100000000: "R$ 1.000.000,00", -250: "-R$ 2,50"}
	for in, want := range cases {
		if got := brlCents(in); got != want {
			t.Errorf("brlCents(%d)=%q, esperado %q", in, got, want)
		}
	}
}

func TestFormatarCPF(t *testing.T) {
	if got := formatarCPF("12345678901"); got != "123.456.789-01" {
		t.Error(got)
	}
	if got := formatarCPF("123"); got != "123" {
		t.Error(got)
	}
}

func TestInteiroAceitaTextoENumero(t *testing.T) {
	var r ConsultaRequest
	if err := jsonUnmarshal(`{"mes":"03","ano":2024,"matricula":"7","matriculaSistema":null}`, &r); err != nil {
		t.Fatal(err)
	}
	if r.Mes != 3 || r.Ano != 2024 || r.Matricula != 7 || r.MatriculaSistema != 0 {
		t.Fatalf("%+v", r)
	}
	if err := jsonUnmarshal(`{"mes":"abc"}`, &r); err == nil {
		t.Fatal("esperava erro para texto não numérico")
	}
}
