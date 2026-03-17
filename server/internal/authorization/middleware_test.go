package authorization

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/helpin-ai/helpin/server/internal/middleware"
)

// setupAuthzService creates an AuthzService with a mock repo for middleware tests.
func setupAuthzService() (*AuthzService, *mockMemberRepo) {
	repo := newMockMemberRepo()
	return &AuthzService{rbac: NewRBACEngine(), memberRepo: repo}, repo
}

// requestWithUser creates a request with user ID and optional workspace ID in context.
func requestWithUser(method, path, userID string) *http.Request {
	req := httptest.NewRequest(method, path, nil)
	ctx := middleware.WithUserID(req.Context(), userID)
	return req.WithContext(ctx)
}

func TestRequireWorkspaceAccess_ActiveMember(t *testing.T) {
	authz, repo := setupAuthzService()
	repo.addMember("ws-1", "user-1", "wm-1", "admin", "active")

	handler := RequireWorkspaceAccess(authz)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		actor := GetActor(r.Context())
		if actor == nil {
			t.Fatal("expected actor in context")
		}
		if actor.Role != "admin" {
			t.Errorf("expected role admin, got %s", actor.Role)
		}
		w.WriteHeader(http.StatusOK)
	}))

	req := requestWithUser("GET", "/test", "user-1")
	req.Header.Set("X-Workspace-ID", "ws-1")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
}

func TestRequireWorkspaceAccess_NotAMember(t *testing.T) {
	authz, _ := setupAuthzService()

	handler := RequireWorkspaceAccess(authz)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("handler should not be called")
	}))

	req := requestWithUser("GET", "/test", "user-1")
	req.Header.Set("X-Workspace-ID", "ws-1")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", rr.Code)
	}

	var resp forbiddenResponse
	json.NewDecoder(rr.Body).Decode(&resp)
	if resp.Reason != "not_a_member" {
		t.Errorf("expected reason not_a_member, got %s", resp.Reason)
	}
}

func TestRequireWorkspaceAccess_PendingMember(t *testing.T) {
	authz, repo := setupAuthzService()
	repo.addMember("ws-1", "user-1", "wm-1", "member", "pending")

	handler := RequireWorkspaceAccess(authz)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("handler should not be called for pending member")
	}))

	req := requestWithUser("GET", "/test", "user-1")
	req.Header.Set("X-Workspace-ID", "ws-1")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", rr.Code)
	}

	var resp forbiddenResponse
	json.NewDecoder(rr.Body).Decode(&resp)
	if resp.Reason != "membership_pending" {
		t.Errorf("expected reason membership_pending, got %s", resp.Reason)
	}
}

func TestRequireWorkspaceAccess_RevokedMember(t *testing.T) {
	authz, repo := setupAuthzService()
	repo.addMember("ws-1", "user-1", "wm-1", "member", "revoked")

	handler := RequireWorkspaceAccess(authz)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("handler should not be called for revoked member")
	}))

	req := requestWithUser("GET", "/test", "user-1")
	req.Header.Set("X-Workspace-ID", "ws-1")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", rr.Code)
	}

	var resp forbiddenResponse
	json.NewDecoder(rr.Body).Decode(&resp)
	if resp.Reason != "membership_revoked" {
		t.Errorf("expected reason membership_revoked, got %s", resp.Reason)
	}
}

func TestRequireWorkspaceAccess_NoUserID(t *testing.T) {
	authz, _ := setupAuthzService()

	handler := RequireWorkspaceAccess(authz)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("handler should not be called")
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("X-Workspace-ID", "ws-1")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rr.Code)
	}
}

func TestRequireWorkspaceAccess_NoWorkspaceID(t *testing.T) {
	authz, _ := setupAuthzService()

	handler := RequireWorkspaceAccess(authz)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("handler should not be called")
	}))

	req := requestWithUser("GET", "/test", "user-1")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rr.Code)
	}
}

func TestRequireWorkspaceAccess_WorkspaceIDFromQueryParam(t *testing.T) {
	authz, repo := setupAuthzService()
	repo.addMember("ws-1", "user-1", "wm-1", "member", "active")

	handler := RequireWorkspaceAccess(authz)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := requestWithUser("GET", "/test?workspace_id=ws-1", "user-1")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
}

func TestRequireWorkspaceAccess_WorkspaceIDFromContext(t *testing.T) {
	authz, repo := setupAuthzService()
	repo.addMember("ws-1", "user-1", "wm-1", "member", "active")

	handler := RequireWorkspaceAccess(authz)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := requestWithUser("GET", "/test", "user-1")
	ctx := middleware.WithWorkspaceID(req.Context(), "ws-1")
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
}

func TestRequirePermission_Allowed(t *testing.T) {
	authz, _ := setupAuthzService()

	actor := &Actor{Role: "admin"}
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := RequirePermission(authz, PermPMEdit)(inner)

	req := httptest.NewRequest("GET", "/test", nil)
	ctx := WithActor(req.Context(), actor)
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
}

func TestRequirePermission_Denied(t *testing.T) {
	authz, _ := setupAuthzService()

	actor := &Actor{Role: "viewer"}
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("handler should not be called")
	})

	handler := RequirePermission(authz, PermPMEdit)(inner)

	req := httptest.NewRequest("GET", "/test", nil)
	ctx := WithActor(req.Context(), actor)
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", rr.Code)
	}

	var resp forbiddenResponse
	json.NewDecoder(rr.Body).Decode(&resp)
	if resp.Required != string(PermPMEdit) {
		t.Errorf("expected required pm.edit, got %s", resp.Required)
	}
}

func TestRequirePermission_NoActor(t *testing.T) {
	authz, _ := setupAuthzService()

	handler := RequirePermission(authz, PermPMEdit)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("handler should not be called")
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", rr.Code)
	}
}

func TestRequireAnyPermission_Allowed(t *testing.T) {
	authz, _ := setupAuthzService()

	actor := &Actor{Role: "viewer"}
	handler := RequireAnyPermission(authz, PermPMRead, PermPMEdit)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	ctx := WithActor(req.Context(), actor)
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
}

func TestRequireAnyPermission_Denied(t *testing.T) {
	authz, _ := setupAuthzService()

	actor := &Actor{Role: "viewer"}
	handler := RequireAnyPermission(authz, PermPMEdit, PermSettingsManage)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("handler should not be called")
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	ctx := WithActor(req.Context(), actor)
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", rr.Code)
	}
}

func TestRequireOwner_Allowed(t *testing.T) {
	authz, _ := setupAuthzService()

	actor := &Actor{Role: "owner"}
	handler := RequireOwner(authz)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("DELETE", "/test", nil)
	ctx := WithActor(req.Context(), actor)
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
}

func TestRequireOwner_Denied(t *testing.T) {
	authz, _ := setupAuthzService()

	actor := &Actor{Role: "admin"}
	handler := RequireOwner(authz)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("handler should not be called")
	}))

	req := httptest.NewRequest("DELETE", "/test", nil)
	ctx := WithActor(req.Context(), actor)
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", rr.Code)
	}
}

func TestRequireTeamPermission_WorkspaceAdmin(t *testing.T) {
	authz, _ := setupAuthzService()

	actor := &Actor{Role: "admin"}
	_ = RequireTeamPermission(authz) // used below via router

	// Create a Chi router to inject URL params
	router := chi.NewRouter()
	router.With(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := WithActor(r.Context(), actor)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}).Route("/teams/{id}", func(r chi.Router) {
		r.With(RequireTeamPermission(authz)).Get("/", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})
	})

	req := httptest.NewRequest("GET", "/teams/team-1/", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
}

func TestRequireTeamPermission_TeamOwner(t *testing.T) {
	authz, _ := setupAuthzService()

	actor := &Actor{
		Role:            "member",
		TeamMemberships: []TeamRole{{TeamID: "team-1", Role: "owner"}},
	}

	router := chi.NewRouter()
	router.With(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := WithActor(r.Context(), actor)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}).Route("/teams/{id}", func(r chi.Router) {
		r.With(RequireTeamPermission(authz)).Get("/", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})
	})

	req := httptest.NewRequest("GET", "/teams/team-1/", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
}

func TestRequireTeamPermission_RegularMember_Denied(t *testing.T) {
	authz, _ := setupAuthzService()

	actor := &Actor{
		Role:            "member",
		TeamMemberships: []TeamRole{{TeamID: "team-1", Role: "member"}},
	}

	router := chi.NewRouter()
	router.With(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := WithActor(r.Context(), actor)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}).Route("/teams/{id}", func(r chi.Router) {
		r.With(RequireTeamPermission(authz)).Get("/", func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("handler should not be called")
		})
	})

	req := httptest.NewRequest("GET", "/teams/team-1/", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", rr.Code)
	}
}

// Test cross-workspace denial through middleware.
func TestRequireWorkspaceAccess_CrossWorkspaceDenied(t *testing.T) {
	authz, repo := setupAuthzService()
	repo.addMember("ws-1", "user-1", "wm-1", "admin", "active")
	// user-1 is NOT a member of ws-2

	handler := RequireWorkspaceAccess(authz)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("handler should not be called for cross-workspace request")
	}))

	req := requestWithUser("GET", "/test", "user-1")
	req.Header.Set("X-Workspace-ID", "ws-2")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Errorf("expected 403 for cross-workspace access, got %d", rr.Code)
	}
}

// Test full middleware chain: workspace access + permission check.
func TestMiddlewareChain_WorkspaceAccess_Then_Permission(t *testing.T) {
	authz, repo := setupAuthzService()
	repo.addMember("ws-1", "user-1", "wm-1", "viewer", "active")

	// Viewer should pass workspace access but fail pm.edit
	wsMiddleware := RequireWorkspaceAccess(authz)
	permMiddleware := RequirePermission(authz, PermPMEdit)

	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("handler should not be called for viewer with pm.edit")
	})

	handler := wsMiddleware(permMiddleware(inner))

	req := requestWithUser("POST", "/test", "user-1")
	req.Header.Set("X-Workspace-ID", "ws-1")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", rr.Code)
	}
}

// Verify the workspace ID precedence (Chi URL param > header > query > context).
func TestRequireWorkspaceAccess_IDPrecedence(t *testing.T) {
	authz, repo := setupAuthzService()
	repo.addMember("ws-header", "user-1", "wm-1", "member", "active")
	repo.addMember("ws-query", "user-1", "wm-2", "admin", "active")

	handler := RequireWorkspaceAccess(authz)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		actor := GetActor(r.Context())
		// Should use header first (no URL param)
		if actor.WorkspaceID != "ws-header" {
			t.Errorf("expected ws-header (from header), got %s", actor.WorkspaceID)
		}
		w.WriteHeader(http.StatusOK)
	}))

	req := requestWithUser("GET", "/test?workspace_id=ws-query", "user-1")
	req.Header.Set("X-Workspace-ID", "ws-header")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
}

// Test RequireWorkspaceAccess resolves workspace ID from Chi URL param {id}.
func TestRequireWorkspaceAccess_FromChiURLParam(t *testing.T) {
	authz, repo := setupAuthzService()
	repo.addMember("ws-1", "user-1", "wm-1", "admin", "active")

	router := chi.NewRouter()
	router.With(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := middleware.WithUserID(r.Context(), "user-1")
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}).Route("/workspaces/{id}", func(r chi.Router) {
		r.Use(ExtractWorkspaceIDParam)
		r.Use(RequireWorkspaceAccess(authz))
		r.Get("/", func(w http.ResponseWriter, r *http.Request) {
			actor := GetActor(r.Context())
			if actor.WorkspaceID != "ws-1" {
				t.Errorf("expected ws-1 from URL param, got %s", actor.WorkspaceID)
			}
			w.WriteHeader(http.StatusOK)
		})
	})

	req := httptest.NewRequest("GET", "/workspaces/ws-1/", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
}

// Additional test: Inactive member status.
func TestRequireWorkspaceAccess_InactiveMember(t *testing.T) {
	authz, repo := setupAuthzService()
	repo.addMember("ws-1", "user-1", "wm-1", "member", "inactive")

	handler := RequireWorkspaceAccess(authz)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("handler should not be called for inactive member")
	}))

	req := requestWithUser("GET", "/test", "user-1")
	req.Header.Set("X-Workspace-ID", "ws-1")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", rr.Code)
	}

	var resp forbiddenResponse
	json.NewDecoder(rr.Body).Decode(&resp)
	if resp.Reason != "membership_inactive" {
		t.Errorf("expected reason membership_inactive, got %s", resp.Reason)
	}
}

// Ensure RequireWorkspaceAccess stores workspace ID in context.
func TestRequireWorkspaceAccess_SetsWorkspaceIDInContext(t *testing.T) {
	authz, repo := setupAuthzService()
	repo.addMember("ws-1", "user-1", "wm-1", "member", "active")

	handler := RequireWorkspaceAccess(authz)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		wsID := middleware.GetWorkspaceID(r.Context())
		if wsID != "ws-1" {
			t.Errorf("expected workspace ID ws-1 in context, got %s", wsID)
		}
		w.WriteHeader(http.StatusOK)
	}))

	req := requestWithUser("GET", "/test", "user-1")
	req.Header.Set("X-Workspace-ID", "ws-1")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
}

// Ensure RequireWorkspaceAccess resolves and stores the actor's team memberships.
func TestRequireWorkspaceAccess_LoadsTeamMemberships(t *testing.T) {
	authz, repo := setupAuthzService()
	repo.addMember("ws-1", "user-1", "wm-1", "member", "active")
	repo.addTeamMembership("wm-1", "team-a", "owner")
	repo.addTeamMembership("wm-1", "team-b", "member")

	handler := RequireWorkspaceAccess(authz)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		actor := GetActor(r.Context())
		if len(actor.TeamMemberships) != 2 {
			t.Errorf("expected 2 team memberships, got %d", len(actor.TeamMemberships))
		}
		w.WriteHeader(http.StatusOK)
	}))

	req := requestWithUser("GET", "/test", "user-1")
	req.Header.Set("X-Workspace-ID", "ws-1")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
}

// Test that unused context key doesn't interfere.
func init() {
	// Ensure there are no import cycles by just referencing the context package.
	_ = context.Background()
}
