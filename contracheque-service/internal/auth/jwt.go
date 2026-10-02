// Package auth valida os tokens JWT emitidos pelo backend Spring Boot (HS256, claim "roles").
package auth

import (
	"strings"

	"github.com/gofiber/fiber/v3"
	"github.com/golang-jwt/jwt/v5"
)

const RoleDashboard = "ROLE_DASHBOARD"

type claimsKey struct{}

// Require valida o token Bearer e exige a role informada (equivalente a hasRole do Spring).
func Require(signingKey []byte, role string) fiber.Handler {
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

		if !hasRole(claims["roles"], role) {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
				"status": 403, "error": "FORBIDDEN", "message": "Forbidden",
			})
		}

		c.Locals(claimsKey{}, claims)
		return c.Next()
	}
}

func unauthorized(c fiber.Ctx) error {
	return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
		"status": 401, "error": "UNAUTHORIZED", "message": "Unauthorized",
	})
}

// hasRole aceita a claim "roles" como string separada por vírgulas (formato do login)
// ou como lista de strings.
func hasRole(raw any, role string) bool {
	switch roles := raw.(type) {
	case string:
		for _, r := range strings.Split(roles, ",") {
			if strings.TrimSpace(r) == role {
				return true
			}
		}
	case []any:
		for _, r := range roles {
			if s, ok := r.(string); ok && s == role {
				return true
			}
		}
	}
	return false
}
