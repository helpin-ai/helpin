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
	NatsURL               string

	// S3 / object storage (optional — attachments disabled if not set)
	AWSAccessKeyID     string
	AWSSecretAccessKey string
	AWSBucket          string
	AWSRegion          string
	AWSEndpointURL     string // For MinIO / local dev

	// Anthropic API (optional — agent/orchestration features disabled if not set)
	AnthropicAPIKey      string
	OpenAIAPIKey         string
	OpenAIBaseURL        string
	OpenAIEmbeddingModel string
	OpenRouterAPIKey     string
	OpenRouterBaseURL    string
	OpenCodePath         string
	BraveSearchAPIKey    string

	// GitHub App (optional — required for shared-runner repo mutation).
	// GITHUB_APP_PRIVATE_KEY should be provided as a base64-encoded PEM value.
	GitHubAppID         string
	GitHubAppSlug       string
	GitHubAppPrivateKey string

	// Postmark email (optional — email sending disabled if not set)
	PostmarkServerToken          string
	PostmarkFromEmail            string
	PostmarkInboundWebhookSecret string
	SupportEmailReplyDomain      string
	AppBaseURL                   string

	// CRM encryption & Gmail OAuth (optional — Gmail sync disabled if not set)
	CRMEncryptionKey      string
	GmailClientID         string
	GmailClientSecret     string
	GmailOAuthRedirectURL string

	// CRM LLM provider selection (optional — defaults to "claude")
	CRMLLMProvider string // "claude" (default) or "openai"
	CRMLLMAPIKey   string
	CRMLLMBaseURL  string
	CRMLLMModel    string

	// Redis (optional — empty = local-only mode, no cross-pod broadcasting)
	RedisURL string
}

// Load reads configuration from environment variables.
// DATABASE_URL and JWT_SECRET are required; PORT defaults to "8080",
// CORS_ORIGINS is a comma-separated list of allowed origins (defaults to "http://localhost:5173").
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

	corsOrigins := parseCORSOrigins(os.Getenv("CORS_ORIGINS"))

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
		DatabaseURL:                  dbURL,
		JWTSecret:                    jwtSecret,
		Port:                         port,
		CORSOrigins:                  corsOrigins,
		TemporalAddress:              temporalAddress,
		TemporalNamespace:            temporalNamespace,
		TemporalAPIKey:               temporalAPIKey,
		TemporalTLSEnabled:           temporalTLSEnabled,
		TemporalTLSServerName:        strings.TrimSpace(os.Getenv("TEMPORAL_TLS_SERVER_NAME")),
		NatsURL:                      strings.TrimSpace(firstNonEmpty(os.Getenv("NATS_URL"), "nats://localhost:4222")),
		AWSAccessKeyID:               os.Getenv("AWS_ACCESS_KEY_ID"),
		AWSSecretAccessKey:           os.Getenv("AWS_SECRET_ACCESS_KEY"),
		AWSBucket:                    os.Getenv("AWS_S3_BUCKET_NAME"),
		AWSRegion:                    os.Getenv("AWS_REGION"),
		AWSEndpointURL:               os.Getenv("AWS_S3_ENDPOINT_URL"),
		AnthropicAPIKey:              os.Getenv("ANTHROPIC_API_KEY"),
		OpenAIAPIKey:                 strings.TrimSpace(os.Getenv("OPENAI_API_KEY")),
		OpenAIBaseURL:                strings.TrimSpace(os.Getenv("OPENAI_BASE_URL")),
		OpenAIEmbeddingModel:         strings.TrimSpace(os.Getenv("OPENAI_EMBEDDING_MODEL")),
		OpenRouterAPIKey:             strings.TrimSpace(os.Getenv("OPENROUTER_API_KEY")),
		OpenRouterBaseURL:            strings.TrimSpace(os.Getenv("OPENROUTER_BASE_URL")),
		OpenCodePath:                 strings.TrimSpace(firstNonEmpty(os.Getenv("OPENCODE_PATH"), "opencode")),
		BraveSearchAPIKey:            strings.TrimSpace(os.Getenv("BRAVE_SEARCH_API_KEY")),
		GitHubAppID:                  os.Getenv("GITHUB_APP_ID"),
		GitHubAppSlug:                os.Getenv("GITHUB_APP_SLUG"),
		GitHubAppPrivateKey:          os.Getenv("GITHUB_APP_PRIVATE_KEY"),
		PostmarkServerToken:          os.Getenv("POSTMARK_SERVER_TOKEN"),
		PostmarkFromEmail:            os.Getenv("POSTMARK_FROM_EMAIL"),
		PostmarkInboundWebhookSecret: strings.TrimSpace(os.Getenv("POSTMARK_INBOUND_WEBHOOK_SECRET")),
		SupportEmailReplyDomain:      strings.TrimSpace(firstNonEmpty(os.Getenv("SUPPORT_EMAIL_REPLY_DOMAIN"), "replies.helpin.ai")),
		AppBaseURL:                   appBaseURL,
		CRMEncryptionKey:             os.Getenv("CRM_ENCRYPTION_KEY"),
		GmailClientID:                os.Getenv("GMAIL_CLIENT_ID"),
		GmailClientSecret:            os.Getenv("GMAIL_CLIENT_SECRET"),
		GmailOAuthRedirectURL:        os.Getenv("GMAIL_OAUTH_REDIRECT_URL"),
		CRMLLMProvider:               os.Getenv("CRM_LLM_PROVIDER"),
		CRMLLMAPIKey:                 os.Getenv("CRM_LLM_API_KEY"),
		CRMLLMBaseURL:                os.Getenv("CRM_LLM_BASE_URL"),
		CRMLLMModel:                  os.Getenv("CRM_LLM_MODEL"),
		RedisURL:                     os.Getenv("REDIS_URL"),
	}, nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func parseCORSOrigins(value string) []string {
	if value == "" {
		return []string{"http://localhost:5173"}
	}
	var origins []string
	for _, o := range strings.Split(value, ",") {
		o = strings.TrimSpace(o)
		if o != "" {
			origins = append(origins, o)
		}
	}
	if len(origins) == 0 {
		return []string{"http://localhost:5173"}
	}
	return origins
}

func parseBoolEnv(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}
