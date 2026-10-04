package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	appcrypto "github.com/helpin-ai/helpin/server/internal/crypto"
	"github.com/helpin-ai/helpin/server/internal/externala2a"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

var (
	// ErrExternalA2ADisabled means EXTERNAL_A2A_ENCRYPTION_KEY is not configured.
	ErrExternalA2ADisabled = errors.New("External agents are not configured on this server")
	// ErrExternalA2ANotFound means the connection does not exist in the workspace.
	ErrExternalA2ANotFound = errors.New("external agent not found")
)

// ExternalA2AInputError is a user-correctable request or remote-card problem.
type ExternalA2AInputError struct{ Message string }

func (e *ExternalA2AInputError) Error() string { return e.Message }

func externalA2AInput(format string, args ...any) error {
	return &ExternalA2AInputError{Message: fmt.Sprintf(format, args...)}
}

// ExternalA2AErrorStatus maps service errors to HTTP status codes.
func ExternalA2AErrorStatus(err error) int {
	var input *ExternalA2AInputError
	var upload *ExternalA2AUploadError
	switch {
	case errors.Is(err, ErrExternalA2ADisabled):
		return http.StatusServiceUnavailable
	case errors.Is(err, ErrExternalA2ANotFound):
		return http.StatusNotFound
	case errors.As(err, &upload):
		return upload.Status
	case errors.As(err, &input):
		return http.StatusBadRequest
	default:
		return http.StatusInternalServerError
	}
}

// ExternalA2AServiceConfig configures external A2A agents.
type ExternalA2AServiceConfig struct {
	// EncryptionKey protects stored agent tokens; the feature is off without it.
	EncryptionKey string
	// AllowedPrivateHosts may resolve to private addresses and use plain HTTP.
	AllowedPrivateHosts []string
	// PublicAPIBaseURL is the externally reachable API origin used in upload links.
	PublicAPIBaseURL string
}

// ExternalA2AService manages workspace connections to remote A2A agents and
// the Helpin side of their runs: per-turn connection details, upload links,
// projection of remote task state, and ticket assignment/comment routing.
type ExternalA2AService struct {
	repo        *repository.ExternalA2ARepository
	agents      *AgentService
	comments    *PMCommentService
	attachments *PMAttachmentService
	client      *externala2a.Client
	key         []byte
	publicAPI   string
	now         func() time.Time
	background  sync.WaitGroup
	// backgroundMu orders dispatch against Shutdown so no work starts after
	// Shutdown begins waiting.
	backgroundMu sync.Mutex
	closing      bool
	// stopCtx is cancelled by Shutdown to abort background work still running.
	stopCtx        context.Context
	stopBackground context.CancelFunc
}

// NewExternalA2AService builds the service. A missing encryption key leaves
// the feature disabled; a malformed key is a configuration error.
func NewExternalA2AService(repo *repository.ExternalA2ARepository, agents *AgentService, cfg ExternalA2AServiceConfig) (*ExternalA2AService, error) {
	key, err := parseAES256Key(cfg.EncryptionKey, "EXTERNAL_A2A_ENCRYPTION_KEY")
	if err != nil {
		return nil, err
	}
	if agents != nil && repo != nil {
		// Renames in the generic agent editor keep the connection name in step.
		agents.externalA2ANames = repo
		// Runs of a disabled or failing connection are refused at launch.
		agents.externalA2AConnections = repo
	}
	stopCtx, stopBackground := context.WithCancel(context.Background())
	return &ExternalA2AService{
		repo:           repo,
		agents:         agents,
		client:         externala2a.NewClient(externala2a.Options{AllowedPrivateHosts: cfg.AllowedPrivateHosts}),
		key:            key,
		publicAPI:      strings.TrimRight(strings.TrimSpace(cfg.PublicAPIBaseURL), "/"),
		now:            time.Now,
		stopCtx:        stopCtx,
		stopBackground: stopBackground,
	}, nil
}

// SetTaskCollaborators wires the comment and attachment services used to
// deliver an external agent's answers and files onto tasks.
func (s *ExternalA2AService) SetTaskCollaborators(comments *PMCommentService, attachments *PMAttachmentService) {
	if s != nil {
		s.comments = comments
		s.attachments = attachments
	}
}

// Enabled reports whether external agents can be used.
func (s *ExternalA2AService) Enabled() bool {
	return s != nil && s.repo != nil && s.agents != nil && len(s.key) == 32
}

// Wait blocks until background dispatches (comment-driven resumes and runs,
// file imports) have finished. Tests use it to observe their effects.
func (s *ExternalA2AService) Wait() {
	if s != nil {
		s.background.Wait()
	}
}

// Shutdown stops accepting background work and waits for work in flight. When
// ctx ends first, the remaining work is cancelled and ctx.Err() is returned.
// A cancelled file import releases its claim, so a replayed event retries it.
func (s *ExternalA2AService) Shutdown(ctx context.Context) error {
	if s == nil {
		return nil
	}
	s.backgroundMu.Lock()
	s.closing = true
	s.backgroundMu.Unlock()
	done := make(chan struct{})
	go func() {
		s.background.Wait()
		close(done)
	}()
	select {
	case <-done:
		s.stopBackground()
		return nil
	case <-ctx.Done():
		s.stopBackground()
		<-done
		return ctx.Err()
	}
}

// List returns the workspace's external agents. Tokens are never serialized.
func (s *ExternalA2AService) List(ctx context.Context, workspaceID string) ([]model.ExternalA2AAgent, error) {
	if !s.Enabled() {
		return nil, ErrExternalA2ADisabled
	}
	return s.repo.List(ctx, workspaceID)
}

// Preview fetches and validates an agent card without saving anything.
func (s *ExternalA2AService) Preview(ctx context.Context, req model.PreviewExternalA2AAgentRequest) (*model.ExternalA2ACardSummary, error) {
	if !s.Enabled() {
		return nil, ErrExternalA2ADisabled
	}
	summary, _, err := s.fetchCard(ctx, req.CardURL, req.Token)
	return summary, err
}

// Create connects a remote agent and creates its linked Helpin agent.
func (s *ExternalA2AService) Create(ctx context.Context, workspaceID, actorID string, req model.CreateExternalA2AAgentRequest) (*model.ExternalA2AAgent, error) {
	if !s.Enabled() {
		return nil, ErrExternalA2ADisabled
	}
	if strings.TrimSpace(workspaceID) == "" || strings.TrimSpace(actorID) == "" {
		return nil, externalA2AInput("workspace and actor are required")
	}
	token, err := validateExternalA2AToken(req.Token)
	if err != nil {
		return nil, err
	}
	summary, rawCard, err := s.fetchCard(ctx, req.CardURL, token)
	if err != nil {
		return nil, err
	}
	teamIDs := normalizeServiceTeamIDs(req.AllowedTeamIDs)
	id := uuid.NewString()
	agent, err := s.agents.createExternalA2AAgent(ctx, externalA2AAgentProfile{
		WorkspaceID: workspaceID, ExternalAgentID: id, Name: summary.Name,
		Description: summary.Description, TeamIDs: teamIDs, ActorID: actorID,
	})
	if err != nil {
		return nil, err
	}
	now := s.now().UTC()
	record := &model.ExternalA2AAgent{
		ID: id, WorkspaceID: workspaceID, AgentID: agent.ID, Status: model.ExternalA2AStatusActive,
		CreatedBy: actorID, LastCheckedAt: &now, AllowedTeamIDs: model.JSONBlob(mustJSONStringSlice(teamIDs)),
	}
	applyExternalA2ACard(record, summary, rawCard)
	if record.EncryptedToken, err = s.encryptToken(workspaceID, id, token); err != nil {
		s.rollbackLinkedAgent(ctx, workspaceID, agent.ID, actorID)
		return nil, err
	}
	record.TokenHint = externalA2ATokenHint(token)
	if err := s.repo.Create(ctx, record); err != nil {
		s.rollbackLinkedAgent(ctx, workspaceID, agent.ID, actorID)
		return nil, err
	}
	slog.InfoContext(ctx, "external agent connected", "workspace_id", workspaceID, "external_agent_id", id, "agent_id", agent.ID, "user_id", actorID)
	return s.get(ctx, workspaceID, id)
}

// Update edits the connection name, token, team scope or status.
func (s *ExternalA2AService) Update(ctx context.Context, workspaceID, id, actorID string, req model.UpdateExternalA2AAgentRequest) (*model.ExternalA2AAgent, error) {
	if !s.Enabled() {
		return nil, ErrExternalA2ADisabled
	}
	current, err := s.get(ctx, workspaceID, id)
	if err != nil {
		return nil, err
	}
	updates := map[string]any{}
	var name *string
	var teamIDs *[]string
	if req.Name != nil {
		value := strings.TrimSpace(*req.Name)
		if value == "" || len([]rune(value)) > 120 {
			return nil, externalA2AInput("name is required and cannot exceed 120 characters")
		}
		updates["name"], name = value, &value
	}
	if req.Token != nil {
		token, err := validateExternalA2AToken(*req.Token)
		if err != nil {
			return nil, err
		}
		if updates["encrypted_token"], err = s.encryptToken(workspaceID, id, token); err != nil {
			return nil, err
		}
		updates["token_hint"] = externalA2ATokenHint(token)
	}
	if req.AllowedTeamIDs != nil {
		normalized := normalizeServiceTeamIDs(*req.AllowedTeamIDs)
		updates["allowed_team_ids"], teamIDs = model.JSONBlob(mustJSONStringSlice(normalized)), &normalized
	}
	if req.Status != nil {
		switch status := strings.TrimSpace(*req.Status); status {
		case model.ExternalA2AStatusActive, model.ExternalA2AStatusDisabled:
			updates["status"] = status
			if status == model.ExternalA2AStatusActive {
				updates["last_error"] = ""
			}
		default:
			return nil, externalA2AInput("status must be active or disabled")
		}
	}
	if len(updates) == 0 {
		return current, nil
	}
	if name != nil || teamIDs != nil {
		if err := s.agents.updateExternalA2AAgentProfile(ctx, workspaceID, current.AgentID, actorID, name, nil, teamIDs); err != nil {
			return nil, err
		}
	}
	if err := s.repo.Update(ctx, workspaceID, id, updates); errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrExternalA2ANotFound
	} else if err != nil {
		return nil, err
	}
	return s.get(ctx, workspaceID, id)
}

// RefreshCard re-fetches the agent card and records the outcome.
func (s *ExternalA2AService) RefreshCard(ctx context.Context, workspaceID, id, actorID string) (*model.ExternalA2AAgent, error) {
	if !s.Enabled() {
		return nil, ErrExternalA2ADisabled
	}
	current, err := s.get(ctx, workspaceID, id)
	if err != nil {
		return nil, err
	}
	token, err := s.decryptToken(current)
	if err != nil {
		return nil, err
	}
	now := s.now().UTC()
	summary, rawCard, fetchErr := s.fetchCard(ctx, current.CardURL, token)
	if fetchErr != nil {
		updates := map[string]any{"last_checked_at": now, "last_error": fetchErr.Error()}
		if current.Status == model.ExternalA2AStatusActive {
			updates["status"] = model.ExternalA2AStatusError
		}
		if err := s.repo.Update(ctx, workspaceID, id, updates); err != nil {
			return nil, err
		}
		return s.get(ctx, workspaceID, id)
	}
	refreshed := *current
	applyExternalA2ACard(&refreshed, summary, rawCard)
	updates := map[string]any{
		"description": refreshed.Description, "interface_url": refreshed.InterfaceURL,
		"protocol_binding": refreshed.ProtocolBinding, "protocol_version": refreshed.ProtocolVersion,
		"provider_name": refreshed.ProviderName, "version": refreshed.Version, "skills": refreshed.Skills,
		"capabilities": refreshed.Capabilities, "agent_card": refreshed.AgentCard,
		"last_checked_at": now, "last_error": "",
	}
	if current.Status == model.ExternalA2AStatusError {
		updates["status"] = model.ExternalA2AStatusActive
	}
	if err := s.repo.Update(ctx, workspaceID, id, updates); err != nil {
		return nil, err
	}
	if err := s.agents.updateExternalA2AAgentProfile(ctx, workspaceID, current.AgentID, actorID, nil, &refreshed.Description, nil); err != nil {
		slog.WarnContext(ctx, "sync external agent description", "workspace_id", workspaceID, "external_agent_id", id, "error", err)
	}
	return s.get(ctx, workspaceID, id)
}

// Delete removes the connection by deleting its linked Helpin agent the way
// agents are normally deleted; the connection row cascades with it.
func (s *ExternalA2AService) Delete(ctx context.Context, workspaceID, id, actorID string) error {
	if !s.Enabled() {
		return ErrExternalA2ADisabled
	}
	current, err := s.get(ctx, workspaceID, id)
	if err != nil {
		return err
	}
	if err := s.agents.DeleteAgent(ctx, workspaceID, current.AgentID, actorID); err != nil {
		return err
	}
	// Databases without the cascade (for example SQLite tests) still drop it.
	if err := s.repo.Delete(ctx, workspaceID, id); err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	slog.InfoContext(ctx, "external agent deleted", "workspace_id", workspaceID, "external_agent_id", id, "agent_id", current.AgentID, "user_id", actorID)
	return nil
}

func (s *ExternalA2AService) get(ctx context.Context, workspaceID, id string) (*model.ExternalA2AAgent, error) {
	record, err := s.repo.Get(ctx, workspaceID, id)
	if err != nil {
		return nil, err
	}
	if record == nil {
		return nil, ErrExternalA2ANotFound
	}
	return record, nil
}

func (s *ExternalA2AService) rollbackLinkedAgent(ctx context.Context, workspaceID, agentID, actorID string) {
	if err := s.agents.DeleteAgent(ctx, workspaceID, agentID, actorID); err != nil {
		slog.ErrorContext(ctx, "roll back external agent", "workspace_id", workspaceID, "agent_id", agentID, "error", err)
	}
}

func (s *ExternalA2AService) fetchCard(ctx context.Context, rawURL, token string) (*model.ExternalA2ACardSummary, []byte, error) {
	if strings.TrimSpace(rawURL) == "" {
		return nil, nil, externalA2AInput("card_url is required")
	}
	cardURL, err := s.client.NormalizeCardURL(rawURL)
	if err != nil {
		return nil, nil, externalA2AInput("card_url: %s", err.Error())
	}
	fetchCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	card, raw, err := s.client.FetchCard(fetchCtx, cardURL, token)
	if err != nil {
		return nil, nil, externalA2AInput("%s", err.Error())
	}
	if _, err := s.client.ValidateURL(card.InterfaceURL); err != nil {
		return nil, nil, externalA2AInput("agent card interface URL: %s", err.Error())
	}
	summary := &model.ExternalA2ACardSummary{
		Name: card.Name, Description: card.Description, CardURL: cardURL, InterfaceURL: card.InterfaceURL,
		ProtocolBinding: card.ProtocolBinding, ProtocolVersion: card.ProtocolVersion,
		ProviderName: card.ProviderName, Version: card.Version, Skills: []model.ExternalA2ASkill{},
		Capabilities: model.ExternalA2ACapabilities{Streaming: card.Streaming, PushNotifications: card.PushNotifications},
	}
	for _, skill := range card.Skills {
		summary.Skills = append(summary.Skills, model.ExternalA2ASkill{ID: skill.ID, Name: skill.Name, Description: skill.Description})
	}
	return summary, raw, nil
}

func applyExternalA2ACard(record *model.ExternalA2AAgent, summary *model.ExternalA2ACardSummary, rawCard []byte) {
	if record.Name == "" {
		record.Name = summary.Name
	}
	record.Description = summary.Description
	record.CardURL = summary.CardURL
	record.InterfaceURL = summary.InterfaceURL
	record.ProtocolBinding = summary.ProtocolBinding
	record.ProtocolVersion = summary.ProtocolVersion
	record.ProviderName = summary.ProviderName
	record.Version = summary.Version
	record.Skills, _ = json.Marshal(summary.Skills)
	record.Capabilities, _ = json.Marshal(summary.Capabilities)
	record.AgentCard = compactExternalA2ACard(rawCard)
}

func compactExternalA2ACard(raw []byte) model.JSONBlob {
	var card map[string]any
	if json.Unmarshal(raw, &card) != nil || card == nil {
		return model.JSONBlob(`{}`)
	}
	encoded, err := json.Marshal(card)
	if err != nil {
		return model.JSONBlob(`{}`)
	}
	return encoded
}

func validateExternalA2AToken(token string) (string, error) {
	token = strings.TrimSpace(token)
	if len(token) > 8<<10 {
		return "", externalA2AInput("token cannot exceed 8192 bytes")
	}
	if strings.ContainsAny(token, "\r\n") {
		return "", externalA2AInput("token cannot contain line breaks")
	}
	return token, nil
}

func externalA2ATokenHint(token string) string {
	if token == "" {
		return ""
	}
	if len(token) <= 8 {
		return "…"
	}
	return "…" + token[len(token)-4:]
}

func (s *ExternalA2AService) aad(workspaceID, id string) []byte {
	return []byte("external-a2a|v1|" + workspaceID + "|" + id + "|token")
}

func (s *ExternalA2AService) encryptToken(workspaceID, id, token string) (string, error) {
	if token == "" {
		return "", nil
	}
	return appcrypto.EncryptStringWithAAD(token, s.key, s.aad(workspaceID, id))
}

func (s *ExternalA2AService) decryptToken(record *model.ExternalA2AAgent) (string, error) {
	if record == nil || record.EncryptedToken == "" {
		return "", nil
	}
	token, err := appcrypto.DecryptStringWithAAD(record.EncryptedToken, s.key, s.aad(record.WorkspaceID, record.ID))
	if err != nil {
		return "", fmt.Errorf("decrypt external agent token: %w", err)
	}
	return token, nil
}
