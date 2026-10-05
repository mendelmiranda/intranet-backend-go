package ferias

import (
	"context"
	"errors"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"
	"tce.ap.gov.br/sistema-corporativo/ferias-service/internal/auth"
)

const requestTimeout = 30 * time.Second

type Handler struct {
	svc *Service
	key []byte
}

func NewHandler(svc *Service, signingKey []byte) *Handler { return &Handler{svc: svc, key: signingKey} }

// Register expõe as rotas do módulo. As roles seguem o WebSecurityConfig do Spring (primeira
// regra que casa); onde o legado deixava a rota só "autenticada" mas a intenção era do RH
// (pagamento, atualizar-pagamento e atualização do período aquisitivo), exigimos RH_AVISO_FERIAS.
func (h *Handler) Register(app *fiber.App) {
	prog := auth.Require(h.key, auth.RoleRHAvisoFerias, auth.RoleDashboard, auth.RoleDashboardNormal)
	rh := auth.Require(h.key, auth.RoleRHAvisoFerias)
	logado := auth.Require(h.key)
	periodo := auth.Require(h.key, auth.RoleDashboard, auth.RoleRHAvisoFerias)

	// programação
	app.Get("/api/programacao/ano/:ano", prog, h.porAno)
	app.Get("/api/programacao/chefe/cpf-chefe/:cpf/ano/:ano/servidores", prog, h.servidoresDoChefe)
	app.Get("/api/programacao/chart/ano/:ano/mes/:mes/periodo/:periodo", rh, h.chart)
	app.Get("/api/programacao/comprovante/:id", rh, h.comprovante) // antes de /:ano/:cpf
	app.Get("/api/programacao/:ano/:cpf", prog, h.periodosDoAno)
	app.Get("/api/programacao/:cpf", prog, h.periodosAnteriores)
	app.Post("/api/programacao/consulta", prog, h.consulta)
	app.Post("/api/programacao/restantes", rh, h.restantes)
	app.Post("/api/programacao/bloquear", rh, h.bloquear(1))
	app.Post("/api/programacao/desbloquear", rh, h.bloquear(0))
	app.Post("/api/programacao", prog, h.nova)
	app.Delete("/api/programacao/remover/:id", prog, h.remover)
	app.Put("/api/programacao/atualizar", prog, h.atualizar)
	app.Put("/api/programacao/atualizar-autorizacao", prog, h.atualizarAutorizacao)
	app.Put("/api/programacao/atualizar-pagamento", rh, h.atualizarPagamento)
	app.Put("/api/programacao/gerar-comprovante/:id", rh, h.gerarComprovante)
	app.Put("/api/pagamento/programacao/gerar-comprovante/:id", rh, h.gerarComprovante)
	app.Post("/api/consulta/ferias/pdf", rh, h.relatorioPDF)

	// histórico e gráfico
	app.Get("/api/programacao-historico/:cpf", logado, h.historico)
	app.Get("/api/programacao-ferias/chart/:ano", logado, h.quantidadePorMes)

	// observações
	app.Get("/api/programacao-observacao", rh, h.observacoes)
	app.Get("/api/programacao-observacao/periodo/:id", rh, h.observacoesDoPeriodo)
	app.Get("/api/programacao-observacao/:id", rh, h.observacaoPorID)
	app.Post("/api/programacao-observacao", rh, h.novaObservacao)
	app.Post("/api/programacao-observacao-alteracao-periodo", rh, h.novaObservacao)
	app.Delete("/api/programacao-observacao/remover/:id", rh, h.removerObservacao)

	// alteração de férias
	app.Get("/api/alteracao-ferias", rh, h.alteracoes)
	app.Post("/api/alteracao-ferias/novo", rh, h.novaAlteracao)
	app.Put("/api/alteracao-ferias/:id", rh, h.atualizarAlteracao)

	// período aquisitivo (o legado expõe com e sem /api)
	for _, base := range []string{"/periodo-aquisitivo", "/api/periodo-aquisitivo"} {
		app.Get(base+"/all", periodo, h.periodosAquisitivos)
		app.Put(base+"/update", rh, h.atualizarPeriodo(base == "/api/periodo-aquisitivo"))
		app.Get(base+"/:ano", periodo, h.periodoDoAno)
	}
}

// --- programação: leitura ---

func (h *Handler) porAno(c fiber.Ctx) error {
	ano, err := inteiro(c, "ano")
	if err != nil {
		return err
	}
	return h.listar(c, func(ctx context.Context) ([]Programacao, error) { return h.svc.PorAno(ctx, ano) })
}

func (h *Handler) periodosDoAno(c fiber.Ctx) error {
	ano, err := inteiro(c, "ano")
	if err != nil {
		return err
	}
	cpf := c.Params("cpf")
	return h.listar(c, func(ctx context.Context) ([]Programacao, error) { return h.svc.PeriodosDoAno(ctx, ano, cpf) })
}

func (h *Handler) periodosAnteriores(c fiber.Ctx) error {
	cpf := c.Params("cpf")
	return h.listar(c, func(ctx context.Context) ([]Programacao, error) { return h.svc.PeriodosAnteriores(ctx, cpf) })
}

func (h *Handler) chart(c fiber.Ctx) error {
	ano, err := inteiro(c, "ano")
	if err != nil {
		return err
	}
	mes, err := inteiro(c, "mes")
	if err != nil {
		return err
	}
	periodo := c.Params("periodo")
	return h.listar(c, func(ctx context.Context) ([]Programacao, error) { return h.svc.repo.Chart(ctx, ano, mes, periodo) })
}

func (h *Handler) consulta(c fiber.Ctx) error {
	var f Parametros
	if err := lerCorpo(c, &f); err != nil {
		return err
	}
	return h.listar(c, func(ctx context.Context) ([]Programacao, error) { return h.svc.Consulta(ctx, f) })
}

func (h *Handler) listar(c fiber.Ctx, fn func(context.Context) ([]Programacao, error)) error {
	ctx, cancel := context.WithTimeout(c.Context(), requestTimeout)
	defer cancel()
	lista, err := fn(ctx)
	if err != nil {
		return falha(c, err)
	}
	if lista == nil {
		lista = []Programacao{}
	}
	return c.JSON(lista)
}

func (h *Handler) restantes(c fiber.Ctx) error {
	var f Parametros
	if err := lerCorpo(c, &f); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(c.Context(), requestTimeout)
	defer cancel()
	lista, err := h.svc.Restantes(ctx, f)
	if err != nil {
		return falha(c, err)
	}
	return c.JSON(lista)
}

func (h *Handler) servidoresDoChefe(c fiber.Ctx) error {
	ano, err := inteiro(c, "ano")
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(c.Context(), requestTimeout)
	defer cancel()
	dto, err := h.svc.ServidoresDoChefe(ctx, c.Params("cpf"), ano)
	if err != nil {
		return falha(c, err)
	}
	return c.JSON(dto)
}

func (h *Handler) historico(c fiber.Ctx) error {
	ctx, cancel := context.WithTimeout(c.Context(), requestTimeout)
	defer cancel()
	lista, err := h.svc.repo.HistoricoPorCPF(ctx, c.Params("cpf"))
	if err != nil {
		return falha(c, err)
	}
	return c.JSON(lista)
}

func (h *Handler) quantidadePorMes(c fiber.Ctx) error {
	ano, err := inteiro(c, "ano")
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(c.Context(), requestTimeout)
	defer cancel()
	lista, err := h.svc.repo.QuantidadePorMes(ctx, ano)
	if err != nil {
		return falha(c, err)
	}
	return c.JSON(lista)
}

// --- programação: escrita ---

func (h *Handler) nova(c fiber.Ctx) error {
	var p Programacao
	if err := lerCorpo(c, &p); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(c.Context(), requestTimeout)
	defer cancel()
	salva, err := h.svc.Salvar(ctx, chamador(c), p)
	if err != nil {
		return falha(c, err)
	}
	return criado(c, salva.ID)
}

func (h *Handler) atualizar(c fiber.Ctx) error {
	var p Programacao
	if err := lerCorpo(c, &p); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(c.Context(), requestTimeout)
	defer cancel()
	salva, err := h.svc.Atualizar(ctx, chamador(c), p)
	if err != nil {
		return falha(c, err)
	}
	return c.JSON(salva)
}

func (h *Handler) atualizarAutorizacao(c fiber.Ctx) error {
	var p Programacao
	if err := lerCorpo(c, &p); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(c.Context(), requestTimeout)
	defer cancel()
	salva, err := h.svc.AtualizarAutorizacao(ctx, chamador(c), p)
	if err != nil {
		return falha(c, err)
	}
	return criado(c, salva.ID)
}

func (h *Handler) atualizarPagamento(c fiber.Ctx) error {
	var p Programacao
	if err := lerCorpo(c, &p); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(c.Context(), requestTimeout)
	defer cancel()
	salva, err := h.svc.DesfazerPagamento(ctx, chamador(c), p)
	if err != nil {
		return falha(c, err)
	}
	return criado(c, salva.ID)
}

func (h *Handler) gerarComprovante(c fiber.Ctx) error {
	var p Programacao
	if err := lerCorpo(c, &p); err != nil {
		return err
	}
	if p.ID == 0 {
		if id, err := strconv.ParseInt(c.Params("id"), 10, 64); err == nil {
			p.ID = id
		}
	}
	if p.DataPagamento == nil {
		return c.SendStatus(http.StatusBadRequest)
	}
	ctx, cancel := context.WithTimeout(c.Context(), requestTimeout)
	defer cancel()
	salva, ok, err := h.svc.RegistrarPagamento(ctx, chamador(c), p)
	if err != nil {
		log.Printf("registro de pagamento de férias: %v", err)
		return c.SendStatus(http.StatusBadRequest)
	}
	if !ok {
		return c.SendStatus(http.StatusBadRequest)
	}
	return c.JSON(salva)
}

func (h *Handler) remover(c fiber.Ctx) error {
	id, err := inteiro64(c, "id")
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(c.Context(), requestTimeout)
	defer cancel()
	if err := h.svc.Remover(ctx, chamador(c), id); err != nil {
		return falha(c, err)
	}
	return c.SendStatus(http.StatusNoContent)
}

func (h *Handler) bloquear(valor int) fiber.Handler {
	return func(c fiber.Ctx) error {
		var lista []Programacao
		if err := lerCorpo(c, &lista); err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(c.Context(), requestTimeout)
		defer cancel()
		if err := h.svc.Bloquear(ctx, chamador(c), lista, valor); err != nil {
			return falha(c, err)
		}
		return criado(c, 0)
	}
}

// --- observações ---

func (h *Handler) observacoes(c fiber.Ctx) error {
	ctx, cancel := context.WithTimeout(c.Context(), requestTimeout)
	defer cancel()
	lista, err := h.svc.repo.Observacoes(ctx)
	if err != nil {
		return falha(c, err)
	}
	return c.JSON(lista)
}

func (h *Handler) observacoesDoPeriodo(c fiber.Ctx) error {
	id, err := inteiro64(c, "id")
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(c.Context(), requestTimeout)
	defer cancel()
	lista, err := h.svc.repo.ObservacoesDoPeriodo(ctx, id)
	if err != nil {
		return falha(c, err)
	}
	return c.JSON(lista)
}

func (h *Handler) observacaoPorID(c fiber.Ctx) error {
	id, err := inteiro64(c, "id")
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(c.Context(), requestTimeout)
	defer cancel()
	o, err := h.svc.repo.ObservacaoPorID(ctx, id)
	if err != nil {
		return falha(c, err)
	}
	return c.JSON(o) // nil → null, como o Optional vazio do legado
}

func (h *Handler) novaObservacao(c fiber.Ctx) error {
	var o Observacao
	if err := lerCorpo(c, &o); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(c.Context(), requestTimeout)
	defer cancel()
	id, err := h.svc.SalvarObservacao(ctx, chamador(c), o)
	if err != nil {
		return falha(c, err)
	}
	return criado(c, id)
}

func (h *Handler) removerObservacao(c fiber.Ctx) error {
	id, err := inteiro64(c, "id")
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(c.Context(), requestTimeout)
	defer cancel()
	if err := h.svc.repo.RemoverObservacao(ctx, id); err != nil {
		return falha(c, err)
	}
	return c.SendStatus(http.StatusNoContent)
}

// --- alteração de férias ---

func (h *Handler) alteracoes(c fiber.Ctx) error {
	ctx, cancel := context.WithTimeout(c.Context(), requestTimeout)
	defer cancel()
	lista, err := h.svc.repo.Alteracoes(ctx)
	if err != nil {
		return falha(c, err)
	}
	return c.JSON(lista)
}

func (h *Handler) novaAlteracao(c fiber.Ctx) error {
	var p Programacao
	if err := lerCorpo(c, &p); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(c.Context(), requestTimeout)
	defer cancel()
	if err := h.svc.NovaAlteracao(ctx, chamador(c), p); err != nil {
		return falha(c, err)
	}
	return c.SendStatus(http.StatusOK)
}

func (h *Handler) atualizarAlteracao(c fiber.Ctx) error {
	id, err := inteiro64(c, "id")
	if err != nil {
		return err
	}
	var a Alteracao
	if err := lerCorpo(c, &a); err != nil {
		return err
	}
	a.ID = id
	ctx, cancel := context.WithTimeout(c.Context(), requestTimeout)
	defer cancel()
	if err := h.svc.repo.AtualizarAlteracao(ctx, a); err != nil {
		return falha(c, err)
	}
	return criado(c, id)
}

// --- período aquisitivo ---

func (h *Handler) periodosAquisitivos(c fiber.Ctx) error {
	ctx, cancel := context.WithTimeout(c.Context(), requestTimeout)
	defer cancel()
	lista, err := h.svc.repo.PeriodosAquisitivos(ctx)
	if err != nil {
		return falha(c, err)
	}
	return c.JSON(lista)
}

func (h *Handler) periodoDoAno(c fiber.Ctx) error {
	ano, err := inteiro(c, "ano")
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(c.Context(), requestTimeout)
	defer cancel()
	lista, err := h.svc.PeriodoDoAno(ctx, ano)
	if err != nil {
		return falha(c, err)
	}
	return c.JSON(lista)
}

// atualizarPeriodo: /api/periodo-aquisitivo/update devolve 200 com o corpo; a rota sem /api devolve 201.
func (h *Handler) atualizarPeriodo(devolveCorpo bool) fiber.Handler {
	return func(c fiber.Ctx) error {
		var p PeriodoAquisitivo
		if err := lerCorpo(c, &p); err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(c.Context(), requestTimeout)
		defer cancel()
		if err := h.svc.SalvarPeriodoAquisitivo(ctx, chamador(c), p); err != nil {
			return falha(c, err)
		}
		if devolveCorpo {
			return c.JSON(p)
		}
		return criado(c, int64(p.Ano))
	}
}

// --- relatório ---

func (h *Handler) relatorioPDF(c fiber.Ctx) error {
	var f Parametros
	if err := lerCorpo(c, &f); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(c.Context(), requestTimeout)
	defer cancel()
	lista, err := h.svc.repo.Consulta(ctx, f)
	if err != nil {
		log.Printf("relatório de férias: %v", err)
		return c.SendStatus(http.StatusInternalServerError)
	}
	// O legado ordena como a consulta; para o PDF usa o resultado do RH sem exigir linhas.
	pdf, err := gerarRelatorioPDF(lista)
	if err != nil {
		log.Printf("relatório de férias: %v", err)
		return c.SendStatus(http.StatusInternalServerError)
	}
	c.Set(fiber.HeaderContentType, "application/pdf")
	c.Set(fiber.HeaderContentDisposition, `inline; filename="relatorio_ferias`+strconv.Itoa(f.Ano)+`.pdf"`)
	return c.Send(pdf)
}

// comprovante devolve o aviso de férias em PDF, gerado na hora.
func (h *Handler) comprovante(c fiber.Ctx) error {
	id, err := inteiro64(c, "id")
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(c.Context(), requestTimeout)
	defer cancel()
	pdf, nome, err := h.svc.ComprovanteDoAviso(ctx, id)
	if err != nil {
		return falha(c, err)
	}
	c.Set(fiber.HeaderContentType, "application/pdf")
	c.Set(fiber.HeaderContentDisposition, `inline; filename="`+nome+`"`)
	return c.Send(pdf)
}

// --- utilitários ---

func chamador(c fiber.Ctx) Chamador {
	return Chamador{
		Usuario: auth.Username(c),
		Admin:   auth.HasRole(c, auth.RoleRHAvisoFerias),
		Membro:  auth.HasRole(c, auth.RoleMembros),
	}
}

func lerCorpo(c fiber.Ctx, destino any) error {
	if err := c.Bind().JSON(destino); err != nil {
		return fiber.NewError(http.StatusBadRequest, "Corpo da requisição inválido: "+err.Error())
	}
	return nil
}

func inteiro(c fiber.Ctx, nome string) (int, error) {
	n, err := strconv.Atoi(c.Params(nome))
	if err != nil {
		return 0, fiber.NewError(http.StatusBadRequest, "Parâmetro inválido: "+nome)
	}
	return n, nil
}

func inteiro64(c fiber.Ctx, nome string) (int64, error) {
	n, err := strconv.ParseInt(c.Params(nome), 10, 64)
	if err != nil {
		return 0, fiber.NewError(http.StatusBadRequest, "Parâmetro inválido: "+nome)
	}
	return n, nil
}

func criado(c fiber.Ctx, id int64) error {
	c.Set(fiber.HeaderLocation, c.BaseURL()+c.Path()+"/"+strconv.FormatInt(id, 10))
	return c.SendStatus(http.StatusCreated)
}

// falha traduz erros de negócio em 400 (HttpResponse do legado) e os demais em 500.
func falha(c fiber.Ctx, err error) error {
	var negocio ErroNegocio
	if errors.As(err, &negocio) {
		return respostaErro(c, http.StatusBadRequest, negocio.Error())
	}
	log.Printf("%s %s: %v", c.Method(), c.Path(), err)
	return c.Status(http.StatusInternalServerError).JSON(fiber.Map{"message": "Erro ao processar a solicitação"})
}

func respostaErro(c fiber.Ctx, status int, mensagem string) error {
	return c.Status(status).JSON(fiber.Map{
		"httpStatusCode": status,
		"httpStatus":     strings.ReplaceAll(strings.ToUpper(http.StatusText(status)), " ", "_"),
		"reason":         strings.ToUpper(http.StatusText(status)),
		"message":        mensagem,
	})
}

// ErrorHandler renderiza erros de requisição (corpo ou parâmetros inválidos) no formato do legado.
func ErrorHandler(c fiber.Ctx, err error) error {
	var fe *fiber.Error
	if errors.As(err, &fe) {
		return respostaErro(c, fe.Code, fe.Message)
	}
	log.Printf("%s %s: %v", c.Method(), c.Path(), err)
	return c.Status(http.StatusInternalServerError).JSON(fiber.Map{"message": "Erro ao processar a solicitação"})
}
