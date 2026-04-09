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
	LogLevel              string
	RunAutoMigrate        bool
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
	AWSEndpointURL     string // S3-compatible API endpoint (MinIO / R2)
	AWSPublicBaseURL   string // Optional public asset base URL (R2 custom domain / CDN)

	// Anthropic API (optional — agent/orchestration features disabled if not set)
	AnthropicAPIKey         string
	AnthropicBaseURL        string
	OpenAIAPIKey            string
	OpenAIBaseURL           string
	OpenAIEmbeddingModel    string
	OpenRouterAPIKey        string
	OpenRouterBaseURL       string
	OpenCodePath            string
	CodexPath               string
	CodexModel              string
	CodexSandboxMode        string
	CodexOpenAIAuthMode     string
	CodexEnableChatGPTOAuth bool
	CodexChatGPTAccessToken string
	CodexChatGPTAccountID   string
	CodexChatGPTPlanType    string
	CodexAuthEncryptionKey  string
	BraveSearchAPIKey       string
	CloudflareAccountID     string
	CloudflareAPIToken      string
	CloudflareAPIBaseURL    string

	// Website content crawler (optional — controls crawl engine and proxy)
	CrawlerMode      string // "cloudflare", "local", or "cloudflare_with_fallback" (default)
	CrawlerProxyURLs string // comma-separated proxy URLs for local crawler (e.g. Decodo/Smartproxy)

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

	// Query expansion for support AI RAG pipeline (optional — defaults to openai/gpt-5.4-mini)
	QueryExpansionModel    string
	QueryExpansionProvider string

	// Redis (optional — empty = local-only mode, no cross-pod broadcasting)
	RedisURL string

	// Agent preview debugging (optional — targeted diagnostics for preview persistence/apply)
	AgentPreviewDebug bool
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
		LogLevel:                     strings.TrimSpace(firstNonEmpty(os.Getenv("LOG_LEVEL"), "info")),
		RunAutoMigrate:               parseBoolEnvDefaultTrue(os.Getenv("RUN_AUTO_MIGRATE")),
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
		AWSPublicBaseURL:             strings.TrimSpace(os.Getenv("AWS_S3_PUBLIC_BASE_URL")),
		AnthropicAPIKey:              os.Getenv("ANTHROPIC_API_KEY"),
		AnthropicBaseURL:             strings.TrimSpace(os.Getenv("ANTHROPIC_BASE_URL")),
		OpenAIAPIKey:                 strings.TrimSpace(os.Getenv("OPENAI_API_KEY")),
		OpenAIBaseURL:                strings.TrimSpace(os.Getenv("OPENAI_BASE_URL")),
		OpenAIEmbeddingModel:         strings.TrimSpace(os.Getenv("OPENAI_EMBEDDING_MODEL")),
		OpenRouterAPIKey:             strings.TrimSpace(os.Getenv("OPENROUTER_API_KEY")),
		OpenRouterBaseURL:            strings.TrimSpace(os.Getenv("OPENROUTER_BASE_URL")),
		OpenCodePath:                 strings.TrimSpace(firstNonEmpty(os.Getenv("OPENCODE_PATH"), "opencode")),
		CodexPath:                    strings.TrimSpace(firstNonEmpty(os.Getenv("CODEX_PATH"), "codex")),
		CodexModel:                   strings.TrimSpace(os.Getenv("CODEX_MODEL")),
		CodexSandboxMode:             strings.TrimSpace(os.Getenv("CODEX_SANDBOX_MODE")),
		CodexOpenAIAuthMode:          strings.TrimSpace(firstNonEmpty(os.Getenv("CODEX_OPENAI_AUTH_MODE"), "api_key")),
		CodexEnableChatGPTOAuth:      parseBoolEnv(os.Getenv("CODEX_ENABLE_CHATGPT_OAUTH")),
		CodexChatGPTAccessToken:      strings.TrimSpace(os.Getenv("CODEX_CHATGPT_ACCESS_TOKEN")),
		CodexChatGPTAccountID:        strings.TrimSpace(os.Getenv("CODEX_CHATGPT_ACCOUNT_ID")),
		CodexChatGPTPlanType:         strings.TrimSpace(os.Getenv("CODEX_CHATGPT_PLAN_TYPE")),
		CodexAuthEncryptionKey:       strings.TrimSpace(os.Getenv("CODEX_AUTH_ENCRYPTION_KEY")),
		BraveSearchAPIKey:            strings.TrimSpace(os.Getenv("BRAVE_SEARCH_API_KEY")),
		CloudflareAccountID:          strings.TrimSpace(os.Getenv("CLOUDFLARE_ACCOUNT_ID")),
		CloudflareAPIToken:           strings.TrimSpace(os.Getenv("CLOUDFLARE_API_TOKEN")),
		CloudflareAPIBaseURL:         strings.TrimSpace(os.Getenv("CLOUDFLARE_API_BASE_URL")),
		CrawlerMode:                  strings.TrimSpace(firstNonEmpty(os.Getenv("CRAWLER_MODE"), "cloudflare_with_fallback")),
		CrawlerProxyURLs:             strings.TrimSpace(os.Getenv("CRAWLER_PROXY_URLS")),
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
		QueryExpansionModel:          strings.TrimSpace(firstNonEmpty(os.Getenv("QUERY_EXPANSION_MODEL"), "gpt-5.4-mini")),
		QueryExpansionProvider:       strings.TrimSpace(firstNonEmpty(os.Getenv("QUERY_EXPANSION_PROVIDER"), "openai")),
		RedisURL:                     os.Getenv("REDIS_URL"),
		AgentPreviewDebug:            parseBoolEnv(os.Getenv("AGENT_PREVIEW_DEBUG")),
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

func parseBoolEnvDefaultTrue(value string) bool {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return true
	}
	return parseBoolEnv(trimmed)
}
