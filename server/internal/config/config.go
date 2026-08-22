package config

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
)

const (
	defaultCommandRouterLLMProvider                  = "openrouter"
	defaultCommandRouterLLMModel                     = "openai/gpt-5.6-luna"
	defaultCommandRouterOpenRouterProviderOptionsRaw = `{"order":["google-vertex/global"],"allow_fallbacks":false}`
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

	// Agent Runtime sidecar/service integration (optional; disabled when base URL is empty).
	AgentRuntimeBaseURL       string
	AgentRuntimeServiceToken  string
	AgentRuntimeAppID         string
	AgentRuntimeEventProtocol string
	AgentRuntimeLaunchEnabled bool

	// S3 / object storage (optional — attachments disabled if not set)
	AWSAccessKeyID     string
	AWSSecretAccessKey string
	AWSBucket          string
	AWSRegion          string
	AWSEndpointURL     string // S3-compatible API endpoint (MinIO / R2)
	AWSPublicBaseURL   string // Optional public asset base URL (R2 custom domain / CDN)

	// Anthropic API (optional — agent/orchestration features disabled if not set)
	AnthropicAPIKey  string
	AnthropicBaseURL string
	OpenAIAPIKey     string
	// FalAPIKey is used by the server-side image editing tool.
	FalAPIKey             string
	OpenAIBaseURL         string
	OpenAIEmbeddingModel  string
	SupportRerankerURL    string
	SupportRerankerModel  string
	SupportRerankerAPIKey string
	OpenRouterAPIKey      string
	OpenRouterBaseURL     string
	// Help center AI answer routing. Empty values resolve to a flash-tier
	// default on the first chat provider that has an API key configured.
	HelpcenterAnswerProvider string
	HelpcenterAnswerModel    string
	// Docs import AI conversion is an opt-in formatter for imported help articles.
	DocsImportAIConversionEnabled      bool
	DocsImportAIConversionProvider     string
	DocsImportAIConversionModel        string
	DocsImportAIConversionArticleLimit int
	CodexOpenAIAuthMode                string
	CodexEnableChatGPTOAuth            bool
	CodexChatGPTAccessToken            string
	CodexChatGPTAccountID              string
	CloudflareAccountID                string
	CloudflareAPIToken                 string
	CloudflareAPIBaseURL               string

	// Website content crawler (optional — controls crawl engine and proxy)
	CrawlerMode         string // "cloudflare", "local", or "cloudflare_with_fallback" (default)
	CrawlerProxyURLs    string // comma-separated proxy URLs for local crawler and agent fetch/crawl tools (e.g. Decodo/Smartproxy)
	GoogleWebRiskAPIKey string // optional; enables support-link reputation lookups

	// GitHub App (optional — required for shared-runner repo mutation).
	// GITHUB_APP_PRIVATE_KEY should be provided as a base64-encoded PEM value.
	GitHubAppID         string
	GitHubAppSlug       string
	GitHubAppPrivateKey string

	// GitOAuthEncryptionKey encrypts stored git provider tokens (GitHub App + GitLab PAT) at rest.
	GitOAuthEncryptionKey string

	// Postmark email (optional — email sending disabled if not set)
	PostmarkAccountToken              string
	PostmarkAppServerToken            string
	PostmarkAppFromEmail              string
	PostmarkReplyServerToken          string
	PostmarkReplyFromEmail            string
	PostmarkReplyInboundWebhookSecret string
	SupportEmailReplyDomain           string
	PostmarkRouteServerToken          string
	PostmarkRouteInboundWebhookSecret string
	SupportEmailRouteDomain           string
	AppBaseURL                        string
	MobileAppBaseURL                  string
	MCPServerEnabled                  bool
	MCPOAuthEnabled                   bool
	MCPServiceTokensEnabled           bool
	MCPPMWriteEnabled                 bool
	MCPDocsWriteEnabled               bool
	MCPAgentRunEnabled                bool
	MCPCRMEnabled                     bool
	MCPSupportEnabled                 bool
	MCPPublicBaseURL                  string
	ExternalMCPEnabled                bool
	ExternalMCPEncryptionKey          string
	ExternalMCPAllowedHosts           []string
	ExternalMCPOAuthRedirectURL       string
	ExternalMCPOAuthClientID          string
	ExternalMCPOAuthClientSecret      string
	ExternalMCPOAuthClientAuthMethod  string
	ExternalMCPAllowInsecureLocalhost bool
	WebAuthnRPID                      string
	WebAuthnRPOrigins                 []string
	PlatformAdminEmails               []string

	// Google account sign-in (optional — Google button disabled if unset)
	GoogleAuthClientID     string
	GoogleAuthClientSecret string
	GoogleAuthRedirectURL  string

	// CRM encryption & Gmail OAuth (optional — Gmail sync disabled if not set)
	TOTPEncryptionKey     string
	CRMEncryptionKey      string
	PMImportEncryptionKey string
	GmailClientID         string
	GmailClientSecret     string
	GmailOAuthRedirectURL string

	// CRM LLM provider selection (optional — defaults to "claude")
	CRMLLMProvider string // "claude" (default) or "openai"
	CRMLLMAPIKey   string
	CRMLLMBaseURL  string
	CRMLLMModel    string

	// CRM meeting capture providers. Selection is deployment-owned and defaults to Recall.
	CRMMeetingCaptureProvider string
	RecallBaseURL             string
	RecallAPIKey              string
	RecallWebhookSecret       string
	VexaBaseURL               string
	VexaAPIKey                string
	VexaWebhookSecret         string

	// Query expansion for support AI RAG pipeline (optional — defaults to openai/gpt-5.6-luna)
	QueryExpansionModel     string
	QueryExpansionProvider  string
	QueryExpansionTimeoutMS int

	// Command bar intent routing LLM (optional — defaults to router default provider)
	CommandRouterLLMProvider               string
	CommandRouterLLMModel                  string
	CommandRouterLLMMaxTokens              int
	CommandRouterLLMTimeoutMS              int
	CommandRouterOpenRouterProviderOptions json.RawMessage

	// MaxMind GeoIP configuration (optional — enables GeoIP enrichment for support/widget traffic)
	MaxMindAccountID   string
	MaxMindDBPath      string
	MaxMindDownloadURL string
	MaxMindLicenseKey  string

	// Redis (optional — empty = local-only mode, no cross-pod broadcasting)
	RedisURL string

	// Stripe Billing (optional — checkout and portal disabled if unset)
	StripePublishableKey        string
	StripeSecretKey             string
	StripeWebhookSecret         string
	StripeStarterMonthlyPriceID string
	StripeStarterAnnualPriceID  string
	StripeGrowthMonthlyPriceID  string
	StripeGrowthAnnualPriceID   string

	// Customer.io Track API (optional — backend identity/object sync disabled if unset)
	CustomerIOSiteID                   string
	CustomerIOTrackAPIKey              string
	CustomerIORegion                   string
	CustomerIOWorkspaceObjectTypeID    string
	CustomerIOOrganizationObjectTypeID string

	// Usermaven Events API (optional — backend product analytics disabled if unset)
	UsermavenAPIKey      string
	UsermavenServerToken string
	UsermavenEndpoint    string

	// Agent preview debugging (optional — targeted diagnostics for preview persistence/apply)
	// Firebase Cloud Messaging (optional — mobile push notifications disabled if unset)
	FCMServiceAccountJSON string

	AgentPreviewDebug bool

	// Docs ordering: when true, reads/writes use fractional sort_key
	// instead of integer position. Enable after backfill completes.
	DocsOrderingUseSortKey bool

	// TLSAskExtraAllowedDomains lists additional hostnames approved by the Caddy
	// on-demand TLS ask endpoint. Entries prefixed with "." or "*." match as
	// suffixes; anything else matches exactly.
	TLSAskExtraAllowedDomains []string
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

	webAuthnRPID := strings.TrimSpace(os.Getenv("WEBAUTHN_RP_ID"))
	if webAuthnRPID == "" {
		webAuthnRPID = originHost(appBaseURL)
	}
	webAuthnRPOrigins := parseOptionalOrigins(os.Getenv("WEBAUTHN_RP_ORIGIN"))
	if len(webAuthnRPOrigins) == 0 {
		webAuthnRPOrigins = []string{originOnly(appBaseURL)}
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
	commandRouterOpenRouterProviderOptions, err := parseOptionalJSONObjectEnv(
		"COMMAND_ROUTER_OPENROUTER_PROVIDER_OPTIONS",
		firstNonEmpty(os.Getenv("COMMAND_ROUTER_OPENROUTER_PROVIDER_OPTIONS"), defaultCommandRouterOpenRouterProviderOptionsRaw),
	)
	if err != nil {
		return nil, err
	}
	meetingCaptureProvider := strings.ToLower(strings.TrimSpace(firstNonEmpty(os.Getenv("CRM_MEETING_CAPTURE_PROVIDER"), "recall")))
	if meetingCaptureProvider != "recall" && meetingCaptureProvider != "vexa" {
		return nil, fmt.Errorf("CRM_MEETING_CAPTURE_PROVIDER must be recall or vexa")
	}

	return &Config{
		DatabaseURL:                            dbURL,
		JWTSecret:                              jwtSecret,
		Port:                                   port,
		LogLevel:                               strings.TrimSpace(firstNonEmpty(os.Getenv("LOG_LEVEL"), "info")),
		RunAutoMigrate:                         parseBoolEnvDefaultTrue(os.Getenv("RUN_AUTO_MIGRATE")),
		CORSOrigins:                            corsOrigins,
		TemporalAddress:                        temporalAddress,
		TemporalNamespace:                      temporalNamespace,
		TemporalAPIKey:                         temporalAPIKey,
		TemporalTLSEnabled:                     temporalTLSEnabled,
		TemporalTLSServerName:                  strings.TrimSpace(os.Getenv("TEMPORAL_TLS_SERVER_NAME")),
		NatsURL:                                strings.TrimSpace(firstNonEmpty(os.Getenv("NATS_URL"), "nats://localhost:4222")),
		AgentRuntimeBaseURL:                    strings.TrimRight(strings.TrimSpace(os.Getenv("AGENT_RUNTIME_BASE_URL")), "/"),
		AgentRuntimeServiceToken:               strings.TrimSpace(os.Getenv("AGENT_RUNTIME_SERVICE_TOKEN")),
		AgentRuntimeAppID:                      strings.TrimSpace(firstNonEmpty(os.Getenv("AGENT_RUNTIME_APP_ID"), "helpin")),
		AgentRuntimeEventProtocol:              strings.ToLower(strings.TrimSpace(firstNonEmpty(os.Getenv("AGENT_RUNTIME_EVENT_PROTOCOL"), "v1"))),
		AgentRuntimeLaunchEnabled:              parseBoolEnv(os.Getenv("AGENT_RUNTIME_LAUNCH_ENABLED")),
		AWSAccessKeyID:                         os.Getenv("AWS_ACCESS_KEY_ID"),
		AWSSecretAccessKey:                     os.Getenv("AWS_SECRET_ACCESS_KEY"),
		AWSBucket:                              os.Getenv("AWS_S3_BUCKET_NAME"),
		AWSRegion:                              os.Getenv("AWS_REGION"),
		AWSEndpointURL:                         os.Getenv("AWS_S3_ENDPOINT_URL"),
		AWSPublicBaseURL:                       strings.TrimSpace(os.Getenv("AWS_S3_PUBLIC_BASE_URL")),
		AnthropicAPIKey:                        os.Getenv("ANTHROPIC_API_KEY"),
		AnthropicBaseURL:                       strings.TrimSpace(os.Getenv("ANTHROPIC_BASE_URL")),
		OpenAIAPIKey:                           strings.TrimSpace(os.Getenv("OPENAI_API_KEY")),
		FalAPIKey:                              strings.TrimSpace(os.Getenv("FAL_KEY")),
		OpenAIBaseURL:                          strings.TrimSpace(os.Getenv("OPENAI_BASE_URL")),
		OpenAIEmbeddingModel:                   strings.TrimSpace(os.Getenv("OPENAI_EMBEDDING_MODEL")),
		SupportRerankerURL:                     strings.TrimRight(strings.TrimSpace(os.Getenv("SUPPORT_RERANKER_URL")), "/"),
		SupportRerankerModel:                   strings.TrimSpace(os.Getenv("SUPPORT_RERANKER_MODEL")),
		SupportRerankerAPIKey:                  strings.TrimSpace(os.Getenv("SUPPORT_RERANKER_API_KEY")),
		OpenRouterAPIKey:                       strings.TrimSpace(os.Getenv("OPENROUTER_API_KEY")),
		OpenRouterBaseURL:                      strings.TrimSpace(os.Getenv("OPENROUTER_BASE_URL")),
		HelpcenterAnswerProvider:               strings.TrimSpace(os.Getenv("HELPCENTER_ANSWER_PROVIDER")),
		HelpcenterAnswerModel:                  strings.TrimSpace(os.Getenv("HELPCENTER_ANSWER_MODEL")),
		DocsImportAIConversionEnabled:          parseBoolEnv(os.Getenv("DOCS_IMPORT_AI_CONVERSION_ENABLED")),
		DocsImportAIConversionProvider:         strings.TrimSpace(firstNonEmpty(os.Getenv("DOCS_IMPORT_AI_CONVERSION_PROVIDER"), "openrouter")),
		DocsImportAIConversionModel:            strings.TrimSpace(firstNonEmpty(os.Getenv("DOCS_IMPORT_AI_CONVERSION_MODEL"), "openai/gpt-5.6-luna")),
		DocsImportAIConversionArticleLimit:     parsePositiveIntEnv(os.Getenv("DOCS_IMPORT_AI_CONVERSION_ARTICLE_LIMIT"), 0),
		CodexOpenAIAuthMode:                    strings.TrimSpace(firstNonEmpty(os.Getenv("CODEX_OPENAI_AUTH_MODE"), "api_key")),
		CodexEnableChatGPTOAuth:                parseBoolEnv(os.Getenv("CODEX_ENABLE_CHATGPT_OAUTH")),
		CodexChatGPTAccessToken:                strings.TrimSpace(os.Getenv("CODEX_CHATGPT_ACCESS_TOKEN")),
		CodexChatGPTAccountID:                  strings.TrimSpace(os.Getenv("CODEX_CHATGPT_ACCOUNT_ID")),
		CloudflareAccountID:                    strings.TrimSpace(os.Getenv("CLOUDFLARE_ACCOUNT_ID")),
		CloudflareAPIToken:                     strings.TrimSpace(os.Getenv("CLOUDFLARE_API_TOKEN")),
		CloudflareAPIBaseURL:                   strings.TrimSpace(os.Getenv("CLOUDFLARE_API_BASE_URL")),
		CrawlerMode:                            strings.TrimSpace(firstNonEmpty(os.Getenv("CRAWLER_MODE"), "cloudflare_with_fallback")),
		CrawlerProxyURLs:                       strings.TrimSpace(os.Getenv("CRAWLER_PROXY_URLS")),
		GoogleWebRiskAPIKey:                    strings.TrimSpace(os.Getenv("GOOGLE_WEB_RISK_API_KEY")),
		GitHubAppID:                            os.Getenv("GITHUB_APP_ID"),
		GitHubAppSlug:                          os.Getenv("GITHUB_APP_SLUG"),
		GitHubAppPrivateKey:                    os.Getenv("GITHUB_APP_PRIVATE_KEY"),
		GitOAuthEncryptionKey:                  strings.TrimSpace(os.Getenv("GIT_OAUTH_ENCRYPTION_KEY")),
		PostmarkAccountToken:                   strings.TrimSpace(os.Getenv("POSTMARK_ACCOUNT_TOKEN")),
		PostmarkAppServerToken:                 strings.TrimSpace(firstNonEmpty(os.Getenv("POSTMARK_APP_SERVER_TOKEN"), os.Getenv("POSTMARK_SERVER_TOKEN"))),
		PostmarkAppFromEmail:                   strings.TrimSpace(firstNonEmpty(os.Getenv("POSTMARK_APP_FROM_EMAIL"), os.Getenv("POSTMARK_FROM_EMAIL"))),
		PostmarkReplyServerToken:               strings.TrimSpace(firstNonEmpty(os.Getenv("POSTMARK_REPLY_SERVER_TOKEN"), os.Getenv("POSTMARK_SERVER_TOKEN"))),
		PostmarkReplyFromEmail:                 strings.TrimSpace(firstNonEmpty(os.Getenv("POSTMARK_REPLY_FROM_EMAIL"), os.Getenv("POSTMARK_FROM_EMAIL"))),
		PostmarkReplyInboundWebhookSecret:      strings.TrimSpace(firstNonEmpty(os.Getenv("POSTMARK_REPLY_INBOUND_WEBHOOK_SECRET"), os.Getenv("POSTMARK_INBOUND_WEBHOOK_SECRET"))),
		SupportEmailReplyDomain:                strings.TrimSpace(firstNonEmpty(os.Getenv("SUPPORT_EMAIL_REPLY_DOMAIN"), "replies.helpin.email")),
		PostmarkRouteServerToken:               strings.TrimSpace(os.Getenv("POSTMARK_ROUTE_SERVER_TOKEN")),
		PostmarkRouteInboundWebhookSecret:      strings.TrimSpace(firstNonEmpty(os.Getenv("POSTMARK_ROUTE_INBOUND_WEBHOOK_SECRET"), os.Getenv("POSTMARK_INBOUND_WEBHOOK_SECRET"))),
		SupportEmailRouteDomain:                strings.TrimSpace(firstNonEmpty(os.Getenv("SUPPORT_EMAIL_ROUTE_DOMAIN"), os.Getenv("SUPPORT_EMAIL_REPLY_DOMAIN"), "on.helpin.email")),
		AppBaseURL:                             appBaseURL,
		MobileAppBaseURL:                       strings.TrimRight(strings.TrimSpace(os.Getenv("MOBILE_APP_BASE_URL")), "/"),
		MCPServerEnabled:                       parseBoolEnvDefaultTrue(os.Getenv("MCP_SERVER_ENABLED")),
		MCPOAuthEnabled:                        parseBoolEnvDefaultTrue(os.Getenv("MCP_OAUTH_ENABLED")),
		MCPServiceTokensEnabled:                parseBoolEnvDefaultTrue(os.Getenv("MCP_SERVICE_TOKENS_ENABLED")),
		MCPPMWriteEnabled:                      parseBoolEnvDefaultTrue(os.Getenv("MCP_PM_WRITE_ENABLED")),
		MCPDocsWriteEnabled:                    parseBoolEnvDefaultTrue(os.Getenv("MCP_DOCS_WRITE_ENABLED")),
		MCPAgentRunEnabled:                     parseBoolEnvDefaultTrue(os.Getenv("MCP_AGENT_RUN_ENABLED")),
		MCPCRMEnabled:                          parseBoolEnvDefaultTrue(os.Getenv("MCP_CRM_ENABLED")),
		MCPSupportEnabled:                      parseBoolEnvDefaultTrue(os.Getenv("MCP_SUPPORT_ENABLED")),
		MCPPublicBaseURL:                       strings.TrimRight(strings.TrimSpace(firstNonEmpty(os.Getenv("MCP_PUBLIC_BASE_URL"), appBaseURL)), "/"),
		ExternalMCPEnabled:                     parseBoolEnv(os.Getenv("EXTERNAL_MCP_ENABLED")),
		ExternalMCPEncryptionKey:               strings.TrimSpace(os.Getenv("EXTERNAL_MCP_ENCRYPTION_KEY")),
		ExternalMCPAllowedHosts:                parseCSV(firstNonEmpty(os.Getenv("EXTERNAL_MCP_ALLOWED_HOSTS"), "*")),
		ExternalMCPOAuthRedirectURL:            strings.TrimSpace(os.Getenv("EXTERNAL_MCP_OAUTH_REDIRECT_URL")),
		ExternalMCPOAuthClientID:               strings.TrimSpace(os.Getenv("EXTERNAL_MCP_OAUTH_CLIENT_ID")),
		ExternalMCPOAuthClientSecret:           strings.TrimSpace(os.Getenv("EXTERNAL_MCP_OAUTH_CLIENT_SECRET")),
		ExternalMCPOAuthClientAuthMethod:       strings.TrimSpace(firstNonEmpty(os.Getenv("EXTERNAL_MCP_OAUTH_CLIENT_AUTH_METHOD"), "none")),
		ExternalMCPAllowInsecureLocalhost:      parseBoolEnv(os.Getenv("EXTERNAL_MCP_ALLOW_INSECURE_LOCALHOST")),
		WebAuthnRPID:                           webAuthnRPID,
		WebAuthnRPOrigins:                      webAuthnRPOrigins,
		PlatformAdminEmails:                    parseCSV(os.Getenv("PLATFORM_ADMIN_EMAILS")),
		GoogleAuthClientID:                     strings.TrimSpace(os.Getenv("GOOGLE_AUTH_CLIENT_ID")),
		GoogleAuthClientSecret:                 strings.TrimSpace(os.Getenv("GOOGLE_AUTH_CLIENT_SECRET")),
		GoogleAuthRedirectURL:                  strings.TrimSpace(os.Getenv("GOOGLE_AUTH_REDIRECT_URL")),
		TOTPEncryptionKey:                      strings.TrimSpace(os.Getenv("TOTP_ENCRYPTION_KEY")),
		CRMEncryptionKey:                       os.Getenv("CRM_ENCRYPTION_KEY"),
		PMImportEncryptionKey:                  strings.TrimSpace(os.Getenv("PM_IMPORT_ENCRYPTION_KEY")),
		GmailClientID:                          os.Getenv("GMAIL_CLIENT_ID"),
		GmailClientSecret:                      os.Getenv("GMAIL_CLIENT_SECRET"),
		GmailOAuthRedirectURL:                  os.Getenv("GMAIL_OAUTH_REDIRECT_URL"),
		CRMLLMProvider:                         os.Getenv("CRM_LLM_PROVIDER"),
		CRMLLMAPIKey:                           os.Getenv("CRM_LLM_API_KEY"),
		CRMLLMBaseURL:                          os.Getenv("CRM_LLM_BASE_URL"),
		CRMLLMModel:                            os.Getenv("CRM_LLM_MODEL"),
		CRMMeetingCaptureProvider:              meetingCaptureProvider,
		RecallBaseURL:                          strings.TrimRight(strings.TrimSpace(os.Getenv("RECALL_BASE_URL")), "/"),
		RecallAPIKey:                           strings.TrimSpace(os.Getenv("RECALL_API_KEY")),
		RecallWebhookSecret:                    strings.TrimSpace(os.Getenv("RECALL_WEBHOOK_SECRET")),
		VexaBaseURL:                            strings.TrimRight(strings.TrimSpace(os.Getenv("VEXA_BASE_URL")), "/"),
		VexaAPIKey:                             strings.TrimSpace(os.Getenv("VEXA_API_KEY")),
		VexaWebhookSecret:                      strings.TrimSpace(os.Getenv("VEXA_WEBHOOK_SECRET")),
		QueryExpansionModel:                    strings.TrimSpace(firstNonEmpty(os.Getenv("QUERY_EXPANSION_MODEL"), "gpt-5.6-luna")),
		QueryExpansionProvider:                 strings.TrimSpace(firstNonEmpty(os.Getenv("QUERY_EXPANSION_PROVIDER"), "openai")),
		QueryExpansionTimeoutMS:                parsePositiveIntEnv(os.Getenv("QUERY_EXPANSION_TIMEOUT_MS"), 10000),
		CommandRouterLLMProvider:               strings.TrimSpace(firstNonEmpty(os.Getenv("COMMAND_ROUTER_LLM_PROVIDER"), defaultCommandRouterLLMProvider)),
		CommandRouterLLMModel:                  strings.TrimSpace(firstNonEmpty(os.Getenv("COMMAND_ROUTER_LLM_MODEL"), defaultCommandRouterLLMModel)),
		CommandRouterLLMMaxTokens:              parsePositiveIntEnv(os.Getenv("COMMAND_ROUTER_LLM_MAX_TOKENS"), 900),
		CommandRouterLLMTimeoutMS:              parsePositiveIntEnv(os.Getenv("COMMAND_ROUTER_LLM_TIMEOUT_MS"), 8000),
		CommandRouterOpenRouterProviderOptions: commandRouterOpenRouterProviderOptions,
		MaxMindAccountID:                       strings.TrimSpace(os.Getenv("MAXMIND_ACCOUNT_ID")),
		MaxMindDBPath:                          strings.TrimSpace(os.Getenv("MAXMIND_DB_PATH")),
		MaxMindDownloadURL:                     strings.TrimSpace(os.Getenv("MAXMIND_DOWNLOAD_URL")),
		MaxMindLicenseKey:                      strings.TrimSpace(os.Getenv("MAXMIND_LICENSE_KEY")),
		RedisURL:                               os.Getenv("REDIS_URL"),
		StripePublishableKey:                   strings.TrimSpace(os.Getenv("STRIPE_PUBLISHABLE_KEY")),
		StripeSecretKey:                        strings.TrimSpace(os.Getenv("STRIPE_SECRET_KEY")),
		StripeWebhookSecret:                    strings.TrimSpace(os.Getenv("STRIPE_WEBHOOK_SECRET")),
		StripeStarterMonthlyPriceID:            strings.TrimSpace(os.Getenv("STRIPE_STARTER_MONTHLY_PRICE_ID")),
		StripeStarterAnnualPriceID:             strings.TrimSpace(os.Getenv("STRIPE_STARTER_ANNUAL_PRICE_ID")),
		StripeGrowthMonthlyPriceID:             strings.TrimSpace(os.Getenv("STRIPE_GROWTH_MONTHLY_PRICE_ID")),
		StripeGrowthAnnualPriceID:              strings.TrimSpace(os.Getenv("STRIPE_GROWTH_ANNUAL_PRICE_ID")),
		CustomerIOSiteID:                       strings.TrimSpace(os.Getenv("CUSTOMER_IO_SITE_ID")),
		CustomerIOTrackAPIKey:                  strings.TrimSpace(os.Getenv("CUSTOMER_IO_TRACK_API_KEY")),
		CustomerIORegion:                       strings.TrimSpace(firstNonEmpty(os.Getenv("CUSTOMER_IO_REGION"), "us")),
		CustomerIOWorkspaceObjectTypeID:        strings.TrimSpace(firstNonEmpty(os.Getenv("CUSTOMER_IO_WORKSPACE_OBJECT_TYPE_ID"), "1")),
		CustomerIOOrganizationObjectTypeID:     strings.TrimSpace(firstNonEmpty(os.Getenv("CUSTOMER_IO_ORGANIZATION_OBJECT_TYPE_ID"), "2")),
		UsermavenAPIKey:                        strings.TrimSpace(os.Getenv("USERMAVEN_API_KEY")),
		UsermavenServerToken:                   strings.TrimSpace(os.Getenv("USERMAVEN_SERVER_TOKEN")),
		UsermavenEndpoint:                      strings.TrimSpace(os.Getenv("USERMAVEN_ENDPOINT")),
		FCMServiceAccountJSON:                  strings.TrimSpace(os.Getenv("FCM_SERVICE_ACCOUNT_JSON")),
		AgentPreviewDebug:                      parseBoolEnv(os.Getenv("AGENT_PREVIEW_DEBUG")),
		DocsOrderingUseSortKey:                 parseBoolEnv(os.Getenv("DOCS_ORDERING_USE_SORT_KEY")),
		TLSAskExtraAllowedDomains:              parseCSV(os.Getenv("TLS_ASK_EXTRA_ALLOWED_DOMAINS")),
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

func parsePositiveIntEnv(value string, fallback int) int {
	parsed, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil || parsed <= 0 {
		return fallback
	}
	return parsed
}

func parseOptionalJSONObjectEnv(name, value string) (json.RawMessage, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, nil
	}
	var raw json.RawMessage
	if err := json.Unmarshal([]byte(value), &raw); err != nil {
		return nil, fmt.Errorf("%s must be a valid JSON object: %w", name, err)
	}
	var object map[string]any
	if err := json.Unmarshal(raw, &object); err != nil {
		return nil, fmt.Errorf("%s must be a JSON object", name)
	}
	if object == nil {
		return nil, fmt.Errorf("%s must be a JSON object", name)
	}
	return append(json.RawMessage(nil), raw...), nil
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

func parseOptionalOrigins(value string) []string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	var origins []string
	for _, origin := range strings.Split(value, ",") {
		if cleaned := originOnly(origin); cleaned != "" {
			origins = append(origins, cleaned)
		}
	}
	return origins
}

func parseCSV(value string) []string {
	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts))
	seen := make(map[string]struct{}, len(parts))
	for _, part := range parts {
		cleaned := strings.TrimSpace(part)
		if cleaned == "" {
			continue
		}
		key := strings.ToLower(cleaned)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, cleaned)
	}
	return out
}

func originHost(value string) string {
	cleaned := originOnly(value)
	if cleaned == "" {
		return ""
	}
	if parsed, err := url.Parse(cleaned); err == nil && parsed.Host != "" {
		return parsed.Host
	}
	return cleaned
}

func originOnly(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	value = strings.TrimSuffix(value, "/")
	if strings.Contains(value, "://") {
		if parsed, err := url.Parse(value); err == nil && parsed.Scheme != "" && parsed.Host != "" {
			return parsed.Scheme + "://" + parsed.Host
		}
	}
	return value
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
