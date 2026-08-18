package service

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
)

const defaultSupportMessagePageSize = 20

// ErrInvalidSupportMessageCursor indicates that a message-page cursor cannot be decoded.
var ErrInvalidSupportMessageCursor = errors.New("invalid support message cursor")

type supportMessageCursor struct {
	CreatedAt time.Time `json:"created_at"`
	ID        string    `json:"id"`
}

// ListConversationMessagePage returns a bounded page counted backward from
// the newest message in a conversation.
func (s *SupportInboxService) ListConversationMessagePage(
	ctx context.Context,
	workspaceID, conversationID string,
	includeInternal bool,
	limit int,
	cursor string,
) (*model.SupportMessagePage, error) {
	startedAt := time.Now()
	defer func() {
		slog.InfoContext(ctx, "listed support message page",
			"workspace_id", workspaceID,
			"conversation_id", conversationID,
			"duration_ms", time.Since(startedAt).Milliseconds())
	}()
	if workspaceID == "" {
		return nil, fmt.Errorf("workspace_id is required")
	}
	if limit == 0 {
		limit = defaultSupportMessagePageSize
	}
	if limit < 1 || limit > 100 {
		return nil, fmt.Errorf("message page limit must be between 1 and 100")
	}
	conversation, err := s.loadConversationAccessible(ctx, workspaceID, conversationID)
	if err != nil {
		return nil, err
	}
	if conversation == nil {
		return nil, fmt.Errorf("conversation not found")
	}

	var beforeCreatedAt *time.Time
	var beforeID string
	if cursor != "" {
		decodedAt, decodedID, decodeErr := decodeSupportMessageCursor(cursor)
		if decodeErr != nil {
			return nil, decodeErr
		}
		beforeCreatedAt = &decodedAt
		beforeID = decodedID
	}
	messages, hasMore, err := s.messageRepo.ListConversationPageBefore(
		ctx,
		workspaceID,
		conversationID,
		includeInternal,
		limit,
		beforeCreatedAt,
		beforeID,
	)
	if err != nil {
		return nil, err
	}
	if messages == nil {
		messages = []model.SupportMessage{}
	}

	if s.attachmentService != nil && len(messages) > 0 {
		if err := s.attachmentService.HydrateMessages(ctx, messages); err != nil {
			slog.ErrorContext(ctx, "hydrate support message page attachments",
				"error", err, "conversation_id", conversationID)
		}
	}
	if s.emailLogRepo != nil && len(messages) > 0 {
		messageIDs := make([]string, len(messages))
		for i := range messages {
			messageIDs[i] = messages[i].ID
		}
		logs, logErr := s.emailLogRepo.ListByMessageIDs(ctx, workspaceID, messageIDs)
		if logErr != nil {
			slog.ErrorContext(ctx, "hydrate support message page email bodies",
				"error", logErr, "conversation_id", conversationID)
		} else {
			hydrateEmailBodiesFromLogs(messages, logs)
		}
	}

	page := &model.SupportMessagePage{Data: messages, HasMore: hasMore}
	if hasMore && len(messages) > 0 {
		next := encodeSupportMessageCursor(messages[0].CreatedAt, messages[0].ID)
		page.NextCursor = &next
	}
	return page, nil
}

func encodeSupportMessageCursor(createdAt time.Time, id string) string {
	payload, _ := json.Marshal(supportMessageCursor{CreatedAt: createdAt.UTC(), ID: id})
	return base64.RawURLEncoding.EncodeToString(payload)
}

func decodeSupportMessageCursor(cursor string) (time.Time, string, error) {
	payload, err := base64.RawURLEncoding.DecodeString(cursor)
	if err != nil {
		return time.Time{}, "", ErrInvalidSupportMessageCursor
	}
	var decoded supportMessageCursor
	if err := json.Unmarshal(payload, &decoded); err != nil || decoded.CreatedAt.IsZero() || strings.TrimSpace(decoded.ID) == "" {
		return time.Time{}, "", ErrInvalidSupportMessageCursor
	}
	return decoded.CreatedAt.UTC(), decoded.ID, nil
}
