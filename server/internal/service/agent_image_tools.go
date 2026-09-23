package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	_ "image/jpeg" // register decoders for image.DecodeConfig
	_ "image/png"
	"io"
	"log/slog"
	"net/http"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/helpin-ai/helpin/server/internal/aipolicy"
	"github.com/helpin-ai/helpin/server/internal/aiusage"
	"github.com/helpin-ai/helpin/server/internal/llm"
	"github.com/helpin-ai/helpin/server/internal/model"
)

// ErrAgentImagesUnavailable is returned when no OpenAI credential can run image tools.
var ErrAgentImagesUnavailable = errors.New("image tools need an OpenAI connection: add one in Settings → AI")

const (
	agentImageMaxSourceBytes = 50 << 20 // OpenAI limit for all inputs combined
	agentImageMaxSources     = 4
	agentImagePromptPreview  = 2000
)

type agentImagesAPI interface {
	Generate(ctx context.Context, req llm.ImageGenerateRequest) (*llm.ImageResult, error)
	Edit(ctx context.Context, req llm.ImageEditRequest) (*llm.ImageResult, error)
}

type agentImageObjectStore interface {
	PutObject(ctx context.Context, key, contentType string, size int64, body io.Reader, publicRead bool) error
	GetObject(ctx context.Context, key string) ([]byte, error)
	DeleteObject(ctx context.Context, key string) error
}

type agentImageAttachmentReader interface {
	ReadForAskAttachment(ctx context.Context, workspaceID, userID, id string) (*model.PMAttachment, []byte, error)
}

// AgentImageConfig holds the operator's server OpenAI key. AllowServerKey is
// true only in Community, where the operator funds AI directly; Helpin Cloud
// requires the workspace's own OpenAI connection so image usage is never
// funded by an unpriced hosted key.
type AgentImageConfig struct {
	ServerOpenAIKey     string
	ServerOpenAIBaseURL string
	AllowServerKey      bool
}

// AgentImageService runs the generate_image and edit_image agent tools.
type AgentImageService struct {
	cfg         AgentImageConfig
	connections *AIConnectionService
	artifacts   agentRuntimeBrowserArtifactRepository
	store       agentImageObjectStore
	attachments agentImageAttachmentReader
	registry    *aipolicy.Registry
	audit       aipolicy.ExecutionAudit
	newClient   func(apiKey, baseURL string) agentImagesAPI
	now         func() time.Time
}

// NewAgentImageService wires the image tools. connections may be nil in
// deployments without AI connection encryption; then only a Community server
// key can run the tools.
func NewAgentImageService(cfg AgentImageConfig, connections *AIConnectionService, artifacts agentRuntimeBrowserArtifactRepository, store agentImageObjectStore, attachments agentImageAttachmentReader, registry *aipolicy.Registry, audit aipolicy.ExecutionAudit) *AgentImageService {
	// One client for every call; large, high-quality images can take minutes.
	httpClient := &http.Client{Timeout: 4 * time.Minute}
	return &AgentImageService{
		cfg: cfg, connections: connections, artifacts: artifacts, store: store, attachments: attachments,
		registry: registry, audit: audit, now: time.Now,
		newClient: func(apiKey, baseURL string) agentImagesAPI {
			return llm.NewOpenAIImagesClient(apiKey, baseURL, httpClient)
		},
	}
}

type agentImageCredential struct {
	APIKey       string
	BaseURL      string
	Source       string // "server" or "workspace"
	ConnectionID string
	FundingMode  aiusage.FundingMode
}

// credential picks the OpenAI key for a workspace. Community may use the
// operator's server key; otherwise the workspace's shared OpenAI connection is
// used when the edition policy admits it. OpenRouter and ChatGPT connections
// cannot call the Images API.
func (s *AgentImageService) credential(ctx context.Context, workspaceID string) (*agentImageCredential, error) {
	if s.cfg.AllowServerKey && strings.TrimSpace(s.cfg.ServerOpenAIKey) != "" {
		return &agentImageCredential{APIKey: s.cfg.ServerOpenAIKey, BaseURL: s.cfg.ServerOpenAIBaseURL,
			Source: "server", FundingMode: aiusage.FundingCustomerUnbilled}, nil
	}
	if s.connections == nil || s.connections.repo == nil || len(s.connections.key) != 32 {
		return nil, ErrAgentImagesUnavailable
	}
	candidates, err := s.connections.repo.SharedEmbeddingConnections(ctx, workspaceID, []string{"openai"})
	if err != nil {
		return nil, fmt.Errorf("list workspace AI connections: %w", err)
	}
	sortEmbeddingCandidates(workspaceID, candidates)
	for i := range candidates {
		c := &candidates[i]
		if c.WorkspaceID != workspaceID || c.Provider != "openai" || c.Scope != "workspace" || c.UserID != nil ||
			c.SupersededBy != nil || c.Status != "connected" || len(c.EncryptedSecret) == 0 {
			continue
		}
		funding := aiusage.FundingCustomerUnbilled
		if s.connections.admissionPolicy != nil {
			snapshot, err := s.connections.admissionPolicy.ResolveConnectionPolicy(ctx, workspaceID, c)
			if err != nil {
				if errors.Is(err, ErrAIConnectionPolicyUnavailable) {
					continue
				}
				return nil, fmt.Errorf("resolve AI connection policy: %w", err)
			}
			if snapshot == nil || snapshot.FundingMode == "" {
				continue
			}
			funding = snapshot.FundingMode
		}
		secret, err := s.connections.open(c)
		if err != nil || strings.TrimSpace(secret.APIKey) == "" {
			slog.WarnContext(ctx, "skip AI connection for images: credential unavailable", "workspace_id", workspaceID, "connection_id", c.ID)
			continue
		}
		return &agentImageCredential{APIKey: secret.APIKey, Source: "workspace", ConnectionID: c.ID, FundingMode: funding}, nil
	}
	return nil, ErrAgentImagesUnavailable
}

// AgentImageRequest is the validated input shared by both tools.
type AgentImageRequest struct {
	Prompt     string
	Images     []string // edit only
	Mask       string   // edit only
	Aspect     string
	Quality    string
	Background string
	Name       string
}

// AgentImageResult is returned to the model.
type AgentImageResult struct {
	ArtifactID  string `json:"artifact_id"`
	ArtifactRef string `json:"artifact_ref"`
	Markdown    string `json:"markdown"`
	FileName    string `json:"file_name"`
	ContentType string `json:"content_type"`
	Width       int    `json:"width,omitempty"`
	Height      int    `json:"height,omitempty"`
	Model       string `json:"model"`
	Note        string `json:"note"`
}

var agentImageAspectSizes = map[string]string{
	"": "auto", "auto": "auto", "square": "1024x1024", "landscape": "1536x1024", "portrait": "1024x1536", "wide": "1792x1008",
}

// agentImageQualityRoutes maps the tool's quality levels to a model and
// API quality. Draft and standard use the fast model; final the quality model.
var agentImageQualityRoutes = map[string][2]string{
	"":         {llm.OpenAIImageModelFast, "medium"},
	"standard": {llm.OpenAIImageModelFast, "medium"},
	"draft":    {llm.OpenAIImageModelFast, "low"},
	"final":    {llm.OpenAIImageModelQuality, "high"},
}

func (req *AgentImageRequest) normalize(edit bool) error {
	req.Prompt = strings.TrimSpace(req.Prompt)
	if req.Prompt == "" {
		return errCommandInput("prompt is required")
	}
	if utf8.RuneCountInString(req.Prompt) > 8000 {
		return errCommandInput("prompt must be at most 8000 characters")
	}
	if _, ok := agentImageAspectSizes[req.Aspect]; !ok {
		return errCommandInput("aspect must be auto, square, landscape, portrait, or wide")
	}
	if _, ok := agentImageQualityRoutes[req.Quality]; !ok {
		return errCommandInput("quality must be draft, standard, or final")
	}
	switch req.Background {
	case "", "auto", "transparent", "opaque":
	default:
		return errCommandInput("background must be auto, transparent, or opaque")
	}
	if edit {
		if len(req.Images) == 0 || len(req.Images) > agentImageMaxSources {
			return errCommandInput("images must list 1 to %d source images", agentImageMaxSources)
		}
	} else if len(req.Images) > 0 || req.Mask != "" {
		return errCommandInput("generate_image does not take source images; use edit_image")
	}
	return nil
}

// Generate creates an image from text.
func (s *AgentImageService) Generate(ctx context.Context, meta model.InternalCommandContext, run *model.AgentRun, req AgentImageRequest) (*AgentImageResult, error) {
	if err := req.normalize(false); err != nil {
		return nil, err
	}
	return s.run(ctx, meta, run, aipolicy.ActionAgentImageGenerate, req, nil, nil)
}

// Edit changes one or more existing images.
func (s *AgentImageService) Edit(ctx context.Context, meta model.InternalCommandContext, run *model.AgentRun, req AgentImageRequest) (*AgentImageResult, error) {
	if err := req.normalize(true); err != nil {
		return nil, err
	}
	sources := make([]llm.ImageInput, 0, len(req.Images))
	total := 0
	for _, ref := range req.Images {
		input, err := s.loadImage(ctx, meta, ref)
		if err != nil {
			return nil, err
		}
		total += len(input.Data)
		sources = append(sources, *input)
	}
	var mask *llm.ImageInput
	if strings.TrimSpace(req.Mask) != "" {
		loaded, err := s.loadImage(ctx, meta, req.Mask)
		if err != nil {
			return nil, err
		}
		if loaded.ContentType != "image/png" {
			return nil, errCommandInput("mask must be a PNG with an alpha channel")
		}
		total += len(loaded.Data)
		mask = loaded
	}
	if total > agentImageMaxSourceBytes {
		return nil, errCommandInput("source images and mask must total less than 50 MB")
	}
	return s.run(ctx, meta, run, aipolicy.ActionAgentImageEdit, req, sources, mask)
}

func (s *AgentImageService) run(ctx context.Context, meta model.InternalCommandContext, run *model.AgentRun, actionKey string, req AgentImageRequest, sources []llm.ImageInput, mask *llm.ImageInput) (*AgentImageResult, error) {
	if s == nil || s.artifacts == nil || s.store == nil {
		return nil, fmt.Errorf("image storage is not configured")
	}
	if run == nil {
		return nil, errCommandInput("image tools run inside an agent run")
	}
	credential, err := s.credential(ctx, meta.WorkspaceID)
	if err != nil {
		return nil, err
	}
	route := agentImageQualityRoutes[req.Quality]
	modelName, quality := route[0], route[1]
	idempotencyKey := "agent-image:" + uuid.NewString()
	action, err := aipolicy.ResolveExecution(s.registry, aipolicy.ExecutionContext{
		WorkspaceID: meta.WorkspaceID, ActionKey: actionKey, FeatureKey: "agent_images",
		IdempotencyKey: idempotencyKey, Attempt: 1,
	}, aipolicy.Route{Provider: "openai", Model: modelName})
	if err != nil {
		return nil, err
	}
	background := req.Background
	if background == "" {
		background = "auto"
	}
	apiReq := llm.ImageGenerateRequest{
		Model: modelName, Prompt: req.Prompt, Size: agentImageAspectSizes[req.Aspect], Quality: quality,
		Background: background, OutputFormat: "png",
	}
	auditMetadata := map[string]any{
		"run_id": run.ID, "agent_id": run.AgentID, "credential_source": credential.Source,
		"connection_id": credential.ConnectionID, "funding_mode": string(credential.FundingMode),
		"source_images": len(sources), "masked": mask != nil,
	}
	execution := s.startAudit(ctx, meta.WorkspaceID, action, modelName, idempotencyKey, auditMetadata)
	callCtx, cancel := context.WithTimeout(ctx, action.Timeout)
	defer cancel()
	client := s.newClient(credential.APIKey, credential.BaseURL)
	var result *llm.ImageResult
	if actionKey == aipolicy.ActionAgentImageEdit {
		result, err = client.Edit(callCtx, llm.ImageEditRequest{ImageGenerateRequest: apiReq, Images: sources, Mask: mask})
	} else {
		result, err = client.Generate(callCtx, apiReq)
	}
	s.finishAudit(ctx, execution, result, err)
	if err != nil {
		var apiErr *llm.ImageAPIError
		if errors.As(err, &apiErr) && apiErr.StatusCode >= 400 && apiErr.StatusCode < 500 {
			// Moderation and validation failures are actionable by the agent.
			return nil, errCommandInput("%s", apiErr.Message)
		}
		return nil, err
	}
	return s.storeResult(ctx, run, req, modelName, result, auditMetadata)
}

var agentImageNameUnsafe = regexp.MustCompile(`[^a-zA-Z0-9._-]+`)

func (s *AgentImageService) storeResult(ctx context.Context, run *model.AgentRun, req AgentImageRequest, modelName string, result *llm.ImageResult, auditMetadata map[string]any) (*AgentImageResult, error) {
	contentType := http.DetectContentType(result.Data)
	ext := map[string]string{"image/png": ".png", "image/jpeg": ".jpg", "image/webp": ".webp"}[contentType]
	if ext == "" {
		return nil, fmt.Errorf("image provider returned unsupported content %q", contentType)
	}
	width, height := 0, 0
	if config, _, err := image.DecodeConfig(bytes.NewReader(result.Data)); err == nil {
		width, height = config.Width, config.Height
	}
	name := strings.Trim(agentImageNameUnsafe.ReplaceAllString(strings.TrimSpace(req.Name), "-"), "-.")
	if name == "" {
		name = "generated-image"
	}
	if len(name) > 80 {
		name = name[:80]
	}
	fileName := name + ext
	assetID := uuid.NewString()
	objectKey := fmt.Sprintf("agent-runs/%s/%s/images/%s%s", run.WorkspaceID, run.ID, assetID, ext)
	if err := s.store.PutObject(ctx, objectKey, contentType, int64(len(result.Data)), bytes.NewReader(result.Data), false); err != nil {
		return nil, fmt.Errorf("store generated image: %w", err)
	}
	cleanup := func(cause error) error {
		if deleteErr := s.store.DeleteObject(ctx, objectKey); deleteErr != nil {
			return fmt.Errorf("%w; cleanup failed: %v", cause, deleteErr)
		}
		return cause
	}
	sequence, err := s.artifacts.NextSequence(ctx, run.WorkspaceID, run.ID)
	if err != nil {
		return nil, cleanup(err)
	}
	prompt := req.Prompt
	if utf8.RuneCountInString(prompt) > agentImagePromptPreview {
		prompt = string([]rune(prompt)[:agentImagePromptPreview])
	}
	metadata, _ := json.Marshal(map[string]any{
		"artifact_id": assetID, "artifact_ref": artifactReference(assetID), "visibility": "private",
		"file_name": fileName, "content_type": contentType, "size_bytes": len(result.Data),
		"width": width, "height": height, "source": "openai-images", "model": modelName,
		"prompt": prompt, "source_images": req.Images, "credential_source": auditMetadata["credential_source"],
	})
	artifact := &model.AgentRunArtifact{
		ID: assetID, WorkspaceID: run.WorkspaceID, RunID: run.ID,
		ArtifactType: model.AgentRunArtifactTypeGeneratedImage, Format: strings.TrimPrefix(ext, "."),
		StorageMode: "object", ObjectKey: &objectKey, Metadata: metadata, SequenceNo: sequence,
	}
	if err := s.artifacts.Create(ctx, artifact); err != nil {
		return nil, cleanup(err)
	}
	alt := strings.NewReplacer("[", "", "]", "", "\n", " ").Replace(name)
	return &AgentImageResult{
		ArtifactID: assetID, ArtifactRef: artifactReference(assetID),
		Markdown: fmt.Sprintf("![%s](%s)", alt, artifactReference(assetID)),
		FileName: fileName, ContentType: contentType, Width: width, Height: height, Model: modelName,
		Note: "Include the markdown in your reply to show the image. Use insert_document_artifact with this artifact_id to add it to a document, or pass the artifact_id to edit_image to refine it.",
	}, nil
}

// loadImage reads a source image from a run artifact or an Ask attachment the
// actor uploaded. Only the actor's own attachments and workspace artifacts resolve.
func (s *AgentImageService) loadImage(ctx context.Context, meta model.InternalCommandContext, ref string) (*llm.ImageInput, error) {
	id := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(ref), "helpin://artifacts/"))
	id = strings.TrimPrefix(id, "helpin://attachments/")
	if _, err := uuid.Parse(id); err != nil {
		return nil, errCommandInput("image %q is not an artifact or attachment ID", ref)
	}
	var data []byte
	name := "image"
	if artifact, err := s.artifacts.GetByIDAndWorkspace(ctx, meta.WorkspaceID, id); err != nil {
		return nil, err
	} else if artifact != nil {
		if artifact.StorageMode != "object" || artifact.ObjectKey == nil || !isImageArtifact(artifact) {
			return nil, errCommandInput("artifact %s is not an image", id)
		}
		data, err = s.store.GetObject(ctx, *artifact.ObjectKey)
		if err != nil {
			return nil, fmt.Errorf("read image artifact: %w", err)
		}
		name = artifact.ArtifactType
	} else {
		if s.attachments == nil {
			return nil, errCommandNotFound("image")
		}
		attachment, body, err := s.attachments.ReadForAskAttachment(ctx, meta.WorkspaceID, meta.ActorID, id)
		if err != nil || attachment == nil {
			return nil, errCommandNotFound("image")
		}
		data, name = body, attachment.FileName
	}
	contentType := http.DetectContentType(data)
	ext := map[string]string{"image/png": ".png", "image/jpeg": ".jpg", "image/webp": ".webp"}[contentType]
	if ext == "" {
		return nil, errCommandInput("image %s must be PNG, JPEG, or WebP", id)
	}
	if !strings.HasSuffix(strings.ToLower(name), ext) {
		name = strings.TrimSuffix(name, ".") + ext
	}
	return &llm.ImageInput{FileName: name, ContentType: contentType, Data: data}, nil
}

func isImageArtifact(artifact *model.AgentRunArtifact) bool {
	switch artifact.ArtifactType {
	case model.AgentRunArtifactTypeBrowserScreenshot, model.AgentRunArtifactTypeGeneratedImage:
		return true
	case "analysis_output":
		return artifact.Format == "png"
	default:
		return false
	}
}

func (s *AgentImageService) startAudit(ctx context.Context, workspaceID string, action aipolicy.Action, modelName, idempotencyKey string, metadata map[string]any) *model.AIActionExecution {
	if s.audit == nil {
		return nil
	}
	execution, err := s.audit.Start(ctx, &model.AIActionExecution{
		WorkspaceID: workspaceID, ActionKey: action.Key, PolicyVersion: action.PolicyVersion,
		FeatureKey: action.FeatureKey, Category: string(action.Category), Origin: action.Origin,
		Modality: string(action.Modality), Provider: "openai", Model: modelName,
		IdempotencyKey: idempotencyKey, Attempt: 1,
		Status: model.AIActionExecutionRunning, Metadata: mustJSONMetadata(metadata), StartedAt: s.now().UTC(),
	})
	if err != nil {
		slog.WarnContext(ctx, "start image audit failed", "workspace_id", workspaceID, "error", err)
		return nil
	}
	return execution
}

func (s *AgentImageService) finishAudit(ctx context.Context, execution *model.AIActionExecution, result *llm.ImageResult, callErr error) {
	if s.audit == nil || execution == nil {
		return
	}
	finished := aipolicy.ExecutionResult{Status: model.AIActionExecutionSucceeded, CompletedAt: s.now().UTC()}
	if result != nil {
		finished.InputTokens, finished.OutputTokens = result.Usage.InputTokens, result.Usage.OutputTokens
	}
	if callErr != nil {
		finished.Status = model.AIActionExecutionFailed
		finished.FailureClass = aiActionFailureClass(callErr)
		finished.FailureMessage = sanitizeAIActionFailure(callErr)
	}
	if err := s.audit.Finish(ctx, execution.ID, finished); err != nil {
		slog.WarnContext(ctx, "finish image audit failed", "execution_id", execution.ID, "error", err)
	}
}
