// Package auth valida os tokens JWT emitidos pelo backend Spring Boot (HS256, claim "roles").
package auth

import (
	"strings"

	"github.com/gofiber/fiber/v3"
	"github.com/golang-jwt/jwt/v5"
)

const (
	RoleDashboard       = "ROLE_DASHBOARD"
	RoleDashboardNormal = "ROLE_DASHBOARD_NORMAL"
	RoleRHAvisoFerias   = "ROLE_RH_AVISO_FERIAS"
	RoleMembros         = "ROLE_MEMBROS"
)

type claimsKey struct{}

// Require valida o token Bearer e, se roles não for vazio, exige ao menos uma delas
// (hasAnyRole do Spring). Sem roles, basta estar autenticado (authenticated()).
func Require(signingKey []byte, roles ...string) fiber.Handler {
	parser := jwt.NewParser(jwt.WithValidMethods([]string{"HS256"}), jwt.WithExpirationRequired())

	return func(c fiber.Ctx) error {
		header := c.Get(fiber.HeaderAuthorization)
		if !strings.HasPrefix(header, "Bearer ") {
			return unauthorized(c)
		}

		claims := jwt.MapClaims{}
		token, err := parser.ParseWithClaims(strings.TrimPrefix(header, "Bearer "), claims,
			func(*jwt.Token) (any, error) { return signingKey, nil })
		if err != nil || !token.Valid {
			return unauthorized(c)
		}

		if len(roles) > 0 && !hasAnyRole(claims["roles"], roles) {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
				"status": 403, "error": "FORBIDDEN", "message": "Forbidden",
			})
		}

		c.Locals(claimsKey{}, claims)
		return c.Next()
	}
}

// Username devolve o subject do token (login do usuário), ou "" se ausente.
func Username(c fiber.Ctx) string {
	claims, _ := c.Locals(claimsKey{}).(jwt.MapClaims)
	sub, _ := claims["sub"].(string)
	return sub
}

// HasRole informa se o token validado contém a role.
func HasRole(c fiber.Ctx, role string) bool {
	claims, _ := c.Locals(claimsKey{}).(jwt.MapClaims)
	return hasAnyRole(claims["roles"], []string{role})
}

func unauthorized(c fiber.Ctx) error {
	return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
		"status": 401, "error": "UNAUTHORIZED", "message": "Unauthorized",
	})
}

// hasAnyRole aceita a claim "roles" como string separada por vírgulas ou como lista de strings.
func hasAnyRole(raw any, wanted []string) bool {
	var have []string
	switch roles := raw.(type) {
	case string:
		have = strings.Split(roles, ",")
	case []any:
		for _, r := range roles {
			if s, ok := r.(string); ok {
				have = append(have, s)
			}
		}
	}
	for _, h := range have {
		h = strings.TrimSpace(h)
		for _, w := range wanted {
			if h == w {
				return true
			}
		}
	}
	return false
}
