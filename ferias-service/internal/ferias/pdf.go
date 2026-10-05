package ferias

import (
	"bytes"
	_ "embed"
	"fmt"
	"strconv"

	"github.com/go-pdf/fpdf"
)

//go:embed logo.png
var logoPNG []byte

// colunas do relatório (mm, A4 paisagem com margens de 15 mm).
var colunasRelatorio = []struct {
	titulo  string
	largura float64
}{
	{"Nome", 75}, {"Matricula", 22}, {"Per. Aquisitivo", 25}, {"Periodo", 22}, {"Inicio", 24},
	{"Fim", 24}, {"Ferias Paga", 25}, {"Ant. Decimo", 25}, {"Autorizado", 25},
}

const (
	margemPDF   = 15.0
	alturaLinha = 7.0
)

func simNao(v string) string {
	if v == "S" {
		return "SIM"
	}
	return "NAO"
}

func dataBR(d *Data) string {
	if d == nil {
		return ""
	}
	return d.T.UTC().Format("02/01/2006")
}

// gerarRelatorioPDF monta o relatório de programação de férias (equivalente ao PdfRelatorioService).
func gerarRelatorioPDF(lista []Programacao) ([]byte, error) {
	ano := ""
	if len(lista) > 0 {
		ano = strconv.Itoa(lista[0].Ano)
	}

	doc := fpdf.New("L", "mm", "A4", "")
	doc.SetMargins(margemPDF, 12, margemPDF)
	doc.SetAutoPageBreak(false, 15)
	tr := doc.UnicodeTranslatorFromDescriptor("")

	pagina := 0
	var y float64
	novaPagina := func() {
		doc.AddPage()
		pagina++
		y = 12
		if pagina == 1 {
			opts := fpdf.ImageOptions{ImageType: "PNG", ReadDpi: false}
			doc.RegisterImageOptionsReader("logo", opts, bytes.NewReader(logoPNG))
			doc.ImageOptions("logo", 123, y, 50, 0, false, opts, 0, "")
			y += 22
			doc.SetFont("Helvetica", "B", 14)
			doc.SetXY(margemPDF, y)
			doc.CellFormat(267, 8, tr("Relatorio de Programacao de Ferias - "+ano), "", 0, "C", false, 0, "")
			y += 12
		}
		doc.SetFillColor(30, 58, 138)
		doc.SetTextColor(255, 255, 255)
		doc.SetFont("Helvetica", "B", 8)
		x := margemPDF
		for _, c := range colunasRelatorio {
			doc.SetXY(x, y)
			doc.CellFormat(c.largura, alturaLinha, c.titulo, "1", 0, "C", true, 0, "")
			x += c.largura
		}
		y += alturaLinha
		doc.SetTextColor(0, 0, 0)
	}

	novaPagina()
	for _, f := range lista {
		if y+alturaLinha > 195 {
			novaPagina()
		}
		paga, decimo := "", f.AnteciparDecimo
		if f.FeriasPaga != nil {
			paga = *f.FeriasPaga
		}
		periodo := "Segundo"
		if f.Periodo == "P" {
			periodo = "Primeiro"
		}
		valores := []string{
			f.NomeFuncionario, strconv.Itoa(f.Matricula), strconv.Itoa(f.Ano), periodo,
			dataBR(f.DataInicio), dataBR(f.DataFim), simNao(paga), simNao(decimo), simNao(f.Autorizado),
		}
		doc.SetFont("Helvetica", "", 8)
		x := margemPDF
		for i, c := range colunasRelatorio {
			alinha := "C"
			if i == 0 {
				alinha = "L"
			}
			doc.SetXY(x, y)
			doc.CellFormat(c.largura, alturaLinha, tr(corta(doc, valores[i], c.largura-2)), "1", 0, alinha, false, 0, "")
			x += c.largura
		}
		y += alturaLinha
	}

	for i := 1; i <= doc.PageNo(); i++ {
		doc.SetPage(i)
		doc.SetFont("Helvetica", "", 8)
		doc.SetXY(margemPDF, 200)
		doc.CellFormat(267, 5, fmt.Sprintf("Pagina %d", i), "", 0, "C", false, 0, "")
	}

	var buf bytes.Buffer
	if err := doc.Output(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// corta reduz o texto até caber na largura (mm), como o fitText do legado.
func corta(doc *fpdf.Fpdf, s string, largura float64) string {
	r := []rune(s)
	for len(r) > 0 && doc.GetStringWidth(string(r)) > largura {
		r = r[:len(r)-1]
	}
	return string(r)
}
