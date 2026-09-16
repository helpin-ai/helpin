package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"maps"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

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

// ListConversationPageFromNewest returns a bounded page counted backward from
// the newest message, while preserving chronological order within the page.
func (r *SupportMessageRepository) ListConversationPageFromNewest(ctx context.Context, workspaceID, conversationID string, includeInternal bool, limit, offset int) ([]model.SupportMessage, int64, error) {
	query := r.db.WithContext(ctx).Model(&model.SupportMessage{}).
		Where("workspace_id = ? AND conversation_id = ?", workspaceID, conversationID)
	if !includeInternal {
		query = query.Where("is_internal = false")
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("count messages: %w", err)
	}
	var messages []model.SupportMessage
	if err := query.Order("created_at DESC, id DESC").Limit(limit).Offset(offset).Find(&messages).Error; err != nil {
		return nil, 0, fmt.Errorf("list newest messages: %w", err)
	}
	for left, right := 0, len(messages)-1; left < right; left, right = left+1, right-1 {
		messages[left], messages[right] = messages[right], messages[left]
	}
	return messages, total, nil
}

// ListConversationPageBefore returns one chronological page before a stable
// newest-first cursor. It fetches one extra row to determine whether older
// history remains without running a separate count query.
func (r *SupportMessageRepository) ListConversationPageBefore(
	ctx context.Context,
	workspaceID, conversationID string,
	includeInternal bool,
	limit int,
	beforeCreatedAt *time.Time,
	beforeID string,
) ([]model.SupportMessage, bool, error) {
	query := r.db.WithContext(ctx).
		Where("workspace_id = ? AND conversation_id = ?", workspaceID, conversationID)
	if !includeInternal {
		query = query.Where("is_internal = false")
	}
	if beforeCreatedAt != nil {
		query = query.Where(
			"created_at < ? OR (created_at = ? AND id < ?)",
			*beforeCreatedAt,
			*beforeCreatedAt,
			beforeID,
		)
	}

	messages := make([]model.SupportMessage, 0, limit+1)
	if err := query.Order("created_at DESC, id DESC").Limit(limit + 1).Find(&messages).Error; err != nil {
		return nil, false, fmt.Errorf("list message page: %w", err)
	}
	hasMore := len(messages) > limit
	if hasMore {
		messages = messages[:limit]
	}
	for left, right := 0, len(messages)-1; left < right; left, right = left+1, right-1 {
		messages[left], messages[right] = messages[right], messages[left]
	}
	return messages, hasMore, nil
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
	if message.ConversationID != "" && !model.IsSupportEmailNotice(message) {
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

// UpdateMetadata updates only the persisted metadata for a support message.
// Link previews are enriched after message creation so a slow external page
// cannot delay the reply acknowledgement.
func (r *SupportMessageRepository) UpdateMetadata(ctx context.Context, id, metadata string) error {
	return r.updateEnrichedMetadata(ctx, id, metadata)
}

// ListEmailFallbackReconciliationCandidates returns recent outbound replies
// that still need offline email fallback processing. The service layer performs
// the final per-workspace delay, duplicate-log, and presence checks before
// sending.
func (r *SupportMessageRepository) ListEmailFallbackReconciliationCandidates(ctx context.Context, after, before time.Time, limit int) ([]model.SupportMessage, error) {
	if limit < 1 || limit > 1000 {
		limit = 25
	}
	explicitEmail := supportDeliveryModeSQL(r.db, "support_messages") + " IN ('email_only','chat_and_email')"
	var messages []model.SupportMessage
	if err := r.db.WithContext(ctx).
		Model(&model.SupportMessage{}).
		Joins("JOIN support_conversations sc ON sc.id = support_messages.conversation_id AND sc.workspace_id = support_messages.workspace_id").
		Where("support_messages.email_notified_at IS NULL").
		Where(supportDeliveryModeSQL(r.db, "support_messages")+" <> ?", model.SupportDeliveryChatOnly).
		Where("(NOT ("+explicitEmail+") OR "+supportMetadataStringSQL(r.db, "support_messages", "email_delivery_status")+" <> ?)", "blocked").
		Where("support_messages.is_internal = ?", false).
		Where("COALESCE(NULLIF(support_messages.message_type, ''), 'reply') = ?", "reply").
		Where("support_messages.sender_type <> ?", "customer").
		Where("support_messages.created_at <= ?", before).
		Where("(support_messages.created_at >= ? OR ("+explicitEmail+"))", after).
		Where("(support_messages.cancellable_until IS NULL OR support_messages.cancellable_until <= ?)", before).
		Where("((sc.customer_email IS NOT NULL AND TRIM(sc.customer_email) <> '') OR ("+explicitEmail+"))").
		Where("(sc.email_unsubscribed = ? OR ("+explicitEmail+"))", false).
		Where("(LOWER(sc.status) NOT IN ? OR ("+explicitEmail+"))", []string{"closed", "resolved", "spam"}).
		Where("(sc.contact_last_seen_at IS NULL OR support_messages.created_at > sc.contact_last_seen_at OR (" + explicitEmail + "))").
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
	return r.rotateKeys(ctx, id, &widgetKey, secretKey, "")
}

// RotateSecret rotates only the server/signing secret and retains its read alias.
func (r *SupportInboxInstallationRepository) RotateSecret(ctx context.Context, id, secretKey, actorUserID string) error {
	return r.rotateKeys(ctx, id, nil, secretKey, actorUserID)
}

func (r *SupportInboxInstallationRepository) rotateKeys(
	ctx context.Context,
	id string,
	widgetKey *string,
	secretKey, actorUserID string,
) error {
	if err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var installation model.SupportWidgetInstallation
		if err := tx.Where("id = ?", id).First(&installation).Error; err != nil {
			return fmt.Errorf("get widget installation for rotation: %w", err)
		}

		now := time.Now().UTC()
		aliases := []model.WorkspaceEventProjectAlias{{
			WorkspaceID: installation.WorkspaceID,
			ProjectID:   installation.SecretKey,
			Source:      "retired_server_secret",
			ValidFrom:   &installation.CreatedAt,
			ValidTo:     &now,
		}}
		updates := map[string]interface{}{"secret_key": secretKey}
		if widgetKey != nil {
			aliases = append(aliases, model.WorkspaceEventProjectAlias{
				WorkspaceID: installation.WorkspaceID,
				ProjectID:   installation.WidgetKey,
				Source:      "retired_widget_key",
				ValidFrom:   &installation.CreatedAt,
				ValidTo:     &now,
			})
			updates["widget_key"] = *widgetKey
		}
		for i := range aliases {
			if err := tx.Where("project_id = ?", aliases[i].ProjectID).
				FirstOrCreate(&aliases[i]).Error; err != nil {
				return fmt.Errorf("retain event project alias: %w", err)
			}
		}
		if err := tx.Model(&model.SupportWidgetInstallation{}).
			Where("id = ?", id).Updates(updates).Error; err != nil {
			return fmt.Errorf("rotate widget credentials: %w", err)
		}
		if actorUserID != "" {
			audit := model.SupportCredentialRotationAudit{
				WorkspaceID:    installation.WorkspaceID,
				InstallationID: installation.ID,
				ActorUserID:    actorUserID,
				RotationKind:   "server_signing_secret",
			}
			if err := tx.Create(&audit).Error; err != nil {
				return fmt.Errorf("audit widget secret rotation: %w", err)
			}
		}
		return nil
	}); err != nil {
		return err
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
	if session.LastActiveAt == nil {
		now := time.Now().UTC()
		session.LastActiveAt = &now
	}
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
		"crm_company_id":  session.CRMCompanyID,
		"last_active_at":  session.LastActiveAt,
		"revoked_at":      session.RevokedAt,
		"expires_at":      session.ExpiresAt,
	}
	if session.ID != "" {
		values["id"] = session.ID
	}
	if !session.CreatedAt.IsZero() {
		values["created_at"] = session.CreatedAt
	}

	if err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&model.SupportWidgetSession{}).Create(values).Error; err != nil {
			return fmt.Errorf("create widget session: %w", err)
		}
		return projectWidgetSessionCountry(tx, session.WorkspaceID, session.ConversationID, session.AnonymousID)
	}); err != nil {
		return err
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
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Save(session).Error; err != nil {
			return fmt.Errorf("update widget session: %w", err)
		}
		return projectWidgetSessionCountry(tx, session.WorkspaceID, session.ConversationID, session.AnonymousID)
	})
}

func projectWidgetSessionCountry(tx *gorm.DB, workspaceID string, conversationID *string, anonymousID string) error {
	query := tx.Model(&model.SupportConversation{}).Where("workspace_id = ?", workspaceID)
	if conversationID != nil && strings.TrimSpace(*conversationID) != "" {
		query = query.Where("id = ?", strings.TrimSpace(*conversationID))
	} else if strings.TrimSpace(anonymousID) != "" {
		query = query.Where("anonymous_id = ?", strings.TrimSpace(anonymousID))
	} else {
		return nil
	}
	result := query.Updates(map[string]any{
		"visitor_country_code": gorm.Expr(`(
			SELECT country_code FROM support_widget_sessions
			WHERE workspace_id = ? AND (conversation_id = support_conversations.id OR anonymous_id = support_conversations.anonymous_id)
			ORDER BY created_at DESC LIMIT 1
		)`, workspaceID),
		"visitor_country_name": gorm.Expr(`(
			SELECT country_name FROM support_widget_sessions
			WHERE workspace_id = ? AND (conversation_id = support_conversations.id OR anonymous_id = support_conversations.anonymous_id)
			ORDER BY created_at DESC LIMIT 1
		)`, workspaceID),
	})
	if result.Error != nil {
		return fmt.Errorf("project widget session country: %w", result.Error)
	}
	return nil
}

// UpdatePageURL updates the page URL and marks visitor activity.
func (r *SupportInboxSessionRepository) UpdatePageURL(ctx context.Context, sessionToken, url string) error {
	result := r.db.WithContext(ctx).
		Model(&model.SupportWidgetSession{}).
		Where("session_token = ? AND revoked_at IS NULL", sessionToken).
		Updates(map[string]any{
			"last_page_url":  url,
			"last_active_at": time.Now().UTC(),
		})
	if result.Error != nil {
		return fmt.Errorf("update session page url: %w", result.Error)
	}
	return nil
}

// TouchActivityByToken records that a widget visitor was active.
func (r *SupportInboxSessionRepository) TouchActivityByToken(ctx context.Context, sessionToken string) error {
	result := r.db.WithContext(ctx).
		Model(&model.SupportWidgetSession{}).
		Where("session_token = ? AND revoked_at IS NULL", sessionToken).
		Update("last_active_at", time.Now().UTC())
	if result.Error != nil {
		return fmt.Errorf("touch session activity: %w", result.Error)
	}
	return nil
}

// ExtendExpiryByToken advances the expiry for an active, non-revoked widget session.
func (r *SupportInboxSessionRepository) ExtendExpiryByToken(ctx context.Context, sessionToken string, expiresAt time.Time) error {
	result := r.db.WithContext(ctx).
		Model(&model.SupportWidgetSession{}).
		Where("session_token = ? AND revoked_at IS NULL AND expires_at < ?", sessionToken, expiresAt).
		Update("expires_at", expiresAt)
	if result.Error != nil {
		return fmt.Errorf("extend session expiry: %w", result.Error)
	}
	return nil
}

// TouchActivityByAnonymousID records activity across the visitor's current anonymous sessions.
func (r *SupportInboxSessionRepository) TouchActivityByAnonymousID(ctx context.Context, workspaceID, anonymousID string) error {
	if strings.TrimSpace(anonymousID) == "" {
		return nil
	}
	result := r.db.WithContext(ctx).
		Model(&model.SupportWidgetSession{}).
		Where("workspace_id = ? AND anonymous_id = ? AND revoked_at IS NULL", workspaceID, anonymousID).
		Update("last_active_at", time.Now().UTC())
	if result.Error != nil {
		return fmt.Errorf("touch anonymous session activity: %w", result.Error)
	}
	return nil
}

func scanNullableTime(value sql.NullString) (*time.Time, error) {
	if !value.Valid || strings.TrimSpace(value.String) == "" {
		return nil, nil
	}
	raw := strings.TrimSpace(value.String)
	formats := []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02 15:04:05.999999999-07:00",
		"2006-01-02 15:04:05.999999999Z07:00",
		"2006-01-02 15:04:05.999999999",
		"2006-01-02 15:04:05-07:00",
		"2006-01-02 15:04:05Z07:00",
		"2006-01-02 15:04:05",
	}
	for _, format := range formats {
		if parsed, err := time.Parse(format, raw); err == nil {
			utc := parsed.UTC()
			return &utc, nil
		}
	}
	return nil, fmt.Errorf("parse timestamp %q", raw)
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

// GetLatestActivityByAnonymousID returns the latest known visitor activity across all sessions for an anonymous_id.
func (r *SupportInboxSessionRepository) GetLatestActivityByAnonymousID(ctx context.Context, workspaceID, anonymousID string) (*time.Time, error) {
	var raw sql.NullString
	if err := r.db.WithContext(ctx).
		Model(&model.SupportWidgetSession{}).
		Where("workspace_id = ? AND anonymous_id = ? AND last_active_at IS NOT NULL", workspaceID, anonymousID).
		Select("MAX(last_active_at)").
		Scan(&raw).Error; err != nil {
		return nil, fmt.Errorf("get latest activity by anonymous_id: %w", err)
	}
	lastActiveAt, err := scanNullableTime(raw)
	if err != nil {
		return nil, fmt.Errorf("get latest activity by anonymous_id: %w", err)
	}
	return lastActiveAt, nil
}

// GetLatestActivityByContactID returns the latest known widget activity across sessions linked to a CRM contact.
func (r *SupportInboxSessionRepository) GetLatestActivityByContactID(ctx context.Context, workspaceID, contactID string) (*time.Time, error) {
	var raw sql.NullString
	if err := r.db.WithContext(ctx).
		Table("support_widget_sessions sws").
		Where("sws.workspace_id = ? AND sws.last_active_at IS NOT NULL", workspaceID).
		Where(`
			EXISTS (
				SELECT 1
				FROM support_conversations sc
				WHERE sc.workspace_id = sws.workspace_id
				  AND sc.crm_contact_id = ?
				  AND (
					sc.id = sws.conversation_id
					OR (sc.anonymous_id IS NOT NULL AND sc.anonymous_id <> '' AND sc.anonymous_id = sws.anonymous_id)
				  )
			)
		`, contactID).
		Select("MAX(sws.last_active_at)").
		Scan(&raw).Error; err != nil {
		return nil, fmt.Errorf("get latest activity by contact_id: %w", err)
	}
	lastActiveAt, err := scanNullableTime(raw)
	if err != nil {
		return nil, fmt.Errorf("get latest activity by contact_id: %w", err)
	}
	return lastActiveAt, nil
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
	mdHardBreakPattern      = regexp.MustCompile(`\\\r?\n`)
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
	cleaned = mdHardBreakPattern.ReplaceAllString(cleaned, "\n")
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

type ConversationRepositorySearchParams struct {
	model.SupportConversationSearchParams
	WorkspaceMemberID string
	Role              string
}

func (r *SupportConversationRepository) mentionExistsCondition(alias string, userID string) (string, []any) {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return "1 = 0", nil
	}
	if r.db.Dialector.Name() == "sqlite" {
		return fmt.Sprintf(`(EXISTS (
			SELECT 1 FROM support_conversation_user_states mention_state
			WHERE mention_state.conversation_id = %s.id
			  AND mention_state.workspace_id = %s.workspace_id
			  AND mention_state.user_id = ?
			  AND (mention_state.relevance_mask & %d) <> 0
		) OR (%s.support_state_version = 0 AND EXISTS (
			SELECT 1 FROM support_messages mention_message
			WHERE mention_message.conversation_id = %s.id
			  AND mention_message.workspace_id = %s.workspace_id
			  AND mention_message.deleted_at IS NULL
			  AND mention_message.metadata LIKE ?
			  AND mention_message.metadata LIKE ?
		)))`, alias, alias, model.SupportRelevanceMention, alias, alias, alias), []any{
				userID, "%mentioned_user_ids%", "%" + userID + "%",
			}
	}
	filterJSON, _ := json.Marshal(map[string][]string{"mentioned_user_ids": {userID}})
	return fmt.Sprintf(`(EXISTS (
		SELECT 1
		FROM support_conversation_user_states mention_state
		WHERE mention_state.conversation_id = %s.id
		  AND mention_state.workspace_id = %s.workspace_id
		  AND mention_state.user_id = ?
		  AND (mention_state.relevance_mask & %d) <> 0
	) OR (%s.support_state_version = 0 AND EXISTS (
		SELECT 1
		FROM support_messages mention_message
		WHERE mention_message.conversation_id = %s.id
		  AND mention_message.workspace_id = %s.workspace_id
		  AND mention_message.deleted_at IS NULL
		  AND mention_message.metadata::jsonb @> ?::jsonb
	)))`, alias, alias, model.SupportRelevanceMention, alias, alias, alias), []any{
			userID,
			string(filterJSON),
		}
}

func (r *SupportConversationRepository) conversationListProjectionSelect() string {
	legacyUnreadCondition := "support_conversations.support_state_version = 0"
	if r.db.Dialector.Name() == "postgres" {
		legacyUnreadCondition += ` OR NOT EXISTS (
			SELECT 1 FROM support_inbox_state_rollouts rollout
			WHERE rollout.workspace_id = support_conversations.workspace_id AND rollout.mode = 'v2'
		)`
	}
	projection := `support_conversations.*, list_activity.created_at AS list_last_activity_at,
		CASE WHEN support_conversations.support_state_version = 0 THEN (
			SELECT CASE WHEN fallback_message.is_internal
				THEN 'Note: ' || fallback_message.content ELSE fallback_message.content END
			FROM support_messages fallback_message
			WHERE fallback_message.conversation_id = support_conversations.id
			  AND fallback_message.deleted_at IS NULL
			  AND fallback_message.system_event_type IS NULL
			  AND fallback_message.message_type = 'reply'
			  AND (NOT fallback_message.is_internal OR TRIM(fallback_message.content) <> '')
			ORDER BY fallback_message.created_at DESC, fallback_message.id DESC LIMIT 1
		) ELSE CASE WHEN support_conversations.list_last_message_is_internal
			THEN 'Note: ' || COALESCE(support_conversations.list_last_message_preview, '')
			ELSE support_conversations.list_last_message_preview END END AS last_message,
		CASE WHEN support_conversations.support_state_version = 0 THEN (
			SELECT fallback_public.sender_type FROM support_messages fallback_public
			WHERE fallback_public.conversation_id = support_conversations.id
			  AND fallback_public.deleted_at IS NULL AND fallback_public.system_event_type IS NULL
			  AND fallback_public.message_type = 'reply' AND fallback_public.is_internal = false
			ORDER BY fallback_public.created_at DESC, fallback_public.id DESC LIMIT 1
		) ELSE support_conversations.last_public_sender_type END AS last_message_sender_type,
		CASE WHEN support_conversations.support_state_version = 0 THEN (
			SELECT fallback_public.sender_display_name FROM support_messages fallback_public
			WHERE fallback_public.conversation_id = support_conversations.id
			  AND fallback_public.deleted_at IS NULL AND fallback_public.system_event_type IS NULL
			  AND fallback_public.message_type = 'reply' AND fallback_public.is_internal = false
			ORDER BY fallback_public.created_at DESC, fallback_public.id DESC LIMIT 1
		) ELSE support_conversations.last_public_sender_display_name END AS last_message_sender_display_name,
		CASE
			WHEN __LEGACY_UNREAD_CONDITION__ THEN (
				SELECT COUNT(*) FROM support_messages fallback_unread
				WHERE fallback_unread.conversation_id = support_conversations.id
				  AND fallback_unread.deleted_at IS NULL AND fallback_unread.system_event_type IS NULL
				  AND fallback_unread.is_internal = false AND fallback_unread.sender_type = 'customer'
				  AND fallback_unread.message_type = 'reply'
				  AND fallback_unread.created_at > COALESCE(support_conversations.team_last_seen_at, '1970-01-01 00:00:00')
			)
			WHEN COALESCE(scus.unread_customer_message_count, 0) > 0 THEN scus.unread_customer_message_count
			WHEN COALESCE(scus.manually_unread, false) THEN 1
			ELSE 0
		END AS unread_count,
		CASE WHEN __LEGACY_UNREAD_CONDITION__ THEN 0 ELSE COALESCE(scus.version, 0) END AS personal_state_version,
		CASE WHEN support_conversations.support_state_version = 0 THEN COALESCE((
			SELECT fallback_public.sender_type = 'customer' FROM support_messages fallback_public
			WHERE fallback_public.conversation_id = support_conversations.id
			  AND fallback_public.deleted_at IS NULL AND fallback_public.system_event_type IS NULL
			  AND fallback_public.message_type = 'reply' AND fallback_public.is_internal = false
			ORDER BY fallback_public.created_at DESC, fallback_public.id DESC LIMIT 1
		), false) ELSE support_conversations.customer_awaiting_response END AS awaiting_reply,
		CASE WHEN support_conversations.support_state_version = 0 THEN (
			SELECT fallback_session.country_code FROM support_widget_sessions fallback_session
			WHERE fallback_session.workspace_id = support_conversations.workspace_id
			  AND (fallback_session.conversation_id = support_conversations.id
				OR fallback_session.anonymous_id = support_conversations.anonymous_id)
			ORDER BY fallback_session.created_at DESC LIMIT 1
		) ELSE support_conversations.visitor_country_code END AS country_code,
		CASE WHEN support_conversations.support_state_version = 0 THEN (
			SELECT fallback_session.country_name FROM support_widget_sessions fallback_session
			WHERE fallback_session.workspace_id = support_conversations.workspace_id
			  AND (fallback_session.conversation_id = support_conversations.id
				OR fallback_session.anonymous_id = support_conversations.anonymous_id)
			ORDER BY fallback_session.created_at DESC LIMIT 1
		) ELSE support_conversations.visitor_country_name END AS country_name,
		sm.name AS mailbox_name,
		sm.handle AS mailbox_handle,
		sm.icon AS mailbox_icon`
	return strings.ReplaceAll(projection, "__LEGACY_UNREAD_CONDITION__", legacyUnreadCondition)
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

// Waiting is a view of public teammate replies, not a status transition.
// Retain explicitly waiting conversations for compatibility with existing workflows.
func conversationWaitingOnCustomerCondition(alias string) string {
	return fmt.Sprintf("(%[1]s.status = 'waiting_on_customer' OR (%[1]s.status = 'open' AND %[1]s.last_public_sender_type = 'user' AND NOT COALESCE(%[1]s.customer_awaiting_response, false)))", alias)
}

func (r *SupportConversationRepository) applyConversationListFilter(query *gorm.DB, alias, filter, userID string) *gorm.DB {
	switch strings.TrimSpace(strings.ToLower(filter)) {
	case model.SupportConversationListFilterWaiting:
		return query.Where(conversationWaitingOnCustomerCondition(alias))
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
		case "none":
			conditions = append(conditions, fmt.Sprintf("NOT (%s) AND NOT (%s) AND NOT (%s)",
				conversationAIActiveCondition(alias),
				conversationAIHandoffCondition(alias),
				conversationResolvedByAICondition(alias),
			))
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

// Reuse the indexed timeline rather than treating unrelated record updates as activity.
// This read-only projection includes internal notes and system events and does not
// modify message timestamps, unread state, or response tracking.
const conversationActivityJoin = `LEFT JOIN support_messages list_activity ON list_activity.id = (
    SELECT activity.id FROM support_messages activity
    WHERE activity.conversation_id = support_conversations.id
      AND activity.workspace_id = support_conversations.workspace_id
      AND activity.deleted_at IS NULL
    ORDER BY activity.created_at DESC, activity.id DESC LIMIT 1
)`

func conversationListOrder(sortOrder string) string {
	if strings.EqualFold(strings.TrimSpace(sortOrder), "oldest") {
		return "COALESCE(list_activity.created_at, support_conversations.created_at) ASC"
	}
	return "COALESCE(list_activity.created_at, support_conversations.created_at) DESC"
}

const supportConversationSearchTotalCap = 1000

var uuidLikePattern = regexp.MustCompile(`(?i)^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

func normalizeSupportSearchQuery(query string) string {
	return strings.TrimSpace(query)
}

func supportSearchLikePattern(query string) string {
	return "%" + escapeLike(strings.ToLower(query)) + "%"
}

func supportSearchDisplayID(query string) (int, bool) {
	trimmed := strings.TrimPrefix(strings.TrimSpace(query), "#")
	if trimmed == "" {
		return 0, false
	}
	value, err := strconv.Atoi(trimmed)
	if err != nil {
		return 0, false
	}
	return value, true
}

func (r *SupportConversationRepository) supportSearchUsesFTS() bool {
	return r.db != nil && r.db.Dialector != nil && r.db.Dialector.Name() == "postgres"
}

func supportSearchTerms(query string) []string {
	query = strings.Trim(strings.TrimSpace(query), `"`)
	fields := strings.FieldsFunc(strings.ToLower(query), func(r rune) bool {
		return !(r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '@' || r == '.' || r == '_' || r == '-')
	})
	terms := make([]string, 0, len(fields))
	seen := map[string]bool{}
	for _, field := range fields {
		field = strings.TrimSpace(field)
		if field == "" || seen[field] {
			continue
		}
		seen[field] = true
		terms = append(terms, field)
	}
	return terms
}

func supportSearchHighlight(field, text, query string) *model.SupportSearchHighlight {
	if strings.TrimSpace(text) == "" || strings.TrimSpace(query) == "" {
		return nil
	}
	lowerText := strings.ToLower(text)
	ranges := []model.SupportSearchHighlightRange{}
	for _, term := range supportSearchTerms(query) {
		start := strings.Index(lowerText, term)
		if start < 0 {
			continue
		}
		ranges = append(ranges, model.SupportSearchHighlightRange{Start: start, End: start + len(term)})
	}
	if len(ranges) == 0 {
		return nil
	}
	sort.Slice(ranges, func(i, j int) bool { return ranges[i].Start < ranges[j].Start })
	return &model.SupportSearchHighlight{Field: field, Text: text, Ranges: ranges}
}

func addMatchedField(fields []string, field string) []string {
	for _, existing := range fields {
		if existing == field {
			return fields
		}
	}
	return append(fields, field)
}

func (r *SupportConversationRepository) applySupportSearchFilters(query *gorm.DB, alias string, params ConversationRepositorySearchParams) *gorm.DB {
	query = r.applyMailboxAccess(query, alias, params.WorkspaceMemberID, params.Role)
	query = r.applyMailboxScopes(query, alias, nil, params.MailboxIDs)
	query = r.applyConversationAssignmentFilter(query, alias, strings.Join(params.AssignedTo, ","), params.UserID)
	query = applyConversationTagFilters(query, alias, params.TagIDs, nil)
	query = applyConversationAIFilters(query, alias, params.AI)
	if len(params.Statuses) > 0 {
		query = query.Where(fmt.Sprintf("%s.status IN ?", alias), params.Statuses)
	}
	if len(params.Priorities) > 0 {
		query = query.Where(fmt.Sprintf("%s.priority IN ?", alias), params.Priorities)
	}
	if params.CreatedFrom != nil {
		query = query.Where(fmt.Sprintf("%s.created_at >= ?", alias), *params.CreatedFrom)
	}
	if params.CreatedTo != nil {
		query = query.Where(fmt.Sprintf("%s.created_at <= ?", alias), *params.CreatedTo)
	}
	if strings.TrimSpace(params.CustomerEmail) != "" {
		pattern := supportSearchLikePattern(params.CustomerEmail)
		query = query.Where(fmt.Sprintf("LOWER(COALESCE(%s.customer_email, '')) LIKE ? ESCAPE '\\'", alias), pattern)
	}
	if strings.TrimSpace(params.Title) != "" {
		pattern := supportSearchLikePattern(params.Title)
		query = query.Where(fmt.Sprintf("LOWER(%s.subject) LIKE ? ESCAPE '\\'", alias), pattern)
	}
	search := normalizeSupportSearchQuery(params.Query)
	if search == "" {
		return query
	}
	pattern := supportSearchLikePattern(search)
	conditions := []string{}
	args := []any{}
	if r.supportSearchUsesFTS() {
		conditions = append(conditions,
			fmt.Sprintf("%s.search_vector @@ websearch_to_tsquery('simple', ?)", alias),
			fmt.Sprintf("%s.subject ILIKE ? ESCAPE '\\'", alias),
			fmt.Sprintf("COALESCE(%s.customer_name, '') ILIKE ? ESCAPE '\\'", alias),
			fmt.Sprintf("COALESCE(%s.customer_email, '') ILIKE ? ESCAPE '\\'", alias),
			fmt.Sprintf(`EXISTS (
			SELECT 1
			FROM support_messages sm_search
			WHERE sm_search.workspace_id = %s.workspace_id
			  AND sm_search.conversation_id = %s.id
			  AND sm_search.deleted_at IS NULL
			  AND sm_search.message_type IN ('reply', 'email_notice')
			  AND sm_search.system_event_type IS NULL
			  AND sm_search.search_vector @@ websearch_to_tsquery('simple', ?)
		)`, alias, alias),
		)
		args = append(args, search, pattern, pattern, pattern, search)
	} else {
		conditions = append(conditions,
			fmt.Sprintf("LOWER(%s.subject) LIKE ? ESCAPE '\\'", alias),
			fmt.Sprintf("LOWER(COALESCE(%s.customer_name, '')) LIKE ? ESCAPE '\\'", alias),
			fmt.Sprintf("LOWER(COALESCE(%s.customer_email, '')) LIKE ? ESCAPE '\\'", alias),
			fmt.Sprintf(`EXISTS (
			SELECT 1
			FROM support_messages sm_search
			WHERE sm_search.workspace_id = %s.workspace_id
			  AND sm_search.conversation_id = %s.id
			  AND sm_search.deleted_at IS NULL
			  AND sm_search.message_type IN ('reply', 'email_notice')
			  AND sm_search.system_event_type IS NULL
			  AND LOWER(COALESCE(sm_search.content, '')) LIKE ? ESCAPE '\'
		)`, alias, alias),
		)
		args = append(args, pattern, pattern, pattern, pattern)
	}
	if displayID, ok := supportSearchDisplayID(search); ok {
		conditions = append(conditions, fmt.Sprintf("%s.display_id = ?", alias))
		args = append(args, displayID)
	}
	if uuidLikePattern.MatchString(search) {
		conditions = append(conditions, fmt.Sprintf("%s.id = ?", alias))
		args = append(args, search)
	}
	return query.Where("("+strings.Join(conditions, " OR ")+")", args...)
}

func supportSearchOrder(sortOrder, query string) string {
	order := conversationListOrder(sortOrder)
	switch strings.ToLower(strings.TrimSpace(sortOrder)) {
	case "oldest", "newest":
		return order
	default:
		if strings.TrimSpace(query) == "" {
			return order
		}
		return "support_search_score DESC, " + order
	}
}

func supportSearchScoreSQL(query string) (string, []any) {
	query = normalizeSupportSearchQuery(query)
	if query == "" {
		return "0", nil
	}
	pattern := supportSearchLikePattern(query)
	score := []string{
		"CASE WHEN LOWER(COALESCE(support_conversations.customer_email, '')) = LOWER(?) THEN 100 ELSE 0 END",
		"CASE WHEN LOWER(support_conversations.subject) LIKE ? ESCAPE '\\' THEN 20 ELSE 0 END",
		"CASE WHEN LOWER(COALESCE(support_conversations.customer_email, '')) LIKE ? ESCAPE '\\' THEN 18 ELSE 0 END",
		"CASE WHEN LOWER(COALESCE(support_conversations.customer_name, '')) LIKE ? ESCAPE '\\' THEN 10 ELSE 0 END",
	}
	args := []any{query, pattern, pattern, pattern}
	if displayID, ok := supportSearchDisplayID(query); ok {
		score = append(score, "CASE WHEN support_conversations.display_id = ? THEN 120 ELSE 0 END")
		args = append(args, displayID)
	}
	if uuidLikePattern.MatchString(query) {
		score = append(score, "CASE WHEN support_conversations.id = ? THEN 120 ELSE 0 END")
		args = append(args, query)
	}
	score = append(score, `CASE WHEN EXISTS (
		SELECT 1
		FROM support_messages sm_score
		WHERE sm_score.workspace_id = support_conversations.workspace_id
		  AND sm_score.conversation_id = support_conversations.id
		  AND sm_score.deleted_at IS NULL
		  AND sm_score.message_type IN ('reply', 'email_notice')
		  AND sm_score.system_event_type IS NULL
		  AND LOWER(COALESCE(sm_score.content, '')) LIKE ? ESCAPE '\'
	) THEN 5 ELSE 0 END`)
	args = append(args, pattern)
	return strings.Join(score, " + "), args
}

func supportSearchPostgresScoreSQL(query string) (string, []any) {
	query = normalizeSupportSearchQuery(query)
	if query == "" {
		return "0", nil
	}
	pattern := supportSearchLikePattern(query)
	score := []string{
		"CASE WHEN LOWER(COALESCE(support_conversations.customer_email, '')) = LOWER(?) THEN 100 ELSE 0 END",
		"CASE WHEN support_conversations.search_vector @@ websearch_to_tsquery('simple', ?) THEN ts_rank_cd(support_conversations.search_vector, websearch_to_tsquery('simple', ?)) * 40 ELSE 0 END",
		"CASE WHEN support_conversations.subject ILIKE ? ESCAPE '\\' THEN 20 ELSE 0 END",
		"CASE WHEN COALESCE(support_conversations.customer_email, '') ILIKE ? ESCAPE '\\' THEN 18 ELSE 0 END",
		"CASE WHEN COALESCE(support_conversations.customer_name, '') ILIKE ? ESCAPE '\\' THEN 10 ELSE 0 END",
	}
	args := []any{query, query, query, pattern, pattern, pattern}
	if displayID, ok := supportSearchDisplayID(query); ok {
		score = append(score, "CASE WHEN support_conversations.display_id = ? THEN 120 ELSE 0 END")
		args = append(args, displayID)
	}
	if uuidLikePattern.MatchString(query) {
		score = append(score, "CASE WHEN support_conversations.id = ? THEN 120 ELSE 0 END")
		args = append(args, query)
	}
	score = append(score, `CASE WHEN EXISTS (
		SELECT 1
		FROM support_messages sm_score
		WHERE sm_score.workspace_id = support_conversations.workspace_id
		  AND sm_score.conversation_id = support_conversations.id
		  AND sm_score.deleted_at IS NULL
		  AND sm_score.message_type IN ('reply', 'email_notice')
		  AND sm_score.system_event_type IS NULL
		  AND sm_score.search_vector @@ websearch_to_tsquery('simple', ?)
	) THEN 5 ELSE 0 END`)
	args = append(args, query)
	return strings.Join(score, " + "), args
}

func (r *SupportConversationRepository) searchSnippet(ctx context.Context, workspaceID, conversationID, query string, conversation model.SupportConversation) (string, []string, []model.SupportSearchHighlight) {
	matchedFields := []string{}
	highlights := []model.SupportSearchHighlight{}
	query = normalizeSupportSearchQuery(query)
	if query == "" {
		if conversation.Subject != "" {
			return conversation.Subject, matchedFields, highlights
		}
		return "", matchedFields, highlights
	}
	pattern := supportSearchLikePattern(query)
	if highlight := supportSearchHighlight("title", conversation.Subject, query); highlight != nil {
		matchedFields = addMatchedField(matchedFields, "title")
		highlights = append(highlights, *highlight)
		return conversation.Subject, matchedFields, highlights
	}
	if conversation.CustomerEmail != nil {
		if highlight := supportSearchHighlight("customer_email", *conversation.CustomerEmail, query); highlight != nil {
			matchedFields = addMatchedField(matchedFields, "customer_email")
			highlights = append(highlights, *highlight)
			return *conversation.CustomerEmail, matchedFields, highlights
		}
	}
	if conversation.CustomerName != nil {
		if highlight := supportSearchHighlight("customer_name", *conversation.CustomerName, query); highlight != nil {
			matchedFields = addMatchedField(matchedFields, "customer_name")
			highlights = append(highlights, *highlight)
			return *conversation.CustomerName, matchedFields, highlights
		}
	}
	if displayID, ok := supportSearchDisplayID(query); ok && displayID == conversation.DisplayID {
		return fmt.Sprintf("#%d", conversation.DisplayID), []string{"display_id"}, highlights
	}

	var message model.SupportMessage
	err := r.db.WithContext(ctx).
		Where(`workspace_id = ? AND conversation_id = ? AND deleted_at IS NULL AND message_type IN ('reply', 'email_notice') AND system_event_type IS NULL AND LOWER(COALESCE(content, '')) LIKE ? ESCAPE '\'`,
			workspaceID, conversationID, pattern).
		Order("created_at DESC").
		First(&message).Error
	if err == nil {
		snippet := cleanMessageSnippet(message.Content, 180)
		matchedFields = addMatchedField(matchedFields, "message")
		if highlight := supportSearchHighlight("message", snippet, query); highlight != nil {
			highlights = append(highlights, *highlight)
		}
		return snippet, matchedFields, highlights
	}
	return conversation.Subject, matchedFields, highlights
}

func (r *SupportConversationRepository) Search(ctx context.Context, params ConversationRepositorySearchParams) (*model.SupportConversationSearchResponse, error) {
	page := params.Pagination.Page
	if page <= 0 {
		page = 1
	}
	perPage := params.Pagination.PerPage
	if perPage <= 0 || perPage > 50 {
		perPage = 50
	}
	sortOrder := strings.ToLower(strings.TrimSpace(params.Sort))
	if sortOrder == "" {
		if strings.TrimSpace(params.Query) == "" {
			sortOrder = "newest"
		} else {
			sortOrder = "relevance"
		}
	}

	base := r.db.WithContext(ctx).
		Table("support_conversations").
		Where("support_conversations.workspace_id = ?", params.WorkspaceID)
	base = r.applySupportSearchFilters(base, "support_conversations", params)

	var cappedIDs []string
	if err := base.Session(&gorm.Session{}).
		Select("support_conversations.id").
		Limit(supportConversationSearchTotalCap+1).
		Pluck("support_conversations.id", &cappedIDs).Error; err != nil {
		return nil, fmt.Errorf("count search conversations: %w", err)
	}
	totalCapped := len(cappedIDs) > supportConversationSearchTotalCap
	total := len(cappedIDs)
	if totalCapped {
		total = supportConversationSearchTotalCap
	}
	totalPages := 0
	if total > 0 {
		totalPages = (total + perPage - 1) / perPage
	}

	scoreSQL, scoreArgs := supportSearchScoreSQL(params.Query)
	if r.supportSearchUsesFTS() {
		scoreSQL, scoreArgs = supportSearchPostgresScoreSQL(params.Query)
	}
	fetch := r.db.WithContext(ctx).
		Table("support_conversations").
		Where("support_conversations.workspace_id = ?", params.WorkspaceID)
	fetch = r.applySupportSearchFilters(fetch, "support_conversations", params)

	var rows []struct {
		model.SupportConversation
		SupportSearchScore float64 `gorm:"column:support_search_score"`
	}
	selectSQL := fmt.Sprintf(`support_conversations.*, list_activity.created_at AS list_last_activity_at, sm.name AS mailbox_name, sm.handle AS mailbox_handle, sm.icon AS mailbox_icon, (%s) AS support_search_score`, scoreSQL)
	offset := (page - 1) * perPage
	if params.Pagination.Offset != nil {
		offset = *params.Pagination.Offset
	}
	if err := fetch.
		Joins("LEFT JOIN support_mailboxes sm ON sm.id = support_conversations.mailbox_id").
		Joins(conversationActivityJoin).
		Select(selectSQL, scoreArgs...).
		Order(supportSearchOrder(sortOrder, params.Query)).
		Offset(offset).
		Limit(perPage).
		Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("search conversations: %w", err)
	}

	results := make([]model.SupportConversationSearchResult, 0, len(rows))
	for _, row := range rows {
		conversation := row.SupportConversation
		snippet, matchedFields, highlights := r.searchSnippet(ctx, params.WorkspaceID, conversation.ID, params.Query, conversation)
		results = append(results, model.SupportConversationSearchResult{
			Conversation:  conversation,
			DisplayID:     conversation.DisplayID,
			MatchedFields: matchedFields,
			Snippet:       snippet,
			Highlights:    highlights,
			Score:         row.SupportSearchScore,
		})
	}

	return &model.SupportConversationSearchResponse{
		Data:       results,
		Total:      total,
		Page:       page,
		PerPage:    perPage,
		TotalPages: totalPages,
		Meta: model.SupportConversationSearchMeta{
			Sort:        sortOrder,
			Query:       strings.TrimSpace(params.Query),
			TotalCapped: totalCapped,
			TotalCap:    supportConversationSearchTotalCap,
		},
	}, nil
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
	if params.Pagination.Offset != nil {
		offset = *params.Pagination.Offset
	}

	// Fresh query for fetch — Count() taints the SELECT clause
	fetch := r.db.WithContext(ctx).Table("support_conversations").Where("support_conversations.workspace_id = ?", params.WorkspaceID)
	fetch = r.applyConversationListParams(fetch, "support_conversations", params)

	var conversations []model.SupportConversation
	if err := fetch.
		Joins("LEFT JOIN support_mailboxes sm ON sm.id = support_conversations.mailbox_id").
		Joins(conversationActivityJoin).
		Joins("LEFT JOIN support_conversation_user_states scus ON scus.conversation_id = support_conversations.id AND scus.user_id = ?", strings.TrimSpace(params.UserID)).
		Select(r.conversationListProjectionSelect()).
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
	total, unread, _, err := r.CountByParamsWithAttention(ctx, params)
	return total, unread, err
}

// CountByParamsWithAttention evaluates one saved-view predicate once and
// derives all count domains from that same result set.
func (r *SupportConversationRepository) CountByParamsWithAttention(ctx context.Context, params ConversationRepositoryListParams) (int, int, int, error) {
	buildQuery := func() *gorm.DB {
		query := r.db.WithContext(ctx).
			Table("support_conversations AS sc").
			Where("sc.workspace_id = ?", params.WorkspaceID)
		return r.applyConversationListParams(query, "sc", params)
	}

	unreadCondition := `EXISTS (
		SELECT 1
		FROM support_conversation_user_states personal_state
		WHERE personal_state.conversation_id = sc.id
		  AND personal_state.workspace_id = sc.workspace_id
		  AND personal_state.user_id = ?
		  AND (personal_state.unread_customer_message_count > 0 OR personal_state.manually_unread)
	)`
	if r.db.Dialector.Name() == "postgres" {
		unreadCondition = `(
			(EXISTS (SELECT 1 FROM support_inbox_state_rollouts rollout
				WHERE rollout.workspace_id = sc.workspace_id AND rollout.mode = 'v2')
			 AND ` + unreadCondition + `)
			OR
			(NOT EXISTS (SELECT 1 FROM support_inbox_state_rollouts rollout
				WHERE rollout.workspace_id = sc.workspace_id AND rollout.mode = 'v2')
			 AND EXISTS (
				SELECT 1 FROM support_messages legacy_message
				WHERE legacy_message.conversation_id = sc.id
				  AND legacy_message.deleted_at IS NULL
				  AND legacy_message.system_event_type IS NULL
				  AND legacy_message.is_internal = FALSE
				  AND legacy_message.sender_type = 'customer'
				  AND legacy_message.message_type = 'reply'
				  AND legacy_message.created_at > COALESCE(sc.team_last_seen_at, '1970-01-01 00:00:00')
			))
		)`
	}

	type aggregate struct {
		TotalCount           int `gorm:"column:total_count"`
		UnreadCount          int `gorm:"column:unread_count"`
		NeedsHumanReplyCount int `gorm:"column:needs_human_reply_count"`
	}
	var counts aggregate
	if err := buildQuery().Select(fmt.Sprintf(`
		COUNT(*) AS total_count,
		COALESCE(SUM(CASE WHEN %s THEN 1 ELSE 0 END), 0) AS unread_count,
		COALESCE(SUM(CASE WHEN sc.needs_human_reply THEN 1 ELSE 0 END), 0) AS needs_human_reply_count
	`, unreadCondition), strings.TrimSpace(params.UserID)).Scan(&counts).Error; err != nil {
		return 0, 0, 0, fmt.Errorf("count conversations: %w", err)
	}

	return counts.TotalCount, counts.UnreadCount, counts.NeedsHumanReplyCount, nil
}

const CoverageAnalysisCandidatePageMax = 200

type CoverageAnalysisCandidateCursor struct {
	UpdatedAt time.Time `json:"updated_at"`
	ID        string    `json:"id"`
}

func (r *SupportConversationRepository) ListCoverageAnalysisCandidates(ctx context.Context, workspaceID string, windowStart, windowEnd time.Time, limit int) ([]model.SupportConversation, error) {
	conversations, _, err := r.ListCoverageAnalysisCandidatesPage(ctx, workspaceID, windowStart, windowEnd, nil, limit)
	return conversations, err
}

func (r *SupportConversationRepository) ListCoverageAnalysisCandidatesPage(ctx context.Context, workspaceID string, windowStart, windowEnd time.Time, cursor *CoverageAnalysisCandidateCursor, limit int) ([]model.SupportConversation, *CoverageAnalysisCandidateCursor, error) {
	if workspaceID == "" {
		return nil, nil, fmt.Errorf("workspace_id is required")
	}
	if limit <= 0 {
		limit = CoverageAnalysisCandidatePageMax
	}
	if limit > CoverageAnalysisCandidatePageMax {
		return nil, nil, fmt.Errorf("coverage candidate page limit %d exceeds maximum %d", limit, CoverageAnalysisCandidatePageMax)
	}

	var conversations []model.SupportConversation
	query := r.db.WithContext(ctx).
		Where("workspace_id = ?", workspaceID).
		Where("status <> ?", model.SupportConversationStatusSpam).
		Where(`(
			(updated_at >= ? AND updated_at < ?)
			OR (resolved_at IS NOT NULL AND resolved_at >= ? AND resolved_at < ?)
		)`, windowStart, windowEnd, windowStart, windowEnd)
	if cursor != nil {
		query = query.Where("(updated_at > ?) OR (updated_at = ? AND id > ?)", cursor.UpdatedAt, cursor.UpdatedAt, cursor.ID)
	}
	err := query.
		Order("updated_at ASC, id ASC").
		Limit(limit).
		Find(&conversations).Error
	if err != nil {
		return nil, nil, fmt.Errorf("list coverage analysis candidates: %w", err)
	}
	if len(conversations) == 0 || len(conversations) < limit {
		return conversations, nil, nil
	}
	last := conversations[len(conversations)-1]
	return conversations, &CoverageAnalysisCandidateCursor{UpdatedAt: last.UpdatedAt, ID: last.ID}, nil
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

// ListActiveByCustomerEmail returns active conversations for a customer email
// within a workspace and optional mailbox, most-recently-updated first.
func (r *SupportConversationRepository) ListActiveByCustomerEmail(ctx context.Context, workspaceID, customerEmail string, mailboxID *string, since time.Time, limit int) ([]model.SupportConversation, error) {
	workspaceID = strings.TrimSpace(workspaceID)
	customerEmail = strings.TrimSpace(customerEmail)
	if workspaceID == "" || customerEmail == "" {
		return nil, nil
	}
	if limit <= 0 || limit > 20 {
		limit = 2
	}

	query := r.db.WithContext(ctx).
		Where("workspace_id = ?", workspaceID).
		Where("LOWER(customer_email) = LOWER(?)", customerEmail).
		Where("status IN ?", []string{
			model.SupportConversationStatusOpen,
			model.SupportConversationStatusWaitingOnCustomer,
		}).
		Where("updated_at >= ?", since).
		Order("updated_at DESC").
		Limit(limit)
	if mailboxID != nil && strings.TrimSpace(*mailboxID) != "" {
		query = query.Where("mailbox_id = ?", strings.TrimSpace(*mailboxID))
	}

	var conversations []model.SupportConversation
	if err := query.Find(&conversations).Error; err != nil {
		return nil, fmt.Errorf("list active conversations by customer email: %w", err)
	}
	return conversations, nil
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
		Joins(conversationActivityJoin).
		Where("support_conversations.workspace_id = ? AND support_conversations.id = ?", workspaceID, id)
	query = r.applyMailboxAccess(query, "support_conversations", workspaceMemberID, role)
	if err := query.Select(fmt.Sprintf(`support_conversations.*, list_activity.created_at AS list_last_activity_at,
		(SELECT COUNT(*)
		 FROM support_messages unread_messages
		 WHERE unread_messages.conversation_id = support_conversations.id
		   AND unread_messages.deleted_at IS NULL
		   AND unread_messages.is_internal = false
		   AND unread_messages.sender_type = 'customer'
		   AND unread_messages.message_type = 'reply'
		   AND unread_messages.system_event_type IS NULL
		   AND unread_messages.created_at > COALESCE(support_conversations.team_last_seen_at, %s)
		) AS unread_count,
		%s AS country_code, %s AS country_name,
		sm.name AS mailbox_name, sm.handle AS mailbox_handle, sm.icon AS mailbox_icon`,
		r.epochExpr(),
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

// GetByIDForUser returns the same authorized detail with the requesting
// agent's personal read state and materialized conversation projections.
func (r *SupportConversationRepository) GetByIDForUser(ctx context.Context, workspaceID, id, userID, workspaceMemberID, role string) (*model.SupportConversation, error) {
	var conversation model.SupportConversation
	query := r.db.WithContext(ctx).
		Table("support_conversations").
		Joins("LEFT JOIN support_mailboxes sm ON sm.id = support_conversations.mailbox_id").
		Joins(conversationActivityJoin).
		Joins("LEFT JOIN support_conversation_user_states scus ON scus.conversation_id = support_conversations.id AND scus.user_id = ?", strings.TrimSpace(userID)).
		Where("support_conversations.workspace_id = ? AND support_conversations.id = ?", workspaceID, id)
	query = r.applyMailboxAccess(query, "support_conversations", workspaceMemberID, role)
	if err := query.Select(r.conversationListProjectionSelect()).First(&conversation).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("get conversation for user: %w", err)
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
	if conversation.Status == "resolved" || conversation.Status == "closed" || conversation.Status == "spam" {
		conversation.DelayedTeamReplySentFor = conversation.AIEscalatedAt
	}
	if err := r.db.WithContext(ctx).Save(conversation).Error; err != nil {
		return fmt.Errorf("update conversation: %w", err)
	}
	return nil
}

// UpdateFields updates specific fields on a conversation by ID and workspace.
func (r *SupportConversationRepository) UpdateFields(ctx context.Context, workspaceID, conversationID string, fields map[string]any) error {
	if status, ok := fields["status"].(string); ok && (status == "resolved" || status == "closed" || status == "spam") {
		fields = maps.Clone(fields)
		fields["delayed_team_reply_sent_for"] = gorm.Expr("ai_escalated_at")
	}

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
		Joins(conversationActivityJoin).
		Where("support_conversations.workspace_id = ? AND support_conversations.id IN ?", workspaceID, ids)
	query = r.applyMailboxAccess(query, "support_conversations", workspaceMemberID, role)
	if err := query.
		Select(fmt.Sprintf("support_conversations.*, list_activity.created_at AS list_last_activity_at, %s AS country_code, %s AS country_name, sm.name AS mailbox_name, sm.handle AS mailbox_handle, sm.icon AS mailbox_icon",
			r.latestSessionCountryExpr("country_code", "support_conversations"),
			r.latestSessionCountryExpr("country_name", "support_conversations"),
		)).
		Order("support_conversations.updated_at DESC").
		Find(&conversations).Error; err != nil {
		return nil, fmt.Errorf("list conversations by ids: %w", err)
	}
	return conversations, nil
}

func (r *SupportConversationRepository) ListTitlesByIDs(ctx context.Context, workspaceID string, ids []string) ([]model.SupportConversation, error) {
	if len(ids) == 0 {
		return []model.SupportConversation{}, nil
	}
	var conversations []model.SupportConversation
	if err := r.db.WithContext(ctx).
		Select("id", "subject").
		Where("workspace_id = ? AND id IN ?", workspaceID, ids).
		Find(&conversations).Error; err != nil {
		return nil, fmt.Errorf("list support conversation titles by ids: %w", err)
	}
	return conversations, nil
}

// ListByLinkedTaskIDs returns conversations linked to any of the provided tasks.
func (r *SupportConversationRepository) ListByLinkedTaskIDs(ctx context.Context, workspaceID string, taskIDs []string) ([]model.SupportConversation, error) {
	if len(taskIDs) == 0 {
		return []model.SupportConversation{}, nil
	}

	var conversations []model.SupportConversation
	if err := r.db.WithContext(ctx).
		Where("workspace_id = ? AND linked_task_id IN ?", workspaceID, taskIDs).
		Order("updated_at DESC").
		Find(&conversations).Error; err != nil {
		return nil, fmt.Errorf("list conversations by linked tasks: %w", err)
	}
	return conversations, nil
}

// ListByContact returns conversations linked to a CRM contact.
func (r *SupportConversationRepository) ListByContact(ctx context.Context, workspaceID, contactID, status, search string, pagination model.PMPagination) ([]model.SupportConversation, int64, error) {
	query := r.db.WithContext(ctx).Model(&model.SupportConversation{}).
		Where("workspace_id = ? AND crm_contact_id = ?", workspaceID, contactID)
	if trimmed := strings.TrimSpace(status); trimmed != "" && trimmed != "all" {
		query = query.Where("status = ?", trimmed)
	}
	if trimmed := strings.TrimSpace(search); trimmed != "" {
		query = query.Where("LOWER(subject) LIKE ?", "%"+strings.ToLower(trimmed)+"%")
	}

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

// ListByCompany returns conversations linked directly to a company or to one of its contacts.
func (r *SupportConversationRepository) ListByCompany(ctx context.Context, workspaceID, companyID, status, search string, pagination model.PMPagination) ([]model.SupportConversation, int64, error) {
	query := r.db.WithContext(ctx).Model(&model.SupportConversation{}).
		Where("support_conversations.workspace_id = ?", workspaceID).
		Where(`(
			support_conversations.crm_company_id = ?
			OR support_conversations.crm_contact_id IN (
				SELECT CASE WHEN ca.from_object_type = 'contact' THEN ca.from_object_id ELSE ca.to_object_id END
				FROM crm_associations ca WHERE ca.workspace_id = support_conversations.workspace_id
				  AND ((ca.from_object_type = 'contact' AND ca.to_object_type = 'company' AND ca.to_object_id = ?)
				    OR (ca.to_object_type = 'contact' AND ca.from_object_type = 'company' AND ca.from_object_id = ?))
			)
			OR EXISTS (
				SELECT 1 FROM crm_associations sa WHERE sa.workspace_id = support_conversations.workspace_id
				  AND ((sa.from_object_type = 'support_conversation' AND sa.from_object_id = support_conversations.id AND sa.to_object_type = 'company' AND sa.to_object_id = ?)
				    OR (sa.to_object_type = 'support_conversation' AND sa.to_object_id = support_conversations.id AND sa.from_object_type = 'company' AND sa.from_object_id = ?))
			)
		)`, companyID, companyID, companyID, companyID, companyID)
	if trimmed := strings.TrimSpace(status); trimmed != "" && trimmed != "all" {
		query = query.Where("support_conversations.status = ?", trimmed)
	}
	if trimmed := strings.TrimSpace(search); trimmed != "" {
		query = query.Where("LOWER(support_conversations.subject) LIKE ?", "%"+strings.ToLower(trimmed)+"%")
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("count company conversations: %w", err)
	}
	page, perPage := pagination.Page, pagination.PerPage
	if page < 1 {
		page = 1
	}
	if perPage < 1 {
		perPage = 50
	}
	if perPage > 100 {
		perPage = 100
	}
	var conversations []model.SupportConversation
	if err := query.Order("support_conversations.updated_at DESC").Offset((page - 1) * perPage).Limit(perPage).Find(&conversations).Error; err != nil {
		return nil, 0, fmt.Errorf("list company conversations: %w", err)
	}
	return conversations, total, nil
}

// ListByAnonymousID returns conversations for a visitor by anonymous_id.
func (r *SupportConversationRepository) ListByAnonymousID(ctx context.Context, workspaceID, anonymousID string) ([]model.SupportConversation, error) {
	var conversations []model.SupportConversation
	if err := r.db.WithContext(ctx).
		Select(fmt.Sprintf(`support_conversations.*,
		(SELECT content FROM support_messages WHERE support_messages.conversation_id = support_conversations.id AND support_messages.deleted_at IS NULL AND support_messages.is_internal = false AND support_messages.message_type = 'reply' AND support_messages.system_event_type IS NULL AND `+supportDeliveryModeSQL(r.db, "support_messages")+` <> 'email_only' ORDER BY created_at DESC LIMIT 1) AS last_message,
		(SELECT COUNT(*)
			FROM support_messages sm
			WHERE sm.conversation_id = support_conversations.id
			  AND sm.deleted_at IS NULL
			  AND sm.is_internal = false AND `+supportDeliveryModeSQL(r.db, "sm")+` <> 'email_only'
			  AND sm.sender_type IN ('user', 'agent', 'ai')
			  AND sm.message_type = 'reply'
			  AND sm.system_event_type IS NULL
			  AND sm.created_at > COALESCE(support_conversations.contact_last_seen_at, %s)
		) AS unread_count`, r.epochExpr())).
		Where("workspace_id = ? AND anonymous_id = ?", workspaceID, anonymousID).
		Order("updated_at DESC").
		Find(&conversations).Error; err != nil {
		return nil, fmt.Errorf("list conversations by anonymous_id: %w", err)
	}
	for i := range conversations {
		// Staff list projections may include email-only replies or internal notes.
		// The widget's safe public preview is computed independently above.
		conversations[i].ListLastMessagePreview = conversations[i].LastMessage
		conversations[i].ListLastMessageID = nil
		conversations[i].ListLastMessageAt = nil
		conversations[i].ListLastMessageIsInternal = false
		conversations[i].LastPublicMessageID = nil
		conversations[i].LastPublicMessageAt = nil
		conversations[i].LastPublicSenderType = nil
		conversations[i].LastPublicSenderDisplayName = nil
		if conversations[i].LastMessage == nil {
			continue
		}
		cleaned := cleanMessageSnippet(*conversations[i].LastMessage, 100)
		conversations[i].LastMessage = &cleaned
	}
	return conversations, nil
}

// MarkInternalRead advances team_last_seen_at to the latest readable customer message.
// Uses raw SQL to avoid GORM's autoUpdateTime touching updated_at (which would re-sort the conversation).
func (r *SupportConversationRepository) MarkInternalRead(ctx context.Context, conversationID string) error {
	result := r.db.WithContext(ctx).Exec(fmt.Sprintf(`
		UPDATE support_conversations
		SET team_last_seen_at = (
			SELECT MAX(read_messages.created_at)
			FROM support_messages read_messages
			WHERE read_messages.conversation_id = support_conversations.id
			  AND read_messages.deleted_at IS NULL
			  AND read_messages.is_internal = false
			  AND read_messages.sender_type = 'customer'
			  AND read_messages.message_type = 'reply'
			  AND read_messages.system_event_type IS NULL
		)
		WHERE id = ?
		  AND EXISTS (
			SELECT 1 FROM support_messages sm
			WHERE sm.conversation_id = support_conversations.id
			  AND sm.deleted_at IS NULL
			  AND sm.is_internal = false
			  AND sm.sender_type = 'customer'
			  AND sm.message_type = 'reply'
			  AND sm.system_event_type IS NULL
			  AND sm.created_at > COALESCE(support_conversations.team_last_seen_at, %s)
		  )
	`, r.epochExpr()), conversationID)
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
		// Explicit conversation deletion remains available after anonymization.
		// Remove the parent first so message cleanup cannot resurrect projections
		// on a read-only conversation. Everything still commits atomically.
		var conversation model.SupportConversation
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("workspace_id = ? AND id = ?", workspaceID, conversationID).Find(&conversation).Error; err != nil {
			return err
		}
		if conversation.AnonymizedAt != nil {
			if err := tx.Where("workspace_id = ? AND id = ?", workspaceID, conversationID).Delete(&model.SupportConversation{}).Error; err != nil {
				return fmt.Errorf("delete anonymized conversation: %w", err)
			}
		}

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
			  AND sm.is_internal = false AND `+supportDeliveryModeSQL(r.db, "sm")+` <> 'email_only'
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
	if stats, enabled, err := r.getUnreadStatsFromCoreBuckets(ctx, workspaceID, userID, workspaceMemberID, role, mailboxID); err != nil {
		return model.UnreadStats{}, err
	} else if enabled {
		return stats, nil
	}
	if r.db.Dialector.Name() == "postgres" {
		return r.getLegacyUnreadStats(ctx, workspaceID, userID, workspaceMemberID, role, mailboxID)
	}

	var stats model.UnreadStats
	humanInboxCondition := conversationHumanInboxCondition("sc")
	aiActiveCondition := conversationAIActiveCondition("sc")
	supportUnreadScopeCondition := `NOT (` + conversationResolvedByAICondition("sc") + `)`
	mineCondition := `(` + conversationHumanInboxCondition("sc") + ` OR sc.status = 'waiting_on_customer')
		AND COALESCE(personal_state.relevance_mask, 0) <> 0`
	unreadCondition := `(COALESCE(personal_state.unread_customer_message_count, 0) > 0 OR COALESCE(personal_state.manually_unread, false))`
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
				  AND `+conversationWaitingOnCustomerCondition("sc")+`
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
				WHERE `+conversationWaitingOnCustomerCondition("sc")+`
			) AS waiting_total,
			COUNT(*) FILTER (
				WHERE %s
			) AS ai_active_total,
			COUNT(*) FILTER (
				WHERE sc.needs_human_reply AND %s
			) AS inbox_needs_human_reply,
			COUNT(*) FILTER (
				WHERE sc.needs_human_reply AND %s
			) AS mine_needs_human_reply,
			COUNT(*) FILTER (
				WHERE sc.needs_human_reply AND `+conversationWaitingOnCustomerCondition("sc")+`
			) AS waiting_needs_human_reply,
			COUNT(*) FILTER (
				WHERE sc.needs_human_reply AND %s
			) AS ai_active_needs_human_reply
		FROM support_conversations sc
		LEFT JOIN support_conversation_user_states personal_state
		  ON personal_state.conversation_id = sc.id
		 AND personal_state.workspace_id = sc.workspace_id
		 AND personal_state.user_id = ?
		WHERE sc.workspace_id = ?
		  AND sc.status NOT IN ('resolved', 'spam')
	`, unreadCondition, humanInboxCondition, unreadCondition, mineCondition, unreadCondition, unreadCondition, aiActiveCondition, unreadCondition, supportUnreadScopeCondition, unreadCondition, mineCondition, unreadCondition, humanInboxCondition, humanInboxCondition, mineCondition, aiActiveCondition, humanInboxCondition, mineCondition, aiActiveCondition)

	args := []any{userID, workspaceID}
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

func (r *SupportConversationRepository) getLegacyUnreadStats(ctx context.Context, workspaceID, userID, workspaceMemberID, role string, mailboxID *string) (model.UnreadStats, error) {
	var stats model.UnreadStats
	humanInboxCondition := conversationHumanInboxCondition("sc")
	aiActiveCondition := conversationAIActiveCondition("sc")
	supportUnreadScopeCondition := `NOT (` + conversationResolvedByAICondition("sc") + `)`
	mentionCondition, mentionArgs := r.mentionExistsCondition("sc", userID)
	mineCondition := `(` + conversationHumanInboxCondition("sc") + ` OR sc.status = 'waiting_on_customer') AND (
		sc.assigned_user_id = ? OR sc.opened_by_user_id = ? OR ` + mentionCondition + `)`
	unreadCondition := fmt.Sprintf(`EXISTS (
		SELECT 1 FROM support_messages unread_message
		WHERE unread_message.conversation_id = sc.id
		  AND unread_message.deleted_at IS NULL
		  AND unread_message.system_event_type IS NULL
		  AND unread_message.is_internal = FALSE
		  AND unread_message.sender_type = 'customer'
		  AND unread_message.message_type = 'reply'
		  AND unread_message.created_at > COALESCE(sc.team_last_seen_at, %s)
	)`, r.epochExpr())
	query := fmt.Sprintf(`
		SELECT
			COUNT(*) FILTER (WHERE %s AND %s) AS inbox,
			COUNT(*) FILTER (WHERE %s AND %s) AS mine,
			COUNT(*) FILTER (WHERE %s AND `+conversationWaitingOnCustomerCondition("sc")+`) AS waiting,
			COUNT(*) FILTER (WHERE %s AND %s) AS ai_active,
			COUNT(*) FILTER (WHERE %s AND %s) AS total,
			COUNT(*) FILTER (WHERE %s AND %s) AS my_inbox,
			COUNT(*) FILTER (WHERE %s AND %s AND sc.assigned_agent_id IS NULL AND sc.assigned_user_id IS NULL) AS unassigned,
			COUNT(*) FILTER (WHERE %s) AS inbox_total,
			COUNT(*) FILTER (WHERE %s) AS mine_total,
			COUNT(*) FILTER (WHERE `+conversationWaitingOnCustomerCondition("sc")+`) AS waiting_total,
			COUNT(*) FILTER (WHERE %s) AS ai_active_total,
			COUNT(*) FILTER (WHERE sc.needs_human_reply AND %s) AS inbox_needs_human_reply,
			COUNT(*) FILTER (WHERE sc.needs_human_reply AND %s) AS mine_needs_human_reply,
			COUNT(*) FILTER (WHERE sc.needs_human_reply AND `+conversationWaitingOnCustomerCondition("sc")+`) AS waiting_needs_human_reply,
			COUNT(*) FILTER (WHERE sc.needs_human_reply AND %s) AS ai_active_needs_human_reply
		FROM support_conversations sc
		WHERE sc.workspace_id = ? AND sc.status NOT IN ('resolved', 'spam')
	`, unreadCondition, humanInboxCondition, unreadCondition, mineCondition,
		unreadCondition, unreadCondition, aiActiveCondition, unreadCondition, supportUnreadScopeCondition,
		unreadCondition, mineCondition, unreadCondition, humanInboxCondition,
		humanInboxCondition, mineCondition, aiActiveCondition, humanInboxCondition, mineCondition, aiActiveCondition)
	args := make([]any, 0, 20)
	for range 4 {
		args = append(args, userID, userID)
		args = append(args, mentionArgs...)
	}
	args = append(args, workspaceID)
	if mailboxID != nil {
		if strings.TrimSpace(*mailboxID) == "" {
			query += " AND sc.mailbox_id IS NULL"
		} else {
			query += " AND sc.mailbox_id = ?"
			args = append(args, strings.TrimSpace(*mailboxID))
		}
	}
	if !isElevatedSupportRole(role) {
		query += ` AND (sc.mailbox_id IS NULL OR sc.mailbox_id IN (
			SELECT mailbox.id FROM support_mailboxes mailbox
			WHERE mailbox.active = TRUE AND ` + supportMailboxAccessCondition("mailbox") + `))`
		args = append(args, workspaceMemberID, workspaceMemberID)
	}
	if err := r.db.WithContext(ctx).Raw(query, args...).Scan(&stats).Error; err != nil {
		return stats, fmt.Errorf("get legacy unread stats: %w", err)
	}
	return stats, nil
}

// getUnreadStatsFromCoreBuckets is the constant-work V2 counter read. The
// workspace rollout row is the cutover boundary: legacy and shadow modes keep
// returning the exact legacy aggregate while operators reconcile projections.
func (r *SupportConversationRepository) getUnreadStatsFromCoreBuckets(
	ctx context.Context,
	workspaceID, userID, workspaceMemberID, role string,
	mailboxID *string,
) (model.UnreadStats, bool, error) {
	var stats model.UnreadStats
	if r.db.Dialector.Name() != "postgres" {
		return stats, false, nil
	}
	var mode string
	if err := r.db.WithContext(ctx).Table("support_inbox_state_rollouts").
		Select("mode").Where("workspace_id = ?", workspaceID).Scan(&mode).Error; err != nil {
		return stats, false, fmt.Errorf("load support inbox rollout mode: %w", err)
	}
	if mode != "v2" {
		return stats, false, nil
	}

	query := `
		SELECT
			COALESCE(SUM(unread_count) FILTER (WHERE audience_type = 'user' AND bucket_id = 'inbox'), 0) AS inbox,
			COALESCE(SUM(unread_count) FILTER (WHERE audience_type = 'user' AND bucket_id = 'mine'), 0) AS mine,
			COALESCE(SUM(unread_count) FILTER (WHERE audience_type = 'user' AND bucket_id = 'waiting'), 0) AS waiting,
			COALESCE(SUM(unread_count) FILTER (WHERE audience_type = 'user' AND bucket_id = 'ai_active'), 0) AS ai_active,
			COALESCE(SUM(unread_count) FILTER (WHERE audience_type = 'user' AND bucket_id = 'support'), 0) AS total,
			COALESCE(SUM(unread_count) FILTER (WHERE audience_type = 'user' AND bucket_id = 'mine'), 0) AS my_inbox,
			COALESCE(SUM(unread_count) FILTER (WHERE audience_type = 'user' AND bucket_id = 'unassigned'), 0) AS unassigned,
			COALESCE(SUM(total_count) FILTER (WHERE audience_type = 'shared' AND bucket_id = 'inbox'), 0) AS inbox_total,
			COALESCE(SUM(total_count) FILTER (WHERE audience_type = 'user' AND bucket_id = 'mine'), 0) AS mine_total,
			COALESCE(SUM(total_count) FILTER (WHERE audience_type = 'shared' AND bucket_id = 'waiting'), 0) AS waiting_total,
			COALESCE(SUM(total_count) FILTER (WHERE audience_type = 'shared' AND bucket_id = 'ai_active'), 0) AS ai_active_total,
			COALESCE(SUM(needs_human_reply_count) FILTER (WHERE audience_type = 'shared' AND bucket_id = 'inbox'), 0) AS inbox_needs_human_reply,
			COALESCE(SUM(needs_human_reply_count) FILTER (WHERE audience_type = 'user' AND bucket_id = 'mine'), 0) AS mine_needs_human_reply,
			COALESCE(SUM(needs_human_reply_count) FILTER (WHERE audience_type = 'shared' AND bucket_id = 'waiting'), 0) AS waiting_needs_human_reply,
			COALESCE(SUM(needs_human_reply_count) FILTER (WHERE audience_type = 'shared' AND bucket_id = 'ai_active'), 0) AS ai_active_needs_human_reply
		FROM support_inbox_counter_buckets bucket
		WHERE bucket.workspace_id = ?
		  AND bucket.bucket_type = 'core'
		  AND ((bucket.audience_type = 'shared' AND bucket.audience_id = 'shared')
		       OR (bucket.audience_type = 'user' AND bucket.audience_id = ?))`
	args := []any{workspaceID, userID}
	if mailboxID != nil {
		scope := strings.TrimSpace(*mailboxID)
		if scope == "" {
			scope = "shared"
		}
		query += " AND bucket.mailbox_scope_id = ?"
		args = append(args, scope)
	} else if !isElevatedSupportRole(role) {
		query += ` AND (
			bucket.mailbox_scope_id = 'shared'
			OR bucket.mailbox_scope_id IN (
				SELECT mailbox.id::TEXT
				FROM support_mailboxes mailbox
				WHERE mailbox.workspace_id = ?
				  AND mailbox.active = TRUE
				  AND ` + supportMailboxAccessCondition("mailbox") + `
			)
		)`
		args = append(args, workspaceID, workspaceMemberID, workspaceMemberID)
	}
	if err := r.db.WithContext(ctx).Raw(query, args...).Scan(&stats).Error; err != nil {
		return stats, true, fmt.Errorf("get materialized unread stats: %w", err)
	}
	return stats, true, nil
}

// UsesV2SupportInboxState reports whether personal unread and materialized
// counters are authoritative for this workspace. Non-Postgres test stores run
// the V2 behavior directly because rollout tables are PostgreSQL-only.
func (r *SupportConversationRepository) UsesV2SupportInboxState(ctx context.Context, workspaceID string) (bool, error) {
	if r.db.Dialector.Name() != "postgres" {
		return true, nil
	}
	var enabled bool
	if err := r.db.WithContext(ctx).Raw(`SELECT EXISTS (
		SELECT 1 FROM support_inbox_state_rollouts
		WHERE workspace_id = ? AND mode = 'v2'
	)`, workspaceID).Scan(&enabled).Error; err != nil {
		return false, fmt.Errorf("load support inbox state mode: %w", err)
	}
	return enabled, nil
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

// UpgradeIdentityProvenanceByAnonymousID records evidence for the identity just
// applied to these sessions. Replacing identity also replaces its verification.
func (r *SupportInboxSessionRepository) UpgradeIdentityProvenanceByAnonymousID(
	ctx context.Context,
	workspaceID, anonymousID, method, trust string,
	verifiedAt *time.Time,
	verifierVersion *string,
) error {
	updates := map[string]interface{}{
		"identity_method": method,
		"identity_trust":  trust,
	}
	updates["identity_verified_at"] = verifiedAt
	updates["identity_verifier_version"] = verifierVersion

	query := r.db.WithContext(ctx).Model(&model.SupportWidgetSession{}).
		Where("workspace_id = ? AND anonymous_id = ?", workspaceID, anonymousID)
	if err := query.Updates(updates).Error; err != nil {
		return fmt.Errorf("upgrade session identity provenance: %w", err)
	}
	return nil
}

// UpdateCompanyByID changes company context for one exact widget session.
func (r *SupportInboxSessionRepository) UpdateCompanyByID(ctx context.Context, workspaceID, sessionID string, companyID *string) error {
	result := r.db.WithContext(ctx).
		Model(&model.SupportWidgetSession{}).
		Where("workspace_id = ? AND id = ?", workspaceID, sessionID).
		UpdateColumn("crm_company_id", companyID)
	if result.Error != nil {
		return fmt.Errorf("update session company: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("update session company: session not found")
	}
	return nil
}

// UpdateActiveSessionsCompanyByAnonymousID updates mutable company context only
// on non-revoked, unexpired sessions and returns the sessions in that scope.
func (r *SupportInboxSessionRepository) UpdateActiveSessionsCompanyByAnonymousID(ctx context.Context, workspaceID, anonymousID, companyID string) ([]model.SupportWidgetSession, error) {
	now := time.Now()
	scope := r.db.WithContext(ctx).
		Model(&model.SupportWidgetSession{}).
		Where("workspace_id = ? AND anonymous_id = ? AND revoked_at IS NULL AND expires_at > ?", workspaceID, anonymousID, now)
	if err := scope.UpdateColumn("crm_company_id", companyID).Error; err != nil {
		return nil, fmt.Errorf("update active session company: %w", err)
	}
	var sessions []model.SupportWidgetSession
	if err := r.db.WithContext(ctx).
		Where("workspace_id = ? AND anonymous_id = ? AND revoked_at IS NULL AND expires_at > ?", workspaceID, anonymousID, now).
		Find(&sessions).Error; err != nil {
		return nil, fmt.Errorf("list active sessions after company update: %w", err)
	}
	return sessions, nil
}

// SetCRMCompanyIfUnset captures stable company identity without overwriting a
// conversation that already has explicit context.
func (r *SupportConversationRepository) SetCRMCompanyIfUnset(ctx context.Context, workspaceID, conversationID, companyID string) (bool, error) {
	result := r.db.WithContext(ctx).
		Model(&model.SupportConversation{}).
		Where("workspace_id = ? AND id = ? AND crm_company_id IS NULL", workspaceID, conversationID).
		UpdateColumn("crm_company_id", companyID)
	if result.Error != nil {
		return false, fmt.Errorf("set conversation company if unset: %w", result.Error)
	}
	return result.RowsAffected > 0, nil
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
