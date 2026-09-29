package config

import (
	"fmt"
	"os"
	"strings"

	"github.com/joho/godotenv"
)

type Config struct {
	Port                 string
	KeycloakIssuerURL    string
	KeycloakBaseURL      string
	KeycloakPublicURL    string
	KeycloakRealm        string
	KeycloakClientID     string
	KeycloakClientSecret string
	AllowedOrigins       []string
	Environment          string
}

// Load loads configuration from environment variables and .env file.
// It performs fail-fast validation to ensure all required variables are present.
func Load() (*Config, error) {
	// Attempt to load .env file if it exists, but do not error out if missing (e.g. in containerized env)
	_ = godotenv.Load()

	rawOrigins := getEnvOrDefault("ALLOWED_ORIGINS", "http://localhost:3000,http://localhost:5173,http://localhost:8080")
	var origins []string
	for _, o := range strings.Split(rawOrigins, ",") {
		trimmed := strings.TrimSpace(o)
		if trimmed != "" {
			origins = append(origins, trimmed)
		}
	}

	baseURL := os.Getenv("KEYCLOAK_BASE_URL")
	realm := os.Getenv("KEYCLOAK_REALM")
	issuerURL := os.Getenv("KEYCLOAK_ISSUER_URL")
	if issuerURL == "" && baseURL != "" && realm != "" {
		issuerURL = fmt.Sprintf("%s/realms/%s", baseURL, realm)
	}

	cfg := &Config{
		Port:                 getEnvOrDefault("PORT", ":3000"),
		KeycloakIssuerURL:    issuerURL,
		KeycloakBaseURL:      baseURL,
		KeycloakPublicURL:    getEnvOrDefault("KEYCLOAK_PUBLIC_URL", getEnvOrDefault("KEYCLOAK_BASE_URL", "http://localhost:8080")),
		KeycloakRealm:        realm,
		KeycloakClientID:     os.Getenv("KEYCLOAK_CLIENT_ID"),
		KeycloakClientSecret: os.Getenv("KEYCLOAK_CLIENT_SECRET"),
		AllowedOrigins:       origins,
		Environment:          getEnvOrDefault("ENV", "development"),
	}

	// Validate required variables
	var missing []string
	if cfg.KeycloakBaseURL == "" {
		missing = append(missing, "KEYCLOAK_BASE_URL")
	}
	if cfg.KeycloakRealm == "" {
		missing = append(missing, "KEYCLOAK_REALM")
	}
	if cfg.KeycloakClientID == "" {
		missing = append(missing, "KEYCLOAK_CLIENT_ID")
	}
	if cfg.KeycloakClientSecret == "" {
		missing = append(missing, "KEYCLOAK_CLIENT_SECRET")
	}

	if len(missing) > 0 {
		return nil, fmt.Errorf("missing required environment variable(s): %s", strings.Join(missing, ", "))
	}

	return cfg, nil
}

func getEnvOrDefault(key, fallback string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return fallback
}
