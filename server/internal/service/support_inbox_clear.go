package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// Errors returned by SupportInboxClearService.Record. Handlers map them to
// 400, 404 and 409.
var (
	ErrSupportInboxClearInvalid      = errors.New("invalid inbox clear")
	ErrSupportInboxClearNotFound     = errors.New("conversation not found")
	ErrSupportInboxClearStillWaiting = errors.New("conversation is not resolved")
)

const (
	supportInboxClearDateLayout = "2006-01-02"
	// supportInboxClearDayStartWindow bounds how far the viewer's local
	// midnight may be from now: any time zone plus a day of slack.
	supportInboxClearDayStartWindow = 36 * time.Hour
	supportInboxClearStreakLookback = 400
)

// supportInboxClearStore is the persistence SupportInboxClearService needs.
type supportInboxClearStore interface {
	FindConversationStatus(ctx context.Context, workspaceID, conversationID string) (string, bool, error)
	CreateIfAbsent(ctx context.Context, entry *model.SupportInboxClear) (bool, error)
	FindClear(ctx context.Context, workspaceID, userID, scopeKey, conversationID string) (*model.SupportInboxClear, error)
	PreviousClearAt(ctx context.Context, workspaceID, userID, scopeKey string, before time.Time, excludeConversationID string) (*time.Time, error)
	CountClearsOnDate(ctx context.Context, workspaceID, userID, scopeKey, localDate string) (int, error)
	ListClearDates(ctx context.Context, workspaceID, userID string, limit int) ([]string, error)
	CountResolvedBy(ctx context.Context, workspaceID, userID string, since time.Time, count model.SupportInboxClearCount) (int, error)
}

// SupportInboxClearService records inbox clears and reports the facts the
// inbox-zero celebration is allowed to state.
type SupportInboxClearService struct {
	store supportInboxClearStore
	now   func() time.Time
}

// NewSupportInboxClearService creates a SupportInboxClearService.
func NewSupportInboxClearService(store supportInboxClearStore) *SupportInboxClearService {
	return &SupportInboxClearService{store: store, now: time.Now}
}

type supportInboxClearInput struct {
	conversationID string
	scopeKey       string
	mailboxID      *string
	count          model.SupportInboxClearCount
	localDate      string
	dayStart       time.Time
}

// Record stores that the user emptied an inbox view by resolving (or marking
// spam) its last conversation, then returns their stats. Recording the same
// clear twice returns the stats without a second row.
func (s *SupportInboxClearService) Record(ctx context.Context, workspaceID, userID string, req model.RecordSupportInboxClearRequest) (*model.SupportInboxClearStats, error) {
	input, err := s.validate(req)
	if err != nil {
		return nil, err
	}
	status, found, err := s.store.FindConversationStatus(ctx, workspaceID, input.conversationID)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, ErrSupportInboxClearNotFound
	}
	if status != model.SupportConversationStatusResolved && status != model.SupportConversationStatusSpam {
		return nil, ErrSupportInboxClearStillWaiting
	}

	entry := &model.SupportInboxClear{
		ID:             uuid.NewString(),
		WorkspaceID:    workspaceID,
		UserID:         userID,
		ScopeKey:       input.scopeKey,
		MailboxID:      input.mailboxID,
		ConversationID: input.conversationID,
		LocalDate:      input.localDate,
		ClearedAt:      s.now().UTC(),
	}
	inserted, err := s.store.CreateIfAbsent(ctx, entry)
	if err != nil {
		return nil, err
	}
	if !inserted {
		existing, err := s.store.FindClear(ctx, workspaceID, userID, input.scopeKey, input.conversationID)
		if err != nil {
			return nil, err
		}
		if existing != nil {
			entry = existing
		}
	}
	return s.stats(ctx, workspaceID, userID, input, entry)
}

func (s *SupportInboxClearService) validate(req model.RecordSupportInboxClearRequest) (supportInboxClearInput, error) {
	var input supportInboxClearInput
	if _, err := uuid.Parse(req.ConversationID); err != nil {
		return input, fmt.Errorf("%w: conversation_id must be a UUID", ErrSupportInboxClearInvalid)
	}
	input.conversationID = req.ConversationID

	switch req.Scope {
	case model.SupportInboxClearScopeMailbox:
		if req.MailboxID == nil {
			return input, fmt.Errorf("%w: mailbox_id is required for a mailbox clear", ErrSupportInboxClearInvalid)
		}
		if _, err := uuid.Parse(*req.MailboxID); err != nil {
			return input, fmt.Errorf("%w: mailbox_id must be a UUID", ErrSupportInboxClearInvalid)
		}
		mailboxID := *req.MailboxID
		input.mailboxID = &mailboxID
		input.count = model.SupportInboxClearCount{MailboxID: &mailboxID}
		input.scopeKey = model.SupportInboxClearScopeMailbox + ":" + mailboxID
	case model.SupportInboxClearScopeShared:
		input.count = model.SupportInboxClearCount{SharedOnly: true}
		input.scopeKey = req.Scope
	case model.SupportInboxClearScopeMine:
		input.scopeKey = req.Scope
	default:
		return input, fmt.Errorf("%w: scope must be mailbox, shared or mine", ErrSupportInboxClearInvalid)
	}

	if _, err := time.Parse(supportInboxClearDateLayout, req.LocalDate); err != nil {
		return input, fmt.Errorf("%w: local_date must be YYYY-MM-DD", ErrSupportInboxClearInvalid)
	}
	input.localDate = req.LocalDate

	dayStart, err := time.Parse(time.RFC3339, req.DayStart)
	if err != nil {
		return input, fmt.Errorf("%w: day_start must be an RFC 3339 time", ErrSupportInboxClearInvalid)
	}
	if gap := s.now().Sub(dayStart); gap > supportInboxClearDayStartWindow || gap < -supportInboxClearDayStartWindow {
		return input, fmt.Errorf("%w: day_start must be within 36 hours of now", ErrSupportInboxClearInvalid)
	}
	input.dayStart = dayStart
	return input, nil
}

func (s *SupportInboxClearService) stats(ctx context.Context, workspaceID, userID string, input supportInboxClearInput, entry *model.SupportInboxClear) (*model.SupportInboxClearStats, error) {
	clearsToday, err := s.store.CountClearsOnDate(ctx, workspaceID, userID, input.scopeKey, input.localDate)
	if err != nil {
		return nil, err
	}
	dates, err := s.store.ListClearDates(ctx, workspaceID, userID, supportInboxClearStreakLookback)
	if err != nil {
		return nil, err
	}
	resolvedToday, err := s.store.CountResolvedBy(ctx, workspaceID, userID, input.dayStart, model.SupportInboxClearCount{})
	if err != nil {
		return nil, err
	}

	since := input.dayStart
	previous, err := s.store.PreviousClearAt(ctx, workspaceID, userID, input.scopeKey, entry.ClearedAt, input.conversationID)
	if err != nil {
		return nil, err
	}
	if previous != nil && previous.After(since) {
		since = *previous
	}
	clearedCount, err := s.store.CountResolvedBy(ctx, workspaceID, userID, since, input.count)
	if err != nil {
		return nil, err
	}

	return &model.SupportInboxClearStats{
		StreakDays:    supportInboxClearStreak(dates, input.localDate),
		ClearsToday:   clearsToday,
		ResolvedToday: resolvedToday,
		ClearedCount:  max(clearedCount, 1),
	}, nil
}

// supportInboxClearStreak counts consecutive dates ending at today that
// appear in dates.
func supportInboxClearStreak(dates []string, today string) int {
	seen := make(map[string]struct{}, len(dates))
	for _, date := range dates {
		seen[date] = struct{}{}
	}
	day, err := time.Parse(supportInboxClearDateLayout, today)
	if err != nil {
		return 0
	}
	streak := 0
	for {
		if _, ok := seen[day.Format(supportInboxClearDateLayout)]; !ok {
			return streak
		}
		streak++
		day = day.AddDate(0, 0, -1)
	}
}
