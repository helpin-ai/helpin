package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"
	agentruntime "github.com/helpin-ai/agent-runtime-go"

	"github.com/helpin-ai/helpin/server/internal/agentcontract"
	"github.com/helpin-ai/helpin/server/internal/authorization"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

const (
	agentRuntimeHelpinBuiltInSkillIDPrefix     = "helpin_builtin:"
	agentRuntimeHelpinBuiltInSkillObjectPrefix = "helpin-builtins/"
	agentRuntimeObjectiveCollectionLimit       = 100
	agentRuntimeBrowserScreenshotMaxBytes      = 10 * 1024 * 1024
	agentRuntimeBrowserRecordingMaxBytes       = 100 * 1024 * 1024
)

var (
	ErrAgentRuntimeHostBadRequest = errors.New("agent runtime host bad request")
	ErrAgentRuntimeHostForbidden  = errors.New("agent runtime host forbidden")
	ErrAgentRuntimeHostNotFound   = errors.New("agent runtime host not found")
)

// AgentRuntimeHostService serves the host callbacks consumed by agent-runtime.
// It exposes runtime contracts only; Helpin lifecycle policy remains in the
// existing projection, billing, and finalizer services.
type AgentRuntimeHostService struct {
	appID             string
	runRepo           *repository.AgentRunRepository
	agentRepo         *repository.AgentRepository
	workspaceRepo     *repository.WorkspaceRepository
	taskRepo          *repository.PMTaskRepository
	epicRepo          *repository.PMEpicRepository
	sprintService     *PMSprintService
	objectiveService  *PMObjectiveService
	supportRepo       *repository.SupportConversationRepository
	supportCoverage   *SupportCoverageService
	docsRepo          *repository.DocsDocumentRepository
	crmContactRepo    *repository.CRMContactRepository
	crmCompanyRepo    *repository.CRMCompanyRepository
	crmDealRepo       *repository.CRMDealRepository
	commandService    *InternalCommandService
	providerCommands  map[string]string
	gitService        *GitService
	skillRepo         *repository.WorkspaceSkillRepository
	skillStore        skillPackageStore
	builtInArchives   *runtimeBuiltInSkillArchiveCache
	authz             *authorization.AuthzService
	artifactRepo      agentRuntimeBrowserArtifactRepository
	assetStore        agentRuntimeBrowserAssetStore
	playbookExecution *CRMPlaybookExecutionService
}

type agentRuntimeBrowserAssetStore interface {
	PutObject(ctx context.Context, key, contentType string, size int64, body io.Reader, publicRead bool) error
	DeleteObject(ctx context.Context, key string) error
	GeneratePresignedInlineGetURL(key string) (string, error)
}

type agentRuntimeBrowserArtifactRepository interface {
	NextSequence(ctx context.Context, workspaceID, runID string) (int, error)
	Create(ctx context.Context, artifact *model.AgentRunArtifact) error
	GetByIDAndWorkspace(ctx context.Context, workspaceID, artifactID string) (*model.AgentRunArtifact, error)
}

// SetBrowserAssetStore enables durable private browser asset uploads from agent-runtime.
func (s *AgentRuntimeHostService) SetBrowserAssetStore(artifactRepo *repository.AgentRunArtifactRepository, assetStore agentRuntimeBrowserAssetStore) *AgentRuntimeHostService {
	if s == nil {
		return s
	}
	s.artifactRepo = artifactRepo
	s.assetStore = assetStore
	return s
}

type AgentRuntimeBrowserAssetUpload struct {
	AppID        string
	RuntimeRunID string
	ArtifactType string
	Metadata     json.RawMessage
	FileName     string
	ContentType  string
	Size         int64
	Body         io.Reader
}

type AgentRuntimeBrowserAsset struct {
	ArtifactID  string `json:"artifact_id"`
	ArtifactRef string `json:"artifact_ref"`
	Visibility  string `json:"visibility"`
	FileName    string `json:"file_name"`
	ContentType string `json:"content_type"`
	SizeBytes   int64  `json:"size_bytes"`
}

type AgentRuntimeArtifactContentURL struct {
	URL       string    `json:"url"`
	ExpiresAt time.Time `json:"expires_at"`
}

// UploadBrowserAsset validates the runtime-to-host run mapping, stores a private
// browser screenshot or recording, and records it as a first-class Helpin run artifact.
func (s *AgentRuntimeHostService) UploadBrowserAsset(ctx context.Context, upload AgentRuntimeBrowserAssetUpload) (*AgentRuntimeBrowserAsset, error) {
	if s == nil || s.runRepo == nil || s.artifactRepo == nil || s.assetStore == nil {
		return nil, fmt.Errorf("browser artifact storage is unavailable")
	}
	if err := s.validateAppID(upload.AppID); err != nil {
		return nil, err
	}
	upload.RuntimeRunID = strings.TrimSpace(upload.RuntimeRunID)
	upload.ArtifactType = strings.TrimSpace(upload.ArtifactType)
	if upload.RuntimeRunID == "" {
		return nil, fmt.Errorf("%w: run_id is required", ErrAgentRuntimeHostBadRequest)
	}
	contentType := strings.ToLower(strings.TrimSpace(strings.Split(upload.ContentType, ";")[0]))
	ext, maxBytes, defaultName, source, err := browserArtifactStoragePolicy(upload.ArtifactType, contentType)
	if err != nil {
		return nil, err
	}
	if upload.Body == nil || upload.Size <= 0 || upload.Size > maxBytes {
		return nil, fmt.Errorf(
			"%w: %s must be between 1 byte and %d MB",
			ErrAgentRuntimeHostBadRequest,
			strings.ReplaceAll(upload.ArtifactType, "_", " "),
			maxBytes/(1024*1024),
		)
	}
	run, err := s.runRepo.GetByExternalRuntimeID(ctx, agentRuntimeName, upload.RuntimeRunID)
	if err != nil {
		return nil, err
	}
	if run == nil {
		return nil, fmt.Errorf("%w: agent run not found", ErrAgentRuntimeHostNotFound)
	}
	workspaceID := strings.TrimSpace(run.WorkspaceID)
	assetID := uuid.NewString()
	objectKey := fmt.Sprintf("agent-runs/%s/%s/browser/%s%s", workspaceID, run.ID, assetID, ext)
	if err := s.assetStore.PutObject(ctx, objectKey, contentType, upload.Size, upload.Body, false); err != nil {
		return nil, err
	}
	sequence, err := s.artifactRepo.NextSequence(ctx, workspaceID, run.ID)
	if err != nil {
		if deleteErr := s.assetStore.DeleteObject(ctx, objectKey); deleteErr != nil {
			return nil, fmt.Errorf("next browser artifact sequence: %w; cleanup failed: %v", err, deleteErr)
		}
		return nil, err
	}
	name := strings.TrimSpace(filepath.Base(upload.FileName))
	if name == "" || name == "." {
		name = defaultName + ext
	}
	var captureMetadata map[string]any
	if len(upload.Metadata) > 0 && strings.TrimSpace(string(upload.Metadata)) != "null" {
		if err := json.Unmarshal(upload.Metadata, &captureMetadata); err != nil {
			if deleteErr := s.assetStore.DeleteObject(ctx, objectKey); deleteErr != nil {
				return nil, fmt.Errorf("%w: invalid artifact metadata; cleanup failed: %v", ErrAgentRuntimeHostBadRequest, deleteErr)
			}
			return nil, fmt.Errorf("%w: invalid artifact metadata", ErrAgentRuntimeHostBadRequest)
		}
	}
	metadata, _ := json.Marshal(map[string]any{
		"artifact_id": assetID, "artifact_ref": artifactReference(assetID), "visibility": "private", "file_name": name,
		"content_type": contentType, "size_bytes": upload.Size,
		"runtime_run_id": upload.RuntimeRunID, "source": source, "capture": captureMetadata,
	})
	artifact := &model.AgentRunArtifact{
		ID: assetID, WorkspaceID: workspaceID, RunID: run.ID,
		ArtifactType: upload.ArtifactType, Format: strings.TrimPrefix(ext, "."),
		StorageMode: "object", ObjectKey: &objectKey, Metadata: metadata, SequenceNo: sequence,
	}
	if err := s.artifactRepo.Create(ctx, artifact); err != nil {
		if deleteErr := s.assetStore.DeleteObject(ctx, objectKey); deleteErr != nil {
			return nil, fmt.Errorf("create browser artifact: %w; cleanup failed: %v", err, deleteErr)
		}
		return nil, err
	}
	return &AgentRuntimeBrowserAsset{
		ArtifactID: assetID, ArtifactRef: artifactReference(assetID), Visibility: "private",
		FileName: name, ContentType: contentType, SizeBytes: upload.Size,
	}, nil
}

// BrowserArtifactContentURL returns a short-lived private URL after the caller
// has passed normal Helpin workspace authorization middleware.
func (s *AgentRuntimeHostService) BrowserArtifactContentURL(ctx context.Context, workspaceID, artifactID string) (*AgentRuntimeArtifactContentURL, error) {
	workspaceID = strings.TrimSpace(workspaceID)
	artifactID = strings.TrimSpace(artifactID)
	if workspaceID == "" || artifactID == "" {
		return nil, fmt.Errorf("%w: workspace_id and artifact_id are required", ErrAgentRuntimeHostBadRequest)
	}
	if s == nil || s.artifactRepo == nil || s.assetStore == nil {
		return nil, fmt.Errorf("browser artifact storage is unavailable")
	}
	artifact, err := s.artifactRepo.GetByIDAndWorkspace(ctx, workspaceID, artifactID)
	if err != nil {
		return nil, err
	}
	if artifact == nil || !isBrowserMediaArtifactType(artifact.ArtifactType) || artifact.StorageMode != "object" || artifact.ObjectKey == nil || strings.TrimSpace(*artifact.ObjectKey) == "" {
		return nil, fmt.Errorf("%w: browser artifact not found", ErrAgentRuntimeHostNotFound)
	}
	contentURL, err := s.assetStore.GeneratePresignedInlineGetURL(strings.TrimSpace(*artifact.ObjectKey))
	if artifact.ArtifactType == "analysis_output" && artifact.Format != "png" {
		if downloads, ok := s.assetStore.(interface {
			GeneratePresignedGetURL(string, string) (string, error)
		}); ok {
			var metadata struct {
				FileName string `json:"file_name"`
			}
			if decodeErr := json.Unmarshal(artifact.Metadata, &metadata); decodeErr != nil {
				return nil, decodeErr
			}
			contentURL, err = downloads.GeneratePresignedGetURL(strings.TrimSpace(*artifact.ObjectKey), metadata.FileName)
		}
	}
	if err != nil {
		return nil, fmt.Errorf("generate browser artifact content URL: %w", err)
	}
	return &AgentRuntimeArtifactContentURL{URL: contentURL, ExpiresAt: time.Now().UTC().Add(time.Hour)}, nil
}

func browserArtifactStoragePolicy(artifactType, contentType string) (string, int64, string, string, error) {
	switch artifactType {
	case "analysis_output":
		ext := map[string]string{"image/png": ".png", "text/csv": ".csv", "application/json": ".json", "text/plain": ".txt"}[contentType]
		if ext == "" {
			return "", 0, "", "", fmt.Errorf("%w: analysis output must be CSV, PNG, JSON or text", ErrAgentRuntimeHostBadRequest)
		}
		return ext, 10 << 20, "analysis-output", "python", nil
	case model.AgentRunArtifactTypeBrowserScreenshot:
		switch contentType {
		case "image/png":
			return ".png", agentRuntimeBrowserScreenshotMaxBytes, "browser-screenshot", "agent-browser", nil
		case "image/jpeg":
			return ".jpg", agentRuntimeBrowserScreenshotMaxBytes, "browser-screenshot", "agent-browser", nil
		default:
			return "", 0, "", "", fmt.Errorf("%w: browser screenshot content type must be image/png or image/jpeg", ErrAgentRuntimeHostBadRequest)
		}
	case model.AgentRunArtifactTypeBrowserRecording:
		if contentType != "video/mp4" {
			return "", 0, "", "", fmt.Errorf("%w: browser recording content type must be video/mp4", ErrAgentRuntimeHostBadRequest)
		}
		return ".mp4", agentRuntimeBrowserRecordingMaxBytes, "browser-recording", "kernel", nil
	default:
		return "", 0, "", "", fmt.Errorf(
			"%w: artifact_type must be browser_screenshot or browser_recording",
			ErrAgentRuntimeHostBadRequest,
		)
	}
}

func isBrowserMediaArtifactType(artifactType string) bool {
	return artifactType == "analysis_output" || artifactType == model.AgentRunArtifactTypeBrowserScreenshot ||
		artifactType == model.AgentRunArtifactTypeBrowserRecording
}

// SetAgentRepository enables repository-backed effective agent scope
// resolution for internal command execution.
func (s *AgentRuntimeHostService) SetAgentRepository(agentRepo *repository.AgentRepository) *AgentRuntimeHostService {
	if s == nil {
		return s
	}
	s.agentRepo = agentRepo
	return s
}

type AgentRuntimeSkillLookupRequest struct {
	AppID    string                 `json:"app_id"`
	AgentID  string                 `json:"agent_id,omitempty"`
	RunID    string                 `json:"run_id,omitempty"`
	Target   agentruntime.TargetRef `json:"target"`
	Trigger  map[string]interface{} `json:"trigger,omitempty"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
	SkillID  string                 `json:"skill_id,omitempty"`
	Key      string                 `json:"key,omitempty"`
}

type AgentRuntimeWorkspaceSkill struct {
	ID                string          `json:"id"`
	Key               string          `json:"key"`
	VersionKey        string          `json:"version_key"`
	Title             string          `json:"title"`
	Description       string          `json:"description,omitempty"`
	SourceKind        string          `json:"source_kind"`
	Instructions      string          `json:"instructions"`
	RequiredTools     []string        `json:"required_tools,omitempty"`
	SupportedRuntimes []string        `json:"supported_runtimes,omitempty"`
	Interface         json.RawMessage `json:"interface,omitempty"`
	Policy            json.RawMessage `json:"policy,omitempty"`
	PackageObjectKey  string          `json:"package_object_key,omitempty"`
	PackageFileName   string          `json:"package_file_name,omitempty"`
	PackageChecksum   string          `json:"package_checksum,omitempty"`
	PackageSize       int64           `json:"package_size,omitempty"`
	IsArchived        bool            `json:"is_archived,omitempty"`
}

func NewAgentRuntimeHostService(
	appID string,
	runRepo *repository.AgentRunRepository,
	workspaceRepo *repository.WorkspaceRepository,
	taskRepo *repository.PMTaskRepository,
	epicRepo *repository.PMEpicRepository,
	supportRepo *repository.SupportConversationRepository,
	docsRepo *repository.DocsDocumentRepository,
	crmContactRepo *repository.CRMContactRepository,
	crmCompanyRepo *repository.CRMCompanyRepository,
	crmDealRepo *repository.CRMDealRepository,
	commandService *InternalCommandService,
	gitService *GitService,
) *AgentRuntimeHostService {
	return &AgentRuntimeHostService{
		appID:            strings.TrimSpace(appID),
		runRepo:          runRepo,
		workspaceRepo:    workspaceRepo,
		taskRepo:         taskRepo,
		epicRepo:         epicRepo,
		supportRepo:      supportRepo,
		docsRepo:         docsRepo,
		crmContactRepo:   crmContactRepo,
		crmCompanyRepo:   crmCompanyRepo,
		crmDealRepo:      crmDealRepo,
		commandService:   commandService,
		providerCommands: providerCommandNames(commandService),
		gitService:       gitService,
		builtInArchives:  newRuntimeBuiltInSkillArchiveCache(agentcontract.BuildSkillArchive),
	}
}

func (s *AgentRuntimeHostService) SetWorkspaceSkillStore(repo *repository.WorkspaceSkillRepository, store skillPackageStore) *AgentRuntimeHostService {
	if s == nil {
		return s
	}
	s.skillRepo = repo
	s.skillStore = store
	return s
}

// SetAuthorizationService enables per-actor RBAC enrichment on command
// execution: when a run carries an external actor (the triggering user),
// commands are executed with that user's workspace role and team memberships.
func (s *AgentRuntimeHostService) SetAuthorizationService(authz *authorization.AuthzService) *AgentRuntimeHostService {
	if s == nil {
		return s
	}
	s.authz = authz
	return s
}

// SetPMSprintService enables sprint target-context resolution without changing
// the positional runtime-host constructor used throughout the service tests.
func (s *AgentRuntimeHostService) SetPMSprintService(sprintService *PMSprintService) *AgentRuntimeHostService {
	if s == nil {
		return s
	}
	s.sprintService = sprintService
	return s
}

// SetPMObjectiveService enables workspace-scoped objective target projection.
func (s *AgentRuntimeHostService) SetPMObjectiveService(objectiveService *PMObjectiveService) *AgentRuntimeHostService {
	if s == nil {
		return s
	}
	s.objectiveService = objectiveService
	return s
}

// SetSupportCoverageService enables typed support coverage gap target
// resolution for documentation-agent runs.
func (s *AgentRuntimeHostService) SetSupportCoverageService(coverageService *SupportCoverageService) *AgentRuntimeHostService {
	if s == nil {
		return s
	}
	s.supportCoverage = coverageService
	return s
}

// enrichCommandActor resolves the external actor's workspace membership,
// stamps role/team info onto the command metadata, and returns the full actor
// for authorization checks in command services. Runs without a human actor
// (schedules, automation rules) are left untouched — agent-level tool policy
// remains their only gate. A non-member actor is rejected outright.
func (s *AgentRuntimeHostService) enrichCommandActor(ctx context.Context, meta *model.InternalCommandContext) (*authorization.Actor, error) {
	if s == nil || s.authz == nil || meta == nil {
		return nil, nil
	}
	actorID := strings.TrimSpace(meta.ActorID)
	if actorID == "" {
		return nil, nil
	}
	actor, err := s.authz.ResolveActor(ctx, meta.WorkspaceID, actorID)
	if err != nil {
		return nil, fmt.Errorf("%w: actor is not an active workspace member", ErrAgentRuntimeHostForbidden)
	}
	meta.ActorRole = actor.Role
	meta.ActorTeamIDs = actor.TeamIDs()
	return actor, nil
}

func (s *AgentRuntimeHostService) ResolveTargetContext(ctx context.Context, req agentruntime.TargetContextRequest) (*agentruntime.TargetContext, error) {
	if s == nil {
		return nil, fmt.Errorf("agent runtime host service is not configured")
	}
	if err := s.validateAppID(req.AppID); err != nil {
		return nil, err
	}
	if scope, err := s.resolveCRMPlaybookCallback(ctx, req.RunID, req.AgentID, req.Target, req.Metadata, req.Target.Metadata); err != nil || scope != nil {
		if err != nil {
			return nil, err
		}
		return &agentruntime.TargetContext{Target: req.Target, Summary: scope.binding.Input.CRMPlaybook.CustomerObjective,
			Data: map[string]interface{}{"crm_playbook": scope.binding.Input.CRMPlaybook, "context_tool": "get_crm_playbook_context"}}, nil
	}
	if err := s.rejectUnclaimedCRMPlaybookCallback(ctx, req.RunID, req.Metadata, req.Target.Metadata); err != nil {
		return nil, err
	}
	target := normalizeRuntimeTarget(req.Target)
	if target.Type == "" || target.ID == "" {
		return nil, fmt.Errorf("%w: target.type and target.id are required", ErrAgentRuntimeHostBadRequest)
	}

	resp := &agentruntime.TargetContext{
		Target: target,
		Data:   map[string]interface{}{},
	}
	requestedWorkspaceID := runtimeWorkspaceID(req.Metadata, req.Target.Metadata)
	if requestedWorkspaceID == "" {
		var err error
		requestedWorkspaceID, err = s.workspaceIDForRuntimeRun(ctx, req.RunID)
		if err != nil {
			return nil, err
		}
	}
	workspaceID := requestedWorkspaceID

	previewRun, err := s.helpinRunForRuntimeRun(ctx, req.RunID)
	if err != nil {
		return nil, err
	}
	if previewRun != nil && previewRun.TargetType == supportPreviewTarget && (target.Type != supportPreviewTarget || target.ID != previewRun.TargetID) {
		return nil, ErrAgentRuntimeHostForbidden
	}
	if target.Type == supportPreviewTarget {
		if previewRun == nil && s.runRepo != nil {
			previewRun, err = s.runRepo.GetByID(ctx, workspaceID, target.ID)
			if err != nil {
				return nil, err
			}
		}
		if previewRun == nil || previewRun.TargetType != supportPreviewTarget || previewRun.TargetID != target.ID || previewRun.AgentID != req.AgentID || previewRun.WorkspaceID != workspaceID {
			return nil, ErrAgentRuntimeHostForbidden
		}
		snapshot, err := supportPreviewSnapshot(previewRun)
		if err != nil {
			return nil, err
		}
		resp.Summary = "Support preview"
		resp.Data = previewConversationData(previewRun)
		resp.Data["messages"] = previewMessages(snapshot)
		resp.Data["required_confidence"] = snapshot.ConfidenceThreshold
		if s.workspaceRepo != nil {
			workspace, err := s.workspaceRepo.GetByID(ctx, workspaceID)
			if err != nil {
				return nil, err
			}
			if workspace != nil {
				resp.Summary = runtimeSupportConversationSummary(&model.SupportConversation{Subject: "Support preview"}, workspace)
				resp.Data["workspace"] = runtimeWorkspaceContextData(workspace)
				resp.Data["product_context"] = map[string]interface{}{"name": workspace.Name, "website_url": agentRuntimeHostString(workspace.WebsiteURL), "summary": agentRuntimeHostString(workspace.Description), "is_current_website_product": true, "resolve_generic_product_references": true}
			}
		}
		return resp, nil
	}

	switch target.Type {
	case "workspace":
		workspace, err := s.workspaceRepo.GetByID(ctx, target.ID)
		if err != nil {
			return nil, err
		}
		if workspace == nil {
			return nil, fmt.Errorf("%w: workspace not found", ErrAgentRuntimeHostNotFound)
		}
		workspaceID = workspace.ID
		if err := ensureRuntimeWorkspaceMatch(requestedWorkspaceID, workspaceID); err != nil {
			return nil, err
		}
		resp.Summary = fmt.Sprintf("Workspace: %s", workspace.Name)
		resp.Target.Display = &agentruntime.TargetDisplay{Title: workspace.Name}
		resp.Data = runtimeWorkspaceContextData(workspace)
	case "task":
		task, err := s.taskRepo.GetByID(ctx, target.ID)
		if err != nil {
			return nil, err
		}
		if task == nil {
			return nil, fmt.Errorf("%w: task not found", ErrAgentRuntimeHostNotFound)
		}
		workspaceID = task.Task.WorkspaceID
		if err := ensureRuntimeWorkspaceMatch(requestedWorkspaceID, workspaceID); err != nil {
			return nil, err
		}
		resp.Summary = fmt.Sprintf("Task %d: %s", task.Task.DisplayID, task.Task.Name)
		resp.Target.Type = "task"
		resp.Target.Display = &agentruntime.TargetDisplay{Title: task.Task.Name}
		resp.Data = runtimeTaskContextData(task)
	case "epic":
		epic, err := s.epicRepo.GetByID(ctx, target.ID)
		if err != nil {
			return nil, err
		}
		if epic == nil {
			return nil, fmt.Errorf("%w: epic not found", ErrAgentRuntimeHostNotFound)
		}
		workspaceID = epic.Epic.WorkspaceID
		if err := ensureRuntimeWorkspaceMatch(requestedWorkspaceID, workspaceID); err != nil {
			return nil, err
		}
		resp.Summary = fmt.Sprintf("Epic: %s", epic.Epic.Name)
		resp.Target.Display = &agentruntime.TargetDisplay{Title: epic.Epic.Name}
		resp.Data = runtimeEpicContextData(epic)
	case "sprint":
		if s.sprintService == nil {
			return nil, fmt.Errorf("sprint target resolver is not configured")
		}
		sprint, err := s.sprintService.GetByID(ctx, target.ID)
		if err != nil || sprint == nil {
			return nil, fmt.Errorf("%w: sprint not found", ErrAgentRuntimeHostNotFound)
		}
		workspaceID = sprint.Sprint.WorkspaceID
		if err := ensureRuntimeWorkspaceMatch(requestedWorkspaceID, workspaceID); err != nil {
			return nil, err
		}
		resp.Summary = fmt.Sprintf("Sprint: %s", sprint.Sprint.Name)
		resp.Target.Display = &agentruntime.TargetDisplay{Title: sprint.Sprint.Name}
		resp.Data = runtimeSprintContextData(sprint)
	case "objective":
		if s.objectiveService == nil {
			return nil, fmt.Errorf("objective target resolver is not configured")
		}
		objective, err := s.objectiveService.GetByID(ctx, target.ID)
		if err != nil || objective == nil {
			return nil, fmt.Errorf("%w: objective not found", ErrAgentRuntimeHostNotFound)
		}
		workspaceID = objective.Objective.WorkspaceID
		if err := ensureRuntimeWorkspaceMatch(requestedWorkspaceID, workspaceID); err != nil {
			return nil, err
		}
		resp.Summary = fmt.Sprintf("Objective: %s", objective.Objective.Name)
		resp.Target.Display = &agentruntime.TargetDisplay{Title: objective.Objective.Name}
		resp.Data = runtimeObjectiveContextData(objective)
	case "support_conversation", "conversation":
		if workspaceID == "" {
			return nil, fmt.Errorf("%w: workspace_id metadata or run mapping is required for support conversation targets", ErrAgentRuntimeHostBadRequest)
		}
		// The runtime host adapter is an authenticated service-to-service path,
		// not a workspace-member inbox view. Use the elevated internal lookup so
		// mailbox membership filtering does not turn a valid conversation into a
		// false not-found response.
		conversation, err := s.supportRepo.GetByID(ctx, workspaceID, target.ID, "", model.RoleOwner)
		if err != nil {
			return nil, err
		}
		if conversation == nil {
			return nil, fmt.Errorf("%w: support conversation not found", ErrAgentRuntimeHostNotFound)
		}
		workspaceID = conversation.WorkspaceID
		if err := ensureRuntimeWorkspaceMatch(requestedWorkspaceID, workspaceID); err != nil {
			return nil, err
		}
		var workspace *model.Workspace
		if s.workspaceRepo != nil {
			workspace, err = s.workspaceRepo.GetByID(ctx, workspaceID)
			if err != nil {
				return nil, err
			}
		}
		resp.Summary = runtimeSupportConversationSummary(conversation, workspace)
		resp.Target.Type = "support_conversation"
		resp.Target.Display = &agentruntime.TargetDisplay{Title: conversation.Subject}
		resp.Data = runtimeSupportConversationContextData(conversation)
		if workspace != nil {
			resp.Data["workspace"] = runtimeWorkspaceContextData(workspace)
			resp.Data["product_context"] = map[string]interface{}{
				"name":                               workspace.Name,
				"website_url":                        agentRuntimeHostString(workspace.WebsiteURL),
				"summary":                            agentRuntimeHostString(workspace.Description),
				"is_current_website_product":         true,
				"resolve_generic_product_references": true,
			}
		}
	case "support_coverage_gap":
		if workspaceID == "" {
			return nil, fmt.Errorf("%w: workspace_id metadata or run mapping is required for support coverage gap targets", ErrAgentRuntimeHostBadRequest)
		}
		if s.supportCoverage == nil {
			return nil, fmt.Errorf("support coverage target resolver is not configured")
		}
		detail, err := s.supportCoverage.GetGapDetail(ctx, workspaceID, target.ID)
		if err != nil {
			return nil, err
		}
		if detail == nil {
			return nil, fmt.Errorf("%w: support coverage gap not found", ErrAgentRuntimeHostNotFound)
		}
		workspaceID = detail.WorkspaceID
		if err := ensureRuntimeWorkspaceMatch(requestedWorkspaceID, workspaceID); err != nil {
			return nil, err
		}
		resp.Summary = supportCoverageGapRunContext(detail, nil)
		resp.Target.Display = &agentruntime.TargetDisplay{Title: detail.Title}
		resp.Data = runtimeSupportCoverageGapContextData(detail)
		if s.workspaceRepo != nil {
			workspace, err := s.workspaceRepo.GetByID(ctx, workspaceID)
			if err != nil {
				return nil, err
			}
			if workspace != nil {
				resp.Data["workspace"] = runtimeWorkspaceContextData(workspace)
				resp.Data["product_context"] = map[string]interface{}{
					"name":                               workspace.Name,
					"website_url":                        agentRuntimeHostString(workspace.WebsiteURL),
					"summary":                            agentRuntimeHostString(workspace.Description),
					"is_current_website_product":         true,
					"resolve_generic_product_references": true,
				}
			}
		}
	case "document":
		doc, err := s.docsRepo.GetByID(ctx, target.ID)
		if err != nil {
			return nil, err
		}
		if doc == nil {
			return nil, fmt.Errorf("%w: document not found", ErrAgentRuntimeHostNotFound)
		}
		workspaceID = doc.WorkspaceID
		if err := ensureRuntimeWorkspaceMatch(requestedWorkspaceID, workspaceID); err != nil {
			return nil, err
		}
		resp.Summary = fmt.Sprintf("Document: %s", doc.Title)
		resp.Target.Display = &agentruntime.TargetDisplay{Title: doc.Title}
		resp.Data = runtimeDocumentContextData(doc)
	case "crm_contact", "contact":
		contact, err := s.crmContactRepo.GetByID(ctx, target.ID)
		if err != nil {
			return nil, err
		}
		if contact == nil {
			return nil, fmt.Errorf("%w: CRM contact not found", ErrAgentRuntimeHostNotFound)
		}
		workspaceID = contact.WorkspaceID
		if err := ensureRuntimeWorkspaceMatch(requestedWorkspaceID, workspaceID); err != nil {
			return nil, err
		}
		title := strings.TrimSpace(contact.FirstName + " " + agentRuntimeHostString(contact.LastName))
		if title == "" {
			title = agentRuntimeHostString(contact.Email)
		}
		resp.Summary = fmt.Sprintf("CRM contact: %s", title)
		resp.Target.Type = "crm_contact"
		resp.Target.Display = &agentruntime.TargetDisplay{Title: title}
		resp.Data = runtimeCRMContactContextData(contact, title)
	case "crm_company", "company":
		company, err := s.crmCompanyRepo.GetByID(ctx, target.ID)
		if err != nil {
			return nil, err
		}
		if company == nil {
			return nil, fmt.Errorf("%w: CRM company not found", ErrAgentRuntimeHostNotFound)
		}
		workspaceID = company.WorkspaceID
		if err := ensureRuntimeWorkspaceMatch(requestedWorkspaceID, workspaceID); err != nil {
			return nil, err
		}
		resp.Summary = fmt.Sprintf("CRM company: %s", company.Name)
		resp.Target.Type = "crm_company"
		resp.Target.Display = &agentruntime.TargetDisplay{Title: company.Name}
		resp.Data = runtimeCRMCompanyContextData(company)
	case "crm_deal", "deal":
		deal, err := s.crmDealRepo.GetByID(ctx, target.ID)
		if err != nil {
			return nil, err
		}
		if deal == nil {
			return nil, fmt.Errorf("%w: CRM deal not found", ErrAgentRuntimeHostNotFound)
		}
		workspaceID = deal.WorkspaceID
		if err := ensureRuntimeWorkspaceMatch(requestedWorkspaceID, workspaceID); err != nil {
			return nil, err
		}
		resp.Summary = fmt.Sprintf("CRM deal: %s", deal.Name)
		resp.Target.Type = "crm_deal"
		resp.Target.Display = &agentruntime.TargetDisplay{Title: deal.Name}
		resp.Data = runtimeCRMDealContextData(deal)
	default:
		resp.Summary = fmt.Sprintf("%s target %s", target.Type, target.ID)
		resp.Data = map[string]interface{}{
			"target_type": target.Type,
			"target_id":   target.ID,
		}
	}

	if resp.Target.Metadata == nil {
		resp.Target.Metadata = map[string]interface{}{}
	}
	if workspaceID != "" {
		resp.Target.Metadata["workspace_id"] = workspaceID
		resp.Data["workspace_id"] = workspaceID
	}
	return resp, nil
}

func runtimeSupportCoverageGapContextData(detail *model.SupportCoverageGapDetail) map[string]interface{} {
	if detail == nil {
		return map[string]interface{}{}
	}
	body := map[string]interface{}{}
	payload, err := json.Marshal(detail)
	if err == nil {
		_ = json.Unmarshal(payload, &body)
	}
	return map[string]interface{}{
		"target_type":          "support_coverage_gap",
		"target_id":            detail.ID,
		"support_coverage_gap": body,
	}
}

func (s *AgentRuntimeHostService) ResolveRepositorySpec(ctx context.Context, req agentruntime.PrepareWorkspaceRequest) (*agentruntime.RepositoryWorkspaceSpec, error) {
	if s == nil || s.gitService == nil {
		return nil, fmt.Errorf("repository workspace provider is not configured")
	}
	if err := s.validateAppID(req.AppID); err != nil {
		return nil, err
	}
	if err := s.rejectUnclaimedCRMPlaybookCallback(ctx, req.RunID, req.Metadata, req.Target.Metadata); err != nil {
		return nil, err
	}
	var contextData map[string]interface{}
	var contextTargetMetadata map[string]interface{}
	if req.TargetContext != nil {
		contextData = req.TargetContext.Data
		contextTargetMetadata = req.TargetContext.Target.Metadata
	}
	workspaceID := runtimeWorkspaceID(req.Metadata, req.Target.Metadata, contextData, contextTargetMetadata)
	run, err := s.helpinRunForRuntimeRun(ctx, req.RunID)
	if err != nil {
		return nil, err
	}
	var mappedWorkspaceID, helpinRunID string
	if run != nil {
		mappedWorkspaceID = strings.TrimSpace(run.WorkspaceID)
		helpinRunID = run.ID
	}
	if workspaceID == "" {
		workspaceID = mappedWorkspaceID
	} else if err := ensureRuntimeWorkspaceMatch(mappedWorkspaceID, workspaceID); err != nil {
		return nil, err
	}
	target := runtimeRepositorySpecTarget(req, contextData, contextTargetMetadata)
	spec, err := s.gitService.ResolveAgentRuntimeRepositorySpec(ctx, workspaceID, target, req.RunID, helpinRunID)
	if err != nil {
		return nil, err
	}
	var runInput model.AgentRunInputPayload
	if spec != nil && run != nil && run.DockChatID != nil && json.Unmarshal(run.Input, &runInput) == nil && runInput.ExecutionEnabled {
		spec.FinalizePolicy = agentruntime.RepositoryFinalizeNone
	}
	if spec != nil && agentRunIsPreview(run) {
		spec.FinalizePolicy = agentruntime.RepositoryFinalizeNone
		if spec.Metadata == nil {
			spec.Metadata = map[string]interface{}{}
		}
		spec.Metadata["delivery_mode"] = "preview"
	}
	return spec, nil
}

// runtimeRepositorySpecTarget restores the concrete repository target for a
// dynamically checked-out repository after an interactive run resumes. Agent
// Runtime keeps the run's product target (for example, a workspace or support
// coverage gap) but records the primary checkout identity in run input
// metadata. Lease validation later calls repository-spec with that original
// product target, so the host adapter must use the explicit checkout metadata
// rather than attempting to resolve the product object as a Git repository.
func runtimeRepositorySpecTarget(req agentruntime.PrepareWorkspaceRequest, contextMaps ...map[string]interface{}) agentruntime.TargetRef {
	target := normalizeRuntimeTarget(req.Target)
	if target.Type == "repository" || target.Type == "task" ||
		strings.TrimSpace(req.WorkspaceMode) != agentruntime.WorkspaceModeRepository {
		return target
	}
	maps := []map[string]interface{}{req.Metadata, target.Metadata}
	maps = append(maps, contextMaps...)
	repositoryID := runtimeMetadataString("repository_id", maps...)
	repoFullName := runtimeMetadataString("repo_full_name", maps...)
	if repositoryID == "" && repoFullName == "" {
		return target
	}
	metadata := make(map[string]interface{}, len(target.Metadata)+5)
	for key, value := range target.Metadata {
		metadata[key] = value
	}
	for _, key := range []string{"repository_id", "repo_full_name", "base_branch", "work_branch", "repo_alias"} {
		if value := runtimeMetadataString(key, maps...); value != "" {
			metadata[key] = value
		}
	}
	return agentruntime.TargetRef{
		Type:     "repository",
		ID:       agentRuntimeHostFirstNonEmpty(repositoryID, repoFullName),
		Metadata: metadata,
	}
}

func (s *AgentRuntimeHostService) ExecuteCommand(ctx context.Context, req agentruntime.CommandExecutionRequest) (*agentruntime.CommandExecutionResponse, error) {
	if s == nil || s.commandService == nil {
		return nil, fmt.Errorf("command service is not configured")
	}
	if err := s.validateAppID(req.Meta.AppID); err != nil {
		return nil, err
	}
	boundTarget := req.Meta.Target
	if boundTarget.Type == "" {
		boundTarget.Type = req.Meta.TargetType
	}
	if boundTarget.ID == "" {
		boundTarget.ID = req.Meta.TargetID
	}
	if scope, err := s.resolveCRMPlaybookCallback(ctx, req.Meta.RunID, req.Meta.AgentID, boundTarget, req.Meta.RunInputMetadata, req.Meta.TargetMetadata, req.Meta.Target.Metadata, req.Meta.WorkspaceMetadata); err != nil || scope != nil {
		if err != nil {
			return nil, err
		}
		if req.Meta.TargetType != "" && req.Meta.TargetType != boundTarget.Type || req.Meta.TargetID != "" && req.Meta.TargetID != boundTarget.ID {
			return nil, ErrAgentRuntimeHostForbidden
		}
		return s.executeCRMPlaybookCommand(ctx, req, scope)
	}
	if err := s.rejectUnclaimedCRMPlaybookCallback(ctx, req.Meta.RunID, req.Meta.RunInputMetadata, req.Meta.TargetMetadata, req.Meta.Target.Metadata); err != nil {
		return nil, err
	}
	meta := model.InternalCommandContext{
		WorkspaceID:  strings.TrimSpace(req.Meta.WorkspaceID),
		ActorID:      strings.TrimSpace(req.Meta.ExternalActorID),
		AuditActorID: runtimeMetadataString("audit_actor_id", req.Meta.RunInputMetadata, req.Meta.TargetMetadata, req.Meta.Target.Metadata),
		AgentID:      strings.TrimSpace(req.Meta.AgentID),
		RunID:        strings.TrimSpace(req.Meta.RunID),
		TargetType:   strings.TrimSpace(req.Meta.TargetType),
		TargetID:     strings.TrimSpace(req.Meta.TargetID),
	}
	if meta.TargetType == "" {
		meta.TargetType = strings.TrimSpace(req.Meta.Target.Type)
	}
	if meta.TargetID == "" {
		meta.TargetID = strings.TrimSpace(req.Meta.Target.ID)
	}
	if meta.WorkspaceID == "" {
		meta.WorkspaceID = runtimeWorkspaceID(req.Meta.WorkspaceMetadata, req.Meta.RunInputMetadata, req.Meta.TargetMetadata, req.Meta.Target.Metadata)
	}
	mappedWorkspaceID, err := s.workspaceIDForRuntimeRun(ctx, meta.RunID)
	if err != nil {
		return nil, err
	}
	if meta.WorkspaceID == "" {
		meta.WorkspaceID = mappedWorkspaceID
	} else if err := ensureRuntimeWorkspaceMatch(mappedWorkspaceID, meta.WorkspaceID); err != nil {
		return nil, err
	}
	if meta.WorkspaceID == "" {
		return nil, fmt.Errorf("%w: workspace_id is required", ErrAgentRuntimeHostBadRequest)
	}
	if err := s.enrichCommandAgentScope(ctx, &meta); err != nil {
		return &agentruntime.CommandExecutionResponse{Error: err.Error()}, nil
	}
	actor, err := s.enrichCommandActor(ctx, &meta)
	if err != nil {
		return &agentruntime.CommandExecutionResponse{Error: err.Error()}, nil
	}
	if len(req.Input) == 0 {
		req.Input = json.RawMessage(`{}`)
	}
	commandCtx := ctx
	if actor != nil {
		commandCtx = authorization.WithActor(commandCtx, actor)
	}
	output, err := s.commandService.Execute(commandCtx, meta, strings.TrimSpace(req.CommandName), req.Input)
	if err != nil {
		return &agentruntime.CommandExecutionResponse{Error: err.Error()}, nil
	}
	return &agentruntime.CommandExecutionResponse{Output: output}, nil
}

func (s *AgentRuntimeHostService) enrichCommandAgentScope(ctx context.Context, meta *model.InternalCommandContext) error {
	if s == nil || s.agentRepo == nil || meta == nil {
		return nil
	}
	if strings.TrimSpace(meta.AgentID) == "" {
		return fmt.Errorf("%w: agent_id is required to resolve command scope", ErrAgentRuntimeHostForbidden)
	}
	if strings.HasSuffix(meta.AgentID, "-execution") {
		run, err := s.helpinRunForRuntimeRun(ctx, meta.RunID)
		if err != nil {
			return err
		}
		var input model.AgentRunInputPayload
		if run == nil || run.WorkspaceID != meta.WorkspaceID || run.DockChatID == nil || json.Unmarshal(run.Input, &input) != nil || !input.ExecutionEnabled || meta.AgentID != run.AgentID+"-execution" {
			return ErrAgentRuntimeHostForbidden
		}
		meta.AgentID = run.AgentID
	}
	agent, err := s.agentRepo.GetByID(ctx, strings.TrimSpace(meta.WorkspaceID), strings.TrimSpace(meta.AgentID))
	if err != nil {
		return fmt.Errorf("resolve command agent scope: %w", err)
	}
	if agent == nil {
		return fmt.Errorf("%w: agent is not available in this workspace", ErrAgentRuntimeHostForbidden)
	}
	teamIDs := agentTeamIDsForScope(agent)
	slices.Sort(teamIDs)
	meta.AgentTeamIDs = teamIDs
	meta.AgentScopeResolved = true
	return nil
}

func (s *AgentRuntimeHostService) ResolveSkillByID(ctx context.Context, req AgentRuntimeSkillLookupRequest) (*AgentRuntimeWorkspaceSkill, error) {
	skillID := strings.TrimSpace(req.SkillID)
	if skillID == "" {
		return nil, fmt.Errorf("%w: skill_id is required", ErrAgentRuntimeHostBadRequest)
	}
	if s == nil {
		return nil, fmt.Errorf("workspace skill lookup is not configured")
	}
	if err := s.validateAppID(req.AppID); err != nil {
		return nil, err
	}
	if skill, handled, err := s.resolveCRMPlaybookSkill(ctx, req); handled {
		return skill, err
	}
	if err := s.rejectUnclaimedCRMPlaybookCallback(ctx, req.RunID, req.Metadata, req.Target.Metadata); err != nil {
		return nil, err
	}
	if strings.HasPrefix(skillID, agentRuntimeHelpinBuiltInSkillIDPrefix) {
		key := strings.TrimPrefix(skillID, agentRuntimeHelpinBuiltInSkillIDPrefix)
		definition, ok := agentcontract.GetBuiltInSkill(key)
		if !ok {
			return nil, fmt.Errorf("%w: workspace skill not found", ErrAgentRuntimeHostNotFound)
		}
		return s.runtimeBuiltInWorkspaceSkill(definition)
	}
	return s.resolveWorkspaceSkill(ctx, req, func(workspaceID string) (*model.WorkspaceSkill, error) {
		return s.skillRepo.GetByID(ctx, workspaceID, skillID)
	})
}

func (s *AgentRuntimeHostService) ResolveActiveSkillByKey(ctx context.Context, req AgentRuntimeSkillLookupRequest) (*AgentRuntimeWorkspaceSkill, error) {
	key := agentcontract.CanonicalBuiltInSkillKey(strings.TrimSpace(req.Key))
	if key == "" {
		return nil, fmt.Errorf("%w: key is required", ErrAgentRuntimeHostBadRequest)
	}
	if s == nil {
		return nil, fmt.Errorf("workspace skill lookup is not configured")
	}
	if err := s.validateAppID(req.AppID); err != nil {
		return nil, err
	}
	if skill, handled, err := s.resolveCRMPlaybookSkill(ctx, req); handled {
		return skill, err
	}
	if err := s.rejectUnclaimedCRMPlaybookCallback(ctx, req.RunID, req.Metadata, req.Target.Metadata); err != nil {
		return nil, err
	}
	// Product-owned built-in skill keys are immutable runtime contracts. Resolve
	// them from the currently deployed package before consulting persisted
	// workspace rows. Older releases materialized built-ins in workspace_skills;
	// allowing one of those rows to win here can silently retain stale required
	// tools or completion-interaction policy across process restarts. Workspace
	// skills remain addressable through their explicit skill IDs.
	if definition, ok := agentcontract.GetBuiltInSkill(key); ok {
		return s.runtimeBuiltInWorkspaceSkill(definition)
	}
	workspaceID, err := s.workspaceIDForSkillLookup(ctx, req)
	if err != nil {
		return nil, err
	}
	if workspaceID != "" && s.skillRepo != nil {
		skill, err := s.skillRepo.GetActiveByKey(ctx, workspaceID, key)
		if err != nil {
			return nil, err
		}
		if skill != nil && !skill.IsArchived {
			return runtimeWorkspaceSkill(skill), nil
		}
	}
	if workspaceID == "" {
		return nil, fmt.Errorf("%w: workspace_id metadata or run mapping is required", ErrAgentRuntimeHostBadRequest)
	}
	return nil, fmt.Errorf("%w: workspace skill not found", ErrAgentRuntimeHostNotFound)
}

func (s *AgentRuntimeHostService) resolveWorkspaceSkill(ctx context.Context, req AgentRuntimeSkillLookupRequest, lookup func(workspaceID string) (*model.WorkspaceSkill, error)) (*AgentRuntimeWorkspaceSkill, error) {
	if s == nil || s.skillRepo == nil {
		return nil, fmt.Errorf("workspace skill lookup is not configured")
	}
	if err := s.validateAppID(req.AppID); err != nil {
		return nil, err
	}
	workspaceID, err := s.workspaceIDForSkillLookup(ctx, req)
	if err != nil {
		return nil, err
	}
	if workspaceID == "" {
		return nil, fmt.Errorf("%w: workspace_id metadata or run mapping is required", ErrAgentRuntimeHostBadRequest)
	}
	skill, err := lookup(workspaceID)
	if err != nil {
		return nil, err
	}
	if skill == nil || skill.IsArchived {
		return nil, fmt.Errorf("%w: workspace skill not found", ErrAgentRuntimeHostNotFound)
	}
	return runtimeWorkspaceSkill(skill), nil
}

func (s *AgentRuntimeHostService) workspaceIDForSkillLookup(ctx context.Context, req AgentRuntimeSkillLookupRequest) (string, error) {
	workspaceID := runtimeWorkspaceID(req.Metadata, req.Target.Metadata)
	mappedWorkspaceID, err := s.workspaceIDForRuntimeRun(ctx, req.RunID)
	if err != nil {
		return "", err
	}
	if workspaceID == "" {
		return mappedWorkspaceID, nil
	}
	if err := ensureRuntimeWorkspaceMatch(mappedWorkspaceID, workspaceID); err != nil {
		return "", err
	}
	return workspaceID, nil
}

func (s *AgentRuntimeHostService) GetSkillPackageObject(ctx context.Context, objectKey string) ([]byte, error) {
	objectKey = strings.TrimSpace(objectKey)
	if s == nil {
		return nil, fmt.Errorf("workspace skill package store is not configured")
	}
	if objectKey == "" {
		return nil, fmt.Errorf("%w: package object key is required", ErrAgentRuntimeHostBadRequest)
	}
	if strings.HasPrefix(objectKey, crmPlaybookPackagePrefix) {
		return s.crmPlaybookPackage(ctx, objectKey)
	}
	if strings.HasPrefix(objectKey, agentRuntimeHelpinBuiltInSkillObjectPrefix) {
		key := strings.TrimSuffix(strings.TrimPrefix(objectKey, agentRuntimeHelpinBuiltInSkillObjectPrefix), ".zip")
		definition, ok := agentcontract.GetBuiltInSkill(key)
		if !ok {
			return nil, fmt.Errorf("%w: workspace skill package not found", ErrAgentRuntimeHostNotFound)
		}
		archive, _, _, err := s.builtInArchives.get(definition)
		if err != nil {
			return nil, err
		}
		return archive, nil
	}
	if s.skillRepo == nil || s.skillStore == nil {
		return nil, fmt.Errorf("workspace skill package store is not configured")
	}
	if !strings.HasPrefix(objectKey, "workspaces/") || strings.Contains(objectKey, "..") {
		return nil, fmt.Errorf("%w: package object key is not allowed", ErrAgentRuntimeHostForbidden)
	}
	skill, err := s.skillRepo.GetActiveByPackageObjectKey(ctx, objectKey)
	if err != nil {
		return nil, err
	}
	if skill == nil {
		return nil, fmt.Errorf("%w: workspace skill package not found", ErrAgentRuntimeHostNotFound)
	}
	payload, err := s.skillStore.GetObject(ctx, objectKey)
	if err != nil {
		return nil, err
	}
	return payload, nil
}

func runtimeWorkspaceSkill(skill *model.WorkspaceSkill) *AgentRuntimeWorkspaceSkill {
	if skill == nil {
		return nil
	}
	return &AgentRuntimeWorkspaceSkill{
		ID:                skill.ID,
		Key:               skill.Key,
		VersionKey:        skill.VersionKey,
		Title:             skill.Title,
		Description:       stringOrDefault(skill.Description, ""),
		SourceKind:        skill.SourceKind,
		Instructions:      skill.Instructions,
		RequiredTools:     parseJSONStringSlice(json.RawMessage(skill.RequiredTools)),
		SupportedRuntimes: parseJSONStringSlice(json.RawMessage(skill.SupportedRuntimes)),
		Interface:         json.RawMessage(skill.InterfaceConfig),
		Policy:            json.RawMessage(skill.PolicyConfig),
		PackageObjectKey:  skill.PackageObjectKey,
		PackageFileName:   skill.PackageFileName,
		PackageChecksum:   skill.PackageChecksum,
		PackageSize:       skill.PackageSize,
		IsArchived:        skill.IsArchived,
	}
}

func (s *AgentRuntimeHostService) runtimeBuiltInWorkspaceSkill(definition agentcontract.SkillDefinition) (*AgentRuntimeWorkspaceSkill, error) {
	archive, checksum, filename, err := s.builtInArchives.get(definition)
	if err != nil {
		return nil, err
	}
	return &AgentRuntimeWorkspaceSkill{
		ID:                agentRuntimeHelpinBuiltInSkillIDPrefix + definition.Key,
		Key:               definition.Key,
		VersionKey:        checksum,
		Title:             definition.Title,
		Description:       definition.Description,
		SourceKind:        model.WorkspaceSkillSourceBuiltIn,
		Instructions:      definition.Instructions,
		RequiredTools:     append([]string(nil), definition.RequiredTools...),
		SupportedRuntimes: append([]string(nil), definition.SupportedRuntimes...),
		Interface:         runtimeSkillInterfaceJSON(definition.Interface),
		Policy:            runtimeSkillPolicyJSON(definition.Policy),
		PackageObjectKey:  agentRuntimeHelpinBuiltInSkillObjectPrefix + definition.Key + ".zip",
		PackageFileName:   filename,
		PackageChecksum:   checksum,
		PackageSize:       int64(len(archive)),
	}, nil
}

func runtimeSkillInterfaceJSON(value agentcontract.SkillInterface) json.RawMessage {
	payload := map[string]interface{}{}
	if strings.TrimSpace(value.DisplayName) != "" {
		payload["display_name"] = strings.TrimSpace(value.DisplayName)
	}
	if strings.TrimSpace(value.ShortDescription) != "" {
		payload["short_description"] = strings.TrimSpace(value.ShortDescription)
	}
	if strings.TrimSpace(value.IconSmall) != "" {
		payload["icon_small"] = strings.TrimSpace(value.IconSmall)
	}
	if strings.TrimSpace(value.IconLarge) != "" {
		payload["icon_large"] = strings.TrimSpace(value.IconLarge)
	}
	if strings.TrimSpace(value.BrandColor) != "" {
		payload["brand_color"] = strings.TrimSpace(value.BrandColor)
	}
	if strings.TrimSpace(value.DefaultPrompt) != "" {
		payload["default_prompt"] = strings.TrimSpace(value.DefaultPrompt)
	}
	return mustMarshalRuntimeHostJSON(payload)
}

func runtimeSkillPolicyJSON(value agentcontract.SkillPolicy) json.RawMessage {
	payload := map[string]interface{}{}
	if value.AllowImplicitInvocation != nil {
		payload["allow_implicit_invocation"] = *value.AllowImplicitInvocation
	}
	if len(value.CompletionRequiresInteractionKinds) > 0 {
		payload["completion_requires_interaction_kinds"] = append([]string(nil), value.CompletionRequiresInteractionKinds...)
	}
	if len(value.InteractionContracts) > 0 {
		contracts := make([]map[string]interface{}, 0, len(value.InteractionContracts))
		for _, contract := range value.InteractionContracts {
			item := map[string]interface{}{}
			if strings.TrimSpace(contract.Kind) != "" {
				item["kind"] = strings.TrimSpace(contract.Kind)
			}
			if strings.TrimSpace(contract.Schema) != "" {
				item["schema"] = strings.TrimSpace(contract.Schema)
			}
			if len(contract.Transports) > 0 {
				transports := make(map[string]interface{}, len(contract.Transports))
				for key, transport := range contract.Transports {
					transportPayload := map[string]interface{}{}
					if strings.TrimSpace(transport.Type) != "" {
						transportPayload["type"] = strings.TrimSpace(transport.Type)
					}
					if strings.TrimSpace(transport.ToolName) != "" {
						transportPayload["tool_name"] = strings.TrimSpace(transport.ToolName)
					}
					if strings.TrimSpace(transport.BlockLabel) != "" {
						transportPayload["block_label"] = strings.TrimSpace(transport.BlockLabel)
					}
					transports[key] = transportPayload
				}
				item["transports"] = transports
			}
			contracts = append(contracts, item)
		}
		payload["interaction_contracts"] = contracts
	}
	return mustMarshalRuntimeHostJSON(payload)
}

func mustMarshalRuntimeHostJSON(value interface{}) json.RawMessage {
	payload, err := json.Marshal(value)
	if err != nil || len(payload) == 0 {
		return json.RawMessage(`{}`)
	}
	return payload
}

func normalizeRuntimeTarget(target agentruntime.TargetRef) agentruntime.TargetRef {
	target.Type = strings.TrimSpace(target.Type)
	target.ID = strings.TrimSpace(target.ID)
	return target
}

func (s *AgentRuntimeHostService) validateAppID(appID string) error {
	expected := strings.TrimSpace(s.appID)
	if expected == "" {
		return nil
	}
	if strings.TrimSpace(appID) != expected {
		return fmt.Errorf("%w: app_id is not allowed", ErrAgentRuntimeHostForbidden)
	}
	return nil
}

func (s *AgentRuntimeHostService) workspaceIDForRuntimeRun(ctx context.Context, runtimeRunID string) (string, error) {
	run, err := s.helpinRunForRuntimeRun(ctx, runtimeRunID)
	if err != nil {
		return "", err
	}
	if run == nil {
		return "", nil
	}
	return strings.TrimSpace(run.WorkspaceID), nil
}

func (s *AgentRuntimeHostService) helpinRunForRuntimeRun(ctx context.Context, runtimeRunID string) (*model.AgentRun, error) {
	runtimeRunID = strings.TrimSpace(runtimeRunID)
	if s == nil || s.runRepo == nil || runtimeRunID == "" {
		return nil, nil
	}
	return s.runRepo.GetByExternalRuntimeID(ctx, agentRuntimeName, runtimeRunID)
}

func ensureRuntimeWorkspaceMatch(requestedWorkspaceID, actualWorkspaceID string) error {
	requestedWorkspaceID = strings.TrimSpace(requestedWorkspaceID)
	actualWorkspaceID = strings.TrimSpace(actualWorkspaceID)
	if requestedWorkspaceID == "" || actualWorkspaceID == "" || requestedWorkspaceID == actualWorkspaceID {
		return nil
	}
	return fmt.Errorf("%w: target does not belong to requested workspace", ErrAgentRuntimeHostForbidden)
}

func runtimeWorkspaceID(maps ...map[string]interface{}) string {
	for _, values := range maps {
		if values == nil {
			continue
		}
		for _, key := range []string{"workspace_id", "workspaceID", "workspaceId"} {
			if value, ok := values[key]; ok {
				if text := strings.TrimSpace(fmt.Sprint(value)); text != "" && text != "<nil>" {
					return text
				}
			}
		}
	}
	return ""
}

func runtimeMetadataString(key string, maps ...map[string]interface{}) string {
	for _, values := range maps {
		if values == nil {
			continue
		}
		if value, ok := values[key]; ok {
			if text := strings.TrimSpace(fmt.Sprint(value)); text != "" && text != "<nil>" {
				return text
			}
		}
	}
	return ""
}

func agentRuntimeHostString(value *string) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(*value)
}

func runtimeWorkspaceContextData(workspace *model.Workspace) map[string]interface{} {
	if workspace == nil {
		return map[string]interface{}{}
	}
	data := map[string]interface{}{
		"id":              workspace.ID,
		"workspace_id":    workspace.ID,
		"name":            workspace.Name,
		"slug":            workspace.Slug,
		"workspace_key":   workspace.WorkspaceKey,
		"organization_id": workspace.OrganizationID,
		"description":     agentRuntimeHostString(workspace.Description),
		"website_url":     agentRuntimeHostString(workspace.WebsiteURL),
	}
	if websiteURL := agentRuntimeHostString(workspace.WebsiteURL); websiteURL != "" {
		data["website_url"] = websiteURL
	}
	companyProductContext := agentRuntimeHostString(workspace.CompanyProductContext)
	if companyProductContext == "" {
		companyProductContext = agentRuntimeHostString(workspace.Description)
	}
	if companyProductContext != "" {
		data["company_product_context"] = companyProductContext
	}
	return data
}

func runtimeSupportConversationSummary(conversation *model.SupportConversation, workspace *model.Workspace) string {
	conversationSummary := "Support conversation"
	if conversation != nil && strings.TrimSpace(conversation.Subject) != "" {
		conversationSummary += ": " + strings.TrimSpace(conversation.Subject)
	}
	if workspace == nil || strings.TrimSpace(workspace.Name) == "" {
		return conversationSummary
	}

	productName := strings.TrimSpace(workspace.Name)
	parts := []string{
		fmt.Sprintf("%s for %s, the product whose website the visitor is currently using.", conversationSummary, productName),
		fmt.Sprintf("Resolve generic references such as 'you', 'your product', and 'your plans' to %s; do not ask which product unless the visitor explicitly names another one.", productName),
	}
	if websiteURL := agentRuntimeHostString(workspace.WebsiteURL); websiteURL != "" {
		parts = append(parts, "Product website: "+websiteURL+".")
	}
	if summary := agentRuntimeHostString(workspace.Description); summary != "" {
		parts = append(parts, "Workspace summary: "+summary)
	}
	return strings.Join(parts, " ")
}

func runtimeTaskContextData(task *model.TaskDetail) map[string]interface{} {
	if task == nil {
		return map[string]interface{}{}
	}
	data := map[string]interface{}{
		"id":                task.Task.ID,
		"workspace_id":      task.Task.WorkspaceID,
		"display_id":        task.Task.DisplayID,
		"task_key":          task.Task.TaskKey,
		"name":              task.Task.Name,
		"description":       agentRuntimeHostString(task.Task.Description),
		"task_type":         task.Task.TaskType,
		"priority":          task.Task.Priority,
		"severity":          task.Task.Severity,
		"workflow_id":       task.Task.WorkflowID,
		"workflow_state_id": task.Task.WorkflowStateID,
		"epic_id":           agentRuntimeHostString(task.Task.EpicID),
		"sprint_id":         agentRuntimeHostString(task.Task.SprintID),
		"team_id":           agentRuntimeHostString(task.Task.TeamID),
		"blocked":           task.Task.Blocked,
		"blocker":           agentRuntimeHostString(task.Task.Blocker),
	}
	if task.State != nil {
		data["state"] = map[string]interface{}{"id": task.State.ID, "name": task.State.Name, "type": task.State.StateType}
	}
	if len(task.Labels) > 0 {
		labels := make([]map[string]interface{}, 0, len(task.Labels))
		for _, label := range task.Labels {
			labels = append(labels, map[string]interface{}{"id": label.ID, "name": label.Name, "color": label.Color})
		}
		data["labels"] = labels
	}
	return data
}

func runtimeEpicContextData(epic *model.EpicWithStats) map[string]interface{} {
	if epic == nil {
		return map[string]interface{}{}
	}
	return map[string]interface{}{
		"id":             epic.Epic.ID,
		"workspace_id":   epic.Epic.WorkspaceID,
		"name":           epic.Epic.Name,
		"description":    agentRuntimeHostString(epic.Epic.Description),
		"external_id":    agentRuntimeHostString(epic.Epic.ExternalID),
		"team_id":        agentRuntimeHostString(epic.Epic.TeamID),
		"health":         epic.Epic.Health,
		"planning_state": epic.Epic.PlanningState,
		"stats":          epic.Stats,
	}
}

func runtimeSprintContextData(sprint *model.SprintWithStats) map[string]interface{} {
	if sprint == nil {
		return map[string]interface{}{}
	}
	labels := make([]map[string]interface{}, 0, len(sprint.Labels))
	for _, label := range sprint.Labels {
		labels = append(labels, map[string]interface{}{
			"id":      label.ID,
			"name":    label.Name,
			"color":   agentRuntimeHostString(label.Color),
			"team_id": agentRuntimeHostString(label.TeamID),
		})
	}
	return map[string]interface{}{
		"id":           sprint.Sprint.ID,
		"workspace_id": sprint.Sprint.WorkspaceID,
		"name":         sprint.Sprint.Name,
		"description":  agentRuntimeHostString(sprint.Sprint.Description),
		"start_date":   runtimeHostDateString(sprint.Sprint.StartDate),
		"end_date":     runtimeHostDateString(sprint.Sprint.EndDate),
		"status":       sprint.Sprint.Status,
		"team_id":      agentRuntimeHostString(sprint.Sprint.TeamID),
		"labels":       labels,
		"stats":        sprint.Stats,
	}
}

func runtimeObjectiveContextData(objective *model.ObjectiveWithDetails) map[string]interface{} {
	if objective == nil {
		return map[string]interface{}{}
	}
	labels := make([]map[string]interface{}, 0, min(len(objective.Labels), agentRuntimeObjectiveCollectionLimit))
	for _, label := range objective.Labels[:min(len(objective.Labels), agentRuntimeObjectiveCollectionLimit)] {
		labels = append(labels, map[string]interface{}{"id": label.ID, "name": label.Name, "color": agentRuntimeHostString(label.Color), "team_id": agentRuntimeHostString(label.TeamID)})
	}
	epics := make([]map[string]interface{}, 0, min(len(objective.Epics), agentRuntimeObjectiveCollectionLimit))
	for _, epic := range objective.Epics[:min(len(objective.Epics), agentRuntimeObjectiveCollectionLimit)] {
		epics = append(epics, map[string]interface{}{"id": epic.Epic.ID, "name": epic.Epic.Name, "team_id": agentRuntimeHostString(epic.Epic.TeamID), "stats": epic.Stats})
	}
	keyResults := make([]map[string]interface{}, 0, min(len(objective.KeyResults), agentRuntimeObjectiveCollectionLimit))
	for _, keyResult := range objective.KeyResults[:min(len(objective.KeyResults), agentRuntimeObjectiveCollectionLimit)] {
		keyResults = append(keyResults, map[string]interface{}{
			"id": keyResult.ID, "name": keyResult.Name, "result_type": keyResult.ResultType,
			"initial_value": keyResult.InitialValue, "current_value": keyResult.CurrentValue,
			"target_value": keyResult.TargetValue, "progress": keyResult.Progress, "note": agentRuntimeHostString(keyResult.Note),
		})
	}
	return map[string]interface{}{
		"id": objective.Objective.ID, "workspace_id": objective.Objective.WorkspaceID, "name": objective.Objective.Name,
		"description": agentRuntimeHostString(objective.Objective.Description), "objective_type": objective.Objective.ObjectiveType,
		"state": objective.Objective.State, "planned_start_date": runtimeHostDateString(objective.Objective.PlannedStartDate),
		"deadline": runtimeHostDateString(objective.Objective.Deadline), "health": objective.Objective.Health,
		"health_comment": agentRuntimeHostString(objective.Objective.HealthComment), "teams": boundedRuntimeStrings(objective.Teams),
		"owners": boundedRuntimeStrings(objective.Owners), "owner_member_ids": boundedRuntimeStrings(objective.OwnerMemberIDs),
		"labels": labels, "epics": epics, "key_results": keyResults, "stats": objective.Stats,
	}
}

func boundedRuntimeStrings(values []string) []string {
	if len(values) == 0 {
		return []string{}
	}
	return append([]string(nil), values[:min(len(values), agentRuntimeObjectiveCollectionLimit)]...)
}

func runtimeHostDateString(value *time.Time) string {
	if value == nil {
		return ""
	}
	return value.UTC().Format("2006-01-02")
}

func runtimeSupportConversationContextData(conversation *model.SupportConversation) map[string]interface{} {
	if conversation == nil {
		return map[string]interface{}{}
	}
	return map[string]interface{}{
		"id":             conversation.ID,
		"workspace_id":   conversation.WorkspaceID,
		"display_id":     conversation.DisplayID,
		"subject":        conversation.Subject,
		"status":         conversation.Status,
		"flow_state":     agentRuntimeHostString(conversation.FlowState),
		"priority":       conversation.Priority,
		"channel":        conversation.Channel,
		"customer_name":  agentRuntimeHostString(conversation.CustomerName),
		"customer_email": agentRuntimeHostString(conversation.CustomerEmail),
		"ai_state":       agentRuntimeHostString(conversation.AIState),
	}
}

func runtimeDocumentContextData(doc *model.DocsDocument) map[string]interface{} {
	if doc == nil {
		return map[string]interface{}{}
	}
	return map[string]interface{}{
		"id":            doc.ID,
		"workspace_id":  doc.WorkspaceID,
		"space_id":      doc.SpaceID,
		"collection_id": agentRuntimeHostString(doc.CollectionID),
		"title":         doc.Title,
		"status":        doc.Status,
		"visibility":    doc.Visibility,
		"excerpt":       agentRuntimeHostString(doc.Excerpt),
		"tags":          doc.Tags,
		"is_locked":     doc.IsLocked,
	}
}

func runtimeCRMContactContextData(contact *model.CRMContact, title string) map[string]interface{} {
	if contact == nil {
		return map[string]interface{}{}
	}
	return map[string]interface{}{
		"id":              contact.ID,
		"workspace_id":    contact.WorkspaceID,
		"display_id":      contact.DisplayID,
		"name":            strings.TrimSpace(title),
		"first_name":      contact.FirstName,
		"last_name":       agentRuntimeHostString(contact.LastName),
		"email":           agentRuntimeHostString(contact.Email),
		"job_title":       agentRuntimeHostString(contact.JobTitle),
		"lifecycle_stage": contact.LifecycleStage,
		"lead_status":     contact.LeadStatus,
		"owner_member_id": agentRuntimeHostString(contact.OwnerMemberID),
	}
}

func runtimeCRMCompanyContextData(company *model.CRMCompany) map[string]interface{} {
	if company == nil {
		return map[string]interface{}{}
	}
	return map[string]interface{}{
		"id":              company.ID,
		"workspace_id":    company.WorkspaceID,
		"display_id":      company.DisplayID,
		"name":            company.Name,
		"domain":          agentRuntimeHostString(company.Domain),
		"industry":        agentRuntimeHostString(company.Industry),
		"employee_count":  company.EmployeeCount,
		"description":     agentRuntimeHostString(company.Description),
		"owner_member_id": agentRuntimeHostString(company.OwnerMemberID),
	}
}

func runtimeCRMDealContextData(deal *model.CRMDeal) map[string]interface{} {
	if deal == nil {
		return map[string]interface{}{}
	}
	data := map[string]interface{}{
		"id":              deal.ID,
		"workspace_id":    deal.WorkspaceID,
		"display_id":      deal.DisplayID,
		"name":            deal.Name,
		"pipeline_id":     deal.PipelineID,
		"stage_id":        deal.StageID,
		"amount":          deal.Amount,
		"currency":        deal.Currency,
		"close_date":      deal.CloseDate,
		"owner_member_id": agentRuntimeHostString(deal.OwnerMemberID),
		"probability":     deal.Probability,
	}
	if deal.Pipeline != nil {
		data["pipeline"] = map[string]interface{}{"id": deal.Pipeline.ID, "name": deal.Pipeline.Name}
	}
	if deal.Stage != nil {
		data["stage"] = map[string]interface{}{"id": deal.Stage.ID, "name": deal.Stage.Name, "stage_type": deal.Stage.StageType}
	}
	return data
}
