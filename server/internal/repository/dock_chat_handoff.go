package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/helpin-ai/helpin/server/internal/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var ErrHandoffConflict = errors.New("conversation handoff changed or lease expired")

const MaxHandoffBytes = 24000

// DockChatHandoffRepository provides storage only. Callers must authorize the
// conversation AND source content before generation or prompt injection.
type DockChatHandoffRepository struct {
	db  *gorm.DB
	now func() time.Time
}

func NewDockChatHandoffRepository(db *gorm.DB) *DockChatHandoffRepository {
	return &DockChatHandoffRepository{db: db, now: time.Now}
}

type HandoffLease struct {
	WorkspaceID, DockChatID, Token, AccessScope string
	Revision, CoveredSequence                   int64
	// SourceThrough is the frozen contiguous delivered prefix at acquisition.
	// Generate from records read AFTER acquiring the lease and at or below it.
	SourceThrough int64
}

func (r *DockChatHandoffRepository) Get(ctx context.Context, workspaceID, chatID string) (*model.DockChatHandoff, error) {
	var state model.DockChatHandoff
	err := r.db.WithContext(ctx).Where("workspace_id = ? AND dock_chat_id = ?", workspaceID, chatID).Take(&state).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &state, err
}

// Acquire serializes maintenance without holding a DB transaction during an
// LLM request. Revision zero initializes a record. A changed authorization
// snapshot requires explicit invalidation, not reuse of the old narrative.
func (r *DockChatHandoffRepository) Acquire(ctx context.Context, workspaceID, chatID, scope string, revision int64, ttl time.Duration) (*HandoffLease, error) {
	if workspaceID == "" || chatID == "" || strings.TrimSpace(scope) == "" || revision < 0 || ttl <= 0 || ttl > 5*time.Minute {
		return nil, fmt.Errorf("invalid handoff lease request")
	}
	var lease HandoffLease
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Consistent lock order: chat, then derived state. Also excludes creation
		// for nonexistent/foreign/archived chats.
		var chat struct{ ID string }
		query := tx.Model(&model.DockChat{}).Select("id").Where("workspace_id = ? AND id = ? AND archived_at IS NULL", workspaceID, chatID)
		if tx.Dialector.Name() == "postgres" {
			query = query.Clauses(clause.Locking{Strength: "UPDATE"})
		}
		if err := query.Take(&chat).Error; err != nil {
			return err
		}
		state := model.DockChatHandoff{WorkspaceID: workspaceID, DockChatID: chatID, FormatVersion: 1, AccessScope: scope}
		if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&state).Error; err != nil {
			return err
		}
		now, token := r.now().UTC(), uuid.NewString()
		result := tx.Model(&model.DockChatHandoff{}).
			Where("workspace_id = ? AND dock_chat_id = ? AND revision = ? AND access_scope = ? AND format_version = 1", workspaceID, chatID, revision, scope).
			Where("lease_expires_at IS NULL OR lease_expires_at <= ?", now).
			Updates(map[string]any{"lease_token": token, "lease_expires_at": now.Add(ttl)})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return ErrHandoffConflict
		}
		if err := tx.Where("workspace_id = ? AND dock_chat_id = ?", workspaceID, chatID).Take(&state).Error; err != nil {
			return err
		}
		through, err := handoffDeliveredPrefix(tx, workspaceID, chatID, state.CoveredSequence)
		if err != nil {
			return err
		}
		if err := tx.Model(&model.DockChatHandoff{}).
			Where("workspace_id = ? AND dock_chat_id = ?", workspaceID, chatID).
			Update("lease_through_sequence", through).Error; err != nil {
			return err
		}
		lease = HandoffLease{WorkspaceID: workspaceID, DockChatID: chatID, Token: token, AccessScope: scope, Revision: revision, CoveredSequence: state.CoveredSequence, SourceThrough: through}
		return nil
	})
	return &lease, err
}

// Publish advances coverage only with a bounded validated payload. Newer
// messages remain in the uncovered tail. It never writes run output_summary.
func (r *DockChatHandoffRepository) Publish(ctx context.Context, lease HandoffLease, through int64, content model.DockChatHandoffContent, generator, promptVersion string) error {
	if through <= lease.CoveredSequence || generator == "" || promptVersion == "" {
		return fmt.Errorf("handoff must advance coverage and identify its generator")
	}
	if through > lease.SourceThrough {
		return fmt.Errorf("handoff coverage exceeds the lease's delivered source prefix")
	}
	payload, err := json.Marshal(content)
	if err != nil {
		return err
	}
	if len(payload) > MaxHandoffBytes {
		return fmt.Errorf("handoff exceeds size limit")
	}
	sources := make(map[string]bool)
	claimCount := 0
	for _, claims := range [][]model.HandoffClaim{content.Objective, content.Constraints, content.Corrections, content.Decisions, content.PendingWork, content.Progress} {
		for _, claim := range claims {
			claimCount++
			if strings.TrimSpace(claim.Text) == "" || (!claim.Assumption && len(claim.SourceIDs) == 0) {
				return fmt.Errorf("handoff claim requires text and sources or an assumption label")
			}
			for _, id := range claim.SourceIDs {
				sources[id] = true
			}
		}
	}
	if claimCount == 0 || claimCount > 64 || len(sources) > 128 {
		return fmt.Errorf("handoff requires 1 to 64 claims and at most 128 distinct sources")
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Only persisted, delivered messages in this exact conversation may be
		// cited. A model cannot invent source IDs or use another tenant's ID.
		if len(sources) > 0 {
			ids := make([]string, 0, len(sources))
			for id := range sources {
				ids = append(ids, id)
			}
			var count int64
			if err := tx.Model(&model.AgentRunMessage{}).Where("workspace_id = ? AND dock_chat_id = ? AND id IN ? AND dock_chat_sequence <= ? AND delivery_status = 'sent'", lease.WorkspaceID, lease.DockChatID, ids, through).Count(&count).Error; err != nil {
				return err
			}
			if count != int64(len(ids)) {
				return fmt.Errorf("handoff source is unavailable")
			}
		}
		var count int64
		if err := tx.Model(&model.AgentRunMessage{}).Distinct("dock_chat_sequence").Where("workspace_id = ? AND dock_chat_id = ? AND dock_chat_sequence > ? AND dock_chat_sequence <= ? AND delivery_status = 'sent'", lease.WorkspaceID, lease.DockChatID, lease.CoveredSequence, through).Count(&count).Error; err != nil {
			return err
		}
		if count != through-lease.CoveredSequence {
			return fmt.Errorf("handoff coverage source is unavailable")
		}
		result := r.leased(tx, lease).Updates(map[string]any{
			"revision": lease.Revision + 1, "previous_revision": lease.Revision,
			"covered_sequence": through, "payload": model.JSONBlob(payload),
			"generator": generator, "prompt_version": promptVersion,
			"lease_token": "", "lease_expires_at": nil, "lease_through_sequence": 0, "failure_code": "",
		})
		return handoffWriteResult(result)
	})
}

// Fail leaves the last valid revision/coverage intact. Only fixed failure codes
// are accepted: provider errors and prompts must never enter this metadata.
func (r *DockChatHandoffRepository) Fail(ctx context.Context, lease HandoffLease, code string) error {
	switch code {
	case "generation_failed", "invalid_output", "source_unavailable", "budget_exceeded":
	default:
		return fmt.Errorf("invalid handoff failure code")
	}
	return handoffWriteResult(r.leased(r.db.WithContext(ctx), lease).Updates(map[string]any{"lease_token": "", "lease_expires_at": nil, "lease_through_sequence": 0, "failure_code": code}))
}

// Invalidate erases derived text and revokes in-flight generations. The host
// must call this on reset/retention/access changes before permitting reuse.
func (r *DockChatHandoffRepository) Invalidate(ctx context.Context, workspaceID, chatID, newScope string, revision int64) error {
	if strings.TrimSpace(newScope) == "" {
		return fmt.Errorf("handoff access scope is required")
	}
	return handoffWriteResult(r.db.WithContext(ctx).Model(&model.DockChatHandoff{}).
		Where("workspace_id = ? AND dock_chat_id = ? AND revision = ?", workspaceID, chatID, revision).
		Updates(map[string]any{"revision": revision + 1, "previous_revision": revision, "access_scope": newScope,
			"payload": nil, "covered_sequence": 0, "generator": "", "prompt_version": "",
			"lease_token": "", "lease_expires_at": nil, "lease_through_sequence": 0, "failure_code": ""}))
}

func (r *DockChatHandoffRepository) leased(tx *gorm.DB, lease HandoffLease) *gorm.DB {
	return tx.Model(&model.DockChatHandoff{}).
		Where("lease_through_sequence = ?", lease.SourceThrough).
		Where("workspace_id = ? AND dock_chat_id = ? AND revision = ? AND covered_sequence = ? AND access_scope = ? AND lease_token = ? AND lease_token <> '' AND lease_expires_at > ? AND format_version = 1", lease.WorkspaceID, lease.DockChatID, lease.Revision, lease.CoveredSequence, lease.AccessScope, lease.Token, r.now().UTC())
}

// handoffDeliveredPrefix bounds source scanning and stops at the first missing
// or undelivered sequence. Sequence allocation precedes insertion/delivery, so
// MAX(sequence) is not a safe coverage cursor. Gaps require recovery; they must
// never be silently retired. A full batch leaves the remaining rows uncovered.
func handoffDeliveredPrefix(tx *gorm.DB, workspaceID, chatID string, after int64) (int64, error) {
	var rows []struct {
		DockChatSequence int64
		DeliveryStatus   string
	}
	if err := tx.Model(&model.AgentRunMessage{}).Select("dock_chat_sequence", "delivery_status").
		Where("workspace_id = ? AND dock_chat_id = ? AND dock_chat_sequence > ?", workspaceID, chatID, after).
		Order("dock_chat_sequence ASC").Limit(1000).Find(&rows).Error; err != nil {
		return 0, fmt.Errorf("read handoff source prefix: %w", err)
	}
	through := after
	for _, row := range rows {
		if row.DockChatSequence != through+1 || row.DeliveryStatus != "sent" {
			break
		}
		through = row.DockChatSequence
	}
	return through, nil
}

func handoffWriteResult(result *gorm.DB) error {
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return ErrHandoffConflict
	}
	return nil
}
