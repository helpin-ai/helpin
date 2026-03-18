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
)

// Handlers aggregates all HTTP handlers.
type Handlers struct {
	Health             *handler.HealthHandler
	Auth               *handler.AuthHandler
	Organization       *handler.OrganizationHandler
	Workspace          *handler.WorkspaceHandler
	Settings           *handler.SettingsHandler
	Invite             *handler.InviteHandler
	PMWorkflow         *handler.PMWorkflowHandler
	PMImport           *handler.PMImportHandler
	PMLabel            *handler.PMLabelHandler
	PMEpic             *handler.PMEpicHandler
	PMSprint           *handler.PMSprintHandler
	PMStory            *handler.PMStoryHandler
	PMComment          *handler.PMCommentHandler
	PMAttachment       *handler.PMAttachmentHandler
	PMObjective        *handler.PMObjectiveHandler
	PMChecklistItem    *handler.PMChecklistItemHandler
	PMExternalLink     *handler.PMExternalLinkHandler
	PMView             *handler.PMViewHandler
	PMAutomation       *handler.PMAutomationHandler
	PMStoryTemplate    *handler.PMStoryTemplateHandler
	Search             *handler.SearchHandler
	Agent              *handler.AgentHandler
	SupportInbox       *handler.SupportInboxHandler
	SupportInboxWidget *handler.SupportInboxWidgetHandler
	Git                *handler.GitHandler
	Orchestration      *handler.OrchestrationHandler
	Docs               *handler.DocsHandler
	Notification       *handler.NotificationHandler
	UserNotifSettings  *handler.UserNotificationSettingsHandler
	CRMContact         *handler.CRMContactHandler
	CRMCompany         *handler.CRMCompanyHandler
	CRMDeal            *handler.CRMDealHandler
	CRMAssociation     *handler.CRMAssociationHandler
	Associations       *handler.AssociationsHandler
	CRMActivity        *handler.CRMActivityHandler
	CRMProperty        *handler.CRMPropertyHandler
	CRMList            *handler.CRMListHandler
	CRMImport          *handler.CRMImportHandler
	CRMEmail           *handler.CRMEmailHandler
	CRMCalendar        *handler.CRMCalendarHandler
	CRMEnrichment      *handler.CRMEnrichmentHandler
	CRMSignal          *handler.CRMSignalHandler
	CRMSummary         *handler.CRMSummaryHandler
	CRMSuggestion      *handler.CRMSuggestionHandler
	CRMSequence        *handler.CRMSequenceHandler
	CRMWritingProfile  *handler.CRMWritingProfileHandler
	CRMSearch          *handler.CRMSearchHandler
	CRMDealAutomation  *handler.CRMDealAutomationHandler
	PlanningSession    *handler.PlanningSessionHandler
	AutomationRule     *handler.AutomationRuleHandler
	PMRoadmap          *handler.PMRoadmapHandler
	Flow               *handler.FlowHandler
	SDKAssets          *handler.SDKAssetsHandler
}

// New creates and configures the Chi router with all routes.
func New(h Handlers, jwtManager *auth.JWTManager, authz *authorization.AuthzService, slugResolver authorization.SlugResolver, corsOrigins []string) *chi.Mux {
	r := chi.NewRouter()

	// Global middleware
	r.Use(chimiddleware.RequestID)
	r.Use(chimiddleware.RealIP)
	r.Use(middleware.RequestLogger)
	r.Use(chimiddleware.Recoverer)
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   corsOrigins,
		AllowedMethods:   []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-Workspace-ID"},
		ExposedHeaders:   []string{"Link", "Deprecation", "Sunset"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	// Permission middleware helpers for readability.
	requirePerm := func(perm authorization.Permission) func(http.Handler) http.Handler {
		return authorization.RequirePermission(authz, perm)
	}
	wsAccess := authorization.RequireWorkspaceAccess(authz)

	// Root endpoint — responds on bare domain requests.
	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"name":"Helpin API","status":"running"}`))
	})
	r.Get("/health", h.Health.Check)

	// ---- Public widget routes for client.helpin.ai (no JWT, open CORS) ----
	// Mounted at /widget (outside /api) so the ingress path /widget/* works directly.
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
	})

	// ---- SDK asset serving (no JWT, open CORS, cache headers) ----
	if h.SDKAssets != nil {
		r.Route("/sdk", func(r chi.Router) {
			r.Get("/*", h.SDKAssets.ServeSDK)
		})
	}

	r.Route("/api", func(r chi.Router) {
		// ---- Public routes ----
		r.Post("/auth/signup", h.Auth.Signup)
		r.Post("/auth/signin", h.Auth.Signin)
		r.Post("/auth/refresh", h.Auth.RefreshToken)
		r.Get("/health", h.Health.Check)
		r.Get("/invitations/info", h.Invite.GetInfo)
		r.Post("/invitations/accept-with-signup", h.Invite.AcceptWithSignup)

		// ---- Public git webhook (no JWT) ----
		r.Get("/git/github/callback", h.Git.GitHubCallback)
		r.Post("/git/webhook", h.Git.Webhook)

		// ---- Public Gmail OAuth callback (Google redirects here without JWT) ----
		r.Get("/crm/email/oauth/callback", h.CRMEmail.OAuthCallbackRedirect)

		// ---- Help Center domain verification (Caddy on_demand_tls) ----
		r.Get("/hc/verify-domain", h.Docs.VerifyDomain)

		// ---- Public Help Center routes (no JWT) ----
		r.Route("/hc/{subdomain}", func(r chi.Router) {
			r.Get("/config", h.Docs.PublicGetConfig)
			r.Get("/spaces", h.Docs.PublicGetSpaces)
			r.Get("/spaces/{spaceSlug}/navigation", h.Docs.PublicGetSpaceNavigation)
			r.Get("/spaces/{spaceSlug}/articles/{articleSlug}", h.Docs.PublicGetSpaceArticle)
			r.Post("/spaces/{spaceSlug}/articles/{articleSlug}/feedback", h.Docs.PublicSubmitFeedback)
			r.Get("/search", h.Docs.PublicSearchArticles)

			// Canonical collection + article routes
			r.Get("/c/{collectionSlug}", h.Docs.PublicGetCollectionPage)
			r.Get("/c/{collectionSlug}/{articleSlug}", h.Docs.PublicGetCanonicalArticle)

			// Legacy/redirect resolver
			r.Get("/resolve/*", h.Docs.PublicResolvePath)
		})

		// ---- Public shared document route (no JWT) ----
		r.Get("/docs/shared/{shareToken}", h.Docs.PublicGetSharedDoc)

		// ---- Public widget routes (no JWT, open CORS) ----
		r.Route("/widget/support", func(r chi.Router) {
			r.Use(cors.Handler(cors.Options{
				AllowedOrigins:   []string{"*"},
				AllowedMethods:   []string{"GET", "POST", "OPTIONS"},
				AllowedHeaders:   []string{"Content-Type"},
				AllowCredentials: false,
				MaxAge:           3600,
			}))
			r.Get("/config", h.SupportInboxWidget.GetConfig)
			r.Get("/help/spaces/{spaceSlug}/collections", h.SupportInboxWidget.GetHelpCollections)
			r.Get("/help/collections/{collectionSlug}/articles", h.SupportInboxWidget.GetHelpArticles)
			r.Get("/help/articles/{articleSlug}", h.SupportInboxWidget.GetHelpArticle)
			r.Post("/session", h.SupportInboxWidget.CreateSession)
			r.Post("/session/revoke", h.SupportInboxWidget.RevokeSession)
			r.Post("/messages", h.SupportInboxWidget.SendMessage)
			r.Post("/typing", h.SupportInboxWidget.TypingIndicator) // Deprecated: use WebSocket typing:start/typing:stop instead. Kept as HTTP fallback.
			r.Get("/messages", h.SupportInboxWidget.GetMessages)
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
		})

		// ---- Internal service-to-service routes (bearer token auth) ----
		r.Route("/internal", func(r chi.Router) {
			r.Use(middleware.RequireInternalAPISecret)
			r.Get("/widget-tokens", h.SupportInboxWidget.GetWidgetTokens)
		})

		// ---- Protected routes ----
		r.Group(func(r chi.Router) {
			r.Use(middleware.RequireAuth(jwtManager))

			// Auth / profile
			r.Get("/auth/me", h.Auth.Me)
			r.Put("/auth/me", h.Auth.UpdateProfile)
			r.Post("/auth/me/avatar", h.Auth.UploadAvatar)
			r.Delete("/auth/me/avatar", h.Auth.DeleteAvatar)
			r.Put("/auth/change-password", h.Auth.ChangePassword)

			// User notification settings (account-level, no workspace scope)
			r.Get("/user/notification-settings", h.UserNotifSettings.Get)
			r.Put("/user/notification-settings", h.UserNotifSettings.Update)

			// Organizations
			r.Get("/organizations", h.Organization.List)
			r.Post("/organizations", h.Organization.Create)
			r.Get("/organizations/{id}", h.Organization.Get)
			r.Put("/organizations/{id}", h.Organization.Update)
			r.Delete("/organizations/{id}", h.Organization.Delete)
			r.Get("/organizations/{id}/members", h.Organization.ListMembers)
			r.Post("/organizations/{id}/members", h.Organization.AddMember)
			r.Put("/organizations/{id}/members/{userId}", h.Organization.UpdateMember)
			r.Delete("/organizations/{id}/members/{userId}", h.Organization.RemoveMember)

			// Workspaces — workspace-scoped routes with RBAC
			r.Get("/workspaces", h.Workspace.List)
			r.Post("/workspaces", h.Workspace.Create)

			// Slug lookup — resolve slug to workspace ID, then check access
			r.With(authorization.ResolveWorkspaceSlug(slugResolver), wsAccess).Get("/workspaces/by-slug/{slug}", h.Workspace.GetBySlug)

			r.Route("/workspaces/{id}", func(r chi.Router) {
				r.Use(authorization.ExtractWorkspaceIDParam)
				r.Use(wsAccess)

				r.Get("/my-role", h.Workspace.GetMyRole)
				r.Get("/my-membership", h.Workspace.GetMyMembership)
				r.Get("/me", h.Workspace.GetMe)
				r.Get("/members", h.Workspace.ListMembers)
				r.Get("/assignable-members", h.Workspace.ListAssignableMembers)

				r.With(requirePerm(authorization.PermWorkspaceUpdate)).Put("/", h.Workspace.Update)
				r.With(requirePerm(authorization.PermWorkspaceUpdate)).Post("/logo", h.Workspace.UploadLogo)
				r.With(requirePerm(authorization.PermWorkspaceUpdate)).Delete("/logo", h.Workspace.DeleteLogo)
				r.With(authorization.RequireOwner(authz)).Delete("/", h.Workspace.Delete)

				// Import routes require pm.import
				r.With(requirePerm(authorization.PermPMImport)).Post("/import/shortcut/preview", h.PMImport.PreviewShortcut)
				r.With(requirePerm(authorization.PermPMImport)).Post("/import/shortcut/execute", h.PMImport.ExecuteShortcut)
				r.With(requirePerm(authorization.PermPMImport)).Get("/import/shortcut/status/{importId}", h.PMImport.ShortcutStatus)
			})

			// Settings — all routes require workspace access
			r.Route("/settings", func(r chi.Router) {
				r.Use(middleware.RequireWorkspaceID)
				r.Use(wsAccess)

				// Read
				r.With(requirePerm(authorization.PermSettingsRead)).Get("/", h.Settings.GetAll)
				r.With(requirePerm(authorization.PermSettingsManage)).Get("/ai-automations", h.Settings.GetAIAutomations)

				// Settings management (admin+)
				r.With(requirePerm(authorization.PermSettingsManage)).Post("/initialize", h.Settings.Initialize)
				r.With(requirePerm(authorization.PermSettingsManage)).Put("/job-roles", h.Settings.UpdateJobRoleCriteria)
				r.With(requirePerm(authorization.PermSettingsManage)).Delete("/job-roles", h.Settings.DeleteJobRole)
				r.With(requirePerm(authorization.PermSettingsManage)).Put("/system", h.Settings.UpdateSystem)

				// People management
				r.With(requirePerm(authorization.PermWorkspaceMembersManage)).Post("/people", h.Settings.CreatePerson)
				r.With(requirePerm(authorization.PermWorkspaceMembersManage)).Put("/people/{id}", h.Settings.UpdatePerson)
				r.With(requirePerm(authorization.PermWorkspaceMembersManage)).Delete("/people/{id}", h.Settings.DeletePerson)

				// Team management — admin+ OR team owner via RequireTeamPermission
				r.With(requirePerm(authorization.PermTeamManage)).Post("/teams", h.Settings.CreateTeam)
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

			// Invitations
			r.Route("/invitations", func(r chi.Router) {
				r.Post("/accept", h.Invite.Accept)
				r.Get("/", h.Invite.List)

				// Sending and managing invitations requires workspace context
				r.With(middleware.RequireWorkspaceID, wsAccess, requirePerm(authorization.PermWorkspaceInvitesManage)).Post("/", h.Invite.Send)
				r.With(middleware.RequireWorkspaceID, wsAccess, requirePerm(authorization.PermWorkspaceInvitesManage)).Post("/{id}/resend", h.Invite.Resend)
				r.With(middleware.RequireWorkspaceID, wsAccess, requirePerm(authorization.PermWorkspaceInvitesManage)).Delete("/{id}", h.Invite.Revoke)
			})

			// Git integrations
			r.Route("/git", func(r chi.Router) {
				r.Use(middleware.RequireWorkspaceID)
				r.Use(wsAccess)
				r.Get("/github/install-url", h.Git.GetGitHubInstallURL)
				r.With(requirePerm(authorization.PermSettingsRead)).Get("/integrations", h.Git.ListIntegrations)
				r.With(requirePerm(authorization.PermSettingsManage)).Post("/integrations", h.Git.CreateIntegration)
				r.With(requirePerm(authorization.PermSettingsManage)).Post("/integrations/{id}/sync", h.Git.SyncRepositories)
				r.With(requirePerm(authorization.PermSettingsRead)).Get("/repositories", h.Git.ListRepositories)
				r.With(requirePerm(authorization.PermSettingsManage)).Put("/repositories/{id}", h.Git.UpdateRepository)
			})

			// Search
			r.Route("/search", func(r chi.Router) {
				r.Use(middleware.RequireWorkspaceID)
				r.Use(wsAccess)
				r.With(requirePerm(authorization.PermSearchRead)).Get("/", h.Search.Search)
			})

			// Support module
			r.Route("/support", func(r chi.Router) {
				r.Use(middleware.RequireWorkspaceID)
				r.Use(wsAccess)

				// Legacy /tickets routes (backward compat)
				r.With(requirePerm(authorization.PermSupportRead)).Get("/tickets", h.SupportInbox.ListConversations)
				r.With(requirePerm(authorization.PermSupportEdit)).Post("/tickets", h.SupportInbox.CreateConversation)
				r.With(requirePerm(authorization.PermSupportRead)).Get("/tickets/{id}", h.SupportInbox.GetConversation)
				r.With(requirePerm(authorization.PermSupportRead)).Get("/tickets/{id}/associations", h.Associations.ListConversationAssociations)
				r.With(requirePerm(authorization.PermSupportEdit)).Put("/tickets/{id}/status", h.SupportInbox.UpdateConversationStatus)
				r.With(requirePerm(authorization.PermSupportRead)).Get("/tickets/{id}/messages", h.SupportInbox.ListConversationMessages)
				r.With(requirePerm(authorization.PermSupportEdit)).Post("/tickets/{id}/messages", h.SupportInbox.CreateConversationMessage)
				r.With(requirePerm(authorization.PermSupportEdit)).Post("/tickets/{id}/link-story", h.SupportInbox.LinkConversationStory)
				r.With(requirePerm(authorization.PermSupportEdit)).Post("/tickets/{id}/assign-agent", h.SupportInbox.AssignConversationAgent)
				r.With(requirePerm(authorization.PermSupportEdit)).Post("/tickets/{id}/run-agent", h.SupportInbox.RunAgent)

				// New /inbox/conversations routes
				r.With(requirePerm(authorization.PermSupportRead)).Get("/inbox/unread-stats", h.SupportInbox.GetUnreadStats)
				r.With(requirePerm(authorization.PermSupportRead)).Get("/inbox/conversations", h.SupportInbox.ListConversations)
				r.With(requirePerm(authorization.PermSupportEdit)).Post("/inbox/conversations", h.SupportInbox.CreateConversation)
				r.With(requirePerm(authorization.PermSupportRead)).Get("/inbox/conversations/{id}", h.SupportInbox.GetConversation)
				r.With(requirePerm(authorization.PermSupportRead)).Get("/inbox/conversations/{id}/associations", h.Associations.ListConversationAssociations)
				r.With(requirePerm(authorization.PermSupportEdit)).Put("/inbox/conversations/{id}/status", h.SupportInbox.UpdateConversationStatus)
				r.With(requirePerm(authorization.PermSupportRead)).Get("/inbox/conversations/{id}/messages", h.SupportInbox.ListConversationMessages)
				r.With(requirePerm(authorization.PermSupportEdit)).Post("/inbox/conversations/{id}/messages", h.SupportInbox.CreateConversationMessage)
				r.With(requirePerm(authorization.PermSupportEdit)).Post("/inbox/conversations/{id}/link-story", h.SupportInbox.LinkConversationStory)
				r.With(requirePerm(authorization.PermSupportEdit)).Post("/inbox/conversations/{id}/assign-agent", h.SupportInbox.AssignConversationAgent)
				r.With(requirePerm(authorization.PermSupportEdit)).Post("/inbox/conversations/{id}/run-agent", h.SupportInbox.RunAgent)
				r.With(requirePerm(authorization.PermSupportRead)).Post("/inbox/conversations/{id}/read", h.SupportInbox.MarkConversationRead)

				// Installation settings
				r.With(requirePerm(authorization.PermSupportRead)).Get("/inbox/installations", h.SupportInbox.GetInstallation)
				r.With(requirePerm(authorization.PermSupportAdmin)).Patch("/inbox/installations", h.SupportInbox.UpdateInstallationSettings)
				r.With(requirePerm(authorization.PermSupportAdmin)).Post("/inbox/installations/regenerate-key", h.SupportInbox.RegenerateWidgetKey)

				// Canned responses
				r.With(requirePerm(authorization.PermSupportRead)).Get("/inbox/canned-responses", h.SupportInbox.ListCannedResponses)
				r.With(requirePerm(authorization.PermSupportRead)).Get("/inbox/canned-responses/search", h.SupportInbox.SearchCannedResponses)
				r.With(requirePerm(authorization.PermSupportEdit)).Post("/inbox/canned-responses", h.SupportInbox.CreateCannedResponse)
				r.With(requirePerm(authorization.PermSupportEdit)).Put("/inbox/canned-responses/{id}", h.SupportInbox.UpdateCannedResponse)
				r.With(requirePerm(authorization.PermSupportEdit)).Delete("/inbox/canned-responses/{id}", h.SupportInbox.DeleteCannedResponse)

				// Typing indicators — Deprecated: use WebSocket support:typing:start/stop instead. Kept as HTTP fallback.
				r.With(requirePerm(authorization.PermSupportEdit)).Post("/inbox/conversations/{id}/typing", h.SupportInbox.TypingIndicator)

				// Viewing presence — Deprecated: use WebSocket support:viewing:start/stop instead. Kept as HTTP fallback.
				r.With(requirePerm(authorization.PermSupportRead)).Post("/inbox/conversations/{id}/viewing", h.SupportInbox.ViewingPresence)
			})

			// PM module
			r.Route("/pm", func(r chi.Router) {
				r.Use(middleware.RequireWorkspaceID)
				r.Use(wsAccess)

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

				// Story Templates — pm.edit
				r.With(requirePerm(authorization.PermPMRead)).Get("/story-templates", h.PMStoryTemplate.List)
				r.With(requirePerm(authorization.PermPMRead)).Get("/story-templates/{id}", h.PMStoryTemplate.Get)
				r.With(requirePerm(authorization.PermPMEdit)).Post("/story-templates", h.PMStoryTemplate.Create)
				r.With(requirePerm(authorization.PermPMEdit)).Put("/story-templates/{id}", h.PMStoryTemplate.Update)
				r.With(requirePerm(authorization.PermPMEdit)).Delete("/story-templates/{id}", h.PMStoryTemplate.Delete)

				// Roadmap — pm.read
				r.With(requirePerm(authorization.PermPMRead)).Get("/roadmap", h.PMRoadmap.Get)

				// Epics — pm.read / pm.edit
				r.With(requirePerm(authorization.PermPMRead)).Get("/epics", h.PMEpic.List)
				r.With(requirePerm(authorization.PermPMEdit)).Post("/epics", h.PMEpic.Create)
				r.With(requirePerm(authorization.PermPMRead)).Get("/epics/{id}", h.PMEpic.Get)
				r.With(requirePerm(authorization.PermPMEdit)).Put("/epics/{id}", h.PMEpic.Update)
				r.With(requirePerm(authorization.PermPMEdit)).Delete("/epics/{id}", h.PMEpic.Delete)
				r.With(requirePerm(authorization.PermPMRead)).Get("/epics/{id}/stories", h.PMEpic.ListStories)
				r.With(requirePerm(authorization.PermPMEdit)).Put("/epics/{id}/health", h.PMEpic.UpdateHealth)
				r.With(requirePerm(authorization.PermPMRead)).Get("/epics/{id}/associations", h.Associations.ListEpicAssociations)
				r.With(requirePerm(authorization.PermPMRead)).Get("/epics/{id}/agent-runs", h.Agent.ListEpicRuns)
				r.With(requirePerm(authorization.PermPMRead)).Get("/flow-runs", h.Flow.ListRuns)
				r.With(requirePerm(authorization.PermPMRead)).Get("/flow-templates", h.Flow.ListTemplates)
				r.With(requirePerm(authorization.PermPMEdit)).Post("/flow-runs", h.Flow.StartRun)
				r.Route("/flow-runs/{flowRunId}", func(r chi.Router) {
					r.With(requirePerm(authorization.PermPMRead)).Get("/", h.Flow.GetRun)
					r.With(requirePerm(authorization.PermPMRead)).Get("/nodes", h.Flow.ListNodeRuns)
					r.With(requirePerm(authorization.PermPMEdit)).Post("/cancel", h.Flow.CancelRun)
					r.With(requirePerm(authorization.PermPMEdit)).Post("/nodes/{nodeRunId}/actions", h.Flow.SendNodeAction)
					r.With(requirePerm(authorization.PermPMEdit)).Post("/nodes/{nodeRunId}/retry", h.Flow.RetryNode)
					r.With(requirePerm(authorization.PermPMRead)).Get("/nodes/{nodeRunId}/messages", h.Flow.ListInteractiveMessages)
					r.With(requirePerm(authorization.PermPMEdit)).Post("/nodes/{nodeRunId}/messages", h.Flow.SendInteractiveMessage)
				})
				r.With(requirePerm(authorization.PermPMEdit)).Post("/epics/{id}/run-agent", h.Agent.RunEpicAgent)
				r.With(requirePerm(authorization.PermPMEdit)).Post("/epics/{id}/draft-spec", h.Agent.DraftEpicSpec)
				r.With(requirePerm(authorization.PermPMEdit)).Post("/epics/{id}/clarify-spec", h.Agent.ClarifyEpicSpec)
				r.With(requirePerm(authorization.PermPMEdit)).Post("/epics/{id}/approve-spec", h.Agent.ApproveEpicSpec)
				r.With(requirePerm(authorization.PermPMEdit)).Post("/epics/{id}/plan-stories", h.Agent.PlanEpicStories)
				r.With(requirePerm(authorization.PermPMEdit)).Post("/epics/{id}/kickoff-execution", h.Agent.KickoffEpicExecution)
				r.With(requirePerm(authorization.PermPMEdit)).Post("/epics/{id}/assign-orchestrator", h.Orchestration.AssignOrchestrator)

				// Planning sessions (interactive epic planning)
				r.With(requirePerm(authorization.PermPMEdit)).Post("/epics/{epicId}/planning-session", h.PlanningSession.Start)
				r.Route("/planning-sessions/{sessionId}", func(r chi.Router) {
					r.With(requirePerm(authorization.PermPMRead)).Get("/", h.PlanningSession.Get)
					r.With(requirePerm(authorization.PermPMRead)).Get("/messages", h.PlanningSession.GetMessages)
					r.With(requirePerm(authorization.PermPMEdit)).Post("/messages", h.PlanningSession.SendMessage)
					r.With(requirePerm(authorization.PermPMEdit)).Post("/finalize", h.PlanningSession.Finalize)
					r.With(requirePerm(authorization.PermPMEdit)).Post("/abandon", h.PlanningSession.Abandon)
				})

				// Sprints (PM) — pm.read / pm.edit
				r.With(requirePerm(authorization.PermPMRead)).Get("/sprints", h.PMSprint.List)
				r.With(requirePerm(authorization.PermPMEdit)).Post("/sprints", h.PMSprint.Create)
				r.With(requirePerm(authorization.PermPMRead)).Get("/sprints/{id}", h.PMSprint.Get)
				r.With(requirePerm(authorization.PermPMEdit)).Put("/sprints/{id}", h.PMSprint.Update)
				r.With(requirePerm(authorization.PermPMEdit)).Delete("/sprints/{id}", h.PMSprint.Delete)
				r.With(requirePerm(authorization.PermPMRead)).Get("/sprints/{id}/stories", h.PMSprint.ListStories)

				// Stories — pm.read / pm.edit
				r.With(requirePerm(authorization.PermPMRead)).Get("/stories", h.PMStory.List)
				r.With(requirePerm(authorization.PermPMEdit)).Post("/stories", h.PMStory.Create)
				r.With(requirePerm(authorization.PermPMRead)).Get("/stories/board", h.PMStory.ListBoard)
				r.With(requirePerm(authorization.PermPMRead)).Get("/stories/board/column", h.PMStory.ListBoardColumn)
				r.With(requirePerm(authorization.PermPMRead)).Get("/stories/board/members", h.PMStory.ListBoardByMember)
				r.With(requirePerm(authorization.PermPMRead)).Get("/stories/board/members/column", h.PMStory.ListBoardMemberColumn)
				r.With(requirePerm(authorization.PermPMRead)).Get("/stories/counts", h.PMStory.CountByState)
				r.With(requirePerm(authorization.PermPMRead)).Get("/stories/display/{displayID}", h.PMStory.GetByDisplayID)
				r.With(requirePerm(authorization.PermPMRead)).Get("/stories/{id}", h.PMStory.Get)
				r.With(requirePerm(authorization.PermPMEdit)).Put("/stories/{id}", h.PMStory.Update)
				r.With(requirePerm(authorization.PermPMEdit)).Delete("/stories/{id}", h.PMStory.Delete)
				r.With(requirePerm(authorization.PermPMEdit)).Put("/stories/{id}/move", h.PMStory.Move)
				r.With(requirePerm(authorization.PermPMEdit)).Put("/stories/{id}/reorder", h.PMStory.Reorder)
				r.With(requirePerm(authorization.PermPMEdit)).Post("/stories/{id}/owners", h.PMStory.AddOwner)
				r.With(requirePerm(authorization.PermPMEdit)).Delete("/stories/{id}/owners/{userId}", h.PMStory.RemoveOwner)
				r.With(requirePerm(authorization.PermPMEdit)).Post("/stories/{id}/followers", h.PMStory.AddFollower)
				r.With(requirePerm(authorization.PermPMEdit)).Delete("/stories/{id}/followers", h.PMStory.RemoveFollower)
				r.With(requirePerm(authorization.PermPMEdit)).Post("/stories/{id}/labels", h.PMStory.AddLabel)
				r.With(requirePerm(authorization.PermPMEdit)).Delete("/stories/{id}/labels/{labelId}", h.PMStory.RemoveLabel)
				r.With(requirePerm(authorization.PermPMRead)).Get("/stories/{id}/associations", h.Associations.ListStoryAssociations)
				r.With(requirePerm(authorization.PermPMEdit)).Post("/stories/{id}/relationships", h.Associations.CreateStoryRelationship)
				r.With(requirePerm(authorization.PermPMEdit)).Delete("/story-relationships/{id}", h.Associations.DeleteStoryRelationship)
				r.With(requirePerm(authorization.PermPMRead)).Get("/stories/{id}/activity", h.PMStory.ListActivity)
				r.With(requirePerm(authorization.PermPMRead)).Get("/stories/{id}/git-links", h.Git.GetStoryGitLinks)
				r.With(requirePerm(authorization.PermPMRead)).Get("/stories/{id}/delivery-target", h.Git.GetStoryDeliveryTarget)
				r.With(requirePerm(authorization.PermPMEdit)).Put("/stories/{id}/delivery-target", h.Git.UpdateStoryDeliveryTarget)
				r.With(requirePerm(authorization.PermPMEdit)).Post("/stories/{id}/create-branch", h.Git.CreateBranch)

				// Comments — pm.read / pm.edit
				r.With(requirePerm(authorization.PermPMRead)).Get("/comments", h.PMComment.List)
				r.With(requirePerm(authorization.PermPMEdit)).Post("/comments", h.PMComment.Create)
				r.With(requirePerm(authorization.PermPMEdit)).Put("/comments/{id}", h.PMComment.Update)
				r.With(requirePerm(authorization.PermPMEdit)).Delete("/comments/{id}", h.PMComment.Delete)
				r.With(requirePerm(authorization.PermPMRead)).Post("/comments/{id}/reactions", h.PMComment.ToggleReaction)

				// Attachments — pm.edit
				r.With(requirePerm(authorization.PermPMEdit)).Post("/attachments", h.PMAttachment.Create)
				r.With(requirePerm(authorization.PermPMEdit)).Patch("/attachments/{id}/confirm", h.PMAttachment.ConfirmUpload)
				r.With(requirePerm(authorization.PermPMRead)).Get("/attachments", h.PMAttachment.List)
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
				r.With(requirePerm(authorization.PermPMRead)).Get("/stories/{id}/checklist", h.PMChecklistItem.List)
				r.With(requirePerm(authorization.PermPMEdit)).Post("/stories/{id}/checklist", h.PMChecklistItem.Create)
				r.With(requirePerm(authorization.PermPMEdit)).Put("/checklist-items/{id}", h.PMChecklistItem.Update)
				r.With(requirePerm(authorization.PermPMEdit)).Delete("/checklist-items/{id}", h.PMChecklistItem.Delete)

				// External links — pm.edit
				r.With(requirePerm(authorization.PermPMRead)).Get("/stories/{id}/links", h.PMExternalLink.List)
				r.With(requirePerm(authorization.PermPMEdit)).Post("/stories/{id}/links", h.PMExternalLink.Create)
				r.With(requirePerm(authorization.PermPMEdit)).Put("/links/{id}", h.PMExternalLink.Update)
				r.With(requirePerm(authorization.PermPMEdit)).Delete("/links/{id}", h.PMExternalLink.Delete)

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

				// Agents — pm.edit
				r.With(requirePerm(authorization.PermPMRead)).Get("/agents", h.Agent.ListAgents)
				r.With(requirePerm(authorization.PermPMEdit)).Post("/agents", h.Agent.CreateAgent)
				r.With(requirePerm(authorization.PermPMRead)).Get("/runtime-profiles", h.Agent.ListRuntimeProfiles)
				r.With(requirePerm(authorization.PermPMRead)).Get("/agent-model-providers", h.Agent.ListModelProviders)
				r.With(requirePerm(authorization.PermPMRead)).Get("/runner-health", h.Agent.GetRunnerHealth)
				r.With(requirePerm(authorization.PermPMRead)).Get("/agents/{id}", h.Agent.GetAgent)
				r.With(requirePerm(authorization.PermPMEdit)).Put("/agents/{id}", h.Agent.UpdateAgent)
				r.With(requirePerm(authorization.PermPMEdit)).Delete("/agents/{id}", h.Agent.DeleteAgent)
				r.With(requirePerm(authorization.PermPMRead)).Get("/agents/{id}/runs", h.Agent.ListAgentRuns)
				r.With(requirePerm(authorization.PermPMEdit)).Post("/stories/{id}/assign-agent", h.Agent.AssignAgentToStory)
				r.With(requirePerm(authorization.PermPMEdit)).Post("/stories/{id}/run-agent", h.Agent.RunAgent)
				r.With(requirePerm(authorization.PermPMRead)).Get("/agent-runs/{id}", h.Agent.GetAgentRun)
				r.With(requirePerm(authorization.PermPMRead)).Get("/agent-runs/{id}/artifacts", h.Agent.ListRunArtifacts)
				r.With(requirePerm(authorization.PermPMEdit)).Post("/agent-runs/{id}/cancel", h.Agent.CancelRun)
				r.With(requirePerm(authorization.PermPMEdit)).Post("/agent-runs/{id}/approve", h.Agent.ApproveRun)
				r.With(requirePerm(authorization.PermPMEdit)).Post("/agent-runs/{id}/confirm-orchestration", h.Agent.ConfirmEpicRun)
				r.With(requirePerm(authorization.PermPMEdit)).Post("/agent-runs/{id}/handoff", h.Agent.HandoffRun)
			})

			// Notifications module
			r.Route("/notifications", func(r chi.Router) {
				r.Use(middleware.RequireWorkspaceID)
				r.Use(wsAccess)

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
				r.Use(wsAccess)

				r.With(requirePerm(authorization.PermPMRead)).Get("/", h.Notification.ListFollowers)
				r.With(requirePerm(authorization.PermPMRead)).Get("/check", h.Notification.IsFollowing)
				r.With(requirePerm(authorization.PermPMRead)).Post("/", h.Notification.Follow)
				r.With(requirePerm(authorization.PermPMRead)).Delete("/", h.Notification.Unfollow)
			})

			// Docs module
			r.Route("/docs", func(r chi.Router) {
				r.Use(middleware.RequireWorkspaceID)
				r.Use(wsAccess)

				// Spaces — docs.read / docs.edit / docs.admin
				r.With(requirePerm(authorization.PermDocsRead)).Get("/spaces", h.Docs.ListSpaces)
				r.With(requirePerm(authorization.PermDocsEdit)).Post("/spaces", h.Docs.CreateSpace)
				r.With(requirePerm(authorization.PermDocsRead)).Get("/spaces/{spaceId}", h.Docs.GetSpace)
				r.With(requirePerm(authorization.PermDocsEdit)).Patch("/spaces/{spaceId}", h.Docs.UpdateSpace)
				r.With(requirePerm(authorization.PermDocsAdmin)).Delete("/spaces/{spaceId}", h.Docs.DeleteSpace)
				r.With(requirePerm(authorization.PermDocsAdmin)).Post("/spaces/{spaceId}/restore", h.Docs.RestoreSpace)

				// Collections — docs.read / docs.edit
				r.With(requirePerm(authorization.PermDocsRead)).Get("/spaces/{spaceId}/collections", h.Docs.ListCollections)
				r.With(requirePerm(authorization.PermDocsEdit)).Post("/spaces/{spaceId}/collections", h.Docs.CreateCollection)
				r.With(requirePerm(authorization.PermDocsEdit)).Patch("/collections/{collectionId}", h.Docs.UpdateCollection)
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

				// Content — docs.read / docs.edit
				r.With(requirePerm(authorization.PermDocsRead)).Get("/documents/{docId}/content", h.Docs.GetContent)
				r.With(requirePerm(authorization.PermDocsEdit)).Put("/documents/{docId}/content", h.Docs.SaveContent)
				r.With(requirePerm(authorization.PermDocsEdit)).Put("/documents/{docId}/content/markdown", h.Docs.SaveMarkdownContent)

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

				// Search
				r.With(requirePerm(authorization.PermDocsRead)).Get("/search", h.Docs.Search)

				// Help Center Config — docs.admin
				r.With(requirePerm(authorization.PermDocsRead)).Get("/helpcenter/config", h.Docs.GetHelpcenterConfig)
				r.With(requirePerm(authorization.PermDocsAdmin)).Put("/helpcenter/config", h.Docs.UpdateHelpcenterConfig)
				r.With(requirePerm(authorization.PermDocsAdmin)).Post("/helpcenter/upload", h.Docs.UploadHelpcenterAsset)

				// Feedback
				r.With(requirePerm(authorization.PermDocsEdit)).Post("/articles/{docId}/feedback", h.Docs.SubmitArticleFeedback)

				// Docs import
				r.With(requirePerm(authorization.PermDocsImport)).Post("/import/helpscout/preview", h.Docs.ImportPreviewHelpscout)
				r.With(requirePerm(authorization.PermDocsImport)).Post("/import/helpscout/start", h.Docs.ImportStartHelpscout)
				r.With(requirePerm(authorization.PermDocsImport)).Get("/import/{jobId}/status", h.Docs.ImportGetStatus)
				r.With(requirePerm(authorization.PermDocsImport)).Post("/import/{jobId}/retry", h.Docs.ImportRetry)
				r.With(requirePerm(authorization.PermDocsImport)).Get("/import/{jobId}/redirect-map", h.Docs.ImportGetRedirectMap)
			})

			// CRM module
			r.Route("/crm", func(r chi.Router) {
				r.Use(middleware.RequireWorkspaceID)
				r.Use(wsAccess)

				// Contacts — crm.read / crm.edit
				r.With(requirePerm(authorization.PermCRMRead)).Get("/contacts", h.CRMContact.List)
				r.With(requirePerm(authorization.PermCRMEdit)).Post("/contacts", h.CRMContact.Create)
				r.With(requirePerm(authorization.PermCRMRead)).Get("/contacts/{id}", h.CRMContact.Get)
				r.With(requirePerm(authorization.PermCRMEdit)).Put("/contacts/{id}", h.CRMContact.Update)
				r.With(requirePerm(authorization.PermCRMEdit)).Delete("/contacts/{id}", h.CRMContact.Delete)
				r.With(requirePerm(authorization.PermCRMRead)).Get("/contacts/{id}/activities", h.CRMActivity.ListByContact)
				r.With(requirePerm(authorization.PermCRMRead)).Get("/contacts/{id}/associations", h.CRMAssociation.ListContactAssociations)
				r.With(requirePerm(authorization.PermCRMRead)).Get("/contacts/{id}/support-conversations", h.SupportInbox.ListContactConversations)

				// Companies — crm.read / crm.edit
				r.With(requirePerm(authorization.PermCRMRead)).Get("/companies", h.CRMCompany.List)
				r.With(requirePerm(authorization.PermCRMEdit)).Post("/companies", h.CRMCompany.Create)
				r.With(requirePerm(authorization.PermCRMRead)).Get("/companies/{id}", h.CRMCompany.Get)
				r.With(requirePerm(authorization.PermCRMEdit)).Put("/companies/{id}", h.CRMCompany.Update)
				r.With(requirePerm(authorization.PermCRMEdit)).Delete("/companies/{id}", h.CRMCompany.Delete)
				r.With(requirePerm(authorization.PermCRMRead)).Get("/companies/{id}/activities", h.CRMActivity.ListByCompany)
				r.With(requirePerm(authorization.PermCRMRead)).Get("/companies/{id}/associations", h.CRMAssociation.ListCompanyAssociations)

				// Deals — crm.read / crm.edit
				r.With(requirePerm(authorization.PermCRMRead)).Get("/deals", h.CRMDeal.List)
				r.With(requirePerm(authorization.PermCRMEdit)).Post("/deals", h.CRMDeal.Create)
				r.With(requirePerm(authorization.PermCRMRead)).Get("/deals/{id}", h.CRMDeal.Get)
				r.With(requirePerm(authorization.PermCRMEdit)).Put("/deals/{id}", h.CRMDeal.Update)
				r.With(requirePerm(authorization.PermCRMEdit)).Delete("/deals/{id}", h.CRMDeal.Delete)
				r.With(requirePerm(authorization.PermCRMRead)).Get("/deals/{id}/activities", h.CRMActivity.ListByDeal)
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

				// Properties — crm.read / crm.admin
				r.With(requirePerm(authorization.PermCRMRead)).Get("/properties", h.CRMProperty.ListDefinitions)
				r.With(requirePerm(authorization.PermCRMAdmin)).Post("/properties", h.CRMProperty.CreateDefinition)
				r.With(requirePerm(authorization.PermCRMAdmin)).Put("/properties/{id}", h.CRMProperty.UpdateDefinition)
				r.With(requirePerm(authorization.PermCRMAdmin)).Delete("/properties/{id}", h.CRMProperty.DeleteDefinition)

				// Property Groups — crm.read / crm.admin
				r.With(requirePerm(authorization.PermCRMRead)).Get("/property-groups", h.CRMProperty.ListGroups)
				r.With(requirePerm(authorization.PermCRMAdmin)).Post("/property-groups", h.CRMProperty.CreateGroup)
				r.With(requirePerm(authorization.PermCRMAdmin)).Put("/property-groups/{id}", h.CRMProperty.UpdateGroup)
				r.With(requirePerm(authorization.PermCRMAdmin)).Delete("/property-groups/{id}", h.CRMProperty.DeleteGroup)

				// Lists — crm.read / crm.edit
				r.With(requirePerm(authorization.PermCRMRead)).Get("/lists", h.CRMList.List)
				r.With(requirePerm(authorization.PermCRMEdit)).Post("/lists", h.CRMList.Create)
				r.With(requirePerm(authorization.PermCRMRead)).Get("/lists/{id}", h.CRMList.Get)
				r.With(requirePerm(authorization.PermCRMEdit)).Put("/lists/{id}", h.CRMList.Update)
				r.With(requirePerm(authorization.PermCRMEdit)).Delete("/lists/{id}", h.CRMList.Delete)
				r.With(requirePerm(authorization.PermCRMRead)).Get("/lists/{id}/members", h.CRMList.ListMembers)
				r.With(requirePerm(authorization.PermCRMEdit)).Post("/lists/{id}/members", h.CRMList.AddMember)
				r.With(requirePerm(authorization.PermCRMEdit)).Delete("/lists/{id}/members/{objectId}", h.CRMList.RemoveMember)

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
				r.With(requirePerm(authorization.PermCRMAdmin)).Get("/email/accounts/{id}/diagnostics", h.CRMEmail.GetAccountDiagnostics)
				r.With(requirePerm(authorization.PermCRMAdmin)).Post("/email/accounts/{id}/maintenance/rebuild-associations", h.CRMEmail.RebuildAssociations)
				r.With(requirePerm(authorization.PermCRMAdmin)).Delete("/email/accounts/{id}/data", h.CRMEmail.PurgeAccountData)
				r.With(requirePerm(authorization.PermCRMEdit)).Post("/email/accounts/{id}/oauth-callback", h.CRMEmail.OAuthCallback)
				r.With(requirePerm(authorization.PermCRMRead)).Get("/email/threads", h.CRMEmail.ListThreads)
				r.With(requirePerm(authorization.PermCRMRead)).Get("/email/messages", h.CRMEmail.ListMessages)
				r.With(requirePerm(authorization.PermCRMEdit)).Post("/email/messages", h.CRMEmail.CreateMessage)
				r.With(requirePerm(authorization.PermCRMEdit)).Post("/email/send", h.CRMEmail.SendEmail)
				r.With(requirePerm(authorization.PermCRMRead)).Get("/contacts/{id}/emails", h.CRMEmail.ListByContact)
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

				// Signals — crm.read / crm.edit
				r.With(requirePerm(authorization.PermCRMRead)).Get("/signals", h.CRMSignal.ListSignals)
				r.With(requirePerm(authorization.PermCRMEdit)).Post("/signals", h.CRMSignal.CreateSignal)
				r.With(requirePerm(authorization.PermCRMEdit)).Delete("/signals/{id}", h.CRMSignal.DeleteSignal)
				r.With(requirePerm(authorization.PermCRMRead)).Get("/contacts/{id}/signals", h.CRMSignal.ListByContact)
				r.With(requirePerm(authorization.PermCRMRead)).Get("/deals/{id}/signals", h.CRMSignal.ListByDeal)
				r.With(requirePerm(authorization.PermCRMRead)).Get("/contacts/{id}/summary", h.CRMSummary.GetContactSummary)
				r.With(requirePerm(authorization.PermCRMRead)).Get("/deals/{id}/summary", h.CRMSummary.GetDealSummary)

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

				// Sequences — crm.read / crm.edit
				r.With(requirePerm(authorization.PermCRMRead)).Get("/sequences", h.CRMSequence.List)
				r.With(requirePerm(authorization.PermCRMEdit)).Post("/sequences", h.CRMSequence.Create)
				r.With(requirePerm(authorization.PermCRMRead)).Get("/sequences/{id}", h.CRMSequence.Get)
				r.With(requirePerm(authorization.PermCRMEdit)).Put("/sequences/{id}", h.CRMSequence.Update)
				r.With(requirePerm(authorization.PermCRMEdit)).Delete("/sequences/{id}", h.CRMSequence.Delete)
				r.With(requirePerm(authorization.PermCRMRead)).Get("/sequences/{id}/enrollments", h.CRMSequence.ListEnrollments)
				r.With(requirePerm(authorization.PermCRMEdit)).Post("/sequences/{id}/enrollments", h.CRMSequence.CreateEnrollment)
				r.With(requirePerm(authorization.PermCRMEdit)).Put("/enrollments/{id}", h.CRMSequence.UpdateEnrollment)
				r.With(requirePerm(authorization.PermCRMEdit)).Delete("/enrollments/{id}", h.CRMSequence.DeleteEnrollment)

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
