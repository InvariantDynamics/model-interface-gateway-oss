package mig

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func TestAuthMiddlewareRejectsMissingBearer(t *testing.T) {
	svc := NewService()
	mux := http.NewServeMux()
	RegisterHTTPRoutes(mux, svc)
	server := httptest.NewServer(AuthMiddleware(AuthConfig{Mode: AuthModeJWT, JWTSecret: "secret"})(mux))
	defer server.Close()

	req := HelloRequest{Header: MessageHeader{TenantID: "acme"}, SupportedVersions: []string{"0.1"}, RequestedBindings: []string{"http"}}
	status, body := postJSONRaw(t, server.URL+"/mig/v0.1/hello", req, nil)
	if status != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d: %s", status, string(body))
	}
}

func TestAuthMiddlewareTenantMismatch(t *testing.T) {
	svc := NewService()
	mux := http.NewServeMux()
	RegisterHTTPRoutes(mux, svc)
	server := httptest.NewServer(AuthMiddleware(AuthConfig{Mode: AuthModeJWT, JWTSecret: "secret"})(mux))
	defer server.Close()

	token := makeJWT(t, "secret", "user-1", "acme", []string{"capability:infer"})
	req := HelloRequest{Header: MessageHeader{TenantID: "other"}, SupportedVersions: []string{"0.1"}, RequestedBindings: []string{"http"}}
	status, body := postJSONRaw(t, server.URL+"/mig/v0.1/hello", req, map[string]string{"Authorization": "Bearer " + token})
	if status != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", status, string(body))
	}
}

func TestAuthMiddlewareAllowsScopedInvoke(t *testing.T) {
	svc := NewService()
	mux := http.NewServeMux()
	RegisterHTTPRoutes(mux, svc)
	server := httptest.NewServer(AuthMiddleware(AuthConfig{Mode: AuthModeJWT, JWTSecret: "secret"})(mux))
	defer server.Close()

	token := makeJWT(t, "secret", "user-1", "acme", []string{"capability:infer"})
	req := InvokeRequest{
		Header:     MessageHeader{},
		Capability: "observatory.models.infer",
		Payload:    map[string]interface{}{"input": "hello"},
	}
	status, body := postJSONRaw(t, server.URL+"/mig/v0.1/invoke/observatory.models.infer", req, map[string]string{"Authorization": "Bearer " + token})
	if status != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", status, string(body))
	}
}

func makeJWT(t *testing.T, secret, sub, tenant string, scopes []string) string {
	t.Helper()
	claims := jwt.MapClaims{
		"sub":       sub,
		"tenant_id": tenant,
		"scopes":    scopes,
		"exp":       time.Now().Add(10 * time.Minute).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString([]byte(secret))
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}
	return signed
}

func postJSONRaw(t *testing.T, url string, reqBody interface{}, headers map[string]string) (int, []byte) {
	t.Helper()
	payload, err := json.Marshal(reqBody)
	if err != nil {
		t.Fatalf("marshal req: %v", err)
	}
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do request: %v", err)
	}
	defer resp.Body.Close()
	body := new(bytes.Buffer)
	_, _ = body.ReadFrom(resp.Body)
	return resp.StatusCode, body.Bytes()
}
