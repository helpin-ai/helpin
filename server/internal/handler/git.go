package handler

import (
	"encoding/json"
	"io"
	"net/http"
	"strconv"

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

// GetGitHubInstallURL handles GET /api/git/github/install-url.
func (h *GitHandler) GetGitHubInstallURL(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	actorID := middleware.GetUserID(r.Context())

	installURL, action, err := h.gitService.GetGitHubInstallURL(r.Context(), workspaceID, actorID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, model.GitHubInstallURLResponse{InstallURL: installURL, Action: action})
}

// GitHubCallback handles GET /api/git/github/callback.
func (h *GitHandler) GitHubCallback(w http.ResponseWriter, r *http.Request) {
	redirectURL, err := h.gitService.CompleteGitHubInstall(
		r.Context(),
		r.URL.Query().Get("state"),
		r.URL.Query().Get("installation_id"),
	)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	http.Redirect(w, r, redirectURL, http.StatusFound)
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

// SyncRepositories handles POST /api/git/integrations/{id}/sync.
func (h *GitHandler) SyncRepositories(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	integrationID := chi.URLParam(r, "id")
	actorID := middleware.GetUserID(r.Context())

	repos, err := h.gitService.SyncRepositories(r.Context(), workspaceID, integrationID, actorID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if repos == nil {
		repos = []model.GitRepository{}
	}
	writeJSON(w, http.StatusOK, repos)
}

// ListRepositories handles GET /api/git/repositories.
func (h *GitHandler) ListRepositories(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	if workspaceID == "" {
		writeError(w, http.StatusBadRequest, "workspace_id is required")
		return
	}

	var (
		repos []model.GitRepository
		err   error
	)
	if r.URL.Query().Get("all") == "true" {
		repos, err = h.gitService.ListRepositoryCatalog(r.Context(), workspaceID)
	} else {
		repos, err = h.gitService.ListRepositories(r.Context(), workspaceID)
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if repos == nil {
		repos = []model.GitRepository{}
	}
	writeJSON(w, http.StatusOK, repos)
}

// UpdateRepository handles PUT /api/git/repositories/{id}.
func (h *GitHandler) UpdateRepository(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	repoID := chi.URLParam(r, "id")
	actorID := middleware.GetUserID(r.Context())

	var req model.UpdateGitRepositoryRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Selected == nil {
		writeError(w, http.StatusBadRequest, "selected is required")
		return
	}

	repo, err := h.gitService.UpdateRepositorySelection(r.Context(), workspaceID, repoID, *req.Selected, actorID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, repo)
}

// GetTaskGitLinks handles GET /api/pm/tasks/{id}/git-links.
func (h *GitHandler) GetTaskGitLinks(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	storyID := chi.URLParam(r, "id")

	links, err := h.gitService.GetTaskGitLinks(r.Context(), workspaceID, storyID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if links == nil {
		links = []model.TaskGitLink{}
	}
	writeJSON(w, http.StatusOK, links)
}

// GetTaskDeliveryTarget handles GET /api/pm/tasks/{id}/delivery-target.
func (h *GitHandler) GetTaskDeliveryTarget(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	storyID := chi.URLParam(r, "id")

	target, err := h.gitService.GetTaskDeliveryTarget(r.Context(), workspaceID, storyID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, target)
}

// UpdateTaskDeliveryTarget handles PUT /api/pm/tasks/{id}/delivery-target.
func (h *GitHandler) UpdateTaskDeliveryTarget(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	storyID := chi.URLParam(r, "id")
	actorID := middleware.GetUserID(r.Context())

	var req model.UpdateTaskDeliveryTargetRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	target, err := h.gitService.UpdateTaskDeliveryTarget(r.Context(), workspaceID, storyID, req, actorID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, target)
}

// CreateBranch handles POST /api/pm/tasks/{id}/create-branch.
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

	if provider == "github" {
		workspaceID := r.URL.Query().Get("workspace_id")
		if installationID, ok := nestedNumber(payload, "installation", "id"); ok {
			resolvedWorkspaceID, err := h.gitService.ResolveGitHubWebhookWorkspace(
				r.Context(),
				intString(installationID),
				body,
				r.Header.Get("X-Hub-Signature-256"),
			)
			if err != nil {
				writeError(w, http.StatusUnauthorized, err.Error())
				return
			}
			workspaceID = resolvedWorkspaceID
		}
		if workspaceID == "" {
			writeError(w, http.StatusBadRequest, "workspace_id or installation match required")
			return
		}

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

	prTitle, _ := pr["title"].(string)

	if err := h.gitService.ProcessWebhookPR(r.Context(), workspaceID, repo, prNumber, prTitle, prURL, prStatus, branch); err != nil {
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

func nestedNumber(m map[string]interface{}, keys ...string) (float64, bool) {
	current := m
	for i, key := range keys {
		if i == len(keys)-1 {
			val, ok := current[key].(float64)
			return val, ok
		}
		next, ok := current[key].(map[string]interface{})
		if !ok {
			return 0, false
		}
		current = next
	}
	return 0, false
}

func intString(value float64) string {
	return strconv.FormatInt(int64(value), 10)
}
