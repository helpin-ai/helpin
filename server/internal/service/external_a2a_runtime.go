package service

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/helpin-ai/helpin/server/internal/model"
)

const (
	// externalA2AMaxTurnSeconds keeps one remote turn inside Agent Runtime's
	// two-hour activity window.
	externalA2AMaxTurnSeconds = 6600
	externalA2AUploadTokenTTL = 24 * time.Hour
	externalA2AUploadPrefix   = "hpa2a_"
	externalA2AUploadPath     = "/api/a2a/uploads"
	// externalA2ACancelGrace keeps connection details available to a remote
	// cancel that reaches Agent Runtime after the run is marked cancelled.
	externalA2ACancelGrace = 10 * time.Minute
)

// ExternalA2AContextRequest identifies the run asking for its per-turn
// connection details in an Agent Runtime target-context callback.
type ExternalA2AContextRequest struct {
	RuntimeRunID string
	// HostRunID is the helpin_run_id metadata hint. Callbacks can arrive
	// before the runtime run ID is bound; the hint narrows, never widens.
	HostRunID   string
	AgentID     string
	WorkspaceID string
	TargetType  string
	TargetID    string
}

// TargetContextData returns the data.a2a object for a non-terminal run of an
// active external agent in the requesting workspace, or nil for every other
// run. The result carries the decrypted token and must never be persisted.
func (s *ExternalA2AService) TargetContextData(ctx context.Context, req ExternalA2AContextRequest) (map[string]any, error) {
	if !s.Enabled() {
		return nil, nil
	}
	run, err := s.resolveContextRun(ctx, req)
	if err != nil || run == nil {
		return nil, err
	}
	active := model.IsAgentRunActiveStatus(run.Status)
	if !isExternalA2ARun(run) || (!active && !s.recentlyCancelled(run)) {
		return nil, nil
	}
	if (req.AgentID != "" && req.AgentID != run.AgentID) || (req.WorkspaceID != "" && req.WorkspaceID != run.WorkspaceID) {
		return nil, nil
	}
	if (req.TargetType != "" && req.TargetType != run.TargetType) || (req.TargetID != "" && req.TargetID != run.TargetID) {
		return nil, nil
	}
	record, err := s.repo.GetByAgentID(ctx, run.WorkspaceID, run.AgentID)
	if err != nil || record == nil || record.Status != model.ExternalA2AStatusActive {
		return nil, err
	}
	token, err := s.decryptToken(record)
	if err != nil {
		return nil, err
	}
	auth := map[string]any{"type": "none"}
	if token != "" {
		auth = map[string]any{"type": "bearer", "token": token}
	}
	contextID, appendix := "", ""
	if run.TaskID != nil && strings.TrimSpace(*run.TaskID) != "" {
		stored, err := s.repo.GetTaskContext(ctx, run.WorkspaceID, *run.TaskID, record.ID)
		if err != nil {
			return nil, err
		}
		if stored != nil {
			contextID = stored.ContextID
		}
		// A cancelled run only needs the connection to cancel the remote task.
		if active && !externalA2ARunCancelling(run) {
			if appendix, err = s.uploadAppendix(ctx, run, record); err != nil {
				return nil, err
			}
		}
	}
	allowPrivate := false
	if u, err := url.Parse(record.InterfaceURL); err == nil {
		allowPrivate = s.client.AllowsPrivateHost(u.Hostname())
	}
	data := map[string]any{
		"external_agent_id":     record.ID,
		"name":                  record.Name,
		"card_url":              record.CardURL,
		"auth":                  auth,
		"context_id":            contextID,
		"message_appendix":      appendix,
		"allow_private_network": allowPrivate,
		"max_turn_seconds":      externalA2AMaxTurnSeconds,
	}
	if len(record.AgentCard) > 0 && string(record.AgentCard) != "{}" {
		data["agent_card"] = json.RawMessage(record.AgentCard)
	}
	return data, nil
}

// recentlyCancelled lets Agent Runtime re-resolve the connection to cancel the
// remote task after Helpin has already marked the run cancelled.
func (s *ExternalA2AService) recentlyCancelled(run *model.AgentRun) bool {
	if run == nil || run.Status != model.AgentRunStatusCancelled {
		return false
	}
	cancelledAt := run.UpdatedAt
	if run.CompletedAt != nil {
		cancelledAt = *run.CompletedAt
	}
	return !cancelledAt.IsZero() && s.now().Sub(cancelledAt) <= externalA2ACancelGrace
}

func externalA2ARunCancelling(run *model.AgentRun) bool {
	return strings.TrimSpace(derefString(run.ExecutionStage)) == "cancelling"
}

func (s *ExternalA2AService) resolveContextRun(ctx context.Context, req ExternalA2AContextRequest) (*model.AgentRun, error) {
	runRepo := s.agents.runRepo
	if runRepo == nil {
		return nil, nil
	}
	runtimeRunID := strings.TrimSpace(req.RuntimeRunID)
	hostRunID := strings.TrimSpace(req.HostRunID)
	if runtimeRunID != "" {
		run, err := runRepo.GetByExternalRuntimeID(ctx, agentRuntimeName, runtimeRunID)
		if err != nil {
			return nil, err
		}
		if run != nil {
			if hostRunID != "" && hostRunID != run.ID {
				return nil, nil
			}
			return run, nil
		}
	}
	if hostRunID == "" {
		return nil, nil
	}
	run, err := runRepo.GetByIDAny(ctx, hostRunID)
	if err != nil || run == nil {
		return nil, err
	}
	// A bound run must be the one making this callback.
	if bound, ok := agentRuntimeRunID(run); ok && bound != runtimeRunID {
		return nil, nil
	}
	return run, nil
}

// uploadAppendix returns the upload-link instructions appended to the first
// message of the run, minting the run's upload token when needed.
func (s *ExternalA2AService) uploadAppendix(ctx context.Context, run *model.AgentRun, record *model.ExternalA2AAgent) (string, error) {
	if s.attachments == nil || s.publicAPI == "" {
		return "", nil
	}
	token, err := s.runUploadToken(ctx, run, record)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("To attach files (videos, screenshots, logs) to the Helpin task, upload each with:\n"+
		"curl -fsS -X POST -H 'Authorization: Bearer %s' -F 'file=@<path>' %s%s\n"+
		"Max %d MB per file; %s.", token, s.publicAPI, externalA2AUploadPath,
		externalA2AMaxFileBytes>>20, strings.Join(externalA2AUploadExtensions(), ", ")), nil
}

// runUploadToken reuses the run's unexpired token or mints one. The token is
// derived from the row ID with the server key, so only its hash is stored.
func (s *ExternalA2AService) runUploadToken(ctx context.Context, run *model.AgentRun, record *model.ExternalA2AAgent) (string, error) {
	now := s.now().UTC()
	existing, err := s.repo.LatestUsableUploadToken(ctx, run.ID, now)
	if err != nil {
		return "", err
	}
	if existing != nil && existing.PMTaskID == derefString(run.TaskID) {
		return s.uploadTokenForRow(existing.ID), nil
	}
	row := &model.A2ARunUploadToken{
		ID: uuid.NewString(), WorkspaceID: run.WorkspaceID, AgentRunID: run.ID, PMTaskID: derefString(run.TaskID),
		ExternalA2AAgentID: record.ID, ExpiresAt: now.Add(externalA2AUploadTokenTTL),
	}
	token := s.uploadTokenForRow(row.ID)
	row.TokenSHA256 = sha256Hex(token)
	if err := s.repo.CreateUploadToken(ctx, row); err != nil {
		return "", err
	}
	slog.InfoContext(ctx, "external agent upload link minted", "workspace_id", run.WorkspaceID, "run_id", run.ID, "external_agent_id", record.ID)
	return token, nil
}

func (s *ExternalA2AService) uploadTokenForRow(rowID string) string {
	mac := hmac.New(sha256.New, s.key)
	mac.Write([]byte("external-a2a|upload|v1|" + rowID))
	return externalA2AUploadPrefix + base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

// RevokeRunUploadTokens ends a run's upload links; it is safe to repeat.
func (s *ExternalA2AService) RevokeRunUploadTokens(ctx context.Context, runID string) error {
	if !s.Enabled() || strings.TrimSpace(runID) == "" {
		return nil
	}
	return s.repo.RevokeRunUploadTokens(ctx, runID, s.now().UTC())
}

func sha256Hex(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}
