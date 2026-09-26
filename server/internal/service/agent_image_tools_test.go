package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"image"
	"image/png"
	"io"
	"strings"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/aipolicy"
	"github.com/helpin-ai/helpin/server/internal/llm"
	"github.com/helpin-ai/helpin/server/internal/model"
)

const (
	imageTestWorkspace  = "ws-1"
	imageTestScreenshot = "11111111-1111-4111-8111-111111111111"
	imageTestAttachment = "22222222-2222-4222-8222-222222222222"
	imageTestRecording  = "33333333-3333-4333-8333-333333333333"
)

func testPNG(t *testing.T, w, h int) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := png.Encode(&buf, image.NewRGBA(image.Rect(0, 0, w, h))); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

type fakeImageStore struct {
	objects map[string][]byte
	deleted []string
	putErr  error
}

func (s *fakeImageStore) PutObject(_ context.Context, key, _ string, _ int64, body io.Reader, _ bool) error {
	if s.putErr != nil {
		return s.putErr
	}
	data, _ := io.ReadAll(body)
	s.objects[key] = data
	return nil
}
func (s *fakeImageStore) GetObject(_ context.Context, key string) ([]byte, error) {
	data, ok := s.objects[key]
	if !ok {
		return nil, errors.New("missing object")
	}
	return data, nil
}
func (s *fakeImageStore) DeleteObject(_ context.Context, key string) error {
	s.deleted = append(s.deleted, key)
	delete(s.objects, key)
	return nil
}

type fakeImageArtifacts struct {
	byID      map[string]*model.AgentRunArtifact
	created   []*model.AgentRunArtifact
	createErr error
}

func (r *fakeImageArtifacts) NextSequence(context.Context, string, string) (int, error) {
	return len(r.created) + 1, nil
}
func (r *fakeImageArtifacts) Create(_ context.Context, artifact *model.AgentRunArtifact) error {
	if r.createErr != nil {
		return r.createErr
	}
	r.created = append(r.created, artifact)
	return nil
}
func (r *fakeImageArtifacts) GetByIDAndWorkspace(_ context.Context, workspaceID, id string) (*model.AgentRunArtifact, error) {
	artifact := r.byID[id]
	if artifact == nil || artifact.WorkspaceID != workspaceID {
		return nil, nil
	}
	return artifact, nil
}

type fakeImageAttachments struct {
	owner string
	data  []byte
}

func (a *fakeImageAttachments) ReadForAskAttachment(_ context.Context, workspaceID, userID, id string) (*model.PMAttachment, []byte, error) {
	if workspaceID != imageTestWorkspace || userID != a.owner || id != imageTestAttachment {
		return nil, nil, errors.New("Ask attachment is unavailable")
	}
	return &model.PMAttachment{ID: id, FileName: "upload"}, a.data, nil
}

type fakeImagesAPI struct {
	generate *llm.ImageGenerateRequest
	edit     *llm.ImageEditRequest
	result   *llm.ImageResult
	err      error
}

func (f *fakeImagesAPI) Generate(_ context.Context, req llm.ImageGenerateRequest) (*llm.ImageResult, error) {
	f.generate = &req
	return f.result, f.err
}
func (f *fakeImagesAPI) Edit(_ context.Context, req llm.ImageEditRequest) (*llm.ImageResult, error) {
	f.edit = &req
	return f.result, f.err
}

type fakeImageAudit struct {
	started  []*model.AIActionExecution
	finished []aipolicy.ExecutionResult
}

func (a *fakeImageAudit) Start(_ context.Context, execution *model.AIActionExecution) (*model.AIActionExecution, error) {
	execution.ID = "exec-" + execution.ActionKey
	a.started = append(a.started, execution)
	return execution, nil
}
func (a *fakeImageAudit) Finish(_ context.Context, _ string, result aipolicy.ExecutionResult) error {
	a.finished = append(a.finished, result)
	return nil
}

type imageTestHarness struct {
	svc       *AgentImageService
	store     *fakeImageStore
	artifacts *fakeImageArtifacts
	api       *fakeImagesAPI
	audit     *fakeImageAudit
	gotKey    string
	meta      model.InternalCommandContext
	run       *model.AgentRun
}

func newImageTestHarness(t *testing.T) *imageTestHarness {
	t.Helper()
	screenshotKey := "agent-runs/ws-1/run-1/browser/shot.png"
	recordingKey := "agent-runs/ws-1/run-1/browser/rec.mp4"
	h := &imageTestHarness{
		store: &fakeImageStore{objects: map[string][]byte{screenshotKey: testPNG(t, 64, 32), recordingKey: []byte("mp4")}},
		artifacts: &fakeImageArtifacts{byID: map[string]*model.AgentRunArtifact{
			imageTestScreenshot: {ID: imageTestScreenshot, WorkspaceID: imageTestWorkspace, ArtifactType: model.AgentRunArtifactTypeBrowserScreenshot, StorageMode: "object", ObjectKey: &screenshotKey, Format: "png"},
			imageTestRecording:  {ID: imageTestRecording, WorkspaceID: imageTestWorkspace, ArtifactType: model.AgentRunArtifactTypeBrowserRecording, StorageMode: "object", ObjectKey: &recordingKey, Format: "mp4"},
		}},
		api:   &fakeImagesAPI{result: &llm.ImageResult{Data: testPNG(t, 48, 16), Usage: llm.ImageUsage{InputTokens: 10, OutputTokens: 400}}},
		audit: &fakeImageAudit{},
		meta:  model.InternalCommandContext{WorkspaceID: imageTestWorkspace, ActorID: "user-1"},
		run:   &model.AgentRun{ID: "run-1", WorkspaceID: imageTestWorkspace},
	}
	h.svc = NewAgentImageService(AgentImageConfig{ServerOpenAIKey: "server-key", AllowServerKey: true}, nil,
		h.artifacts, h.store, &fakeImageAttachments{owner: "user-1", data: testPNG(t, 8, 8)}, aipolicy.DefaultRegistry(), h.audit)
	h.svc.newClient = func(apiKey, _ string) agentImagesAPI {
		h.gotKey = apiKey
		return h.api
	}
	return h
}

func TestAgentImageGenerateStoresPrivateArtifact(t *testing.T) {
	h := newImageTestHarness(t)
	result, err := h.svc.Generate(context.Background(), h.meta, h.run, AgentImageRequest{
		Prompt: "Explain event sourcing as a river of events", Aspect: "wide", Quality: "final", Name: "Event sourcing!",
	})
	if err != nil {
		t.Fatal(err)
	}
	if h.gotKey != "server-key" {
		t.Fatalf("expected the Community server key, got %q", h.gotKey)
	}
	req := h.api.generate
	if req == nil || req.Model != llm.OpenAIImageModelQuality || req.Quality != "high" || req.Size != "1792x1008" || req.OutputFormat != "png" {
		t.Fatalf("generate request %+v", req)
	}
	if len(h.artifacts.created) != 1 {
		t.Fatalf("created %d artifacts", len(h.artifacts.created))
	}
	artifact := h.artifacts.created[0]
	if artifact.ArtifactType != model.AgentRunArtifactTypeGeneratedImage || artifact.RunID != "run-1" || artifact.Format != "png" ||
		!strings.HasPrefix(*artifact.ObjectKey, "agent-runs/ws-1/run-1/images/") {
		t.Fatalf("artifact %+v", artifact)
	}
	if _, ok := h.store.objects[*artifact.ObjectKey]; !ok {
		t.Fatal("image bytes were not stored")
	}
	if result.Width != 48 || result.Height != 16 || result.FileName != "Event-sourcing.png" ||
		result.Markdown != "![Event-sourcing](helpin://artifacts/"+artifact.ID+")" {
		t.Fatalf("result %+v", result)
	}
	var metadata map[string]any
	_ = json.Unmarshal(artifact.Metadata, &metadata)
	if metadata["visibility"] != "private" || metadata["model"] != llm.OpenAIImageModelQuality || metadata["credential_source"] != "server" {
		t.Fatalf("metadata %v", metadata)
	}
	if len(h.audit.started) != 1 || h.audit.started[0].ActionKey != aipolicy.ActionAgentImageGenerate ||
		h.audit.started[0].Modality != "image" || len(h.audit.finished) != 1 || h.audit.finished[0].OutputTokens != 400 ||
		h.audit.finished[0].Status != model.AIActionExecutionSucceeded {
		t.Fatalf("audit started=%+v finished=%+v", h.audit.started, h.audit.finished)
	}
}

func TestAgentImageDefaultsUseFastModel(t *testing.T) {
	h := newImageTestHarness(t)
	if _, err := h.svc.Generate(context.Background(), h.meta, h.run, AgentImageRequest{Prompt: "icon"}); err != nil {
		t.Fatal(err)
	}
	if req := h.api.generate; req.Model != llm.OpenAIImageModelFast || req.Quality != "medium" || req.Size != "auto" || req.Background != "auto" {
		t.Fatalf("defaults %+v", req)
	}
}

func TestAgentImageEditLoadsScreenshotsAndOwnAttachments(t *testing.T) {
	h := newImageTestHarness(t)
	_, err := h.svc.Edit(context.Background(), h.meta, h.run, AgentImageRequest{
		Prompt: "Blur the email addresses; keep everything else identical",
		Images: []string{"helpin://artifacts/" + imageTestScreenshot, imageTestAttachment},
	})
	if err != nil {
		t.Fatal(err)
	}
	req := h.api.edit
	if req == nil || len(req.Images) != 2 || req.Images[0].ContentType != "image/png" || req.Images[1].FileName != "upload.png" {
		t.Fatalf("edit request %+v", req)
	}
	if h.audit.started[0].ActionKey != aipolicy.ActionAgentImageEdit {
		t.Fatalf("action %s", h.audit.started[0].ActionKey)
	}
}

func TestAgentImageEditRejectsUnusableSources(t *testing.T) {
	h := newImageTestHarness(t)
	for name, ref := range map[string]string{
		"not an ID":             "screenshot.png",
		"non-image artifact":    imageTestRecording,
		"another user's upload": imageTestAttachment,
		"unknown in workspace":  "44444444-4444-4444-8444-444444444444",
	} {
		t.Run(name, func(t *testing.T) {
			meta := h.meta
			if name == "another user's upload" {
				meta.ActorID = "someone-else"
			}
			if _, err := h.svc.Edit(context.Background(), meta, h.run, AgentImageRequest{Prompt: "x", Images: []string{ref}}); err == nil {
				t.Fatal("expected an error")
			}
			if h.api.edit != nil {
				t.Fatal("the provider must not be called")
			}
		})
	}
}

func TestAgentImageValidatesInput(t *testing.T) {
	h := newImageTestHarness(t)
	five := []string{imageTestScreenshot, imageTestScreenshot, imageTestScreenshot, imageTestScreenshot, imageTestScreenshot}
	cases := map[string]func() error{
		"empty prompt": func() error {
			_, err := h.svc.Generate(context.Background(), h.meta, h.run, AgentImageRequest{Prompt: " "})
			return err
		},
		"unknown aspect": func() error {
			_, err := h.svc.Generate(context.Background(), h.meta, h.run, AgentImageRequest{Prompt: "x", Aspect: "ultrawide"})
			return err
		},
		"unknown quality": func() error {
			_, err := h.svc.Generate(context.Background(), h.meta, h.run, AgentImageRequest{Prompt: "x", Quality: "max"})
			return err
		},
		"generate with sources": func() error {
			_, err := h.svc.Generate(context.Background(), h.meta, h.run, AgentImageRequest{Prompt: "x", Images: []string{imageTestScreenshot}})
			return err
		},
		"edit without sources": func() error {
			_, err := h.svc.Edit(context.Background(), h.meta, h.run, AgentImageRequest{Prompt: "x"})
			return err
		},
		"too many sources": func() error {
			_, err := h.svc.Edit(context.Background(), h.meta, h.run, AgentImageRequest{Prompt: "x", Images: five})
			return err
		},
	}
	for name, call := range cases {
		t.Run(name, func(t *testing.T) {
			var commandErr *CommandError
			if err := call(); !errors.As(err, &commandErr) || commandErr.Kind != CommandErrorInvalidInput {
				t.Fatalf("expected an input error, got %v", err)
			}
		})
	}
	if h.api.generate != nil || h.api.edit != nil {
		t.Fatal("invalid input must not reach the provider")
	}
}

func TestAgentImageServerKeyOnlyWhenAllowed(t *testing.T) {
	h := newImageTestHarness(t)
	h.svc.cfg.AllowServerKey = false // Helpin Cloud: the hosted key is never used
	if _, err := h.svc.Generate(context.Background(), h.meta, h.run, AgentImageRequest{Prompt: "x"}); !errors.Is(err, ErrAgentImagesUnavailable) {
		t.Fatalf("expected ErrAgentImagesUnavailable, got %v", err)
	}
	if h.api.generate != nil {
		t.Fatal("the provider must not be called without a workspace connection")
	}
}

func TestAgentImageModerationFailureIsActionable(t *testing.T) {
	h := newImageTestHarness(t)
	h.api.result, h.api.err = nil, &llm.ImageAPIError{StatusCode: 400, Code: "moderation_blocked", Message: "Your request was rejected by the safety system."}
	_, err := h.svc.Generate(context.Background(), h.meta, h.run, AgentImageRequest{Prompt: "x"})
	var commandErr *CommandError
	if !errors.As(err, &commandErr) || !strings.Contains(commandErr.Message, "safety system") {
		t.Fatalf("expected an actionable input error, got %v", err)
	}
	if len(h.audit.finished) != 1 || h.audit.finished[0].Status != model.AIActionExecutionFailed {
		t.Fatalf("failed call must be audited: %+v", h.audit.finished)
	}
	if len(h.artifacts.created) != 0 {
		t.Fatal("no artifact on failure")
	}
}

func TestAgentImageCleansUpObjectWhenArtifactCreateFails(t *testing.T) {
	h := newImageTestHarness(t)
	h.artifacts.createErr = errors.New("db down")
	if _, err := h.svc.Generate(context.Background(), h.meta, h.run, AgentImageRequest{Prompt: "x"}); err == nil {
		t.Fatal("expected an error")
	}
	if len(h.store.deleted) != 1 || !strings.Contains(h.store.deleted[0], "/images/") {
		t.Fatalf("stored object must be removed, deleted=%v", h.store.deleted)
	}
}
