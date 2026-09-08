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

// List computes filtered category facets and totals before paging the work records.
func (r *CRMSituationRepository) List(
	ctx context.Context, workspaceID, memberID string, filters model.CRMSituationListFilters,
) (*model.CRMSituationList, error) {
	var response *model.CRMSituationList
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var err error
		response, err = listSituations(tx, workspaceID, memberID, filters)
		return err
	}, &sql.TxOptions{ReadOnly: true, Isolation: sql.LevelRepeatableRead})
	return response, err
}

func listSituations(db *gorm.DB, workspaceID, memberID string, filters model.CRMSituationListFilters) (*model.CRMSituationList, error) {
	return listSituationsFromQuery(db, workspaceID, memberID, filters, situationReadQuery(db, workspaceID))
}

func listSituationsFromQuery(db *gorm.DB, workspaceID, memberID string, filters model.CRMSituationListFilters, base *gorm.DB) (*model.CRMSituationList, error) {
	if filters.PlaybookID != "" {
		base = base.Where("s.playbook_id = ?", filters.PlaybookID)
	}
	base = scopeSituationQuery(base, workspaceID, memberID, filters.Scope)
	base = stateSituationQuery(base, memberID, filters.Scope, filters.State)
	if filters.Search != "" {
		pattern := "%" + strings.NewReplacer("!", "!!", "%", "!%", "_", "!_").Replace(strings.ToLower(filters.Search)) + "%"
		base = base.Where(`LOWER(s.title) LIKE ? ESCAPE '!' OR LOWER(s.objective) LIKE ? ESCAPE '!'
			OR LOWER(c.name) LIKE ? ESCAPE '!' OR LOWER(d.name) LIKE ? ESCAPE '!'
			OR LOWER(TRIM(COALESCE(contact.first_name, '') || ' ' || COALESCE(contact.last_name, ''))) LIKE ? ESCAPE '!'`,
			pattern, pattern, pattern, pattern, pattern)
	}
	base, err := querybuilder.ApplyGORM(base, filters.Query, situationQueryDefinitions())
	if err != nil {
		return nil, err
	}
	response := &model.CRMSituationList{
		Data: []model.CRMSituationItem{}, Page: filters.Page, PageSize: filters.PageSize,
		CategoryCounts: map[string]int64{"all": 0},
	}
	for _, category := range model.CRMSituationCategories() {
		response.CategoryCounts[category.Key] = 0
	}
	var facets []struct {
		CommercialMotion string
		Total            int64
	}
	if err := base.Session(&gorm.Session{}).Select("s.commercial_motion, COUNT(DISTINCT s.id) AS total").
		Group("s.commercial_motion").Scan(&facets).Error; err != nil {
		return nil, fmt.Errorf("count situation categories: %w", err)
	}
	for _, facet := range facets {
		if category := model.CRMSituationCategoryForMotion(facet.CommercialMotion); category != "" {
			response.CategoryCounts[category] += facet.Total
		} else {
			response.UncategorizedCount += facet.Total
		}
		response.CategoryCounts["all"] += facet.Total
	}
	// The selected total and sibling facets share exactly the same filters.
	// The read snapshot also keeps the page consistent with these counts.
	response.Total = response.CategoryCounts[filters.Category]
	query := base.Session(&gorm.Session{})
	if filters.Category != "all" {
		for _, category := range model.CRMSituationCategories() {
			if filters.Category == category.Key {
				query = query.Where("s.commercial_motion IN ?", category.Motions)
			}
		}
	}
	// Materialize matching IDs once, keeping navigation joins separate from
	// the distinct-action count. This avoids expanding the combined query plan.
	if err := db.Raw(`WITH matching_situations AS MATERIALIZED (?)
		SELECT COUNT(DISTINCT a.id) FROM crm_situation_references ref
		JOIN crm_suggestions a ON a.workspace_id = ref.workspace_id AND a.id = ref.source_id
		JOIN matching_situations matched ON matched.id = ref.situation_id
		WHERE ref.workspace_id = ? AND ref.kind = 'suggestion' AND a.status = 'pending'`,
		query.Session(&gorm.Session{}).Select("s.id"), workspaceID).Scan(&response.PendingActionTotal).Error; err != nil {
		return nil, fmt.Errorf("count pending situation actions: %w", err)
	}
	if err := query.Order("s.priority DESC, s.created_at ASC, s.id ASC").
		Offset((filters.Page - 1) * filters.PageSize).Limit(filters.PageSize).Scan(&response.Data).Error; err != nil {
		return nil, fmt.Errorf("list customer situations: %w", err)
	}
	return response, nil
}

func situationReadQuery(db *gorm.DB, workspaceID string) *gorm.DB {
	return db.Table("crm_situations AS s").Where("s.workspace_id = ?", workspaceID).
		Joins(situationPlaybookExecutionJoin(db)).
		Joins(situationCountsJoin(db), workspaceID).
		Joins(`LEFT JOIN automation_scheduled_events checkpoint_event ON checkpoint_event.workspace_id = s.workspace_id
			AND checkpoint_event.event_key = 'crm.checkpoint:' || CAST(s.id AS text) || ':' || CAST(s.revision AS text)
			AND checkpoint_event.kind = 'crm.checkpoint_due' AND checkpoint_event.target_type = 'crm_situation'
			AND checkpoint_event.target_id = s.id AND checkpoint_event.expected_revision = s.revision`).
		Joins("LEFT JOIN crm_companies c ON c.id = s.company_id AND c.workspace_id = s.workspace_id").
		Joins("LEFT JOIN crm_contacts contact ON contact.id = s.contact_id AND contact.workspace_id = s.workspace_id").
		Joins("LEFT JOIN crm_deals d ON d.id = s.deal_id AND d.workspace_id = s.workspace_id").
		Joins("LEFT JOIN workspace_members owner ON owner.id = s.owner_member_id AND owner.workspace_id = s.workspace_id").
		Joins("LEFT JOIN workspace_members next_owner ON next_owner.id = s.next_action_owner_member_id AND next_owner.workspace_id = s.workspace_id").
		Select(`s.*, ` + situationCategorySQL() + ` AS category, ` + situationAttentionSQL + ` AS effective_attention,
			COALESCE(checkpoint_event.status, '') AS checkpoint_status,
			COALESCE(checkpoint_event.result_code, '') AS checkpoint_result,
			COALESCE(checkpoint_event.attempts, 0) AS checkpoint_attempts,
			checkpoint_event.completed_at AS checkpoint_completed_at,
			COALESCE(action_counts.pending,0) AS pending_action_count,
			COALESCE(action_counts.failed,0) AS failed_action_count,
			COALESCE(action_counts.uncertain,0) AS uncertain_action_count,
			COALESCE(action_counts.manual,0) AS manual_action_count,
			COALESCE(action_counts.executing,0) AS executing_action_count,
			COALESCE(c.name, '') AS company_name, COALESCE(d.name, '') AS deal_name,
			TRIM(COALESCE(contact.first_name, '') || ' ' || COALESCE(contact.last_name, '')) AS contact_name,
			COALESCE(owner.display_name, '') AS owner_name,
			COALESCE(next_owner.display_name, '') AS next_action_owner_name,
			CASE WHEN owner.status = 'active' THEN TRUE ELSE FALSE END AS owner_available,
			CASE WHEN next_owner.status = 'active' THEN TRUE ELSE FALSE END AS next_action_owner_available`)
}

func scopeSituationQuery(query *gorm.DB, workspaceID, memberID, scope string) *gorm.DB {
	switch scope {
	case "mine":
		return query.Where("s.owner_member_id = ? OR s.next_action_owner_member_id = ? OR "+fmt.Sprintf(situationActionExistsSQL, "am.id = ?")+" OR playbook_execution.escalation_member_id = ?", memberID, memberID, memberID, memberID)
	case "unassigned":
		return query.Where(`(owner.id IS NULL OR owner.status <> 'active') OR (playbook_execution.escalated_at IS NOT NULL AND playbook_execution.escalation_member_id IS NULL) OR ` + situationMissingCheckpointOwnerSQL + ` OR
			(s.lifecycle = 'open' AND s.attention IN ('needs_context','needs_approval','follow_up_due','automation_failed')
			AND (next_owner.id IS NULL OR next_owner.status <> 'active')) OR ` + fmt.Sprintf(situationActionExistsSQL, "a.user_id IS NOT NULL AND am.id IS NULL"))
	case "my_teams":
		return query.Where(`EXISTS (
			SELECT 1 FROM team_workspace_memberships mine
			JOIN team_workspace_memberships peer ON peer.team_id = mine.team_id
			JOIN workspace_members member ON member.id = peer.workspace_member_id
			WHERE mine.workspace_member_id = ? AND member.workspace_id = ? AND member.status = 'active'
			AND (member.id = s.owner_member_id OR member.id = s.next_action_owner_member_id OR CAST(member.id AS text) = playbook_execution.escalation_member_id OR `+fmt.Sprintf(situationActionExistsSQL, "am.id = member.id")+`)
		)`, memberID, workspaceID)
	default:
		return query
	}
}

func stateSituationQuery(query *gorm.DB, memberID, scope, state string) *gorm.DB {
	switch state {
	case "needs_attention":
		query = query.Where(`s.lifecycle = 'open' AND
			((` + situationAttentionSQL + `) IN ('needs_context','needs_approval','follow_up_due','automation_failed')
			OR owner.id IS NULL OR owner.status <> 'active')`)
		if scope == "mine" {
			query = query.Where("("+situationPersonalAttentionSQL()+") OR playbook_execution.escalation_member_id = ?", memberID, memberID, memberID, memberID, memberID)
		}
		return query
	case "waiting":
		if scope == "mine" {
			// Owned work stays visible while an active colleague holds the next
			// action, without claiming it needs this member's attention.
			return query.Where(`s.lifecycle = 'open' AND
				((`+situationAttentionSQL+`) IN ('waiting_customer','waiting_work') OR
				(s.owner_member_id = ? AND NOT (`+situationPersonalAttentionSQL()+`)))`, memberID, memberID, memberID, memberID, memberID)
		}
		return query.Where("s.lifecycle = 'open' AND ("+situationAttentionSQL+") IN ?",
			[]string{model.CRMSituationWaitingCustomer, model.CRMSituationWaitingWork})
	case "open", "paused", "closed":
		return query.Where("s.lifecycle = ?", state)
	default:
		return query
	}
}

func situationCategorySQL() string {
	var expression strings.Builder
	expression.WriteString("CASE s.commercial_motion")
	for _, category := range model.CRMSituationCategories() {
		for _, motion := range category.Motions {
			// Values come only from the fixed domain taxonomy, never query input.
			fmt.Fprintf(&expression, " WHEN '%s' THEN '%s'", motion, category.Key)
		}
	}
	expression.WriteString(" ELSE '' END")
	return expression.String()
}
