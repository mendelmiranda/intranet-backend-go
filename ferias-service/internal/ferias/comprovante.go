package ferias

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"strings"
	"time"

	"github.com/go-pdf/fpdf"
)

// TipoComprovante distingue os documentos emitidos para o servidor.
type TipoComprovante int

const (
	ComprovanteAviso     TipoComprovante = iota // aviso de férias (pagamento realizado)
	ComprovanteAlteracao                        // aviso de alteração de período
	ComprovanteReversao                         // informação de reversão de pagamento
)

// PeriodoComprovante é um intervalo de férias exibido no documento.
type PeriodoComprovante struct {
	Rotulo   string // "1º período", "2º período"...
	Inicio   *Data
	Fim      *Data
	Situacao string // "Paga em 05/10/2026", "Programada"...
}

func (p PeriodoComprovante) dias() int { return totalDias(p.Inicio, p.Fim) }

// DadosComprovante reúne tudo o que o documento exibe; é independente do banco.
type DadosComprovante struct {
	Tipo      TipoComprovante
	Servidor  string
	Matricula int
	CPF       string
	Lotacao   string
	Admissao  *Instante
	Exercicio int

	Periodos        []PeriodoComprovante // aviso: períodos do exercício
	Anterior, Novo  *PeriodoComprovante  // alteração
	Observacao      string               // alteração e reversão
	AbonoPecuniario string
	AnteciparDecimo string
	Autorizado      string

	Emissao time.Time
}

const (
	pdfMargem  = 18.0
	pdfLargura = 210 - 2*pdfMargem
)

var (
	azulTCE   = [3]int{0, 92, 171}
	douradoTC = [3]int{184, 134, 11}
	cinza700  = [3]int{55, 65, 81}
	cinza500  = [3]int{107, 114, 128}
	cinza200  = [3]int{229, 231, 235}
	cinza50   = [3]int{249, 250, 251}
	azul50    = [3]int{239, 246, 255}
)

var meses = [...]string{"janeiro", "fevereiro", "março", "abril", "maio", "junho", "julho", "agosto",
	"setembro", "outubro", "novembro", "dezembro"}

func dataExtenso(t time.Time) string {
	t = t.In(belem)
	return fmt.Sprintf("%d de %s de %d", t.Day(), meses[t.Month()-1], t.Year())
}

func formatarCPF(cpf string) string {
	var d strings.Builder
	for _, r := range cpf {
		if r >= '0' && r <= '9' {
			d.WriteRune(r)
		}
	}
	s := d.String()
	if len(s) != 11 {
		return strings.TrimSpace(cpf)
	}
	return s[:3] + "." + s[3:6] + "." + s[6:9] + "-" + s[9:]
}

func simNaoDoc(v string) string {
	switch strings.ToUpper(strings.TrimSpace(v)) {
	case "S":
		return "Sim"
	case "A":
		return "Aguardando"
	}
	return "Não"
}

// CodigoControle identifica o conteúdo do documento (SHA-256 truncado). Não inclui a data de
// emissão: reemitir o mesmo comprovante gera o mesmo código.
func (d DadosComprovante) CodigoControle() string {
	h := sha256.New()
	fmt.Fprintf(h, "%d|%s|%d|%s|%d|%s|%s|%s", d.Tipo, strings.TrimSpace(d.CPF), d.Matricula, d.Servidor, d.Exercicio,
		d.Observacao, d.AbonoPecuniario, d.AnteciparDecimo)
	add := func(p *PeriodoComprovante) {
		if p != nil {
			fmt.Fprintf(h, "|%s|%s|%s|%s", p.Rotulo, fmtData(p.Inicio), fmtData(p.Fim), p.Situacao)
		}
	}
	for i := range d.Periodos {
		add(&d.Periodos[i])
	}
	add(d.Anterior)
	add(d.Novo)
	s := strings.ToUpper(fmt.Sprintf("%x", h.Sum(nil)))[:16]
	return s[0:4] + "-" + s[4:8] + "-" + s[8:12] + "-" + s[12:16]
}

func (d DadosComprovante) titulo() (titulo, subtitulo string) {
	switch d.Tipo {
	case ComprovanteAlteracao:
		return "AVISO DE ALTERAÇÃO DE FÉRIAS", fmt.Sprintf("Exercício %d", d.Exercicio)
	case ComprovanteReversao:
		return "INFORMAÇÃO DE REVERSÃO DE PAGAMENTO", "Férias"
	}
	return "AVISO DE FÉRIAS", fmt.Sprintf("Exercício %d", d.Exercicio)
}

func (d DadosComprovante) comunicado() string {
	switch d.Tipo {
	case ComprovanteAlteracao:
		return "Comunicamos a Vossa Senhoria que o período de férias programado foi alterado, passando a vigorar " +
			"conforme os períodos discriminados abaixo, em substituição ao anteriormente informado."
	case ComprovanteReversao:
		return "Informamos a Vossa Senhoria que o pagamento das férias referente ao período programado foi revertido " +
			"pela unidade responsável, conforme a justificativa registrada neste documento. Eventuais providências " +
			"decorrentes serão comunicadas oportunamente."
	}
	return fmt.Sprintf("Comunicamos a Vossa Senhoria que lhe serão concedidas férias relativas ao exercício de %d, "+
		"de acordo com a sua programação de férias, nos períodos discriminados abaixo.", d.Exercicio)
}

// GerarComprovante produz o PDF (A4, retrato) do documento.
func GerarComprovante(d DadosComprovante) ([]byte, error) {
	doc := fpdf.New("P", "mm", "A4", "")
	doc.SetMargins(pdfMargem, 14, pdfMargem)
	doc.SetAutoPageBreak(true, 22)
	doc.AliasNbPages("{nb}")
	doc.SetTitle("Aviso de Férias - "+d.Servidor, true)
	doc.SetAuthor("Tribunal de Contas do Estado do Amapá", true)
	doc.SetCreator("S3i - Sistema Corporativo", true)
	tr := doc.UnicodeTranslatorFromDescriptor("")
	codigo := d.CodigoControle()

	cor := func(c [3]int) { doc.SetTextColor(c[0], c[1], c[2]) }
	preenche := func(c [3]int) { doc.SetFillColor(c[0], c[1], c[2]) }
	traco := func(c [3]int) { doc.SetDrawColor(c[0], c[1], c[2]) }

	doc.SetFooterFunc(func() {
		doc.SetY(-17)
		traco(cinza200)
		doc.SetLineWidth(0.3)
		doc.Line(pdfMargem, doc.GetY(), 210-pdfMargem, doc.GetY())
		doc.Ln(1.5)
		doc.SetFont("Helvetica", "", 7)
		cor(cinza500)
		doc.CellFormat(pdfLargura/3, 4, tr("Documento gerado eletronicamente pelo S3i — TCE/AP"), "", 0, "L", false, 0, "")
		doc.CellFormat(pdfLargura/3, 4, tr(fmt.Sprintf("Página %d de {nb}", doc.PageNo())), "", 0, "C", false, 0, "")
		doc.CellFormat(pdfLargura/3, 4, tr("Código de controle: "+codigo), "", 0, "R", false, 0, "")
	})
	doc.AddPage()

	// --- cabeçalho institucional ---
	logoOpts := fpdf.ImageOptions{ImageType: "PNG", ReadDpi: false}
	doc.RegisterImageOptionsReader("logo", logoOpts, bytes.NewReader(logoPNG))
	const larguraLogo = 62.0
	doc.ImageOptions("logo", (210-larguraLogo)/2, 12, larguraLogo, 0, false, logoOpts, 0, "")
	doc.SetY(12 + larguraLogo*1079/4856 + 4)
	doc.Ln(1)
	traco(douradoTC)
	doc.SetLineWidth(0.7)
	doc.Line(pdfMargem, doc.GetY(), 210-pdfMargem, doc.GetY())
	doc.Ln(7)

	titulo, subtitulo := d.titulo()
	doc.SetFont("Helvetica", "B", 16)
	cor(cinza700)
	doc.CellFormat(pdfLargura, 8, tr(titulo), "", 1, "C", false, 0, "")
	doc.SetFont("Helvetica", "", 10)
	cor(cinza500)
	doc.CellFormat(pdfLargura, 5, tr(subtitulo), "", 1, "C", false, 0, "")
	doc.Ln(7)

	// --- blocos ---
	secao := func(texto string) {
		preenche(azulTCE)
		doc.SetFont("Helvetica", "B", 8)
		doc.SetTextColor(255, 255, 255)
		doc.CellFormat(pdfLargura, 6, "  "+tr(strings.ToUpper(texto)), "", 1, "L", true, 0, "")
	}
	campo := func(x, y, w float64, rotulo, valor string) {
		traco(cinza200)
		doc.SetLineWidth(0.25)
		preenche(cinza50)
		doc.Rect(x, y, w, 12, "FD")
		doc.SetFont("Helvetica", "", 6.5)
		cor(cinza500)
		doc.SetXY(x+2, y+1.4)
		doc.CellFormat(w-4, 3.2, tr(strings.ToUpper(rotulo)), "", 0, "L", false, 0, "")
		doc.SetFont("Helvetica", "B", 9.5)
		cor(cinza700)
		doc.SetXY(x+2, y+5.2)
		doc.CellFormat(w-4, 5, tr(corta(doc, valor, w-4)), "", 0, "L", false, 0, "")
	}
	linhaCampos := func(rotulos, valores []string, larguras []float64) {
		y := doc.GetY()
		x := pdfMargem
		for i := range rotulos {
			campo(x, y, larguras[i], rotulos[i], valores[i])
			x += larguras[i]
		}
		doc.SetXY(pdfMargem, y+12)
	}

	secao("Identificação do servidor(a)")
	adm := "-"
	if d.Admissao != nil {
		adm = d.Admissao.T.In(belem).Format("02/01/2006")
	}
	nome := strings.TrimSpace(d.Servidor)
	if nome == "" {
		nome = "-"
	}
	lot := strings.TrimSpace(d.Lotacao)
	if lot == "" {
		lot = "-"
	}
	linhaCampos([]string{"Servidor(a)", "Matrícula"}, []string{nome, fmt.Sprint(d.Matricula)}, []float64{pdfLargura * 0.75, pdfLargura * 0.25})
	linhaCampos([]string{"CPF", "Data de admissão"}, []string{formatarCPF(d.CPF), adm}, []float64{pdfLargura * 0.5, pdfLargura * 0.5})
	linhaCampos([]string{"Lotação"}, []string{lot}, []float64{pdfLargura})
	doc.Ln(6)

	secao("Comunicado")
	doc.Ln(2)
	doc.SetFont("Helvetica", "", 10)
	cor(cinza700)
	doc.SetX(pdfMargem)
	doc.MultiCell(pdfLargura, 5.4, tr(d.comunicado()), "", "J", false)
	doc.Ln(5)

	// --- tabela de períodos ---
	larg := []float64{34, 30, 30, 20, pdfLargura - 114}
	cab := func(titulos ...string) {
		preenche(azul50)
		traco(cinza200)
		doc.SetFont("Helvetica", "B", 8)
		cor(azulTCE)
		for i, t := range titulos {
			doc.CellFormat(larg[i], 7, tr(t), "1", 0, "C", true, 0, "")
		}
		doc.Ln(-1)
	}
	linha := func(p PeriodoComprovante, destaque bool) {
		doc.SetFont("Helvetica", "", 9.5)
		cor(cinza700)
		traco(cinza200)
		preenche(cinza50)
		doc.CellFormat(larg[0], 8, tr(p.Rotulo), "1", 0, "C", destaque, 0, "")
		doc.CellFormat(larg[1], 8, dataBR(p.Inicio), "1", 0, "C", destaque, 0, "")
		doc.CellFormat(larg[2], 8, dataBR(p.Fim), "1", 0, "C", destaque, 0, "")
		doc.CellFormat(larg[3], 8, fmt.Sprint(p.dias()), "1", 0, "C", destaque, 0, "")
		doc.CellFormat(larg[4], 8, tr(p.Situacao), "1", 1, "C", destaque, 0, "")
	}

	switch d.Tipo {
	case ComprovanteAlteracao:
		secao("Períodos")
		doc.Ln(2)
		cab("Situação", "Início", "Término", "Dias", "Exercício")
		for _, p := range []*PeriodoComprovante{d.Anterior, d.Novo} {
			if p != nil {
				linha(*p, p == d.Novo)
			}
		}
	default:
		secao("Períodos de férias")
		doc.Ln(2)
		cab("Período", "Início", "Término", "Dias", "Situação")
		total := 0
		for _, p := range d.Periodos {
			linha(p, false)
			total += p.dias()
		}
		if len(d.Periodos) == 0 {
			doc.SetFont("Helvetica", "I", 9)
			cor(cinza500)
			doc.CellFormat(pdfLargura, 8, tr("Nenhum período programado."), "1", 1, "C", false, 0, "")
		} else {
			doc.SetFont("Helvetica", "B", 9.5)
			cor(cinza700)
			doc.CellFormat(larg[0]+larg[1]+larg[2], 8, "Total de dias", "1", 0, "R", true, 0, "")
			doc.CellFormat(larg[3], 8, fmt.Sprint(total), "1", 0, "C", true, 0, "")
			doc.CellFormat(larg[4], 8, "", "1", 1, "C", true, 0, "")
		}
	}
	doc.Ln(6)

	if d.Tipo == ComprovanteAviso {
		secao("Opções do servidor(a)")
		linhaCampos([]string{"Abono pecuniário", "Antecipação da gratificação natalina", "Autorizado pela chefia"},
			[]string{simNaoDoc(d.AbonoPecuniario), simNaoDoc(d.AnteciparDecimo), simNaoDoc(d.Autorizado)},
			[]float64{pdfLargura * 0.28, pdfLargura * 0.44, pdfLargura * 0.28})
		doc.Ln(6)
	}

	if obs := strings.TrimSpace(d.Observacao); obs != "" {
		secao("Observação")
		doc.Ln(2)
		doc.SetFont("Helvetica", "", 10)
		cor(cinza700)
		doc.SetX(pdfMargem)
		doc.MultiCell(pdfLargura, 5.4, tr(obs), "", "J", false)
		doc.Ln(5)
	}

	// --- fecho ---
	if doc.GetY() > 235 {
		doc.AddPage()
	}
	doc.Ln(4)
	doc.SetFont("Helvetica", "", 10)
	cor(cinza700)
	doc.CellFormat(pdfLargura, 6, tr("Macapá-AP, "+dataExtenso(d.Emissao)+"."), "", 1, "R", false, 0, "")
	doc.Ln(8)
	traco(douradoTC)
	doc.SetLineWidth(0.5)
	doc.Line(pdfMargem, doc.GetY(), 210-pdfMargem, doc.GetY())
	doc.Ln(2)
	doc.SetFont("Helvetica", "", 7.5)
	cor(cinza500)
	doc.MultiCell(pdfLargura, 3.8, tr("Este documento foi emitido eletronicamente pelo Sistema Corporativo S3i do Tribunal de Contas "+
		"do Estado do Amapá. O código de controle acima identifica o seu conteúdo e deve ser informado em caso de "+
		"conferência junto à unidade responsável."), "", "C", false)

	var buf bytes.Buffer
	if err := doc.Output(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
