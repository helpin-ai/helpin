package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"mime"
	"net/url"
	"path"
	"strings"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
)

const (
	externalA2AMaxProjectedFiles = 20
	externalA2ADispatchTimeout   = 10 * time.Minute
)

// a2aTaskEvent is the data of an a2a.task runtime event.
type a2aTaskEvent struct {
	ExternalAgentID string `json:"external_agent_id"`
	ContextID       string `json:"context_id"`
	TaskID          string `json:"task_id"`
	State           string `json:"state"`
	Message         string `json:"message"`
	MessageID       string `json:"message_id"`
	Files           []struct {
		Name      string `json:"name"`
		MediaType string `json:"media_type"`
		URL       string `json:"url"`
	} `json:"files"`
}

// ProjectA2ATaskEvent applies an a2a.task event for a task-targeted run: it
// remembers the remote context, posts the agent's question or answer as a
// task comment once per message, and imports published files once per URL.
// Replaying the same event has no further effect.
func (s *ExternalA2AService) ProjectA2ATaskEvent(ctx context.Context, run *model.AgentRun, data map[string]any) error {
	if !s.Enabled() || run == nil || !isExternalA2ARun(run) {
		return nil
	}
	var event a2aTaskEvent
	encoded, err := json.Marshal(data)
	if err != nil || json.Unmarshal(encoded, &event) != nil {
		slog.WarnContext(ctx, "ignore malformed a2a.task event", "run_id", run.ID)
		return nil
	}
	record, err := s.repo.GetByAgentID(ctx, run.WorkspaceID, run.AgentID)
	if err != nil || record == nil {
		return err
	}
	if event.ExternalAgentID != "" && event.ExternalAgentID != record.ID {
		slog.WarnContext(ctx, "ignore a2a.task event for another external agent", "run_id", run.ID, "external_agent_id", record.ID)
		return nil
	}
	if run.TargetType != "task" || strings.TrimSpace(derefString(run.TaskID)) == "" {
		return nil
	}
	taskID := strings.TrimSpace(*run.TaskID)
	if contextID := strings.TrimSpace(event.ContextID); contextID != "" {
		if err := s.repo.UpsertTaskContext(ctx, &model.A2ATaskContext{
			WorkspaceID: run.WorkspaceID, TaskID: taskID, ExternalA2AAgentID: record.ID,
			ContextID: contextID, LastRemoteTaskID: strings.TrimSpace(event.TaskID),
		}); err != nil {
			return err
		}
	}
	if err := s.projectA2AComment(ctx, run, record, taskID, event); err != nil {
		return err
	}
	for i, file := range event.Files {
		if i >= externalA2AMaxProjectedFiles {
			break
		}
		s.importA2AFile(ctx, run, record, taskID, file.Name, file.MediaType, file.URL)
	}
	return nil
}

func (s *ExternalA2AService) projectA2AComment(ctx context.Context, run *model.AgentRun, record *model.ExternalA2AAgent, taskID string, event a2aTaskEvent) error {
	state := strings.ToLower(strings.TrimSpace(event.State))
	body := strings.TrimSpace(event.Message)
	kind := ""
	switch state {
	case "input_required":
		kind = "question"
	case "auth_required":
		kind = "question"
		body = strings.TrimSpace("The external agent needs authorization before it can continue. " + body)
	case "completed":
		kind = "answer"
	}
	if kind == "" || body == "" || s.comments == nil {
		return nil
	}
	messageKey := firstNonEmptyString(strings.TrimSpace(event.MessageID), strings.TrimSpace(event.TaskID)+":"+state)
	key := "comment:" + kind + ":" + messageKey
	claimed, err := s.repo.ClaimProjectedItem(ctx, run.WorkspaceID, run.ID, key)
	if err != nil || !claimed {
		return err
	}
	if normalized := normalizeTaskDescriptionRichText(&body); normalized != nil {
		body = *normalized
	}
	agentID, runID := record.AgentID, run.ID
	_, err = s.comments.Create(ctx, model.CreateCommentRequest{
		EntityType: "task", EntityID: taskID, Body: body,
		AgentID: &agentID, AgentName: record.Name, AgentRunID: &runID,
	}, externalA2AAccountableUser(run, record), run.WorkspaceID)
	if err != nil {
		if releaseErr := s.repo.ReleaseProjectedItem(ctx, run.ID, key); releaseErr != nil {
			slog.ErrorContext(ctx, "release a2a comment claim", "run_id", run.ID, "error", releaseErr)
		}
		return fmt.Errorf("post external agent comment: %w", err)
	}
	return nil
}

// importA2AFile claims a file URL for the run and downloads it in the
// background so large files do not hold up event projection. A failed import
// releases its claim so a replayed event retries it.
func (s *ExternalA2AService) importA2AFile(ctx context.Context, run *model.AgentRun, record *model.ExternalA2AAgent, taskID, name, mediaType, rawURL string) {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" || s.attachments == nil {
		return
	}
	key := "file:" + sha256Hex(rawURL)
	claimed, err := s.repo.ClaimProjectedItem(ctx, run.WorkspaceID, run.ID, key)
	if err != nil || !claimed {
		if err != nil {
			slog.ErrorContext(ctx, "claim external agent file", "run_id", run.ID, "error", err)
		}
		return
	}
	runCopy, recordCopy := *run, *record
	s.dispatch(ctx, func(ctx context.Context) {
		if err := s.downloadA2AFile(ctx, &runCopy, &recordCopy, taskID, name, mediaType, rawURL); err != nil {
			slog.WarnContext(ctx, "import external agent file", "workspace_id", runCopy.WorkspaceID, "run_id", runCopy.ID, "error", err)
			if releaseErr := s.repo.ReleaseProjectedItem(ctx, runCopy.ID, key); releaseErr != nil {
				slog.ErrorContext(ctx, "release a2a file claim", "run_id", runCopy.ID, "error", releaseErr)
			}
		}
	})
}

func (s *ExternalA2AService) downloadA2AFile(ctx context.Context, run *model.AgentRun, record *model.ExternalA2AAgent, taskID, name, mediaType, rawURL string) error {
	fileName := externalA2AFileName(name, mediaType, rawURL)
	contentType, ok := externalA2AContentType(fileName)
	if !ok {
		return fmt.Errorf("file type of %q is not allowed", fileName)
	}
	token, err := s.decryptToken(record)
	if err != nil {
		return err
	}
	bearerHost := ""
	if u, err := url.Parse(record.InterfaceURL); err == nil {
		bearerHost = u.Hostname()
	}
	download, err := s.client.Download(ctx, rawURL, bearerHost, token, externalA2AMaxFileBytes)
	if err != nil {
		return err
	}
	defer download.Body.Close()
	spool, size, err := spoolExternalA2AFile(download.Body, contentType)
	if err != nil {
		return err
	}
	defer closeExternalA2ASpool(spool)
	_, err = s.storeAgentFile(ctx, run, record, taskID, fileName, contentType, size, spool)
	return err
}

// externalA2AFileName picks a storable name for a published file, adding an
// extension from the media type when the name lacks one.
func externalA2AFileName(name, mediaType, rawURL string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		if u, err := url.Parse(rawURL); err == nil {
			name = path.Base(u.Path)
		}
	}
	name = sanitizeExternalA2AFileName(name)
	if _, ok := externalA2AContentType(name); ok {
		return name
	}
	if mediaType = strings.TrimSpace(strings.Split(mediaType, ";")[0]); mediaType != "" {
		for ext, known := range externalA2AFileTypes {
			if known == mediaType && ext != ".log" && ext != ".jpg" {
				return strings.TrimSuffix(name, path.Ext(name)) + ext
			}
		}
		if exts, _ := mime.ExtensionsByType(mediaType); len(exts) > 0 {
			return strings.TrimSuffix(name, path.Ext(name)) + exts[0]
		}
	}
	return name
}

// dispatch runs fn in a tracked background goroutine with a bounded context
// detached from the caller's cancellation; Shutdown cancels it via stopCtx.
func (s *ExternalA2AService) dispatch(ctx context.Context, fn func(context.Context)) {
	s.backgroundMu.Lock()
	if s.closing {
		s.backgroundMu.Unlock()
		slog.WarnContext(ctx, "external agent background task skipped during shutdown")
		return
	}
	s.background.Add(1)
	s.backgroundMu.Unlock()
	go func() {
		defer s.background.Done()
		runCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), externalA2ADispatchTimeout)
		defer cancel()
		if s.stopCtx != nil {
			stop := context.AfterFunc(s.stopCtx, cancel)
			defer stop()
		}
		defer func() {
			if recovered := recover(); recovered != nil {
				slog.Error("external agent background task panic", "panic", recovered)
			}
		}()
		fn(runCtx)
	}()
}
