package mig

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strings"

	"github.com/golang-jwt/jwt/v5"
)

type Principal struct {
	Subject       string
	TenantID      string
	Scopes        map[string]struct{}
	Authenticated bool
}

func AnonymousPrincipal() Principal {
	return Principal{Subject: "anonymous", Scopes: map[string]struct{}{}}
}

func (p Principal) HasAnyScope(required []string) bool {
	if !p.Authenticated {
		return true
	}
	if len(required) == 0 {
		return true
	}
	for _, scope := range required {
		if _, ok := p.Scopes[scope]; ok {
			return true
		}
	}
	return false
}

func (p Principal) ScopesList() []string {
	out := make([]string, 0, len(p.Scopes))
	for scope := range p.Scopes {
		out = append(out, scope)
	}
	slices.Sort(out)
	return out
}

type AuthMode string

const (
	AuthModeNone AuthMode = "none"
	AuthModeJWT  AuthMode = "jwt"
)

type AuthConfig struct {
	Mode          AuthMode
	JWTSecret     string
	RequireTenant bool
}

type principalKey struct{}

func principalFromContext(ctx context.Context) Principal {
	principal, ok := ctx.Value(principalKey{}).(Principal)
	if !ok {
		return AnonymousPrincipal()
	}
	if principal.Scopes == nil {
		principal.Scopes = map[string]struct{}{}
	}
	return principal
}

func authMiddleware(cfg AuthConfig) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if strings.HasPrefix(r.URL.Path, "/ui") {
				r = r.WithContext(context.WithValue(r.Context(), principalKey{}, AnonymousPrincipal()))
				next.ServeHTTP(w, r)
				return
			}
			principal, err := principalFromRequest(r, cfg)
			if err != nil {
				header := MessageHeader{TenantID: tenantFromRequest(r)}
				writeMigError(w, header, httpStatusForError(err), MigError{Code: migCodeForAuthErr(err), Message: err.Error(), Retryable: false})
				return
			}
			r = r.WithContext(context.WithValue(r.Context(), principalKey{}, principal))
			next.ServeHTTP(w, r)
		})
	}
}

func AuthMiddleware(cfg AuthConfig) func(http.Handler) http.Handler {
	return authMiddleware(cfg)
}

func MiddlewareChain(next http.Handler, middlewares ...func(http.Handler) http.Handler) http.Handler {
	current := next
	for i := len(middlewares) - 1; i >= 0; i-- {
		current = middlewares[i](current)
	}
	return current
}

func principalFromRequest(r *http.Request, cfg AuthConfig) (Principal, error) {
	return PrincipalFromAuthHeaders(r.Header.Get("Authorization"), r.Header.Get("X-Tenant-ID"), cfg)
}

func PrincipalFromAuthHeaders(authorizationHeader, tenantHeader string, cfg AuthConfig) (Principal, error) {
	principal := AnonymousPrincipal()
	headerTenant := strings.TrimSpace(tenantHeader)

	switch cfg.Mode {
	case AuthModeNone:
		principal.TenantID = headerTenant
		if cfg.RequireTenant && principal.TenantID == "" {
			return Principal{}, fmt.Errorf("tenant header is required")
		}
		return principal, nil
	case AuthModeJWT:
		authorization := strings.TrimSpace(authorizationHeader)
		if !strings.HasPrefix(authorization, "Bearer ") {
			return Principal{}, fmt.Errorf("missing bearer token")
		}
		tokenString := strings.TrimSpace(strings.TrimPrefix(authorization, "Bearer "))
		claims := jwt.MapClaims{}
		token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
			if token.Method.Alg() != jwt.SigningMethodHS256.Alg() {
				return nil, fmt.Errorf("unsupported token signing method")
			}
			return []byte(cfg.JWTSecret), nil
		})
		if err != nil || !token.Valid {
			return Principal{}, fmt.Errorf("invalid bearer token")
		}
		principal.Authenticated = true
		principal.Subject = asStringClaim(claims, "sub", "unknown")
		principal.TenantID = asStringClaim(claims, "tenant_id", "")
		if principal.TenantID == "" {
			principal.TenantID = asStringClaim(claims, "tenant", "")
		}
		if principal.TenantID == "" {
			return Principal{}, fmt.Errorf("token is missing tenant claim")
		}
		principal.Scopes = scopesFromClaims(claims)
		if cfg.RequireTenant && headerTenant == "" {
			return Principal{}, fmt.Errorf("tenant header is required")
		}
		if headerTenant != "" && headerTenant != principal.TenantID {
			return Principal{}, fmt.Errorf("tenant header does not match token tenant")
		}
		return principal, nil
	default:
		return Principal{}, fmt.Errorf("unsupported auth mode %q", cfg.Mode)
	}
}

func asStringClaim(claims jwt.MapClaims, key, fallback string) string {
	if value, ok := claims[key]; ok {
		if s, ok := value.(string); ok {
			return strings.TrimSpace(s)
		}
	}
	return fallback
}

func scopesFromClaims(claims jwt.MapClaims) map[string]struct{} {
	out := map[string]struct{}{}
	if scopeRaw, ok := claims["scope"]; ok {
		if scopeString, ok := scopeRaw.(string); ok {
			for _, scope := range strings.Fields(scopeString) {
				out[scope] = struct{}{}
			}
		}
	}
	if scopesRaw, ok := claims["scopes"]; ok {
		switch typed := scopesRaw.(type) {
		case []interface{}:
			for _, value := range typed {
				if scope, ok := value.(string); ok {
					out[strings.TrimSpace(scope)] = struct{}{}
				}
			}
		case []string:
			for _, scope := range typed {
				out[strings.TrimSpace(scope)] = struct{}{}
			}
		}
	}
	return out
}

func applyPrincipalHeader(head *MessageHeader, principal Principal, r *http.Request) *MigError {
	if head.Meta == nil {
		head.Meta = map[string]interface{}{}
	}
	head.Meta["idg.principal_subject"] = principal.Subject
	head.Meta["idg.principal_scopes"] = principal.ScopesList()

	if head.TenantID == "" {
		switch {
		case principal.TenantID != "":
			head.TenantID = principal.TenantID
		default:
			head.TenantID = tenantFromRequest(r)
		}
	}
	if principal.TenantID != "" && head.TenantID != principal.TenantID {
		return &MigError{Code: ErrorForbidden, Message: "tenant_id does not match authenticated principal", Retryable: false}
	}
	if head.TenantID == "unknown" {
		return &MigError{Code: ErrorInvalidRequest, Message: "tenant_id is required", Retryable: false}
	}
	return nil
}

func httpStatusForError(err error) int {
	switch {
	case errors.Is(err, context.DeadlineExceeded):
		return http.StatusGatewayTimeout
	default:
		msg := err.Error()
		if strings.Contains(msg, "tenant") {
			return http.StatusForbidden
		}
		return http.StatusUnauthorized
	}
}

func migCodeForAuthErr(err error) string {
	msg := err.Error()
	if strings.Contains(msg, "tenant") {
		return ErrorForbidden
	}
	return ErrorUnauthorized
}
