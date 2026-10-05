package chefe

import (
	"context"
	"errors"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/gofiber/fiber/v3"
	"tce.ap.gov.br/sistema-corporativo/chefe-service/internal/auth"
)

const requestTimeout = 20 * time.Second

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

// Register expõe as rotas sob o grupo informado (/api/chefe).
func (h *Handler) Register(g fiber.Router) {
	g.Get("", h.listarTodos)
	g.Get("/cpf/:cpf", h.porCPF)
	g.Get("/cpf-chefe/:cpf", h.servidoresDoChefe)
	g.Get("/cpf-chefe/:cpf/avaliacao", h.avaliacao(false))
	g.Get("/cpf-chefe/:cpf/avaliacao/pares", h.avaliacao(true))
	g.Get("/cpf-chefe/:cpf/avaliacao/menu", h.avaliacao(false))
	g.Get("/cpf-funcionario/:cpf", h.chefeDoFuncionario)
	g.Get("/efetivo/cpf/:cpf", h.efetivo)
	g.Get("/lotacao/:lotacao", h.porLotacao)
	g.Get("/:id", h.porID)
	g.Post("", h.novo)
	g.Put("", h.atualizar)
	g.Delete("/remover-servidor/:cpfChefe/:cpfFuncionario", h.remover)
}

func (h *Handler) listarTodos(c fiber.Ctx) error {
	ctx, cancel := context.WithTimeout(c.Context(), requestTimeout)
	defer cancel()
	lista, err := h.svc.repo.Listar(ctx)
	if err != nil {
		return falha(c, "listagem de chefes", err)
	}
	return c.JSON(lista)
}

func (h *Handler) porID(c fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return erro(c, http.StatusBadRequest, "id inválido")
	}
	ctx, cancel := context.WithTimeout(c.Context(), requestTimeout)
	defer cancel()
	item, err := h.svc.repo.BuscarPorID(ctx, id)
	if err != nil {
		return falha(c, "consulta de chefe por id", err)
	}
	return c.JSON(item) // nil → null, como o Optional vazio do legado
}

func (h *Handler) porCPF(c fiber.Ctx) error {
	ctx, cancel := context.WithTimeout(c.Context(), requestTimeout)
	defer cancel()
	lista, err := h.svc.PorCPF(ctx, c.Params("cpf"))
	if err != nil {
		return falha(c, "consulta de servidores do chefe", err)
	}
	return c.JSON(lista)
}

func (h *Handler) servidoresDoChefe(c fiber.Ctx) error {
	ctx, cancel := context.WithTimeout(c.Context(), requestTimeout)
	defer cancel()
	lista, err := h.svc.PorCPFChefe(ctx, c.Params("cpf"))
	if err != nil {
		return falha(c, "consulta de servidores do chefe", err)
	}
	return c.JSON(lista)
}

func (h *Handler) avaliacao(semPares bool) fiber.Handler {
	return func(c fiber.Ctx) error {
		ctx, cancel := context.WithTimeout(c.Context(), requestTimeout)
		defer cancel()
		lista, err := h.svc.NaAvaliacao(ctx, c.Params("cpf"), semPares)
		if err != nil {
			return falha(c, "servidores do chefe na avaliação", err)
		}
		return c.JSON(lista)
	}
}

// chefeDoFuncionario responde 200 com null quando o servidor não tem chefe cadastrado;
// o frontend usa isso para saber se o servidor já está vinculado.
func (h *Handler) chefeDoFuncionario(c fiber.Ctx) error {
	ctx, cancel := context.WithTimeout(c.Context(), requestTimeout)
	defer cancel()
	item, err := h.svc.repo.BuscarPorCPFFuncionario(ctx, c.Params("cpf"))
	if err != nil {
		return falha(c, "consulta do chefe do servidor", err)
	}
	return c.JSON(item)
}

func (h *Handler) efetivo(c fiber.Ctx) error {
	ctx, cancel := context.WithTimeout(c.Context(), requestTimeout)
	defer cancel()
	ok, err := h.svc.Efetivo(ctx, c.Params("cpf"))
	if err != nil {
		return falha(c, "consulta de servidor efetivo", err)
	}
	return c.JSON(ok)
}

func (h *Handler) porLotacao(c fiber.Ctx) error {
	cod, err := strconv.Atoi(c.Params("lotacao"))
	if err != nil {
		return erro(c, http.StatusBadRequest, "lotação inválida")
	}
	ctx, cancel := context.WithTimeout(c.Context(), requestTimeout)
	defer cancel()
	lista, err := h.svc.repo.ListarPorLotacao(ctx, cod)
	if err != nil {
		return falha(c, "consulta por lotação", err)
	}
	return c.JSON(lista)
}

func (h *Handler) novo(c fiber.Ctx) error {
	var corpo ChefeFuncionario
	if err := c.Bind().JSON(&corpo); err != nil {
		return erro(c, http.StatusBadRequest, "corpo inválido")
	}
	if corpo.CPFChefe == "" || corpo.CPFFuncionario == "" {
		return erro(c, http.StatusBadRequest, "cpfChefe e cpfFuncionario são obrigatórios")
	}
	ctx, cancel := context.WithTimeout(c.Context(), requestTimeout)
	defer cancel()
	id, err := h.svc.Salvar(ctx, auth.Username(c), corpo)
	if err != nil {
		return falha(c, "cadastro de servidor do chefe", err)
	}
	c.Set(fiber.HeaderLocation, c.BaseURL()+c.Path()+"/"+strconv.FormatInt(id, 10))
	return c.SendStatus(http.StatusCreated)
}

func (h *Handler) atualizar(c fiber.Ctx) error {
	var corpo AtualizaChefeDTO
	if err := c.Bind().JSON(&corpo); err != nil {
		return erro(c, http.StatusBadRequest, "corpo inválido")
	}
	if corpo.CPFChefeAntigo == "" || corpo.CPFChefeNovo == "" {
		return erro(c, http.StatusBadRequest, "cpfChefeAntigo e cpfChefeNovo são obrigatórios")
	}
	ctx, cancel := context.WithTimeout(c.Context(), requestTimeout)
	defer cancel()
	err := h.svc.AtualizarChefe(ctx, auth.Username(c), corpo)
	if errors.Is(err, ErrChefeNaoEncontrado) {
		return erro(c, http.StatusNotFound, "Chefe não encontrado")
	}
	if err != nil {
		return falha(c, "atualização do chefe", err)
	}
	c.Set(fiber.HeaderLocation, c.BaseURL()+c.Path()+"/"+corpo.CPFChefeNovo)
	return c.SendStatus(http.StatusCreated)
}

func (h *Handler) remover(c fiber.Ctx) error {
	ctx, cancel := context.WithTimeout(c.Context(), requestTimeout)
	defer cancel()
	if err := h.svc.RemoverServidor(ctx, c.Params("cpfChefe"), c.Params("cpfFuncionario")); err != nil {
		return falha(c, "remoção de servidor do chefe", err)
	}
	return c.SendStatus(http.StatusNoContent)
}

func falha(c fiber.Ctx, contexto string, err error) error {
	log.Printf("%s: %v", contexto, err)
	return erro(c, http.StatusInternalServerError, "Erro ao processar a solicitação")
}

func erro(c fiber.Ctx, status int, message string) error {
	return c.Status(status).JSON(fiber.Map{"message": message})
}
