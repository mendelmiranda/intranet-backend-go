package contracheque

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"
)

const requestTimeout = 20 * time.Second

type Handler struct {
	svc      *Service
	location *time.Location
}

func NewHandler(svc *Service, location *time.Location) *Handler {
	return &Handler{svc: svc, location: location}
}

// RegisterProtected registra as rotas de /api/contra-cheque.
// As rotas estáticas vêm antes das parametrizadas para não serem capturadas por
// /:mes/:ano/:matricula.
func (h *Handler) RegisterProtected(r fiber.Router) {
	r.Get("/informacao", h.informacao)
	r.Get("/meses/:ano", h.mesesDoAno)
	r.Get("/meses-ecidade/:ano/:matricula", h.mesesEcidade)
	r.Get("/intranet/mes/:mes/ano/:ano/matricula/:matricula", h.intranet)
	r.Post("/gerar", h.gerar)
	r.Post("/decimo", h.decimo)
	r.Post("/", h.pesquisar)
	r.Get("/:mes/:ano/:matricula", h.doMes)
}

// RegisterPublic registra a verificação de autenticidade (permitAll no legado).
func (h *Handler) RegisterPublic(app *fiber.App) {
	app.Get("/verifica-contracheque/:codigo", h.verificar)
}

func (h *Handler) informacao(c fiber.Ctx) error {
	return c.JSON(NovoContraCheque())
}

func (h *Handler) doMes(c fiber.Ctx) error {
	mes, ano, matricula, err := paramsInt3(c, "mes", "ano", "matricula")
	if err != nil {
		return h.erro(c, http.StatusBadRequest, err.Error())
	}
	ctx, cancel := context.WithTimeout(c.Context(), requestTimeout)
	defer cancel()
	rubricas, err := h.svc.Consulta(ctx, mes, ano, matricula)
	if err != nil {
		return h.tratar(c, err)
	}
	return c.JSON(rubricas)
}

func (h *Handler) pesquisar(c fiber.Ctx) error {
	req, err := h.corpo(c)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(c.Context(), requestTimeout)
	defer cancel()
	rubricas, err := h.svc.Consulta(ctx, int(req.Mes), int(req.Ano), int(req.MatriculaSistema))
	if err != nil {
		return h.tratar(c, err)
	}
	return c.JSON(OrdenadasSemRepeticao(rubricas))
}

func (h *Handler) decimo(c fiber.Ctx) error {
	req, err := h.corpo(c)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(c.Context(), requestTimeout)
	defer cancel()
	rubricas, err := h.svc.ConsultaDecimo(ctx, int(req.Mes), int(req.Ano), int(req.MatriculaSistema))
	if err != nil {
		return h.tratar(c, err)
	}
	return c.JSON(OrdenadasSemRepeticao(rubricas))
}

func (h *Handler) mesesDoAno(c fiber.Ctx) error {
	ano, err := strconv.Atoi(c.Params("ano"))
	if err != nil {
		return h.erro(c, http.StatusBadRequest, "Parâmetro 'ano' inválido")
	}
	ctx, cancel := context.WithTimeout(c.Context(), requestTimeout)
	defer cancel()
	meses, err := h.svc.MesesDoAno(ctx, ano)
	if err != nil {
		return h.tratar(c, err)
	}
	return c.JSON(meses)
}

func (h *Handler) mesesEcidade(c fiber.Ctx) error {
	ano, err1 := strconv.Atoi(c.Params("ano"))
	matricula, err2 := strconv.Atoi(c.Params("matricula"))
	if err1 != nil || err2 != nil {
		return h.erro(c, http.StatusBadRequest, "Parâmetros 'ano' e 'matricula' devem ser numéricos")
	}
	ctx, cancel := context.WithTimeout(c.Context(), requestTimeout)
	defer cancel()
	meses, err := h.svc.MesesEcidade(ctx, ano, matricula)
	if err != nil {
		return h.tratar(c, err)
	}
	return c.JSON(meses)
}

func (h *Handler) intranet(c fiber.Ctx) error {
	ano, mes, matricula, err := paramsInt3(c, "ano", "mes", "matricula")
	if err != nil {
		return h.erro(c, http.StatusBadRequest, err.Error())
	}
	ctx, cancel := context.WithTimeout(c.Context(), requestTimeout)
	defer cancel()
	par, err := h.svc.Intranet(ctx, mes, ano, matricula)
	if err != nil {
		return h.tratar(c, err)
	}
	return c.JSON(par)
}

func (h *Handler) gerar(c fiber.Ctx) error {
	req, err := h.corpo(c)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(c.Context(), requestTimeout)
	defer cancel()
	pdf, err := h.svc.GerarPDF(ctx, int(req.Mes), int(req.Ano), int(req.Matricula))
	if err != nil {
		return h.tratar(c, err)
	}
	nome := fmt.Sprintf("contra-cheque-%d-%02d-%d.pdf", int(req.Matricula), int(req.Mes), int(req.Ano))
	c.Set(fiber.HeaderContentType, "application/pdf")
	c.Set(fiber.HeaderContentDisposition, fmt.Sprintf(`inline; filename="%s"`, nome))
	return c.Send(pdf)
}

type respostaValida struct {
	Valido      bool   `json:"valido"`
	Mensagem    string `json:"mensagem"`
	Matricula   int    `json:"matricula"`
	Mes         int    `json:"mes"`
	Ano         int    `json:"ano"`
	DataGeracao Data   `json:"dataGeracao"`
}

func (h *Handler) verificar(c fiber.Ctx) error {
	ctx, cancel := context.WithTimeout(c.Context(), requestTimeout)
	defer cancel()
	v, err := h.svc.Verificar(ctx, c.Params("codigo"))
	if err != nil {
		log.Printf("verificar código de contracheque: %v", err)
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{
			"erro": "Erro ao verificar código", "mensagem": "Erro desconhecido",
		})
	}
	if v == nil {
		return c.Status(http.StatusNotFound).JSON(fiber.Map{
			"valido": false, "mensagem": "Código de verificação inválido ou expirado",
		})
	}
	return c.JSON(respostaValida{
		Valido: true, Mensagem: "Contracheque autêntico",
		Matricula: v.Matricula, Mes: v.Mes, Ano: v.Ano, DataGeracao: Data{Time: v.DataReg},
	})
}

// corpo lê o JSON da requisição (consumes = application/json).
func (h *Handler) corpo(c fiber.Ctx) (ConsultaRequest, error) {
	var req ConsultaRequest
	if !strings.HasPrefix(strings.ToLower(c.Get(fiber.HeaderContentType)), fiber.MIMEApplicationJSON) {
		return req, h.erro(c, http.StatusUnsupportedMediaType, "Content-Type deve ser application/json")
	}
	if err := c.Bind().JSON(&req); err != nil {
		return req, h.erro(c, http.StatusBadRequest, "Corpo da requisição inválido: "+err.Error())
	}
	return req, nil
}

func (h *Handler) tratar(c fiber.Ctx, err error) error {
	var naoLocalizado *NaoLocalizadoError
	switch {
	case errors.As(err, &naoLocalizado):
		return h.erro(c, http.StatusBadRequest, naoLocalizado.Message)
	case errors.Is(err, ErrSemContraCheque):
		return h.erro(c, http.StatusNotFound, "Contracheque não disponível para a competência informada")
	default:
		log.Printf("erro interno: %v", err)
		return h.erro(c, http.StatusInternalServerError, "Erro interno do servidor")
	}
}

// erro devolve o mesmo JSON do HttpResponse do legado.
func (h *Handler) erro(c fiber.Ctx, status int, message string) error {
	return c.Status(status).JSON(HTTPResponse{
		HTTPStatusCode: status,
		HTTPStatus:     strings.ReplaceAll(strings.ToUpper(http.StatusText(status)), " ", "_"),
		Reason:         strings.ToUpper(http.StatusText(status)),
		Message:        message,
		TimeStamp:      time.Now().In(h.location).Format("02-01--2006 15:04:05"),
	})
}

// HTTPResponse espelha br.gov.ap.tce.s3i.config.exception.HttpResponse.
type HTTPResponse struct {
	HTTPStatusCode int    `json:"httpStatusCode"`
	HTTPStatus     string `json:"httpStatus"`
	Reason         string `json:"reason"`
	Message        string `json:"message"`
	TimeStamp      string `json:"timeStamp"`
}

func paramsInt3(c fiber.Ctx, a, b, d string) (int, int, int, error) {
	x, err1 := strconv.Atoi(c.Params(a))
	y, err2 := strconv.Atoi(c.Params(b))
	z, err3 := strconv.Atoi(c.Params(d))
	if err1 != nil || err2 != nil || err3 != nil {
		return 0, 0, 0, fmt.Errorf("Parâmetros '%s', '%s' e '%s' devem ser numéricos", a, b, d)
	}
	return x, y, z, nil
}
