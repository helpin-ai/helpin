package repository

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// CRMCompanyTimelineRepository reads a normalized company timeline from existing source tables.
type CRMCompanyTimelineRepository struct {
	db *gorm.DB
}

// NewCRMCompanyTimelineRepository creates a company timeline repository.
func NewCRMCompanyTimelineRepository(db *gorm.DB) *CRMCompanyTimelineRepository {
	return &CRMCompanyTimelineRepository{db: db}
}

type crmCompanyTimelineRow struct {
	ID              string
	Kind            string
	EventType       string
	SourceType      string
	SourceID        string
	Title           string
	Description     *string
	OccurredAt      sql.NullTime
	ActorID         *string
	ActorName       *string
	ContactID       *string
	ContactName     *string
	EntityType      *string
	EntityID        *string
	EntityName      *string
	EntityDisplayID *string
	CanEdit         bool
	CanDelete       bool
}

const crmCompanyTimelineCursorClause = `
    CAST(@cursor_at AS timestamptz) IS NULL
    OR occurred_at < CAST(@cursor_at AS timestamptz)
    OR (occurred_at = CAST(@cursor_at AS timestamptz) AND id < @cursor_id)`

// List returns one page of timeline items derived from current company associations.
func (r *CRMCompanyTimelineRepository) List(
	ctx context.Context,
	workspaceID, companyID string,
	query model.CRMCompanyTimelineQuery,
) ([]model.CRMCompanyTimelineItem, error) {
	if r.db.Dialector.Name() != "postgres" {
		return r.listPortable(ctx, workspaceID, companyID, query)
	}

	filter := strings.TrimSpace(query.Filter)
	if filter == "" {
		filter = model.CRMCompanyTimelineFilterAll
	}
	limit := query.Limit
	if limit <= 0 {
		limit = 25
	}

	raw := `
WITH company_contacts AS (
    SELECT DISTINCT CASE
        WHEN ca.from_object_type = 'contact' THEN ca.from_object_id
        ELSE ca.to_object_id
    END AS id
    FROM crm_associations ca
    WHERE ca.workspace_id = @workspace_id
      AND (
        (ca.from_object_type = 'company' AND ca.from_object_id = @company_id AND ca.to_object_type = 'contact')
        OR
        (ca.to_object_type = 'company' AND ca.to_object_id = @company_id AND ca.from_object_type = 'contact')
      )
), related_tasks AS (
    SELECT DISTINCT CASE WHEN ca.from_object_type = 'task' THEN ca.from_object_id ELSE ca.to_object_id END AS id
    FROM crm_associations ca
    WHERE ca.workspace_id = @workspace_id
      AND (
        (ca.from_object_type = 'task' AND ca.to_object_type = 'company' AND ca.to_object_id = @company_id)
        OR (ca.to_object_type = 'task' AND ca.from_object_type = 'company' AND ca.from_object_id = @company_id)
        OR (ca.from_object_type = 'task' AND ca.to_object_type = 'contact' AND ca.to_object_id IN (SELECT id FROM company_contacts))
        OR (ca.to_object_type = 'task' AND ca.from_object_type = 'contact' AND ca.from_object_id IN (SELECT id FROM company_contacts))
      )
), related_deals AS (
    SELECT DISTINCT CASE WHEN ca.from_object_type = 'deal' THEN ca.from_object_id ELSE ca.to_object_id END AS id
    FROM crm_associations ca
    WHERE ca.workspace_id = @workspace_id
      AND (
        (ca.from_object_type = 'deal' AND ca.to_object_type = 'company' AND ca.to_object_id = @company_id)
        OR (ca.to_object_type = 'deal' AND ca.from_object_type = 'company' AND ca.from_object_id = @company_id)
        OR (ca.from_object_type = 'deal' AND ca.to_object_type = 'contact' AND ca.to_object_id IN (SELECT id FROM company_contacts))
        OR (ca.to_object_type = 'deal' AND ca.from_object_type = 'contact' AND ca.from_object_id IN (SELECT id FROM company_contacts))
      )
), related_support AS (
    SELECT sc.id
    FROM support_conversations sc
    WHERE sc.workspace_id = @workspace_id
      AND (sc.crm_company_id = @company_id OR sc.crm_contact_id IN (SELECT id FROM company_contacts))
), candidates AS (
    SELECT
      'activity:' || a.id AS id,
      CASE WHEN COALESCE(a.metadata->>'event_type', '') = 'crm_enrichment_applied' THEN 'enrichment' ELSE a.activity_type END AS kind,
      CASE WHEN COALESCE(a.metadata->>'event_type', '') <> '' THEN a.metadata->>'event_type' ELSE 'activity.' || a.activity_type END AS event_type,
      'crm_activity' AS source_type,
      a.id AS source_id,
      COALESCE(NULLIF(a.subject, ''), INITCAP(a.activity_type)) AS title,
      NULLIF(a.body, '') AS description,
      a.occurred_at,
      COALESCE(wm.id::text, eu.id::text) AS actor_id,
      COALESCE(NULLIF(wm.display_name, ''), NULLIF(eu.full_name, '')) AS actor_name,
      c.id::text AS contact_id,
      NULLIF(TRIM(CONCAT(c.first_name, ' ', c.last_name)), '') AS contact_name,
      CASE WHEN COALESCE(a.metadata->>'meeting_id', '') <> '' THEN 'meeting' END AS entity_type,
      NULLIF(a.metadata->>'meeting_id', '') AS entity_id,
      CASE WHEN COALESCE(a.metadata->>'meeting_id', '') <> '' THEN COALESCE(NULLIF(a.subject, ''), 'Meeting') END AS entity_name,
      NULL::text AS entity_display_id,
      (COALESCE((a.metadata->>'immutable')::boolean, false) = false) AS can_edit,
      (COALESCE((a.metadata->>'immutable')::boolean, false) = false) AS can_delete,
      CASE
        WHEN COALESCE(a.metadata->>'event_type', '') = 'crm_enrichment_applied' THEN 'system'
        ELSE a.activity_type
      END AS filter_key
    FROM crm_activities a
    LEFT JOIN crm_contacts c ON c.id = a.contact_id
    LEFT JOIN workspace_members wm ON wm.id = a.owner_member_id
    LEFT JOIN users eu ON eu.id::text = a.metadata->>'actor_user_id'
    WHERE a.workspace_id = @workspace_id
      AND (a.company_id = @company_id OR a.contact_id IN (SELECT id FROM company_contacts))

    UNION ALL

    SELECT DISTINCT ON (m.id)
      'email:' || m.id,
      'email',
      'email.' || COALESCE(NULLIF(m.direction, ''), 'message'),
      'crm_email_message',
      m.id,
      COALESCE(NULLIF(m.subject, ''), '(no subject)'),
      NULLIF(LEFT(COALESCE(m.body_text, ''), 500), ''),
      m.sent_at,
      NULL::text,
      NULL::text,
      c.id::text,
      NULLIF(TRIM(CONCAT(c.first_name, ' ', c.last_name)), ''),
      'email_thread',
      COALESCE(m.thread_id, m.id)::text,
      COALESCE(NULLIF(m.subject, ''), '(no subject)'),
      NULL::text,
      false,
      false,
      'email'
    FROM crm_email_messages m
    JOIN crm_email_message_contacts mc ON mc.message_id = m.id AND mc.workspace_id = m.workspace_id
    JOIN company_contacts cc ON cc.id = mc.contact_id
    LEFT JOIN crm_contacts c ON c.id = mc.contact_id
    WHERE m.workspace_id = @workspace_id

    UNION ALL

    SELECT DISTINCT ON (ce.id)
      'calendar:' || ce.id,
      'meeting',
      'meeting.calendar',
      'crm_calendar_event',
      ce.id,
      ce.title,
      NULLIF(ce.description, ''),
      ce.start_time,
      NULL::text,
      NULL::text,
      c.id::text,
      NULLIF(TRIM(CONCAT(c.first_name, ' ', c.last_name)), ''),
      'calendar_event',
      ce.id::text,
      ce.title,
      NULL::text,
      false,
      false,
      'meeting'
    FROM crm_calendar_events ce
    JOIN company_contacts cc ON jsonb_exists(ce.contact_ids, cc.id::text)
    LEFT JOIN crm_contacts c ON c.id = cc.id
    WHERE ce.workspace_id = @workspace_id
      AND ce.status <> 'cancelled'
      AND ce.start_time <= NOW()
      AND NOT EXISTS (
        SELECT 1 FROM crm_meetings cm
        WHERE cm.workspace_id = ce.workspace_id AND cm.calendar_event_id = ce.id AND cm.activity_id IS NOT NULL
      )

    UNION ALL

    SELECT
      'task:' || pal.id,
      'task',
      COALESCE(NULLIF(pal.event_type, ''), NULLIF(pal.metadata->>'event_type', ''), CASE
        WHEN pal.action LIKE 'created this task%' THEN 'task.created'
        WHEN pal.action LIKE 'moved this task to %' THEN 'task.state_changed'
        WHEN pal.action = 'marked this task as blocked' THEN 'task.blocked'
        WHEN pal.action = 'unblocked this task' THEN 'task.unblocked'
        ELSE 'task.activity'
      END),
      'pm_activity',
      pal.id,
      pt.name,
      pal.action,
      pal.created_at,
      u.id::text,
      NULLIF(u.full_name, ''),
      NULL::text,
      NULL::text,
      'task',
      pt.id::text,
      pt.name,
      COALESCE(NULLIF(w.workspace_key, ''), 'TASK') || '-' || pt.display_id::text,
      false,
      false,
      'task'
    FROM pm_activity_log pal
    JOIN related_tasks rt ON rt.id = pal.entity_id
    JOIN pm_tasks pt ON pt.id = pal.entity_id
    JOIN workspaces w ON w.id = pal.workspace_id
    LEFT JOIN users u ON u.id = pal.actor_id
    WHERE pal.workspace_id = @workspace_id
      AND pal.entity_type = 'task'
      AND (
        COALESCE(NULLIF(pal.event_type, ''), pal.metadata->>'event_type') IN ('task.created', 'task.state_changed', 'task.completed', 'task.reopened', 'task.blocked', 'task.unblocked')
        OR pal.action LIKE 'created this task%'
        OR pal.action LIKE 'moved this task to %'
        OR pal.action IN ('marked this task as blocked', 'unblocked this task')
      )

    UNION ALL

    SELECT
      'deal:' || pal.id,
      'deal',
      COALESCE(NULLIF(pal.event_type, ''), NULLIF(pal.metadata->>'event_type', ''), 'deal.activity'),
      'pm_activity',
      pal.id,
      d.name,
      pal.action,
      pal.created_at,
      u.id::text,
      NULLIF(u.full_name, ''),
      NULL::text,
      NULL::text,
      'deal',
      d.id::text,
      d.name,
      d.display_id,
      false,
      false,
      'system'
    FROM pm_activity_log pal
    JOIN related_deals rd ON rd.id = pal.entity_id
    JOIN crm_deals d ON d.id = pal.entity_id
    LEFT JOIN users u ON u.id = pal.actor_id
    WHERE pal.workspace_id = @workspace_id
      AND pal.entity_type = 'deal'
      AND COALESCE(NULLIF(pal.event_type, ''), pal.metadata->>'event_type') IN ('deal.created', 'deal.stage_changed', 'deal.won', 'deal.lost')

    UNION ALL

    SELECT
      'deal-created:' || d.id,
      'deal',
      'deal.created',
      'crm_deal',
      d.id,
      d.name,
      'created this deal in ' || COALESCE(NULLIF(s.name, ''), 'its initial stage'),
      d.created_at,
      NULL::text,
      NULL::text,
      NULL::text,
      NULL::text,
      'deal',
      d.id::text,
      d.name,
      d.display_id,
      false,
      false,
      'system'
    FROM related_deals rd
    JOIN crm_deals d ON d.id = rd.id AND d.workspace_id = @workspace_id
    LEFT JOIN crm_pipeline_stages s ON s.id = d.stage_id
    WHERE NOT EXISTS (
      SELECT 1
      FROM pm_activity_log pal
      WHERE pal.workspace_id = @workspace_id
        AND pal.entity_type = 'deal'
        AND pal.entity_id = d.id
        AND COALESCE(NULLIF(pal.event_type, ''), pal.metadata->>'event_type') = 'deal.created'
    )

    UNION ALL

    SELECT
      'support:' || se.id,
      'support',
      'support.' || se.event_type,
      'support_event',
      se.id,
      sc.subject,
      CASE se.event_type
        WHEN 'conversation_created' THEN 'Support conversation opened'
        WHEN 'customer_message_created' THEN 'Customer replied'
        WHEN 'human_reply_sent' THEN 'Teammate replied'
        WHEN 'ai_answer_sent' THEN 'AI agent replied'
        WHEN 'conversation_resolved' THEN 'Support conversation resolved'
        WHEN 'conversation_status_changed' THEN 'Support status changed'
        ELSE REPLACE(se.event_type, '_', ' ')
      END,
      se.occurred_at,
      NULL::text,
      NULLIF(INITCAP(se.actor_type), ''),
      c.id::text,
      NULLIF(TRIM(CONCAT(c.first_name, ' ', c.last_name)), ''),
      'support_conversation',
      sc.id::text,
      sc.subject,
      sc.display_id::text,
      false,
      false,
      'system'
    FROM support_events se
    JOIN related_support rs ON rs.id = se.conversation_id
    JOIN support_conversations sc ON sc.id = se.conversation_id
    LEFT JOIN crm_contacts c ON c.id = sc.crm_contact_id
    WHERE se.workspace_id = @workspace_id
      AND se.event_type IN (
        'conversation_created', 'customer_message_created', 'human_reply_sent',
        'ai_answer_sent', 'conversation_resolved', 'conversation_status_changed'
      )
)
SELECT id, kind, event_type, source_type, source_id, title, description, occurred_at,
       actor_id, actor_name, contact_id, contact_name, entity_type, entity_id,
       entity_name, entity_display_id, can_edit, can_delete
FROM candidates
WHERE (@filter = 'all' OR filter_key = @filter)
  AND (` + crmCompanyTimelineCursorClause + `
  )
ORDER BY occurred_at DESC, id DESC
LIMIT @limit`

	var cursorAt interface{}
	if query.CursorAt != nil {
		cursorAt = *query.CursorAt
	}
	args := map[string]interface{}{
		"workspace_id": workspaceID,
		"company_id":   companyID,
		"filter":       filter,
		"cursor_at":    cursorAt,
		"cursor_id":    query.CursorID,
		"limit":        limit,
	}
	var rows []crmCompanyTimelineRow
	if err := r.db.WithContext(ctx).Raw(raw, args).Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("list company timeline: %w", err)
	}
	return mapCRMCompanyTimelineRows(rows), nil
}

func mapCRMCompanyTimelineRows(rows []crmCompanyTimelineRow) []model.CRMCompanyTimelineItem {
	items := make([]model.CRMCompanyTimelineItem, 0, len(rows))
	for _, row := range rows {
		if !row.OccurredAt.Valid {
			continue
		}
		item := model.CRMCompanyTimelineItem{
			ID: row.ID, Kind: row.Kind, EventType: row.EventType,
			SourceType: row.SourceType, SourceID: row.SourceID,
			Title: row.Title, Description: row.Description, OccurredAt: row.OccurredAt.Time,
			CanEdit: row.CanEdit, CanDelete: row.CanDelete,
		}
		if row.ActorID != nil || row.ActorName != nil {
			item.Actor = &model.CRMCompanyTimelineReference{Type: "actor", ID: timelineStringValue(row.ActorID), Name: timelineStringValue(row.ActorName)}
		}
		if row.ContactID != nil {
			item.Contact = &model.CRMCompanyTimelineReference{Type: "contact", ID: *row.ContactID, Name: timelineStringValue(row.ContactName)}
		}
		if row.EntityType != nil && row.EntityID != nil {
			item.Entity = &model.CRMCompanyTimelineReference{Type: *row.EntityType, ID: *row.EntityID, Name: timelineStringValue(row.EntityName), DisplayID: row.EntityDisplayID}
		}
		items = append(items, item)
	}
	return items
}

func (r *CRMCompanyTimelineRepository) listPortable(
	ctx context.Context,
	workspaceID, companyID string,
	query model.CRMCompanyTimelineQuery,
) ([]model.CRMCompanyTimelineItem, error) {
	limit := query.Limit
	if limit <= 0 {
		limit = 25
	}
	dbQuery := r.db.WithContext(ctx).
		Where("workspace_id = ? AND company_id = ?", workspaceID, companyID)
	if query.Filter != "" && query.Filter != model.CRMCompanyTimelineFilterAll {
		dbQuery = dbQuery.Where("activity_type = ?", query.Filter)
	}
	if query.CursorAt != nil {
		dbQuery = dbQuery.Where("occurred_at < ? OR (occurred_at = ? AND id < ?)", *query.CursorAt, *query.CursorAt, strings.TrimPrefix(query.CursorID, "activity:"))
	}
	var activities []model.CRMActivity
	if err := dbQuery.Order("occurred_at DESC, id DESC").Limit(limit).Find(&activities).Error; err != nil {
		return nil, fmt.Errorf("list portable company timeline: %w", err)
	}
	items := make([]model.CRMCompanyTimelineItem, 0, len(activities))
	for _, activity := range activities {
		immutable := activity.Metadata != nil && activity.Metadata["immutable"] == true
		kind := activity.ActivityType
		eventType := "activity." + activity.ActivityType
		if activity.Metadata != nil && activity.Metadata["event_type"] == "crm_enrichment_applied" {
			kind = "enrichment"
			eventType = "crm_enrichment_applied"
		}
		items = append(items, model.CRMCompanyTimelineItem{
			ID: "activity:" + activity.ID, Kind: kind, EventType: eventType,
			SourceType: "crm_activity", SourceID: activity.ID,
			Title: timelineStringValue(activity.Subject), Description: activity.Body,
			OccurredAt: activity.OccurredAt, CanEdit: !immutable, CanDelete: !immutable,
		})
	}
	return items, nil
}

func timelineStringValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
