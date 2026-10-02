package contracheque

import (
	"bytes"
	_ "embed"
	"fmt"
	"math"
	"strings"
	"time"
	"unicode"

	"github.com/go-pdf/fpdf"
	qrcode "github.com/skip2/go-qrcode"
)

//go:embed assets/logo.png
var logoPNG []byte

type rgb struct{ r, g, b int }

var (
	slate300 = rgb{203, 213, 225}
	slate600 = rgb{71, 85, 105}
	textCol  = rgb{15, 23, 42}
	black    = rgb{0, 0, 0}
	blueDark = rgb{23, 37, 84}
)

// Layout em pontos, idêntico ao HeaderECards (A4, origem no canto inferior esquerdo).
const (
	pageW     = 595.2756
	pageH     = 841.8898
	margin    = 36.0
	contentW  = pageW - 2*margin
	lineH     = 14.0
	cellPad   = 5.0
	fsTitle   = 14.0
	fsSection = 11.0
	fsNormal  = 9.0
	fsBold    = 9.0
	fsNote    = 8.0
	fsValue   = 10.0
	fsLiquido = 11.0

	validacaoHeight = 140.0
)

type pdfInput struct {
	Mes, Ano, Matricula int
	Header              Header
	Linhas              []Linha
	Codigo, URL         string
	Emissao             time.Time
}

type canvas struct {
	pdf *fpdf.Fpdf
	tr  func(string) string
	n   int // contador para nomes de imagens
}

func renderPDF(in pdfInput) ([]byte, error) {
	doc := fpdf.NewCustom(&fpdf.InitType{UnitStr: "pt", Size: fpdf.SizeType{Wd: pageW, Ht: pageH}})
	doc.SetAutoPageBreak(false, 0)
	doc.SetMargins(0, 0, 0)
	doc.SetLineWidth(0.7)
	doc.AddPage()
	c := &canvas{pdf: doc, tr: doc.UnicodeTranslatorFromDescriptor("")}

	y := pageH - margin
	y = c.header(in, y)
	y -= 4
	c.hrule(margin, y, contentW, black)
	y -= 8

	h := in.Header
	y = c.cards(
		[][2]string{{"Matrícula:", fmt.Sprint(h.Matricula)}, {"Nome:", h.Nome}, {"CPF:", formatarCPF(deref(h.CPF))}},
		[][2]string{{"Banco:", deref(h.Banco)}, {"Agência:", deref(h.Agencia)}, {"Conta:", deref(h.Conta)}},
		[][2]string{{"Lotação:", deref(h.Lotacao)}, {"Cargo:", h.Cargo}, {"Admissão:", formatarData(h.Admissao)}},
		y)

	y -= 12
	y = c.text("Detalhes do Contracheque", true, fsSection, margin, y, black)
	y -= 4

	baseIRRF := ""
	for _, l := range in.Linhas {
		if strings.EqualFold(strings.TrimSpace(l.TipoEvento), "F") &&
			strings.EqualFold(strings.TrimSpace(l.Descricao), "BASE IRRF (FOLHA)") {
			baseIRRF = brl(math.Abs(l.Valor))
			break
		}
	}
	if baseIRRF == "" {
		baseIRRF = "R$ 0,00"
	}

	var linhasNF []Linha
	for _, l := range in.Linhas {
		if !strings.EqualFold(strings.TrimSpace(l.TipoEvento), "F") {
			linhasNF = append(linhasNF, l)
		}
	}

	cols := []float64{contentW * 0.45, contentW * 0.15, contentW * 0.20, contentW * 0.20}
	headers := []string{"Descrição", "Quantidade", "Proventos", "Descontos"}
	y = c.tableHeader(headers, cols, margin, y)

	// Valores acumulados em centavos para evitar erro de ponto flutuante (BigDecimal no legado).
	var totalProv, totalDesc int64
	for _, l := range linhasNF {
		tipo := strings.ToUpper(strings.TrimSpace(l.TipoEvento))
		desconto := l.Valor < 0 || strings.HasPrefix(tipo, "D") || strings.Contains(tipo, "DESC")
		cents := int64(math.Round(math.Abs(l.Valor) * 100))

		var prov, desc string
		if desconto {
			desc = brlCents(cents)
			totalDesc += cents
		} else {
			prov = brlCents(cents)
			totalProv += cents
		}

		if y-lineH-cellPad*2 < margin+60 {
			y = c.newPage()
			y = c.tableHeader(headers, cols, margin, y)
		}
		y = c.tableRow([]string{l.Descricao, l.Quantidade, prov, desc}, cols, margin, y)
	}

	liquido := totalProv - totalDesc
	y = c.totalsRow("Totais", brlCents(totalProv), brlCents(totalDesc), cols, margin, y)
	y = c.liquidoRow("Líquido a Receber", brlCents(liquido), cols, margin, y)

	y -= 14
	y = c.text(fmt.Sprintf("Valores calculados automaticamente %d itens", len(linhasNF)), false, fsNormal, margin, y, black)

	if y-120 < margin {
		y = c.newPage()
	}
	y = c.totais(brlCents(totalProv), brlCents(totalDesc), brlCents(liquido), baseIRRF, y)

	if y-validacaoHeight < margin {
		y = c.newPage()
	}
	y -= 20
	if err := c.validacao(in, y); err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	if err := doc.Output(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func (c *canvas) newPage() float64 {
	c.pdf.AddPage()
	c.pdf.SetLineWidth(0.7)
	return pageH - margin
}

// ── cabeçalho ───────────────────────────────────────────────────────────────

func (c *canvas) header(in pdfInput, y float64) float64 {
	const rowH = 50.0
	third := contentW / 3

	const imgW, imgH = 140.0, 40.0
	if cfg, err := pngSize(logoPNG); err == nil {
		scale := math.Min(imgW/float64(cfg.w), imgH/float64(cfg.h))
		c.image("logo", logoPNG, margin, pageH-(y-imgH)-float64(cfg.h)*scale, float64(cfg.w)*scale, float64(cfg.h)*scale)
	} else {
		c.text("TCE-AP", true, fsBold, margin, y-12, black)
	}

	title := fmt.Sprintf("Resumo do mês %d/%d", in.Mes, in.Ano)
	tx := margin + third + (third-c.width(title, true, fsTitle))/2
	c.text(title, true, fsTitle, tx, y-20, textCol)

	mx, my := margin+2*third, y-10
	my = c.labelValue("CNPJ: ", "34.870.246/0001-36", mx, my)
	my = c.labelValue("Endereço: ", "Av. FAB 900, Macapá - AP", mx, my)
	my = c.labelValue("Ano: ", fmt.Sprint(in.Ano), mx, my)
	c.labelValue("Mês: ", fmt.Sprint(in.Mes), mx, my)
	return y - rowH
}

// ── cards ───────────────────────────────────────────────────────────────────

func (c *canvas) cards(pessoais, bancarios, funcionais [][2]string, y float64) float64 {
	y -= 8
	const cardPad, lh = 8.0, 12.0
	cardW := contentW/3 - 4
	maxW := cardW - cardPad*2

	cardH := math.Max(c.cardHeight(pessoais, maxW, lh, cardPad),
		math.Max(c.cardHeight(bancarios, maxW, lh, cardPad), c.cardHeight(funcionais, maxW, lh, cardPad)))

	c.card("DADOS PESSOAIS", pessoais, margin, y, cardW, cardH, cardPad, lh, maxW)
	c.card("DADOS BANCÁRIOS", bancarios, margin+contentW/3, y, cardW, cardH, cardPad, lh, maxW)
	c.card("DADOS FUNCIONAIS", funcionais, margin+2*contentW/3, y, cardW, cardH, cardPad, lh, maxW)
	return y - cardH - 8
}

func (c *canvas) cardHeight(itens [][2]string, maxW, lh, pad float64) float64 {
	total := pad + lh
	for _, kv := range itens {
		label := kv[0] + " "
		total += lh
		if !c.cabeInline(label, kv[1], maxW) {
			total += float64(len(c.wrap(kv[1], fsNormal, maxW))) * lh
		}
	}
	return total + pad
}

func (c *canvas) card(titulo string, itens [][2]string, x, y, w, h, pad, lh, maxW float64) {
	c.rect(x, y-h, w, h, black, false)
	ty := y - pad - lh
	c.text(titulo, true, fsBold, x+pad, ty, blueDark)
	ty -= lh
	for _, kv := range itens {
		label := kv[0] + " "
		if c.cabeInline(label, kv[1], maxW) {
			c.text(label, true, fsBold, x+pad, ty, black)
			c.text(kv[1], false, fsNormal, x+pad+c.width(label, true, fsBold), ty, textCol)
			ty -= lh
			continue
		}
		c.text(label, true, fsBold, x+pad, ty, black)
		ty -= lh
		for _, line := range c.wrap(kv[1], fsNormal, maxW) {
			c.text(line, false, fsNormal, x+pad, ty, textCol)
			ty -= lh
		}
	}
}

// cabeInline informa se o valor cabe na mesma linha do rótulo.
func (c *canvas) cabeInline(label, value string, maxW float64) bool {
	avail := maxW - c.width(label, true, fsBold)
	return avail > 0 && c.width(value, false, fsNormal) <= avail
}

func (c *canvas) wrap(text string, size, maxW float64) []string {
	if text == "" {
		return []string{""}
	}
	var lines []string
	cur := ""
	flushWord := func(word string) {
		if c.width(word, false, size) > maxW {
			lines = append(lines, c.wrapByChars(word, size, maxW)...)
			return
		}
		cur = word
	}
	for _, word := range strings.Split(sanitize(text), " ") {
		if cur == "" {
			flushWord(word)
			continue
		}
		if c.width(cur+" "+word, false, size) <= maxW {
			cur += " " + word
			continue
		}
		lines = append(lines, cur)
		cur = ""
		flushWord(word)
	}
	if cur != "" {
		lines = append(lines, cur)
	}
	if len(lines) == 0 {
		lines = []string{""}
	}
	return lines
}

func (c *canvas) wrapByChars(word string, size, maxW float64) []string {
	var lines []string
	cur := ""
	for _, r := range word {
		if c.width(cur+string(r), false, size) > maxW && cur != "" {
			lines = append(lines, cur)
			cur = ""
		}
		cur += string(r)
	}
	if cur != "" {
		lines = append(lines, cur)
	}
	return lines
}

// ── tabela ──────────────────────────────────────────────────────────────────

func (c *canvas) tableHeader(headers []string, cols []float64, startX, y float64) float64 {
	x := startX
	rowH := lineH + cellPad*2
	for i, h := range headers {
		c.rect(x, y-rowH, cols[i], rowH, slate300, true)
		tx := x + cellPad
		if i > 0 {
			tx = x + cols[i] - cellPad - c.width(h, true, fsBold)
		}
		c.text(h, true, fsBold, tx, y-lineH-cellPad+2, blueDark)
		x += cols[i]
	}
	return y - rowH
}

func (c *canvas) tableRow(cells []string, cols []float64, startX, y float64) float64 {
	x := startX
	rowH := lineH + cellPad*2
	for i, cell := range cells {
		c.rect(x, y-rowH, cols[i], rowH, black, false)
		tx := x + cellPad
		if i >= 2 {
			tx = x + cols[i] - cellPad - c.width(cell, false, fsNormal)
		}
		c.text(cell, false, fsNormal, tx, y-lineH-cellPad+2, textCol)
		x += cols[i]
	}
	return y - rowH
}

func (c *canvas) totalsRow(label, prov, desc string, cols []float64, startX, y float64) float64 {
	rowH := lineH + cellPad*2
	span := cols[0] + cols[1]
	base := y - lineH - cellPad + 2

	c.rect(startX, y-rowH, span, rowH, black, false)
	c.text(label, true, fsValue, startX+cellPad, base, black)

	x2 := startX + span
	c.rect(x2, y-rowH, cols[2], rowH, black, false)
	c.text(prov, true, fsValue, x2+cols[2]-cellPad-c.width(prov, true, fsValue), base, black)

	x3 := x2 + cols[2]
	c.rect(x3, y-rowH, cols[3], rowH, black, false)
	c.text(desc, true, fsValue, x3+cols[3]-cellPad-c.width(desc, true, fsValue), base, black)
	return y - rowH
}

func (c *canvas) liquidoRow(label, liquido string, cols []float64, startX, y float64) float64 {
	rowH := lineH + cellPad*2
	span := cols[0] + cols[1] + cols[2]
	base := y - lineH - cellPad + 2

	c.rect(startX, y-rowH, span, rowH, black, false)
	c.text(label, true, fsValue, startX+cellPad, base, black)

	x3 := startX + span
	c.rect(x3, y-rowH, cols[3], rowH, black, false)
	c.text(liquido, true, fsLiquido, x3+cols[3]-cellPad-c.width(liquido, true, fsLiquido), base, black)
	return y - rowH
}

// ── totais ──────────────────────────────────────────────────────────────────

func (c *canvas) totais(vantagens, descontos, liquido, baseIRRF string, y float64) float64 {
	y -= 12
	c.text("Totais do Contracheque", true, fsSection, margin, y, blueDark)
	y -= 10

	const pad = 12.0
	innerH := lineH*6 + pad*2 + 16
	c.rect(margin, y-innerH, contentW, innerH, black, false)

	ty := y - pad - lineH
	right := margin + contentW - pad

	c.text("TOTAL VANTAGENS", true, fsBold, margin+pad, ty, slate600)
	c.text(vantagens, false, fsValue, right-c.width(vantagens, false, fsValue), ty, black)
	ty -= lineH + 2

	c.text("TOTAL DESCONTOS", true, fsBold, margin+pad, ty, slate600)
	c.text(descontos, false, fsValue, right-c.width(descontos, false, fsValue), ty, black)
	ty -= lineH + 2

	c.text("LÍQUIDO A RECEBER", true, fsBold, margin+pad, ty, slate600)
	c.text(liquido, true, fsLiquido, right-c.width(liquido, true, fsLiquido), ty, black)
	ty -= lineH + 8

	c.hrule(margin+pad, ty, contentW-pad*2, black)
	ty -= 10

	c.text("BASE P/ IRRF", true, fsBold, margin+pad, ty, slate600)
	c.text(baseIRRF, false, fsValue, right-c.width(baseIRRF, false, fsValue), ty, black)
	ty -= lineH + 10

	c.text("Valores referentes ao mês selecionado.", false, fsNote, margin+pad, ty, black)
	return y - innerH - 8
}

// ── validação (QR Code) ─────────────────────────────────────────────────────

func (c *canvas) validacao(in pdfInput, top float64) error {
	c.text("VALIDAÇÃO", true, 11, margin, top, blueDark)
	under := top - 3
	c.pdf.SetDrawColor(blueDark.r, blueDark.g, blueDark.b)
	c.pdf.SetLineWidth(1)
	c.pdf.Line(margin, pageH-under, margin+c.width("VALIDAÇÃO", true, 11), pageH-under)
	top -= 18

	const blockH = 90.0
	c.pdf.SetDrawColor(slate300.r, slate300.g, slate300.b)
	c.pdf.SetLineWidth(0.7)
	c.pdf.Rect(margin, pageH-top, contentW, blockH, "D")

	const qrSize, qrPad = 70.0, 10.0
	png, err := qrcode.Encode(in.URL, qrcode.Medium, 200)
	if err != nil {
		return fmt.Errorf("gerar QR Code: %w", err)
	}
	c.image("qr", png, margin+qrPad, pageH-(top-blockH+(blockH-qrSize)/2)-qrSize, qrSize, qrSize)

	tx := margin + qrPad + qrSize + 16
	ty := top - 20
	label := "Código de Verificação:  "
	c.text(label, true, 10, tx, ty, black)
	c.text(in.Codigo, true, 10, tx+c.width(label, true, 10), ty, black)
	ty -= 18
	c.text("Documento emitido em "+in.Emissao.Format("02/01/2006 15:04"), false, 8, tx, ty, slate600)
	return nil
}

// ── primitivos (coordenadas PDF: origem embaixo à esquerda) ─────────────────

func (c *canvas) setFont(bold bool, size float64) {
	style := ""
	if bold {
		style = "B"
	}
	c.pdf.SetFont("Helvetica", style, size)
}

func (c *canvas) width(s string, bold bool, size float64) float64 {
	if s == "" {
		return 0
	}
	c.setFont(bold, size)
	return c.pdf.GetStringWidth(c.tr(sanitize(s)))
}

// text desenha na linha de base y e devolve a próxima posição vertical, como o legado.
func (c *canvas) text(s string, bold bool, size, x, y float64, col rgb) float64 {
	if s == "" {
		return y
	}
	c.setFont(bold, size)
	c.pdf.SetTextColor(col.r, col.g, col.b)
	c.pdf.Text(x, pageH-y, c.tr(sanitize(s)))
	return y - size - 2
}

func (c *canvas) labelValue(label, value string, x, y float64) float64 {
	c.text(label, true, fsBold, x, y, black)
	c.text(value, false, fsNormal, x+c.width(label, true, fsBold), y, textCol)
	return y - lineH
}

func (c *canvas) hrule(x, y, w float64, col rgb) {
	c.pdf.SetDrawColor(col.r, col.g, col.b)
	c.pdf.SetLineWidth(0.7)
	c.pdf.Line(x, pageH-y, x+w, pageH-y)
}

// rect: (x, y) é o canto inferior esquerdo.
func (c *canvas) rect(x, y, w, h float64, col rgb, fill bool) {
	c.pdf.SetDrawColor(0, 0, 0)
	c.pdf.SetLineWidth(0.7)
	style := "D"
	if fill {
		c.pdf.SetFillColor(col.r, col.g, col.b)
		style = "FD"
	} else {
		c.pdf.SetDrawColor(col.r, col.g, col.b)
	}
	c.pdf.Rect(x, pageH-y-h, w, h, style)
}

// image: (x, topY) em coordenadas fpdf (origem no topo).
func (c *canvas) image(name string, data []byte, x, topY, w, h float64) {
	c.n++
	key := fmt.Sprintf("%s-%d", name, c.n)
	c.pdf.RegisterImageOptionsReader(key, fpdf.ImageOptions{ImageType: "PNG"}, bytes.NewReader(data))
	c.pdf.ImageOptions(key, x, topY, w, h, false, fpdf.ImageOptions{ImageType: "PNG"}, 0, "")
}

// ── utilitários ─────────────────────────────────────────────────────────────

func sanitize(s string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return -1
		}
		return r
	}, s)
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func formatarData(d *Data) string {
	if d == nil {
		return ""
	}
	return d.Time.Format("02/01/2006")
}

// formatarCPF aplica a máscara 000.000.000-00 (Util.formatarContraCheque).
func formatarCPF(cpf string) string {
	var digits strings.Builder
	for _, r := range cpf {
		if r >= '0' && r <= '9' {
			digits.WriteRune(r)
		}
	}
	d := digits.String()
	if len(d) != 11 {
		return cpf
	}
	return d[0:3] + "." + d[3:6] + "." + d[6:9] + "-" + d[9:11]
}

func brl(v float64) string { return brlCents(int64(math.Round(v * 100))) }

// brlCents formata como NumberFormat pt-BR: "R$ 1.234,56".
func brlCents(cents int64) string {
	sign := ""
	if cents < 0 {
		sign, cents = "-", -cents
	}
	inteiro := fmt.Sprint(cents / 100)
	var grupos []string
	for len(inteiro) > 3 {
		grupos = append([]string{inteiro[len(inteiro)-3:]}, grupos...)
		inteiro = inteiro[:len(inteiro)-3]
	}
	grupos = append([]string{inteiro}, grupos...)
	return fmt.Sprintf("%sR$ %s,%02d", sign, strings.Join(grupos, "."), cents%100)
}
