package repository

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/querybuilder"
)

// List returns Playbooks with current Signal lifecycle counts, before pagination.
func (r *CRMPlaybookRepository) List(ctx context.Context, ws string, filters model.CRMPlaybookListFilters) (*model.CRMPlaybookList, error) {
	result := &model.CRMPlaybookList{Data: []model.CRMPlaybookItem{}, Page: filters.Page, PageSize: filters.PageSize}
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		query := playbookReadQuery(tx, ws)
		if filters.Search != "" {
			pattern := "%" + strings.NewReplacer("!", "!!", "%", "!%", "_", "!_").Replace(strings.ToLower(filters.Search)) + "%"
			query = query.Where("LOWER(p.draft->>'name') LIKE ? ESCAPE '!'", pattern)
		}
		switch filters.State {
		case "draft":
			query = query.Where("p.published_version_id IS NULL")
		case "accepting":
			query = query.Where("p.accepting_customers = ?", true)
		case "stopped":
			query = query.Where("p.published_version_id IS NOT NULL AND p.accepting_customers = ?", false)
		}
		if err := query.Session(&gorm.Session{}).Select("COUNT(*)").Scan(&result.Total).Error; err != nil {
			return err
		}
		return query.Order("p.created_at DESC, p.id ASC").Limit(filters.PageSize).Offset((filters.Page - 1) * filters.PageSize).Scan(&result.Data).Error
	}, &sql.TxOptions{ReadOnly: true, Isolation: sql.LevelRepeatableRead})
	return result, err
}

// Get reads a draft and its published definition without mutating either.
func (r *CRMPlaybookRepository) Get(ctx context.Context, ws, id string) (*model.CRMPlaybookItem, error) {
	var item *model.CRMPlaybookItem
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var found model.CRMPlaybookItem
		err := playbookReadQuery(tx, ws).Where("p.id = ?", id).Take(&found).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		if err != nil {
			return err
		}
		if found.Playbook.PublishedVersionID != nil {
			var version model.CRMPlaybookVersion
			if err := tx.Where("workspace_id = ? AND playbook_id = ? AND id = ?", ws, id, *found.Playbook.PublishedVersionID).Take(&version).Error; err != nil {
				return err
			}
			found.PublishedVersion = &version
		}
		item = &found
		return nil
	}, &sql.TxOptions{ReadOnly: true, Isolation: sql.LevelRepeatableRead})
	return item, err
}

// History reads immutable configuration changes using a revision cursor.
func (r *CRMPlaybookRepository) History(ctx context.Context, ws, id string, before int64, limit int) (*model.CRMPlaybookHistory, error) {
	result := &model.CRMPlaybookHistory{Data: []model.CRMPlaybookChange{}}
	query := r.db.WithContext(ctx).Where("workspace_id = ? AND playbook_id = ?", ws, id)
	if before > 0 {
		query = query.Where("revision < ?", before)
	}
	if err := query.Order("revision DESC").Limit(limit + 1).Find(&result.Data).Error; err != nil {
		return nil, err
	}
	if len(result.Data) > limit {
		result.Data = result.Data[:limit]
		value := result.Data[limit-1].Revision
		result.NextBeforeRevision = &value
	}
	return result, nil
}

// Versions reads frozen published definitions using a version cursor.
func (r *CRMPlaybookRepository) Versions(ctx context.Context, ws, id string, before int64, limit int) (*model.CRMPlaybookVersions, error) {
	result := &model.CRMPlaybookVersions{Data: []model.CRMPlaybookVersion{}}
	query := r.db.WithContext(ctx).Where("workspace_id = ? AND playbook_id = ?", ws, id)
	if before > 0 {
		query = query.Where("version < ?", before)
	}
	if err := query.Order("version DESC").Limit(limit + 1).Find(&result.Data).Error; err != nil {
		return nil, err
	}
	if len(result.Data) > limit {
		result.Data = result.Data[:limit]
		value := result.Data[limit-1].Version
		result.NextBeforeVersion = &value
	}
	return result, nil
}

// Preview evaluates the exact stored draft revision against current, unassigned-to-Playbook Signals.
func (r *CRMPlaybookRepository) Preview(ctx context.Context, ws, id, member, versionID string, revision int64, page, size int) (*model.CRMPlaybookPreview, error) {
	var result *model.CRMPlaybookPreview
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var pb model.CRMPlaybook
		err := tx.Where("workspace_id = ? AND id = ?", ws, id).Take(&pb).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		if err != nil {
			return err
		}
		if pb.Revision != revision {
			return ErrCRMPlaybookStale
		}
		definition := pb.Draft
		var selectedVersion *string
		if versionID != "" {
			if pb.PublishedVersionID == nil || *pb.PublishedVersionID != versionID {
				return ErrCRMPlaybookStale
			}
			var version model.CRMPlaybookVersion
			if err := tx.Where("workspace_id = ? AND playbook_id = ? AND id = ?", ws, id, versionID).Take(&version).Error; err != nil {
				return err
			}
			definition, selectedVersion = version.Definition, &versionID
		}
		query, err := eligiblePlaybookSignals(tx, ws, definition)
		if err != nil {
			return err
		}
		list, err := listSituationsFromQuery(tx, ws, member, model.CRMSituationListFilters{Scope: "all", State: "open", Category: "all", Page: page, PageSize: size}, query)
		if err != nil {
			return err
		}
		result = &model.CRMPlaybookPreview{VersionID: selectedVersion, PlaybookRevision: pb.Revision, Definition: definition, Signals: *list, Scope: "existing_signals", ExecutionEnabled: false}
		return nil
	}, &sql.TxOptions{ReadOnly: true, Isolation: sql.LevelRepeatableRead})
	return result, err
}

func playbookReadQuery(db *gorm.DB, ws string) *gorm.DB {
	return db.Table("crm_playbooks p").Where("p.workspace_id = ?", ws).
		Joins(`LEFT JOIN (SELECT playbook_id, workspace_id,
            SUM(CASE WHEN lifecycle = 'open' THEN 1 ELSE 0 END) AS open_count,
            SUM(CASE WHEN lifecycle = 'paused' THEN 1 ELSE 0 END) AS paused_count,
            SUM(CASE WHEN lifecycle = 'closed' THEN 1 ELSE 0 END) AS closed_count
            FROM crm_situations WHERE workspace_id = ? AND playbook_id IS NOT NULL
            GROUP BY workspace_id, playbook_id) participants ON participants.workspace_id = p.workspace_id AND participants.playbook_id = p.id`, ws).
		Select("p.*, COALESCE(participants.open_count,0) AS open_count, COALESCE(participants.paused_count,0) AS paused_count, COALESCE(participants.closed_count,0) AS closed_count")
}

func eligiblePlaybookSignals(tx *gorm.DB, ws string, definition model.CRMPlaybookDefinition) (*gorm.DB, error) {
	query, err := matchingPlaybookSignals(tx, ws, definition)
	if err != nil {
		return nil, err
	}
	return query.Where("s.playbook_id IS NULL"), nil
}

func matchingPlaybookSignals(tx *gorm.DB, ws string, definition model.CRMPlaybookDefinition) (*gorm.DB, error) {
	query := situationReadQuery(tx, ws).Where("s.lifecycle = 'open'").
		Where("s.commercial_motion IN ?", definition.Eligibility.CommercialMotions).
		Where("c.id IS NOT NULL OR contact.id IS NOT NULL OR d.id IS NOT NULL")
	return querybuilder.ApplyGORM(query, definition.Eligibility.Filter, playbookEligibilityDefinitions())
}

func validatePlaybookReferences(tx *gorm.DB, ws string, definition model.CRMPlaybookDefinition) error {
	if id := definition.Responsibilities.EscalationMemberID; id != nil {
		if err := situationSourceExists(tx, "workspace_members", ws, *id); err != nil {
			return err
		}
	}
	if _, err := eligiblePlaybookSignals(tx, ws, definition); err != nil {
		return err
	}
	if definition.Eligibility.Filter == nil {
		return nil
	}
	for _, rule := range definition.Eligibility.Filter.Rules {
		table := map[string]string{"company_id": "crm_companies", "contact_id": "crm_contacts", "deal_id": "crm_deals", "pipeline_id": "crm_pipelines", "owner_member_id": "workspace_members"}[rule.Field]
		if table == "" && rule.Field != "stage_id" {
			continue
		}
		values := append([]string(nil), rule.Values...)
		if rule.Value != nil {
			values = append(values, *rule.Value)
		}
		for _, value := range values {
			parsed, err := uuid.Parse(value)
			if err != nil || parsed == uuid.Nil || parsed.String() != value {
				return ErrCRMSituationInvalidReference
			}
			if rule.Field != "stage_id" {
				if err := situationSourceExists(tx, table, ws, value); err != nil {
					return err
				}
				continue
			}
			var count int64
			if err := tx.Table("crm_pipeline_stages stage").Joins("JOIN crm_pipelines pipeline ON pipeline.id = stage.pipeline_id").Where("pipeline.workspace_id = ? AND stage.id = ?", ws, value).Count(&count).Error; err != nil {
				return err
			}
			if count != 1 {
				return ErrCRMSituationInvalidReference
			}
		}
	}
	return nil
}
