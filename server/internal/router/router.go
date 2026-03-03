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
	Health    *handler.HealthHandler
	Auth      *handler.AuthHandler
	Workspace *handler.WorkspaceHandler
	Quarter   *handler.QuarterHandler
	Sprint    *handler.SprintHandler
	Goal      *handler.GoalHandler
	Bonus     *handler.BonusHandler
	Finance   *handler.FinanceHandler
	Settings  *handler.SettingsHandler
	Audit     *handler.AuditHandler
	Draft     *handler.DraftHandler
	Invite    *handler.InviteHandler
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

			// Invite (stub)
			r.Post("/invite", h.Invite.Send)
		})
	})

	return r
}
