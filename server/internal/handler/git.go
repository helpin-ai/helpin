package handler

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/helpin-ai/helpin/server/internal/middleware"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"github.com/helpin-ai/helpin/server/internal/service"
)

// GitHandler handles git integration HTTP endpoints.
type GitHandler struct {
	gitService       *service.GitService
	webhookEventRepo *repository.GitWebhookEventRepository
}

// NewGitHandler creates a new GitHandler.
func NewGitHandler(gitService *service.GitService, webhookEventRepo *repository.GitWebhookEventRepository) *GitHandler {
	return &GitHandler{gitService: gitService, webhookEventRepo: webhookEventRepo}
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

func (h *GitHandler) GetOrgGitHubInstallURL(w http.ResponseWriter, r *http.Request) {
	orgID := chi.URLParam(r, "id")
	actorID := middleware.GetUserID(r.Context())
	forceInstall := strings.EqualFold(strings.TrimSpace(r.URL.Query().Get("force_install")), "true")
	returnWorkspaceID := strings.TrimSpace(r.URL.Query().Get("workspace_id"))

	installURL, action, integrationID, err := h.gitService.GetGitHubInstallURLForOrganization(r.Context(), orgID, returnWorkspaceID, actorID, forceInstall)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, model.GitHubInstallURLResponse{InstallURL: installURL, Action: action, IntegrationID: integrationID})
}

// ConnectOrgGitLab handles POST /api/organizations/{id}/git/gitlab/connect.
// Stores an org-scoped GitLab Personal/Group/Project Access Token after verifying
// it against the customer's GitLab instance.
func (h *GitHandler) ConnectOrgGitLab(w http.ResponseWriter, r *http.Request) {
	orgID := chi.URLParam(r, "id")
	actorID := middleware.GetUserID(r.Context())
	returnWorkspaceID := strings.TrimSpace(r.URL.Query().Get("workspace_id"))

	var req model.GitLabConnectTokenRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	integration, err := h.gitService.ConnectGitLabWithToken(r.Context(), orgID, returnWorkspaceID, actorID, req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	login := ""
	if integration.AccountLogin != nil {
		login = strings.TrimSpace(*integration.AccountLogin)
	}
	baseURL := ""
	if integration.BaseURL != nil {
		baseURL = strings.TrimSpace(*integration.BaseURL)
	}
	writeJSON(w, http.StatusOK, model.GitLabConnectResponse{
		IntegrationID: integration.ID,
		AccountLogin:  login,
		BaseURL:       baseURL,
	})
}

func (h *GitHandler) ListOrgIntegrations(w http.ResponseWriter, r *http.Request) {
	orgID := chi.URLParam(r, "id")
	actorID := middleware.GetUserID(r.Context())
	integrations, err := h.gitService.ListOrganizationIntegrations(r.Context(), orgID, actorID)
	if err != nil {
		writeError(w, http.StatusForbidden, err.Error())
		return
	}
	if integrations == nil {
		integrations = []model.GitIntegration{}
	}
	writeJSON(w, http.StatusOK, integrations)
}

func (h *GitHandler) CreateOrgIntegration(w http.ResponseWriter, r *http.Request) {
	orgID := chi.URLParam(r, "id")
	actorID := middleware.GetUserID(r.Context())
	returnWorkspaceID := strings.TrimSpace(r.URL.Query().Get("workspace_id"))
	var req model.CreateGitIntegrationRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	integration, err := h.gitService.CreateOrganizationIntegration(r.Context(), orgID, returnWorkspaceID, req, actorID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, integration)
}

func (h *GitHandler) GetOrgIntegration(w http.ResponseWriter, r *http.Request) {
	orgID := chi.URLParam(r, "id")
	integrationID := chi.URLParam(r, "integrationId")
	actorID := middleware.GetUserID(r.Context())
	detail, err := h.gitService.GetOrganizationIntegrationDetail(r.Context(), orgID, integrationID, actorID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, detail)
}

func (h *GitHandler) UpdateOrgIntegration(w http.ResponseWriter, r *http.Request) {
	orgID := chi.URLParam(r, "id")
	integrationID := chi.URLParam(r, "integrationId")
	actorID := middleware.GetUserID(r.Context())
	var req model.UpdateGitIntegrationRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	integration, err := h.gitService.UpdateOrganizationIntegration(r.Context(), orgID, integrationID, req, actorID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, integration)
}

func (h *GitHandler) DeleteOrgIntegration(w http.ResponseWriter, r *http.Request) {
	orgID := chi.URLParam(r, "id")
	integrationID := chi.URLParam(r, "integrationId")
	actorID := middleware.GetUserID(r.Context())
	returnWorkspaceID := strings.TrimSpace(r.URL.Query().Get("workspace_id"))
	if err := h.gitService.DeleteOrganizationIntegration(r.Context(), orgID, integrationID, actorID, returnWorkspaceID); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func (h *GitHandler) SyncOrgRepositories(w http.ResponseWriter, r *http.Request) {
	orgID := chi.URLParam(r, "id")
	integrationID := chi.URLParam(r, "integrationId")
	actorID := middleware.GetUserID(r.Context())
	repos, err := h.gitService.SyncOrganizationRepositories(r.Context(), orgID, integrationID, actorID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if repos == nil {
		repos = []model.GitRepository{}
	}
	writeJSON(w, http.StatusOK, repos)
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
// Accepts optional `search` (server-side filter passed to GitLab) and
// `nocache=1` (bypass the in-memory cache, used by the manual Refresh button).
func (h *GitHandler) ListAvailableRepos(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	integrationID := chi.URLParam(r, "id")
	actorID := middleware.GetUserID(r.Context())
	opts := service.ListAvailableReposOptions{
		Search:  strings.TrimSpace(r.URL.Query().Get("search")),
		NoCache: r.URL.Query().Get("nocache") == "1",
	}

	repos, err := h.gitService.ListAvailableRepos(r.Context(), workspaceID, integrationID, actorID, opts)
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

// GetEpicDeliveryTarget handles GET /api/pm/epics/{id}/delivery-target.
func (h *GitHandler) GetEpicDeliveryTarget(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	epicID := chi.URLParam(r, "id")

	target, err := h.gitService.GetEpicDeliveryTarget(r.Context(), workspaceID, epicID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, target)
}

// UpdateEpicDeliveryTarget handles PUT /api/pm/epics/{id}/delivery-target.
func (h *GitHandler) UpdateEpicDeliveryTarget(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	epicID := chi.URLParam(r, "id")
	actorID := middleware.GetUserID(r.Context())

	var req model.UpdateEpicDeliveryTargetRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	target, err := h.gitService.UpdateEpicDeliveryTarget(r.Context(), workspaceID, epicID, req, actorID)
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

	recorder := &gitWebhookResponseRecorder{ResponseWriter: w}

	// Parse minimal fields to route the event.
	var payload map[string]interface{}
	if err := json.Unmarshal(body, &payload); err != nil {
		slog.WarnContext(r.Context(), "git webhook invalid json", "provider", provider, "error", err)
		writeError(recorder, http.StatusBadRequest, "invalid JSON payload")
		return
	}

	webhookEventID := h.recordGitWebhookEvent(r, provider, body, payload)
	var resolvedIntegrationID *string
	defer func() {
		h.markGitWebhookEventHandled(r, webhookEventID, recorder, resolvedIntegrationID)
	}()

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
				writeError(recorder, http.StatusUnauthorized, err.Error())
				return
			}
			integration = resolvedIntegration
			if integration != nil {
				resolvedIntegrationID = &integration.ID
			}
		}

		event := r.Header.Get("X-GitHub-Event")
		switch event {
		case "installation":
			h.handleGitHubInstallation(r, recorder, integration, payload)
		case "installation_repositories":
			h.handleGitHubInstallationRepositories(r, recorder, integration, payload)
		case "push":
			h.handleGitHubPush(r, recorder, integration, payload)
		case "pull_request":
			h.handleGitHubPR(r, recorder, integration, payload)
		case "release":
			h.handleGitHubRelease(r, recorder, integration, payload)
		case "check_suite":
			h.handleGitHubCheckSuite(r, recorder, integration, payload)
		default:
			writeJSON(recorder, http.StatusOK, map[string]string{"status": "ignored"})
		}
	} else {
		h.handleGitLabWebhook(r, recorder, payload, &resolvedIntegrationID)
	}
}

type gitWebhookResponseRecorder struct {
	http.ResponseWriter
	statusCode int
	body       bytes.Buffer
}

func (r *gitWebhookResponseRecorder) WriteHeader(statusCode int) {
	r.statusCode = statusCode
	r.ResponseWriter.WriteHeader(statusCode)
}

func (r *gitWebhookResponseRecorder) Write(payload []byte) (int, error) {
	if r.statusCode == 0 {
		r.statusCode = http.StatusOK
	}
	if r.body.Len() < 2048 {
		remaining := 2048 - r.body.Len()
		if len(payload) > remaining {
			_, _ = r.body.Write(payload[:remaining])
		} else {
			_, _ = r.body.Write(payload)
		}
	}
	return r.ResponseWriter.Write(payload)
}

func (h *GitHandler) recordGitWebhookEvent(r *http.Request, provider string, body []byte, payload map[string]interface{}) string {
	if h.webhookEventRepo == nil {
		return ""
	}
	eventType := ""
	deliveryID := ""
	if provider == "github" {
		eventType = strings.TrimSpace(r.Header.Get("X-GitHub-Event"))
		deliveryID = strings.TrimSpace(r.Header.Get("X-GitHub-Delivery"))
	} else if provider == "gitlab" {
		eventType = strings.TrimSpace(r.Header.Get("X-Gitlab-Event"))
		deliveryID = strings.TrimSpace(r.Header.Get("X-Gitlab-Event-UUID"))
	}
	repo, _ := nestedString(payload, "repository", "full_name")
	action, _ := payload["action"].(string)
	event := &model.GitWebhookEvent{
		Provider:           provider,
		EventType:          eventType,
		DeliveryID:         optionalString(deliveryID),
		RepositoryFullName: optionalString(repo),
		Action:             optionalString(action),
		Status:             model.GitWebhookStatusReceived,
		RawPayload:         string(body),
		ReceivedAt:         time.Now().UTC(),
	}
	if err := h.webhookEventRepo.Create(r.Context(), event); err != nil {
		slog.WarnContext(r.Context(), "record git webhook event failed", "error", err, "provider", provider, "event_type", eventType, "delivery_id", deliveryID)
		return ""
	}
	slog.InfoContext(r.Context(), "git webhook received", "provider", provider, "event_type", eventType, "delivery_id", deliveryID, "webhook_event_id", event.ID, "repo", repo, "action", action)
	return event.ID
}

func (h *GitHandler) markGitWebhookEventHandled(r *http.Request, eventID string, recorder *gitWebhookResponseRecorder, integrationID *string) {
	if h.webhookEventRepo == nil || eventID == "" || recorder == nil {
		return
	}
	statusCode := recorder.statusCode
	if statusCode == 0 {
		statusCode = http.StatusOK
	}
	status := model.GitWebhookStatusProcessed
	body := recorder.body.String()
	if statusCode >= 400 {
		status = model.GitWebhookStatusFailed
	} else if statusCode == http.StatusNoContent || strings.Contains(body, `"status":"ignored"`) {
		status = model.GitWebhookStatusIgnored
	}
	var errMessage *string
	if status == model.GitWebhookStatusFailed {
		errMessage = optionalString(strings.TrimSpace(body))
	}
	workspaceID := optionalString(strings.TrimSpace(r.URL.Query().Get("workspace_id")))
	if err := h.webhookEventRepo.MarkHandled(r.Context(), eventID, status, statusCode, errMessage, integrationID, workspaceID); err != nil {
		slog.WarnContext(r.Context(), "mark git webhook event handled failed", "error", err, "webhook_event_id", eventID)
	}
	slog.InfoContext(r.Context(), "git webhook handled", "webhook_event_id", eventID, "status", status, "status_code", statusCode)
}

func optionalString(value string) *string {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	return &value
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
		if !hasClaims && integration.WorkspaceID != nil && strings.TrimSpace(*integration.WorkspaceID) != "" {
			return strings.TrimSpace(*integration.WorkspaceID), true, nil
		}
		return "", false, nil
	}
	return repo.WorkspaceID, true, nil
}

func (h *GitHandler) resolveWebhookRepositories(r *http.Request, integration *model.GitIntegration, payload map[string]interface{}) ([]model.GitRepository, bool, error) {
	if integration == nil {
		workspaceID := strings.TrimSpace(r.URL.Query().Get("workspace_id"))
		if workspaceID == "" {
			return nil, false, nil
		}
		repo, _ := nestedString(payload, "repository", "full_name")
		if repo == "" {
			return nil, false, nil
		}
		return []model.GitRepository{{WorkspaceID: workspaceID, Provider: "github", FullName: repo}}, true, nil
	}
	if !integration.Active {
		return nil, false, nil
	}
	repoExternalID, ok := nestedNumber(payload, "repository", "id")
	if !ok {
		return nil, false, nil
	}
	repos, err := h.gitService.ResolveWebhookRepositories(r.Context(), integration.ID, intString(repoExternalID))
	if err != nil {
		return nil, false, err
	}
	if len(repos) > 0 {
		return repos, true, nil
	}
	hasClaims, err := h.gitService.IntegrationHasWebhookClaims(r.Context(), integration.ID)
	if err != nil {
		return nil, false, err
	}
	if !hasClaims && integration.WorkspaceID != nil && strings.TrimSpace(*integration.WorkspaceID) != "" {
		repo, _ := nestedString(payload, "repository", "full_name")
		return []model.GitRepository{{
			WorkspaceID: strings.TrimSpace(*integration.WorkspaceID),
			Provider:    integration.Provider,
			FullName:    repo,
		}}, true, nil
	}
	return nil, false, nil
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

	repoRecords, routed, err := h.resolveWebhookRepositories(r, integration, payload)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if !routed {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	for _, repoRecord := range repoRecords {
		if err := h.gitService.ProcessWebhookPushForProvider(r.Context(), repoRecord.WorkspaceID, repoRecord.Provider, repo, branch, commitSHA); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
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
	repoRecords, routed, err := h.resolveWebhookRepositories(r, integration, payload)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if !routed {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	for _, repoRecord := range repoRecords {
		if err := h.gitService.ProcessWebhookPRForProvider(r.Context(), repoRecord.WorkspaceID, repoRecord.Provider, repo, action, prNumber, prTitle, prURL, prStatus, branch, baseBranch); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
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
	releaseName, _ := release["name"].(string)
	releaseURL, _ := release["html_url"].(string)
	isPrerelease, _ := release["prerelease"].(bool)
	var publishedAt *time.Time
	if rawPublishedAt, _ := release["published_at"].(string); strings.TrimSpace(rawPublishedAt) != "" {
		if parsed, err := time.Parse(time.RFC3339, rawPublishedAt); err == nil {
			publishedAt = &parsed
		}
	}
	repoRecords, routed, err := h.resolveWebhookRepositories(r, integration, payload)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if !routed {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	for _, repoRecord := range repoRecords {
		if err := h.gitService.ProcessWebhookReleaseForProvider(r.Context(), repoRecord.WorkspaceID, repoRecord.Provider, repo, action, tagName, targetCommitish, releaseName, releaseURL, publishedAt, isPrerelease); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
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
	repoRecords, routed, err := h.resolveWebhookRepositories(r, integration, payload)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if !routed {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	for _, repoRecord := range repoRecords {
		if err := h.gitService.ProcessWebhookCheckSuiteForProvider(r.Context(), repoRecord.WorkspaceID, repoRecord.Provider, repo, action, branch, conclusion); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "processed"})
}

func (h *GitHandler) handleGitLabWebhook(r *http.Request, w http.ResponseWriter, payload map[string]interface{}, resolvedIntegrationID **string) {
	projectID, ok := gitlabProjectID(payload)
	if !ok {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ignored"})
		return
	}
	integration, repoRecords, err := h.gitService.ResolveGitLabWebhookRepositories(r.Context(), projectID, r.Header.Get("X-Gitlab-Token"))
	if err != nil {
		writeError(w, http.StatusUnauthorized, err.Error())
		return
	}
	if integration == nil || len(repoRecords) == 0 {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	*resolvedIntegrationID = &integration.ID
	event := strings.TrimSpace(r.Header.Get("X-Gitlab-Event"))
	switch event {
	case "Push Hook":
		h.handleGitLabPush(r, w, repoRecords, payload)
	case "Merge Request Hook":
		h.handleGitLabMergeRequest(r, w, repoRecords, payload)
	case "Pipeline Hook":
		h.handleGitLabPipeline(r, w, repoRecords, payload)
	case "Release Hook":
		h.handleGitLabRelease(r, w, repoRecords, payload)
	default:
		writeJSON(w, http.StatusOK, map[string]string{"status": "ignored"})
	}
}

func (h *GitHandler) handleGitLabPush(r *http.Request, w http.ResponseWriter, repoRecords []model.GitRepository, payload map[string]interface{}) {
	ref, _ := payload["ref"].(string)
	branch := strings.TrimPrefix(ref, "refs/heads/")
	commitSHA, _ := payload["checkout_sha"].(string)
	if commitSHA == "" {
		commitSHA, _ = payload["after"].(string)
	}
	if len(repoRecords) == 0 || branch == "" {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ignored"})
		return
	}
	for _, repoRecord := range repoRecords {
		if err := h.gitService.ProcessWebhookPushForProvider(r.Context(), repoRecord.WorkspaceID, repoRecord.Provider, repoRecord.FullName, branch, commitSHA); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "processed"})
}

func (h *GitHandler) handleGitLabMergeRequest(r *http.Request, w http.ResponseWriter, repoRecords []model.GitRepository, payload map[string]interface{}) {
	attrs, ok := payload["object_attributes"].(map[string]interface{})
	if !ok || len(repoRecords) == 0 {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ignored"})
		return
	}
	action, _ := attrs["action"].(string)
	state, _ := attrs["state"].(string)
	prStatus := "open"
	if action == "merge" || state == "merged" {
		prStatus = "merged"
	} else if action == "close" || state == "closed" {
		prStatus = "closed"
	}
	iid := intFromAny(attrs["iid"])
	title, _ := attrs["title"].(string)
	url, _ := attrs["url"].(string)
	sourceBranch, _ := attrs["source_branch"].(string)
	targetBranch, _ := attrs["target_branch"].(string)
	if iid == 0 {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ignored"})
		return
	}
	for _, repoRecord := range repoRecords {
		if err := h.gitService.ProcessWebhookPRForProvider(r.Context(), repoRecord.WorkspaceID, repoRecord.Provider, repoRecord.FullName, gitlabMRAction(action, prStatus), iid, title, url, prStatus, sourceBranch, targetBranch); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "processed"})
}

func (h *GitHandler) handleGitLabPipeline(r *http.Request, w http.ResponseWriter, repoRecords []model.GitRepository, payload map[string]interface{}) {
	attrs, _ := payload["object_attributes"].(map[string]interface{})
	status, _ := attrs["status"].(string)
	ref, _ := attrs["ref"].(string)
	if len(repoRecords) == 0 || status == "" {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ignored"})
		return
	}
	if !gitlabPipelineTerminalStatus(status) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ignored"})
		return
	}
	for _, repoRecord := range repoRecords {
		if err := h.gitService.ProcessWebhookCheckSuiteForProvider(r.Context(), repoRecord.WorkspaceID, repoRecord.Provider, repoRecord.FullName, "completed", ref, status); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "processed"})
}

func (h *GitHandler) handleGitLabRelease(r *http.Request, w http.ResponseWriter, repoRecords []model.GitRepository, payload map[string]interface{}) {
	if len(repoRecords) == 0 {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ignored"})
		return
	}
	tag, _ := payload["tag"].(string)
	name, _ := payload["name"].(string)
	url, _ := payload["url"].(string)
	action := "published"
	if strings.TrimSpace(tag) == "" {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ignored"})
		return
	}
	for _, repoRecord := range repoRecords {
		if err := h.gitService.ProcessWebhookReleaseForProvider(r.Context(), repoRecord.WorkspaceID, repoRecord.Provider, repoRecord.FullName, action, tag, "", name, url, nil, false); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "processed"})
}

func gitlabProjectID(payload map[string]interface{}) (string, bool) {
	if id, ok := nestedNumber(payload, "project", "id"); ok {
		return intString(id), true
	}
	if id := intFromAny(payload["project_id"]); id > 0 {
		return strconv.Itoa(id), true
	}
	return "", false
}

func intFromAny(value interface{}) int {
	switch typed := value.(type) {
	case float64:
		return int(typed)
	case int:
		return typed
	case string:
		parsed, _ := strconv.Atoi(strings.TrimSpace(typed))
		return parsed
	default:
		return 0
	}
}

func gitlabMRAction(action, status string) string {
	switch {
	case status == "merged":
		return "closed"
	case status == "closed":
		return "closed"
	case action == "open" || action == "reopen":
		return "opened"
	default:
		return action
	}
}

func gitlabPipelineTerminalStatus(status string) bool {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "success", "failed", "canceled", "skipped":
		return true
	default:
		return false
	}
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
