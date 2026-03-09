package config

import (
	"fmt"
	"os"
	"strings"
)

// Config holds all application configuration loaded from environment variables.
type Config struct {
	DatabaseURL           string
	JWTSecret             string
	Port                  string
	CORSOrigins           []string
	TemporalAddress       string
	TemporalNamespace     string
	TemporalAPIKey        string
	TemporalTLSEnabled    bool
	TemporalTLSServerName string

	// S3 / object storage (optional — attachments disabled if not set)
	AWSAccessKeyID     string
	AWSSecretAccessKey string
	AWSBucket          string
	AWSRegion          string
	AWSEndpointURL     string // For MinIO / local dev

	// Anthropic API (optional — agent/orchestration features disabled if not set)
	AnthropicAPIKey   string
	BraveSearchAPIKey string

	// GitHub App (optional — required for shared-runner repo mutation).
	// GITHUB_APP_PRIVATE_KEY should be provided as a base64-encoded PEM value.
	GitHubAppID         string
	GitHubAppSlug       string
	GitHubAppPrivateKey string

	// Postmark email (optional — email sending disabled if not set)
	PostmarkServerToken string
	PostmarkFromEmail   string
	AppBaseURL          string
}

// Load reads configuration from environment variables.
// DATABASE_URL and JWT_SECRET are required; PORT defaults to "8080",
// CORS_ORIGIN defaults to "http://localhost:5173".
func Load() (*Config, error) {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		return nil, fmt.Errorf("DATABASE_URL environment variable is required")
	}

	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		return nil, fmt.Errorf("JWT_SECRET environment variable is required")
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	corsOrigins := []string{
		"http://app.helpin.ai",
		"https://app.helpin.ai",
		"http://stage.helpin.ai",
		"https://stage.helpin.ai",
		"http://91.98.85.12",
		"http://localhost:5173",
	}

	appBaseURL := os.Getenv("APP_BASE_URL")
	if appBaseURL == "" {
		appBaseURL = "http://localhost:5173"
	}

	temporalAddress := os.Getenv("TEMPORAL_ADDRESS")
	if temporalAddress == "" {
		temporalAddress = "localhost:7233"
	}

	temporalNamespace := os.Getenv("TEMPORAL_NAMESPACE")
	if temporalNamespace == "" {
		temporalNamespace = "default"
	}

	temporalAPIKey := strings.TrimSpace(os.Getenv("TEMPORAL_API_KEY"))
	temporalTLSEnabled := parseBoolEnv(os.Getenv("TEMPORAL_TLS_ENABLED"))
	if temporalAPIKey != "" {
		temporalTLSEnabled = true
	}

	return &Config{
		DatabaseURL:           dbURL,
		JWTSecret:             jwtSecret,
		Port:                  port,
		CORSOrigins:           corsOrigins,
		TemporalAddress:       temporalAddress,
		TemporalNamespace:     temporalNamespace,
		TemporalAPIKey:        temporalAPIKey,
		TemporalTLSEnabled:    temporalTLSEnabled,
		TemporalTLSServerName: strings.TrimSpace(os.Getenv("TEMPORAL_TLS_SERVER_NAME")),
		AWSAccessKeyID:        os.Getenv("AWS_ACCESS_KEY_ID"),
		AWSSecretAccessKey:    os.Getenv("AWS_SECRET_ACCESS_KEY"),
		AWSBucket:             os.Getenv("AWS_S3_BUCKET_NAME"),
		AWSRegion:             os.Getenv("AWS_REGION"),
		AWSEndpointURL:        os.Getenv("AWS_S3_ENDPOINT_URL"),
		AnthropicAPIKey:       os.Getenv("ANTHROPIC_API_KEY"),
		BraveSearchAPIKey:     strings.TrimSpace(os.Getenv("BRAVE_SEARCH_API_KEY")),
		GitHubAppID:           os.Getenv("GITHUB_APP_ID"),
		GitHubAppSlug:         os.Getenv("GITHUB_APP_SLUG"),
		GitHubAppPrivateKey:   os.Getenv("GITHUB_APP_PRIVATE_KEY"),
		PostmarkServerToken:   os.Getenv("POSTMARK_SERVER_TOKEN"),
		PostmarkFromEmail:     os.Getenv("POSTMARK_FROM_EMAIL"),
		AppBaseURL:            appBaseURL,
	}, nil
}

func parseBoolEnv(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}
