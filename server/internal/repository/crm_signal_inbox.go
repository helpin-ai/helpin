package repository

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/querybuilder"
)

// ListInbox composes work, unlinked recommendations, and untracked evidence in one repeatable-read snapshot.
// It never invokes source projection, changes ownership, or claims an approval.
func (r *CRMSituationRepository) ListInbox(ctx context.Context, ws, member string, filters model.CRMSignalInboxFilters) (*model.CRMSignalInboxList, error) {
	var result *model.CRMSignalInboxList
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var groups []model.CRMSignalAccountStory
		var err error
		if inboxIncludesEvidence(filters.Navigation.State) {
			groups, err = r.inboxSignalGroups(ctx, tx, ws)
			if err != nil {
				return err
			}
		}
		result, err = listSignalInbox(tx, ws, member, filters, groups)
		return err
	}, &sql.TxOptions{ReadOnly: true, Isolation: sql.LevelRepeatableRead})
	return result, err
}

// InboxRecommendation resolves old links without creating work as a side effect.
func (r *CRMSituationRepository) InboxRecommendation(ctx context.Context, ws, id string) (*model.CRMInboxRecommendation, error) {
	action, err := NewCRMSuggestionRepository(r.db).GetByID(ctx, ws, id)
	if err != nil || action == nil {
		return nil, err
	}
	// The suggestion endpoint historically exposed execution dependency errors.
	// Keep status without leaking those internals through this consolidated surface.
	if action.ExecutionError != nil {
		message := "Execution needs review. Check the action's targets and configuration."
		action.ExecutionError = &message
	}
	result := &model.CRMInboxRecommendation{Action: *action, LinkedSituations: []string{}}
	err = r.db.WithContext(ctx).Table("crm_situation_references ref").
		Joins("JOIN crm_situations s ON s.workspace_id = ref.workspace_id AND s.id = ref.situation_id").
		Where("ref.workspace_id = ? AND ref.kind = 'suggestion' AND ref.source_id = ?", ws, id).
		Order("s.created_at ASC, s.id ASC").Pluck("s.id", &result.LinkedSituations).Error
	return result, err
}

func listSignalInbox(db *gorm.DB, ws, member string, filters model.CRMSignalInboxFilters, groups []model.CRMSignalAccountStory) (*model.CRMSignalInboxList, error) {
	nav := filters.Navigation
	work := inboxSituations(db, ws, member, nav)
	proposals := inboxStandalone(db, ws, member, nav)
	base := db.Table("(? UNION ALL ?) AS inbox", work, proposals)
	if len(groups) > 0 {
		evidence, err := inboxEvidenceQuery(db, ws, member, nav, groups)
		if err != nil {
			return nil, err
		}
		base = db.Table("(? UNION ALL ? UNION ALL ?) AS inbox", work, proposals, evidence)
	}
	if nav.Search != "" {
		pattern := "%" + strings.NewReplacer("!", "!!", "%", "!%", "_", "!_").Replace(strings.ToLower(nav.Search)) + "%"
		base = base.Where("LOWER(inbox.search_text) LIKE ? ESCAPE '!'", pattern)
	}
	var err error
	base, err = querybuilder.ApplyGORM(base, nav.Query, inboxQueryDefinitions())
	if err != nil {
		return nil, err
	}
	if nav.State == "needs_approval" {
		base = base.Where("inbox.pending_action_count > 0")
	}
	result := &model.CRMSignalInboxList{Data: []model.CRMSignalInboxItem{}, Page: nav.Page, PageSize: nav.PageSize, CategoryCounts: map[string]int64{"all": 0, "sales": 0, "onboarding_adoption": 0, "expansion": 0, "retention": 0}}
	var facets []struct {
		Category string
		Total    int64
	}
	if err := base.Session(&gorm.Session{}).Select("inbox.category, COUNT(*) AS total").Group("inbox.category").Scan(&facets).Error; err != nil {
		return nil, fmt.Errorf("count signal inbox: %w", err)
	}
	for _, facet := range facets {
		result.CategoryCounts["all"] += facet.Total
		if facet.Category == "" {
			result.UncategorizedCount += facet.Total
		} else {
			result.CategoryCounts[facet.Category] += facet.Total
		}
	}
	result.Total = result.CategoryCounts[nav.Category]
	if nav.Category != "all" {
		base = base.Where("inbox.category = ?", nav.Category)
	}
	order := "inbox.priority DESC NULLS LAST, inbox.created_at ASC, inbox.kind ASC, inbox.id ASC"
	switch filters.Sort {
	case "newest":
		order = "inbox.created_at DESC, inbox.kind ASC, inbox.id ASC"
	case "oldest":
		order = "inbox.created_at ASC, inbox.kind ASC, inbox.id ASC"
	case "recommended":
		order = "inbox.approval_confidence DESC NULLS LAST, inbox.created_at DESC, inbox.kind ASC, inbox.id ASC"
	}
	// Do not return internal search context or compute it when no search needs it.
	columns := "inbox.id, inbox.kind, inbox.title, inbox.next_step, inbox.category, inbox.customer_name, inbox.owner_name, inbox.owner_member_id, inbox.owner_available, inbox.priority, inbox.priority_band, inbox.lifecycle, inbox.attention, inbox.pending_action_count, inbox.evidence_review, inbox.created_at"
	if err := base.Select(columns).Order(order).Offset((nav.Page - 1) * nav.PageSize).Limit(nav.PageSize).Scan(&result.Data).Error; err != nil {
		return nil, fmt.Errorf("list signal inbox: %w", err)
	}
	return result, nil
}

func inboxSituations(db *gorm.DB, ws, member string, nav model.CRMSituationListFilters) *gorm.DB {
	state := nav.State
	if state == "needs_approval" {
		state = "all"
	}
	query := stateSituationQuery(scopeSituationQuery(situationReadQuery(db, ws), ws, member, nav.Scope), member, nav.Scope, state)
	query = query.Where(`NOT EXISTS (SELECT 1 FROM crm_suggestions internal_follow_up WHERE internal_follow_up.workspace_id = s.workspace_id AND s.creation_key = 'source:suggestion:' || CAST(internal_follow_up.id AS TEXT) AND NOT (` + CRMVisibleMeetingFollowUpsSQL(db, "internal_follow_up") + `))`)
	priority := "CASE WHEN s.origin_kind = 'suggestion' AND s.priority = 0 THEN NULL ELSE s.priority END"
	actions := `SELECT a.* FROM crm_suggestions a JOIN crm_situation_references ar ON ar.workspace_id = a.workspace_id AND ar.source_id = a.id AND ar.kind = 'suggestion' WHERE ar.workspace_id = s.workspace_id AND ar.situation_id = s.id`
	evidence := inboxSituationEvidenceSQL(db)
	return query.Select(`CAST(s.id AS TEXT) AS id, CAST(s.workspace_id AS TEXT) AS workspace_id, 'situation' AS kind, s.title,
		CASE WHEN s.lifecycle = 'closed' THEN COALESCE(s.outcome_summary,'Outcome recorded') ELSE s.next_step END AS next_step,
		` + situationCategorySQL() + ` AS category,
		COALESCE(NULLIF(c.name,''), NULLIF(TRIM(COALESCE(contact.first_name,'') || ' ' || COALESCE(contact.last_name,'')),''), d.name, '') AS customer_name,
		COALESCE(owner.display_name,'') AS owner_name, CAST(s.owner_member_id AS TEXT) AS owner_member_id,
		CASE WHEN owner.status = 'active' THEN TRUE ELSE FALSE END AS owner_available,
		` + priority + ` AS priority, ` + inboxPrioritySQL(priority) + ` AS priority_band,
		s.lifecycle, (` + situationAttentionSQL + `) AS attention,
		COALESCE(action_counts.pending,0) AS pending_action_count,
		` + inboxReviewSQL(evidence) + ` AS evidence_review, s.created_at,
		(SELECT MAX(a.confidence) FROM (` + actions + `) a WHERE a.status = 'pending') AS approval_confidence,
		COALESCE(s.title,'') || ' ' || COALESCE(s.objective,'') || ' ' || COALESCE(s.next_step,'') || ' ' || COALESCE(c.name,'') || ' ' || COALESCE(d.name,'') || ' ' || COALESCE(contact.first_name,'') || ' ' || COALESCE(contact.last_name,'') || ' ' ||
		COALESCE((SELECT ` + inboxStringAggregate(db, "COALESCE(a.title,'') || ' ' || COALESCE(a.description,'') || ' ' || COALESCE(a.suggestion_type,'') || ' ' || COALESCE(a.object_type,'') || ' ' || COALESCE(CAST(a.context AS TEXT),'')") + ` FROM (` + actions + `) a WHERE a.status = 'pending'),'') AS search_text`)
}

func inboxStandalone(db *gorm.DB, ws, member string, nav model.CRMSituationListFilters) *gorm.DB {
	query := db.Table("crm_suggestions a").Where("a.workspace_id = ?", ws).
		Where(situationOpenActionSQL).
		Where(CRMVisibleMeetingFollowUpsSQL(db, "a")).
		Where(`NOT EXISTS (SELECT 1 FROM crm_situation_references ref JOIN crm_situations s ON s.workspace_id = ref.workspace_id AND s.id = ref.situation_id WHERE ref.workspace_id = a.workspace_id AND ref.kind = 'suggestion' AND ref.source_id = a.id)`).
		Joins("LEFT JOIN workspace_members am ON am.workspace_id = a.workspace_id AND am.user_id = a.user_id AND am.status = 'active'").
		Joins("LEFT JOIN crm_companies c ON c.workspace_id = a.workspace_id AND CAST(c.id AS TEXT) = COALESCE(CASE WHEN a.object_type = 'company' THEN CAST(a.object_id AS TEXT) END, a.context->>'company_id')").
		Joins("LEFT JOIN crm_contacts contact ON contact.workspace_id = a.workspace_id AND CAST(contact.id AS TEXT) = COALESCE(CASE WHEN a.object_type = 'contact' THEN CAST(a.object_id AS TEXT) END, a.context->>'contact_id')").
		Joins("LEFT JOIN crm_deals d ON d.workspace_id = a.workspace_id AND CAST(d.id AS TEXT) = COALESCE(CASE WHEN a.object_type = 'deal' THEN CAST(a.object_id AS TEXT) END, a.context->>'deal_id')").
		Joins("LEFT JOIN crm_pipelines p ON p.workspace_id = d.workspace_id AND p.id = d.pipeline_id")
	switch nav.Scope {
	case "mine":
		query = query.Where("am.id = ?", member)
	case "unassigned":
		query = query.Where("am.id IS NULL")
	case "my_teams":
		query = query.Where(`EXISTS (SELECT 1 FROM team_workspace_memberships mine JOIN team_workspace_memberships peer ON peer.team_id = mine.team_id WHERE mine.workspace_member_id = ? AND peer.workspace_member_id = am.id)`, member)
	}
	staleAt := "CURRENT_TIMESTAMP - INTERVAL '5 minutes'"
	if db.Dialector.Name() == "sqlite" {
		staleAt = "datetime('now', '-5 minutes')"
	}
	attention := `CASE WHEN a.status = 'pending' THEN 'needs_approval' WHEN a.execution_status = 'failed' OR a.execution_status IS NULL OR a.execution_status IN ('','pending') OR (a.execution_status = 'in_progress' AND (a.updated_at IS NULL OR a.updated_at < ` + staleAt + `)) THEN 'automation_failed' WHEN a.execution_status = 'in_progress' THEN 'waiting_work' ELSE 'needs_context' END`
	switch nav.State {
	case "paused", "closed":
		query = query.Where("FALSE")
	case "waiting":
		query = query.Where("(" + attention + ") = 'waiting_work'")
	case "needs_attention":
		query = query.Where("(" + attention + ") <> 'waiting_work'")
	}
	evidence := "SELECT e.* FROM crm_signals e WHERE e.workspace_id = a.workspace_id AND " + inboxSignalMembership(db, "e.id", "a.signal_ids")
	// Match the source bridge's precedence without projecting on read: explicit
	// motion, the target deal's motion, unambiguous evidence, then action type.
	validMotions := "('prospecting','conversion','onboarding','adoption','expansion','renewal','retention')"
	motion := `COALESCE(CASE WHEN a.context->>'commercial_motion' IN ` + validMotions + ` THEN a.context->>'commercial_motion' END,
		CASE WHEN a.object_type = 'deal' AND d.id IS NOT NULL THEN CASE COALESCE(NULLIF(d.commercial_motion,''), p.default_commercial_motion, 'new_business') WHEN 'expansion' THEN 'expansion' WHEN 'renewal' THEN 'renewal' WHEN 'existing_business' THEN NULL ELSE 'conversion' END END,
		(SELECT CASE WHEN COUNT(DISTINCT COALESCE(e.commercial_motion,'')) = 1 AND MIN(e.commercial_motion) IN ` + validMotions + ` THEN MIN(e.commercial_motion) END FROM (` + evidence + `) e WHERE e.superseded_at IS NULL AND e.dismissed_at IS NULL),
		CASE WHEN a.object_type = 'deal' AND d.id IS NOT NULL AND COALESCE(NULLIF(d.commercial_motion,''), p.default_commercial_motion, 'new_business') = 'existing_business' THEN 'needs_context' WHEN a.suggestion_type IN ('deal_create','deal_advance') THEN 'conversion' WHEN a.suggestion_type = 'risk_alert' THEN 'retention' ELSE 'needs_context' END)`
	category := strings.ReplaceAll(situationCategorySQL(), "s.commercial_motion", "("+motion+")")
	return query.Select(`CAST(a.id AS TEXT) AS id, CAST(a.workspace_id AS TEXT) AS workspace_id, 'recommendation' AS kind, COALESCE(NULLIF(TRIM(a.title),''),'Review recommendation') AS title,
		COALESCE(a.description,'') AS next_step, ` + category + ` AS category,
		COALESCE(NULLIF(c.name,''), NULLIF(TRIM(COALESCE(contact.first_name,'') || ' ' || COALESCE(contact.last_name,'')),''), d.name, '') AS customer_name,
		COALESCE(am.display_name,'') AS owner_name, CAST(am.id AS TEXT) AS owner_member_id,
		CASE WHEN am.id IS NOT NULL THEN TRUE ELSE FALSE END AS owner_available,
		CAST(NULL AS DOUBLE PRECISION) AS priority, 'unscored' AS priority_band, 'open' AS lifecycle,
		` + attention + ` AS attention, CASE WHEN a.status = 'pending' THEN 1 ELSE 0 END AS pending_action_count,
		` + inboxReviewSQL(evidence) + ` AS evidence_review, a.created_at,
		CASE WHEN a.status = 'pending' THEN a.confidence END AS approval_confidence,
		COALESCE(a.title,'') || ' ' || COALESCE(a.description,'') || ' ' || COALESCE(a.suggestion_type,'') || ' ' || COALESCE(a.object_type,'') || ' ' || COALESCE(CAST(a.context AS TEXT),'') || ' ' || COALESCE(c.name,'') || ' ' || COALESCE(d.name,'') || ' ' || COALESCE(contact.first_name,'') || ' ' || COALESCE(contact.last_name,'') AS search_text`)
}

func inboxPrioritySQL(score string) string {
	return fmt.Sprintf("CASE WHEN (%s) IS NULL THEN 'unscored' WHEN (%s) >= %d THEN 'high' WHEN (%s) >= %d THEN 'medium' ELSE 'low' END", score, score, model.CRMSignalHighPriority, score, model.CRMSignalMediumPriority)
}

func inboxReviewSQL(evidence string) string {
	active := " FROM (" + evidence + ") e WHERE e.dismissed_at IS NULL AND e.superseded_at IS NULL"
	return "CASE WHEN EXISTS (SELECT 1" + active + " AND e.reviewed_at IS NULL) THEN 'needs_review' WHEN EXISTS (SELECT 1" + active + ") THEN 'reviewed' ELSE 'none' END"
}

func inboxSituationEvidenceSQL(db *gorm.DB) string {
	return `SELECT e.* FROM crm_signals e WHERE e.workspace_id = s.workspace_id AND (EXISTS (
		SELECT 1 FROM crm_situation_references er WHERE er.workspace_id = s.workspace_id AND er.situation_id = s.id AND er.kind = 'signal' AND er.source_id = e.id)
		OR EXISTS (SELECT 1 FROM crm_situation_references ar JOIN crm_suggestions a ON a.workspace_id = ar.workspace_id AND a.id = ar.source_id AND ar.kind = 'suggestion' WHERE ar.workspace_id = s.workspace_id AND ar.situation_id = s.id AND ` + inboxSignalMembership(db, "e.id", "a.signal_ids") + `))`
}

func inboxSignalMembership(db *gorm.DB, id, ids string) string {
	if db.Dialector.Name() == "sqlite" {
		// The fixture uses the canonical StringArray PostgreSQL literal encoding.
		return "instr(',' || replace(trim(COALESCE(" + ids + ",''), '{}'), '\"', '') || ',', ',' || CAST(" + id + " AS TEXT) || ',') > 0"
	}
	return "CAST(" + id + " AS TEXT) = ANY(" + ids + ")"
}

func inboxStringAggregate(db *gorm.DB, expression string) string {
	if db.Dialector.Name() == "sqlite" {
		return "GROUP_CONCAT(" + expression + ", ' ')"
	}
	return "STRING_AGG(" + expression + ", ' ')"
}
