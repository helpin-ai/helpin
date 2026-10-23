package service

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

const (
	clearTestWorkspace = "6f1d0a52-0000-4000-8000-000000000001"
	clearTestUser      = "6f1d0a52-0000-4000-8000-0000000000aa"
	clearTestBilling   = "6f1d0a52-0000-4000-8000-0000000000b1"
	clearTestSales     = "6f1d0a52-0000-4000-8000-0000000000b2"
	clearTestConvA     = "6f1d0a52-0000-4000-8000-00000000c001"
	clearTestConvB     = "6f1d0a52-0000-4000-8000-00000000c002"
	clearTestConvC     = "6f1d0a52-0000-4000-8000-00000000c003"
	clearTestConvD     = "6f1d0a52-0000-4000-8000-00000000c004"
	clearTestConvOpen  = "6f1d0a52-0000-4000-8000-00000000c005"
)

var clearTestNow = time.Date(2026, 10, 23, 12, 0, 0, 0, time.UTC)

func setupSupportInboxClearService(t *testing.T) (*SupportInboxClearService, *gorm.DB) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:support_inbox_clear_svc_%d?mode=memory&cache=shared", time.Now().UnixNano())), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	for _, stmt := range []string{
		`CREATE TABLE support_inbox_clears (id TEXT PRIMARY KEY, workspace_id TEXT NOT NULL, user_id TEXT NOT NULL, scope_key TEXT NOT NULL, mailbox_id TEXT, conversation_id TEXT NOT NULL, local_date TEXT NOT NULL, cleared_at DATETIME NOT NULL, created_at DATETIME)`,
		`CREATE UNIQUE INDEX idx_support_inbox_clears_once ON support_inbox_clears(workspace_id, user_id, scope_key, conversation_id)`,
		`CREATE TABLE support_conversations (id TEXT PRIMARY KEY, workspace_id TEXT NOT NULL, mailbox_id TEXT, status TEXT NOT NULL)`,
		`CREATE TABLE support_messages (id TEXT PRIMARY KEY, workspace_id TEXT NOT NULL, conversation_id TEXT NOT NULL, sender_user_id TEXT, message_type TEXT NOT NULL, system_event_type TEXT, created_at DATETIME NOT NULL)`,
	} {
		if err := db.Exec(stmt).Error; err != nil {
			t.Fatalf("create schema: %v", err)
		}
	}
	svc := NewSupportInboxClearService(repository.NewSupportInboxClearRepository(db))
	svc.now = func() time.Time { return clearTestNow }
	return svc, db
}

func seedClearConversation(t *testing.T, db *gorm.DB, id, mailboxID, status string) {
	t.Helper()
	if err := db.Exec(`INSERT INTO support_conversations (id, workspace_id, mailbox_id, status) VALUES (?, ?, ?, ?)`, id, clearTestWorkspace, mailboxID, status).Error; err != nil {
		t.Fatal(err)
	}
}

func seedResolvedNotice(t *testing.T, db *gorm.DB, id, conversationID string, at time.Time) {
	t.Helper()
	if err := db.Exec(`INSERT INTO support_messages (id, workspace_id, conversation_id, sender_user_id, message_type, system_event_type, created_at) VALUES (?, ?, ?, ?, 'system', 'resolved', ?)`,
		id, clearTestWorkspace, conversationID, clearTestUser, at.UTC()).Error; err != nil {
		t.Fatal(err)
	}
}

func seedClear(t *testing.T, db *gorm.DB, id, scopeKey, conversationID, localDate string, at time.Time) {
	t.Helper()
	if err := db.Exec(`INSERT INTO support_inbox_clears (id, workspace_id, user_id, scope_key, conversation_id, local_date, cleared_at) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		id, clearTestWorkspace, clearTestUser, scopeKey, conversationID, localDate, at.UTC()).Error; err != nil {
		t.Fatal(err)
	}
}

func mailboxClearRequest(conversationID string) model.RecordSupportInboxClearRequest {
	mailbox := clearTestBilling
	return model.RecordSupportInboxClearRequest{
		ConversationID: conversationID,
		Scope:          model.SupportInboxClearScopeMailbox,
		MailboxID:      &mailbox,
		LocalDate:      "2026-10-23",
		DayStart:       "2026-10-23T00:00:00Z",
	}
}

func TestSupportInboxClearService_RejectsInvalidRequests(t *testing.T) {
	svc, _ := setupSupportInboxClearService(t)
	notUUID := "billing"
	cases := []struct {
		name   string
		mutate func(*model.RecordSupportInboxClearRequest)
	}{
		{name: "conversation id is not a uuid", mutate: func(r *model.RecordSupportInboxClearRequest) { r.ConversationID = "conv-1" }},
		{name: "unknown scope", mutate: func(r *model.RecordSupportInboxClearRequest) { r.Scope = "team" }},
		{name: "mailbox scope without mailbox", mutate: func(r *model.RecordSupportInboxClearRequest) { r.MailboxID = nil }},
		{name: "mailbox id is not a uuid", mutate: func(r *model.RecordSupportInboxClearRequest) { r.MailboxID = &notUUID }},
		{name: "malformed local date", mutate: func(r *model.RecordSupportInboxClearRequest) { r.LocalDate = "23/10/2026" }},
		{name: "malformed day start", mutate: func(r *model.RecordSupportInboxClearRequest) { r.DayStart = "yesterday" }},
		{name: "day start too far away", mutate: func(r *model.RecordSupportInboxClearRequest) { r.DayStart = "2026-10-20T00:00:00Z" }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := mailboxClearRequest(clearTestConvB)
			tc.mutate(&req)
			_, err := svc.Record(context.Background(), clearTestWorkspace, clearTestUser, req)
			if !errors.Is(err, ErrSupportInboxClearInvalid) {
				t.Fatalf("Record error = %v, want invalid", err)
			}
		})
	}
}

func TestSupportInboxClearService_RequiresAResolvedConversation(t *testing.T) {
	svc, db := setupSupportInboxClearService(t)
	seedClearConversation(t, db, clearTestConvOpen, clearTestBilling, model.SupportConversationStatusOpen)

	if _, err := svc.Record(context.Background(), clearTestWorkspace, clearTestUser, mailboxClearRequest(clearTestConvB)); !errors.Is(err, ErrSupportInboxClearNotFound) {
		t.Fatalf("missing conversation error = %v, want not found", err)
	}
	if _, err := svc.Record(context.Background(), clearTestWorkspace, clearTestUser, mailboxClearRequest(clearTestConvOpen)); !errors.Is(err, ErrSupportInboxClearStillWaiting) {
		t.Fatalf("open conversation error = %v, want still waiting", err)
	}
}

func TestSupportInboxClearService_ReportsStats(t *testing.T) {
	svc, db := setupSupportInboxClearService(t)
	dayStart := time.Date(2026, 10, 23, 0, 0, 0, 0, time.UTC)
	for _, conv := range []struct{ id, mailbox string }{
		{clearTestConvA, clearTestBilling}, {clearTestConvB, clearTestBilling},
		{clearTestConvC, clearTestBilling}, {clearTestConvD, clearTestSales},
	} {
		seedClearConversation(t, db, conv.id, conv.mailbox, model.SupportConversationStatusResolved)
	}
	billingScope := model.SupportInboxClearScopeMailbox + ":" + clearTestBilling
	// Earlier clears: today at 10:00 in Billing, the two days before, and a
	// gap before that, so the streak is 3 including today's new clear.
	seedClear(t, db, "prev-today", billingScope, clearTestConvA, "2026-10-23", dayStart.Add(10*time.Hour))
	seedClear(t, db, "prev-22", "all", clearTestConvC, "2026-10-22", dayStart.Add(-14*time.Hour))
	seedClear(t, db, "prev-21", "mine", clearTestConvD, "2026-10-21", dayStart.Add(-38*time.Hour))
	seedClear(t, db, "prev-19", "all", clearTestConvA, "2026-10-19", dayStart.Add(-86*time.Hour))
	seedResolvedNotice(t, db, "n1", clearTestConvA, dayStart.Add(9*time.Hour))
	seedResolvedNotice(t, db, "n2", clearTestConvC, dayStart.Add(11*time.Hour))
	seedResolvedNotice(t, db, "n3", clearTestConvB, dayStart.Add(11*time.Hour+30*time.Minute))
	seedResolvedNotice(t, db, "n4", clearTestConvD, dayStart.Add(11*time.Hour+45*time.Minute))

	stats, err := svc.Record(context.Background(), clearTestWorkspace, clearTestUser, mailboxClearRequest(clearTestConvB))
	if err != nil {
		t.Fatalf("Record: %v", err)
	}
	want := model.SupportInboxClearStats{StreakDays: 3, ClearsToday: 2, ResolvedToday: 4, ClearedCount: 2}
	if *stats != want {
		t.Fatalf("stats = %+v, want %+v", *stats, want)
	}

	repeat, err := svc.Record(context.Background(), clearTestWorkspace, clearTestUser, mailboxClearRequest(clearTestConvB))
	if err != nil {
		t.Fatalf("repeat Record: %v", err)
	}
	if *repeat != want {
		t.Fatalf("repeat stats = %+v, want %+v", *repeat, want)
	}
	var rows int64
	if err := db.Table("support_inbox_clears").Where("conversation_id = ?", clearTestConvB).Count(&rows).Error; err != nil || rows != 1 {
		t.Fatalf("clear rows = %d, %v; want 1", rows, err)
	}
}

func TestSupportInboxClearService_CountsFromDayStartWithoutAnEarlierClear(t *testing.T) {
	svc, db := setupSupportInboxClearService(t)
	dayStart := time.Date(2026, 10, 23, 0, 0, 0, 0, time.UTC)
	seedClearConversation(t, db, clearTestConvA, clearTestBilling, model.SupportConversationStatusResolved)
	seedClearConversation(t, db, clearTestConvB, clearTestSales, model.SupportConversationStatusSpam)
	seedResolvedNotice(t, db, "n0", clearTestConvA, dayStart.Add(-time.Hour))
	seedResolvedNotice(t, db, "n1", clearTestConvA, dayStart.Add(3*time.Hour))

	req := model.RecordSupportInboxClearRequest{ConversationID: clearTestConvB, Scope: model.SupportInboxClearScopeAll, LocalDate: "2026-10-23", DayStart: "2026-10-23T00:00:00Z"}
	stats, err := svc.Record(context.Background(), clearTestWorkspace, clearTestUser, req)
	if err != nil {
		t.Fatalf("Record: %v", err)
	}
	want := model.SupportInboxClearStats{StreakDays: 1, ClearsToday: 1, ResolvedToday: 1, ClearedCount: 1}
	if *stats != want {
		t.Fatalf("stats = %+v, want %+v", *stats, want)
	}

	// A spam clear in a mailbox with no resolved notices still reports one.
	sales := clearTestSales
	mailboxReq := model.RecordSupportInboxClearRequest{ConversationID: clearTestConvB, Scope: model.SupportInboxClearScopeMailbox, MailboxID: &sales, LocalDate: "2026-10-23", DayStart: "2026-10-23T00:00:00Z"}
	stats, err = svc.Record(context.Background(), clearTestWorkspace, clearTestUser, mailboxReq)
	if err != nil {
		t.Fatalf("Record mailbox: %v", err)
	}
	if stats.ClearedCount != 1 {
		t.Fatalf("cleared_count = %d, want the minimum of 1", stats.ClearedCount)
	}
}

func TestSupportInboxClearStreak(t *testing.T) {
	cases := []struct {
		name  string
		dates []string
		want  int
	}{
		{name: "no clears", dates: nil, want: 0},
		{name: "only today", dates: []string{"2026-10-23"}, want: 1},
		{name: "stops at a gap", dates: []string{"2026-10-23", "2026-10-22", "2026-10-20"}, want: 2},
		{name: "crosses a month boundary", dates: []string{"2026-11-01", "2026-10-31", "2026-10-30"}, want: 3},
		{name: "today missing", dates: []string{"2026-10-22", "2026-10-21"}, want: 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			today := "2026-10-23"
			if tc.name == "crosses a month boundary" {
				today = "2026-11-01"
			}
			if got := supportInboxClearStreak(tc.dates, today); got != tc.want {
				t.Fatalf("streak = %d, want %d", got, tc.want)
			}
		})
	}
}
