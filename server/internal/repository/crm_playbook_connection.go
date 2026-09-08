package repository

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// ConnectionSource reads one consistent selection without reconciling Agents or saving Flows.
func (r *CRMPlaybookRepository) ConnectionSource(ctx context.Context, ws, id string, selection model.CRMPlaybookConnectionSelection) (*model.CRMPlaybookConnectionSource, error) {
	var source *model.CRMPlaybookConnectionSource
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var err error
		source, err = playbookConnectionSource(tx, ws, id, selection, false)
		return err
	}, &sql.TxOptions{ReadOnly: true, Isolation: sql.LevelRepeatableRead})
	return source, err
}

// PublishConnection validates the reviewed settings under row locks and appends a disabled receipt.
// The compiler operates only on supplied records. No network or execution is allowed here.
func (r *CRMPlaybookRepository) PublishConnection(ctx context.Context, ws, id, actor string, req model.PublishCRMPlaybookConnectionRequest,
	fingerprint string, compile func(model.CRMPlaybookConnectionSource) (model.CRMPlaybookConnectionSnapshot, string, error),
) (*model.CRMPlaybookConnectionResult, error) {
	var result *model.CRMPlaybookConnectionResult
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := lockSituationWorkspace(tx, ws); err != nil {
			return err
		}
		// Serialize with policy publication before reading a command receipt.
		var pb model.CRMPlaybook
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("workspace_id = ? AND id = ?", ws, id).Take(&pb).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil
			}
			return err
		}
		var member model.WorkspaceMember
		if err := tx.Where("workspace_id = ? AND id = ? AND status = ?", ws, actor, model.WorkspaceMemberStatusActive).Take(&member).Error; err != nil {
			return ErrCRMPlaybookUnavailable
		}
		var receipt model.CRMPlaybookConnection
		err := tx.Where("workspace_id = ? AND playbook_id = ? AND command_key = ?", ws, id, req.CommandKey).Take(&receipt).Error
		if err == nil {
			if receipt.CommandFingerprint != fingerprint {
				return ErrCRMPlaybookConflict
			}
			result = &model.CRMPlaybookConnectionResult{Connection: receipt, Replayed: true}
			return nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		source, err := playbookConnectionSource(tx, ws, id, req.Selection(), true)
		if err != nil || source == nil {
			return err
		}
		if source.ConnectionVersion != req.ExpectedConnectionVersion {
			return ErrCRMPlaybookStale
		}
		snapshot, reviewed, err := compile(*source)
		if err != nil {
			return err
		}
		if reviewed != req.ReviewFingerprint {
			return ErrCRMPlaybookStale
		}
		receipt = model.CRMPlaybookConnection{ID: uuid.NewString(), WorkspaceID: ws, PlaybookID: id, PlaybookVersionID: source.Policy.ID,
			Version: source.ConnectionVersion + 1, CommandKey: req.CommandKey, CommandFingerprint: fingerprint, Fingerprint: reviewed,
			Snapshot: snapshot, PublishedByMemberID: actor, PublishedAt: time.Now().UTC().Truncate(time.Microsecond)}
		if err := tx.Create(&receipt).Error; err != nil {
			return err
		}
		result = &model.CRMPlaybookConnectionResult{Connection: receipt}
		return nil
	})
	return result, err
}

// Connection loads an exact historical publication; it never substitutes the newest setup.
func (r *CRMPlaybookRepository) Connection(ctx context.Context, ws, id, connectionID string) (*model.CRMPlaybookConnection, error) {
	var connection model.CRMPlaybookConnection
	err := r.db.WithContext(ctx).Where("workspace_id = ? AND playbook_id = ? AND id = ?", ws, id, connectionID).Take(&connection).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &connection, nil
}

// Connections returns bounded receipt metadata; package bytes are not read for history.
func (r *CRMPlaybookRepository) Connections(ctx context.Context, ws, id string, before int64, limit int) (*model.CRMPlaybookConnections, error) {
	result := &model.CRMPlaybookConnections{Data: []model.CRMPlaybookConnection{}}
	query := r.db.WithContext(ctx).Select("id, playbook_id, playbook_version_id, version, fingerprint, execution_enabled, published_by_member_id, published_at").Where("workspace_id = ? AND playbook_id = ?", ws, id)
	if before > 0 {
		query = query.Where("version < ?", before)
	}
	if err := query.Order("version DESC").Limit(limit + 1).Find(&result.Data).Error; err != nil {
		return nil, err
	}
	if len(result.Data) > limit {
		result.Data = result.Data[:limit]
		last := result.Data[limit-1].Version
		result.NextBeforeVersion = &last
	}
	return result, nil
}

func playbookConnectionSource(tx *gorm.DB, ws, id string, s model.CRMPlaybookConnectionSelection, lock bool) (*model.CRMPlaybookConnectionSource, error) {
	source := &model.CRMPlaybookConnectionSource{}
	read := func(dst any, query string, args ...any) error {
		q := tx.Where(query, args...)
		if lock {
			q = q.Clauses(clause.Locking{Strength: "UPDATE"})
		}
		return q.Take(dst).Error
	}
	if err := read(&source.Playbook, "workspace_id = ? AND id = ?", ws, id); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	if source.Playbook.Revision != s.ExpectedRevision || source.Playbook.PublishedVersionID == nil || *source.Playbook.PublishedVersionID != s.PlaybookVersionID {
		return nil, ErrCRMPlaybookStale
	}
	for _, record := range []struct {
		dst any
		id  string
	}{{&source.Policy, s.PlaybookVersionID}, {&source.Flow, s.FlowID}, {&source.Agent, s.AgentID}} {
		if err := read(record.dst, "workspace_id = ? AND id = ?", ws, record.id); err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, nil
			}
			return nil, err
		}
	}
	if source.Policy.PlaybookID != id {
		return nil, ErrCRMPlaybookUnavailable
	}
	// Fail closed if team scope cannot be loaded; do not use legacy missing-table fallbacks.
	if err := tx.Model(&model.AgentTeamAccess{}).Where("agent_id = ?", s.AgentID).Order("team_id ASC").Pluck("team_id", &source.Agent.TeamIDs).Error; err != nil {
		return nil, err
	}
	if err := tx.Model(&model.CRMPlaybookConnection{}).Where("workspace_id = ? AND playbook_id = ?", ws, id).Select("COALESCE(MAX(version),0)").Scan(&source.ConnectionVersion).Error; err != nil {
		return nil, err
	}
	return source, nil
}
