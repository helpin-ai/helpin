package middleware

import "net/http"

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
