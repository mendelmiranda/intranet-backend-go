package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
	"unicode"
)

const requestTimeout = 15 * time.Second

// Session é o resultado de um login bem-sucedido no Keycloak.
type Session struct {
	AccessToken string
	ExpiresIn   int
	TokenType   string
	CPF         string
	Nome        string
}

// Error é uma falha de autenticação pronta para a resposta HTTP.
type Error struct {
	Status  int
	Code    string
	Message string
}

func (e *Error) Error() string { return e.Message }

// Keycloak troca o código de autorização por um access token.
type Keycloak struct {
	issuer       string
	clientID     string
	clientSecret string
	redirectURI  string
	http         *http.Client

	mu             sync.Mutex
	serviceToken   string
	serviceExpires time.Time
}

func NewKeycloak(issuer, clientID, clientSecret, redirectURI string) *Keycloak {
	return &Keycloak{
		issuer:       strings.TrimRight(issuer, "/"),
		clientID:     clientID,
		clientSecret: clientSecret,
		redirectURI:  redirectURI,
		http:         &http.Client{Timeout: requestTimeout},
	}
}

func (k *Keycloak) AuthorizeURL(state, challenge string) string {
	query := url.Values{}
	query.Set("response_type", "code")
	query.Set("client_id", k.clientID)
	query.Set("redirect_uri", k.redirectURI)
	query.Set("scope", "openid profile email")
	query.Set("state", state)
	query.Set("code_challenge", challenge)
	query.Set("code_challenge_method", "S256")
	return k.issuer + "/protocol/openid-connect/auth?" + query.Encode()
}

// ServiceToken pede um access token de client credentials e o reutiliza até perto do vencimento.
// A API de RH exige esse token do client, com a role rh-consulta, e não o token do login do usuário.
func (k *Keycloak) ServiceToken(ctx context.Context) (string, error) {
	k.mu.Lock()
	defer k.mu.Unlock()

	if k.serviceToken != "" && time.Until(k.serviceExpires) > 30*time.Second {
		return k.serviceToken, nil
	}

	form := url.Values{}
	form.Set("grant_type", "client_credentials")
	form.Set("client_id", k.clientID)
	form.Set("client_secret", k.clientSecret)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, k.issuer+"/protocol/openid-connect/token", strings.NewReader(form.Encode()))
	if err != nil {
		return "", fmt.Errorf("montar token de consulta: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	res, err := k.http.Do(req)
	if err != nil {
		return "", &Error{Status: http.StatusBadGateway, Code: "BAD_GATEWAY", Message: "Não foi possível falar com o Keycloak."}
	}
	defer res.Body.Close()

	body, err := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if err != nil {
		return "", &Error{Status: http.StatusBadGateway, Code: "BAD_GATEWAY", Message: "Resposta inválida do Keycloak."}
	}

	var payload struct {
		AccessToken      string `json:"access_token"`
		ExpiresIn        int    `json:"expires_in"`
		Error            string `json:"error"`
		ErrorDescription string `json:"error_description"`
	}
	if err := json.Unmarshal(body, &payload); err != nil || res.StatusCode != http.StatusOK || payload.AccessToken == "" {
		if payload.Error != "" {
			fmt.Printf("keycloak recusou o token de consulta: %s (%s)\n", payload.Error, payload.ErrorDescription)
		}
		return "", &Error{Status: http.StatusBadGateway, Code: "BAD_GATEWAY", Message: "Não foi possível obter o token de consulta no Keycloak."}
	}
	if payload.ExpiresIn <= 0 {
		payload.ExpiresIn = 300
	}

	k.serviceToken = payload.AccessToken
	k.serviceExpires = time.Now().Add(time.Duration(payload.ExpiresIn) * time.Second)
	return k.serviceToken, nil
}

func (k *Keycloak) Exchange(ctx context.Context, code, verifier string) (Session, error) {
	form := url.Values{}
	form.Set("grant_type", "authorization_code")
	form.Set("client_id", k.clientID)
	form.Set("client_secret", k.clientSecret)
	form.Set("code", code)
	form.Set("redirect_uri", k.redirectURI)
	form.Set("code_verifier", verifier)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, k.issuer+"/protocol/openid-connect/token", strings.NewReader(form.Encode()))
	if err != nil {
		return Session{}, fmt.Errorf("montar requisição do token: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	res, err := k.http.Do(req)
	if err != nil {
		return Session{}, &Error{Status: http.StatusBadGateway, Code: "BAD_GATEWAY", Message: "Não foi possível falar com o Keycloak."}
	}
	defer res.Body.Close()

	body, err := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if err != nil {
		return Session{}, &Error{Status: http.StatusBadGateway, Code: "BAD_GATEWAY", Message: "Resposta inválida do Keycloak."}
	}
	if res.StatusCode != http.StatusOK {
		return Session{}, keycloakFailure(res.StatusCode, body)
	}
	return k.sessionFromToken(ctx, body)
}

func (k *Keycloak) sessionFromToken(ctx context.Context, body []byte) (Session, error) {
	var token struct {
		AccessToken string `json:"access_token"`
		IDToken     string `json:"id_token"`
		ExpiresIn   int    `json:"expires_in"`
		TokenType   string `json:"token_type"`
	}
	if err := json.Unmarshal(body, &token); err != nil || token.AccessToken == "" {
		return Session{}, &Error{Status: http.StatusBadGateway, Code: "BAD_GATEWAY", Message: "O Keycloak não devolveu um access token."}
	}

	claims := map[string]any{}
	if parsed, err := payloadClaims(token.AccessToken); err == nil {
		mergeClaims(claims, parsed)
	}
	if token.IDToken != "" {
		if parsed, err := payloadClaims(token.IDToken); err == nil {
			mergeClaims(claims, parsed)
		}
	}
	if cpfFromClaims(claims) == "" {
		if info, err := k.userinfo(ctx, token.AccessToken); err == nil {
			mergeClaims(claims, info)
		}
	}

	cpf := cpfFromClaims(claims)
	if cpf == "" {
		logClaimKeys(claims)
		return Session{}, &Error{Status: http.StatusBadGateway, Code: "BAD_GATEWAY", Message: "O token não contém o CPF do usuário."}
	}
	if token.ExpiresIn <= 0 {
		token.ExpiresIn = 300
	}
	if token.TokenType == "" {
		token.TokenType = "Bearer"
	}
	return Session{
		AccessToken: token.AccessToken,
		ExpiresIn:   token.ExpiresIn,
		TokenType:   token.TokenType,
		CPF:         cpf,
		Nome:        stringClaim(claims, "name", "nome", "given_name"),
	}, nil
}

func (k *Keycloak) userinfo(ctx context.Context, accessToken string) (map[string]any, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, k.issuer+"/protocol/openid-connect/userinfo", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/json")

	res, err := k.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	body, err := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if err != nil || res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("userinfo status %d", res.StatusCode)
	}
	claims := map[string]any{}
	if err := json.Unmarshal(body, &claims); err != nil {
		return nil, err
	}
	return claims, nil
}

func mergeClaims(dest, src map[string]any) {
	for key, value := range src {
		dest[key] = value
	}
}

func logClaimKeys(claims map[string]any) {
	keys := make([]string, 0, len(claims))
	for key := range claims {
		keys = append(keys, key)
	}
	fmt.Printf("token sem cpf; claims: %s\n", strings.Join(keys, ","))
}

func keycloakFailure(status int, body []byte) error {
	var payload struct {
		Error            string `json:"error"`
		ErrorDescription string `json:"error_description"`
	}
	_ = json.Unmarshal(body, &payload)
	if payload.Error != "" {
		fmt.Printf("keycloak recusou a troca do código: %s (%s)\n", payload.Error, payload.ErrorDescription)
	}

	switch payload.Error {
	case "invalid_grant", "invalid_request":
		return &Error{Status: http.StatusUnauthorized, Code: "UNAUTHORIZED", Message: "Não foi possível concluir o login no Keycloak."}
	case "unauthorized_client", "invalid_client":
		return &Error{Status: http.StatusBadGateway, Code: "BAD_GATEWAY", Message: "O cliente Keycloak recusou o login."}
	default:
		if status == http.StatusUnauthorized || status == http.StatusBadRequest {
			return &Error{Status: http.StatusUnauthorized, Code: "UNAUTHORIZED", Message: "Não foi possível concluir o login no Keycloak."}
		}
		return &Error{Status: http.StatusBadGateway, Code: "BAD_GATEWAY", Message: "Não foi possível autenticar no Keycloak."}
	}
}

func payloadClaims(token string) (map[string]any, error) {
	parts := strings.Split(token, ".")
	if len(parts) < 2 {
		return nil, fmt.Errorf("jwt incompleto")
	}
	raw, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		raw, err = base64.URLEncoding.DecodeString(parts[1])
		if err != nil {
			return nil, err
		}
	}
	claims := map[string]any{}
	if err := json.Unmarshal(raw, &claims); err != nil {
		return nil, err
	}
	return claims, nil
}

func cpfFromClaims(claims map[string]any) string {
	for _, key := range []string{"cpf", "CPF", "preferred_username", "username"} {
		digits := onlyDigits(stringClaim(claims, key))
		if len(digits) == 11 {
			return digits
		}
	}
	return ""
}

func stringClaim(claims map[string]any, keys ...string) string {
	for _, key := range keys {
		value, ok := claims[key].(string)
		if ok && strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func onlyDigits(value string) string {
	var builder strings.Builder
	for _, r := range value {
		if unicode.IsDigit(r) {
			builder.WriteRune(r)
		}
	}
	return builder.String()
}

func randomToken() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

func codeChallenge(verifier string) string {
	sum := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}
