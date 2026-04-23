package handler

import (
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"

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
	forceInstall := strings.EqualFold(strings.TrimSpace(r.URL.Query().Get("force_install")), "true")

	installURL, action, integrationID, err := h.gitService.GetGitHubInstallURL(r.Context(), workspaceID, actorID, forceInstall)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, model.GitHubInstallURLResponse{InstallURL: installURL, Action: action, IntegrationID: integrationID})
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

// DeleteIntegration handles DELETE /api/git/integrations/{id}.
func (h *GitHandler) DeleteIntegration(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	integrationID := chi.URLParam(r, "id")
	actorID := middleware.GetUserID(r.Context())

	if err := h.gitService.DeleteIntegration(r.Context(), workspaceID, integrationID, actorID); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// GetIntegration handles GET /api/git/integrations/{id}.
func (h *GitHandler) GetIntegration(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	integrationID := chi.URLParam(r, "id")

	detail, err := h.gitService.GetIntegrationDetail(r.Context(), workspaceID, integrationID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, detail)
}

// ListAvailableRepos handles GET /api/git/integrations/{id}/available-repos.
func (h *GitHandler) ListAvailableRepos(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	integrationID := chi.URLParam(r, "id")
	actorID := middleware.GetUserID(r.Context())

	repos, err := h.gitService.ListAvailableRepos(r.Context(), workspaceID, integrationID, actorID)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "forbidden") {
			writeError(w, http.StatusForbidden, err.Error())
			return
		}
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if repos == nil {
		repos = []model.GitAvailableRepo{}
	}
	writeJSON(w, http.StatusOK, repos)
}

// WireRepositories handles POST /api/git/integrations/{id}/repositories.
func (h *GitHandler) WireRepositories(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	integrationID := chi.URLParam(r, "id")
	actorID := middleware.GetUserID(r.Context())

	var req model.WireGitRepositoriesRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if strings.TrimSpace(req.WorkspaceID) == "" {
		req.WorkspaceID = workspaceID
	}

	repos, conflicts, err := h.gitService.WireRepositories(r.Context(), workspaceID, integrationID, req, actorID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if conflicts != nil && len(conflicts.Conflicts) > 0 {
		writeJSON(w, http.StatusConflict, conflicts)
		return
	}
	if repos == nil {
		repos = []model.GitRepository{}
	}
	writeJSON(w, http.StatusOK, model.WireGitRepositoriesResponse{Repositories: repos})
}

// UnwireRepository handles DELETE /api/git/integrations/{id}/repositories/{repoId}.
func (h *GitHandler) UnwireRepository(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	integrationID := chi.URLParam(r, "id")
	repoID := chi.URLParam(r, "repoId")
	actorID := middleware.GetUserID(r.Context())

	if err := h.gitService.UnwireRepository(r.Context(), workspaceID, integrationID, repoID, actorID); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
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

// ListRepositoryBranches handles GET /api/git/repositories/{id}/branches.
func (h *GitHandler) ListRepositoryBranches(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	repoID := chi.URLParam(r, "id")

	branches, err := h.gitService.ListRepositoryBranches(r.Context(), workspaceID, repoID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if branches == nil {
		branches = []model.GitBranch{}
	}
	writeJSON(w, http.StatusOK, branches)
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
		var integration *model.GitIntegration
		if installationID, ok := nestedNumber(payload, "installation", "id"); ok {
			resolvedIntegration, err := h.gitService.ResolveGitHubWebhookIntegration(
				r.Context(),
				intString(installationID),
				body,
				r.Header.Get("X-Hub-Signature-256"),
			)
			if err != nil {
				writeError(w, http.StatusUnauthorized, err.Error())
				return
			}
			integration = resolvedIntegration
		}

		event := r.Header.Get("X-GitHub-Event")
		switch event {
		case "installation":
			h.handleGitHubInstallation(r, w, integration, payload)
		case "installation_repositories":
			h.handleGitHubInstallationRepositories(r, w, integration, payload)
		case "push":
			h.handleGitHubPush(r, w, integration, payload)
		case "pull_request":
			h.handleGitHubPR(r, w, integration, payload)
		case "release":
			h.handleGitHubRelease(r, w, integration, payload)
		case "check_suite":
			h.handleGitHubCheckSuite(r, w, integration, payload)
		default:
			writeJSON(w, http.StatusOK, map[string]string{"status": "ignored"})
		}
	} else {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ignored"})
	}
}

func (h *GitHandler) handleGitHubInstallation(r *http.Request, w http.ResponseWriter, integration *model.GitIntegration, payload map[string]interface{}) {
	if integration == nil {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ignored"})
		return
	}
	action, _ := payload["action"].(string)
	if err := h.gitService.HandleInstallationLifecycleEvent(r.Context(), integration, "installation", action, nil); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "processed"})
}

func (h *GitHandler) handleGitHubInstallationRepositories(r *http.Request, w http.ResponseWriter, integration *model.GitIntegration, payload map[string]interface{}) {
	if integration == nil {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ignored"})
		return
	}
	action, _ := payload["action"].(string)
	var key string
	switch strings.TrimSpace(action) {
	case "added":
		key = "repositories_added"
	case "removed":
		key = "repositories_removed"
	default:
		writeJSON(w, http.StatusOK, map[string]string{"status": "ignored"})
		return
	}
	externalIDs := externalRepoIDsFromPayloadArray(payload, key)
	if err := h.gitService.HandleInstallationLifecycleEvent(r.Context(), integration, "installation_repositories", action, externalIDs); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "processed"})
}

func (h *GitHandler) resolveWebhookWorkspaceID(r *http.Request, integration *model.GitIntegration, payload map[string]interface{}) (string, bool, error) {
	if integration == nil {
		workspaceID := r.URL.Query().Get("workspace_id")
		if strings.TrimSpace(workspaceID) == "" {
			return "", false, nil
		}
		return workspaceID, true, nil
	}
	if !integration.Active {
		return "", false, nil
	}
	repoExternalID, ok := nestedNumber(payload, "repository", "id")
	if !ok {
		return "", false, nil
	}
	repo, err := h.gitService.ResolveWebhookRepository(r.Context(), integration.ID, intString(repoExternalID))
	if err != nil {
		return "", false, err
	}
	if repo == nil {
		hasClaims, claimsErr := h.gitService.IntegrationHasWebhookClaims(r.Context(), integration.ID)
		if claimsErr != nil {
			return "", false, claimsErr
		}
		if !hasClaims && strings.TrimSpace(integration.WorkspaceID) != "" {
			return integration.WorkspaceID, true, nil
		}
		return "", false, nil
	}
	return repo.WorkspaceID, true, nil
}

func (h *GitHandler) handleGitHubPush(r *http.Request, w http.ResponseWriter, integration *model.GitIntegration, payload map[string]interface{}) {
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

	workspaceID, routed, err := h.resolveWebhookWorkspaceID(r, integration, payload)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if !routed {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	if err := h.gitService.ProcessWebhookPush(r.Context(), workspaceID, repo, branch, commitSHA); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "processed"})
}

func (h *GitHandler) handleGitHubPR(r *http.Request, w http.ResponseWriter, integration *model.GitIntegration, payload map[string]interface{}) {
	pr, ok := payload["pull_request"].(map[string]interface{})
	if !ok {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ignored"})
		return
	}

	action, _ := payload["action"].(string)
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
	baseBranch := ""
	if base, ok := pr["base"].(map[string]interface{}); ok {
		baseBranch, _ = base["ref"].(string)
	}

	if repo == "" {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ignored"})
		return
	}

	prTitle, _ := pr["title"].(string)
	workspaceID, routed, err := h.resolveWebhookWorkspaceID(r, integration, payload)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if !routed {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	if err := h.gitService.ProcessWebhookPR(r.Context(), workspaceID, repo, action, prNumber, prTitle, prURL, prStatus, branch, baseBranch); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "processed"})
}

func (h *GitHandler) handleGitHubRelease(r *http.Request, w http.ResponseWriter, integration *model.GitIntegration, payload map[string]interface{}) {
	action, _ := payload["action"].(string)
	repo, _ := nestedString(payload, "repository", "full_name")
	release, ok := payload["release"].(map[string]interface{})
	if !ok || repo == "" {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ignored"})
		return
	}

	tagName, _ := release["tag_name"].(string)
	targetCommitish, _ := release["target_commitish"].(string)
	workspaceID, routed, err := h.resolveWebhookWorkspaceID(r, integration, payload)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if !routed {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	if err := h.gitService.ProcessWebhookRelease(r.Context(), workspaceID, repo, action, tagName, targetCommitish); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "processed"})
}

func (h *GitHandler) handleGitHubCheckSuite(r *http.Request, w http.ResponseWriter, integration *model.GitIntegration, payload map[string]interface{}) {
	action, _ := payload["action"].(string)
	repo, _ := nestedString(payload, "repository", "full_name")
	checkSuite, ok := payload["check_suite"].(map[string]interface{})
	if !ok || repo == "" {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ignored"})
		return
	}

	branch, _ := checkSuite["head_branch"].(string)
	conclusion, _ := checkSuite["conclusion"].(string)
	workspaceID, routed, err := h.resolveWebhookWorkspaceID(r, integration, payload)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if !routed {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	if err := h.gitService.ProcessWebhookCheckSuite(r.Context(), workspaceID, repo, action, branch, conclusion); err != nil {
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

func externalRepoIDsFromPayloadArray(payload map[string]interface{}, key string) []string {
	items, ok := payload[key].([]interface{})
	if !ok {
		return nil
	}
	ids := make([]string, 0, len(items))
	for _, item := range items {
		entry, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		id, ok := entry["id"].(float64)
		if !ok {
			continue
		}
		ids = append(ids, intString(id))
	}
	return ids
}

func intString(value float64) string {
	return strconv.FormatInt(int64(value), 10)
}
