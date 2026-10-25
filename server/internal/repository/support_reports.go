package repository

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// SupportReportScope selects the conversations a report covers. Facts include
// every conversation created or resolved in [From, To); callers split that
// window into the current and previous periods.
type SupportReportScope struct {
	WorkspaceID string
	From        time.Time
	To          time.Time
	// MailboxIDs limits the report to these inboxes; "shared" selects
	// conversations with no inbox. Empty means every inbox the actor can open.
	MailboxIDs        []string
	Channels          []string
	WorkspaceMemberID string
	Role              string
}

// SupportReportConversationFact is one conversation's timing and outcome.
type SupportReportConversationFact struct {
	ID                       string
	DisplayID                int
	Subject                  string
	Channel                  string
	MailboxID                *string
	MailboxName              *string
	Status                   string
	FlowState                *string
	Priority                 string
	AssignedUserID           *string
	CustomerName             *string
	CustomerEmail            *string
	CreatedAt                time.Time
	ResolvedAt               *time.Time
	AIState                  *string
	AIResolutionType         *string
	AIEscalatedAt            *time.Time
	CustomerRequestedHumanAt *time.Time
	HumanTakeover            *bool
	// FirstCustomerAt is the first public customer message.
	FirstCustomerAt *time.Time
	// FirstResponseAt is the first public reply by anyone but the customer.
	FirstResponseAt *time.Time
	// FirstHumanReplyAt and FirstHumanReplyUserID describe the first public
	// teammate reply.
	FirstHumanReplyAt     *time.Time
	FirstHumanReplyUserID *string
	AIReplyCount          int
}

// SupportReportReplyCount is the number of public replies one teammate sent.
type SupportReportReplyCount struct {
	UserID  string
	Replies int
}

// reportTime scans timestamps from aggregate expressions, which SQLite
// returns as text and PostgreSQL as time values.
type reportTime struct{ value *time.Time }

func (t *reportTime) Scan(src any) error {
	t.value = nil
	switch v := src.(type) {
	case nil:
		return nil
	case time.Time:
		utc := v.UTC()
		t.value = &utc
		return nil
	case string:
		return t.parse(v)
	case []byte:
		return t.parse(string(v))
	default:
		return fmt.Errorf("scan report time from %T", src)
	}
}

// Value lets GORM treat reportTime as a column type rather than a relation.
func (t reportTime) Value() (driver.Value, error) {
	if t.value == nil {
		return nil, nil
	}
	return *t.value, nil
}

func (t *reportTime) parse(raw string) error {
	raw = strings.TrimSpace(raw)
	parsed, err := scanNullableTime(sql.NullString{String: raw, Valid: raw != ""})
	if err != nil {
		return err
	}
	t.value = parsed
	return nil
}

func (r *SupportConversationRepository) reportCandidates(ctx context.Context, scope SupportReportScope) *gorm.DB {
	query := r.db.WithContext(ctx).Table("support_conversations AS c").Select("c.id").
		Where("c.workspace_id = ? AND c.merged_into_id IS NULL AND c.status <> ?", scope.WorkspaceID, model.SupportConversationStatusSpam).
		Where("((c.created_at >= ? AND c.created_at < ?) OR (c.resolved_at >= ? AND c.resolved_at < ?))", scope.From, scope.To, scope.From, scope.To)
	query = r.applyMailboxAccess(query, "c", scope.WorkspaceMemberID, scope.Role)
	query = r.applyMailboxScopes(query, "c", nil, scope.MailboxIDs)
	if channels := compactStrings(scope.Channels); len(channels) > 0 {
		query = query.Where("c.channel IN ?", channels)
	}
	return query
}

// ListReportFacts returns one fact row per conversation created or resolved
// in the scope's window.
func (r *SupportConversationRepository) ListReportFacts(ctx context.Context, scope SupportReportScope) ([]SupportReportConversationFact, error) {
	publicReplies := "deleted_at IS NULL AND is_internal = ? AND message_type = 'reply' AND workspace_id = ? AND conversation_id IN (?)"
	firstCustomer := r.db.Table("support_messages").
		Select("conversation_id, MIN(created_at) AS first_customer_at").
		Where(publicReplies+" AND sender_type = 'customer'", false, scope.WorkspaceID, r.reportCandidates(ctx, scope)).
		Group("conversation_id")
	firstResponse := r.db.Table("support_messages").
		Select("conversation_id, MIN(created_at) AS first_response_at, SUM(CASE WHEN sender_type IN ('ai', 'agent') THEN 1 ELSE 0 END) AS ai_reply_count").
		Where(publicReplies+" AND sender_type <> 'customer'", false, scope.WorkspaceID, r.reportCandidates(ctx, scope)).
		Group("conversation_id")
	rankedHuman := r.db.Table("support_messages").
		Select("conversation_id, created_at, sender_user_id, ROW_NUMBER() OVER (PARTITION BY conversation_id ORDER BY created_at, id) AS reply_rank").
		Where(publicReplies+" AND sender_type = 'user'", false, scope.WorkspaceID, r.reportCandidates(ctx, scope))
	firstHuman := r.db.Table("(?) AS ranked", rankedHuman).
		Select("conversation_id, created_at AS first_human_reply_at, sender_user_id AS first_human_reply_user_id").
		Where("reply_rank = 1")

	type row struct {
		ID                       string
		DisplayID                int
		Subject                  string
		Channel                  string
		MailboxID                *string
		MailboxName              *string
		Status                   string
		FlowState                *string
		Priority                 string
		AssignedUserID           *string
		CustomerName             *string
		CustomerEmail            *string
		CreatedAt                reportTime
		ResolvedAt               reportTime
		AIState                  *string
		AIResolutionType         *string
		AIEscalatedAt            reportTime
		CustomerRequestedHumanAt reportTime
		HumanTakeover            *bool
		FirstCustomerAt          reportTime
		FirstResponseAt          reportTime
		FirstHumanReplyAt        reportTime
		FirstHumanReplyUserID    *string
		AIReplyCount             int
	}
	var rows []row
	err := r.db.WithContext(ctx).Table("support_conversations AS conv").
		Select(`conv.id, conv.display_id, conv.subject, conv.channel, conv.mailbox_id, mailbox.name AS mailbox_name,
			conv.status, conv.flow_state, conv.priority, conv.assigned_user_id, conv.customer_name, conv.customer_email,
			conv.created_at, conv.resolved_at, conv.ai_state, conv.ai_resolution_type, conv.ai_escalated_at,
			conv.customer_requested_human_at, conv.human_takeover,
			first_customer.first_customer_at, first_response.first_response_at,
			COALESCE(first_response.ai_reply_count, 0) AS ai_reply_count,
			first_human.first_human_reply_at, first_human.first_human_reply_user_id`).
		Joins("LEFT JOIN support_mailboxes AS mailbox ON mailbox.id = conv.mailbox_id").
		Joins("LEFT JOIN (?) AS first_customer ON first_customer.conversation_id = conv.id", firstCustomer).
		Joins("LEFT JOIN (?) AS first_response ON first_response.conversation_id = conv.id", firstResponse).
		Joins("LEFT JOIN (?) AS first_human ON first_human.conversation_id = conv.id", firstHuman).
		Where("conv.id IN (?)", r.reportCandidates(ctx, scope)).
		Order("conv.created_at, conv.id").
		Scan(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("list support report facts: %w", err)
	}
	facts := make([]SupportReportConversationFact, 0, len(rows))
	for _, row := range rows {
		created := time.Time{}
		if row.CreatedAt.value != nil {
			created = *row.CreatedAt.value
		}
		facts = append(facts, SupportReportConversationFact{
			ID: row.ID, DisplayID: row.DisplayID, Subject: row.Subject, Channel: row.Channel,
			MailboxID: row.MailboxID, MailboxName: row.MailboxName, Status: row.Status, FlowState: row.FlowState,
			Priority: row.Priority, AssignedUserID: row.AssignedUserID, CustomerName: row.CustomerName,
			CustomerEmail: row.CustomerEmail, CreatedAt: created, ResolvedAt: row.ResolvedAt.value,
			AIState: row.AIState, AIResolutionType: row.AIResolutionType, AIEscalatedAt: row.AIEscalatedAt.value,
			CustomerRequestedHumanAt: row.CustomerRequestedHumanAt.value, HumanTakeover: row.HumanTakeover,
			FirstCustomerAt: row.FirstCustomerAt.value, FirstResponseAt: row.FirstResponseAt.value,
			FirstHumanReplyAt: row.FirstHumanReplyAt.value, FirstHumanReplyUserID: row.FirstHumanReplyUserID,
			AIReplyCount: row.AIReplyCount,
		})
	}
	return facts, nil
}

// ListReportReplyCounts counts public teammate replies sent in [from, to) on
// conversations the actor can open.
func (r *SupportConversationRepository) ListReportReplyCounts(ctx context.Context, scope SupportReportScope, from, to time.Time) ([]SupportReportReplyCount, error) {
	conversations := r.db.WithContext(ctx).Table("support_conversations AS c").Select("c.id").
		Where("c.workspace_id = ? AND c.status <> ?", scope.WorkspaceID, model.SupportConversationStatusSpam)
	conversations = r.applyMailboxAccess(conversations, "c", scope.WorkspaceMemberID, scope.Role)
	conversations = r.applyMailboxScopes(conversations, "c", nil, scope.MailboxIDs)
	if channels := compactStrings(scope.Channels); len(channels) > 0 {
		conversations = conversations.Where("c.channel IN ?", channels)
	}
	var rows []SupportReportReplyCount
	err := r.db.WithContext(ctx).Table("support_messages").
		Select("sender_user_id AS user_id, COUNT(*) AS replies").
		Where("workspace_id = ? AND deleted_at IS NULL AND is_internal = ? AND message_type = 'reply' AND sender_type = 'user' AND sender_user_id IS NOT NULL",
			scope.WorkspaceID, false).
		Where("created_at >= ? AND created_at < ? AND conversation_id IN (?)", from, to, conversations).
		Group("sender_user_id").Scan(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("count support report replies: %w", err)
	}
	return rows, nil
}

// ReportBacklog counts conversations that are open right now.
type ReportBacklog struct {
	Open       int `json:"open"`
	Waiting    int `json:"waiting_on_customer"`
	Unassigned int `json:"unassigned"`
}

// CountReportBacklog counts the actor's open, waiting and unassigned
// conversations, independent of the report period.
func (r *SupportConversationRepository) CountReportBacklog(ctx context.Context, scope SupportReportScope) (ReportBacklog, error) {
	query := r.db.WithContext(ctx).Table("support_conversations AS c").
		Where("c.workspace_id = ? AND c.merged_into_id IS NULL AND c.status IN ?", scope.WorkspaceID,
			[]string{model.SupportConversationStatusOpen, model.SupportConversationStatusWaitingOnCustomer})
	query = r.applyMailboxAccess(query, "c", scope.WorkspaceMemberID, scope.Role)
	query = r.applyMailboxScopes(query, "c", nil, scope.MailboxIDs)
	if channels := compactStrings(scope.Channels); len(channels) > 0 {
		query = query.Where("c.channel IN ?", channels)
	}
	var backlog ReportBacklog
	err := query.Select(`
		COALESCE(SUM(CASE WHEN c.status = 'open' THEN 1 ELSE 0 END), 0) AS open,
		COALESCE(SUM(CASE WHEN c.status = 'waiting_on_customer' THEN 1 ELSE 0 END), 0) AS waiting,
		COALESCE(SUM(CASE WHEN c.assigned_user_id IS NULL AND c.assigned_agent_id IS NULL THEN 1 ELSE 0 END), 0) AS unassigned`).
		Scan(&backlog).Error
	if err != nil {
		return backlog, fmt.Errorf("count support report backlog: %w", err)
	}
	return backlog, nil
}

// SupportReportCSATRow is one rating with the conversation it belongs to.
type SupportReportCSATRow struct {
	ConversationID string
	DisplayID      int
	Subject        string
	Rating         int
	Comment        *string
	Channel        string
	ResolvedBy     string
	AssignedUserID *string
	SubmittedAt    time.Time
}

// ListReportCSAT returns ratings submitted in [from, to) on conversations the
// actor can open, newest first.
func (r *SupportConversationRepository) ListReportCSAT(ctx context.Context, scope SupportReportScope, from, to time.Time) ([]SupportReportCSATRow, error) {
	query := r.db.WithContext(ctx).Table("support_csat_responses AS csat").
		Joins("JOIN support_conversations AS c ON c.id = csat.conversation_id AND c.workspace_id = csat.workspace_id").
		Where("csat.workspace_id = ? AND csat.submitted_at >= ? AND csat.submitted_at < ?", scope.WorkspaceID, from, to)
	query = r.applyMailboxAccess(query, "c", scope.WorkspaceMemberID, scope.Role)
	query = r.applyMailboxScopes(query, "c", nil, scope.MailboxIDs)
	if channels := compactStrings(scope.Channels); len(channels) > 0 {
		query = query.Where("c.channel IN ?", channels)
	}
	type row struct {
		ConversationID string
		DisplayID      int
		Subject        string
		Rating         int
		Comment        *string
		Channel        string
		ResolvedBy     string
		AssignedUserID *string
		SubmittedAt    reportTime
	}
	var rows []row
	err := query.Select(`csat.conversation_id, c.display_id, c.subject, csat.rating, csat.comment, csat.channel,
		csat.resolved_by, csat.assigned_user_id, csat.submitted_at`).
		Order("csat.submitted_at DESC, csat.id").Scan(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("list support report csat: %w", err)
	}
	result := make([]SupportReportCSATRow, 0, len(rows))
	for _, row := range rows {
		submitted := time.Time{}
		if row.SubmittedAt.value != nil {
			submitted = *row.SubmittedAt.value
		}
		result = append(result, SupportReportCSATRow{
			ConversationID: row.ConversationID, DisplayID: row.DisplayID, Subject: row.Subject,
			Rating: row.Rating, Comment: row.Comment, Channel: row.Channel, ResolvedBy: row.ResolvedBy,
			AssignedUserID: row.AssignedUserID, SubmittedAt: submitted,
		})
	}
	return result, nil
}

// SupportReportUser is a teammate shown in a report.
type SupportReportUser struct {
	ID        string
	FullName  string
	AvatarURL *string
}

// ListReportUsers returns the named users.
func (r *SupportConversationRepository) ListReportUsers(ctx context.Context, ids []string) ([]SupportReportUser, error) {
	ids = compactStrings(ids)
	if len(ids) == 0 {
		return nil, nil
	}
	var users []SupportReportUser
	if err := r.db.WithContext(ctx).Table("users").Select("id, full_name, avatar_url").
		Where("id IN ?", ids).Scan(&users).Error; err != nil {
		return nil, fmt.Errorf("list support report users: %w", err)
	}
	return users, nil
}
