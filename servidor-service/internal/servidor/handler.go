package servidor

import (
	"context"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/gofiber/fiber/v3"
)

const requestTimeout = 20 * time.Second

type Handler struct {
	repo Repository
}

func NewHandler(repo Repository) *Handler {
	return &Handler{repo: repo}
}

func (h *Handler) Register(app *fiber.App) {
	app.Get("/api/servidores/detalhe", h.detalhe)
}

func (h *Handler) detalhe(c fiber.Ctx) error {
	filtro, err := interpretarConsulta(
		c.Query("cpf"),
		c.Query("nome"),
		c.Query("matricula"),
		c.Query("q"),
	)
	if err != nil {
		return erro(c, http.StatusBadRequest, err.Error())
	}

	ctx, cancel := context.WithTimeout(c.Context(), requestTimeout)
	defer cancel()

	servidores, err := h.repo.Buscar(ctx, filtro)
	if err != nil {
		log.Printf("consulta de servidor: %v", err)
		return erro(c, http.StatusInternalServerError, "Erro ao consultar o cadastro do servidor")
	}
	if len(servidores) == 0 {
		return erro(c, http.StatusNotFound, "Nenhum servidor encontrado")
	}
	return c.JSON(Resultado{Total: len(servidores), Servidores: servidores})
}

func interpretarConsulta(cpf, nome, matricula, q string) (Filtro, error) {
	cpf = strings.TrimSpace(cpf)
	nome = strings.TrimSpace(nome)
	matricula = strings.TrimSpace(matricula)
	q = strings.TrimSpace(q)

	if cpf == "" && nome == "" && matricula == "" {
		if q == "" {
			return Filtro{}, errConsulta("Informe cpf, nome, matricula ou q")
		}
		return interpretarTermo(q)
	}

	var filtro Filtro
	if cpf != "" {
		digitos := somenteDigitos(cpf)
		if len(digitos) != 11 {
			return Filtro{}, errConsulta("CPF deve ter 11 dígitos")
		}
		filtro.CPF = digitos
	}
	if matricula != "" {
		valor, err := strconv.Atoi(matricula)
		if err != nil || valor <= 0 {
			return Filtro{}, errConsulta("Matrícula deve ser um número positivo")
		}
		filtro.Matricula = &valor
	}
	if nome != "" {
		if len([]rune(nome)) < 2 {
			return Filtro{}, errConsulta("Nome deve ter ao menos 2 caracteres")
		}
		filtro.Nome = nome
	}
	return filtro, nil
}

func interpretarTermo(termo string) (Filtro, error) {
	if termoNumerico(termo) {
		digitos := somenteDigitos(termo)
		if len(digitos) == 11 {
			return Filtro{CPF: digitos}, nil
		}
		if strings.ContainsAny(termo, ".-") {
			return Filtro{}, errConsulta("CPF deve ter 11 dígitos")
		}
		valor, err := strconv.Atoi(digitos)
		if err != nil || valor <= 0 {
			return Filtro{}, errConsulta("Matrícula deve ser um número positivo")
		}
		return Filtro{Matricula: &valor}, nil
	}
	if len([]rune(termo)) < 2 {
		return Filtro{}, errConsulta("Nome deve ter ao menos 2 caracteres")
	}
	return Filtro{Nome: termo}, nil
}

func termoNumerico(value string) bool {
	temDigito := false
	for _, r := range value {
		switch {
		case unicode.IsDigit(r):
			temDigito = true
		case r == '.' || r == '-' || unicode.IsSpace(r):
		default:
			return false
		}
	}
	return temDigito
}

func somenteDigitos(value string) string {
	var b strings.Builder
	for _, r := range value {
		if unicode.IsDigit(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

type consultaError string

func (e consultaError) Error() string { return string(e) }

func errConsulta(message string) error { return consultaError(message) }

func erro(c fiber.Ctx, status int, message string) error {
	return c.Status(status).JSON(fiber.Map{"message": message})
}
