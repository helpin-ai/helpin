package authorization

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/helpin-ai/helpin/server/internal/middleware"
	"github.com/helpin-ai/helpin/server/internal/model"
)

// forbiddenResponse is the standard 403 response body.
type forbiddenResponse struct {
	Error    string `json:"error"`
	Reason   string `json:"reason"`
	Required string `json:"required,omitempty"`
}

func writeForbidden(w http.ResponseWriter, reason, required string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusForbidden)
	json.NewEncoder(w).Encode(forbiddenResponse{
		Error:    "forbidden",
		Reason:   reason,
		Required: required,
	})
}

// SlugResolver resolves a workspace slug to a workspace ID.
type SlugResolver func(ctx context.Context, slug string) (string, error)

// ExtractWorkspaceIDParam reads Chi URL param {id} and stores it as the
// workspace ID in context. Use this BEFORE RequireWorkspaceAccess on routes
// where {id} is actually a workspace ID (e.g. /workspaces/{id}).
func ExtractWorkspaceIDParam(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if wsID := chi.URLParam(r, "id"); wsID != "" {
			ctx := middleware.WithWorkspaceID(r.Context(), wsID)
			next.ServeHTTP(w, r.WithContext(ctx))
			return
		}
		next.ServeHTTP(w, r)
	})
}

// ResolveWorkspaceSlug reads Chi URL param {slug}, resolves it to a workspace
// ID, and stores the workspace ID in context. Use BEFORE RequireWorkspaceAccess
// on slug-based routes (e.g. /workspaces/by-slug/{slug}).
func ResolveWorkspaceSlug(resolver SlugResolver) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			slug := chi.URLParam(r, "slug")
			if slug == "" {
				http.Error(w, "workspace slug required", http.StatusBadRequest)
				return
			}
			if resolver == nil {
				http.Error(w, "workspace resolver unavailable", http.StatusInternalServerError)
				return
			}
			wsID, err := resolver(r.Context(), slug)
			if err != nil {
				http.Error(w, "workspace not found", http.StatusNotFound)
				return
			}
			if wsID == "" {
				http.Error(w, "workspace not found", http.StatusNotFound)
				return
			}
			ctx := middleware.WithWorkspaceID(r.Context(), wsID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// RequireWorkspaceAccess resolves the actor's workspace membership and stores
// the Actor in context. Rejects requests from non-members or non-active members.
//
// Workspace ID resolution precedence (safe sources only):
// 1. Context value (from RequireWorkspaceID or ExtractWorkspaceIDParam)
// 2. X-Workspace-ID header
// 3. workspace_id query parameter
//
// This middleware deliberately does NOT read Chi URL params like {id} or {slug}
// because those may refer to entity IDs (team, quarter, ticket, etc.) on
// non-workspace routes. Use ExtractWorkspaceIDParam or ResolveWorkspaceSlug
// before this middleware when the URL param is a workspace identifier.
func RequireWorkspaceAccess(authz *AuthzService) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()
			userID := middleware.GetUserID(ctx)
			if userID == "" {
				http.Error(w, "authentication required", http.StatusUnauthorized)
				return
			}

			// Resolve workspace ID from safe sources only.
			workspaceID := middleware.GetWorkspaceID(ctx)
			if workspaceID == "" {
				workspaceID = r.Header.Get("X-Workspace-ID")
			}
			if workspaceID == "" {
				workspaceID = r.URL.Query().Get("workspace_id")
			}
			if workspaceID == "" {
				http.Error(w, "workspace identification required", http.StatusBadRequest)
				return
			}

			actor, err := authz.ResolveActor(ctx, workspaceID, userID)
			if err != nil {
				switch {
				case errors.Is(err, ErrNotAMember):
					writeForbidden(w, "not_a_member", "")
				case errors.Is(err, ErrMembershipPending):
					writeForbidden(w, "membership_pending", "")
				case errors.Is(err, ErrMembershipRevoked):
					writeForbidden(w, "membership_revoked", "")
				case errors.Is(err, ErrMembershipInactive):
					writeForbidden(w, "membership_inactive", "")
				default:
					http.Error(w, "failed to verify workspace access", http.StatusInternalServerError)
				}
				return
			}

			// Store actor and workspace ID in context
			ctx = WithActor(ctx, actor)
			ctx = middleware.WithWorkspaceID(ctx, workspaceID)

			if !mfaRouteExempt(r.URL.Path) {
				policy, err := authz.WorkspaceMFAPolicy(ctx, workspaceID, userID)
				if err != nil {
					http.Error(w, "failed to verify workspace security policy", http.StatusInternalServerError)
					return
				}
				claims := middleware.ClaimsFrom(ctx)
				if policy.EnforceTwoFactor && (claims == nil || !claims.MFASatisfied) {
					writeForbidden(w, "mfa_required", "mfa")
					return
				}
			}

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func mfaRouteExempt(path string) bool {
	if strings.HasPrefix(path, "/api/auth/2fa/") || strings.HasPrefix(path, "/api/auth/passkey/") {
		return true
	}
	if path == "/api/auth/me" {
		return true
	}
	if strings.HasPrefix(path, "/api/workspaces/by-slug/") {
		return true
	}
	if strings.HasPrefix(path, "/api/workspaces/") && strings.HasSuffix(path, "/me") {
		return true
	}
	return false
}

// RequirePermission checks that the actor has the specified permission.
// Must be applied after RequireWorkspaceAccess.
func RequirePermission(authz *AuthzService, perm Permission) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			actor := GetActor(r.Context())
			if actor == nil {
				http.Error(w, "authorization context missing", http.StatusInternalServerError)
				return
			}
			if !authz.Can(actor, perm) {
				writeForbidden(w, "insufficient_permission", string(perm))
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// RequireModuleAccess checks whether the actor can enter the requested module.
// Must be applied after RequireWorkspaceAccess.
func RequireModuleAccess(authz *AuthzService, module model.ModuleID) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			actor := GetActor(r.Context())
			if actor == nil {
				http.Error(w, "authorization context missing", http.StatusInternalServerError)
				return
			}
			allowed, err := authz.CanAccessModule(r.Context(), actor, module)
			if err != nil {
				http.Error(w, "failed to verify module access", http.StatusInternalServerError)
				return
			}
			if !allowed {
				writeForbidden(w, "module_access_denied", string(module))
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// RequireAnyPermission checks that the actor has at least one of the listed permissions.
func RequireAnyPermission(authz *AuthzService, perms ...Permission) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			actor := GetActor(r.Context())
			if actor == nil {
				http.Error(w, "authorization context missing", http.StatusInternalServerError)
				return
			}
			if !authz.CanAny(actor, perms...) {
				writeForbidden(w, "insufficient_permission", "")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// RequireTeamPermission extracts a team ID from the URL path and checks
// team-level management permission. Grants access if the actor is a workspace
// owner/admin OR is a team owner for the specific team.
func RequireTeamPermission(authz *AuthzService) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			actor := GetActor(r.Context())
			if actor == nil {
				http.Error(w, "authorization context missing", http.StatusInternalServerError)
				return
			}

			teamID := chi.URLParam(r, "id")
			if teamID == "" {
				teamID = chi.URLParam(r, "teamId")
			}
			if teamID == "" {
				http.Error(w, "team ID required", http.StatusBadRequest)
				return
			}

			if !authz.CanManageTeam(actor, teamID) {
				writeForbidden(w, "insufficient_permission", string(PermTeamManage))
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// RequireOwner checks that the actor is the workspace owner.
func RequireOwner(authz *AuthzService) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			actor := GetActor(r.Context())
			if actor == nil {
				http.Error(w, "authorization context missing", http.StatusInternalServerError)
				return
			}
			if !authz.IsOwnerOnly(actor) {
				writeForbidden(w, "owner_required", string(PermWorkspaceDelete))
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
