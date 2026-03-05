package router

import (
	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"

	"github.com/d4interactive/teampulse/server/internal/auth"
	"github.com/d4interactive/teampulse/server/internal/handler"
	"github.com/d4interactive/teampulse/server/internal/middleware"
)

// Handlers aggregates all HTTP handlers.
type Handlers struct {
	Health      *handler.HealthHandler
	Auth        *handler.AuthHandler
	Workspace   *handler.WorkspaceHandler
	Quarter     *handler.QuarterHandler
	Sprint      *handler.SprintHandler
	Goal        *handler.GoalHandler
	Bonus       *handler.BonusHandler
	Finance     *handler.FinanceHandler
	Settings    *handler.SettingsHandler
	Audit       *handler.AuditHandler
	Draft       *handler.DraftHandler
	Invite      *handler.InviteHandler
	PMWorkflow  *handler.PMWorkflowHandler
	PMLabel     *handler.PMLabelHandler
	PMEpic      *handler.PMEpicHandler
	PMSprint *handler.PMSprintHandler
	PMStory     *handler.PMStoryHandler
	PMComment    *handler.PMCommentHandler
	PMAttachment *handler.PMAttachmentHandler
	PMObjective      *handler.PMObjectiveHandler
	PMChecklistItem  *handler.PMChecklistItemHandler
	PMExternalLink   *handler.PMExternalLinkHandler
	PMView           *handler.PMViewHandler
	Search           *handler.SearchHandler
}

// New creates and configures the Chi router with all routes.
func New(h Handlers, jwtManager *auth.JWTManager, corsOrigin string) *chi.Mux {
	r := chi.NewRouter()

	// Global middleware
	r.Use(chimiddleware.RequestID)
	r.Use(chimiddleware.RealIP)
	r.Use(middleware.RequestLogger)
	r.Use(chimiddleware.Recoverer)
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{corsOrigin},
		AllowedMethods:   []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-Workspace-ID"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	r.Route("/api", func(r chi.Router) {
		// ---- Public routes ----
		r.Post("/auth/signup", h.Auth.Signup)
		r.Post("/auth/signin", h.Auth.Signin)
		r.Post("/auth/refresh", h.Auth.RefreshToken)
		r.Get("/health", h.Health.Check)
		r.Get("/invitations/info", h.Invite.GetInfo)

		// ---- Protected routes ----
		r.Group(func(r chi.Router) {
			r.Use(middleware.RequireAuth(jwtManager))

			// Auth / profile
			r.Get("/auth/me", h.Auth.Me)
			r.Put("/auth/me", h.Auth.UpdateProfile)

			// Workspaces
			r.Get("/workspaces", h.Workspace.List)
			r.Post("/workspaces", h.Workspace.Create)
			r.Get("/workspaces/by-slug/{slug}", h.Workspace.GetBySlug)
			r.Put("/workspaces/{id}", h.Workspace.Update)
			r.Delete("/workspaces/{id}", h.Workspace.Delete)
			r.Get("/workspaces/{id}/my-role", h.Workspace.GetMyRole)
			r.Get("/workspaces/{id}/my-membership", h.Workspace.GetMyMembership)
			r.Get("/workspaces/{id}/members", h.Workspace.ListMembers)

			// Quarters
			r.Get("/quarters", h.Quarter.List)
			r.Post("/quarters", h.Quarter.Create)
			r.Get("/quarters/{id}", h.Quarter.Get)
			r.Patch("/quarters/{id}/status", h.Quarter.UpdateStatus)

			// Sprints
			r.Get("/sprints", h.Sprint.List)
			r.Get("/sprints/{id}", h.Sprint.Get)
			r.Get("/sprints/{id}/checks", h.Sprint.GetIndividualChecks)
			r.Post("/sprints/{id}/checks", h.Sprint.UpsertIndividualCheck)
			r.Post("/sprints/{id}/lock", h.Sprint.Lock)
			r.Post("/sprints/{id}/unlock", h.Sprint.Unlock)

			// Goals
			r.Get("/goals", h.Goal.List)
			r.Post("/goals", h.Goal.Create)
			r.Get("/goals/sprint", h.Goal.ListSprintGoals)
			r.Post("/goals/sprint", h.Goal.UpsertSprintGoal)

			// Bonus
			r.Get("/bonus/calculations", h.Bonus.GetCalculations)
			r.Post("/bonus/calculations", h.Bonus.SaveCalculations)
			r.Post("/bonus/lock", h.Bonus.Lock)
			r.Post("/bonus/unlock", h.Bonus.Unlock)
			r.Get("/bonus/team-sprint-data", h.Bonus.GetTeamSprintData)

			// Finance
			r.Get("/finance", h.Finance.Get)
			r.Post("/finance", h.Finance.Upsert)

			// Settings
			r.Get("/settings", h.Settings.GetAll)
			r.Post("/settings/initialize", h.Settings.Initialize)
			r.Post("/settings/teams", h.Settings.CreateTeam)
			r.Put("/settings/teams/{id}", h.Settings.UpdateTeam)
			r.Delete("/settings/teams/{id}", h.Settings.DeleteTeam)
			r.Post("/settings/people", h.Settings.CreatePerson)
			r.Put("/settings/people/{id}", h.Settings.UpdatePerson)
			r.Delete("/settings/people/{id}", h.Settings.DeletePerson)
			r.Put("/settings/bonus-tiers", h.Settings.UpdateBonusTiers)
			r.Put("/settings/job-roles", h.Settings.UpdateJobRoleCriteria)
			r.Delete("/settings/job-roles", h.Settings.DeleteJobRole)
			r.Put("/settings/system", h.Settings.UpdateSystem)

			// Audit
			r.Get("/audit", h.Audit.List)

			// Drafts
			r.Get("/drafts", h.Draft.List)
			r.Post("/drafts", h.Draft.Create)
			r.Get("/drafts/{id}", h.Draft.Get)
			r.Put("/drafts/{id}", h.Draft.Update)
			r.Delete("/drafts/{id}", h.Draft.Delete)

			// Invitations
			r.Post("/invitations", h.Invite.Send)
			r.Get("/invitations", h.Invite.List)
			r.Post("/invitations/accept", h.Invite.Accept)
			r.Post("/invitations/{id}/resend", h.Invite.Resend)
			r.Delete("/invitations/{id}", h.Invite.Revoke)

			// Search
			r.Route("/search", func(r chi.Router) {
				r.Use(middleware.RequireWorkspaceID)
				r.Get("/", h.Search.Search)
			})

			// PM module
			r.Route("/pm", func(r chi.Router) {
				r.Use(middleware.RequireWorkspaceID)

				// Workflows
				r.Get("/workflows", h.PMWorkflow.List)
				r.Post("/workflows", h.PMWorkflow.Create)
				r.Get("/workflows/epic-states", h.PMWorkflow.ListEpicStates)
				r.Get("/workflows/{id}", h.PMWorkflow.Get)
				r.Put("/workflows/{id}", h.PMWorkflow.Update)
				r.Delete("/workflows/{id}", h.PMWorkflow.Delete)
				r.Post("/workflows/{id}/states", h.PMWorkflow.CreateState)
				r.Put("/workflows/{id}/states/{stateId}", h.PMWorkflow.UpdateState)
				r.Delete("/workflows/{id}/states/{stateId}", h.PMWorkflow.DeleteState)
				r.Put("/workflows/{id}/states/reorder", h.PMWorkflow.ReorderStates)

				// Views
				r.Get("/views", h.PMView.List)
				r.Post("/views", h.PMView.Create)
				r.Put("/views/{id}", h.PMView.Update)
				r.Delete("/views/{id}", h.PMView.Delete)

				// Labels
				r.Get("/labels", h.PMLabel.List)
				r.Post("/labels", h.PMLabel.Create)
				r.Put("/labels/{id}", h.PMLabel.Update)
				r.Delete("/labels/{id}", h.PMLabel.Delete)

				// Epics
				r.Get("/epics", h.PMEpic.List)
				r.Post("/epics", h.PMEpic.Create)
				r.Get("/epics/{id}", h.PMEpic.Get)
				r.Put("/epics/{id}", h.PMEpic.Update)
				r.Delete("/epics/{id}", h.PMEpic.Delete)
				r.Get("/epics/{id}/stories", h.PMEpic.ListStories)
				r.Put("/epics/{id}/health", h.PMEpic.UpdateHealth)

				// Sprints (PM)
				r.Get("/sprints", h.PMSprint.List)
				r.Post("/sprints", h.PMSprint.Create)
				r.Get("/sprints/{id}", h.PMSprint.Get)
				r.Put("/sprints/{id}", h.PMSprint.Update)
				r.Delete("/sprints/{id}", h.PMSprint.Delete)
				r.Get("/sprints/{id}/stories", h.PMSprint.ListStories)

				// Stories
				r.Get("/stories", h.PMStory.List)
				r.Post("/stories", h.PMStory.Create)
				r.Get("/stories/board", h.PMStory.ListBoard)
				r.Get("/stories/counts", h.PMStory.CountByState)
				r.Get("/stories/display/{displayID}", h.PMStory.GetByDisplayID)
				r.Get("/stories/{id}", h.PMStory.Get)
				r.Put("/stories/{id}", h.PMStory.Update)
				r.Delete("/stories/{id}", h.PMStory.Delete)
				r.Put("/stories/{id}/move", h.PMStory.Move)
				r.Put("/stories/{id}/reorder", h.PMStory.Reorder)
				r.Post("/stories/{id}/owners", h.PMStory.AddOwner)
				r.Delete("/stories/{id}/owners/{userId}", h.PMStory.RemoveOwner)
				r.Post("/stories/{id}/followers", h.PMStory.AddFollower)
				r.Delete("/stories/{id}/followers", h.PMStory.RemoveFollower)
				r.Post("/stories/{id}/labels", h.PMStory.AddLabel)
				r.Delete("/stories/{id}/labels/{labelId}", h.PMStory.RemoveLabel)
				r.Get("/stories/{id}/activity", h.PMStory.ListActivity)

				// Comments
				r.Get("/comments", h.PMComment.List)
				r.Post("/comments", h.PMComment.Create)
				r.Put("/comments/{id}", h.PMComment.Update)
				r.Delete("/comments/{id}", h.PMComment.Delete)

				// Attachments
				r.Post("/attachments", h.PMAttachment.Create)
				r.Patch("/attachments/{id}/confirm", h.PMAttachment.ConfirmUpload)
				r.Get("/attachments", h.PMAttachment.List)
				r.Delete("/attachments/{id}", h.PMAttachment.Delete)

				// Objectives
				r.Get("/objectives", h.PMObjective.List)
				r.Post("/objectives", h.PMObjective.Create)
				r.Get("/objectives/{id}", h.PMObjective.Get)
				r.Put("/objectives/{id}", h.PMObjective.Update)
				r.Delete("/objectives/{id}", h.PMObjective.Delete)
				r.Post("/objectives/{id}/teams", h.PMObjective.AddTeam)
				r.Delete("/objectives/{id}/teams/{teamId}", h.PMObjective.RemoveTeam)
				r.Post("/objectives/{id}/owners", h.PMObjective.AddOwner)
				r.Delete("/objectives/{id}/owners/{userId}", h.PMObjective.RemoveOwner)
				r.Post("/objectives/{id}/epics", h.PMObjective.AddEpic)
				r.Delete("/objectives/{id}/epics/{epicId}", h.PMObjective.RemoveEpic)
				r.Post("/objectives/{id}/key-results", h.PMObjective.CreateKeyResult)
				r.Put("/key-results/{id}", h.PMObjective.UpdateKeyResult)
				r.Delete("/key-results/{id}", h.PMObjective.DeleteKeyResult)

				// Checklist items
				r.Get("/stories/{id}/checklist", h.PMChecklistItem.List)
				r.Post("/stories/{id}/checklist", h.PMChecklistItem.Create)
				r.Put("/checklist-items/{id}", h.PMChecklistItem.Update)
				r.Delete("/checklist-items/{id}", h.PMChecklistItem.Delete)

				// External links
				r.Get("/stories/{id}/links", h.PMExternalLink.List)
				r.Post("/stories/{id}/links", h.PMExternalLink.Create)
				r.Put("/links/{id}", h.PMExternalLink.Update)
				r.Delete("/links/{id}", h.PMExternalLink.Delete)
			})
		})
	})

	return r
}
