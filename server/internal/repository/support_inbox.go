package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"

	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// SupportMessageRepository handles DB operations for support messages.
type SupportMessageRepository struct {
	db *gorm.DB
}

// NewSupportMessageRepository creates a new SupportMessageRepository.
func NewSupportMessageRepository(db *gorm.DB) *SupportMessageRepository {
	return &SupportMessageRepository{db: db}
}

// ListByConversation returns messages for a conversation.
func (r *SupportMessageRepository) ListByConversation(ctx context.Context, workspaceID, conversationID string, includeInternal bool) ([]model.SupportMessage, error) {
	query := r.db.WithContext(ctx).Where("workspace_id = ? AND conversation_id = ?", workspaceID, conversationID)
	if !includeInternal {
		query = query.Where("is_internal = false")
	}
	var messages []model.SupportMessage
	if err := query.Order("created_at ASC").Find(&messages).Error; err != nil {
		return nil, fmt.Errorf("list messages: %w", err)
	}
	return messages, nil
}

// Create creates a new message.
//
// Invariant: every row with MessageType == "system" MUST carry a recognized
// SystemEventType. Rendering logic on both the widget and admin surfaces
// dispatches on SystemEventType rather than keyword-matching the content, so
// a missing value would silently render via the legacy fallback path.
// Enforcing here means new emitters can't forget the event type.
func (r *SupportMessageRepository) Create(ctx context.Context, message *model.SupportMessage) error {
	if message != nil && message.MessageType == "system" {
		if message.SystemEventType == nil || !model.IsValidSupportSystemEventType(*message.SystemEventType) {
			return fmt.Errorf("create message: system message requires a valid system_event_type")
		}
	}
	if err := r.db.WithContext(ctx).Create(message).Error; err != nil {
		return fmt.Errorf("create message: %w", err)
	}
	// Bump parent conversation's updated_at so it moves to top of inbox list
	if message.ConversationID != "" {
		r.db.WithContext(ctx).
			Model(&model.SupportConversation{}).
			Where("id = ?", message.ConversationID).
			Update("updated_at", time.Now())
	}
	return nil
}

// GetByID returns a support message by ID.
func (r *SupportMessageRepository) GetByID(ctx context.Context, id string) (*model.SupportMessage, error) {
	var message model.SupportMessage
	if err := r.db.WithContext(ctx).Where("id = ?", id).First(&message).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("get message: %w", err)
	}
	return &message, nil
}

// GetByIDs returns support messages by ID ordered by created_at.
func (r *SupportMessageRepository) GetByIDs(ctx context.Context, ids []string) ([]model.SupportMessage, error) {
	if len(ids) == 0 {
		return []model.SupportMessage{}, nil
	}
	var messages []model.SupportMessage
	if err := r.db.WithContext(ctx).
		Where("id IN ?", ids).
		Order("created_at ASC").
		Find(&messages).Error; err != nil {
		return nil, fmt.Errorf("get messages by ids: %w", err)
	}
	return messages, nil
}

// ListEmailFallbackReconciliationCandidates returns recent outbound replies
// that still need offline email fallback processing. The service layer performs
// the final per-workspace delay, duplicate-log, and presence checks before
// sending.
func (r *SupportMessageRepository) ListEmailFallbackReconciliationCandidates(ctx context.Context, after, before time.Time, limit int) ([]model.SupportMessage, error) {
	if limit < 1 || limit > 1000 {
		limit = 25
	}
	var messages []model.SupportMessage
	if err := r.db.WithContext(ctx).
		Model(&model.SupportMessage{}).
		Joins("JOIN support_conversations sc ON sc.id = support_messages.conversation_id AND sc.workspace_id = support_messages.workspace_id").
		Where("support_messages.email_notified_at IS NULL").
		Where("support_messages.is_internal = ?", false).
		Where("COALESCE(NULLIF(support_messages.message_type, ''), 'reply') = ?", "reply").
		Where("support_messages.sender_type <> ?", "customer").
		Where("support_messages.created_at <= ?", before).
		Where("support_messages.created_at >= ?", after).
		Where("(support_messages.cancellable_until IS NULL OR support_messages.cancellable_until <= ?)", before).
		Where("sc.customer_email IS NOT NULL AND TRIM(sc.customer_email) <> ''").
		Where("sc.email_unsubscribed = ?", false).
		Where("LOWER(sc.status) NOT IN ?", []string{"closed", "resolved", "spam"}).
		Where("(sc.contact_last_seen_at IS NULL OR support_messages.created_at > sc.contact_last_seen_at)").
		Order("support_messages.created_at ASC").
		Limit(limit).
		Find(&messages).Error; err != nil {
		return nil, fmt.Errorf("list email fallback reconciliation candidates: %w", err)
	}
	return messages, nil
}

// UpdateEmailNotifiedAt stamps email_notified_at for the provided message IDs.
func (r *SupportMessageRepository) UpdateEmailNotifiedAt(ctx context.Context, ids []string, notifiedAt time.Time) error {
	if len(ids) == 0 {
		return nil
	}
	if err := r.db.WithContext(ctx).
		Model(&model.SupportMessage{}).
		Where("id IN ?", ids).
		Update("email_notified_at", notifiedAt).Error; err != nil {
		return fmt.Errorf("update email_notified_at: %w", err)
	}
	return nil
}

// UpdateEmailReadAt stamps email_read_at for the provided message IDs.
func (r *SupportMessageRepository) UpdateEmailReadAt(ctx context.Context, ids []string, readAt time.Time) error {
	if len(ids) == 0 {
		return nil
	}
	if err := r.db.WithContext(ctx).
		Model(&model.SupportMessage{}).
		Where("id IN ?", ids).
		Update("email_read_at", readAt).Error; err != nil {
		return fmt.Errorf("update email_read_at: %w", err)
	}
	return nil
}

// DB returns the underlying *gorm.DB for transaction support.
func (r *SupportMessageRepository) DB() *gorm.DB {
	return r.db
}

// WithTx returns a new SupportMessageRepository using the given transaction.
func (r *SupportMessageRepository) WithTx(tx *gorm.DB) *SupportMessageRepository {
	return &SupportMessageRepository{db: tx}
}

// ownedReplyClause matches outbound, non-internal, non-system messages
// authored by the given human user. It is the canonical eligibility
// predicate for the message-actions feature: only the original author can
// undo or remove their own reply, and only "real" replies (not csat
// surveys, not system events, not internal notes) are mutable.
const ownedReplyClause = `
	sender_type    = 'user'
AND sender_user_id = ?
AND message_type   = 'reply'
AND is_internal    = false`

// GetMessageForActor returns the message if it exists, is not soft-deleted,
// and was authored by the given user as an outbound reply (not internal,
// not a system event). Returns (nil, nil) when there is no match.
func (r *SupportMessageRepository) GetMessageForActor(ctx context.Context, id, userID string) (*model.SupportMessage, error) {
	var msg model.SupportMessage
	err := r.db.WithContext(ctx).
		Where("id = ? AND "+ownedReplyClause, id, userID).
		First(&msg).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get message for actor: %w", err)
	}
	return &msg, nil
}

// SoftDeleteMessage marks a message deleted iff the actor authored it.
// A no-op (no error) when nothing matches — the service layer is expected
// to call GetMessageForActor first if it needs to distinguish "not yours"
// from "already gone".
func (r *SupportMessageRepository) SoftDeleteMessage(ctx context.Context, id, userID string) error {
	res := r.db.WithContext(ctx).Model(&model.SupportMessage{}).
		Where("id = ? AND "+ownedReplyClause, id, userID).
		Update("deleted_at", time.Now())
	if res.Error != nil {
		return fmt.Errorf("soft delete message: %w", res.Error)
	}
	return nil
}

// SetCancellableUntil writes the email-fallback cancel-window expiry on a
// message. Called by EmailFallbackService.OnAgentReply right after the
// message is enqueued so the UI countdown matches the actual fire time.
func (r *SupportMessageRepository) SetCancellableUntil(ctx context.Context, id string, t time.Time) error {
	if err := r.db.WithContext(ctx).Model(&model.SupportMessage{}).
		Where("id = ?", id).
		Update("cancellable_until", t).Error; err != nil {
		return fmt.Errorf("set cancellable_until: %w", err)
	}
	return nil
}

// SupportInboxInstallationRepository handles widget installations.
type SupportInboxInstallationRepository struct {
	db *gorm.DB
}

// NewSupportInboxInstallationRepository creates a new SupportInboxInstallationRepository.
func NewSupportInboxInstallationRepository(db *gorm.DB) *SupportInboxInstallationRepository {
	return &SupportInboxInstallationRepository{db: db}
}

// GetByWorkspace returns the widget installation for a workspace.
func (r *SupportInboxInstallationRepository) GetByWorkspace(ctx context.Context, workspaceID string) (*model.SupportWidgetInstallation, error) {
	var inst model.SupportWidgetInstallation
	if err := r.db.WithContext(ctx).Where("workspace_id = ?", workspaceID).First(&inst).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("get widget installation: %w", err)
	}
	return &inst, nil
}

// GetByWidgetKey returns the installation by public widget key.
func (r *SupportInboxInstallationRepository) GetByWidgetKey(ctx context.Context, widgetKey string) (*model.SupportWidgetInstallation, error) {
	var inst model.SupportWidgetInstallation
	if err := r.db.WithContext(ctx).Where("widget_key = ? AND active = true", widgetKey).First(&inst).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("get widget by key: %w", err)
	}
	return &inst, nil
}

// GetByID returns the installation by its UUID.
func (r *SupportInboxInstallationRepository) GetByID(ctx context.Context, id string) (*model.SupportWidgetInstallation, error) {
	var inst model.SupportWidgetInstallation
	if err := r.db.WithContext(ctx).Where("id = ? AND active = true", id).First(&inst).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("get widget by id: %w", err)
	}
	return &inst, nil
}

// Create creates a new installation.
func (r *SupportInboxInstallationRepository) Create(ctx context.Context, inst *model.SupportWidgetInstallation) error {
	if err := r.db.WithContext(ctx).Create(inst).Error; err != nil {
		return fmt.Errorf("create widget installation: %w", err)
	}
	return nil
}

// Update saves an installation.
func (r *SupportInboxInstallationRepository) Update(ctx context.Context, inst *model.SupportWidgetInstallation) error {
	if err := r.db.WithContext(ctx).Save(inst).Error; err != nil {
		return fmt.Errorf("update widget installation: %w", err)
	}
	return nil
}

// ListAllActive returns all active widget installations across all workspaces.
func (r *SupportInboxInstallationRepository) ListAllActive(ctx context.Context) ([]model.SupportWidgetInstallation, error) {
	var installations []model.SupportWidgetInstallation
	if err := r.db.WithContext(ctx).Where("active = true").Find(&installations).Error; err != nil {
		return nil, fmt.Errorf("list active installations: %w", err)
	}
	return installations, nil
}

// RegenerateKeys updates just the widget_key and secret_key columns.
func (r *SupportInboxInstallationRepository) RegenerateKeys(ctx context.Context, id, widgetKey, secretKey string) error {
	if err := r.db.WithContext(ctx).Model(&model.SupportWidgetInstallation{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{"widget_key": widgetKey, "secret_key": secretKey}).Error; err != nil {
		return fmt.Errorf("regenerate widget keys: %w", err)
	}
	return nil
}

// SupportInboxSessionRepository handles widget sessions.
type SupportInboxSessionRepository struct {
	db *gorm.DB
}

// NewSupportInboxSessionRepository creates a new SupportInboxSessionRepository.
func NewSupportInboxSessionRepository(db *gorm.DB) *SupportInboxSessionRepository {
	return &SupportInboxSessionRepository{db: db}
}

// GetByToken returns a session by token.
func (r *SupportInboxSessionRepository) GetByToken(ctx context.Context, token string) (*model.SupportWidgetSession, error) {
	var session model.SupportWidgetSession
	if err := r.db.WithContext(ctx).Where("session_token = ?", token).First(&session).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("get widget session: %w", err)
	}
	return &session, nil
}

// Create creates a new session.
func (r *SupportInboxSessionRepository) Create(ctx context.Context, session *model.SupportWidgetSession) error {
	values := map[string]interface{}{
		"workspace_id":    session.WorkspaceID,
		"conversation_id": session.ConversationID,
		"session_token":   session.SessionToken,
		"anonymous_id":    session.AnonymousID,
		"is_anonymous":    session.IsAnonymous,
		"customer_name":   session.CustomerName,
		"customer_email":  session.CustomerEmail,
		"customer_phone":  session.CustomerPhone,
		"user_agent":      session.UserAgent,
		"last_page_url":   session.LastPageURL,
		"timezone":        session.Timezone,
		"locale":          session.Locale,
		"ip_address":      session.IPAddress,
		"country_code":    session.CountryCode,
		"country_name":    session.CountryName,
		"region_name":     session.RegionName,
		"city_name":       session.CityName,
		"revoked_at":      session.RevokedAt,
		"expires_at":      session.ExpiresAt,
	}
	if session.ID != "" {
		values["id"] = session.ID
	}
	if !session.CreatedAt.IsZero() {
		values["created_at"] = session.CreatedAt
	}

	if err := r.db.WithContext(ctx).Model(&model.SupportWidgetSession{}).Create(values).Error; err != nil {
		return fmt.Errorf("create widget session: %w", err)
	}

	created, err := r.GetByToken(ctx, session.SessionToken)
	if err != nil {
		return err
	}
	if created == nil {
		return fmt.Errorf("create widget session: session not found after insert")
	}
	*session = *created
	return nil
}

// Update saves a session.
func (r *SupportInboxSessionRepository) Update(ctx context.Context, session *model.SupportWidgetSession) error {
	if err := r.db.WithContext(ctx).Save(session).Error; err != nil {
		return fmt.Errorf("update widget session: %w", err)
	}
	return nil
}

// UpdatePageURL updates only the last_page_url on a session by token in a single query.
func (r *SupportInboxSessionRepository) UpdatePageURL(ctx context.Context, sessionToken, url string) error {
	result := r.db.WithContext(ctx).
		Model(&model.SupportWidgetSession{}).
		Where("session_token = ? AND revoked_at IS NULL", sessionToken).
		Update("last_page_url", url)
	if result.Error != nil {
		return fmt.Errorf("update session page url: %w", result.Error)
	}
	return nil
}

// GetLatestByConversation returns the most recent session for a conversation.
func (r *SupportInboxSessionRepository) GetLatestByConversation(ctx context.Context, workspaceID, conversationID string) (*model.SupportWidgetSession, error) {
	var session model.SupportWidgetSession
	if err := r.db.WithContext(ctx).
		Where("workspace_id = ? AND conversation_id = ?", workspaceID, conversationID).
		Order("created_at DESC").
		First(&session).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("get latest session by conversation: %w", err)
	}
	return &session, nil
}

// GetLatestByAnonymousID returns the most recent session for an anonymous visitor.
func (r *SupportInboxSessionRepository) GetLatestByAnonymousID(ctx context.Context, workspaceID, anonymousID string) (*model.SupportWidgetSession, error) {
	var session model.SupportWidgetSession
	if err := r.db.WithContext(ctx).
		Where("workspace_id = ? AND anonymous_id = ?", workspaceID, anonymousID).
		Order("created_at DESC").
		First(&session).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("get latest session by anonymous_id: %w", err)
	}
	return &session, nil
}

// ListGeoBackfillCandidates returns recent sessions that have an IP address but no country metadata yet.
func (r *SupportInboxSessionRepository) ListGeoBackfillCandidates(ctx context.Context, limit int) ([]model.SupportWidgetSession, error) {
	if limit <= 0 {
		limit = 500
	}

	var sessions []model.SupportWidgetSession
	if err := r.db.WithContext(ctx).
		Where("COALESCE(LENGTH(TRIM(ip_address)), 0) > 0").
		Where("COALESCE(LENGTH(TRIM(country_code)), 0) = 0").
		Order("created_at DESC").
		Limit(limit).
		Find(&sessions).Error; err != nil {
		return nil, fmt.Errorf("list geo backfill sessions: %w", err)
	}
	return sessions, nil
}

// UpdateGeoLocation updates the session's stored GeoIP fields.
func (r *SupportInboxSessionRepository) UpdateGeoLocation(ctx context.Context, sessionID string, countryCode, countryName, regionName, cityName *string) error {
	updates := map[string]interface{}{
		"country_code": countryCode,
		"country_name": countryName,
		"region_name":  regionName,
		"city_name":    cityName,
	}
	if err := r.db.WithContext(ctx).
		Model(&model.SupportWidgetSession{}).
		Where("id = ?", sessionID).
		Updates(updates).Error; err != nil {
		return fmt.Errorf("update session geo location: %w", err)
	}
	return nil
}

// SupportConversationRepository handles DB operations for support conversations.
type SupportConversationRepository struct {
	db *gorm.DB
}

// NewSupportConversationRepository creates a new SupportConversationRepository.
func NewSupportConversationRepository(db *gorm.DB) *SupportConversationRepository {
	return &SupportConversationRepository{db: db}
}

func isElevatedSupportRole(role string) bool {
	return role == model.RoleOwner || role == model.RoleAdmin
}

func (r *SupportConversationRepository) epochExpr() string {
	if r.db != nil && r.db.Dialector.Name() == "sqlite" {
		return "'1970-01-01 00:00:00'"
	}
	return "'1970-01-01'::timestamptz"
}

func (r *SupportConversationRepository) nowExpr() string {
	if r.db != nil && r.db.Dialector.Name() == "sqlite" {
		return "CURRENT_TIMESTAMP"
	}
	return "NOW()"
}

func (r *SupportConversationRepository) textPrefixExpr(column string, limit int) string {
	if r.db != nil && r.db.Dialector.Name() == "sqlite" {
		return fmt.Sprintf("substr(%s, 1, %d)", column, limit)
	}
	return fmt.Sprintf("LEFT(%s, %d)", column, limit)
}

var (
	mdAutolinkPattern       = regexp.MustCompile(`<((?:https?|mailto):[^>\s]+)>`)
	mdImageInlinePattern    = regexp.MustCompile(`!\[([^\]]*)\]\([^)]*\)`)
	mdLinkInlinePattern     = regexp.MustCompile(`\[([^\]]+)\]\([^)]*\)`)
	mdHTMLTagPattern        = regexp.MustCompile(`<[^>]+>`)
	mdHeadingPattern        = regexp.MustCompile(`(?m)^\s{0,3}#{1,6}\s+`)
	mdBlockquotePattern     = regexp.MustCompile(`(?m)^\s{0,3}>\s?`)
	mdListBulletPattern     = regexp.MustCompile(`(?m)^\s{0,3}(?:[-*+]|\d+[.)])\s+`)
	mdTableSepPattern       = regexp.MustCompile(`(?m)^\s*\|?\s*:?-{3,}:?\s*(?:\|\s*:?-{3,}:?\s*)+\|?\s*$`)
	mdEmphasisPattern       = regexp.MustCompile("(\\*\\*|__|\\*|_|`)")
	whitespacePattern       = regexp.MustCompile(`\s+`)
	spaceBeforePunctPattern = regexp.MustCompile(`\s+([,.;:!?\)])`)
)

// cleanMessageSnippet renders a plain-text preview of a Markdown or HTML
// message body for inbox row display. It unwraps autolinks, link/image
// syntax, and table separators, strips emphasis markers, collapses
// whitespace, and truncates to limit characters with an ellipsis.
func cleanMessageSnippet(raw string, limit int) string {
	const notePrefix = "Note: "
	hasNote := strings.HasPrefix(raw, notePrefix)
	if hasNote {
		raw = strings.TrimPrefix(raw, notePrefix)
	}

	cleaned := raw
	cleaned = mdAutolinkPattern.ReplaceAllString(cleaned, "$1")
	cleaned = mdImageInlinePattern.ReplaceAllString(cleaned, "$1")
	cleaned = mdLinkInlinePattern.ReplaceAllString(cleaned, "$1")
	cleaned = mdTableSepPattern.ReplaceAllString(cleaned, " ")
	cleaned = mdHTMLTagPattern.ReplaceAllString(cleaned, " ")
	cleaned = strings.NewReplacer("&nbsp;", " ", "&amp;", "&", "&lt;", "<", "&gt;", ">", "&#39;", "'", "&quot;", `"`).Replace(cleaned)
	cleaned = mdHeadingPattern.ReplaceAllString(cleaned, "")
	cleaned = mdBlockquotePattern.ReplaceAllString(cleaned, "")
	cleaned = mdListBulletPattern.ReplaceAllString(cleaned, "")
	cleaned = strings.ReplaceAll(cleaned, "|", " ")
	cleaned = mdEmphasisPattern.ReplaceAllString(cleaned, "")
	cleaned = whitespacePattern.ReplaceAllString(cleaned, " ")
	cleaned = spaceBeforePunctPattern.ReplaceAllString(cleaned, "$1")
	cleaned = strings.TrimSpace(cleaned)

	if limit > 0 && len([]rune(cleaned)) > limit {
		runes := []rune(cleaned)
		cleaned = strings.TrimRight(string(runes[:limit]), " ") + "…"
	}
	if hasNote {
		cleaned = notePrefix + cleaned
	}
	return cleaned
}

func (r *SupportConversationRepository) latestSessionCountryExpr(column, alias string) string {
	return fmt.Sprintf(`COALESCE(
		(SELECT sws.%s
			FROM support_widget_sessions sws
			WHERE sws.workspace_id = %s.workspace_id
			  AND sws.conversation_id = %s.id
			  AND sws.%s IS NOT NULL
			  AND sws.%s <> ''
			ORDER BY sws.created_at DESC
			LIMIT 1),
		(SELECT sws.%s
			FROM support_widget_sessions sws
			WHERE sws.workspace_id = %s.workspace_id
			  AND %s.anonymous_id IS NOT NULL
			  AND sws.anonymous_id = %s.anonymous_id
			  AND sws.%s IS NOT NULL
			  AND sws.%s <> ''
			ORDER BY sws.created_at DESC
			LIMIT 1)
	)`, column, alias, alias, column, column, column, alias, alias, alias, column, column)
}

func (r *SupportConversationRepository) maybeAcquireWorkspaceDisplayIDLock(tx *gorm.DB, workspaceID string) {
	if tx == nil || tx.Dialector.Name() != "postgres" {
		return
	}
	tx.Exec("SELECT pg_advisory_xact_lock(hashtext(?))", workspaceID)
}

func conversationAIActiveCondition(alias string) string {
	return fmt.Sprintf("(COALESCE(%s.human_takeover, false) = false AND (COALESCE(%s.flow_state, '') = '%s' OR (COALESCE(%s.flow_state, '') = '' AND COALESCE(%s.ai_state, '') = 'pending')))",
		alias,
		alias,
		model.SupportConversationFlowStateAIHandling,
		alias,
		alias,
	)
}

func conversationResolvedByAICondition(alias string) string {
	return fmt.Sprintf("(COALESCE(%s.human_takeover, false) = false AND (COALESCE(%s.flow_state, '') = '%s' OR (COALESCE(%s.flow_state, '') = '' AND COALESCE(%s.ai_state, '') = 'resolved')))",
		alias,
		alias,
		model.SupportConversationFlowStateResolvedByAI,
		alias,
		alias,
	)
}

func conversationAIHandoffCondition(alias string) string {
	return fmt.Sprintf(`(
		COALESCE(%s.ai_state, '') = 'escalated'
		OR %s.ai_escalated_at IS NOT NULL
		OR (%s.customer_requested_human_at IS NOT NULL AND (%s.ai_state IS NOT NULL OR COALESCE(%s.ai_turn_count, 0) > 0))
		OR (
			COALESCE(%s.flow_state, '') IN ('%s', '%s', '%s')
			AND (%s.ai_state IS NOT NULL OR COALESCE(%s.ai_turn_count, 0) > 0)
		)
	)`,
		alias,
		alias,
		alias, alias, alias,
		alias,
		model.SupportConversationFlowStateWaitingForHuman,
		model.SupportConversationFlowStateQueuedForHuman,
		model.SupportConversationFlowStateAfterHoursQueue,
		alias, alias,
	)
}

func compactStrings(values []string) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

func conversationHumanQueueCondition(alias string) string {
	return fmt.Sprintf("NOT (%s) AND NOT (%s)",
		conversationAIActiveCondition(alias),
		conversationResolvedByAICondition(alias),
	)
}

func conversationHumanInboxCondition(alias string) string {
	return fmt.Sprintf("(%s.status IN ('%s', '%s') AND (%s OR %s.ai_state = 'escalated' OR %s.customer_requested_human_at IS NOT NULL))",
		alias,
		model.SupportConversationStatusOpen,
		model.SupportConversationStatusWaitingOnCustomer,
		conversationHumanQueueCondition(alias),
		alias,
		alias,
	)
}

func conversationHumanResolvedCondition(alias string) string {
	return fmt.Sprintf("(%s.status = '%s' AND NOT (%s))",
		alias,
		model.SupportConversationStatusResolved,
		conversationResolvedByAICondition(alias),
	)
}

func applyConversationFlowState(query *gorm.DB, alias, flowState string) *gorm.DB {
	trimmed := strings.TrimSpace(flowState)
	if trimmed == "" {
		return query
	}
	switch trimmed {
	case model.SupportConversationFlowStateAIHandling:
		return query.Where(conversationAIActiveCondition(alias))
	case model.SupportConversationFlowStateResolvedByAI:
		return query.Where(conversationResolvedByAICondition(alias))
	case model.SupportConversationFlowStateWaitingForHuman:
		return query.Where(fmt.Sprintf("(%s.flow_state = ? OR %s.flow_state = ?)", alias, alias),
			model.SupportConversationFlowStateWaitingForHuman,
			model.SupportConversationFlowStateQueuedForHuman,
		)
	default:
		return query.Where(fmt.Sprintf("%s.flow_state = ?", alias), trimmed)
	}
}

// ConversationListParams contains user-facing list filters before mailbox access is resolved.
type ConversationListParams struct {
	WorkspaceID string
	UserID      string
	Status      string
	Statuses    []string
	Priority    string
	Pagination  model.PMPagination
	MailboxID   *string
	MailboxIDs  []string
	FlowState   string
	Search      string
	Filter      string
	AssignedTo  string
	Sort        string
	AIState     []string
	TagIDs      []string
	SystemTags  []string
	AIFilters   []string
}

// ConversationRepositoryListParams adds resolved actor access data for repository queries.
type ConversationRepositoryListParams struct {
	ConversationListParams
	WorkspaceMemberID string
	Role              string
}

func (r *SupportConversationRepository) mentionExistsCondition(alias string, userID string) (string, []any) {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return "1 = 0", nil
	}
	if r.db.Dialector.Name() == "sqlite" {
		return fmt.Sprintf(`EXISTS (
			SELECT 1
			FROM support_messages sm_mention
			WHERE sm_mention.conversation_id = %s.id
			  AND sm_mention.workspace_id = %s.workspace_id
			  AND sm_mention.deleted_at IS NULL
			  AND sm_mention.metadata LIKE ?
			  AND sm_mention.metadata LIKE ?
		)`, alias, alias), []any{"%mentioned_user_ids%", "%" + userID + "%"}
	}
	filterJSON, _ := json.Marshal(map[string][]string{"mentioned_user_ids": {userID}})
	return fmt.Sprintf(`EXISTS (
		SELECT 1
		FROM support_messages sm_mention
		WHERE sm_mention.conversation_id = %s.id
		  AND sm_mention.workspace_id = %s.workspace_id
		  AND sm_mention.deleted_at IS NULL
		  AND sm_mention.metadata::jsonb @> ?::jsonb
	)`, alias, alias), []any{string(filterJSON)}
}

func (r *SupportConversationRepository) applyMineFilter(query *gorm.DB, alias, userID string) *gorm.DB {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return query.Where("1 = 0")
	}
	mentionCondition, mentionArgs := r.mentionExistsCondition(alias, userID)
	args := []any{
		model.SupportConversationStatusWaitingOnCustomer,
		userID,
		userID,
	}
	args = append(args, mentionArgs...)
	return query.Where(fmt.Sprintf(`(%s OR %s.status = ?) AND (
		%s.assigned_user_id = ?
		OR %s.opened_by_user_id = ?
		OR %s
	)`, conversationHumanInboxCondition(alias), alias, alias, alias, mentionCondition), args...)
}

func (r *SupportConversationRepository) applyConversationListFilter(query *gorm.DB, alias, filter, userID string) *gorm.DB {
	switch strings.TrimSpace(strings.ToLower(filter)) {
	case model.SupportConversationListFilterInbox:
		return query.Where(conversationHumanInboxCondition(alias))
	case model.SupportConversationListFilterMine, model.SupportConversationListFilterMentions:
		return r.applyMineFilter(query, alias, userID)
	case model.SupportConversationListFilterResolved:
		return query.Where(fmt.Sprintf("%s.status = ?", alias), model.SupportConversationStatusResolved)
	default:
		return query
	}
}

func applyConversationTagFilters(query *gorm.DB, alias string, tagIDs, systemTags []string) *gorm.DB {
	tagIDs = compactStrings(tagIDs)
	systemTags = compactStrings(systemTags)
	if len(tagIDs) == 0 && len(systemTags) == 0 {
		return query
	}
	conditions := make([]string, 0, 2+len(systemTags))
	args := make([]any, 0, len(tagIDs))
	if len(tagIDs) > 0 {
		conditions = append(conditions, fmt.Sprintf(`EXISTS (
			SELECT 1
			FROM support_conversation_tags sct
			WHERE sct.conversation_id = %s.id
			  AND sct.tag_id IN ?
		)`, alias))
		args = append(args, tagIDs)
	}
	for _, tag := range systemTags {
		switch tag {
		case model.SupportSystemTagAIHandoff:
			conditions = append(conditions, conversationAIHandoffCondition(alias))
		case model.SupportSystemTagAIResolved:
			conditions = append(conditions, conversationResolvedByAICondition(alias))
		}
	}
	if len(conditions) == 0 {
		return query.Where("1 = 0")
	}
	return query.Where("("+strings.Join(conditions, " OR ")+")", args...)
}

func applyConversationAIFilters(query *gorm.DB, alias string, aiFilters []string) *gorm.DB {
	aiFilters = compactStrings(aiFilters)
	if len(aiFilters) == 0 {
		return query
	}
	conditions := make([]string, 0, len(aiFilters))
	for _, filter := range aiFilters {
		switch strings.TrimSpace(strings.ToLower(filter)) {
		case model.SupportAIFilterHandling, "ai_handling", "ai-active", "ai_active":
			conditions = append(conditions, conversationAIActiveCondition(alias))
		case model.SupportAIFilterHandoff, model.SupportSystemTagAIHandoff, "needs_human":
			conditions = append(conditions, conversationAIHandoffCondition(alias))
		case model.SupportAIFilterResolved, model.SupportSystemTagAIResolved, "resolved_by_ai":
			conditions = append(conditions, conversationResolvedByAICondition(alias))
		}
	}
	if len(conditions) == 0 {
		return query.Where("1 = 0")
	}
	return query.Where("(" + strings.Join(conditions, " OR ") + ")")
}

func (r *SupportConversationRepository) applyConversationAssignmentFilter(query *gorm.DB, alias, assignedTo, userID string) *gorm.DB {
	filters := compactStrings(strings.Split(assignedTo, ","))
	if len(filters) == 0 {
		return query
	}
	conditions := make([]string, 0, len(filters))
	args := make([]any, 0, len(filters))
	seen := make(map[string]bool, len(filters))
	for _, filter := range filters {
		switch strings.TrimSpace(strings.ToLower(filter)) {
		case "me":
			if seen["me"] {
				continue
			}
			seen["me"] = true
			userID = strings.TrimSpace(userID)
			if userID == "" {
				conditions = append(conditions, "1 = 0")
				continue
			}
			conditions = append(conditions, fmt.Sprintf("%s.assigned_user_id = ?", alias))
			args = append(args, userID)
		case "mentioned_me":
			if seen["mentioned_me"] {
				continue
			}
			seen["mentioned_me"] = true
			userID = strings.TrimSpace(userID)
			mentionCondition, mentionArgs := r.mentionExistsCondition(alias, userID)
			conditions = append(conditions, mentionCondition)
			args = append(args, mentionArgs...)
		case "opened_by_me":
			if seen["opened_by_me"] {
				continue
			}
			seen["opened_by_me"] = true
			userID = strings.TrimSpace(userID)
			if userID == "" {
				conditions = append(conditions, "1 = 0")
				continue
			}
			conditions = append(conditions, fmt.Sprintf("%s.opened_by_user_id = ?", alias))
			args = append(args, userID)
		case "unassigned":
			if seen["unassigned"] {
				continue
			}
			seen["unassigned"] = true
			conditions = append(conditions, fmt.Sprintf("%s.assigned_user_id IS NULL AND %s.assigned_agent_id IS NULL", alias, alias))
		case "others":
			if seen["others"] {
				continue
			}
			seen["others"] = true
			userID = strings.TrimSpace(userID)
			if userID == "" {
				conditions = append(conditions, fmt.Sprintf("%s.assigned_user_id IS NOT NULL", alias))
				continue
			}
			conditions = append(conditions, fmt.Sprintf("%s.assigned_user_id IS NOT NULL AND %s.assigned_user_id <> ?", alias, alias))
			args = append(args, userID)
		case "none":
			conditions = append(conditions, "1 = 0")
		}
	}
	if len(conditions) == 0 {
		return query.Where("1 = 0")
	}
	return query.Where("("+strings.Join(conditions, " OR ")+")", args...)
}

func (r *SupportConversationRepository) applyMailboxScopes(query *gorm.DB, alias string, mailboxID *string, mailboxIDs []string) *gorm.DB {
	mailboxIDs = compactStrings(mailboxIDs)
	if len(mailboxIDs) == 0 {
		return r.applyMailboxScope(query, alias, mailboxID)
	}
	ids := make([]string, 0, len(mailboxIDs))
	includeShared := false
	seen := make(map[string]bool, len(mailboxIDs))
	for _, mailboxID := range mailboxIDs {
		trimmed := strings.TrimSpace(mailboxID)
		if trimmed == "" || trimmed == "shared" {
			includeShared = true
			continue
		}
		if seen[trimmed] {
			continue
		}
		seen[trimmed] = true
		ids = append(ids, trimmed)
	}
	switch {
	case includeShared && len(ids) > 0:
		return query.Where(fmt.Sprintf("(%s.mailbox_id IS NULL OR %s.mailbox_id IN ?)", alias, alias), ids)
	case includeShared:
		return query.Where(fmt.Sprintf("%s.mailbox_id IS NULL", alias))
	case len(ids) > 0:
		return query.Where(fmt.Sprintf("%s.mailbox_id IN ?", alias), ids)
	default:
		return query
	}
}

func (r *SupportConversationRepository) applyConversationListParams(query *gorm.DB, alias string, params ConversationRepositoryListParams) *gorm.DB {
	query = r.applyMailboxAccess(query, alias, params.WorkspaceMemberID, params.Role)
	query = r.applyMailboxScopes(query, alias, params.MailboxID, params.MailboxIDs)
	query = applyConversationFlowState(query, alias, params.FlowState)
	query = r.applyConversationSearch(query, strings.TrimSpace(params.Search))
	query = r.applyConversationListFilter(query, alias, params.Filter, params.UserID)
	query = r.applyConversationAssignmentFilter(query, alias, params.AssignedTo, params.UserID)
	query = applyConversationTagFilters(query, alias, params.TagIDs, params.SystemTags)
	query = applyConversationAIFilters(query, alias, params.AIFilters)
	if len(params.Statuses) > 0 {
		query = query.Where(fmt.Sprintf("%s.status IN ?", alias), params.Statuses)
	} else if params.Status != "" {
		query = query.Where(fmt.Sprintf("%s.status = ?", alias), params.Status)
	}
	if params.Priority != "" {
		query = query.Where(fmt.Sprintf("%s.priority = ?", alias), params.Priority)
	}
	if len(params.AIState) > 0 && params.AIState[0] != "" {
		if params.AIState[0] == "any" {
			query = query.Where(fmt.Sprintf("%s.ai_state IS NOT NULL", alias))
		} else {
			query = query.Where(fmt.Sprintf("%s.ai_state = ?", alias), params.AIState[0])
		}
	}
	return query
}

func conversationListOrder(sortOrder string) string {
	if strings.EqualFold(strings.TrimSpace(sortOrder), "oldest") {
		return "support_conversations.updated_at ASC"
	}
	return "support_conversations.updated_at DESC"
}

// List returns conversations with optional filters and pagination.
func (r *SupportConversationRepository) List(ctx context.Context, params ConversationRepositoryListParams) ([]model.SupportConversation, int64, error) {
	base := r.db.WithContext(ctx).Model(&model.SupportConversation{}).Where("support_conversations.workspace_id = ?", params.WorkspaceID)
	base = r.applyConversationListParams(base, "support_conversations", params)

	var total int64
	if err := base.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("count conversations: %w", err)
	}

	page := params.Pagination.Page
	perPage := params.Pagination.PerPage
	if page <= 0 {
		page = 1
	}
	if perPage <= 0 {
		perPage = 50
	}
	offset := (page - 1) * perPage

	// Fresh query for fetch — Count() taints the SELECT clause
	fetch := r.db.WithContext(ctx).Table("support_conversations").Where("support_conversations.workspace_id = ?", params.WorkspaceID)
	fetch = r.applyConversationListParams(fetch, "support_conversations", params)

	var conversations []model.SupportConversation
	if err := fetch.
		Joins("LEFT JOIN support_mailboxes sm ON sm.id = support_conversations.mailbox_id").
		Select(fmt.Sprintf(`support_conversations.*, (
			SELECT CASE WHEN m.is_internal THEN 'Note: ' || %s ELSE %s END
			FROM support_messages m
			WHERE m.conversation_id = support_conversations.id
			  AND m.deleted_at IS NULL
			ORDER BY m.created_at DESC LIMIT 1
		) AS last_message,
		(SELECT COUNT(*)
			FROM support_messages sm
			WHERE sm.conversation_id = support_conversations.id
			  AND sm.deleted_at IS NULL
			  AND sm.is_internal = false
			  AND sm.sender_type = 'customer'
			  AND sm.message_type = 'reply'
			  AND sm.created_at > COALESCE(support_conversations.team_last_seen_at, %s)
		) AS unread_count,
		%s AS country_code,
		%s AS country_name,
		sm.name AS mailbox_name,
		sm.handle AS mailbox_handle,
		sm.icon AS mailbox_icon`,
			r.textPrefixExpr("m.content", 500),
			r.textPrefixExpr("m.content", 500),
			r.epochExpr(),
			r.latestSessionCountryExpr("country_code", "support_conversations"),
			r.latestSessionCountryExpr("country_name", "support_conversations"),
		)).
		Order(conversationListOrder(params.Sort)).Offset(offset).Limit(perPage).Find(&conversations).Error; err != nil {
		return nil, 0, fmt.Errorf("list conversations: %w", err)
	}
	for i := range conversations {
		if conversations[i].LastMessage == nil {
			continue
		}
		cleaned := cleanMessageSnippet(*conversations[i].LastMessage, 100)
		conversations[i].LastMessage = &cleaned
	}
	return conversations, total, nil
}

// CountByParams returns total and unread counts for the same filter set used by List.
func (r *SupportConversationRepository) CountByParams(ctx context.Context, params ConversationRepositoryListParams) (int, int, error) {
	buildQuery := func() *gorm.DB {
		query := r.db.WithContext(ctx).
			Table("support_conversations AS sc").
			Where("sc.workspace_id = ?", params.WorkspaceID)
		return r.applyConversationListParams(query, "sc", params)
	}

	var total int64
	if err := buildQuery().Count(&total).Error; err != nil {
		return 0, 0, fmt.Errorf("count conversations: %w", err)
	}

	unreadCondition := fmt.Sprintf(`EXISTS (
		SELECT 1
		FROM support_messages sm
		WHERE sm.conversation_id = sc.id
		  AND sm.deleted_at IS NULL
		  AND sm.is_internal = false
		  AND sm.sender_type = 'customer'
		  AND sm.message_type = 'reply'
		  AND sm.created_at > COALESCE(sc.team_last_seen_at, %s)
	)`, r.epochExpr())

	var unread int64
	if err := buildQuery().Where(unreadCondition).Count(&unread).Error; err != nil {
		return 0, 0, fmt.Errorf("count unread conversations: %w", err)
	}

	return int(total), int(unread), nil
}

func (r *SupportConversationRepository) ListCoverageAnalysisCandidates(ctx context.Context, workspaceID string, windowStart, windowEnd time.Time, limit int) ([]model.SupportConversation, error) {
	if workspaceID == "" {
		return nil, fmt.Errorf("workspace_id is required")
	}
	if limit <= 0 || limit > 500 {
		limit = 200
	}

	var conversations []model.SupportConversation
	err := r.db.WithContext(ctx).
		Where("workspace_id = ?", workspaceID).
		Where("status <> ?", model.SupportConversationStatusSpam).
		Where(`(
			(updated_at >= ? AND updated_at < ?)
			OR (resolved_at IS NOT NULL AND resolved_at >= ? AND resolved_at < ?)
		)`, windowStart, windowEnd, windowStart, windowEnd).
		Order("updated_at ASC, id ASC").
		Limit(limit).
		Find(&conversations).Error
	if err != nil {
		return nil, fmt.Errorf("list coverage analysis candidates: %w", err)
	}
	return conversations, nil
}

func (r *SupportConversationRepository) ListWorkspacesForCoverageAnalysisCandidates(ctx context.Context, windowStart, windowEnd time.Time, limit int) ([]string, error) {
	if limit <= 0 || limit > 5000 {
		limit = 1000
	}
	var workspaceIDs []string
	err := r.db.WithContext(ctx).
		Model(&model.SupportConversation{}).
		Distinct("workspace_id").
		Where("status <> ?", model.SupportConversationStatusSpam).
		Where(`(
			(updated_at >= ? AND updated_at < ?)
			OR (resolved_at IS NOT NULL AND resolved_at >= ? AND resolved_at < ?)
		)`, windowStart, windowEnd, windowStart, windowEnd).
		Order("workspace_id ASC").
		Limit(limit).
		Pluck("workspace_id", &workspaceIDs).Error
	if err != nil {
		return nil, fmt.Errorf("list coverage analysis workspaces: %w", err)
	}
	return workspaceIDs, nil
}

func (r *SupportConversationRepository) applyConversationSearch(query *gorm.DB, search string) *gorm.DB {
	if search == "" {
		return query
	}
	escaped := escapeLike(search)
	pattern := "%" + escaped + "%"
	if r.db.Dialector.Name() == "sqlite" {
		lowerPattern := strings.ToLower(pattern)
		return query.Where(`(
			LOWER(support_conversations.subject) LIKE ? ESCAPE '\'
			OR LOWER(COALESCE(support_conversations.customer_name, '')) LIKE ? ESCAPE '\'
			OR LOWER(COALESCE(support_conversations.customer_email, '')) LIKE ? ESCAPE '\'
			OR CAST(support_conversations.display_id AS TEXT) LIKE ? ESCAPE '\'
		)`, lowerPattern, lowerPattern, lowerPattern, pattern)
	}
	return query.Where(`(
		support_conversations.subject ILIKE ? ESCAPE '\'
		OR COALESCE(support_conversations.customer_name, '') ILIKE ? ESCAPE '\'
		OR COALESCE(support_conversations.customer_email, '') ILIKE ? ESCAPE '\'
		OR CAST(support_conversations.display_id AS TEXT) LIKE ? ESCAPE '\'
	)`, pattern, pattern, pattern, pattern)
}

// GetByID returns a single conversation.
func (r *SupportConversationRepository) GetByID(ctx context.Context, workspaceID, id, workspaceMemberID, role string) (*model.SupportConversation, error) {
	var conversation model.SupportConversation
	query := r.db.WithContext(ctx).
		Table("support_conversations").
		Joins("LEFT JOIN support_mailboxes sm ON sm.id = support_conversations.mailbox_id").
		Where("support_conversations.workspace_id = ? AND support_conversations.id = ?", workspaceID, id)
	query = r.applyMailboxAccess(query, "support_conversations", workspaceMemberID, role)
	if err := query.Select(fmt.Sprintf("support_conversations.*, %s AS country_code, %s AS country_name, sm.name AS mailbox_name, sm.handle AS mailbox_handle, sm.icon AS mailbox_icon",
		r.latestSessionCountryExpr("country_code", "support_conversations"),
		r.latestSessionCountryExpr("country_name", "support_conversations"),
	)).First(&conversation).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("get conversation: %w", err)
	}
	return &conversation, nil
}

// Create creates a new conversation with auto-allocated display_id.
func (r *SupportConversationRepository) Create(ctx context.Context, conversation *model.SupportConversation) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Advisory lock on workspace to prevent duplicate display_id under concurrency.
		r.maybeAcquireWorkspaceDisplayIDLock(tx, conversation.WorkspaceID)

		var maxDisplayID int
		tx.Model(&model.SupportConversation{}).Where("workspace_id = ?", conversation.WorkspaceID).
			Select("COALESCE(MAX(display_id), 0)").Scan(&maxDisplayID)
		conversation.DisplayID = maxDisplayID + 1

		if err := tx.Create(conversation).Error; err != nil {
			return fmt.Errorf("create conversation: %w", err)
		}
		return nil
	})
}

// Update saves a conversation.
func (r *SupportConversationRepository) Update(ctx context.Context, conversation *model.SupportConversation) error {
	if err := r.db.WithContext(ctx).Save(conversation).Error; err != nil {
		return fmt.Errorf("update conversation: %w", err)
	}
	return nil
}

// UpdateFields updates specific fields on a conversation by ID and workspace.
func (r *SupportConversationRepository) UpdateFields(ctx context.Context, workspaceID, conversationID string, fields map[string]any) error {
	if err := r.db.WithContext(ctx).
		Model(&model.SupportConversation{}).
		Where("id = ? AND workspace_id = ?", conversationID, workspaceID).
		Updates(fields).Error; err != nil {
		return fmt.Errorf("update conversation fields: %w", err)
	}
	return nil
}

// UpdateSubject updates only the subject of a conversation.
func (r *SupportConversationRepository) UpdateSubject(ctx context.Context, id, subject string) error {
	if err := r.db.WithContext(ctx).Model(&model.SupportConversation{}).Where("id = ?", id).Update("subject", subject).Error; err != nil {
		return fmt.Errorf("update conversation subject: %w", err)
	}
	return nil
}

// ListConversationIDsWithMentions returns conversation IDs where the given user was mentioned in internal notes.
func (r *SupportConversationRepository) ListConversationIDsWithMentions(ctx context.Context, workspaceID, userID string) ([]string, error) {
	filterJSON, _ := json.Marshal(map[string][]string{"mentioned_user_ids": {userID}})
	var ids []string
	if err := r.db.WithContext(ctx).
		Table("support_messages").
		Where("workspace_id = ? AND deleted_at IS NULL AND is_internal = true AND metadata::jsonb @> ?::jsonb", workspaceID, string(filterJSON)).
		Distinct().
		Pluck("conversation_id", &ids).Error; err != nil {
		return nil, fmt.Errorf("list conversations with mentions: %w", err)
	}
	return ids, nil
}

// ListByIDs returns conversations by ID for a workspace.
func (r *SupportConversationRepository) ListByIDs(ctx context.Context, workspaceID string, ids []string, workspaceMemberID, role string) ([]model.SupportConversation, error) {
	if len(ids) == 0 {
		return []model.SupportConversation{}, nil
	}

	var conversations []model.SupportConversation
	query := r.db.WithContext(ctx).
		Table("support_conversations").
		Joins("LEFT JOIN support_mailboxes sm ON sm.id = support_conversations.mailbox_id").
		Where("support_conversations.workspace_id = ? AND support_conversations.id IN ?", workspaceID, ids)
	query = r.applyMailboxAccess(query, "support_conversations", workspaceMemberID, role)
	if err := query.
		Select(fmt.Sprintf("support_conversations.*, %s AS country_code, %s AS country_name, sm.name AS mailbox_name, sm.handle AS mailbox_handle, sm.icon AS mailbox_icon",
			r.latestSessionCountryExpr("country_code", "support_conversations"),
			r.latestSessionCountryExpr("country_name", "support_conversations"),
		)).
		Order("updated_at DESC").
		Find(&conversations).Error; err != nil {
		return nil, fmt.Errorf("list conversations by ids: %w", err)
	}
	return conversations, nil
}

// ListByLinkedStoryIDs returns conversations linked to any of the provided tasks.
func (r *SupportConversationRepository) ListByLinkedStoryIDs(ctx context.Context, workspaceID string, storyIDs []string) ([]model.SupportConversation, error) {
	if len(storyIDs) == 0 {
		return []model.SupportConversation{}, nil
	}

	var conversations []model.SupportConversation
	if err := r.db.WithContext(ctx).
		Where("workspace_id = ? AND linked_task_id IN ?", workspaceID, storyIDs).
		Order("updated_at DESC").
		Find(&conversations).Error; err != nil {
		return nil, fmt.Errorf("list conversations by linked stories: %w", err)
	}
	return conversations, nil
}

// ListByContact returns conversations linked to a CRM contact.
func (r *SupportConversationRepository) ListByContact(ctx context.Context, workspaceID, contactID string, pagination model.PMPagination) ([]model.SupportConversation, int64, error) {
	query := r.db.WithContext(ctx).Model(&model.SupportConversation{}).
		Where("workspace_id = ? AND crm_contact_id = ?", workspaceID, contactID)

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("count contact conversations: %w", err)
	}

	page := pagination.Page
	perPage := pagination.PerPage
	if page <= 0 {
		page = 1
	}
	if perPage <= 0 {
		perPage = 50
	}
	offset := (page - 1) * perPage

	var conversations []model.SupportConversation
	if err := query.Order("created_at DESC").Offset(offset).Limit(perPage).Find(&conversations).Error; err != nil {
		return nil, 0, fmt.Errorf("list contact conversations: %w", err)
	}
	return conversations, total, nil
}

// ListByAnonymousID returns conversations for a visitor by anonymous_id.
func (r *SupportConversationRepository) ListByAnonymousID(ctx context.Context, workspaceID, anonymousID string) ([]model.SupportConversation, error) {
	var conversations []model.SupportConversation
	if err := r.db.WithContext(ctx).
		Select(fmt.Sprintf(`support_conversations.*,
		(SELECT content FROM support_messages WHERE support_messages.conversation_id = support_conversations.id AND support_messages.deleted_at IS NULL AND support_messages.is_internal = false ORDER BY created_at DESC LIMIT 1) AS last_message,
		(SELECT COUNT(*)
			FROM support_messages sm
			WHERE sm.conversation_id = support_conversations.id
			  AND sm.deleted_at IS NULL
			  AND sm.is_internal = false
			  AND sm.sender_type IN ('user', 'agent', 'ai')
			  AND sm.message_type = 'reply'
			  AND sm.created_at > COALESCE(support_conversations.contact_last_seen_at, %s)
		) AS unread_count`, r.epochExpr())).
		Where("workspace_id = ? AND anonymous_id = ?", workspaceID, anonymousID).
		Order("updated_at DESC").
		Find(&conversations).Error; err != nil {
		return nil, fmt.Errorf("list conversations by anonymous_id: %w", err)
	}
	for i := range conversations {
		if conversations[i].LastMessage == nil {
			continue
		}
		cleaned := cleanMessageSnippet(*conversations[i].LastMessage, 100)
		conversations[i].LastMessage = &cleaned
	}
	return conversations, nil
}

// MarkInternalRead sets team_last_seen_at = NOW() if unread messages exist beyond the current cursor.
// Uses raw SQL to avoid GORM's autoUpdateTime touching updated_at (which would re-sort the conversation).
func (r *SupportConversationRepository) MarkInternalRead(ctx context.Context, conversationID string) error {
	result := r.db.WithContext(ctx).Exec(fmt.Sprintf(`
		UPDATE support_conversations
		SET team_last_seen_at = %s
		WHERE id = ?
		  AND EXISTS (
			SELECT 1 FROM support_messages sm
			WHERE sm.conversation_id = support_conversations.id
			  AND sm.deleted_at IS NULL
			  AND sm.is_internal = false
			  AND sm.sender_type = 'customer'
			  AND sm.message_type = 'reply'
			  AND sm.created_at > COALESCE(support_conversations.team_last_seen_at, %s)
		  )
	`, r.nowExpr(), r.epochExpr()), conversationID)
	if result.Error != nil {
		return fmt.Errorf("mark internal read: %w", result.Error)
	}
	return nil
}

// MarkUnread resets team_last_seen_at to epoch so the conversation appears unread again.
// Uses raw SQL to avoid GORM's autoUpdateTime touching updated_at.
func (r *SupportConversationRepository) MarkUnread(ctx context.Context, conversationID string) error {
	result := r.db.WithContext(ctx).Exec(fmt.Sprintf(`
		UPDATE support_conversations
		SET team_last_seen_at = %s
		WHERE id = ?
	`, r.epochExpr()), conversationID)
	if result.Error != nil {
		return fmt.Errorf("mark unread: %w", result.Error)
	}
	return nil
}

// Delete permanently removes a conversation and its messages.
func (r *SupportConversationRepository) Delete(ctx context.Context, workspaceID, conversationID string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("workspace_id = ? AND conversation_id = ?", workspaceID, conversationID).Delete(&model.SupportMessage{}).Error; err != nil {
			return fmt.Errorf("delete conversation messages: %w", err)
		}
		if err := tx.Where("workspace_id = ? AND id = ?", workspaceID, conversationID).Delete(&model.SupportConversation{}).Error; err != nil {
			return fmt.Errorf("delete conversation: %w", err)
		}
		return nil
	})
}

// MarkContactRead sets contact_last_seen_at = NOW() if unread messages exist beyond the current cursor.
// Uses raw SQL to avoid GORM's autoUpdateTime touching updated_at (which would re-sort the conversation).
func (r *SupportConversationRepository) MarkContactRead(ctx context.Context, conversationID string) error {
	result := r.db.WithContext(ctx).Exec(fmt.Sprintf(`
		UPDATE support_conversations
		SET contact_last_seen_at = %s
		WHERE id = ?
		  AND EXISTS (
			SELECT 1 FROM support_messages sm
			WHERE sm.conversation_id = support_conversations.id
			  AND sm.deleted_at IS NULL
			  AND sm.is_internal = false
			  AND sm.sender_type IN ('user', 'agent', 'ai')
			  AND sm.message_type = 'reply'
			  AND sm.created_at > COALESCE(support_conversations.contact_last_seen_at, %s)
		  )
	`, r.nowExpr(), r.epochExpr()), conversationID)
	if result.Error != nil {
		return fmt.Errorf("mark contact read: %w", result.Error)
	}
	return nil
}

// GetUnreadStats returns aggregate unread conversation counts for an accessible inbox scope.
func (r *SupportConversationRepository) GetUnreadStats(ctx context.Context, workspaceID, userID, workspaceMemberID, role string, mailboxID *string) (model.UnreadStats, error) {
	var stats model.UnreadStats
	humanInboxCondition := conversationHumanInboxCondition("sc")
	humanOpenInboxCondition := fmt.Sprintf("(%s AND sc.status = '%s')", humanInboxCondition, model.SupportConversationStatusOpen)
	aiActiveCondition := conversationAIActiveCondition("sc")
	mentionCondition, mentionArgs := r.mentionExistsCondition("sc", userID)
	mineCondition := `(` + conversationHumanInboxCondition("sc") + ` OR sc.status = 'waiting_on_customer') AND (
		sc.assigned_user_id = ?
		OR sc.opened_by_user_id = ?
		OR ` + mentionCondition + `
	)`
	unreadCondition := fmt.Sprintf(`(
		SELECT COUNT(*)
		FROM support_messages sm
		WHERE sm.conversation_id = sc.id
		  AND sm.deleted_at IS NULL
		  AND sm.is_internal = false
		  AND sm.sender_type = 'customer'
		  AND sm.message_type = 'reply'
		  AND sm.created_at > COALESCE(sc.team_last_seen_at, %s)
	) > 0`, r.epochExpr())
	baseQuery := fmt.Sprintf(`
		SELECT
			COUNT(*) FILTER (
				WHERE %s
				  AND %s
			) AS inbox,
			COUNT(*) FILTER (
				WHERE %s
				  AND %s
			) AS mine,
			COUNT(*) FILTER (
				WHERE %s
				  AND sc.status = 'waiting_on_customer'
			) AS waiting,
			COUNT(*) FILTER (
				WHERE %s
				  AND %s
			) AS ai_active,
			COUNT(*) FILTER (
				WHERE %s
				  AND %s
			) AS total,
			COUNT(*) FILTER (
				WHERE %s
				  AND %s
			) AS my_inbox,
			COUNT(*) FILTER (
				WHERE %s
				  AND %s
				  AND sc.assigned_agent_id IS NULL
				  AND sc.assigned_user_id IS NULL
			) AS unassigned,
			COUNT(*) FILTER (
				WHERE %s
			) AS inbox_total,
			COUNT(*) FILTER (
				WHERE %s
			) AS mine_total,
			COUNT(*) FILTER (
				WHERE sc.status = 'waiting_on_customer'
			) AS waiting_total,
			COUNT(*) FILTER (
				WHERE %s
			) AS ai_active_total
		FROM support_conversations sc
		WHERE sc.workspace_id = ?
		  AND sc.status NOT IN ('resolved', 'spam')
	`, unreadCondition, humanOpenInboxCondition, unreadCondition, mineCondition, unreadCondition, unreadCondition, aiActiveCondition, unreadCondition, humanOpenInboxCondition, unreadCondition, mineCondition, unreadCondition, humanOpenInboxCondition, humanOpenInboxCondition, mineCondition, aiActiveCondition)

	args := []any{}
	args = append(args, userID, userID)
	args = append(args, mentionArgs...)
	args = append(args, userID, userID)
	args = append(args, mentionArgs...)
	args = append(args, userID, userID)
	args = append(args, mentionArgs...)
	args = append(args, workspaceID)
	if mailboxID != nil {
		if *mailboxID == "" {
			baseQuery += " AND sc.mailbox_id IS NULL"
		} else {
			baseQuery += " AND sc.mailbox_id = ?"
			args = append(args, *mailboxID)
		}
	}
	if !isElevatedSupportRole(role) {
		baseQuery += ` AND (
			sc.mailbox_id IS NULL
			OR sc.mailbox_id IN (
				SELECT sm.id
				FROM support_mailboxes sm
				WHERE sm.active = true
				  AND ` + supportMailboxAccessCondition("sm") + `
			)
		)`
		args = append(args, workspaceMemberID, workspaceMemberID)
	}

	err := r.db.WithContext(ctx).Raw(baseQuery, args...).Scan(&stats).Error
	if err != nil {
		return stats, fmt.Errorf("get unread stats: %w", err)
	}
	return stats, nil
}

func (r *SupportConversationRepository) applyMailboxScope(query *gorm.DB, alias string, mailboxID *string) *gorm.DB {
	if mailboxID == nil {
		return query
	}
	if strings.TrimSpace(*mailboxID) == "" {
		return query.Where(fmt.Sprintf("%s.mailbox_id IS NULL", alias))
	}
	return query.Where(fmt.Sprintf("%s.mailbox_id = ?", alias), strings.TrimSpace(*mailboxID))
}

func (r *SupportConversationRepository) applyMailboxAccess(query *gorm.DB, alias, workspaceMemberID, role string) *gorm.DB {
	if isElevatedSupportRole(role) {
		return query
	}
	if strings.TrimSpace(workspaceMemberID) == "" {
		return query.Where("1 = 0")
	}
	return query.Where(fmt.Sprintf(`
		(
			%s.mailbox_id IS NULL
			OR %s.mailbox_id IN (
				SELECT sm.id
				FROM support_mailboxes sm
				WHERE sm.active = true
				  AND `+supportMailboxAccessCondition("sm")+`
			)
		)
	`, alias, alias), workspaceMemberID, workspaceMemberID)
}

// UpdateIdentityByAnonymousID batch-updates all anonymous conversations for a visitor
// with the provided email, name, and CRM contact ID. Returns the IDs of updated conversations.
func (r *SupportConversationRepository) UpdateIdentityByAnonymousID(ctx context.Context, workspaceID, anonymousID, email, name string, crmContactID *string) ([]string, error) {
	normalizedEmail := strings.ToLower(strings.TrimSpace(email))
	if normalizedEmail == "" {
		return nil, nil
	}

	updates := map[string]interface{}{
		"customer_email": email,
	}
	if strings.TrimSpace(name) != "" {
		updates["customer_name"] = name
	}
	if crmContactID != nil {
		updates["crm_contact_id"] = gorm.Expr("COALESCE(crm_contact_id, ?)", *crmContactID)
	}

	query := r.db.WithContext(ctx).
		Model(&model.SupportConversation{}).
		Where(
			"workspace_id = ? AND anonymous_id = ? AND (customer_email IS NULL OR customer_email = '' OR LOWER(customer_email) = ?)",
			workspaceID, anonymousID, normalizedEmail,
		)

	// First, get the IDs of conversations that will be updated
	var ids []string
	if err := r.db.WithContext(ctx).
		Model(&model.SupportConversation{}).
		Where(
			"workspace_id = ? AND anonymous_id = ? AND (customer_email IS NULL OR customer_email = '' OR LOWER(customer_email) = ?)",
			workspaceID, anonymousID, normalizedEmail,
		).
		Pluck("id", &ids).Error; err != nil {
		return nil, fmt.Errorf("find anonymous conversations: %w", err)
	}

	if len(ids) == 0 {
		return nil, nil
	}

	if err := query.UpdateColumns(updates).Error; err != nil {
		return nil, fmt.Errorf("backfill conversation identity: %w", err)
	}

	return ids, nil
}

// UpdateSessionsByAnonymousID batch-updates all anonymous sessions for a visitor
// with the provided email and name.
func (r *SupportInboxSessionRepository) UpdateSessionsByAnonymousID(ctx context.Context, workspaceID, anonymousID, email, name string) error {
	normalizedEmail := strings.ToLower(strings.TrimSpace(email))
	if normalizedEmail == "" {
		return nil
	}

	updates := map[string]interface{}{
		"customer_email": email,
		"is_anonymous":   false,
	}
	if strings.TrimSpace(name) != "" {
		updates["customer_name"] = name
	}

	if err := r.db.WithContext(ctx).
		Model(&model.SupportWidgetSession{}).
		Where(
			"workspace_id = ? AND anonymous_id = ? AND (is_anonymous = ? OR customer_email IS NULL OR customer_email = '' OR LOWER(customer_email) = ?)",
			workspaceID, anonymousID, true, normalizedEmail,
		).
		Updates(updates).Error; err != nil {
		return fmt.Errorf("backfill session identity: %w", err)
	}
	return nil
}

// DB returns the underlying *gorm.DB for transaction support.
func (r *SupportConversationRepository) DB() *gorm.DB {
	return r.db
}

// WithTx returns a new SupportConversationRepository using the given transaction.
func (r *SupportConversationRepository) WithTx(tx *gorm.DB) *SupportConversationRepository {
	return &SupportConversationRepository{db: tx}
}

// WithTx returns a new SupportInboxSessionRepository using the given transaction.
func (r *SupportInboxSessionRepository) WithTx(tx *gorm.DB) *SupportInboxSessionRepository {
	return &SupportInboxSessionRepository{db: tx}
}

// WithTx returns a new CRMContactRepository using the given transaction.
func (r *CRMContactRepository) WithTx(tx *gorm.DB) *CRMContactRepository {
	return &CRMContactRepository{db: tx}
}

// SupportCannedResponseRepository handles canned responses.
type SupportCannedResponseRepository struct {
	db *gorm.DB
}

// NewSupportCannedResponseRepository creates a new SupportCannedResponseRepository.
func NewSupportCannedResponseRepository(db *gorm.DB) *SupportCannedResponseRepository {
	return &SupportCannedResponseRepository{db: db}
}

// List returns canned responses for a workspace.
func (r *SupportCannedResponseRepository) List(ctx context.Context, workspaceID string) ([]model.SupportCannedResponse, error) {
	var responses []model.SupportCannedResponse
	if err := r.db.WithContext(ctx).
		Where("workspace_id = ?", workspaceID).
		Order("tag ASC, short_code ASC").
		Find(&responses).Error; err != nil {
		return nil, fmt.Errorf("list canned responses: %w", err)
	}
	return responses, nil
}

// Search returns canned responses matching a query.
func (r *SupportCannedResponseRepository) Search(ctx context.Context, workspaceID, query string) ([]model.SupportCannedResponse, error) {
	// Escape LIKE wildcards in the user-provided search term.
	escaped := escapeLike(query)
	pattern := "%" + escaped + "%"

	var responses []model.SupportCannedResponse
	if err := r.db.WithContext(ctx).
		Where("workspace_id = ?", workspaceID).
		Where("short_code LIKE ? OR content LIKE ? OR tag LIKE ?", pattern, pattern, pattern).
		Order("tag ASC, short_code ASC").
		Limit(10).
		Find(&responses).Error; err != nil {
		return nil, fmt.Errorf("search canned responses: %w", err)
	}
	return responses, nil
}

// escapeLike escapes LIKE pattern special characters (%, _, \).
func escapeLike(s string) string {
	replacer := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)
	return replacer.Replace(s)
}

// GetByID returns a single canned response.
func (r *SupportCannedResponseRepository) GetByID(ctx context.Context, workspaceID, id string) (*model.SupportCannedResponse, error) {
	var response model.SupportCannedResponse
	if err := r.db.WithContext(ctx).Where("workspace_id = ? AND id = ?", workspaceID, id).First(&response).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("get canned response: %w", err)
	}
	return &response, nil
}

// GetByShortCode returns a canned response by workspace-scoped shortcut.
func (r *SupportCannedResponseRepository) GetByShortCode(ctx context.Context, workspaceID, shortCode string) (*model.SupportCannedResponse, error) {
	var response model.SupportCannedResponse
	if err := r.db.WithContext(ctx).Where("workspace_id = ? AND short_code = ?", workspaceID, shortCode).First(&response).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("get canned response by short code: %w", err)
	}
	return &response, nil
}

// Create creates a new canned response.
func (r *SupportCannedResponseRepository) Create(ctx context.Context, response *model.SupportCannedResponse) error {
	if err := r.db.WithContext(ctx).Create(response).Error; err != nil {
		return fmt.Errorf("create canned response: %w", err)
	}
	return nil
}

// Update saves a canned response.
func (r *SupportCannedResponseRepository) Update(ctx context.Context, response *model.SupportCannedResponse) error {
	if err := r.db.WithContext(ctx).Save(response).Error; err != nil {
		return fmt.Errorf("update canned response: %w", err)
	}
	return nil
}

// Delete removes a canned response.
func (r *SupportCannedResponseRepository) Delete(ctx context.Context, workspaceID, id string) error {
	if err := r.db.WithContext(ctx).Where("workspace_id = ? AND id = ?", workspaceID, id).Delete(&model.SupportCannedResponse{}).Error; err != nil {
		return fmt.Errorf("delete canned response: %w", err)
	}
	return nil
}
