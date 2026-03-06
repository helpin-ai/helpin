package config

import (
	"fmt"
	"os"
)

// Config holds all application configuration loaded from environment variables.
type Config struct {
	DatabaseURL       string
	JWTSecret         string
	Port              string
	CORSOrigin        string
	TemporalAddress   string
	TemporalNamespace string

	// S3 / object storage (optional — attachments disabled if not set)
	AWSAccessKeyID     string
	AWSSecretAccessKey string
	AWSBucket          string
	AWSRegion          string
	AWSEndpointURL     string // For MinIO / local dev

	// Anthropic API (optional — agent/orchestration features disabled if not set)
	AnthropicAPIKey string

	// GitHub App (optional — required for shared-runner repo mutation)
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

	corsOrigin := os.Getenv("CORS_ORIGIN")
	if corsOrigin == "" {
		corsOrigin = "http://localhost:5173"
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

	return &Config{
		DatabaseURL:         dbURL,
		JWTSecret:           jwtSecret,
		Port:                port,
		CORSOrigin:          corsOrigin,
		TemporalAddress:     temporalAddress,
		TemporalNamespace:   temporalNamespace,
		AWSAccessKeyID:      os.Getenv("AWS_ACCESS_KEY_ID"),
		AWSSecretAccessKey:  os.Getenv("AWS_SECRET_ACCESS_KEY"),
		AWSBucket:           os.Getenv("AWS_S3_BUCKET_NAME"),
		AWSRegion:           os.Getenv("AWS_REGION"),
		AWSEndpointURL:      os.Getenv("AWS_S3_ENDPOINT_URL"),
		AnthropicAPIKey:     os.Getenv("ANTHROPIC_API_KEY"),
		GitHubAppID:         os.Getenv("GITHUB_APP_ID"),
		GitHubAppSlug:       os.Getenv("GITHUB_APP_SLUG"),
		GitHubAppPrivateKey: os.Getenv("GITHUB_APP_PRIVATE_KEY"),
		PostmarkServerToken: os.Getenv("POSTMARK_SERVER_TOKEN"),
		PostmarkFromEmail:   os.Getenv("POSTMARK_FROM_EMAIL"),
		AppBaseURL:          appBaseURL,
	}, nil
}
