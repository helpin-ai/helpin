package router

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"

	"github.com/helpin-ai/helpin/server/internal/auth"
	"github.com/helpin-ai/helpin/server/internal/authorization"
	"github.com/helpin-ai/helpin/server/internal/handler"
	"github.com/helpin-ai/helpin/server/internal/middleware"
	"github.com/helpin-ai/helpin/server/internal/model"
)

// Handlers aggregates all HTTP handlers.
type Handlers struct {
	// WidgetRateLimit guards the unauthenticated /widget write endpoints
	// (nil disables limiting, e.g. when Redis is not configured).
	WidgetRateLimit func(http.Handler) http.Handler

	// HelpcenterAnswerRateLimit guards the public AI answer endpoints
	// (nil disables limiting, e.g. when Redis is not configured).
	HelpcenterAnswerRateLimit func(http.Handler) http.Handler

	Health              *handler.HealthHandler
	Auth                *handler.AuthHandler
	Passkey             *handler.PasskeyHandler
	Organization        *handler.OrganizationHandler
	Workspace           *handler.WorkspaceHandler
	Setup               *handler.SetupHandler
	Billing             *handler.BillingHandler
	AIUsage             *handler.AIUsageHandler
	Settings            *handler.SettingsHandler
	Automation          *handler.AutomationHandler
	Invite              *handler.InviteHandler
	PMWorkflow          *handler.PMWorkflowHandler
	PMImport            *handler.PMImportHandler
	PMLabel             *handler.PMLabelHandler
	PMEpic              *handler.PMEpicHandler
	PMSprint            *handler.PMSprintHandler
	PMTask              *handler.PMTaskHandler
	PMTaskInsights      *handler.PMTaskInsightsHandler
	PMComment           *handler.PMCommentHandler
	PMAttachment        *handler.PMAttachmentHandler
	PMObjective         *handler.PMObjectiveHandler
	PMChecklistItem     *handler.PMChecklistItemHandler
	PMExternalLink      *handler.PMExternalLinkHandler
	PMView              *handler.PMViewHandler
	PMAutomation        *handler.PMAutomationHandler
	PMTaskTemplate      *handler.PMTaskTemplateHandler
	PMRecurringTemplate *handler.PMRecurringTemplateHandler
	Search              *handler.SearchHandler
	CommandBar          *handler.CommandBarHandler
	DockChat            *handler.DockChatHandler
	PublicShare         *handler.PublicShareHandler
	Agent               *handler.AgentHandler
	AgentRuntimeHost    *handler.AgentRuntimeHostHandler
	MCP                 *handler.MCPHandler
	ExternalMCP         *handler.ExternalMCPHandler
	SupportInbox        *handler.SupportInboxHandler
	SupportInboxView    *handler.SupportInboxViewHandler
	SupportTag          *handler.SupportTagHandler
	SupportInboxWidget  *handler.SupportInboxWidgetHandler
	Git                 *handler.GitHandler
	Docs                *handler.DocsHandler
	TLSAsk              *handler.TLSAskHandler
	Notification        *handler.NotificationHandler
	UserNotifSettings   *handler.UserNotificationSettingsHandler
	PushDevice          *handler.PushDeviceHandler
	CRMContact          *handler.CRMContactHandler
	CRMCompany          *handler.CRMCompanyHandler
	CRMDeal             *handler.CRMDealHandler
	CRMAssociation      *handler.CRMAssociationHandler
	Associations        *handler.AssociationsHandler
	CRMActivity         *handler.CRMActivityHandler
	CRMMeeting          *handler.CRMMeetingHandler
	CRMImport           *handler.CRMImportHandler
	CRMEmail            *handler.CRMEmailHandler
	CRMCalendar         *handler.CRMCalendarHandler
	CRMEnrichment       *handler.CRMEnrichmentHandler
	CRMSignal           *handler.CRMSignalHandler
	CRMSummary          *handler.CRMSummaryHandler
	CRMSuggestion       *handler.CRMSuggestionHandler
	CRMWritingProfile   *handler.CRMWritingProfileHandler
	CRMSearch           *handler.CRMSearchHandler
	CRMDealAutomation   *handler.CRMDealAutomationHandler
	AutomationRule      *handler.AutomationRuleHandler
	PMRoadmap           *handler.PMRoadmapHandler
	SDKAssets           *handler.SDKAssetsHandler
	SupportAI           *handler.SupportAIHandler
	SupportAttachment   *handler.SupportAttachmentHandler
	SupportCoverage     *handler.SupportCoverageHandler
	PostmarkInbound     *handler.PostmarkInboundHandler
	EmailImageProxy     *handler.EmailImageProxyHandler
	AdminWebhookEvent   *handler.AdminWebhookEventHandler
	AdminEmailQueue     *handler.AdminEmailQueueHandler
}

// New creates and configures the Chi router with all routes.
func New(h Handlers, jwtManager *auth.JWTManager, authz *authorization.AuthzService, slugResolver authorization.SlugResolver, corsOrigins []string) *chi.Mux {
	r := chi.NewRouter()

	helpcenterAnswerLimiter := h.HelpcenterAnswerRateLimit
	if helpcenterAnswerLimiter == nil {
		helpcenterAnswerLimiter = func(next http.Handler) http.Handler { return next }
	}

	// Global middleware (applied to all routes)
	r.Use(chimiddleware.RequestID)
	r.Use(chimiddleware.RealIP)
	r.Use(middleware.RequestLogger)
	r.Use(chimiddleware.Recoverer)
	r.Use(middleware.SentryHTTP)
	r.Use(middleware.SentryRequestContext)

	// API-scoped CORS — restricted to configured origins (dashboard, frontend).
	// Widget/SDK routes have their own open CORS (AllowedOrigins: *).
	// This must NOT be global, otherwise it short-circuits widget preflight requests
	// from customer domains that aren't in corsOrigins.
	apiCORS := cors.Handler(cors.Options{
		AllowedOrigins:   corsOrigins,
		AllowedMethods:   []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-Workspace-ID", "Idempotency-Key"},
		ExposedHeaders:   []string{"Link", "Deprecation", "Sunset"},
		AllowCredentials: true,
		MaxAge:           300,
	})

	// Permission middleware helpers for readability.
	requirePerm := func(perm authorization.Permission) func(http.Handler) http.Handler {
		return authorization.RequirePermission(authz, perm)
	}
	requireCommandBarRead := func() func(http.Handler) http.Handler {
		return authorization.RequireAnyPermission(authz, authorization.PermPMRead, authorization.PermDocsRead, authorization.PermCRMRead)
	}
	requireCommandBarEdit := func() func(http.Handler) http.Handler {
		return authorization.RequireAnyPermission(authz, authorization.PermPMEdit, authorization.PermDocsEdit, authorization.PermCRMEdit)
	}
	requireAutomationRead := func() func(http.Handler) http.Handler {
		return authorization.RequireAnyPermission(authz, authorization.PermPMRead, authorization.PermDocsRead, authorization.PermCRMRead, authorization.PermSupportRead)
	}
	requireAutomationEdit := func() func(http.Handler) http.Handler {
		return authorization.RequireAnyPermission(authz, authorization.PermPMEdit, authorization.PermDocsEdit, authorization.PermCRMEdit, authorization.PermSupportEdit)
	}
	requireModule := func(module model.ModuleID) func(http.Handler) http.Handler {
		return authorization.RequireModuleAccess(authz, module)
	}
	wsAccess := authorization.RequireWorkspaceAccess(authz)
	wsActive := wsAccess
	if h.Billing != nil {
		wsActive = func(next http.Handler) http.Handler {
			return wsAccess(h.Billing.RequireUnlockedWorkspace(next))
		}
	}

	// Root endpoint — responds on bare domain requests.
	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"name":"Helpin API","status":"running"}`))
	})
	r.With(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Content-Type", "Authorization", "X-Session-Token"},
		AllowCredentials: false,
		MaxAge:           3600,
	})).Get("/view_headers", h.Health.ViewHeaders)
	r.Get("/health", h.Health.Check)
	if h.MCP != nil {
		r.Get("/.well-known/oauth-authorization-server", h.MCP.AuthorizationServerMetadata)
		r.Get("/.well-known/oauth-protected-resource", h.MCP.ProtectedResourceMetadata)
		r.Handle("/mcp", http.HandlerFunc(h.MCP.Protocol))
	}

	// ---- Public widget routes for client.helpin.ai (no JWT, open CORS) ----
	// Mounted at /widget (outside /api) so the ingress path /widget/* works directly.
	r.Route("/widget", func(r chi.Router) {
		r.Use(cors.Handler(cors.Options{
			AllowedOrigins:   []string{"*"},
			AllowedMethods:   []string{"GET", "POST", "PATCH", "OPTIONS"},
			AllowedHeaders:   []string{"Content-Type", "X-Session-Token"},
			AllowCredentials: false,
			MaxAge:           3600,
		}))
		if h.WidgetRateLimit != nil {
			r.Use(h.WidgetRateLimit)
		}
		r.Get("/config", h.SupportInboxWidget.GetConfig)
		r.Post("/session", h.SupportInboxWidget.CreateSession)
		r.Post("/session/revoke", h.SupportInboxWidget.RevokeSession)
		r.Post("/messages", h.SupportInboxWidget.SendMessage)
		r.Post("/conversations/{conversationId}/transcript", h.SupportInboxWidget.SendTranscript)
		r.Post("/support/conversations/{conversationId}/transcript", h.SupportInboxWidget.SendTranscript)
		r.Post("/typing", h.SupportInboxWidget.TypingIndicator) // Deprecated: use WebSocket typing:start/typing:stop instead. Kept as HTTP fallback.
		r.Get("/messages", h.SupportInboxWidget.GetMessages)
		r.Get("/settings/{id}", h.SupportInboxWidget.GetConfigByID)
		if h.SupportAttachment != nil {
			r.Post("/support/attachments", h.SupportAttachment.WidgetCreate)
			r.Patch("/support/attachments/{attachmentId}/confirm", h.SupportAttachment.WidgetConfirmUpload)
		}
		// Help center routes (used by widget-core helpApi.ts)
		r.Get("/support/help/spaces/{spaceSlug}/collections", h.SupportInboxWidget.GetHelpCollections)
		r.Get("/support/help/collections/{collectionSlug}/articles", h.SupportInboxWidget.GetHelpArticles)
		r.Get("/support/help/search", h.SupportInboxWidget.SearchHelpArticles)
		r.Get("/support/help/articles/{articleKey}", h.SupportInboxWidget.GetHelpArticle)
		r.Post("/identify", h.SupportInboxWidget.Identify) // SDK identify/lead path
		if h.SupportAI != nil {
			r.Post("/support/{conversationId}/escalate", h.SupportAI.EscalateToHuman)
		}
	})

	// ---- SDK asset serving (no JWT, open CORS, cache headers) ----
	if h.SDKAssets != nil {
		r.Route("/sdk", func(r chi.Router) {
			r.Get("/*", h.SDKAssets.ServeSDK)
		})
	}

	// ---- Public Help Center API (no JWT, open CORS for custom domains) ----
	r.Route("/api/hc", func(r chi.Router) {
		r.Use(cors.Handler(cors.Options{
			AllowedOrigins:   []string{"*"},
			AllowedMethods:   []string{"GET", "POST", "OPTIONS"},
			AllowedHeaders:   []string{"Accept", "Content-Type"},
			AllowCredentials: false,
			MaxAge:           3600,
		}))

		// Caddy on-demand TLS "ask" check (200 = issue certificate, 404 = deny)
		r.Get("/verify-domain", h.TLSAsk.Verify)

		// Public Help Center routes
		r.Route("/{subdomain}", func(r chi.Router) {
			r.Get("/config", h.Docs.PublicGetConfig)
			r.Get("/bootstrap", h.Docs.PublicGetBootstrap)
			r.Get("/spaces", h.Docs.PublicGetSpaces)
			r.Get("/spaces/{spaceSlug}/navigation", h.Docs.PublicGetSpaceNavigation)
			r.Get("/spaces/{spaceSlug}/api-references", h.Docs.PublicListAPIReferences)
			r.Get("/spaces/{spaceSlug}/api-references/{referenceSlug}", h.Docs.PublicGetAPIReference)
			r.Get("/spaces/{spaceSlug}/articles/{articleSlug}", h.Docs.PublicGetSpaceArticle)
			r.Post("/spaces/{spaceSlug}/articles/{articleSlug}/feedback", h.Docs.PublicSubmitFeedback)
			r.Get("/search", h.Docs.PublicSearchArticles)
			r.With(helpcenterAnswerLimiter).Post("/answer", h.Docs.PublicAnswerQuestion)
			r.With(helpcenterAnswerLimiter).Post("/answer/{answerID}/feedback", h.Docs.PublicAnswerFeedback)

			// Canonical collection + article routes
			r.Get("/c/{collectionSlug}", h.Docs.PublicGetCollectionPage)
			r.Get("/articles/{articleKey}", h.Docs.PublicGetCanonicalArticle)

			// Document preview (token-authenticated)
			r.Get("/preview/{docId}", h.Docs.PublicPreviewArticle)

			// Legacy/redirect resolver
			r.Get("/resolve/*", h.Docs.PublicResolvePath)
		})
	})

	r.Route("/api", func(r chi.Router) {
		r.Use(apiCORS)

		// ---- Public routes ----
		r.Post("/auth/signup", h.Auth.Signup)
		r.Post("/auth/verify-email", h.Auth.VerifyEmail)
		r.Get("/auth/google/start", h.Auth.GoogleStart)
		r.Get("/auth/google/callback", h.Auth.GoogleCallback)
		r.Post("/auth/google/mobile-exchange", h.Auth.GoogleMobileExchange)
		r.Post("/auth/signin", h.Auth.Signin)
		r.Post("/auth/passkey/authentication-options", h.Passkey.AuthenticationOptions)
		r.Post("/auth/passkey/authenticate", h.Passkey.Authenticate)
		r.Post("/auth/2fa/verify-signin", h.Auth.Verify2FASignin)
		r.Post("/auth/forgot-password", h.Auth.ForgotPassword)
		r.Post("/auth/reset-password", h.Auth.ResetPassword)
		r.Post("/auth/refresh", h.Auth.RefreshToken)
		r.Post("/auth/signout", h.Auth.Signout)
		if h.MCP != nil {
			r.Post("/mcp/oauth/register", h.MCP.RegisterClient)
			r.Get("/mcp/oauth/authorize", h.MCP.AuthorizeRedirect)
			r.Post("/mcp/oauth/token", h.MCP.Token)
			r.Post("/mcp/oauth/revoke", h.MCP.RevokeToken)
		}
		r.Get("/health", h.Health.Check)
		if h.AIUsage != nil {
			r.Get("/ai-pricing", h.AIUsage.Pricing)
		}
		r.Get("/system/ensure-cors", h.Health.EnsureStorageCORS)
		r.Get("/invitations/info", h.Invite.GetInfo)
		r.Post("/invitations/accept-with-signup", h.Invite.AcceptWithSignup)

		// ---- Public git webhook (no JWT) ----
		r.Get("/git/github/callback", h.Git.GitHubCallback)
		r.Post("/git/webhook", h.Git.Webhook)
		if h.PostmarkInbound != nil {
			r.Post("/webhooks/postmark/inbound", h.PostmarkInbound.PostmarkInbound)
			r.Post("/webhooks/postmark/open", h.PostmarkInbound.PostmarkOpen)
			r.Post("/webhooks/postmark/delivery", h.PostmarkInbound.PostmarkDelivery)
			r.Post("/webhooks/postmark/bounce", h.PostmarkInbound.PostmarkBounce)
			r.Post("/webhooks/postmark/spam-complaint", h.PostmarkInbound.PostmarkSpamComplaint)
		}
		if h.Billing != nil {
			r.Post("/webhooks/stripe", h.Billing.StripeWebhook)
		}
		if h.CRMMeeting != nil {
			r.Post("/webhooks/meeting-capture/{provider}", h.CRMMeeting.Webhook)
		}

		// ---- Public Gmail OAuth callback (Google redirects here without JWT) ----
		r.Get("/crm/email/oauth/callback", h.CRMEmail.OAuthCallbackRedirect)

		// Public attachment content. Inline editor images cannot send bearer auth headers,
		// so this keeps the durable attachment ID as the app-controlled image URL.
		r.Get("/pm/attachments/{id}/content", h.PMAttachment.Content)

		// ---- Caddy on-demand TLS "ask" check ----
		r.Get("/hc/verify-domain", h.TLSAsk.Verify)

		// ---- Public Help Center routes (no JWT) ----
		r.Route("/hc/{subdomain}", func(r chi.Router) {
			r.Get("/config", h.Docs.PublicGetConfig)
			r.Get("/bootstrap", h.Docs.PublicGetBootstrap)
			r.Get("/{locale}/spaces", h.Docs.PublicGetSpaces)
			r.Get("/{locale}/spaces/{spaceSlug}/navigation", h.Docs.PublicGetSpaceNavigation)
			r.Get("/{locale}/spaces/{spaceSlug}/api-references", h.Docs.PublicListAPIReferences)
			r.Get("/{locale}/spaces/{spaceSlug}/api-references/{referenceSlug}", h.Docs.PublicGetAPIReference)
			r.Get("/{locale}/spaces/{spaceSlug}/collections/{collectionSlug}", h.Docs.PublicGetCollectionPage)
			r.Get("/{locale}/spaces/{spaceSlug}/collections/{collectionSlug}/articles/{articleSlug}", h.Docs.PublicGetSpaceArticle)
			r.Post("/{locale}/spaces/{spaceSlug}/collections/{collectionSlug}/articles/{articleSlug}/feedback", h.Docs.PublicSubmitFeedback)
			r.Get("/{locale}/c/{collectionSlug}", h.Docs.PublicGetCollectionPage)
			r.Get("/{locale}/articles/{articleKey}", h.Docs.PublicGetCanonicalArticle)
			r.Post("/{locale}/articles/{articleKey}/feedback", h.Docs.PublicSubmitFeedback)
			r.Get("/{locale}/collections/{collectionSlug}", h.Docs.PublicGetCollectionPage)
			r.Get("/{locale}/collections/{collectionSlug}/articles/{articleSlug}", h.Docs.PublicGetSpaceArticle)
			r.Post("/{locale}/collections/{collectionSlug}/articles/{articleSlug}/feedback", h.Docs.PublicSubmitFeedback)
			r.Get("/{locale}/search", h.Docs.PublicSearchArticles)
			r.With(helpcenterAnswerLimiter).Post("/{locale}/answer", h.Docs.PublicAnswerQuestion)
			r.With(helpcenterAnswerLimiter).Post("/{locale}/answer/{answerID}/feedback", h.Docs.PublicAnswerFeedback)

			r.Get("/spaces", h.Docs.PublicGetSpaces)
			r.Get("/spaces/{spaceSlug}/navigation", h.Docs.PublicGetSpaceNavigation)
			r.Get("/spaces/{spaceSlug}/api-references", h.Docs.PublicListAPIReferences)
			r.Get("/spaces/{spaceSlug}/api-references/{referenceSlug}", h.Docs.PublicGetAPIReference)
			r.Get("/spaces/{spaceSlug}/articles/{articleSlug}", h.Docs.PublicGetSpaceArticle)
			r.Post("/spaces/{spaceSlug}/articles/{articleSlug}/feedback", h.Docs.PublicSubmitFeedback)
			r.Get("/search", h.Docs.PublicSearchArticles)
			r.With(helpcenterAnswerLimiter).Post("/answer", h.Docs.PublicAnswerQuestion)
			r.With(helpcenterAnswerLimiter).Post("/answer/{answerID}/feedback", h.Docs.PublicAnswerFeedback)

			// Canonical collection + article routes
			r.Get("/c/{collectionSlug}", h.Docs.PublicGetCollectionPage)
			r.Get("/articles/{articleKey}", h.Docs.PublicGetCanonicalArticle)
			r.Post("/articles/{articleKey}/feedback", h.Docs.PublicSubmitFeedback)

			// Document preview (token-authenticated)
			r.Get("/preview/{docId}", h.Docs.PublicPreviewArticle)

			// Legacy/redirect resolver
			r.Get("/resolve/*", h.Docs.PublicResolvePath)
		})
		// ---- Public shared document route (no JWT) ----
		r.Get("/docs/shared/{shareToken}", h.Docs.PublicGetSharedDoc)
		if h.PublicShare != nil {
			r.Get("/public/shares/{token}", h.PublicShare.GetPublic)
		}

		// ---- Public widget routes (no JWT, open CORS) ----
		r.Route("/widget/support", func(r chi.Router) {
			r.Use(cors.Handler(cors.Options{
				AllowedOrigins:   []string{"*"},
				AllowedMethods:   []string{"GET", "POST", "PATCH", "OPTIONS"},
				AllowedHeaders:   []string{"Content-Type", "X-Session-Token"},
				AllowCredentials: false,
				MaxAge:           3600,
			}))
			if h.WidgetRateLimit != nil {
				r.Use(h.WidgetRateLimit)
			}
			r.Get("/config", h.SupportInboxWidget.GetConfig)
			r.Get("/help/spaces/{spaceSlug}/collections", h.SupportInboxWidget.GetHelpCollections)
			r.Get("/help/collections/{collectionSlug}/articles", h.SupportInboxWidget.GetHelpArticles)
			r.Get("/help/search", h.SupportInboxWidget.SearchHelpArticles)
			r.Get("/help/articles/{articleKey}", h.SupportInboxWidget.GetHelpArticle)
			r.Post("/session", h.SupportInboxWidget.CreateSession)
			r.Post("/session/revoke", h.SupportInboxWidget.RevokeSession)
			r.Post("/messages", h.SupportInboxWidget.SendMessage)
			r.Post("/conversations/{conversationId}/transcript", h.SupportInboxWidget.SendTranscript)
			r.Post("/typing", h.SupportInboxWidget.TypingIndicator) // Deprecated: use WebSocket typing:start/typing:stop instead. Kept as HTTP fallback.
			r.Get("/messages", h.SupportInboxWidget.GetMessages)
			if h.SupportAI != nil {
				r.Post("/{conversationId}/escalate", h.SupportAI.EscalateToHuman)
			}
			if h.SupportAttachment != nil {
				r.Post("/attachments", h.SupportAttachment.WidgetCreate)
				r.Patch("/attachments/{attachmentId}/confirm", h.SupportAttachment.WidgetConfirmUpload)
			}
		})

		// ---- Public widget config by installation ID (no JWT, open CORS) ----
		r.Route("/settings/website", func(r chi.Router) {
			r.Use(cors.Handler(cors.Options{
				AllowedOrigins:   []string{"*"},
				AllowedMethods:   []string{"GET", "OPTIONS"},
				AllowedHeaders:   []string{"Content-Type"},
				AllowCredentials: false,
				MaxAge:           3600,
			}))
			r.Get("/{id}", h.SupportInboxWidget.GetConfigByID)
		})

		// ---- Public widget routes for client.helpin.ai (no JWT, open CORS) ----
		// SDK-facing endpoints under /widget/ prefix.
		r.Route("/widget", func(r chi.Router) {
			r.Use(cors.Handler(cors.Options{
				AllowedOrigins:   []string{"*"},
				AllowedMethods:   []string{"GET", "POST", "OPTIONS"},
				AllowedHeaders:   []string{"Content-Type"},
				AllowCredentials: false,
				MaxAge:           3600,
			}))
			r.Get("/config", h.SupportInboxWidget.GetConfig)
			r.Post("/session", h.SupportInboxWidget.CreateSession)
			r.Post("/session/revoke", h.SupportInboxWidget.RevokeSession)
			r.Post("/messages", h.SupportInboxWidget.SendMessage)
			r.Post("/typing", h.SupportInboxWidget.TypingIndicator) // Deprecated: use WebSocket typing:start/typing:stop instead. Kept as HTTP fallback.
			r.Get("/messages", h.SupportInboxWidget.GetMessages)
			r.Get("/settings/{id}", h.SupportInboxWidget.GetConfigByID)
			r.Post("/identify", h.SupportInboxWidget.Identify) // Headless SDK identify/lead path
		})

		// ---- Internal service-to-service routes (bearer token auth) ----
		r.Route("/internal", func(r chi.Router) {
			r.Use(middleware.RequireInternalAPISecret)
			r.Get("/widget-tokens", h.SupportInboxWidget.GetWidgetTokens)
			if h.AgentRuntimeHost != nil {
				r.Route("/agent-runtime", func(r chi.Router) {
					r.Post("/events", h.AgentRuntimeHost.ApplyEvent)
					r.Post("/target-context", h.AgentRuntimeHost.ResolveTargetContext)
					r.Post("/workspace/repository-spec", h.AgentRuntimeHost.ResolveRepositorySpec)
					r.Get("/mcp/helpin/tools", h.AgentRuntimeHost.ListProviderTools)
					r.Post("/mcp/helpin/call", h.AgentRuntimeHost.CallProviderTool)
					r.Post("/artifacts", h.AgentRuntimeHost.UploadBrowserAsset)
					r.Post("/skills/by-id", h.AgentRuntimeHost.ResolveSkillByID)
					r.Post("/skills/active-by-key", h.AgentRuntimeHost.ResolveActiveSkillByKey)
					r.Get("/skill-packages/objects/*", h.AgentRuntimeHost.GetSkillPackageObject)
				})
			}
		})

		// ---- Platform admin routes (audited before auth so denied attempts are logged) ----
		r.Route("/admin", func(r chi.Router) {
			r.Use(middleware.AdminAuditLogger(jwtManager))
			r.Use(middleware.RequireAuth(jwtManager))
			r.Use(authorization.RequirePlatformAdmin)
			r.Get("/webhook-events", h.AdminWebhookEvent.List)
			r.Get("/webhook-events/{id}", h.AdminWebhookEvent.GetByID)
			r.Get("/email-queue", h.AdminEmailQueue.List)
			r.Get("/email-diagnostics", h.AdminEmailQueue.Diagnostics)
			r.Get("/email-diagnostics/conversations/{conversationID}", h.AdminEmailQueue.ConversationDiagnostics)
		})

		// ---- Protected routes ----
		r.Group(func(r chi.Router) {
			r.Use(middleware.RequireAuth(jwtManager))
			if h.AgentRuntimeHost != nil {
				r.With(middleware.RequireWorkspaceID, wsAccess).Get("/agent-artifacts/{id}/content-url", h.AgentRuntimeHost.BrowserArtifactContentURL)
			}
			if h.MCP != nil {
				r.Get("/mcp/oauth/request", h.MCP.AuthorizationRequest)
				r.Post("/mcp/oauth/authorize", h.MCP.Authorize)
				r.Route("/mcp", func(r chi.Router) {
					r.Use(middleware.RequireWorkspaceID)
					r.Use(wsActive)
					r.Get("/", h.MCP.Dashboard)
					r.With(requirePerm(authorization.PermSettingsManage)).Put("/policy", h.MCP.UpdatePolicy)
					r.With(requirePerm(authorization.PermSettingsManage)).Delete("/connections", h.MCP.RevokeWorkspaceAccess)
					r.Delete("/connections/{connectionID}", h.MCP.RevokeConnection)
					r.With(requirePerm(authorization.PermSettingsRead)).Get("/activity", h.MCP.Activity)
					r.With(requirePerm(authorization.PermSettingsManage)).Post("/service-principals", h.MCP.CreateServicePrincipal)
					r.With(requirePerm(authorization.PermSettingsManage)).Delete("/service-principals/{principalID}", h.MCP.RevokeServicePrincipal)
					r.With(requirePerm(authorization.PermSettingsManage)).Get("/service-principals/{principalID}/tokens", h.MCP.ListServiceTokens)
					r.With(requirePerm(authorization.PermSettingsManage)).Post("/service-principals/{principalID}/tokens", h.MCP.RotateServiceToken)
					r.With(requirePerm(authorization.PermSettingsManage)).Delete("/service-principals/{principalID}/tokens/{tokenID}", h.MCP.RevokeServiceToken)
				})
			}
			if h.ExternalMCP != nil {
				r.Get("/external-mcp/oauth/callback", h.ExternalMCP.OAuthCallback)
				r.Route("/external-mcp", func(r chi.Router) {
					r.Use(middleware.RequireWorkspaceID)
					r.Use(wsActive)
					r.With(requirePerm(authorization.PermSettingsRead)).Get("/providers", h.ExternalMCP.Providers)
					r.With(requirePerm(authorization.PermSettingsRead)).Get("/servers", h.ExternalMCP.ListServers)
					r.With(requirePerm(authorization.PermSettingsManage)).Post("/servers", h.ExternalMCP.CreateServer)
					r.With(requirePerm(authorization.PermSettingsManage)).Put("/servers/{serverID}", h.ExternalMCP.UpdateServer)
					r.With(requirePerm(authorization.PermSettingsManage)).Delete("/servers/{serverID}", h.ExternalMCP.DeleteServer)
					r.With(requirePerm(authorization.PermSettingsManage)).Post("/servers/{serverID}/oauth/start", h.ExternalMCP.StartOAuth)
					r.With(requirePerm(authorization.PermSettingsManage)).Post("/servers/{serverID}/tools/refresh", h.ExternalMCP.RefreshTools)
					r.With(requirePerm(authorization.PermSettingsManage)).Put("/servers/{serverID}/tools", h.ExternalMCP.UpdateTools)
				})
			}

			// Auth / profile
			r.Get("/auth/me", h.Auth.Me)
			r.Put("/auth/me", h.Auth.UpdateProfile)
			r.Post("/auth/resend-verification", h.Auth.ResendVerificationEmail)
			r.Post("/auth/me/avatar", h.Auth.UploadAvatar)
			r.Delete("/auth/me/avatar", h.Auth.DeleteAvatar)
			r.Put("/auth/change-password", h.Auth.ChangePassword)
			r.Post("/auth/passkey/registration-options", h.Passkey.RegistrationOptions)
			r.Post("/auth/passkey/register", h.Passkey.Register)
			r.Get("/auth/passkey/list", h.Passkey.List)
			r.Delete("/auth/passkey/{id}", h.Passkey.Delete)
			r.Get("/auth/2fa/status", h.Auth.Get2FAStatus)
			r.Post("/auth/2fa/setup", h.Auth.Setup2FA)
			r.Post("/auth/2fa/verify", h.Auth.Verify2FA)
			r.Post("/auth/2fa/step-up", h.Auth.StepUp2FA)
			r.Delete("/auth/2fa", h.Auth.Disable2FA)
			r.Post("/auth/2fa/regenerate-recovery-codes", h.Auth.RegenerateRecoveryCodes)

			// User notification settings (account-level, no workspace scope)
			r.Get("/user/notification-settings", h.UserNotifSettings.Get)
			r.Put("/user/notification-settings", h.UserNotifSettings.Update)

			// Push device registration (account-level, no workspace scope)
			r.Post("/user/push-devices", h.PushDevice.Register)
			r.Delete("/user/push-devices", h.PushDevice.Unregister)

			// Organizations
			r.Get("/organizations", h.Organization.List)
			r.Post("/organizations", h.Organization.Create)
			r.Get("/organizations/{id}", h.Organization.Get)
			r.Put("/organizations/{id}", h.Organization.Update)
			r.Delete("/organizations/{id}", h.Organization.Delete)
			r.Get("/organizations/{id}/git/integrations", h.Git.ListOrgIntegrations)
			r.Post("/organizations/{id}/git/integrations", h.Git.CreateOrgIntegration)
			r.Get("/organizations/{id}/git/github/install-url", h.Git.GetOrgGitHubInstallURL)
			r.Post("/organizations/{id}/git/gitlab/connect", h.Git.ConnectOrgGitLab)
			r.Get("/organizations/{id}/git/integrations/{integrationId}", h.Git.GetOrgIntegration)
			r.Put("/organizations/{id}/git/integrations/{integrationId}", h.Git.UpdateOrgIntegration)
			r.Post("/organizations/{id}/git/integrations/{integrationId}/sync", h.Git.SyncOrgRepositories)
			r.Delete("/organizations/{id}/git/integrations/{integrationId}", h.Git.DeleteOrgIntegration)
			r.Get("/organizations/{id}/members", h.Organization.ListMembers)
			r.Post("/organizations/{id}/members", h.Organization.AddMember)
			r.Put("/organizations/{id}/members/{userId}", h.Organization.UpdateMember)
			r.Delete("/organizations/{id}/members/{userId}", h.Organization.RemoveMember)
			r.Post("/organizations/{id}/transfer-ownership", h.Organization.TransferOwnership)

			// Organization billing is owner-only.
			if h.Billing != nil {
				r.With(h.Billing.RequireOrgBillingOwner).Get("/organizations/{id}/billing", h.Billing.GetOrganizationBilling)
				r.With(h.Billing.RequireOrgBillingOwner).Get("/organizations/{id}/billing/cards", h.Billing.ListCards)
				r.With(h.Billing.RequireOrgBillingOwner).Put("/organizations/{id}/billing/cards/{cardId}", h.Billing.UpdateCard)
				r.With(h.Billing.RequireOrgBillingOwner).Delete("/organizations/{id}/billing/cards/{cardId}", h.Billing.DeleteCard)
				r.With(h.Billing.RequireOrgBillingOwner).Get("/organizations/{id}/billing/invoices", h.Billing.ListInvoices)
			}

			// Workspaces — workspace-scoped routes with RBAC
			r.Get("/workspaces", h.Workspace.List)
			r.Post("/workspaces", h.Workspace.Create)
			r.Post("/workspaces/context/generate-description", h.Workspace.GenerateCompanyProductDescription)

			// Cross-workspace support unread summary for the workspace switcher badge.
			r.Get("/support/workspace-unread", h.SupportInbox.ListWorkspaceUnread)

			// Slug lookup — resolve slug to workspace ID, then check access
			r.With(authorization.ResolveWorkspaceSlug(slugResolver), wsAccess).Get("/workspaces/by-slug/{slug}", h.Workspace.GetBySlug)

			r.Route("/workspaces/{id}", func(r chi.Router) {
				r.Use(authorization.ExtractWorkspaceIDParam)
				r.Use(wsActive)

				r.Get("/my-role", h.Workspace.GetMyRole)
				r.Get("/my-membership", h.Workspace.GetMyMembership)
				r.Get("/me", h.Workspace.GetMe)
				r.Patch("/me/support-task-preferences", h.Workspace.UpdateSupportTaskPreferences)
				r.Get("/members", h.Workspace.ListMembers)
				r.Get("/members/presence", h.Workspace.ListMemberPresence)
				r.Get("/assignable-members", h.Workspace.ListAssignableMembers)
				r.Get("/key-history", h.Workspace.GetKeyHistory)
				if h.Setup != nil {
					r.Get("/setup", h.Setup.Get)
					r.With(requirePerm(authorization.PermWorkspaceUpdate)).Put("/setup/goals", h.Setup.UpdateGoals)
					r.Patch("/setup/me", h.Setup.UpdatePreference)
					r.Post("/setup/recommendations/{taskKey}/start", h.Setup.StartRecommendation)
				}
				r.With(requirePerm(authorization.PermWorkspaceMembersManage)).Put("/members/{memberId}", h.Workspace.UpdateMember)
				r.With(requirePerm(authorization.PermWorkspaceMembersManage)).Delete("/members/{memberId}", h.Workspace.RemoveMember)

				r.With(requirePerm(authorization.PermWorkspaceUpdate)).Put("/", h.Workspace.Update)
				r.With(requirePerm(authorization.PermWorkspaceUpdate)).Post("/logo", h.Workspace.UploadLogo)
				r.With(requirePerm(authorization.PermWorkspaceUpdate)).Delete("/logo", h.Workspace.DeleteLogo)
				r.With(authorization.RequireOwner(authz)).Delete("/", h.Workspace.Delete)
				if h.Billing != nil {
					r.With(requirePerm(authorization.PermSettingsRead)).Get("/billing", h.Billing.Get)
					r.With(requirePerm(authorization.PermSettingsManage), h.Billing.RequireWorkspaceBillingOwner).Post("/billing/checkout", h.Billing.Checkout)
					r.With(requirePerm(authorization.PermSettingsManage), h.Billing.RequireWorkspaceBillingOwner).Post("/billing/confirm-checkout", h.Billing.ConfirmCheckout)
					r.With(requirePerm(authorization.PermSettingsManage), h.Billing.RequireWorkspaceBillingOwner).Post("/billing/preview-plan-change", h.Billing.PreviewPlanChange)
					r.With(requirePerm(authorization.PermSettingsManage), h.Billing.RequireWorkspaceBillingOwner).Post("/billing/change-plan", h.Billing.ChangePlan)
					r.With(requirePerm(authorization.PermSettingsManage), h.Billing.RequireWorkspaceBillingOwner).Post("/billing/resume-subscription", h.Billing.ResumeSubscription)
					r.With(requirePerm(authorization.PermSettingsManage), h.Billing.RequireWorkspaceBillingOwner).Post("/billing/portal", h.Billing.Portal)
					r.With(requirePerm(authorization.PermSettingsManage), h.Billing.RequireWorkspaceBillingOwner).Put("/billing/extra-usage", h.Billing.SetOnDemand)
					r.With(requirePerm(authorization.PermSettingsManage), h.Billing.RequireWorkspaceBillingOwner).Post("/billing/test-scenario", h.Billing.ApplyTestScenario)
					// Usage is read-only and visible to any settings reader (matches the
					// billing summary). Payment-method changes remain billing-owner-only.
					r.With(requirePerm(authorization.PermSettingsRead)).Get("/billing/usage", h.Billing.GetUsage)
					r.With(h.Billing.RequireWorkspaceBillingOwner).Put("/billing/payment-method", h.Billing.LinkPaymentMethod)
				}

				// Import routes require pm.import
				r.With(requirePerm(authorization.PermPMImport)).Post("/import/shortcut/preview", h.PMImport.PreviewShortcut)
				r.With(requirePerm(authorization.PermPMImport)).Post("/import/shortcut/execute", h.PMImport.ExecuteShortcut)
				r.With(requirePerm(authorization.PermPMImport)).Post("/import/shortcut/api/preview", h.PMImport.PreviewShortcutAPI)
				r.With(requirePerm(authorization.PermPMImport)).Get("/import/shortcut/api/preview/{scanId}", h.PMImport.GetShortcutAPIPreview)
				r.With(requirePerm(authorization.PermPMImport)).Post("/import/shortcut/api/execute", h.PMImport.ExecuteShortcutAPI)
				r.With(requirePerm(authorization.PermPMImport)).Get("/import/shortcut/status", h.PMImport.ListShortcutStatuses)
				r.With(requirePerm(authorization.PermPMImport)).Get("/import/shortcut/status/{importId}", h.PMImport.ShortcutStatus)
				r.With(requirePerm(authorization.PermPMImport)).Get("/import/shortcut/status/{importId}/detail", h.PMImport.ShortcutStatusDetail)
				r.With(requirePerm(authorization.PermPMImport)).Post("/import/shortcut/status/{importId}/cancel", h.PMImport.CancelShortcutImport)
				r.With(requirePerm(authorization.PermPMImport)).Post("/import/shortcut/status/{importId}/retry", h.PMImport.RetryShortcutImport)
			})

			// Settings — all routes require workspace access
			r.Route("/settings", func(r chi.Router) {
				r.Use(middleware.RequireWorkspaceID)
				r.Use(wsActive)

				// Read
				r.With(requirePerm(authorization.PermSettingsRead)).Get("/", h.Settings.GetAll)
				r.With(requirePerm(authorization.PermSettingsManage)).Get("/ai-automations", h.Settings.GetAIAutomations)
				r.With(requirePerm(authorization.PermSettingsManage)).Get("/ai-automations/executions", h.Settings.GetAIAutomationExecutions)
				r.With(requirePerm(authorization.PermModuleAccessManage)).Get("/module-access", h.Settings.GetModuleAccess)

				// Settings management (admin+)
				r.With(requirePerm(authorization.PermSettingsManage)).Post("/initialize", h.Settings.Initialize)
				r.With(requirePerm(authorization.PermSettingsManage)).Put("/job-roles", h.Settings.UpdateJobRoleCriteria)
				r.With(requirePerm(authorization.PermSettingsManage)).Delete("/job-roles", h.Settings.DeleteJobRole)
				r.With(requirePerm(authorization.PermSettingsManage)).Put("/system", h.Settings.UpdateSystem)
				r.With(requirePerm(authorization.PermModuleAccessManage)).Post("/module-access", h.Settings.UpsertModuleAccessGrant)
				r.With(requirePerm(authorization.PermModuleAccessManage)).Delete("/module-access/{id}", h.Settings.DeleteModuleAccessGrant)

				// People management
				r.With(requirePerm(authorization.PermWorkspaceMembersManage)).Post("/people", h.Settings.CreatePerson)
				r.With(requirePerm(authorization.PermWorkspaceMembersManage)).Put("/people/{id}", h.Settings.UpdatePerson)
				r.With(requirePerm(authorization.PermWorkspaceMembersManage)).Delete("/people/{id}", h.Settings.DeletePerson)

				// Team management — admin+ OR team owner via RequireTeamPermission
				r.With(requirePerm(authorization.PermTeamManage)).Post("/teams", h.Settings.CreateTeam)
				r.With(requirePerm(authorization.PermTeamManage)).Post("/teams/ensure-default", h.Settings.EnsureDefaultTeam)
				r.With(authorization.RequireTeamPermission(authz)).Put("/teams/{id}", h.Settings.UpdateTeam)
				r.With(authorization.RequireTeamPermission(authz)).Delete("/teams/{id}", h.Settings.DeleteTeam)

				// Team member management — admin+ OR team owner via RequireTeamPermission
				r.With(authorization.RequireTeamPermission(authz)).Post("/teams/{id}/members", h.Settings.AddTeamMember)
				r.With(authorization.RequireTeamPermission(authz)).Put("/teams/{id}/members/{userId}", h.Settings.UpdateTeamMember)
				r.With(authorization.RequireTeamPermission(authz)).Delete("/teams/{id}/members/{userId}", h.Settings.DeleteTeamMember)
				r.With(authorization.RequireTeamPermission(authz)).Post("/teams/{id}/invitations", h.Settings.AddTeamInvitation)
				r.With(authorization.RequireTeamPermission(authz)).Delete("/teams/{id}/invitations/{invitationId}", h.Settings.DeleteTeamInvitation)

				// Team settings reads (any member)
				r.Get("/teams/{id}/estimates", h.Settings.GetTeamEstimateSettings)
				r.Get("/teams/{id}/field-visibility", h.Settings.GetTeamFieldVisibility)
				r.Get("/teams/{id}/repo-default", h.Settings.GetTeamRepoDefault)

				// Team settings writes — admin+ OR team owner
				r.With(authorization.RequireTeamPermission(authz)).Put("/teams/{id}/estimates", h.Settings.UpdateTeamEstimateSettings)
				r.With(authorization.RequireTeamPermission(authz)).Put("/teams/{id}/field-visibility", h.Settings.UpdateTeamFieldVisibility)
				r.With(authorization.RequireTeamPermission(authz)).Put("/teams/{id}/repo-default", h.Settings.UpdateTeamRepoDefault)
			})

			// Automation — platform-wide automation API facade
			r.Route("/automation", func(r chi.Router) {
				r.Use(middleware.RequireWorkspaceID)
				r.Use(wsActive)
				r.Use(requireModule(model.ModuleAutomation))

				r.With(requirePerm(authorization.PermSettingsManage)).Get("/overview", h.Automation.GetOverview)
				r.Route("/flows", func(r chi.Router) {
					r.With(requirePerm(authorization.PermPMRead)).Get("/", h.Automation.ListFlows)
					r.With(requirePerm(authorization.PermPMAdminAutomations)).Post("/", h.Automation.CreateFlow)
					r.Route("/{id}", func(r chi.Router) {
						r.With(requirePerm(authorization.PermPMRead)).Get("/", h.Automation.GetFlow)
						r.With(requirePerm(authorization.PermPMEdit)).Post("/run", h.Automation.RunFlowNow)
						r.With(requirePerm(authorization.PermPMAdminAutomations)).Put("/", h.Automation.UpdateFlow)
						r.With(requirePerm(authorization.PermPMAdminAutomations)).Delete("/", h.Automation.DeleteFlow)
					})
				})

				r.Route("/templates", func(r chi.Router) {
					r.With(requirePerm(authorization.PermPMRead)).Get("/", h.Automation.ListFlowTemplates)
					r.Route("/{key}", func(r chi.Router) {
						r.With(requirePerm(authorization.PermPMRead)).Get("/", h.Automation.GetFlowTemplate)
						r.With(requirePerm(authorization.PermPMAdminAutomations)).Post("/install", h.Automation.InstallFlowTemplate)
					})
				})
				r.Route("/template-instances", func(r chi.Router) {
					r.Route("/{instanceID}", func(r chi.Router) {
						r.With(requirePerm(authorization.PermPMAdminAutomations)).Post("/uninstall", h.Automation.UninstallFlowTemplate)
					})
				})

				r.With(requirePerm(authorization.PermSettingsManage)).Get("/activity", h.Automation.ListActivity)

				r.Route("/library", func(r chi.Router) {
					r.With(requirePerm(authorization.PermSettingsManage)).Get("/triggers", h.Automation.ListTriggerCatalog)
					r.With(requireAutomationRead()).Get("/tools", h.Automation.ListToolCatalog)
					r.Route("/skills", func(r chi.Router) {
						r.With(requireAutomationRead()).Get("/", h.Automation.ListSkillCatalog)
						r.With(requireAutomationEdit()).Post("/", h.Automation.CreateSkill)
						r.With(requireAutomationEdit()).Post("/import", h.Automation.ImportSkill)
						r.Route("/{id}", func(r chi.Router) {
							r.With(requireAutomationEdit()).Put("/", h.Automation.UpdateSkill)
							r.With(requireAutomationEdit()).Delete("/", h.Automation.DeleteSkill)
						})
					})
				})

				r.Route("/agents", func(r chi.Router) {
					r.With(requireAutomationRead()).Get("/", h.Automation.ListAgents)
					r.With(requireAutomationEdit()).Post("/", h.Automation.CreateAgent)
					r.With(requireAutomationEdit()).Post("/draft", h.Automation.DraftCustomAgent)
					r.Route("/{id}", func(r chi.Router) {
						r.With(requireAutomationRead()).Get("/", h.Automation.GetAgent)
						r.With(requireAutomationEdit()).Put("/", h.Automation.UpdateAgent)
						r.With(requireAutomationEdit()).Delete("/", h.Automation.DeleteAgent)
						r.With(requireAutomationRead()).Get("/usage", h.Automation.GetAgentUsage)
						r.With(requireAutomationRead()).Get("/analytics", h.Automation.GetAgentAnalytics)
						r.With(requireAutomationRead()).Get("/versions", h.Automation.ListAgentVersions)
						r.With(requireAutomationEdit()).Post("/versions", h.Automation.CreateAgentVersion)
						r.With(requireAutomationEdit()).Put("/versions/{versionID}", h.Automation.UpdateAgentVersion)
						r.With(requireAutomationEdit()).Post("/versions/{versionID}/activate", h.Automation.ActivateAgentVersion)
						r.With(requireAutomationEdit()).Delete("/versions/{versionID}", h.Automation.DeleteAgentVersion)
					})
				})
				r.With(requireAutomationRead()).Get("/agent-fleet", h.Automation.GetAgentFleet)

				r.Route("/runs", func(r chi.Router) {
					r.With(requireAutomationRead()).Get("/", h.Automation.ListRuns)
					r.With(requireAutomationRead()).Get("/attention-count", h.Automation.GetRunAttentionCount)
					r.With(requireAutomationEdit()).Post("/", h.Automation.StartRun)
					r.Route("/{id}", func(r chi.Router) {
						r.With(requireAutomationRead()).Get("/", h.Automation.GetRun)
						r.With(requireAutomationRead()).Get("/messages", h.Automation.ListRunMessages)
						r.With(requireAutomationEdit()).Post("/messages", h.Automation.SendRunMessage)
						r.With(requireAutomationRead()).Get("/artifacts", h.Automation.ListRunArtifacts)
						r.With(requireAutomationEdit()).Post("/resume", h.Automation.ResumeRun)
						r.With(requireAutomationEdit()).Post("/continue", h.Automation.ContinueRun)
						r.With(requireAutomationEdit()).Post("/approve", h.Automation.ApproveRun)
						r.With(requireAutomationEdit()).Post("/request-changes", h.Automation.RequestRunChanges)
						r.With(requireAutomationEdit()).Post("/cancel", h.Automation.CancelRun)
						r.With(requireAutomationEdit()).Post("/handoff", h.Automation.HandoffRun)
					})
				})
			})

			// Invitations
			r.Route("/invitations", func(r chi.Router) {
				r.Post("/accept", h.Invite.Accept)
				r.Get("/", h.Invite.List)

				// Sending and managing invitations requires workspace context
				r.With(middleware.RequireWorkspaceID, wsActive, requirePerm(authorization.PermWorkspaceInvitesManage)).Post("/", h.Invite.Send)
				r.With(middleware.RequireWorkspaceID, wsActive, requirePerm(authorization.PermWorkspaceInvitesManage)).Post("/{id}/resend", h.Invite.Resend)
				r.With(middleware.RequireWorkspaceID, wsActive, requirePerm(authorization.PermWorkspaceInvitesManage)).Delete("/{id}", h.Invite.Revoke)
			})

			// Git integrations
			r.Route("/git", func(r chi.Router) {
				r.Use(middleware.RequireWorkspaceID)
				r.Use(wsActive)
				r.With(requirePerm(authorization.PermIntegrationsConnect)).Get("/github/install-url", h.Git.GetGitHubInstallURL)
				r.With(requirePerm(authorization.PermSettingsRead)).Get("/integrations", h.Git.ListIntegrations)
				r.With(requirePerm(authorization.PermSettingsManage)).Post("/integrations", h.Git.CreateIntegration)
				r.With(requirePerm(authorization.PermSettingsRead)).Get("/integrations/{id}", h.Git.GetIntegration)
				r.With(requirePerm(authorization.PermIntegrationsUninstall)).Delete("/integrations/{id}", h.Git.DeleteIntegration)
				r.With(requirePerm(authorization.PermSettingsManage)).Post("/integrations/{id}/sync", h.Git.SyncRepositories)
				r.With(authorization.RequireAnyPermission(authz, authorization.PermSettingsRead, authorization.PermPMRead)).Get("/integrations/{id}/available-repos", h.Git.ListAvailableRepos)
				r.With(requirePerm(authorization.PermIntegrationsLinkRepo)).Post("/integrations/{id}/repositories", h.Git.WireRepositories)
				r.With(requirePerm(authorization.PermIntegrationsLinkRepo)).Delete("/integrations/{id}/repositories/{repoId}", h.Git.UnwireRepository)
				r.With(authorization.RequireAnyPermission(authz, authorization.PermSettingsRead, authorization.PermPMRead)).Get("/repositories", h.Git.ListRepositories)
				r.With(authorization.RequireAnyPermission(authz, authorization.PermSettingsRead, authorization.PermPMRead)).Get("/repositories/{id}/branches", h.Git.ListRepositoryBranches)
				r.With(requirePerm(authorization.PermSettingsManage)).Put("/repositories/{id}", h.Git.UpdateRepository)
			})

			// Search
			r.Route("/search", func(r chi.Router) {
				r.Use(middleware.RequireWorkspaceID)
				r.Use(wsActive)
				r.With(requirePerm(authorization.PermSearchRead)).Get("/", h.Search.Search)
			})

			r.Route("/command-bar", func(r chi.Router) {
				r.Use(middleware.RequireWorkspaceID)
				r.Use(wsActive)
				r.With(requireCommandBarRead()).Get("/plans", h.CommandBar.ListPlans)
				r.With(requireCommandBarRead()).Get("/plans/{planID}", h.CommandBar.GetPlan)
				r.With(requireCommandBarRead()).Get("/agents/{agentID}/tools", h.CommandBar.ListAgentToolCatalog)
				r.With(requireCommandBarEdit()).Post("/plans/dispatch", h.CommandBar.DispatchPlan)
				r.With(requireCommandBarEdit()).Post("/plans/{planID}/cancel", h.CommandBar.CancelPlan)
				r.With(requireCommandBarEdit()).Post("/plans/{planID}/resume", h.CommandBar.ResumePlan)
				r.With(requireCommandBarEdit()).Post("/plans/{planID}/retry", h.CommandBar.RetryPlan)
				r.With(requireCommandBarRead()).Post("/plans/dismiss", h.CommandBar.DismissPlans)
				r.With(requireCommandBarRead()).Post("/plans/{planID}/dismiss", h.CommandBar.DismissPlan)
				r.With(requirePerm(authorization.PermSettingsManage)).Post("/runs/{runID}/promote-agent", h.CommandBar.PromoteRunToAgent)
			})

			// Dock chats: private or explicitly shared conversations backed by
			// agent-runtime chat-mode runs. Run-scoped reads are proxied through
			// the chat visibility check (the generic /agent-runs routes are PM-gated).
			r.Route("/dock", func(r chi.Router) {
				r.Use(middleware.RequireWorkspaceID)
				r.Use(wsActive)
				r.With(requireCommandBarRead()).Get("/chats", h.DockChat.ListChats)
				r.With(requireCommandBarRead()).Post("/chats", h.DockChat.CreateChat)
				r.With(requireCommandBarRead()).Get("/chats/support-conversation", h.DockChat.FindSupportConversationChat)
				r.With(requireCommandBarRead()).Get("/chats/{chatID}", h.DockChat.GetChat)
				r.With(requireCommandBarRead()).Patch("/chats/{chatID}", h.DockChat.UpdateChat)
				r.With(requireCommandBarRead()).Get("/chats/{chatID}/messages", h.DockChat.ListMessages)
				r.With(requireCommandBarRead()).Get("/chats/{chatID}/messages/{messageID}/work", h.DockChat.GetMessageWorkDetail)
				r.With(requireCommandBarRead()).Post("/chats/{chatID}/messages", h.DockChat.SendMessage)
				r.With(requireCommandBarRead()).Post("/chats/{chatID}/title", h.DockChat.GenerateTitle)
				r.With(requireCommandBarRead()).Get("/chats/{chatID}/run", h.DockChat.GetChatRun)
				r.With(requireCommandBarRead()).Get("/chats/{chatID}/run/events", h.DockChat.ListChatRunEvents)
				r.With(requireCommandBarRead()).Get("/chats/{chatID}/run/interactions", h.DockChat.ListChatRunInteractions)
				r.With(requireCommandBarEdit()).Post("/chats/{chatID}/interactions/{interactionID}/resolve", h.DockChat.ResolveChatRunInteraction)
				r.With(requireCommandBarEdit()).Post("/chats/{chatID}/run/cancel", h.DockChat.CancelChatRun)
				r.With(requireCommandBarRead()).Get("/runs", h.DockChat.ListRuns)
				r.With(requireCommandBarRead()).Get("/runs/{runID}/snapshot", h.DockChat.GetRunSnapshot)
				if h.PublicShare != nil {
					r.With(requireCommandBarRead()).Get("/shares/{resourceType}/{resourceID}", h.PublicShare.GetLink)
					r.With(requireCommandBarRead()).Post("/shares/{resourceType}/{resourceID}", h.PublicShare.Create)
					r.With(requireCommandBarRead()).Delete("/shares/{resourceType}/{resourceID}", h.PublicShare.Revoke)
				}
				r.With(requireCommandBarRead()).Get("/runs/{runID}/events", h.DockChat.ListRunEvents)
				r.With(requireCommandBarRead()).Get("/runs/{runID}/interactions", h.DockChat.ListRunInteractions)
				r.With(requireCommandBarEdit()).Post("/runs/{runID}/interactions/{interactionID}/resolve", h.DockChat.ResolveRunInteraction)
				r.With(requireCommandBarEdit()).Post("/runs/{runID}/messages", h.DockChat.SendRunMessage)
				r.With(requireCommandBarEdit()).Post("/runs/{runID}/continue", h.DockChat.ContinueRun)
				r.With(requireCommandBarEdit()).Post("/runs/{runID}/cancel", h.DockChat.CancelRun)
				r.With(requireCommandBarEdit()).Post("/runs/{runID}/auth/device-code/start", h.DockChat.StartRunAuth)
				r.With(requireCommandBarEdit()).Post("/runs/{runID}/auth/device-code/cancel", h.DockChat.CancelRunAuth)
			})

			// Support module
			r.Route("/support", func(r chi.Router) {
				r.Use(middleware.RequireWorkspaceID)
				r.Use(wsActive)
				r.Use(requireModule(model.ModuleSupport))

				// Legacy /tickets routes (backward compat)
				r.With(requirePerm(authorization.PermSupportRead)).Get("/tickets", h.SupportInbox.ListConversations)
				r.With(requirePerm(authorization.PermSupportEdit)).Post("/tickets", h.SupportInbox.CreateConversation)
				r.With(requirePerm(authorization.PermSupportRead)).Get("/tickets/{id}", h.SupportInbox.GetConversation)
				r.With(requirePerm(authorization.PermSupportRead)).Get("/tickets/{id}/associations", h.Associations.ListConversationAssociations)
				r.With(requirePerm(authorization.PermSupportEdit)).Put("/tickets/{id}/status", h.SupportInbox.UpdateConversationStatus)
				r.With(requirePerm(authorization.PermSupportRead)).Get("/tickets/{id}/messages", h.SupportInbox.ListConversationMessages)
				r.With(requirePerm(authorization.PermSupportEdit)).Post("/tickets/{id}/messages", h.SupportInbox.CreateConversationMessage)
				r.With(requirePerm(authorization.PermSupportEdit)).Post("/tickets/{id}/link-task", h.SupportInbox.LinkConversationStory)
				r.With(requirePerm(authorization.PermSupportEdit), requirePerm(authorization.PermPMEdit)).Post("/tickets/{id}/create-task", h.SupportInbox.CreateTaskFromConversation)
				r.With(requirePerm(authorization.PermSupportEdit)).Post("/tickets/{id}/assign-agent", h.SupportInbox.AssignConversationAgent)
				r.With(requirePerm(authorization.PermSupportEdit)).Post("/tickets/{id}/assign-user", h.SupportInbox.AssignConversationUser)
				r.With(requirePerm(authorization.PermSupportEdit)).Post("/tickets/{id}/run-agent", h.SupportInbox.RunAgent)

				// New /inbox/conversations routes
				r.With(requirePerm(authorization.PermSupportRead)).Get("/inbox/unread-stats", h.SupportInbox.GetUnreadStats)
				r.With(requirePerm(authorization.PermSupportRead)).Get("/inbox/views", h.SupportInboxView.List)
				r.With(requirePerm(authorization.PermSupportRead)).Get("/inbox/views/builtin", h.SupportInboxView.ListBuiltin)
				r.With(requirePerm(authorization.PermSupportRead)).Get("/inbox/views/counts", h.SupportInboxView.ListCounts)
				r.With(requirePerm(authorization.PermSupportEdit)).Post("/inbox/views", h.SupportInboxView.Create)
				r.With(requirePerm(authorization.PermSupportEdit)).Put("/inbox/views/builtin/{viewKey}", h.SupportInboxView.UpdateBuiltin)
				r.With(requirePerm(authorization.PermSupportEdit)).Put("/inbox/views/{viewId}", h.SupportInboxView.Update)
				r.With(requirePerm(authorization.PermSupportEdit)).Delete("/inbox/views/{viewId}", h.SupportInboxView.Delete)
				r.With(requirePerm(authorization.PermSupportRead)).Get("/inbox/tags", h.SupportTag.List)
				r.With(requirePerm(authorization.PermSupportEdit)).Post("/inbox/tags", h.SupportTag.Create)
				r.With(requirePerm(authorization.PermSupportEdit)).Put("/inbox/tags/{tagId}", h.SupportTag.Update)
				r.With(requirePerm(authorization.PermSupportEdit)).Delete("/inbox/tags/{tagId}", h.SupportTag.Delete)
				r.With(requirePerm(authorization.PermSupportRead)).Get("/inbox/mailboxes/scopes", h.SupportInbox.ListInboxScopes)
				r.With(requirePerm(authorization.PermSupportAdmin)).Get("/inbox/mailboxes", h.SupportInbox.ListMailboxes)
				r.With(requirePerm(authorization.PermSupportAdmin)).Post("/inbox/mailboxes", h.SupportInbox.CreateMailbox)
				r.With(requirePerm(authorization.PermSupportAdmin)).Put("/inbox/mailboxes/{mailboxId}", h.SupportInbox.UpdateMailbox)
				r.With(requirePerm(authorization.PermSupportAdmin)).Post("/inbox/mailboxes/{mailboxId}/archive", h.SupportInbox.ArchiveMailbox)
				r.With(requirePerm(authorization.PermSupportAdmin)).Post("/inbox/mailboxes/reorder", h.SupportInbox.ReorderMailboxes)
				r.With(requirePerm(authorization.PermSupportAdmin)).Get("/inbox/mailboxes/{mailboxId}/members", h.SupportInbox.ListMailboxMembers)
				r.With(requirePerm(authorization.PermSupportAdmin)).Get("/inbox/email-routes", h.SupportInbox.ListEmailRoutes)
				r.With(requirePerm(authorization.PermSupportAdmin)).Post("/inbox/email-routes", h.SupportInbox.CreateEmailRoute)
				r.With(requirePerm(authorization.PermSupportAdmin)).Post("/inbox/email-routes/{routeId}/send-test", h.SupportInbox.SendEmailRouteTest)
				r.With(requirePerm(authorization.PermSupportAdmin)).Post("/inbox/email-routes/{routeId}/disable", h.SupportInbox.DisableEmailRoute)
				r.With(requirePerm(authorization.PermSupportAdmin)).Get("/inbox/email-senders", h.SupportInbox.ListEmailSenders)
				r.With(requirePerm(authorization.PermSupportAdmin)).Post("/inbox/email-senders", h.SupportInbox.CreateEmailSender)
				r.With(requirePerm(authorization.PermSupportAdmin)).Post("/inbox/email-senders/{senderId}/verify-dns", h.SupportInbox.VerifyEmailSender)
				r.With(requirePerm(authorization.PermSupportAdmin)).Put("/inbox/email-senders/{senderId}", h.SupportInbox.UpdateEmailSender)
				r.With(requirePerm(authorization.PermSupportAdmin)).Post("/inbox/email-senders/{senderId}/set-default", h.SupportInbox.SetDefaultEmailSender)
				r.With(requirePerm(authorization.PermSupportAdmin)).Post("/inbox/email-senders/{senderId}/disable", h.SupportInbox.DisableEmailSender)
				r.With(requirePerm(authorization.PermSupportAdmin)).Get("/inbox/email-sender-domains", h.SupportInbox.ListEmailSenderDomains)
				r.With(requirePerm(authorization.PermSupportAdmin)).Post("/inbox/email-sender-domains", h.SupportInbox.CreateEmailSenderDomain)
				r.With(requirePerm(authorization.PermSupportAdmin)).Post("/inbox/email-sender-domains/{domainId}/verify", h.SupportInbox.VerifyEmailSenderDomain)
				r.With(requirePerm(authorization.PermSupportAdmin)).Post("/inbox/email-sender-domains/{domainId}/activate", h.SupportInbox.ActivateEmailSenderDomain)
				r.With(requirePerm(authorization.PermSupportAdmin)).Post("/inbox/email-sender-domains/{domainId}/deactivate", h.SupportInbox.DeactivateEmailSenderDomain)
				r.With(requirePerm(authorization.PermSupportAdmin)).Get("/inbox/triage-rules", h.SupportInbox.ListTriageRules)
				r.With(requirePerm(authorization.PermSupportAdmin)).Post("/inbox/triage-rules", h.SupportInbox.CreateTriageRule)
				r.With(requirePerm(authorization.PermSupportAdmin)).Put("/inbox/triage-rules/{ruleId}", h.SupportInbox.UpdateTriageRule)
				r.With(requirePerm(authorization.PermSupportAdmin)).Delete("/inbox/triage-rules/{ruleId}", h.SupportInbox.DeleteTriageRule)
				r.With(requirePerm(authorization.PermSupportRead)).Get("/inbox/teammates/presence", h.SupportInbox.ListTeammatePresence)
				r.With(requirePerm(authorization.PermSupportRead)).Put("/inbox/me/presence", h.SupportInbox.UpdateMyTeammatePresence)
				r.With(requirePerm(authorization.PermSupportRead)).Get("/inbox/search", h.SupportInbox.SearchConversations)
				r.With(requirePerm(authorization.PermSupportRead)).Get("/inbox/conversations", h.SupportInbox.ListConversations)
				r.With(requirePerm(authorization.PermSupportEdit)).Post("/inbox/conversations", h.SupportInbox.CreateConversation)
				r.With(requirePerm(authorization.PermSupportEdit)).Post("/inbox/conversations/create-and-send", h.SupportInbox.CreateConversationWithMessage)
				if h.SupportAI != nil {
					r.With(requirePerm(authorization.PermSupportEdit)).Post("/inbox/rewrite-draft", h.SupportAI.RewriteNewSupportDraft)
				}
				r.With(requirePerm(authorization.PermSupportRead)).Get("/inbox/conversations/{id}", h.SupportInbox.GetConversation)
				r.With(requirePerm(authorization.PermSupportRead)).Get("/inbox/conversations/{id}/assignees", h.SupportInbox.ListConversationAssignableUsers)
				r.With(requirePerm(authorization.PermSupportRead)).Get("/inbox/conversations/{id}/associations", h.Associations.ListConversationAssociations)
				r.With(requirePerm(authorization.PermSupportEdit)).Put("/inbox/conversations/{id}/status", h.SupportInbox.UpdateConversationStatus)
				r.With(requirePerm(authorization.PermSupportRead)).Get("/inbox/conversations/{id}/messages", h.SupportInbox.ListConversationMessages)
				r.With(requirePerm(authorization.PermSupportRead)).Get("/inbox/conversations/{id}/message-pages", h.SupportInbox.ListConversationMessagePage)
				r.With(requirePerm(authorization.PermSupportRead)).Get("/inbox/messages/{id}/email", h.SupportInbox.GetMessageEmailDetail)
				if h.EmailImageProxy != nil {
					r.With(requirePerm(authorization.PermSupportRead)).Get("/email/image-proxy", h.EmailImageProxy.Proxy)
				}
				r.With(requirePerm(authorization.PermSupportEdit)).Post("/inbox/conversations/{id}/messages", h.SupportInbox.CreateConversationMessage)
				r.With(requirePerm(authorization.PermSupportEdit)).Delete("/inbox/conversations/{id}/messages/{msg_id}", h.SupportInbox.DeleteMessage)
				r.With(requirePerm(authorization.PermSupportRead)).Get("/inbox/conversations/{id}/messages/{msg_id}", h.SupportInbox.GetMessageInfo)
				if h.SupportAI != nil {
					r.With(requirePerm(authorization.PermSupportEdit)).Post("/inbox/conversations/{id}/rewrite-draft", h.SupportAI.RewriteSupportDraft)
				}
				r.With(requirePerm(authorization.PermSupportEdit)).Post("/inbox/conversations/{id}/link-task", h.SupportInbox.LinkConversationStory)
				r.With(requirePerm(authorization.PermSupportEdit), requirePerm(authorization.PermPMEdit)).Post("/inbox/conversations/{id}/create-task", h.SupportInbox.CreateTaskFromConversation)
				r.With(requirePerm(authorization.PermSupportEdit)).Post("/inbox/conversations/{id}/assign-agent", h.SupportInbox.AssignConversationAgent)
				r.With(requirePerm(authorization.PermSupportEdit)).Post("/inbox/conversations/{id}/assign-user", h.SupportInbox.AssignConversationUser)
				r.With(requirePerm(authorization.PermSupportEdit)).Put("/inbox/conversations/{id}/crm-contact", h.SupportInbox.UpdateConversationCRMContact)
				r.With(requirePerm(authorization.PermSupportEdit)).Put("/inbox/conversations/{id}/crm-company", h.SupportInbox.UpdateConversationCRMCompany)
				r.With(requirePerm(authorization.PermSupportEdit)).Put("/inbox/conversations/{id}/customer-name", h.SupportInbox.UpdateConversationCustomerName)
				r.With(requirePerm(authorization.PermSupportEdit)).Put("/inbox/conversations/{id}/email-recipients", h.SupportInbox.UpdateConversationEmailRecipients)
				r.With(requirePerm(authorization.PermSupportEdit)).Post("/inbox/conversations/{id}/transcript", h.SupportInbox.SendConversationTranscript)
				r.With(requirePerm(authorization.PermSupportEdit)).Post("/inbox/conversations/{id}/run-agent", h.SupportInbox.RunAgent)
				r.With(requirePerm(authorization.PermSupportRead)).Get("/inbox/conversations/{id}/ai-run/interactions", h.SupportInbox.ListAIRunInteractions)
				r.With(requirePerm(authorization.PermSupportEdit)).Post("/inbox/conversations/{id}/ai-run/interactions/{interactionID}/resolve", h.SupportInbox.ResolveAIRunInteraction)
				r.With(requirePerm(authorization.PermSupportEdit)).Post("/inbox/conversations/{id}/move", h.SupportInbox.MoveConversation)
				r.With(requirePerm(authorization.PermSupportEdit)).Post("/inbox/conversations/{id}/tags/{tagId}", h.SupportTag.AddConversationTag)
				r.With(requirePerm(authorization.PermSupportEdit)).Delete("/inbox/conversations/{id}/tags/{tagId}", h.SupportTag.RemoveConversationTag)
				r.With(requirePerm(authorization.PermSupportEdit)).Post("/inbox/conversations/{id}/triage/dismiss", h.SupportInbox.DismissConversationTriage)
				r.With(requirePerm(authorization.PermSupportRead)).Post("/inbox/conversations/{id}/read", h.SupportInbox.MarkConversationRead)
				r.With(requirePerm(authorization.PermSupportEdit)).Post("/inbox/conversations/{id}/unread", h.SupportInbox.MarkConversationUnread)
				r.With(requirePerm(authorization.PermSupportEdit)).Put("/inbox/conversations/{id}/subject", h.SupportInbox.UpdateConversationSubject)
				r.With(requirePerm(authorization.PermSupportEdit)).Delete("/inbox/conversations/{id}", h.SupportInbox.DeleteConversation)
				r.With(requirePerm(authorization.PermSupportRead)).Get("/inbox/conversations/{id}/visitor-context", h.SupportInbox.GetVisitorContext)

				// Installation settings
				r.With(requirePerm(authorization.PermSupportRead)).Get("/inbox/installations", h.SupportInbox.GetInstallation)
				r.With(requirePerm(authorization.PermSupportRead)).Get("/inbox/routing-usage", h.SupportInbox.GetRoutingUsageStatus)
				r.With(requirePerm(authorization.PermSupportAdmin)).Patch("/inbox/installations", h.SupportInbox.UpdateInstallationSettings)
				r.With(requirePerm(authorization.PermSupportAdmin)).Post("/inbox/installations/regenerate-key", h.SupportInbox.RegenerateWidgetKey)
				r.With(requirePerm(authorization.PermSupportAdmin)).Post("/inbox/installations/rotate-secret", h.SupportInbox.RotateWidgetSecret)

				// Canned responses
				r.With(requirePerm(authorization.PermSupportRead)).Get("/inbox/canned-responses", h.SupportInbox.ListCannedResponses)
				r.With(requirePerm(authorization.PermSupportRead)).Get("/inbox/canned-responses/search", h.SupportInbox.SearchCannedResponses)
				r.With(requirePerm(authorization.PermSupportAdmin)).Post("/inbox/canned-responses", h.SupportInbox.CreateCannedResponse)
				r.With(requirePerm(authorization.PermSupportAdmin)).Put("/inbox/canned-responses/{id}", h.SupportInbox.UpdateCannedResponse)
				r.With(requirePerm(authorization.PermSupportAdmin)).Delete("/inbox/canned-responses/{id}", h.SupportInbox.DeleteCannedResponse)

				// Typing indicators — Deprecated: use WebSocket support:typing:start/stop instead. Kept as HTTP fallback.
				r.With(requirePerm(authorization.PermSupportEdit)).Post("/inbox/conversations/{id}/typing", h.SupportInbox.TypingIndicator)

				// Viewing presence — Deprecated: use WebSocket support:viewing:start/stop instead. Kept as HTTP fallback.
				r.With(requirePerm(authorization.PermSupportRead)).Post("/inbox/conversations/{id}/viewing", h.SupportInbox.ViewingPresence)

				// File attachments
				if h.SupportAttachment != nil {
					r.With(requirePerm(authorization.PermSupportEdit)).Post("/inbox/conversations/{convId}/attachments", h.SupportAttachment.Create)
					r.With(requirePerm(authorization.PermSupportEdit)).Patch("/inbox/attachments/{attachmentId}/confirm", h.SupportAttachment.ConfirmUpload)
					r.With(requirePerm(authorization.PermSupportEdit)).Delete("/inbox/attachments/{attachmentId}", h.SupportAttachment.Delete)
				}

				// Docs coverage
				if h.SupportCoverage != nil {
					r.Route("/coverage", func(r chi.Router) {
						r.With(requirePerm(authorization.PermSupportRead)).Get("/v2/topics", h.SupportCoverage.ListTopicsV2)
						r.With(requirePerm(authorization.PermSupportRead)).Get("/v2/topics/{topicId}", h.SupportCoverage.GetTopicV2)
						r.With(requirePerm(authorization.PermSupportRead)).Get("/v2/signals", h.SupportCoverage.ListSignalsV2)
						r.With(requirePerm(authorization.PermSupportEdit)).Post("/v2/signals/{signalId}/review", h.SupportCoverage.ReviewSignalV2)
						r.With(requirePerm(authorization.PermSupportEdit)).Post("/v2/signals/{signalId}/dismiss", h.SupportCoverage.DismissSignalV2)
						r.With(requirePerm(authorization.PermSupportRead)).Get("/v2/health", h.SupportCoverage.PipelineHealthV2)
						r.With(requirePerm(authorization.PermSettingsManage)).Post("/v2/attempts/{attemptId}/replay", h.SupportCoverage.ReplayAttemptV2)
						r.With(requirePerm(authorization.PermSettingsManage)).Get("/v2/legacy", h.SupportCoverage.ListArchivedV1)
						r.With(requirePerm(authorization.PermSupportEdit)).Post("/events", h.SupportCoverage.RecordEvent)
						r.With(requirePerm(authorization.PermSupportRead)).Get("/summary", h.SupportCoverage.GetSummary)
						r.With(requirePerm(authorization.PermSupportRead)).Get("/gaps", h.SupportCoverage.ListGaps)
						r.With(requirePerm(authorization.PermSupportRead)).Get("/gaps/{gapId}", h.SupportCoverage.GetGap)
						r.With(requirePerm(authorization.PermSupportRead)).Get("/gaps/{gapId}/merge-suggestions", h.SupportCoverage.ListMergeSuggestions)
						r.With(requirePerm(authorization.PermSupportEdit)).Post("/gaps/{gapId}/regenerate", h.SupportCoverage.RegenerateGap)
						r.With(requirePerm(authorization.PermSupportEdit)).Post("/gaps/{gapId}/status", h.SupportCoverage.UpdateGapStatus)
						r.With(requirePerm(authorization.PermSupportEdit)).Post("/gaps/{gapId}/reclassify", h.SupportCoverage.ReclassifyGap)
						r.With(requirePerm(authorization.PermSupportEdit), requirePerm(authorization.PermDocsEdit)).Post("/gaps/{gapId}/add", h.SupportCoverage.AddDocumentToGap)
						r.With(requirePerm(authorization.PermSupportEdit)).Post("/gaps/{gapId}/merge", h.SupportCoverage.MergeGap)
						r.With(requirePerm(authorization.PermSupportEdit), requirePerm(authorization.PermDocsEdit)).Post("/gaps/{gapId}/suggestions/article-draft", h.SupportCoverage.CreateArticleDraftSuggestion)
						r.With(requirePerm(authorization.PermSupportEdit), requirePerm(authorization.PermDocsEdit)).Post("/gaps/{gapId}/suggestions/article-update", h.SupportCoverage.CreateArticleUpdateSuggestion)
						r.With(requirePerm(authorization.PermSupportEdit), requirePerm(authorization.PermDocsEdit)).Post("/suggestions/{suggestionId}/apply", h.SupportCoverage.ApplySuggestion)
						r.With(requirePerm(authorization.PermSupportEdit)).Post("/suggestions/{suggestionId}/discard", h.SupportCoverage.DiscardSuggestion)
						r.With(requirePerm(authorization.PermSupportRead)).Get("/conversations/{conversationId}/state", h.SupportCoverage.GetConversationState)
						r.With(requirePerm(authorization.PermSupportEdit)).Post("/conversations/{conversationId}/docs-issue", h.SupportCoverage.SubmitDocsIssueFeedback)
						r.With(requirePerm(authorization.PermSettingsManage)).Post("/reanalyze", h.SupportCoverage.TriggerReanalysis)
						// Cluster rebuild endpoints compare existing gaps and surface merge suggestions.
						r.With(requirePerm(authorization.PermSettingsManage)).Post("/clusters/rebuild", h.SupportCoverage.RebuildClusters)
						r.With(requirePerm(authorization.PermSupportRead)).Get("/clusters/rebuild/latest", h.SupportCoverage.GetLatestClusterRebuild)
						r.With(requirePerm(authorization.PermSupportEdit)).Post("/merge-suggestions/{suggestionId}/apply", h.SupportCoverage.ApplyMergeSuggestion)
						r.With(requirePerm(authorization.PermSupportEdit)).Post("/merge-suggestions/{suggestionId}/dismiss", h.SupportCoverage.DismissMergeSuggestion)
					})
				}
			})

			// PM module
			r.Route("/pm", func(r chi.Router) {
				r.Use(middleware.RequireWorkspaceID)
				r.Use(wsActive)
				if h.SupportAI != nil {
					r.With(requirePerm(authorization.PermPMEdit)).Post("/rewrite-draft", h.SupportAI.RewritePMCommentDraft)
				}

				// Workflows — read: pm.read, write: pm.admin.workflows
				r.With(requirePerm(authorization.PermPMRead)).Get("/workflows", h.PMWorkflow.List)
				r.With(requirePerm(authorization.PermPMAdminWorkflows)).Post("/workflows", h.PMWorkflow.Create)
				r.With(requirePerm(authorization.PermPMRead)).Get("/workflows/epic-states", h.PMWorkflow.ListEpicStates)
				r.With(requirePerm(authorization.PermPMRead)).Get("/workflows/resolve", h.PMWorkflow.ResolveTeamWorkflow)
				r.With(requirePerm(authorization.PermPMRead)).Get("/workflows/{id}", h.PMWorkflow.Get)
				r.With(requirePerm(authorization.PermPMAdminWorkflows)).Put("/workflows/{id}", h.PMWorkflow.Update)
				r.With(requirePerm(authorization.PermPMAdminWorkflows)).Delete("/workflows/{id}", h.PMWorkflow.Delete)
				r.With(requirePerm(authorization.PermPMAdminWorkflows)).Post("/workflows/{id}/copy-to-team", h.PMWorkflow.CopyToTeam)
				r.With(requirePerm(authorization.PermPMAdminWorkflows)).Post("/workflows/{id}/states", h.PMWorkflow.CreateState)
				r.With(requirePerm(authorization.PermPMAdminWorkflows)).Put("/workflows/{id}/states/{stateId}", h.PMWorkflow.UpdateState)
				r.With(requirePerm(authorization.PermPMAdminWorkflows)).Delete("/workflows/{id}/states/{stateId}", h.PMWorkflow.DeleteState)
				r.With(requirePerm(authorization.PermPMAdminWorkflows)).Put("/workflows/{id}/states/reorder", h.PMWorkflow.ReorderStates)

				// Views — pm.read / pm.edit
				r.With(requirePerm(authorization.PermPMRead)).Get("/views", h.PMView.List)
				r.With(requirePerm(authorization.PermPMEdit)).Post("/views", h.PMView.Create)
				r.With(requirePerm(authorization.PermPMEdit)).Put("/views/{id}", h.PMView.Update)
				r.With(requirePerm(authorization.PermPMEdit)).Delete("/views/{id}", h.PMView.Delete)

				// Labels — read: pm.read, write: pm.admin.labels
				r.With(requirePerm(authorization.PermPMRead)).Get("/labels", h.PMLabel.List)
				r.With(requirePerm(authorization.PermPMRead)).Get("/labels/stats", h.PMLabel.ListWithStats)
				r.With(requirePerm(authorization.PermPMAdminLabels)).Post("/labels", h.PMLabel.Create)
				r.With(requirePerm(authorization.PermPMAdminLabels)).Put("/labels/{id}", h.PMLabel.Update)
				r.With(requirePerm(authorization.PermPMAdminLabels)).Delete("/labels/{id}", h.PMLabel.Delete)

				// Task Templates — pm.edit
				r.With(requirePerm(authorization.PermPMRead)).Get("/task-templates", h.PMTaskTemplate.List)
				r.With(requirePerm(authorization.PermPMRead)).Get("/task-templates/{id}", h.PMTaskTemplate.Get)
				r.With(requirePerm(authorization.PermPMEdit)).Post("/task-templates", h.PMTaskTemplate.Create)
				r.With(requirePerm(authorization.PermPMEdit)).Put("/task-templates/{id}", h.PMTaskTemplate.Update)
				r.With(requirePerm(authorization.PermPMEdit)).Delete("/task-templates/{id}", h.PMTaskTemplate.Delete)

				// Recurring Templates — pm.read / pm.edit
				r.With(requirePerm(authorization.PermPMRead)).Get("/recurring-templates", h.PMRecurringTemplate.List)
				r.With(requirePerm(authorization.PermPMRead)).Get("/recurring-templates/{id}", h.PMRecurringTemplate.Get)
				r.With(requirePerm(authorization.PermPMEdit)).Post("/recurring-templates", h.PMRecurringTemplate.Create)
				r.With(requirePerm(authorization.PermPMEdit)).Put("/recurring-templates/{id}", h.PMRecurringTemplate.Update)
				r.With(requirePerm(authorization.PermPMEdit)).Post("/recurring-templates/{id}/pause", h.PMRecurringTemplate.Pause)
				r.With(requirePerm(authorization.PermPMEdit)).Post("/recurring-templates/{id}/resume", h.PMRecurringTemplate.Resume)
				r.With(requirePerm(authorization.PermPMEdit)).Post("/recurring-templates/{id}/stop", h.PMRecurringTemplate.Stop)
				r.With(requirePerm(authorization.PermPMEdit)).Post("/recurring-templates/{id}/skip-next", h.PMRecurringTemplate.SkipNext)
				r.With(requirePerm(authorization.PermPMEdit)).Post("/recurring-templates/{id}/generate-now", h.PMRecurringTemplate.GenerateNow)
				r.With(requirePerm(authorization.PermPMEdit)).Post("/recurring-templates/{id}/duplicate", h.PMRecurringTemplate.Duplicate)

				// Roadmap — pm.read
				r.With(requirePerm(authorization.PermPMRead)).Get("/roadmap", h.PMRoadmap.Get)

				// Epics — pm.read / pm.edit
				r.With(requirePerm(authorization.PermPMRead)).Get("/epics", h.PMEpic.List)
				r.With(requirePerm(authorization.PermPMEdit)).Post("/epics", h.PMEpic.Create)
				r.With(requirePerm(authorization.PermPMRead)).Get("/epics/{id}", h.PMEpic.Get)
				r.With(requirePerm(authorization.PermPMEdit)).Put("/epics/{id}", h.PMEpic.Update)
				r.With(requirePerm(authorization.PermPMEdit)).Delete("/epics/{id}", h.PMEpic.Delete)
				r.With(requirePerm(authorization.PermPMRead)).Get("/epics/{id}/tasks", h.PMEpic.ListTasks)
				r.With(requirePerm(authorization.PermPMEdit)).Post("/epics/{id}/tasks/link", h.PMEpic.LinkTasks)
				r.With(requirePerm(authorization.PermPMRead)).Get("/epics/{id}/activity", h.PMEpic.ListActivity)
				r.With(requirePerm(authorization.PermPMRead)).Get("/epics/{id}/delivery-target", h.Git.GetEpicDeliveryTarget)
				r.With(requirePerm(authorization.PermPMEdit)).Put("/epics/{id}/delivery-target", h.Git.UpdateEpicDeliveryTarget)
				r.With(requirePerm(authorization.PermPMEdit)).Put("/epics/{id}/health", h.PMEpic.UpdateHealth)
				r.With(requirePerm(authorization.PermPMRead)).Get("/epics/{id}/associations", h.Associations.ListEpicAssociations)
				r.With(requirePerm(authorization.PermPMRead)).Get("/epics/{id}/command-bar-plans", h.CommandBar.ListEpicPlans)
				r.With(requirePerm(authorization.PermPMEdit)).Post("/epics/{id}/delivery-pipeline", h.CommandBar.StartEpicDeliveryPipeline)
				// Sprints (PM) — pm.read / pm.edit
				r.With(requirePerm(authorization.PermPMRead)).Get("/sprints", h.PMSprint.List)
				r.With(requirePerm(authorization.PermPMRead)).Get("/sprints/planning", h.PMSprint.PlanningWorkspace)
				r.With(requirePerm(authorization.PermPMRead)).Get("/sprints/backlog-tasks", h.PMSprint.ListBacklogTasks)
				r.With(requirePerm(authorization.PermPMRead)).Get("/sprints/closeouts", h.PMSprint.ListCloseouts)
				r.With(requirePerm(authorization.PermPMEdit)).Post("/sprints", h.PMSprint.Create)
				r.With(requirePerm(authorization.PermPMRead)).Get("/sprints/{id}", h.PMSprint.Get)
				r.With(requirePerm(authorization.PermPMRead)).Get("/sprints/{id}/closeout", h.PMSprint.GetCloseout)
				r.With(requirePerm(authorization.PermPMEdit)).Put("/sprints/{id}", h.PMSprint.Update)
				r.With(requirePerm(authorization.PermPMEdit)).Delete("/sprints/{id}", h.PMSprint.Delete)
				r.With(requirePerm(authorization.PermPMRead)).Get("/sprints/{id}/tasks", h.PMSprint.ListTasks)
				r.With(requirePerm(authorization.PermPMEdit)).Post("/sprints/{id}/tasks/link", h.PMSprint.LinkTasks)
				r.With(requirePerm(authorization.PermPMRead)).Get("/sprints/{id}/preview-tasks", h.PMSprint.ListPreviewTasks)

				// Tasks — pm.read / pm.edit
				r.With(requirePerm(authorization.PermPMRead)).Get("/tasks", h.PMTask.List)
				r.With(requirePerm(authorization.PermPMEdit)).Post("/tasks", h.PMTask.Create)
				r.With(requirePerm(authorization.PermPMEdit)).Post("/tasks/seed", h.PMTask.Seed)
				r.With(requirePerm(authorization.PermPMRead)).Get("/tasks/board", h.PMTask.ListBoard)
				r.With(requirePerm(authorization.PermPMRead)).Get("/tasks/board/column", h.PMTask.ListBoardColumn)
				r.With(requirePerm(authorization.PermPMRead)).Get("/tasks/board/members", h.PMTask.ListBoardByMember)
				r.With(requirePerm(authorization.PermPMRead)).Get("/tasks/board/members/column", h.PMTask.ListBoardMemberColumn)
				r.With(requirePerm(authorization.PermPMRead)).Get("/tasks/counts", h.PMTask.CountByState)
				r.With(requirePerm(authorization.PermPMRead)).Get("/tasks/display/{displayID}", h.PMTask.GetByDisplayID)
				r.With(requirePerm(authorization.PermPMRead)).Get("/tasks/{id}", h.PMTask.Get)
				r.With(requirePerm(authorization.PermPMEdit)).Post("/tasks/{id}/save-as-template", h.PMTask.SaveAsTemplate)
				r.With(requirePerm(authorization.PermPMEdit)).Post("/tasks/{id}/duplicate", h.PMTask.Duplicate)
				r.With(requirePerm(authorization.PermPMEdit)).Put("/tasks/{id}", h.PMTask.Update)
				r.With(requirePerm(authorization.PermPMEdit)).Delete("/tasks/{id}", h.PMTask.Delete)
				r.With(requirePerm(authorization.PermPMEdit)).Put("/tasks/{id}/move", h.PMTask.Move)
				r.With(requirePerm(authorization.PermPMEdit)).Put("/tasks/{id}/reorder", h.PMTask.Reorder)
				r.With(requirePerm(authorization.PermPMEdit)).Post("/tasks/{id}/owners", h.PMTask.AddOwner)
				r.With(requirePerm(authorization.PermPMEdit)).Delete("/tasks/{id}/owners/{userId}", h.PMTask.RemoveOwner)
				r.With(requirePerm(authorization.PermPMEdit)).Post("/tasks/{id}/followers", h.PMTask.AddFollower)
				r.With(requirePerm(authorization.PermPMEdit)).Delete("/tasks/{id}/followers", h.PMTask.RemoveFollower)
				r.With(requirePerm(authorization.PermPMEdit)).Post("/tasks/{id}/labels", h.PMTask.AddLabel)
				r.With(requirePerm(authorization.PermPMEdit)).Delete("/tasks/{id}/labels/{labelId}", h.PMTask.RemoveLabel)
				r.With(requirePerm(authorization.PermPMRead)).Get("/tasks/{taskId}/recurring-template", h.PMRecurringTemplate.GetByStory)
				r.With(requirePerm(authorization.PermPMRead)).Get("/tasks/{id}/associations", h.Associations.ListTaskAssociations)
				r.With(requirePerm(authorization.PermPMEdit)).Post("/tasks/{id}/relationships", h.Associations.CreateTaskRelationship)
				r.With(requirePerm(authorization.PermPMEdit)).Delete("/task-relationships/{id}", h.Associations.DeleteTaskRelationship)
				r.With(requirePerm(authorization.PermPMRead)).Get("/tasks/{id}/activity", h.PMTask.ListActivity)
				r.With(requirePerm(authorization.PermPMRead)).Get("/tasks/{id}/updates", h.PMTaskInsights.ListUpdates)
				r.With(requirePerm(authorization.PermPMRead)).Put("/tasks/{id}/updates/read-state", h.PMTaskInsights.UpdateReadState)
				r.With(requirePerm(authorization.PermPMRead)).Get("/tasks/{id}/standing-brief", h.PMTaskInsights.GetStandingBrief)
				r.With(requirePerm(authorization.PermPMEdit)).Post("/tasks/{id}/standing-brief/refresh", h.PMTaskInsights.RefreshStandingBrief)
				r.With(requirePerm(authorization.PermPMEdit)).Post("/tasks/{id}/standing-brief/suggestions/{key}/dismiss", h.PMTaskInsights.DismissStandingBriefSuggestion)
				r.With(requirePerm(authorization.PermPMRead)).Get("/tasks/{id}/git-links", h.Git.GetTaskGitLinks)
				r.With(requirePerm(authorization.PermPMRead)).Get("/tasks/{id}/delivery-target", h.Git.GetTaskDeliveryTarget)
				r.With(requirePerm(authorization.PermPMEdit)).Put("/tasks/{id}/delivery-target", h.Git.UpdateTaskDeliveryTarget)
				r.With(requirePerm(authorization.PermPMEdit)).Post("/tasks/{id}/delivery-target/use-epic", h.Git.UseTaskEpicDeliveryTarget)
				r.With(requirePerm(authorization.PermPMEdit)).Post("/tasks/{id}/create-branch", h.Git.CreateBranch)

				// Comments — pm.read / pm.edit
				r.With(requirePerm(authorization.PermPMRead)).Get("/comments", h.PMComment.List)
				r.With(requirePerm(authorization.PermPMEdit)).Post("/comments", h.PMComment.Create)
				r.With(requirePerm(authorization.PermPMEdit)).Put("/comments/{id}", h.PMComment.Update)
				r.With(requirePerm(authorization.PermPMEdit)).Delete("/comments/{id}", h.PMComment.Delete)
				r.With(requirePerm(authorization.PermPMEdit)).Post("/comments/{id}/resolve", h.PMComment.Resolve)
				r.With(requirePerm(authorization.PermPMEdit)).Post("/comments/{id}/reopen", h.PMComment.Reopen)
				r.With(requirePerm(authorization.PermPMRead)).Post("/comments/{id}/reactions", h.PMComment.ToggleReaction)

				// Attachments — pm.edit
				r.With(requirePerm(authorization.PermPMEdit)).Post("/attachments", h.PMAttachment.Create)
				r.With(requirePerm(authorization.PermPMEdit)).Patch("/attachments/{id}/confirm", h.PMAttachment.ConfirmUpload)
				r.With(requirePerm(authorization.PermPMRead)).Get("/attachments", h.PMAttachment.List)
				r.With(requirePerm(authorization.PermPMRead)).Get("/attachments/{id}/content", h.PMAttachment.Content)
				r.With(requirePerm(authorization.PermPMEdit)).Delete("/attachments/{id}", h.PMAttachment.Delete)

				// Objectives — pm.read / pm.edit
				r.With(requirePerm(authorization.PermPMRead)).Get("/objectives", h.PMObjective.List)
				r.With(requirePerm(authorization.PermPMEdit)).Post("/objectives", h.PMObjective.Create)
				r.With(requirePerm(authorization.PermPMRead)).Get("/objectives/{id}", h.PMObjective.Get)
				r.With(requirePerm(authorization.PermPMEdit)).Put("/objectives/{id}", h.PMObjective.Update)
				r.With(requirePerm(authorization.PermPMEdit)).Delete("/objectives/{id}", h.PMObjective.Delete)
				r.With(requirePerm(authorization.PermPMEdit)).Post("/objectives/{id}/teams", h.PMObjective.AddTeam)
				r.With(requirePerm(authorization.PermPMEdit)).Delete("/objectives/{id}/teams/{teamId}", h.PMObjective.RemoveTeam)
				r.With(requirePerm(authorization.PermPMEdit)).Post("/objectives/{id}/owners", h.PMObjective.AddOwner)
				r.With(requirePerm(authorization.PermPMEdit)).Delete("/objectives/{id}/owners/{userId}", h.PMObjective.RemoveOwner)
				r.With(requirePerm(authorization.PermPMEdit)).Post("/objectives/{id}/epics", h.PMObjective.AddEpic)
				r.With(requirePerm(authorization.PermPMEdit)).Delete("/objectives/{id}/epics/{epicId}", h.PMObjective.RemoveEpic)
				r.With(requirePerm(authorization.PermPMEdit)).Post("/objectives/{id}/key-results", h.PMObjective.CreateKeyResult)
				r.With(requirePerm(authorization.PermPMEdit)).Put("/key-results/{id}", h.PMObjective.UpdateKeyResult)
				r.With(requirePerm(authorization.PermPMEdit)).Delete("/key-results/{id}", h.PMObjective.DeleteKeyResult)

				// Checklist items — pm.edit
				r.With(requirePerm(authorization.PermPMRead)).Get("/tasks/{id}/checklist", h.PMChecklistItem.List)
				r.With(requirePerm(authorization.PermPMEdit)).Post("/tasks/{id}/checklist", h.PMChecklistItem.Create)
				r.With(requirePerm(authorization.PermPMEdit)).Put("/checklist-items/{id}", h.PMChecklistItem.Update)
				r.With(requirePerm(authorization.PermPMEdit)).Delete("/checklist-items/{id}", h.PMChecklistItem.Delete)

				// External links — pm.edit
				r.With(requirePerm(authorization.PermPMRead)).Get("/tasks/{id}/links", h.PMExternalLink.List)
				r.With(requirePerm(authorization.PermPMEdit)).Post("/tasks/{id}/links", h.PMExternalLink.Create)
				r.With(requirePerm(authorization.PermPMEdit)).Put("/links/{id}", h.PMExternalLink.Update)
				r.With(requirePerm(authorization.PermPMEdit)).Delete("/links/{id}", h.PMExternalLink.Delete)

				// Generic entity external links — pm.read / pm.edit
				r.With(requirePerm(authorization.PermPMRead)).Get("/entity-links/{entity_type}/{entity_id}", h.PMExternalLink.ListByEntity)
				r.With(requirePerm(authorization.PermPMEdit)).Post("/entity-links/{entity_type}/{entity_id}", h.PMExternalLink.CreateForEntity)

				// Automations — pm.admin.automations
				r.With(requirePerm(authorization.PermPMRead)).Get("/automations", h.PMAutomation.List)
				r.With(requirePerm(authorization.PermPMAdminAutomations)).Put("/automations", h.PMAutomation.Upsert)
				r.With(requirePerm(authorization.PermPMAdminAutomations)).Delete("/automations", h.PMAutomation.Delete)

				// Automation Rules — pm.admin.automations
				r.Route("/automation-rules", func(r chi.Router) {
					r.With(requirePerm(authorization.PermPMRead)).Get("/", h.AutomationRule.List)
					r.With(requirePerm(authorization.PermPMAdminAutomations)).Post("/", h.AutomationRule.Create)
					r.Route("/{ruleId}", func(r chi.Router) {
						r.With(requirePerm(authorization.PermPMRead)).Get("/", h.AutomationRule.Get)
						r.With(requirePerm(authorization.PermPMAdminAutomations)).Put("/", h.AutomationRule.Update)
						r.With(requirePerm(authorization.PermPMAdminAutomations)).Delete("/", h.AutomationRule.Delete)
					})
				})

				// Associations (PM-side) — pm.edit
				r.With(requirePerm(authorization.PermPMEdit)).Post("/associations", h.CRMAssociation.Create)
				r.With(requirePerm(authorization.PermPMEdit)).Delete("/associations/{id}", h.CRMAssociation.Delete)

				if h.SupportAI != nil {
					r.With(requirePerm(authorization.PermPMRead)).Get("/agents/{id}/knowledge-sources", h.SupportAI.GetKnowledgeSources)
					r.With(requirePerm(authorization.PermPMRead)).Post("/agents/{id}/support-preview", h.SupportAI.PreviewSupportReply)
					r.With(requirePerm(authorization.PermPMEdit)).Put("/agents/{id}/knowledge-sources", h.SupportAI.UpdateKnowledgeSources)
					r.With(requirePerm(authorization.PermPMEdit)).Post("/agents/{id}/knowledge-sources/{spaceId}/reindex", h.SupportAI.ReindexKnowledgeSource)
					r.With(requirePerm(authorization.PermPMRead)).Get("/agents/{id}/curated-guidance", h.SupportAI.ListCuratedGuidance)
					r.With(requirePerm(authorization.PermPMEdit)).Post("/agents/{id}/curated-guidance", h.SupportAI.CreateCuratedGuidance)
					r.With(requirePerm(authorization.PermPMEdit)).Put("/agents/{id}/curated-guidance/{guidanceId}", h.SupportAI.UpdateCuratedGuidance)
					r.With(requirePerm(authorization.PermPMEdit)).Delete("/agents/{id}/curated-guidance/{guidanceId}", h.SupportAI.DeleteCuratedGuidance)
					r.With(requirePerm(authorization.PermPMRead)).Get("/agents/{id}/content-sources", h.SupportAI.GetAgentContentSources)
					r.With(requirePerm(authorization.PermPMEdit)).Put("/agents/{id}/content-sources", h.SupportAI.UpdateAgentContentSources)
					r.With(requirePerm(authorization.PermPMRead)).Get("/content-sources", h.SupportAI.ListContentSources)
					r.With(requirePerm(authorization.PermPMEdit)).Post("/content-sources", h.SupportAI.CreateContentSource)
					r.With(requirePerm(authorization.PermPMEdit)).Post("/content-sources/files", h.SupportAI.CreateContentSourceFileUpload)
					r.With(requirePerm(authorization.PermPMEdit)).Patch("/content-sources/{contentSourceId}/file/confirm", h.SupportAI.ConfirmContentSourceFileUpload)
					r.With(requirePerm(authorization.PermPMEdit)).Put("/content-sources/{contentSourceId}", h.SupportAI.UpdateContentSource)
					r.With(requirePerm(authorization.PermPMEdit)).Delete("/content-sources/{contentSourceId}", h.SupportAI.DeleteContentSource)
					r.With(requirePerm(authorization.PermPMRead)).Get("/content-sources/{contentSourceId}/pages", h.SupportAI.ListContentSourcePages)
					r.With(requirePerm(authorization.PermPMRead)).Get("/content-sources/{contentSourceId}/pages/{pageId}", h.SupportAI.GetContentSourcePage)
					r.With(requirePerm(authorization.PermPMEdit)).Post("/content-sources/{contentSourceId}/reindex", h.SupportAI.ReindexContentSource)
				}

				// Agents — gated by Automation module access plus PM permissions.
				r.Group(func(r chi.Router) {
					r.Use(requireModule(model.ModuleAutomation))
					r.With(requirePerm(authorization.PermPMRead)).Get("/agents", h.Agent.ListAgents)
					r.With(requirePerm(authorization.PermPMEdit)).Post("/agents", h.Agent.CreateAgent)
					r.With(requirePerm(authorization.PermPMRead)).Get("/agent-presets", h.Agent.ListAgentPresets)
					r.With(requirePerm(authorization.PermPMEdit)).Post("/agent-preset-versions", h.Agent.CreateWorkspacePresetVersion)
					r.With(requirePerm(authorization.PermPMEdit)).Put("/agent-preset-versions/{id}", h.Agent.UpdateWorkspacePresetVersion)
					r.With(requirePerm(authorization.PermPMEdit)).Delete("/agent-preset-versions/{id}", h.Agent.DeleteWorkspacePresetVersion)
					r.With(requirePerm(authorization.PermPMRead)).Get("/agent-model-providers", h.Agent.ListModelProviders)
					r.With(requirePerm(authorization.PermPMRead)).Get("/tool-catalog", h.Agent.ListToolCatalog)
					r.With(requirePerm(authorization.PermPMRead)).Get("/agents/{id}", h.Agent.GetAgent)
					r.With(requirePerm(authorization.PermPMRead)).Get("/agents/{id}/usage", h.Agent.GetAgentUsage)
					r.With(requirePerm(authorization.PermPMRead)).Get("/agents/{id}/analytics", h.Agent.GetAgentAnalytics)
					r.With(requirePerm(authorization.PermPMRead)).Get("/agents/{id}/versions", h.Agent.ListAgentVersions)
					r.With(requirePerm(authorization.PermPMEdit)).Post("/agents/{id}/versions", h.Agent.CreateAgentVersion)
					r.With(requirePerm(authorization.PermPMEdit)).Put("/agents/{id}/versions/{versionID}", h.Agent.UpdateAgentVersion)
					r.With(requirePerm(authorization.PermPMEdit)).Post("/agents/{id}/versions/{versionID}/activate", h.Agent.ActivateAgentVersion)
					r.With(requirePerm(authorization.PermPMEdit)).Delete("/agents/{id}/versions/{versionID}", h.Agent.DeleteAgentVersion)
					r.With(requirePerm(authorization.PermPMEdit)).Put("/agents/{id}", h.Agent.UpdateAgent)
					r.With(requirePerm(authorization.PermPMEdit)).Delete("/agents/{id}", h.Agent.DeleteAgent)
					r.With(requirePerm(authorization.PermPMRead)).Get("/agents/{id}/runs", h.Agent.ListAgentRuns)
					r.With(requirePerm(authorization.PermPMEdit)).Post("/tasks/{id}/run-agent", h.Agent.RunTaskAgent)
					r.With(requirePerm(authorization.PermPMEdit)).Post("/epics/{id}/run-agent", h.Agent.RunEpicAgent)
					r.With(requirePerm(authorization.PermPMEdit)).Post("/agent-runs", h.Agent.StartTargetRun)
					r.With(requirePerm(authorization.PermPMRead)).Get("/agent-runs/workspace", h.Agent.ListWorkspaceRuns)
					r.With(requirePerm(authorization.PermPMRead)).Get("/agent-runs/recent", h.Agent.ListRecentRuns)
					r.With(requirePerm(authorization.PermPMRead)).Get("/agent-runs", h.Agent.ListTargetRuns)
					r.With(requirePerm(authorization.PermPMRead)).Get("/agent-runs/{id}", h.Agent.GetAgentRun)
					r.With(requirePerm(authorization.PermPMRead)).Get("/agent-runs/{id}/messages", h.Agent.ListRunMessages)
					r.With(requirePerm(authorization.PermPMEdit)).Post("/agent-runs/{id}/messages", h.Agent.SendRunMessage)
					r.With(requirePerm(authorization.PermPMEdit)).Post("/agent-runs/{id}/codex-auth/device-code/start", h.Agent.StartCodexDeviceCodeAuth)
					r.With(requirePerm(authorization.PermPMEdit)).Post("/agent-runs/{id}/codex-auth/device-code/cancel", h.Agent.CancelCodexDeviceCodeAuth)
					r.With(requirePerm(authorization.PermPMEdit)).Post("/agent-runs/{id}/resume", h.Agent.ResumeRun)
					r.With(requirePerm(authorization.PermPMEdit)).Post("/agent-runs/{id}/continue", h.Agent.ContinueRun)
					r.With(requirePerm(authorization.PermPMRead)).Get("/agent-runs/{id}/artifacts", h.Agent.ListRunArtifacts)
					r.With(requirePerm(authorization.PermPMEdit)).Post("/agent-runs/{id}/cancel", h.Agent.CancelRun)
					r.With(requirePerm(authorization.PermPMEdit)).Post("/agent-runs/{id}/approve", h.Agent.ApproveRun)
					r.With(requirePerm(authorization.PermPMEdit)).Post("/agent-runs/{id}/request-changes", h.Agent.RequestRunChanges)
					r.With(requirePerm(authorization.PermPMEdit)).Post("/agent-runs/{id}/handoff", h.Agent.HandoffRun)
					// Run-scoped streaming endpoints. Same handlers serve the coding-session
					// drawer and the Ask-agents dock — sessionID/runID are interchangeable.
					r.With(requirePerm(authorization.PermPMRead)).Get("/agent-runs/{id}/snapshot", h.Agent.GetCodingSession)
					r.With(requirePerm(authorization.PermPMRead)).Get("/agent-runs/{id}/events", h.Agent.ListCodingSessionEvents)
					r.With(requirePerm(authorization.PermPMEdit)).Post("/agent-runs/{id}/interactions/{interactionId}/resolve", h.Agent.ResolveCodingSessionInteraction)
					r.With(requirePerm(authorization.PermPMRead)).Get("/coding-sessions/{id}", h.Agent.GetCodingSession)
					r.With(requirePerm(authorization.PermPMRead)).Get("/coding-sessions/{id}/events", h.Agent.ListCodingSessionEvents)
					r.With(requirePerm(authorization.PermPMRead)).Get("/coding-sessions/{id}/repo", h.Agent.GetCodingSessionRepo)
					r.With(requirePerm(authorization.PermPMRead)).Get("/coding-sessions/{id}/diff", h.Agent.GetCodingSessionDiff)
					r.With(requirePerm(authorization.PermPMEdit)).Post("/coding-sessions/{id}/interactions/{interactionId}/resolve", h.Agent.ResolveCodingSessionInteraction)
					r.With(requirePerm(authorization.PermPMEdit)).Post("/coding-sessions/{id}/message", h.Agent.SendCodingSessionMessage)
					r.With(requirePerm(authorization.PermPMEdit)).Post("/coding-sessions/{id}/continue", h.Agent.ContinueCodingSession)
					r.With(requirePerm(authorization.PermPMEdit)).Post("/coding-sessions/{id}/resume", h.Agent.ResumeCodingSession)
					r.With(requirePerm(authorization.PermPMEdit)).Post("/coding-sessions/{id}/approve", h.Agent.ApproveCodingSession)
					r.With(requirePerm(authorization.PermPMEdit)).Post("/coding-sessions/{id}/request-changes", h.Agent.RequestCodingSessionChanges)
					r.With(requirePerm(authorization.PermPMEdit)).Post("/coding-sessions/{id}/cancel", h.Agent.CancelCodingSession)
					r.With(requirePerm(authorization.PermPMEdit)).Post("/coding-sessions/{id}/auth/device-code/start", h.Agent.StartCodingSessionDeviceCodeAuth)
					r.With(requirePerm(authorization.PermPMEdit)).Post("/coding-sessions/{id}/auth/device-code/cancel", h.Agent.CancelCodingSessionDeviceCodeAuth)
				})
			})

			// Notifications module
			r.Route("/notifications", func(r chi.Router) {
				r.Use(middleware.RequireWorkspaceID)
				r.Use(wsActive)

				// Inbox
				r.With(requirePerm(authorization.PermNotificationsRead)).Get("/", h.Notification.List)
				r.With(requirePerm(authorization.PermNotificationsRead)).Get("/unread-count", h.Notification.GetUnreadCount)
				r.With(requirePerm(authorization.PermNotificationsManage)).Patch("/{notifId}", h.Notification.Update)
				r.With(requirePerm(authorization.PermNotificationsManage)).Post("/mark-all-read", h.Notification.MarkAllRead)
				r.With(requirePerm(authorization.PermNotificationsManage)).Post("/archive-all-read", h.Notification.ArchiveAllRead)
				r.With(requirePerm(authorization.PermNotificationsManage)).Delete("/{notifId}", h.Notification.Delete)

				// Preferences
				r.With(requirePerm(authorization.PermNotificationsRead)).Get("/preferences", h.Notification.GetPreferences)
				r.With(requirePerm(authorization.PermNotificationsManage)).Put("/preferences", h.Notification.UpdatePreferences)

				// Following
				r.With(requirePerm(authorization.PermNotificationsRead)).Get("/following", h.Notification.ListFollowing)
			})

			// Followers (on PM entities)
			r.Route("/pm/{entityType}/{entityId}/followers", func(r chi.Router) {
				r.Use(middleware.RequireWorkspaceID)
				r.Use(wsActive)

				r.With(requirePerm(authorization.PermPMRead)).Get("/", h.Notification.ListFollowers)
				r.With(requirePerm(authorization.PermPMRead)).Get("/check", h.Notification.IsFollowing)
				r.With(requirePerm(authorization.PermPMRead)).Post("/", h.Notification.Follow)
				r.With(requirePerm(authorization.PermPMRead)).Delete("/", h.Notification.Unfollow)
			})

			// Docs module
			r.Route("/docs", func(r chi.Router) {
				r.Use(middleware.RequireWorkspaceID)
				r.Use(wsActive)
				r.Use(handler.NoStoreOnWrites)

				// Spaces — docs.read / docs.edit / docs.admin
				r.With(requirePerm(authorization.PermDocsRead)).Get("/spaces", h.Docs.ListSpaces)
				r.With(requirePerm(authorization.PermDocsEdit)).Post("/spaces", h.Docs.CreateSpace)
				r.With(requirePerm(authorization.PermDocsRead)).Get("/spaces/{spaceId}", h.Docs.GetSpace)
				r.With(requirePerm(authorization.PermDocsEdit)).Patch("/spaces/{spaceId}", h.Docs.UpdateSpace)
				r.With(requirePerm(authorization.PermDocsRead)).Get("/spaces/{spaceId}/delete-impact", h.Docs.GetSpaceDeleteImpact)
				r.With(requirePerm(authorization.PermDocsAdmin)).Delete("/spaces/{spaceId}", h.Docs.DeleteSpace)
				r.With(requirePerm(authorization.PermDocsAdmin)).Post("/spaces/{spaceId}/restore", h.Docs.RestoreSpace)

				// API references — external-space OpenAPI resources.
				r.With(requirePerm(authorization.PermDocsRead)).Get("/spaces/{spaceId}/api-references", h.Docs.ListAPIReferences)
				r.With(requirePerm(authorization.PermDocsEdit)).Post("/spaces/{spaceId}/api-references", h.Docs.CreateAPIReference)
				r.With(requirePerm(authorization.PermDocsRead)).Get("/api-references/{referenceId}", h.Docs.GetAPIReference)
				r.With(requirePerm(authorization.PermDocsEdit)).Patch("/api-references/{referenceId}", h.Docs.UpdateAPIReference)
				r.With(requirePerm(authorization.PermDocsEdit)).Post("/api-references/{referenceId}/sync", h.Docs.SyncAPIReference)
				r.With(requirePerm(authorization.PermDocsEdit)).Post("/api-references/{referenceId}/publish", h.Docs.PublishAPIReference)
				r.With(requirePerm(authorization.PermDocsEdit)).Post("/api-references/{referenceId}/unpublish", h.Docs.UnpublishAPIReference)
				r.With(requirePerm(authorization.PermDocsEdit)).Delete("/api-references/{referenceId}", h.Docs.DeleteAPIReference)

				// Collections — docs.read / docs.edit
				r.With(requirePerm(authorization.PermDocsRead)).Get("/collections", h.Docs.ListAllCollections)
				r.With(requirePerm(authorization.PermDocsRead)).Get("/spaces/{spaceId}/collections", h.Docs.ListCollections)
				r.With(requirePerm(authorization.PermDocsEdit)).Post("/spaces/{spaceId}/collections", h.Docs.CreateCollection)
				r.With(requirePerm(authorization.PermDocsEdit)).Patch("/collections/{collectionId}", h.Docs.UpdateCollection)
				r.With(requirePerm(authorization.PermDocsRead)).Get("/collections/{collectionId}/delete-impact", h.Docs.GetCollectionDeleteImpact)
				r.With(requirePerm(authorization.PermDocsEdit)).Delete("/collections/{collectionId}", h.Docs.DeleteCollection)
				r.With(requirePerm(authorization.PermDocsEdit)).Post("/collections/{collectionId}/restore", h.Docs.RestoreCollection)

				// Documents — docs.read / docs.edit
				r.With(requirePerm(authorization.PermDocsRead)).Get("/documents", h.Docs.ListDocuments)
				r.With(requirePerm(authorization.PermDocsEdit)).Post("/documents", h.Docs.CreateDocument)
				r.With(requirePerm(authorization.PermDocsRead)).Get("/documents/{docId}", h.Docs.GetDocument)
				r.With(requirePerm(authorization.PermDocsEdit)).Patch("/documents/{docId}", h.Docs.UpdateDocument)
				r.With(requirePerm(authorization.PermDocsEdit)).Delete("/documents/{docId}", h.Docs.DeleteDocument)
				r.With(requirePerm(authorization.PermDocsEdit)).Post("/documents/{docId}/restore", h.Docs.RestoreDocument)
				r.With(requirePerm(authorization.PermDocsEdit)).Post("/documents/{docId}/archive", h.Docs.ArchiveDocument)
				r.With(requirePerm(authorization.PermDocsEdit)).Post("/documents/{docId}/unarchive", h.Docs.UnarchiveDocument)
				r.With(requirePerm(authorization.PermDocsEdit)).Post("/documents/{docId}/move", h.Docs.MoveDocument)
				r.With(requirePerm(authorization.PermDocsEdit)).Post("/documents/{docId}/publish", h.Docs.PublishDocument)
				r.With(requirePerm(authorization.PermDocsEdit)).Post("/documents/{docId}/unpublish", h.Docs.UnpublishDocument)

				// Reorder — docs.edit
				r.With(requirePerm(authorization.PermDocsEdit)).Put("/spaces/reorder", h.Docs.ReorderSpaces)
				r.With(requirePerm(authorization.PermDocsEdit)).Put("/spaces/{spaceId}/collections/reorder", h.Docs.ReorderCollections)
				r.With(requirePerm(authorization.PermDocsEdit)).Put("/spaces/{spaceId}/documents/reorder", h.Docs.ReorderDocuments)
				r.With(requirePerm(authorization.PermDocsEdit)).Put("/spaces/{spaceId}/children/reorder", h.Docs.ReorderChildren)
				r.With(requirePerm(authorization.PermDocsEdit)).Post("/items/move", h.Docs.MoveItem)

				// Content — docs.read / docs.edit
				r.With(requirePerm(authorization.PermDocsRead)).Get("/documents/{docId}/content", h.Docs.GetContent)
				r.With(requirePerm(authorization.PermDocsEdit)).Put("/documents/{docId}/content", h.Docs.SaveContent)
				r.With(requirePerm(authorization.PermDocsEdit)).Put("/documents/{docId}/content/markdown", h.Docs.SaveMarkdownContent)
				r.With(requirePerm(authorization.PermDocsEdit)).Post("/documents/{docId}/images/edit", h.Docs.EditImage)
				r.With(requirePerm(authorization.PermDocsRead)).Get("/documents/{docId}/blocks", h.Docs.ListBlocks)
				r.With(requirePerm(authorization.PermDocsEdit)).Post("/documents/{docId}/blocks", h.Docs.CreateBlock)
				r.With(requirePerm(authorization.PermDocsEdit)).Patch("/documents/{docId}/blocks/{blockId}", h.Docs.PatchBlock)
				r.With(requirePerm(authorization.PermDocsEdit)).Post("/documents/{docId}/blocks/reorder", h.Docs.ReorderBlocks)
				r.With(requirePerm(authorization.PermDocsEdit)).Delete("/documents/{docId}/blocks/{blockId}", h.Docs.DeleteBlock)
				r.With(requirePerm(authorization.PermDocsRead)).Get("/documents/{docId}/blocks/{blockId}/ai-section/candidate", h.Docs.GetAISectionCandidate)
				r.With(requirePerm(authorization.PermDocsEdit)).Post("/documents/{docId}/blocks/{blockId}/ai-section/regenerate", h.Docs.RegenerateAISection)
				r.With(requirePerm(authorization.PermDocsEdit)).Post("/documents/{docId}/blocks/{blockId}/ai-section/approve", h.Docs.ApproveAISection)
				r.With(requirePerm(authorization.PermDocsEdit)).Post("/documents/{docId}/blocks/{blockId}/ai-section/reject", h.Docs.RejectAISection)
				r.With(requirePerm(authorization.PermDocsRead)).Get("/documents/{docId}/change-proposals", h.Docs.ListChangeProposals)
				r.With(requirePerm(authorization.PermDocsRead)).Get("/documents/{docId}/change-proposals/{proposalId}", h.Docs.GetChangeProposal)
				r.With(requirePerm(authorization.PermDocsEdit)).Post("/documents/{docId}/change-proposals/{proposalId}/apply", h.Docs.ApplyChangeProposal)
				r.With(requirePerm(authorization.PermDocsEdit)).Post("/documents/{docId}/change-proposals/{proposalId}/discard", h.Docs.DiscardChangeProposal)

				// Comments — docs.read / docs.edit
				r.With(requirePerm(authorization.PermDocsRead)).Get("/documents/{docId}/comments", h.Docs.ListComments)
				r.With(requirePerm(authorization.PermDocsEdit)).Post("/documents/{docId}/comments", h.Docs.CreateComment)
				r.With(requirePerm(authorization.PermDocsEdit)).Put("/comments/{id}", h.Docs.UpdateComment)
				r.With(requirePerm(authorization.PermDocsEdit)).Delete("/comments/{id}", h.Docs.DeleteComment)
				r.With(requirePerm(authorization.PermDocsEdit)).Post("/comments/{id}/resolve", h.Docs.ResolveComment)
				r.With(requirePerm(authorization.PermDocsEdit)).Post("/comments/{id}/reopen", h.Docs.ReopenComment)
				r.With(requirePerm(authorization.PermDocsEdit)).Post("/comments/{id}/reactions", h.Docs.ToggleCommentReaction)
				r.With(requirePerm(authorization.PermDocsRead)).Get("/documents/{docId}/references", h.Docs.ListReferences)

				// Preview token — docs.read
				r.With(requirePerm(authorization.PermDocsRead)).Post("/documents/{docId}/preview-token", h.Docs.GeneratePreviewToken)

				// Versions — docs.read / docs.edit
				r.With(requirePerm(authorization.PermDocsRead)).Get("/documents/{docId}/versions", h.Docs.ListVersions)
				r.With(requirePerm(authorization.PermDocsEdit)).Post("/documents/{docId}/versions", h.Docs.CreateVersion)
				r.With(requirePerm(authorization.PermDocsEdit)).Post("/documents/{docId}/revert/{versionId}", h.Docs.RevertVersion)
				r.With(requirePerm(authorization.PermDocsRead)).Get("/documents/{docId}/versions/{versionId}", h.Docs.GetVersion)
				r.With(requirePerm(authorization.PermDocsEdit)).Patch("/documents/{docId}/versions/{versionId}", h.Docs.UpdateVersionLabel)

				// Links — docs.read / docs.edit
				r.With(requirePerm(authorization.PermDocsRead)).Get("/documents/{docId}/links", h.Docs.ListLinks)
				r.With(requirePerm(authorization.PermDocsEdit)).Post("/documents/{docId}/links", h.Docs.CreateLink)
				r.With(requirePerm(authorization.PermDocsEdit)).Delete("/links/{linkId}", h.Docs.DeleteLink)
				r.With(requirePerm(authorization.PermDocsRead)).Get("/linked-docs/{objectType}/{objectId}", h.Docs.ListLinkedDocs)

				// Share toggle — docs.edit
				r.With(requirePerm(authorization.PermDocsEdit)).Post("/documents/{docId}/toggle-share", h.Docs.ToggleDocShare)
				r.With(requirePerm(authorization.PermDocsEdit)).Post("/documents/{docId}/toggle-lock", h.Docs.ToggleDocLock)

				// External publish/unpublish — docs.publish
				r.With(requirePerm(authorization.PermDocsPublish)).Post("/documents/{docId}/publish-external", h.Docs.PublishExternally)
				r.With(requirePerm(authorization.PermDocsPublish)).Post("/documents/{docId}/unpublish-external", h.Docs.UnpublishExternally)
				r.With(requirePerm(authorization.PermDocsEdit)).Post("/documents/{docId}/update-slug", h.Docs.UpdateArticleSlug)
				r.With(requirePerm(authorization.PermDocsEdit)).Put("/documents/{docId}/helpcenter/metadata", h.Docs.UpdateHelpcenterArticleMetadata)

				// Search
				r.With(requirePerm(authorization.PermDocsRead)).Get("/search", h.Docs.Search)
				r.With(requirePerm(authorization.PermDocsRead)).Get("/embeds/resolve", h.Docs.ResolveEmbed)
				r.With(requirePerm(authorization.PermDocsRead)).Post("/entity-refs/resolve", h.Docs.ResolveEntityRefs)

				// Help Center Config — docs.admin
				r.With(requirePerm(authorization.PermDocsRead)).Get("/helpcenter/config", h.Docs.GetHelpcenterConfig)
				r.With(requirePerm(authorization.PermDocsAdmin)).Put("/helpcenter/config", h.Docs.UpdateHelpcenterConfig)
				r.With(requirePerm(authorization.PermDocsRead)).Get("/helpcenter/locales", h.Docs.GetHelpcenterLocales)
				r.With(requirePerm(authorization.PermDocsAdmin)).Put("/helpcenter/locales", h.Docs.UpdateHelpcenterLocales)
				r.With(requirePerm(authorization.PermDocsAdmin)).Post("/helpcenter/upload", h.Docs.UploadHelpcenterAsset)
				r.With(requirePerm(authorization.PermDocsEdit)).Post("/images/import", h.Docs.ImportExternalImage)
				r.With(requirePerm(authorization.PermDocsRead)).Get("/spaces/{spaceId}/helpcenter/translations", h.Docs.ListSpaceTranslations)
				r.With(requirePerm(authorization.PermDocsAdmin)).Put("/spaces/{spaceId}/helpcenter/translations", h.Docs.UpsertSpaceTranslation)
				r.With(requirePerm(authorization.PermDocsAdmin)).Post("/spaces/{spaceId}/helpcenter/translations/{locale}/publish", h.Docs.PublishSpaceTranslation)
				r.With(requirePerm(authorization.PermDocsAdmin)).Post("/spaces/{spaceId}/helpcenter/translations/{locale}/unpublish", h.Docs.UnpublishSpaceTranslation)
				r.With(requirePerm(authorization.PermDocsAdmin)).Post("/spaces/{spaceId}/helpcenter/translations/{locale}/mark-reviewed", h.Docs.MarkSpaceTranslationReviewed)
				r.With(requirePerm(authorization.PermDocsAdmin)).Post("/spaces/{spaceId}/helpcenter/translations/{locale}/generate", h.Docs.GenerateSpaceTranslation)
				r.With(requirePerm(authorization.PermDocsRead)).Get("/collections/{collectionId}/helpcenter/translations", h.Docs.ListCollectionTranslations)
				r.With(requirePerm(authorization.PermDocsAdmin)).Put("/collections/{collectionId}/helpcenter/translations", h.Docs.UpsertCollectionTranslation)
				r.With(requirePerm(authorization.PermDocsAdmin)).Post("/collections/{collectionId}/helpcenter/translations/{locale}/publish", h.Docs.PublishCollectionTranslation)
				r.With(requirePerm(authorization.PermDocsAdmin)).Post("/collections/{collectionId}/helpcenter/translations/{locale}/unpublish", h.Docs.UnpublishCollectionTranslation)
				r.With(requirePerm(authorization.PermDocsAdmin)).Post("/collections/{collectionId}/helpcenter/translations/{locale}/mark-reviewed", h.Docs.MarkCollectionTranslationReviewed)
				r.With(requirePerm(authorization.PermDocsAdmin)).Post("/collections/{collectionId}/helpcenter/translations/{locale}/generate", h.Docs.GenerateCollectionTranslation)
				r.With(requirePerm(authorization.PermDocsAdmin)).Post("/helpcenter/translations/auto-translate-missing", h.Docs.AutoTranslateMissing)
				r.With(requirePerm(authorization.PermDocsRead)).Get("/documents/{docId}/helpcenter/translations", h.Docs.ListArticleTranslations)
				r.With(requirePerm(authorization.PermDocsAdmin)).Put("/documents/{docId}/helpcenter/translations", h.Docs.UpsertArticleTranslation)
				r.With(requirePerm(authorization.PermDocsAdmin)).Post("/documents/{docId}/helpcenter/translations/{locale}/generate", h.Docs.GenerateArticleTranslationDraft)
				r.With(requirePerm(authorization.PermDocsAdmin)).Post("/documents/{docId}/helpcenter/translations/{locale}/publish", h.Docs.PublishArticleTranslation)
				r.With(requirePerm(authorization.PermDocsAdmin)).Post("/documents/{docId}/helpcenter/translations/{locale}/update-slug", h.Docs.UpdateArticleTranslationSlug)
				r.With(requirePerm(authorization.PermDocsAdmin)).Post("/documents/{docId}/helpcenter/translations/{locale}/unpublish", h.Docs.UnpublishArticleTranslation)
				r.With(requirePerm(authorization.PermDocsAdmin)).Post("/documents/{docId}/helpcenter/translations/{locale}/mark-reviewed", h.Docs.MarkArticleTranslationReviewed)

				// Redirect management
				r.With(requirePerm(authorization.PermDocsAdmin)).Get("/redirects", h.Docs.ListRedirects)
				r.With(requirePerm(authorization.PermDocsAdmin)).Post("/redirects", h.Docs.CreateRedirect)
				r.With(requirePerm(authorization.PermDocsAdmin)).Patch("/redirects/{id}", h.Docs.UpdateRedirect)
				r.With(requirePerm(authorization.PermDocsAdmin)).Delete("/redirects/{id}", h.Docs.DeleteRedirect)

				// Feedback
				r.With(requirePerm(authorization.PermDocsEdit)).Post("/articles/{docId}/feedback", h.Docs.SubmitArticleFeedback)

				// Docs import
				r.With(requirePerm(authorization.PermDocsImport)).Get("/import/jobs", h.Docs.ImportListJobs)
				r.With(requirePerm(authorization.PermDocsImport)).Post("/import/helpscout/preview", h.Docs.ImportPreviewHelpscout)
				r.With(requirePerm(authorization.PermDocsImport)).Post("/import/helpscout/start", h.Docs.ImportStartHelpscout)
				r.With(requirePerm(authorization.PermDocsImport)).Post("/import/nextra/preview", h.Docs.ImportPreviewNextra)
				r.With(requirePerm(authorization.PermDocsImport)).Post("/import/nextra/start", h.Docs.ImportStartNextra)
				r.With(requirePerm(authorization.PermDocsImport)).Get("/import/{jobId}/status", h.Docs.ImportGetStatus)
				r.With(requirePerm(authorization.PermDocsImport)).Post("/import/{jobId}/retry", h.Docs.ImportRetry)
				r.With(requirePerm(authorization.PermDocsImport)).Post("/import/{jobId}/cancel", h.Docs.ImportCancel)
				r.With(requirePerm(authorization.PermDocsImport)).Get("/import/{jobId}/redirect-map", h.Docs.ImportGetRedirectMap)
				r.With(requirePerm(authorization.PermDocsImport)).Post("/import/{jobId}/reconvert", h.Docs.ImportReconvert)
			})

			// CRM module
			r.Route("/crm", func(r chi.Router) {
				r.Use(middleware.RequireWorkspaceID)
				r.Use(wsActive)
				r.Use(requireModule(model.ModuleCRM))
				if h.SupportAI != nil {
					r.With(requirePerm(authorization.PermCRMEdit)).Post("/email/rewrite-draft", h.SupportAI.RewriteCRMEmailDraft)
				}

				// Contacts — crm.read / crm.edit
				r.With(requirePerm(authorization.PermCRMRead)).Get("/contacts", h.CRMContact.List)
				r.With(requirePerm(authorization.PermCRMEdit)).Post("/contacts", h.CRMContact.Create)
				r.With(requirePerm(authorization.PermCRMEdit)).Post("/contacts/seed", h.CRMContact.Seed)
				r.With(requirePerm(authorization.PermCRMRead)).Get("/contacts/{id}", h.CRMContact.Get)
				r.With(requirePerm(authorization.PermCRMEdit)).Put("/contacts/{id}", h.CRMContact.Update)
				r.With(requirePerm(authorization.PermCRMEdit)).Delete("/contacts/{id}", h.CRMContact.Delete)
				r.With(requirePerm(authorization.PermCRMRead)).Get("/contacts/{id}/activities", h.CRMActivity.ListByContact)
				r.With(requirePerm(authorization.PermCRMRead)).Get("/contacts/{id}/timeline", h.CRMContact.ListTimeline)
				r.With(requirePerm(authorization.PermCRMRead)).Get("/contacts/{id}/associations", h.CRMAssociation.ListContactAssociations)
				r.With(requirePerm(authorization.PermCRMRead)).Get("/contacts/{id}/support-conversations", h.SupportInbox.ListContactConversations)

				// Companies — crm.read / crm.edit
				r.With(requirePerm(authorization.PermCRMRead)).Get("/companies", h.CRMCompany.List)
				r.With(requirePerm(authorization.PermCRMEdit)).Post("/companies", h.CRMCompany.Create)
				r.With(requirePerm(authorization.PermCRMRead)).Get("/companies/{id}", h.CRMCompany.Get)
				r.With(requirePerm(authorization.PermCRMEdit)).Put("/companies/{id}", h.CRMCompany.Update)
				r.With(requirePerm(authorization.PermCRMEdit)).Delete("/companies/{id}", h.CRMCompany.Delete)
				r.With(requirePerm(authorization.PermCRMRead)).Get("/companies/{id}/activities", h.CRMActivity.ListByCompany)
				r.With(requirePerm(authorization.PermCRMRead)).Get("/companies/{id}/timeline", h.CRMCompany.ListTimeline)
				r.With(requirePerm(authorization.PermCRMRead)).Get("/companies/{id}/associations", h.CRMAssociation.ListCompanyAssociations)
				r.With(requirePerm(authorization.PermCRMRead)).Get("/companies/{id}/contacts", h.CRMCompany.ListContacts)
				r.With(requirePerm(authorization.PermCRMRead)).Get("/companies/{id}/deals", h.CRMCompany.ListDeals)
				r.With(requirePerm(authorization.PermCRMRead)).Get("/companies/{id}/support-conversations", h.SupportInbox.ListCompanyConversations)

				// Deals — crm.read / crm.edit
				r.With(requirePerm(authorization.PermCRMRead)).Get("/deals", h.CRMDeal.List)
				r.With(requirePerm(authorization.PermCRMEdit)).Post("/deals", h.CRMDeal.Create)
				r.With(requirePerm(authorization.PermCRMRead)).Get("/deals/{id}", h.CRMDeal.Get)
				r.With(requirePerm(authorization.PermCRMEdit)).Put("/deals/{id}", h.CRMDeal.Update)
				r.With(requirePerm(authorization.PermCRMEdit)).Put("/deals/{id}/customer", h.CRMDeal.SetCustomer)
				r.With(requirePerm(authorization.PermCRMEdit)).Delete("/deals/{id}", h.CRMDeal.Delete)
				r.With(requirePerm(authorization.PermCRMRead)).Get("/deals/{id}/activities", h.CRMActivity.ListByDeal)
				r.With(requirePerm(authorization.PermCRMRead)).Get("/deals/{id}/timeline", h.CRMDeal.ListTimeline)
				r.With(requirePerm(authorization.PermCRMRead)).Get("/deals/{id}/associations", h.CRMAssociation.ListDealAssociations)

				// Pipelines — crm.read / crm.admin
				r.With(requirePerm(authorization.PermCRMRead)).Get("/pipelines", h.CRMDeal.ListPipelines)
				r.With(requirePerm(authorization.PermCRMAdmin)).Post("/pipelines", h.CRMDeal.CreatePipeline)
				r.With(requirePerm(authorization.PermCRMRead)).Get("/pipelines/{id}", h.CRMDeal.GetPipeline)
				r.With(requirePerm(authorization.PermCRMAdmin)).Put("/pipelines/{id}", h.CRMDeal.UpdatePipeline)
				r.With(requirePerm(authorization.PermCRMAdmin)).Delete("/pipelines/{id}", h.CRMDeal.DeletePipeline)

				// Associations — crm.read / crm.edit
				r.With(requirePerm(authorization.PermCRMEdit)).Post("/associations", h.CRMAssociation.Create)
				r.With(requirePerm(authorization.PermCRMEdit)).Delete("/associations/{id}", h.CRMAssociation.Delete)

				// Activities — crm.read / crm.edit
				r.With(requirePerm(authorization.PermCRMRead)).Get("/activities", h.CRMActivity.List)
				r.With(requirePerm(authorization.PermCRMEdit)).Post("/activities", h.CRMActivity.Create)
				r.With(requirePerm(authorization.PermCRMRead)).Get("/activities/{id}", h.CRMActivity.Get)
				r.With(requirePerm(authorization.PermCRMEdit)).Put("/activities/{id}", h.CRMActivity.Update)
				r.With(requirePerm(authorization.PermCRMEdit)).Delete("/activities/{id}", h.CRMActivity.Delete)

				// Meetings — crm.read / crm.edit / crm.admin settings
				r.With(requirePerm(authorization.PermCRMRead)).Get("/meetings", h.CRMMeeting.List)
				r.With(requirePerm(authorization.PermCRMEdit)).Post("/meetings", h.CRMMeeting.Create)
				r.With(requirePerm(authorization.PermCRMRead)).Get("/meetings/calendar-upcoming", h.CRMMeeting.ListUpcomingCalendar)
				r.With(requirePerm(authorization.PermCRMEdit)).Put("/meetings/calendar-series/capture", h.CRMMeeting.UpdateCalendarSeriesCapture)
				r.With(requirePerm(authorization.PermCRMEdit)).Put("/meetings/calendar/{eventID}/capture", h.CRMMeeting.UpdateCalendarCapture)
				r.With(requirePerm(authorization.PermCRMRead)).Get("/meetings/{id}", h.CRMMeeting.Get)
				r.With(requirePerm(authorization.PermCRMEdit)).Put("/meetings/{id}", h.CRMMeeting.Update)
				r.With(requirePerm(authorization.PermCRMEdit)).Delete("/meetings/{id}", h.CRMMeeting.Delete)
				r.With(requirePerm(authorization.PermCRMEdit)).Post("/meetings/{id}/capture", h.CRMMeeting.StartCapture)
				r.With(requirePerm(authorization.PermCRMEdit)).Post("/meetings/{id}/capture/stop", h.CRMMeeting.StopCapture)
				r.With(requirePerm(authorization.PermCRMEdit)).Post("/meetings/{id}/process", h.CRMMeeting.RetryProcessing)
				r.With(requirePerm(authorization.PermCRMRead)).Get("/meetings/{id}/recording", h.CRMMeeting.GetRecording)
				r.With(requirePerm(authorization.PermCRMEdit)).Delete("/meetings/{id}/recording", h.CRMMeeting.DeleteRecording)
				r.With(requirePerm(authorization.PermCRMEdit), requirePerm(authorization.PermPMEdit)).Post("/meetings/{id}/action-items/{itemID}/accept", h.CRMMeeting.AcceptActionItem)
				r.With(requirePerm(authorization.PermCRMEdit)).Post("/meetings/{id}/action-items/{itemID}/dismiss", h.CRMMeeting.DismissActionItem)
				r.With(requirePerm(authorization.PermCRMRead)).Get("/meeting-settings", h.CRMMeeting.GetSettings)
				r.With(requirePerm(authorization.PermCRMAdmin)).Put("/meeting-settings", h.CRMMeeting.UpdateSettings)

				// Imports — crm.edit
				r.With(requirePerm(authorization.PermCRMRead)).Get("/imports", h.CRMImport.List)
				r.With(requirePerm(authorization.PermCRMEdit)).Post("/imports", h.CRMImport.Create)
				r.With(requirePerm(authorization.PermCRMRead)).Get("/imports/{id}", h.CRMImport.Get)
				r.With(requirePerm(authorization.PermCRMEdit)).Post("/imports/{id}/process", h.CRMImport.Process)

				// Email OAuth — crm.edit (callback is public, registered above)
				r.With(requirePerm(authorization.PermCRMEdit)).Get("/email/oauth/initiate", h.CRMEmail.InitiateOAuth)

				// Email — crm.read / crm.edit
				r.With(requirePerm(authorization.PermCRMRead)).Get("/email/accounts", h.CRMEmail.ListAccounts)
				r.With(requirePerm(authorization.PermCRMEdit)).Post("/email/accounts", h.CRMEmail.CreateAccount)
				r.With(requirePerm(authorization.PermCRMRead)).Get("/email/accounts/{id}", h.CRMEmail.GetAccount)
				r.With(requirePerm(authorization.PermCRMEdit)).Delete("/email/accounts/{id}", h.CRMEmail.DeleteAccount)
				r.With(requirePerm(authorization.PermCRMRead)).Get("/email/accounts/{id}/diagnostics", h.CRMEmail.GetAccountDiagnostics)
				r.With(requirePerm(authorization.PermCRMEdit)).Post("/email/accounts/{id}/sync", h.CRMEmail.SyncAccount)
				r.With(requirePerm(authorization.PermCRMAdmin)).Post("/email/accounts/{id}/maintenance/rebuild-associations", h.CRMEmail.RebuildAssociations)
				r.With(requirePerm(authorization.PermCRMAdmin)).Delete("/email/accounts/{id}/data", h.CRMEmail.PurgeAccountData)
				r.With(requirePerm(authorization.PermCRMEdit)).Post("/email/accounts/{id}/oauth-callback", h.CRMEmail.OAuthCallback)
				r.With(requirePerm(authorization.PermCRMRead)).Get("/email/threads", h.CRMEmail.ListThreads)
				r.With(requirePerm(authorization.PermCRMRead)).Get("/email/threads/{id}", h.CRMEmail.GetThread)
				r.With(requirePerm(authorization.PermCRMEdit)).Post("/email/threads/{id}/reply", h.CRMEmail.ReplyToThread)
				r.With(requirePerm(authorization.PermCRMEdit)).Put("/email/threads/{id}/needs-reply-dismissal", h.CRMEmail.SetThreadDismissal)
				r.With(requirePerm(authorization.PermCRMEdit)).Delete("/email/threads/{id}/needs-reply-dismissal", h.CRMEmail.SetThreadDismissal)
				r.With(requirePerm(authorization.PermCRMEdit)).Patch("/email/threads/{id}/deal", h.CRMEmail.LinkThreadDeal)
				r.With(requirePerm(authorization.PermCRMRead)).Get("/email/messages", h.CRMEmail.ListMessages)
				r.With(requirePerm(authorization.PermCRMEdit)).Post("/email/messages", h.CRMEmail.CreateMessage)
				r.With(requirePerm(authorization.PermCRMEdit)).Post("/email/send", h.CRMEmail.SendEmail)
				r.With(requirePerm(authorization.PermCRMEdit)).Post("/email/attachments", h.CRMEmail.CreateAttachment)
				r.With(requirePerm(authorization.PermCRMEdit)).Patch("/email/attachments/{attachmentId}/confirm", h.CRMEmail.ConfirmAttachment)
				r.With(requirePerm(authorization.PermCRMEdit)).Delete("/email/attachments/{attachmentId}", h.CRMEmail.DeleteAttachment)
				r.With(requirePerm(authorization.PermCRMRead)).Get("/email/attachments/{attachmentId}/download", h.CRMEmail.DownloadAttachment)
				r.With(requirePerm(authorization.PermCRMRead)).Get("/contacts/{id}/emails", h.CRMEmail.ListByContact)
				r.With(requirePerm(authorization.PermCRMRead)).Get("/companies/{id}/emails", h.CRMEmail.ListByCompany)
				r.With(requirePerm(authorization.PermCRMRead)).Get("/deals/{id}/emails", h.CRMEmail.ListByDeal)

				// Email Sync Settings — crm.admin
				r.With(requirePerm(authorization.PermCRMAdmin)).Get("/email/sync-settings", h.CRMEmail.GetEmailSyncSettings)
				r.With(requirePerm(authorization.PermCRMAdmin)).Put("/email/sync-settings", h.CRMEmail.UpdateEmailSyncSettings)
				r.With(requirePerm(authorization.PermCRMAdmin)).Get("/email/sync-settings/default-prefixes", h.CRMEmail.GetDefaultBlockedPrefixes)

				// Calendar — crm.read / crm.edit
				r.With(requirePerm(authorization.PermCRMRead)).Get("/calendar/events", h.CRMCalendar.List)
				r.With(requirePerm(authorization.PermCRMEdit)).Post("/calendar/events", h.CRMCalendar.Create)
				r.With(requirePerm(authorization.PermCRMRead)).Get("/calendar/events/{id}", h.CRMCalendar.Get)
				r.With(requirePerm(authorization.PermCRMEdit)).Put("/calendar/events/{id}", h.CRMCalendar.Update)
				r.With(requirePerm(authorization.PermCRMEdit)).Delete("/calendar/events/{id}", h.CRMCalendar.Delete)
				r.With(requirePerm(authorization.PermCRMRead)).Get("/contacts/{id}/calendar", h.CRMCalendar.ListByContact)
				r.With(requirePerm(authorization.PermCRMRead)).Get("/deals/{id}/calendar", h.CRMCalendar.ListByDeal)

				// Enrichments — crm.read / crm.edit
				r.With(requirePerm(authorization.PermCRMRead)).Get("/enrichments", h.CRMEnrichment.List)
				r.With(requirePerm(authorization.PermCRMEdit)).Post("/enrichments", h.CRMEnrichment.Create)
				r.With(requirePerm(authorization.PermCRMEdit)).Post("/enrichments/{id}/apply-suggestion", h.CRMEnrichment.ApplySuggestion)

				// Signals — crm.read / crm.edit
				r.With(requirePerm(authorization.PermCRMRead)).Get("/signals/feed", h.CRMSignal.ListWorkspaceFeed)
				r.With(requirePerm(authorization.PermCRMAdmin)).Get("/signals/shadow-preview", h.CRMSignal.ListWorkspaceShadowPreview)
				r.With(requirePerm(authorization.PermCRMAdmin)).Get("/signals/shadow-gate", h.CRMSignal.ShadowGate)
				r.With(requirePerm(authorization.PermCRMRead)).Get("/signals/rollout", h.CRMSignal.GetRolloutSettings)
				r.With(requirePerm(authorization.PermCRMAdmin)).Post("/signals/rollout/activate", h.CRMSignal.ActivateRollout)
				r.With(requirePerm(authorization.PermCRMRead)).Get("/signals/brief", h.CRMSignal.SignalBrief)
				r.With(requirePerm(authorization.PermCRMRead)).Get("/meetings/{id}/signal-brief", h.CRMSignal.MeetingSignalBrief)
				r.With(requirePerm(authorization.PermCRMRead)).Get("/signals/precision", h.CRMSignal.PrecisionReport)
				r.With(requirePerm(authorization.PermCRMAdmin)).Get("/signals/outcomes", h.CRMSignal.OutcomeCalibrationReport)
				r.With(requirePerm(authorization.PermCRMAdmin)).Get("/signals/rules", h.CRMSignal.ListRuleConfigs)
				r.With(requirePerm(authorization.PermCRMRead)).Get("/signals/routing-policy", h.CRMSignal.GetRoutingPolicy)
				r.With(requirePerm(authorization.PermCRMRead)).Get("/signals/routing-settings", h.CRMSignal.GetRoutingSettings)
				r.With(requirePerm(authorization.PermCRMAdmin)).Put("/signals/routing-settings", h.CRMSignal.UpdateRoutingSettings)
				r.With(requirePerm(authorization.PermCRMAdmin)).Post("/signals/routing-policy", h.CRMSignal.CreateRoutingPolicy)
				r.With(requirePerm(authorization.PermCRMAdmin)).Post("/signals/routing-policy/versions/{version}/activate", h.CRMSignal.ActivateRoutingPolicy)
				r.With(requirePerm(authorization.PermCRMAdmin)).Post("/signals/rules/{ruleKey}/versions/{version}/activate", h.CRMSignal.ActivateRuleVersion)
				r.With(requirePerm(authorization.PermCRMRead)).Get("/signals", h.CRMSignal.ListSignals)
				r.With(requirePerm(authorization.PermCRMEdit)).Post("/signals", h.CRMSignal.CreateSignal)
				r.With(requirePerm(authorization.PermCRMAdmin)).Post("/signals/external-evidence", h.CRMSignal.IngestExternalEvidence)
				r.With(requirePerm(authorization.PermCRMEdit)).Delete("/signals/{id}", h.CRMSignal.DeleteSignal)
				r.With(requirePerm(authorization.PermCRMEdit)).Post("/signals/{id}/dismiss", h.CRMSignal.DismissSignal)
				r.With(requirePerm(authorization.PermCRMEdit)).Post("/signals/{id}/review", h.CRMSignal.ReviewSignal)
				r.With(requirePerm(authorization.PermCRMEdit)).Post("/signals/{id}/acted", h.CRMSignal.ActOnSignal)
				r.With(requirePerm(authorization.PermCRMRead)).Get("/contacts/{id}/signals", h.CRMSignal.ListByContact)
				r.With(requirePerm(authorization.PermCRMRead)).Get("/deals/{id}/signals", h.CRMSignal.ListByDeal)
				r.With(requirePerm(authorization.PermCRMRead)).Get("/companies/{id}/signals", h.CRMSignal.ListByCompany)
				r.With(requirePerm(authorization.PermCRMRead)).Get("/contacts/{id}/summary", h.CRMSummary.GetContactSummary)
				r.With(requirePerm(authorization.PermCRMRead)).Get("/deals/{id}/summary", h.CRMSummary.GetDealSummary)
				r.With(requirePerm(authorization.PermCRMRead)).Get("/companies/{id}/summary", h.CRMSummary.GetCompanySummary)
				r.With(requirePerm(authorization.PermCRMRead)).Post("/contacts/{id}/summary/refresh", h.CRMSummary.RefreshContactSummary)
				r.With(requirePerm(authorization.PermCRMRead)).Post("/deals/{id}/summary/refresh", h.CRMSummary.RefreshDealSummary)
				r.With(requirePerm(authorization.PermCRMRead)).Post("/companies/{id}/summary/refresh", h.CRMSummary.RefreshCompanySummary)
				r.With(requirePerm(authorization.PermCRMRead)).Post("/contacts/{id}/intelligence/refresh", h.CRMSummary.RefreshContactIntelligence)
				r.With(requirePerm(authorization.PermCRMRead)).Post("/deals/{id}/intelligence/refresh", h.CRMSummary.RefreshDealIntelligence)
				r.With(requirePerm(authorization.PermCRMRead)).Post("/companies/{id}/intelligence/refresh", h.CRMSummary.RefreshCompanyIntelligence)

				// Health Scores — crm.read / crm.edit
				r.With(requirePerm(authorization.PermCRMRead)).Get("/health-scores", h.CRMSignal.ListHealthScores)
				r.With(requirePerm(authorization.PermCRMEdit)).Post("/health-scores", h.CRMSignal.CreateHealthScore)
				r.With(requirePerm(authorization.PermCRMRead)).Get("/deals/{id}/health-score", h.CRMSignal.GetDealHealthScore)

				// Suggestions — crm.read / crm.edit
				r.With(requirePerm(authorization.PermCRMRead)).Get("/suggestions", h.CRMSuggestion.List)
				r.With(requirePerm(authorization.PermCRMRead)).Get("/suggestions/{id}", h.CRMSuggestion.Get)
				r.With(requirePerm(authorization.PermCRMEdit)).Post("/suggestions", h.CRMSuggestion.Create)
				r.With(requirePerm(authorization.PermCRMEdit)).Put("/suggestions/{id}", h.CRMSuggestion.Update)
				r.With(requirePerm(authorization.PermCRMEdit)).Delete("/suggestions/{id}", h.CRMSuggestion.Delete)
				r.With(requirePerm(authorization.PermCRMEdit)).Post("/suggestions/{id}/accept", h.CRMSuggestion.Accept)
				r.With(requirePerm(authorization.PermCRMEdit)).Post("/suggestions/{id}/dismiss", h.CRMSuggestion.Dismiss)

				// Autonomy Settings — crm.admin
				r.With(requirePerm(authorization.PermCRMAdmin)).Get("/autonomy-settings", h.CRMDealAutomation.GetAutonomySettings)
				r.With(requirePerm(authorization.PermCRMAdmin)).Put("/autonomy-settings", h.CRMDealAutomation.UpdateAutonomySettings)

				// Writing Profiles — crm.read / crm.edit
				r.With(requirePerm(authorization.PermCRMRead)).Get("/writing-profiles", h.CRMWritingProfile.List)
				r.With(requirePerm(authorization.PermCRMEdit)).Post("/writing-profiles", h.CRMWritingProfile.Create)
				r.With(requirePerm(authorization.PermCRMRead)).Get("/writing-profiles/member/{memberId}", h.CRMWritingProfile.GetByMember)
				r.With(requirePerm(authorization.PermCRMEdit)).Put("/writing-profiles/{id}", h.CRMWritingProfile.Update)
				r.With(requirePerm(authorization.PermCRMEdit)).Delete("/writing-profiles/{id}", h.CRMWritingProfile.Delete)

				// Search — crm.read
				r.With(requirePerm(authorization.PermCRMRead)).Get("/search", h.CRMSearch.Search)
			})
		})
	})

	return r
}
