package config

import (
	"encoding/json"
	"fmt"
	"math"
	"net/url"
	"os"
	"strconv"
	"strings"

	"github.com/helpin-ai/helpin/server/internal/deployment"
	"github.com/helpin-ai/helpin/server/internal/model"
)

const (
	defaultCommandRouterLLMProvider                  = "openrouter"
	defaultCommandRouterLLMModel                     = "openai/gpt-5.6-luna"
	defaultCommandRouterOpenRouterProviderOptionsRaw = `{"order":["google-vertex/global"],"allow_fallbacks":false}`
)

// Config holds all application configuration loaded from environment variables.
type Config struct {
	AuthenticatedRateLimit    int
	ExpensiveRateLimit        int
	PublicWidgetURL           string
	PublicSDKURL              string
	SMTPHost                  string
	SMTPPort                  int
	SMTPUsername              string
	SMTPPassword              string
	SMTPFrom                  string
	SMTPTLSMode               string
	EmailVerificationRequired bool
	// DemoViewerEmail enables the public read-only demo login when set. It is the
	// email of the shared viewer account visitors are signed in as.
	DemoViewerEmail string
	// DemoRequireEmail makes the visitor email mandatory on POST /api/auth/demo.
	DemoRequireEmail bool
	// DemoLeadWebhookURL receives a JSON POST for every visitor email captured.
	DemoLeadWebhookURL    string
	EnabledModules        []model.ModuleID
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
	AWSAccessKeyID        string
	AWSSecretAccessKey    string
	AWSBucket             string
	AWSRegion             string
	AWSEndpointURL        string // S3-compatible API endpoint (MinIO / R2)
	AWSPresignEndpointURL string // Public S3 origin for browser-signed requests
	AWSPrivateBucket      bool
	AWSPublicBaseURL      string // Optional public asset base URL (R2 custom domain / CDN)

	// Anthropic API (optional — agent/orchestration features disabled if not set)
	AnthropicAPIKey  string
	AnthropicBaseURL string
	OpenAIAPIKey     string
	// FalAPIKey is used by the server-side image editing tool.
	FalAPIKey                   string
	OpenAIBaseURL               string
	OpenAIEmbeddingModel        string
	JevAPIKey                   string
	JevRoutingThreshold         float64
	JevTagThreshold             float64
	JevRoutingMode              string
	JevTagsMode                 string
	JevTimeoutMS                int
	JevWorkspaceIDs             string
	JevDailyLimit               int
	SupportDecisionMode         string
	SupportDecisionURL          string
	SupportDecisionToken        string
	SupportDecisionTimeoutMS    int
	SupportDecisionWorkspaceIDs string
	SupportDecisionPolicies     string
	SupportRerankerURL          string
	SupportRerankerModel        string
	SupportRerankerAPIKey       string
	OpenRouterAPIKey            string
	OpenRouterBaseURL           string
	// Help center AI answer routing. Empty values resolve to a flash-tier
	// default on the first chat provider that has an API key configured.
	HelpcenterAnswerProvider string
	HelpcenterAnswerModel    string
	// Docs import AI conversion is an opt-in formatter for imported help articles.
	DocsImportAIConversionEnabled      bool
	DocsImportAIConversionProvider     string
	DocsImportAIConversionModel        string
	DocsImportAIConversionArticleLimit int
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
	CLIEnabled                        bool
	CLIModelGatewayEnabled            bool
	CLIPublicBaseURL                  string
	MCPServerEnabled                  bool
	MCPOAuthEnabled                   bool
	MCPServiceTokensEnabled           bool
	MCPPMWriteEnabled                 bool
	MCPDocsWriteEnabled               bool
	MCPAgentRunEnabled                bool
	MCPCRMEnabled                     bool
	MCPSupportEnabled                 bool
	MCPPublicBaseURL                  string
	AIConnectionEncryptionKey         string
	ChatGPTConnectionsEnabled         bool
	ChatGPTClientID                   string
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

	// CRM direct-completion routes. Empty values preserve the reviewed
	// defaults in service.DefaultAICompletionRouteRegistry.
	CRMLLMProvider                       string
	CRMLLMModel                          string
	CRMLLMOpenRouterProvider             string
	CRMLLMFallbackProvider               string
	CRMLLMFallbackModel                  string
	CRMLLMFallbackOpenRouterProvider     string
	CRMMeetingFallbackProvider           string
	CRMMeetingFallbackModel              string
	CRMMeetingFallbackOpenRouterProvider string

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
	ClickHouseDSN        string

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

	smtpPort := 587
	if raw := os.Getenv("SMTP_PORT"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 1 || parsed > 65535 {
			return nil, fmt.Errorf("SMTP_PORT must be between 1 and 65535")
		}
		smtpPort = parsed
	}
	emailVerificationRequired, err := deployment.EmailVerificationPolicy(os.Getenv("AUTH_EMAIL_VERIFICATION_REQUIRED"))
	if err != nil {
		return nil, err
	}
	corsOrigins := parseCORSOrigins(os.Getenv("CORS_ORIGINS"))
	enabledModules, err := deployment.ParseModules(os.Getenv("HELPIN_ENABLED_MODULES"))
	if err != nil {
		return nil, err
	}

	appBaseURL := os.Getenv("APP_BASE_URL")
	if appBaseURL == "" {
		appBaseURL = "http://localhost:5173"
	}

	publicWidgetURL, err := publicURL(firstNonEmpty(os.Getenv("PUBLIC_WIDGET_URL"), deployment.DefaultWidgetOrigin, appBaseURL), true)
	if err != nil {
		return nil, fmt.Errorf("PUBLIC_WIDGET_URL: %w", err)
	}
	publicSDKURL, err := publicURL(firstNonEmpty(os.Getenv("PUBLIC_SDK_URL"), deployment.DefaultSDKLoaderURL, publicWidgetURL+"/sdk/lib.js"), false)
	if err != nil {
		return nil, fmt.Errorf("PUBLIC_SDK_URL: %w", err)
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
	crmLLMProvider := strings.TrimSpace(os.Getenv("CRM_LLM_PROVIDER"))
	crmLLMModel := strings.TrimSpace(os.Getenv("CRM_LLM_MODEL"))
	crmLLMOpenRouterProvider := strings.TrimSpace(os.Getenv("CRM_LLM_OPENROUTER_PROVIDER"))
	crmLLMFallbackProvider := strings.TrimSpace(os.Getenv("CRM_LLM_FALLBACK_PROVIDER"))
	crmLLMFallbackModel := strings.TrimSpace(os.Getenv("CRM_LLM_FALLBACK_MODEL"))
	crmLLMFallbackOpenRouterProvider := strings.TrimSpace(os.Getenv("CRM_LLM_FALLBACK_OPENROUTER_PROVIDER"))
	crmMeetingLLMFallbackProvider := strings.TrimSpace(os.Getenv("CRM_MEETING_LLM_FALLBACK_PROVIDER"))
	crmMeetingLLMFallbackModel := strings.TrimSpace(os.Getenv("CRM_MEETING_LLM_FALLBACK_MODEL"))
	crmMeetingLLMFallbackOpenRouterProvider := strings.TrimSpace(os.Getenv("CRM_MEETING_LLM_FALLBACK_OPENROUTER_PROVIDER"))
	if err := validateOptionalLLMRouteEnv("CRM_LLM_PROVIDER", crmLLMProvider, "CRM_LLM_MODEL", crmLLMModel); err != nil {
		return nil, err
	}
	if err := validateOptionalLLMRouteEnv("CRM_LLM_FALLBACK_PROVIDER", crmLLMFallbackProvider, "CRM_LLM_FALLBACK_MODEL", crmLLMFallbackModel); err != nil {
		return nil, err
	}
	if err := validateOptionalLLMRouteEnv("CRM_MEETING_LLM_FALLBACK_PROVIDER", crmMeetingLLMFallbackProvider, "CRM_MEETING_LLM_FALLBACK_MODEL", crmMeetingLLMFallbackModel); err != nil {
		return nil, err
	}
	if err := validateOpenRouterProviderEnv("CRM_LLM_OPENROUTER_PROVIDER", crmLLMOpenRouterProvider, crmLLMProvider, "openrouter"); err != nil {
		return nil, err
	}
	if err := validateOpenRouterProviderEnv("CRM_LLM_FALLBACK_OPENROUTER_PROVIDER", crmLLMFallbackOpenRouterProvider, crmLLMFallbackProvider, "openrouter"); err != nil {
		return nil, err
	}
	if err := validateOpenRouterProviderEnv("CRM_MEETING_LLM_FALLBACK_OPENROUTER_PROVIDER", crmMeetingLLMFallbackOpenRouterProvider, crmMeetingLLMFallbackProvider, "openrouter"); err != nil {
		return nil, err
	}
	meetingCaptureProvider := strings.ToLower(strings.TrimSpace(firstNonEmpty(os.Getenv("CRM_MEETING_CAPTURE_PROVIDER"), "recall")))
	if meetingCaptureProvider != "recall" && meetingCaptureProvider != "vexa" {
		return nil, fmt.Errorf("CRM_MEETING_CAPTURE_PROVIDER must be recall or vexa")
	}

	authenticatedRateLimit, err := rateLimitEnv("AUTHENTICATED_RATE_LIMIT_PER_MINUTE", 1200)
	if err != nil {
		return nil, err
	}
	expensiveRateLimit, err := rateLimitEnv("EXPENSIVE_RATE_LIMIT_PER_MINUTE", 120)
	if err != nil {
		return nil, err
	}
	return &Config{
		AuthenticatedRateLimit:                 authenticatedRateLimit,
		ExpensiveRateLimit:                     expensiveRateLimit,
		DatabaseURL:                            dbURL,
		JWTSecret:                              jwtSecret,
		Port:                                   port,
		LogLevel:                               strings.TrimSpace(firstNonEmpty(os.Getenv("LOG_LEVEL"), "info")),
		RunAutoMigrate:                         parseBoolEnvDefaultTrue(os.Getenv("RUN_AUTO_MIGRATE")),
		CORSOrigins:                            corsOrigins,
		EnabledModules:                         enabledModules,
		PublicWidgetURL:                        publicWidgetURL,
		PublicSDKURL:                           publicSDKURL,
		EmailVerificationRequired:              emailVerificationRequired,
		DemoViewerEmail:                        strings.ToLower(strings.TrimSpace(os.Getenv("DEMO_VIEWER_EMAIL"))),
		DemoRequireEmail:                       parseBoolEnv(os.Getenv("DEMO_REQUIRE_EMAIL")),
		DemoLeadWebhookURL:                     strings.TrimSpace(os.Getenv("DEMO_LEAD_WEBHOOK_URL")),
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
		AWSPresignEndpointURL:                  strings.TrimSpace(os.Getenv("AWS_S3_PRESIGN_ENDPOINT_URL")),
		AWSBucket:                              os.Getenv("AWS_S3_BUCKET_NAME"),
		AWSRegion:                              os.Getenv("AWS_REGION"),
		AWSEndpointURL:                         os.Getenv("AWS_S3_ENDPOINT_URL"),
		AWSPublicBaseURL:                       strings.TrimSpace(os.Getenv("AWS_S3_PUBLIC_BASE_URL")),
		AWSPrivateBucket:                       strings.EqualFold(os.Getenv("AWS_S3_PRIVATE_BUCKET"), "true"),
		AnthropicAPIKey:                        os.Getenv("ANTHROPIC_API_KEY"),
		AnthropicBaseURL:                       strings.TrimSpace(os.Getenv("ANTHROPIC_BASE_URL")),
		OpenAIAPIKey:                           strings.TrimSpace(os.Getenv("OPENAI_API_KEY")),
		FalAPIKey:                              strings.TrimSpace(os.Getenv("FAL_KEY")),
		OpenAIBaseURL:                          strings.TrimSpace(os.Getenv("OPENAI_BASE_URL")),
		OpenAIEmbeddingModel:                   strings.TrimSpace(os.Getenv("OPENAI_EMBEDDING_MODEL")),
		JevRoutingThreshold:                    parseJevProbability(os.Getenv("JEV_ROUTING_THRESHOLD"), 0.9),
		JevTagThreshold:                        parseJevProbability(os.Getenv("JEV_TAG_THRESHOLD"), 0.95),
		JevAPIKey:                              os.Getenv("JEV_API_KEY"),
		JevRoutingMode:                         strings.TrimSpace(os.Getenv("JEV_ROUTING_MODE")),
		JevTagsMode:                            strings.TrimSpace(os.Getenv("JEV_TAGS_MODE")),
		JevTimeoutMS:                           parsePositiveIntEnv(os.Getenv("JEV_TIMEOUT_MS"), 1000),
		JevWorkspaceIDs:                        os.Getenv("JEV_WORKSPACE_IDS"),
		JevDailyLimit:                          parsePositiveIntEnv(os.Getenv("JEV_DAILY_LIMIT"), 1000),
		SupportDecisionMode:                    strings.TrimSpace(os.Getenv("SUPPORT_DECISION_MODE")),
		SupportDecisionURL:                     strings.TrimSpace(os.Getenv("SUPPORT_DECISION_URL")),
		SupportDecisionToken:                   os.Getenv("SUPPORT_DECISION_TOKEN"),
		SupportDecisionTimeoutMS:               parsePositiveIntEnv(os.Getenv("SUPPORT_DECISION_TIMEOUT_MS"), 200),
		SupportDecisionWorkspaceIDs:            os.Getenv("SUPPORT_DECISION_WORKSPACE_IDS"),
		SupportDecisionPolicies:                os.Getenv("SUPPORT_DECISION_POLICIES"),
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
		SMTPHost:                               strings.TrimSpace(os.Getenv("SMTP_HOST")),
		SMTPPort:                               smtpPort,
		SMTPUsername:                           os.Getenv("SMTP_USERNAME"),
		SMTPPassword:                           os.Getenv("SMTP_PASSWORD"),
		SMTPFrom:                               strings.TrimSpace(os.Getenv("SMTP_FROM")),
		SMTPTLSMode:                            strings.TrimSpace(os.Getenv("SMTP_TLS_MODE")),
		PostmarkAccountToken:                   strings.TrimSpace(os.Getenv("POSTMARK_ACCOUNT_TOKEN")),
		PostmarkAppServerToken:                 strings.TrimSpace(firstNonEmpty(os.Getenv("POSTMARK_APP_SERVER_TOKEN"), os.Getenv("POSTMARK_SERVER_TOKEN"))),
		PostmarkAppFromEmail:                   strings.TrimSpace(firstNonEmpty(os.Getenv("POSTMARK_APP_FROM_EMAIL"), os.Getenv("POSTMARK_FROM_EMAIL"))),
		PostmarkReplyServerToken:               strings.TrimSpace(firstNonEmpty(os.Getenv("POSTMARK_REPLY_SERVER_TOKEN"), os.Getenv("POSTMARK_SERVER_TOKEN"))),
		PostmarkReplyFromEmail:                 strings.TrimSpace(firstNonEmpty(os.Getenv("POSTMARK_REPLY_FROM_EMAIL"), os.Getenv("POSTMARK_FROM_EMAIL"))),
		PostmarkReplyInboundWebhookSecret:      strings.TrimSpace(firstNonEmpty(os.Getenv("POSTMARK_REPLY_INBOUND_WEBHOOK_SECRET"), os.Getenv("POSTMARK_INBOUND_WEBHOOK_SECRET"))),
		SupportEmailReplyDomain:                strings.TrimSpace(firstNonEmpty(os.Getenv("SUPPORT_EMAIL_REPLY_DOMAIN"), deployment.DefaultReplyDomain)),
		PostmarkRouteServerToken:               strings.TrimSpace(os.Getenv("POSTMARK_ROUTE_SERVER_TOKEN")),
		PostmarkRouteInboundWebhookSecret:      strings.TrimSpace(firstNonEmpty(os.Getenv("POSTMARK_ROUTE_INBOUND_WEBHOOK_SECRET"), os.Getenv("POSTMARK_INBOUND_WEBHOOK_SECRET"))),
		SupportEmailRouteDomain:                strings.TrimSpace(firstNonEmpty(os.Getenv("SUPPORT_EMAIL_ROUTE_DOMAIN"), os.Getenv("SUPPORT_EMAIL_REPLY_DOMAIN"), deployment.DefaultRouteDomain)),
		AppBaseURL:                             appBaseURL,
		MobileAppBaseURL:                       strings.TrimRight(strings.TrimSpace(os.Getenv("MOBILE_APP_BASE_URL")), "/"),
		CLIEnabled:                             parseBoolEnv(os.Getenv("CLI_ENABLED")),
		CLIModelGatewayEnabled:                 parseBoolEnv(os.Getenv("CLI_MODEL_GATEWAY_ENABLED")),
		CLIPublicBaseURL:                       strings.TrimRight(strings.TrimSpace(os.Getenv("CLI_PUBLIC_BASE_URL")), "/"),
		MCPServerEnabled:                       parseBoolEnvDefaultTrue(os.Getenv("MCP_SERVER_ENABLED")),
		MCPOAuthEnabled:                        parseBoolEnvDefaultTrue(os.Getenv("MCP_OAUTH_ENABLED")),
		MCPServiceTokensEnabled:                parseBoolEnvDefaultTrue(os.Getenv("MCP_SERVICE_TOKENS_ENABLED")),
		MCPPMWriteEnabled:                      parseBoolEnvDefaultTrue(os.Getenv("MCP_PM_WRITE_ENABLED")),
		MCPDocsWriteEnabled:                    parseBoolEnvDefaultTrue(os.Getenv("MCP_DOCS_WRITE_ENABLED")),
		MCPAgentRunEnabled:                     parseBoolEnvDefaultTrue(os.Getenv("MCP_AGENT_RUN_ENABLED")),
		MCPCRMEnabled:                          parseBoolEnvDefaultTrue(os.Getenv("MCP_CRM_ENABLED")),
		MCPSupportEnabled:                      parseBoolEnvDefaultTrue(os.Getenv("MCP_SUPPORT_ENABLED")),
		MCPPublicBaseURL:                       strings.TrimRight(strings.TrimSpace(firstNonEmpty(os.Getenv("MCP_PUBLIC_BASE_URL"), appBaseURL)), "/"),
		AIConnectionEncryptionKey:              strings.TrimSpace(os.Getenv("AI_CONNECTION_ENCRYPTION_KEY")),
		ChatGPTConnectionsEnabled:              parseBoolEnv(os.Getenv("CHATGPT_CONNECTIONS_ENABLED")),
		ChatGPTClientID:                        strings.TrimSpace(os.Getenv("CHATGPT_OAUTH_CLIENT_ID")),
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
		CRMLLMProvider:                         crmLLMProvider,
		CRMLLMModel:                            crmLLMModel,
		CRMLLMOpenRouterProvider:               crmLLMOpenRouterProvider,
		CRMLLMFallbackProvider:                 crmLLMFallbackProvider,
		CRMLLMFallbackModel:                    crmLLMFallbackModel,
		CRMLLMFallbackOpenRouterProvider:       crmLLMFallbackOpenRouterProvider,
		CRMMeetingFallbackProvider:             crmMeetingLLMFallbackProvider,
		CRMMeetingFallbackModel:                crmMeetingLLMFallbackModel,
		CRMMeetingFallbackOpenRouterProvider:   crmMeetingLLMFallbackOpenRouterProvider,
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
		ClickHouseDSN:                          strings.TrimSpace(os.Getenv("CLICKHOUSE_DSN")),
		FCMServiceAccountJSON:                  strings.TrimSpace(os.Getenv("FCM_SERVICE_ACCOUNT_JSON")),
		AgentPreviewDebug:                      parseBoolEnv(os.Getenv("AGENT_PREVIEW_DEBUG")),
		DocsOrderingUseSortKey:                 parseBoolEnv(os.Getenv("DOCS_ORDERING_USE_SORT_KEY")),
		TLSAskExtraAllowedDomains:              parseCSV(os.Getenv("TLS_ASK_EXTRA_ALLOWED_DOMAINS")),
	}, nil
}

func validateOptionalLLMRouteEnv(providerName, provider, modelName, model string) error {
	providerSet := strings.TrimSpace(provider) != ""
	modelSet := strings.TrimSpace(model) != ""
	if providerSet == modelSet {
		return nil
	}
	return fmt.Errorf("%s and %s must be set together", providerName, modelName)
}

func validateOpenRouterProviderEnv(name, openRouterProvider, routeProvider, defaultRouteProvider string) error {
	if strings.TrimSpace(openRouterProvider) == "" {
		return nil
	}
	effectiveProvider := strings.ToLower(strings.TrimSpace(routeProvider))
	if effectiveProvider == "" {
		effectiveProvider = strings.ToLower(strings.TrimSpace(defaultRouteProvider))
	}
	if effectiveProvider != "openrouter" && effectiveProvider != "openrouter-responses" {
		return fmt.Errorf("%s requires the corresponding LLM provider to be openrouter", name)
	}
	return nil
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

func rateLimitEnv(name string, fallback int) (int, error) {
	raw := strings.TrimSpace(os.Getenv(name))
	if raw == "" {
		return fallback, nil
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value < 0 {
		return 0, fmt.Errorf("%s must be a non-negative integer (0 disables the limit)", name)
	}
	return value, nil
}

// parseJevProbability rejects invalid configured thresholds at client construction.
func parseJevProbability(raw string, fallback float64) float64 {
	if strings.TrimSpace(raw) == "" {
		return fallback
	}
	value, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return math.NaN()
	}
	return value
}
