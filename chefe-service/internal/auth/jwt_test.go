package auth

import (
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/golang-jwt/jwt/v5"
)

func chamar(t *testing.T, roles any) int {
	t.Helper()
	key := []byte("k")
	tok, _ := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub": "u", "roles": roles, "exp": time.Now().Add(time.Hour).Unix(),
	}).SignedString(key)
	app := fiber.New()
	app.Get("/", RequireAny(key, RoleDashboard, RoleRHAvisoFerias), func(c fiber.Ctx) error { return c.SendString(Username(c)) })
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	resp, _ := app.Test(req)
	return resp.StatusCode
}

func TestRequireAny(t *testing.T) {
	if s := chamar(t, "ROLE_X,ROLE_RH_AVISO_FERIAS"); s != 200 {
		t.Fatalf("status %d", s)
	}
	if s := chamar(t, "ROLE_X"); s != 403 {
		t.Fatalf("status %d", s)
	}
}
