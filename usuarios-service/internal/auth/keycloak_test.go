package auth

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestExchangeLeCpfDoIdToken(t *testing.T) {
	t.Parallel()

	access := fakeJWT(t, map[string]any{"sub": "usuario"})
	id := fakeJWT(t, map[string]any{"preferred_username": "111.222.333-96", "name": "Ana Lima"})
	client := NewKeycloak("https://keycloak.test/realms/intranet", "intranet", "segredo", redirectURI)
	body, err := json.Marshal(map[string]any{
		"access_token": access,
		"id_token":     id,
		"expires_in":   60,
		"token_type":   "Bearer",
	})
	if err != nil {
		t.Fatal(err)
	}
	session, err := client.sessionFromToken(context.Background(), body)
	if err != nil {
		t.Fatal(err)
	}
	if session.CPF != "11122233396" || session.Nome != "Ana Lima" {
		t.Fatalf("sessão inesperada: %+v", session)
	}
}

func fakeJWT(t *testing.T, claims map[string]any) string {
	t.Helper()
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"none"}`))
	payload, err := json.Marshal(claims)
	if err != nil {
		t.Fatal(err)
	}
	return header + "." + base64.RawURLEncoding.EncodeToString(payload) + ".sig"
}

func TestServiceTokenPedeClientCredentialsEReutiliza(t *testing.T) {
	t.Parallel()

	var calls int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.URL.Path != "/protocol/openid-connect/token" {
			t.Fatalf("caminho %s", r.URL.Path)
		}
		if err := r.ParseForm(); err != nil {
			t.Fatal(err)
		}
		if r.PostForm.Get("grant_type") != "client_credentials" || r.PostForm.Get("client_id") != "intranet" || r.PostForm.Get("client_secret") != "segredo" {
			t.Fatalf("form inesperado: %v", r.PostForm)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"access_token":"token-de-servico","expires_in":300,"token_type":"Bearer"}`)
	}))
	t.Cleanup(server.Close)

	client := NewKeycloak(server.URL, "intranet", "segredo", redirectURI)
	first, err := client.ServiceToken(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	second, err := client.ServiceToken(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if first != "token-de-servico" || second != first || calls != 1 {
		t.Fatalf("token %q chamadas %d", first, calls)
	}
}
