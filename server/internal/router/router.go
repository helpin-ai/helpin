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
	Health          *handler.HealthHandler
	Auth            *handler.AuthHandler
	Organization    *handler.OrganizationHandler
	Workspace       *handler.WorkspaceHandler
	RewardQuarter   *handler.RewardQuarterHandler
	RewardSprint    *handler.RewardSprintHandler
	RewardGoal      *handler.RewardGoalHandler
	RewardBonus     *handler.RewardBonusHandler
	RewardFinance   *handler.RewardFinanceHandler
	Settings        *handler.SettingsHandler
	RewardAudit     *handler.RewardAuditHandler
	RewardDraft     *handler.RewardDraftHandler
	Invite          *handler.InviteHandler
	PMWorkflow      *handler.PMWorkflowHandler
	PMImport        *handler.PMImportHandler
	PMLabel         *handler.PMLabelHandler
	PMEpic          *handler.PMEpicHandler
	PMSprint        *handler.PMSprintHandler
	PMStory         *handler.PMStoryHandler
	PMComment       *handler.PMCommentHandler
	PMAttachment    *handler.PMAttachmentHandler
	PMObjective     *handler.PMObjectiveHandler
	PMChecklistItem *handler.PMChecklistItemHandler
	PMExternalLink  *handler.PMExternalLinkHandler
	PMView          *handler.PMViewHandler
	PMAutomation      *handler.PMAutomationHandler
	PMStoryTemplate   *handler.PMStoryTemplateHandler
	Search            *handler.SearchHandler
	Agent           *handler.AgentHandler
	Support         *handler.SupportHandler
	Widget          *handler.WidgetHandler
	Git             *handler.GitHandler
	Orchestration   *handler.OrchestrationHandler
	Docs            *handler.DocsHandler
	Notification    *handler.NotificationHandler
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
		ExposedHeaders:   []string{"Link"},
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

	r.Route("/api", func(r chi.Router) {
		// ---- Public routes ----
		r.Post("/auth/signup", h.Auth.Signup)
		r.Post("/auth/signin", h.Auth.Signin)
		r.Post("/auth/refresh", h.Auth.RefreshToken)
		r.Get("/health", h.Health.Check)
		r.Get("/invitations/info", h.Invite.GetInfo)

		// ---- Public git webhook (no JWT) ----
		r.Get("/git/github/callback", h.Git.GitHubCallback)
		r.Post("/git/webhook", h.Git.Webhook)

		// ---- Public Help Center routes (no JWT) ----
		r.Route("/hc/{subdomain}", func(r chi.Router) {
			r.Get("/articles/{slug}", h.Docs.PublicGetArticle)
			r.Get("/search", h.Docs.PublicSearchArticles)
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
			r.Get("/config", h.Widget.GetConfig)
			r.Post("/session", h.Widget.CreateSession)
			r.Post("/messages", h.Widget.SendMessage)
			r.Get("/messages", h.Widget.GetMessages)
		})

		// ---- Protected routes ----
		r.Group(func(r chi.Router) {
			r.Use(middleware.RequireAuth(jwtManager))

			// Auth / profile
			r.Get("/auth/me", h.Auth.Me)
			r.Put("/auth/me", h.Auth.UpdateProfile)

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

			// Rewards module — all routes require workspace access
			r.Route("/rewards", func(r chi.Router) {
				r.Use(middleware.RequireWorkspaceID)
				r.Use(wsAccess)

				// Read routes — rewards.read
				r.With(requirePerm(authorization.PermRewardsRead)).Get("/quarters", h.RewardQuarter.List)
				r.With(requirePerm(authorization.PermRewardsRead)).Get("/quarters/{id}", h.RewardQuarter.Get)
				r.With(requirePerm(authorization.PermRewardsRead)).Get("/sprints", h.RewardSprint.List)
				r.With(requirePerm(authorization.PermRewardsRead)).Get("/sprints/{id}", h.RewardSprint.Get)
				r.With(requirePerm(authorization.PermRewardsRead)).Get("/sprints/{id}/checks", h.RewardSprint.GetIndividualChecks)
				r.With(requirePerm(authorization.PermRewardsRead)).Get("/goals", h.RewardGoal.List)
				r.With(requirePerm(authorization.PermRewardsRead)).Get("/goals/sprint", h.RewardGoal.ListSprintGoals)
				r.With(requirePerm(authorization.PermRewardsRead)).Get("/bonus/calculations", h.RewardBonus.GetCalculations)
				r.With(requirePerm(authorization.PermRewardsRead)).Get("/bonus/team-sprint-data", h.RewardBonus.GetTeamSprintData)
				r.With(requirePerm(authorization.PermRewardsRead)).Get("/finance", h.RewardFinance.Get)
				r.With(requirePerm(authorization.PermRewardsRead)).Get("/audit", h.RewardAudit.List)
				r.With(requirePerm(authorization.PermRewardsRead)).Get("/drafts", h.RewardDraft.List)
				r.With(requirePerm(authorization.PermRewardsRead)).Get("/drafts/{id}", h.RewardDraft.Get)

				// Write routes — rewards.manage
				r.With(requirePerm(authorization.PermRewardsManage)).Post("/quarters", h.RewardQuarter.Create)
				r.With(requirePerm(authorization.PermRewardsManage)).Patch("/quarters/{id}/status", h.RewardQuarter.UpdateStatus)
				r.With(requirePerm(authorization.PermRewardsManage)).Post("/sprints/{id}/checks", h.RewardSprint.UpsertIndividualCheck)
				r.With(requirePerm(authorization.PermRewardsManage)).Post("/sprints/{id}/lock", h.RewardSprint.Lock)
				r.With(requirePerm(authorization.PermRewardsManage)).Post("/sprints/{id}/unlock", h.RewardSprint.Unlock)
				r.With(requirePerm(authorization.PermRewardsManage)).Post("/goals", h.RewardGoal.Create)
				r.With(requirePerm(authorization.PermRewardsManage)).Post("/goals/sprint", h.RewardGoal.UpsertSprintGoal)
				r.With(requirePerm(authorization.PermRewardsManage)).Post("/bonus/calculations", h.RewardBonus.SaveCalculations)
				r.With(requirePerm(authorization.PermRewardsManage)).Post("/bonus/lock", h.RewardBonus.Lock)
				r.With(requirePerm(authorization.PermRewardsManage)).Post("/bonus/unlock", h.RewardBonus.Unlock)
				r.With(requirePerm(authorization.PermRewardsManage)).Post("/finance", h.RewardFinance.Upsert)
				r.With(requirePerm(authorization.PermRewardsManage)).Post("/drafts", h.RewardDraft.Create)
				r.With(requirePerm(authorization.PermRewardsManage)).Put("/drafts/{id}", h.RewardDraft.Update)
				r.With(requirePerm(authorization.PermRewardsManage)).Delete("/drafts/{id}", h.RewardDraft.Delete)
			})

			// Settings — all routes require workspace access
			r.Route("/settings", func(r chi.Router) {
				r.Use(middleware.RequireWorkspaceID)
				r.Use(wsAccess)

				// Read
				r.With(requirePerm(authorization.PermSettingsRead)).Get("/", h.Settings.GetAll)

				// Settings management (admin+)
				r.With(requirePerm(authorization.PermSettingsManage)).Post("/initialize", h.Settings.Initialize)
				r.With(requirePerm(authorization.PermSettingsManage)).Put("/bonus-tiers", h.Settings.UpdateBonusTiers)
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

				r.With(requirePerm(authorization.PermPMRead)).Get("/tickets", h.Support.ListTickets)
				r.With(requirePerm(authorization.PermPMEdit)).Post("/tickets", h.Support.CreateTicket)
				r.With(requirePerm(authorization.PermPMRead)).Get("/tickets/{id}", h.Support.GetTicket)
				r.With(requirePerm(authorization.PermPMEdit)).Put("/tickets/{id}/status", h.Support.UpdateTicketStatus)
				r.With(requirePerm(authorization.PermPMRead)).Get("/tickets/{id}/messages", h.Support.ListMessages)
				r.With(requirePerm(authorization.PermPMEdit)).Post("/tickets/{id}/messages", h.Support.CreateMessage)
				r.With(requirePerm(authorization.PermPMEdit)).Post("/tickets/{id}/link-story", h.Support.LinkStory)
				r.With(requirePerm(authorization.PermPMEdit)).Post("/tickets/{id}/assign-agent", h.Support.AssignAgent)
				r.With(requirePerm(authorization.PermPMEdit)).Post("/tickets/{id}/run-agent", h.Support.RunAgent)
			})

			// PM module
			r.Route("/pm", func(r chi.Router) {
				r.Use(middleware.RequireWorkspaceID)
				r.Use(wsAccess)

				// Workflows — read: pm.read, write: pm.admin.workflows
				r.With(requirePerm(authorization.PermPMRead)).Get("/workflows", h.PMWorkflow.List)
				r.With(requirePerm(authorization.PermPMAdminWorkflows)).Post("/workflows", h.PMWorkflow.Create)
				r.With(requirePerm(authorization.PermPMRead)).Get("/workflows/epic-states", h.PMWorkflow.ListEpicStates)
				r.With(requirePerm(authorization.PermPMRead)).Get("/workflows/{id}", h.PMWorkflow.Get)
				r.With(requirePerm(authorization.PermPMAdminWorkflows)).Put("/workflows/{id}", h.PMWorkflow.Update)
				r.With(requirePerm(authorization.PermPMAdminWorkflows)).Delete("/workflows/{id}", h.PMWorkflow.Delete)
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

				// Epics — pm.read / pm.edit
				r.With(requirePerm(authorization.PermPMRead)).Get("/epics", h.PMEpic.List)
				r.With(requirePerm(authorization.PermPMEdit)).Post("/epics", h.PMEpic.Create)
				r.With(requirePerm(authorization.PermPMRead)).Get("/epics/{id}", h.PMEpic.Get)
				r.With(requirePerm(authorization.PermPMEdit)).Put("/epics/{id}", h.PMEpic.Update)
				r.With(requirePerm(authorization.PermPMEdit)).Delete("/epics/{id}", h.PMEpic.Delete)
				r.With(requirePerm(authorization.PermPMRead)).Get("/epics/{id}/stories", h.PMEpic.ListStories)
				r.With(requirePerm(authorization.PermPMEdit)).Put("/epics/{id}/health", h.PMEpic.UpdateHealth)
				r.With(requirePerm(authorization.PermPMEdit)).Post("/epics/{id}/orchestrate", h.Orchestration.Orchestrate)
				r.With(requirePerm(authorization.PermPMEdit)).Post("/epics/{id}/orchestrate/confirm", h.Orchestration.ConfirmOrchestration)
				r.With(requirePerm(authorization.PermPMEdit)).Post("/epics/{id}/assign-orchestrator", h.Orchestration.AssignOrchestrator)

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

				// Agents — pm.edit
				r.With(requirePerm(authorization.PermPMRead)).Get("/agents", h.Agent.ListAgents)
				r.With(requirePerm(authorization.PermPMEdit)).Post("/agents", h.Agent.CreateAgent)
				r.With(requirePerm(authorization.PermPMRead)).Get("/runtime-profiles", h.Agent.ListRuntimeProfiles)
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

				// Feedback
				r.With(requirePerm(authorization.PermDocsEdit)).Post("/articles/{docId}/feedback", h.Docs.SubmitArticleFeedback)
			})
		})
	})

	return r
}
