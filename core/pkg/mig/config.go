package mig

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	Addr              string
	GRPCAddr          string
	Auth              AuthConfig
	NATSURL           string
	EnableNATSBinding bool
	AuditLogPath      string
	EnableMetrics     bool
}

func ConfigFromEnv() (Config, error) {
	cfg := Config{
		Addr:              envOrDefault("MIGD_ADDR", ":8080"),
		GRPCAddr:          strings.TrimSpace(os.Getenv("MIGD_GRPC_ADDR")),
		NATSURL:           strings.TrimSpace(os.Getenv("MIGD_NATS_URL")),
		EnableNATSBinding: envBool("MIGD_ENABLE_NATS_BINDING", true),
		AuditLogPath:      strings.TrimSpace(os.Getenv("MIGD_AUDIT_LOG_PATH")),
		EnableMetrics:     envBool("MIGD_ENABLE_METRICS", true),
	}

	authMode := strings.ToLower(strings.TrimSpace(envOrDefault("MIGD_AUTH_MODE", string(AuthModeNone))))
	switch AuthMode(authMode) {
	case AuthModeNone:
		cfg.Auth.Mode = AuthModeNone
	case AuthModeJWT:
		cfg.Auth.Mode = AuthModeJWT
		cfg.Auth.JWTSecret = strings.TrimSpace(os.Getenv("MIGD_JWT_HS256_SECRET"))
		if cfg.Auth.JWTSecret == "" {
			return Config{}, fmt.Errorf("MIGD_JWT_HS256_SECRET is required when MIGD_AUTH_MODE=jwt")
		}
	default:
		return Config{}, fmt.Errorf("unsupported MIGD_AUTH_MODE %q", authMode)
	}

	cfg.Auth.RequireTenant = envBool("MIGD_REQUIRE_TENANT_HEADER", false)
	return cfg, nil
}

func envBool(key string, fallback bool) bool {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func envOrDefault(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}
