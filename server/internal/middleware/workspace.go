package middleware

import (
	"context"
	"net/http"
)

// MembershipChecker looks up a user's role in a workspace.
// Returns the role string (e.g. "owner", "admin", "manager", "member") or
// empty string if the user is not an active member.
type MembershipChecker func(ctx context.Context, workspaceID, userID string) (string, error)

// RequireWorkspaceID ensures workspace context is available in PM routes.
// Priority: X-Workspace-ID header, then workspace_id query param.
func RequireWorkspaceID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		workspaceID := r.Header.Get("X-Workspace-ID")
		if workspaceID == "" {
			workspaceID = r.URL.Query().Get("workspace_id")
		}
		if workspaceID == "" {
			http.Error(w, "X-Workspace-ID header (or workspace_id query param) is required", http.StatusBadRequest)
			return
		}
		ctx := WithWorkspaceID(r.Context(), workspaceID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// RequireWorkspaceMembership verifies the authenticated user is an active
// member of the workspace identified by the X-Workspace-ID context value.
// Must be applied after RequireAuth and RequireWorkspaceID.
func RequireWorkspaceMembership(check MembershipChecker) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()
			workspaceID := GetWorkspaceID(ctx)
			userID := GetUserID(ctx)
			if workspaceID == "" || userID == "" {
				http.Error(w, "workspace and user context required", http.StatusUnauthorized)
				return
			}
			role, err := check(ctx, workspaceID, userID)
			if err != nil {
				http.Error(w, "failed to verify workspace membership", http.StatusInternalServerError)
				return
			}
			if role == "" {
				http.Error(w, "not a member of this workspace", http.StatusForbidden)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
