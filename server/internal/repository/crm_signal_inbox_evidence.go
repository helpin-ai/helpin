package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// CRMInboxSignalComposer scores and groups evidence using snapshot-bound reads.
// It must not import situations, schedule work, or change automation policy.
type CRMInboxSignalComposer func(context.Context, *CRMSignalRepository, string, []model.CRMSignal) ([]model.CRMSignalAccountStory, error)

// SetInboxSignalComposer connects the existing presentation scorer to the inbox.
func (r *CRMSituationRepository) SetInboxSignalComposer(compose CRMInboxSignalComposer) {
	r.inboxSignals = compose
}

// InboxSignalGroup reads a currently visible group without creating tracked work.
func (r *CRMSituationRepository) InboxSignalGroup(ctx context.Context, ws, id string) (*model.CRMSignalAccountStory, error) {
	var result *model.CRMSignalAccountStory
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		groups, err := r.inboxSignalGroups(ctx, tx, ws)
		if err != nil {
			return err
		}
		for _, group := range groups {
			if group.ID == id {
				result = &group
				break
			}
		}
		return nil
	}, &sql.TxOptions{ReadOnly: true, Isolation: sql.LevelRepeatableRead})
	return result, err
}

func (r *CRMSituationRepository) inboxSignalGroups(ctx context.Context, tx *gorm.DB, ws string) ([]model.CRMSignalAccountStory, error) {
	if r.inboxSignals == nil {
		return nil, nil
	}
	repo := NewCRMSignalRepository(tx)
	rollout, err := repo.GetSignalRolloutSettings(ctx, ws)
	if err != nil {
		return nil, err
	}
	if rollout.Mode != model.CRMSignalRolloutLive {
		return nil, nil
	}
	signals, err := repo.ListWorkspaceSignalCandidates(ctx, ws, model.CRMSignalListFilters{CommercialOnly: true}, time.Now().UTC(), 0)
	if err != nil {
		return nil, err
	}
	// A tracked objective owns its referenced evidence even when paused/closed.
	// Pending standalone recommendations also represent their evidence already.
	var linked []string
	err = tx.Table("crm_signals e").Select("e.id").Where("e.workspace_id = ?", ws).
		Where(`EXISTS (SELECT 1 FROM crm_situation_references ref JOIN crm_situations s ON s.workspace_id = ref.workspace_id AND s.id = ref.situation_id
			WHERE ref.workspace_id = e.workspace_id AND ((ref.kind = 'signal' AND ref.source_id = e.id) OR (ref.kind = 'suggestion' AND EXISTS (
			SELECT 1 FROM crm_suggestions a WHERE a.workspace_id = ref.workspace_id AND a.id = ref.source_id AND `+inboxSignalMembership(tx, "e.id", "a.signal_ids")+`))))
			OR EXISTS (SELECT 1 FROM crm_suggestions a WHERE a.workspace_id = e.workspace_id AND (`+situationOpenActionSQL+`) AND `+inboxSignalMembership(tx, "e.id", "a.signal_ids")+`)`).
		Pluck("e.id", &linked).Error
	if err != nil {
		return nil, fmt.Errorf("read tracked inbox evidence: %w", err)
	}
	tracked := make(map[string]bool, len(linked))
	for _, id := range linked {
		tracked[id] = true
	}
	untracked := signals[:0]
	for _, signal := range signals {
		if !tracked[signal.ID] && signal.ActedAt == nil {
			untracked = append(untracked, signal)
		}
	}
	return r.inboxSignals(ctx, repo, ws, untracked)
}

type inboxEvidenceRow struct {
	ID             string  `json:"id"`
	Title          string  `json:"title"`
	NextStep       string  `json:"next_step"`
	Category       string  `json:"category"`
	CustomerName   string  `json:"customer_name"`
	OwnerMemberID  *string `json:"owner_member_id"`
	Priority       float64 `json:"priority"`
	EvidenceReview string  `json:"evidence_review"`
	LatestSignalID string  `json:"latest_signal_id"`
	SearchText     string  `json:"search_text"`
}

func inboxIncludesEvidence(state string) bool {
	return state != "needs_approval" && state != "paused" && state != "closed" && state != "waiting"
}

func inboxEvidenceQuery(db *gorm.DB, ws, member string, nav model.CRMSituationListFilters, groups []model.CRMSignalAccountStory) (*gorm.DB, error) {
	rows := make([]inboxEvidenceRow, 0, len(groups))
	for _, group := range groups {
		if len(group.Signals) == 0 {
			continue
		}
		row := inboxEvidenceRow{ID: group.ID, Title: group.Signals[0].Summary, Category: model.CRMSituationCategoryForMotion(group.CommercialMotion), CustomerName: group.AccountName,
			OwnerMemberID: group.OwnerMemberID, Priority: group.Priority, EvidenceReview: "reviewed", LatestSignalID: group.Signals[0].ID}
		if group.RecommendedActionLabel != nil {
			row.NextStep = *group.RecommendedActionLabel
		}
		if group.NeedsJudgment {
			row.NextStep = "Review the conflicting evidence before choosing a next step."
		}
		var summaries []string
		for _, signal := range group.Signals {
			if signal.DetectedAt.Equal(group.LatestDetectedAt) {
				row.LatestSignalID = signal.ID
			}
			summaries = append(summaries, signal.Summary)
			if signal.EvidenceExcerpt != nil {
				summaries = append(summaries, *signal.EvidenceExcerpt)
			}
			if signal.ReviewedAt == nil {
				row.EvidenceReview = "needs_review"
			}
		}
		row.SearchText = strings.Join(append(summaries, group.AccountName, row.NextStep), " ")
		rows = append(rows, row)
	}
	encoded, err := json.Marshal(rows)
	if err != nil {
		return nil, fmt.Errorf("encode inbox evidence: %w", err)
	}
	table := `jsonb_to_recordset(CAST(? AS jsonb)) AS evidence(id text, title text, next_step text, category text, customer_name text, owner_member_id text, priority double precision, evidence_review text, latest_signal_id text, search_text text)`
	if db.Dialector.Name() == "sqlite" {
		var columns []string
		for _, column := range []string{"id", "title", "next_step", "category", "customer_name", "owner_member_id", "priority", "evidence_review", "latest_signal_id", "search_text"} {
			columns = append(columns, "json_extract(value, '$."+column+"') AS "+column)
		}
		table = "(SELECT " + strings.Join(columns, ", ") + " FROM json_each(?)) AS evidence"
	}
	query := db.Table(table, string(encoded)).
		Joins("JOIN crm_signals latest ON CAST(latest.id AS TEXT) = evidence.latest_signal_id AND latest.workspace_id = ?", ws).
		Joins("LEFT JOIN workspace_members owner ON CAST(owner.id AS TEXT) = evidence.owner_member_id AND owner.workspace_id = ?", ws)
	switch nav.Scope {
	case "mine":
		query = query.Where("evidence.owner_member_id = ?", member)
	case "unassigned":
		query = query.Where("owner.id IS NULL OR owner.status <> 'active'")
	case "my_teams":
		query = query.Where(`owner.status = 'active' AND EXISTS (SELECT 1 FROM team_workspace_memberships mine JOIN team_workspace_memberships peer ON peer.team_id = mine.team_id WHERE mine.workspace_member_id = ? AND peer.workspace_member_id = owner.id)`, member)
	}
	if !inboxIncludesEvidence(nav.State) {
		query = query.Where("FALSE")
	}
	return query.Select(`evidence.id, ? AS workspace_id, 'evidence' AS kind, evidence.title, evidence.next_step, evidence.category, evidence.customer_name,
		COALESCE(owner.display_name,'') AS owner_name, evidence.owner_member_id, CASE WHEN owner.status = 'active' THEN TRUE ELSE FALSE END AS owner_available,
		evidence.priority, `+inboxPrioritySQL("evidence.priority")+` AS priority_band, 'open' AS lifecycle, 'needs_context' AS attention,
		0 AS pending_action_count, evidence.evidence_review, latest.detected_at AS created_at, CAST(NULL AS DOUBLE PRECISION) AS approval_confidence, evidence.search_text`, ws), nil
}
