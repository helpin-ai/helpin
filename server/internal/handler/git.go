package handler

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/helpin-ai/helpin/server/internal/middleware"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/service"
)

// GitHandler handles git integration HTTP endpoints.
type GitHandler struct {
	gitService *service.GitService
}

// NewGitHandler creates a new GitHandler.
func NewGitHandler(gitService *service.GitService) *GitHandler {
	return &GitHandler{gitService: gitService}
}

// ListIntegrations handles GET /api/git/integrations.
func (h *GitHandler) ListIntegrations(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	if workspaceID == "" {
		writeError(w, http.StatusBadRequest, "workspace_id is required")
		return
	}

	integrations, err := h.gitService.ListIntegrations(r.Context(), workspaceID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if integrations == nil {
		integrations = []model.GitIntegration{}
	}
	writeJSON(w, http.StatusOK, integrations)
}

// CreateIntegration handles POST /api/git/integrations.
func (h *GitHandler) CreateIntegration(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	actorID := middleware.GetUserID(r.Context())

	var req model.CreateGitIntegrationRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	req.WorkspaceID = workspaceID

	integration, err := h.gitService.CreateIntegration(r.Context(), req, actorID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, integration)
}

// GetStoryGitLinks handles GET /api/pm/stories/{id}/git-links.
func (h *GitHandler) GetStoryGitLinks(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	storyID := chi.URLParam(r, "id")

	links, err := h.gitService.GetStoryGitLinks(r.Context(), workspaceID, storyID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if links == nil {
		links = []model.StoryGitLink{}
	}
	writeJSON(w, http.StatusOK, links)
}

// CreateBranch handles POST /api/pm/stories/{id}/create-branch.
func (h *GitHandler) CreateBranch(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	storyID := chi.URLParam(r, "id")
	actorID := middleware.GetUserID(r.Context())

	var req model.CreateBranchRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	link, err := h.gitService.CreateBranch(r.Context(), workspaceID, storyID, req, actorID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, link)
}

// Webhook handles POST /api/git/webhook.
// This is a public endpoint that accepts webhooks from GitHub/GitLab.
func (h *GitHandler) Webhook(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20)) // 1MB limit
	if err != nil {
		writeError(w, http.StatusBadRequest, "failed to read body")
		return
	}
	defer r.Body.Close()

	// Determine provider from headers.
	var provider string
	if r.Header.Get("X-GitHub-Event") != "" {
		provider = "github"
	} else if r.Header.Get("X-Gitlab-Event") != "" {
		provider = "gitlab"
	} else {
		writeError(w, http.StatusBadRequest, "unknown webhook provider")
		return
	}

	// Parse minimal fields to route the event.
	var payload map[string]interface{}
	if err := json.Unmarshal(body, &payload); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON payload")
		return
	}

	// Extract workspace from query param (webhook URL includes it).
	workspaceID := r.URL.Query().Get("workspace_id")
	if workspaceID == "" {
		writeError(w, http.StatusBadRequest, "workspace_id query param required")
		return
	}

	if provider == "github" {
		event := r.Header.Get("X-GitHub-Event")
		switch event {
		case "push":
			h.handleGitHubPush(r, w, workspaceID, payload)
		case "pull_request":
			h.handleGitHubPR(r, w, workspaceID, payload)
		default:
			writeJSON(w, http.StatusOK, map[string]string{"status": "ignored"})
		}
	} else {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ignored"})
	}
}

func (h *GitHandler) handleGitHubPush(r *http.Request, w http.ResponseWriter, workspaceID string, payload map[string]interface{}) {
	repo, _ := nestedString(payload, "repository", "full_name")
	ref, _ := payload["ref"].(string)
	branch := ""
	if len(ref) > 11 { // refs/heads/
		branch = ref[11:]
	}
	headCommit, _ := payload["head_commit"].(map[string]interface{})
	commitSHA := ""
	if headCommit != nil {
		commitSHA, _ = headCommit["id"].(string)
	}

	if repo == "" || branch == "" {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ignored"})
		return
	}

	if err := h.gitService.ProcessWebhookPush(r.Context(), workspaceID, repo, branch, commitSHA); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "processed"})
}

func (h *GitHandler) handleGitHubPR(r *http.Request, w http.ResponseWriter, workspaceID string, payload map[string]interface{}) {
	pr, ok := payload["pull_request"].(map[string]interface{})
	if !ok {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ignored"})
		return
	}

	repo, _ := nestedString(payload, "repository", "full_name")
	prNumber := int(pr["number"].(float64))
	prURL, _ := pr["html_url"].(string)

	prStatus := "open"
	if merged, ok := pr["merged"].(bool); ok && merged {
		prStatus = "merged"
	} else if state, ok := pr["state"].(string); ok && state == "closed" {
		prStatus = "closed"
	}

	branch := ""
	if head, ok := pr["head"].(map[string]interface{}); ok {
		branch, _ = head["ref"].(string)
	}

	if repo == "" {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ignored"})
		return
	}

	if err := h.gitService.ProcessWebhookPR(r.Context(), workspaceID, repo, prNumber, prURL, prStatus, branch); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "processed"})
}

func nestedString(m map[string]interface{}, keys ...string) (string, bool) {
	current := m
	for i, key := range keys {
		if i == len(keys)-1 {
			val, ok := current[key].(string)
			return val, ok
		}
		next, ok := current[key].(map[string]interface{})
		if !ok {
			return "", false
		}
		current = next
	}
	return "", false
}
