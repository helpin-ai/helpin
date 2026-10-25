package service

import (
	"bytes"
	"context"
	"encoding/csv"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/authorization"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

type supportReportFixture struct {
	db                   *gorm.DB
	svc                  *SupportReportService
	workspaceID          string
	memberID             string
	restrictedMailboxID  string
	userOne, userTwo     string
	conversationsByLabel map[string]string
	nextDisplayID        int
}

var reportBase = time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)

func at(day int, hour, minute int) time.Time {
	return reportBase.AddDate(0, 0, day).Add(time.Duration(hour)*time.Hour + time.Duration(minute)*time.Minute)
}

func (f *supportReportFixture) conversation(t *testing.T, label string, c model.SupportConversation) string {
	t.Helper()
	f.nextDisplayID++
	c.ID = uuid.NewString()
	c.WorkspaceID = f.workspaceID
	c.DisplayID = f.nextDisplayID
	if c.Subject == "" {
		c.Subject = label
	}
	if c.Channel == "" {
		c.Channel = "widget"
	}
	if c.Priority == "" {
		c.Priority = "medium"
	}
	if err := f.db.Create(&c).Error; err != nil {
		t.Fatal(err)
	}
	f.conversationsByLabel[label] = c.ID
	return c.ID
}

func (f *supportReportFixture) message(t *testing.T, conversationID, senderType string, userID *string, createdAt time.Time) {
	t.Helper()
	if err := f.db.Create(&model.SupportMessage{
		ID: uuid.NewString(), WorkspaceID: f.workspaceID, ConversationID: conversationID, SenderType: senderType,
		SenderUserID: userID, MessageType: "reply", Content: "message", CreatedAt: createdAt, UpdatedAt: createdAt,
	}).Error; err != nil {
		t.Fatal(err)
	}
}

func (f *supportReportFixture) rating(t *testing.T, conversationID string, rating int, comment *string, resolvedBy string, assigned *string, submitted time.Time) {
	t.Helper()
	if err := f.db.Create(&model.SupportCSATResponse{
		ID: uuid.NewString(), WorkspaceID: f.workspaceID, ConversationID: conversationID, Rating: rating, Comment: comment,
		Channel: model.SupportCSATChannelWidget, ResolvedBy: resolvedBy, AssignedUserID: assigned, SubmittedAt: submitted,
	}).Error; err != nil {
		t.Fatal(err)
	}
}

func newSupportReportFixture(t *testing.T) *supportReportFixture {
	t.Helper()
	return seedSupportReportFixture(t, newTestDB(t))
}

// seedSupportReportFixture seeds one week of support activity plus a
// previous week. The PostgreSQL integration test reuses it.
func seedSupportReportFixture(t *testing.T, db *gorm.DB) *supportReportFixture {
	t.Helper()
	f := &supportReportFixture{db: db, workspaceID: uuid.NewString(), conversationsByLabel: map[string]string{}}
	f.userOne, f.userTwo, f.memberID = uuid.NewString(), uuid.NewString(), uuid.NewString()
	seedUser(t, db, f.userOne, "one@example.com", "Uma One", "hash")
	seedUser(t, db, f.userTwo, "two@example.com", "Theo Two", "hash")
	if db.Dialector.Name() == "postgres" {
		mustExec(t, db, `INSERT INTO workspaces (id, name, slug, workspace_key, owner_id) VALUES (?, 'Reports', ?, ?, ?)`,
			f.workspaceID, "reports-"+f.workspaceID[:8], strings.ToUpper(f.workspaceID[:5]), f.userOne)
	} else {
		seedWorkspace(t, db, f.workspaceID, "Reports", "reports-"+f.workspaceID[:8], f.userOne)
	}
	seedWorkspaceMember(t, db, f.memberID, f.workspaceID, f.userTwo, "two@example.com", "Theo Two", model.RoleMember)
	f.restrictedMailboxID = uuid.NewString()
	mustExec(t, db, `INSERT INTO support_mailboxes (id, workspace_id, name, handle, created_by_id, active) VALUES (?, ?, 'VIP', 'vip', ?, true)`,
		f.restrictedMailboxID, f.workspaceID, f.userOne)
	f.svc = NewSupportReportService(repository.NewSupportConversationRepository(db))

	resolved, open, spam := model.SupportConversationStatusResolved, model.SupportConversationStatusOpen, model.SupportConversationStatusSpam
	byHuman, byAI := model.SupportConversationFlowStateResolvedByHuman, model.SupportConversationFlowStateResolvedByAI
	confirmed := "confirmed"
	escalatedAt := at(0, 10, 2)

	// c1: AI answers, hands off, a teammate replies after 10 minutes and resolves.
	c1 := f.conversation(t, "c1", model.SupportConversation{Status: resolved, FlowState: &byHuman, AssignedUserID: &f.userOne,
		CreatedAt: at(0, 10, 0), ResolvedAt: ptrTime(at(0, 12, 0)), AIEscalatedAt: &escalatedAt})
	f.message(t, c1, "customer", nil, at(0, 10, 0))
	f.message(t, c1, "ai", nil, at(0, 10, 0).Add(30*time.Second))
	f.message(t, c1, "user", &f.userOne, at(0, 10, 10))
	// c2: the AI resolves it on its own.
	c2 := f.conversation(t, "c2", model.SupportConversation{Status: resolved, FlowState: &byAI, AIResolutionType: &confirmed,
		CreatedAt: at(1, 9, 0), ResolvedAt: ptrTime(at(1, 9, 30))})
	f.message(t, c2, "customer", nil, at(1, 9, 0))
	f.message(t, c2, "ai", nil, at(1, 9, 1))
	// c3: an email waits two hours for a teammate and is still open.
	c3 := f.conversation(t, "c3", model.SupportConversation{Status: open, Channel: "email", Subject: "=HYPERLINK(\"x\")",
		CreatedAt: at(2, 8, 0), CustomerEmail: strPtr("ada@example.com")})
	f.message(t, c3, "customer", nil, at(2, 8, 0))
	f.message(t, c3, "user", &f.userTwo, at(2, 10, 0))
	// c4: a teammate starts the conversation, so it has no first response time.
	c4 := f.conversation(t, "c4", model.SupportConversation{Status: open, Channel: "internal", CreatedAt: at(3, 9, 0)})
	f.message(t, c4, "user", &f.userOne, at(3, 9, 0))
	f.message(t, c4, "customer", nil, at(3, 11, 0))
	// c5: created in the previous period, resolved in this one.
	c5 := f.conversation(t, "c5", model.SupportConversation{Status: resolved, FlowState: &byHuman, AssignedUserID: &f.userTwo,
		CreatedAt: at(-6, 9, 0), ResolvedAt: ptrTime(at(1, 9, 0))})
	f.message(t, c5, "customer", nil, at(-6, 9, 0))
	f.message(t, c5, "user", &f.userTwo, at(-6, 10, 0))
	// userOne resolved c5 although userTwo is the assignee.
	if err := f.db.Create(&model.SupportMessage{
		ID: uuid.NewString(), WorkspaceID: f.workspaceID, ConversationID: c5, SenderType: "user", SenderUserID: &f.userOne,
		MessageType: "system", SystemEventType: model.SupportSystemEventTypeStrPtr(model.SystemEventResolved),
		Content: "Uma resolved this conversation.", IsInternal: true, CreatedAt: at(1, 9, 0), UpdatedAt: at(1, 9, 0),
	}).Error; err != nil {
		t.Fatal(err)
	}
	// c6: spam is never reported.
	c6 := f.conversation(t, "c6", model.SupportConversation{Status: spam, CreatedAt: at(1, 12, 0)})
	f.message(t, c6, "customer", nil, at(1, 12, 0))
	// c7: in an inbox the member cannot open.
	c7 := f.conversation(t, "c7", model.SupportConversation{Status: open, MailboxID: &f.restrictedMailboxID, CreatedAt: at(4, 9, 0)})
	f.message(t, c7, "customer", nil, at(4, 9, 0))
	// c8: entirely in the previous period.
	c8 := f.conversation(t, "c8", model.SupportConversation{Status: resolved, FlowState: &byHuman, CreatedAt: at(-5, 9, 0), ResolvedAt: ptrTime(at(-5, 10, 0))})

	for _, row := range []model.SupportConversationSLARecord{
		{ConversationID: c1, FirstResponseState: model.SupportSLAStateAchieved, ResolutionState: model.SupportSLAStateBreached, NextResponseState: model.SupportSLAStateAchieved},
		{ConversationID: c3, FirstResponseState: model.SupportSLAStateBreached, NextResponseBreaches: 2, NextResponseState: model.SupportSLAStateBreached, ResolutionState: model.SupportSLAStatePending},
		{ConversationID: c7, FirstResponseState: model.SupportSLAStatePending},
		{ConversationID: c8, FirstResponseState: model.SupportSLAStateBreached}, // previous period
	} {
		row.WorkspaceID = f.workspaceID
		if err := f.db.Create(&row).Error; err != nil {
			t.Fatal(err)
		}
	}

	f.rating(t, c1, 5, strPtr("Great help"), model.SupportCSATResolvedByHuman, &f.userOne, at(0, 13, 0))
	f.rating(t, c2, 3, nil, model.SupportCSATResolvedByAI, nil, at(1, 10, 0))
	f.rating(t, c5, 1, strPtr("Took a week"), model.SupportCSATResolvedByHuman, &f.userTwo, at(1, 9, 30))
	f.rating(t, c8, 2, nil, model.SupportCSATResolvedByHuman, nil, at(-5, 11, 0))
	return f
}


func (f *supportReportFixture) request() model.SupportReportRequest {
	return model.SupportReportRequest{WorkspaceID: f.workspaceID, From: reportBase, To: reportBase.AddDate(0, 0, 7), Timezone: "UTC"}
}

func (f *supportReportFixture) ctx(role, memberID, userID string) context.Context {
	return authorization.WithActor(context.Background(), &authorization.Actor{
		UserID: userID, WorkspaceID: f.workspaceID, WorkspaceMemberID: memberID, Role: role,
	})
}

func assertSeconds(t *testing.T, name string, got *int64, want int64) {
	t.Helper()
	if got == nil || *got != want {
		t.Fatalf("%s = %v, want %d", name, got, want)
	}
}

func assertRate(t *testing.T, name string, got *float64, want float64) {
	t.Helper()
	if got == nil || *got != want {
		t.Fatalf("%s = %v, want %v", name, got, want)
	}
}

func TestSupportReportOverviewMetrics(t *testing.T) { checkSupportReportOverviewMetrics(t, newSupportReportFixture(t)) }

func checkSupportReportOverviewMetrics(t *testing.T, f *supportReportFixture) {
	t.Helper()
	overview, err := f.svc.Overview(f.ctx(model.RoleOwner, "", f.userOne), f.request())
	if err != nil {
		t.Fatal(err)
	}
	s := overview.Summary
	if s.ConversationsCreated != 5 || s.ConversationsResolved != 3 || s.ResolvedByAI != 1 || s.ResolvedByHuman != 2 {
		t.Fatalf("volume: %+v", s)
	}
	// First human response: c1 10m and c3 2h; c4 was started by a teammate.
	if s.FirstHumanResponse.Count != 2 {
		t.Fatalf("first human response count %d", s.FirstHumanResponse.Count)
	}
	assertSeconds(t, "first human median", s.FirstHumanResponse.MedianSeconds, (600+7200)/2)
	assertSeconds(t, "first human p90", s.FirstHumanResponse.P90Seconds, 7200)
	// Any first response counts the AI: 30s, 60s and 2h.
	assertSeconds(t, "first response median", s.FirstResponse.MedianSeconds, 60)
	assertSeconds(t, "first response p90", s.FirstResponse.P90Seconds, 7200)
	// Resolution: c2 30m, c1 2h, c5 seven days.
	assertSeconds(t, "resolution median", s.Resolution.MedianSeconds, 7200)
	assertSeconds(t, "resolution p90", s.Resolution.P90Seconds, int64((7*24*time.Hour)/time.Second))
	if s.AIInvolved != 2 || s.AIResolved != 1 || s.AIResolvedConfirm != 1 || s.AIHandoffs != 1 {
		t.Fatalf("ai: %+v", s)
	}
	assertRate(t, "involvement", s.AIInvolvementRate, 0.4)
	assertRate(t, "ai resolution", s.AIResolutionRate, 0.5)
	assertRate(t, "handoff", s.AIHandoffRate, 0.5)
	if s.CSATResponses != 3 {
		t.Fatalf("csat responses %d", s.CSATResponses)
	}
	assertRate(t, "csat average", s.CSATAverage, 3)
	assertRate(t, "csat positive", s.CSATPositiveRate, 0.3333)

	p := overview.Previous
	if p.ConversationsCreated != 2 || p.ConversationsResolved != 1 || p.CSATResponses != 1 {
		t.Fatalf("previous: %+v", p)
	}
	if !overview.Range.PreviousFrom.Equal(reportBase.AddDate(0, 0, -7)) {
		t.Fatalf("previous range %v", overview.Range)
	}
	if overview.Backlog.Open != 3 || overview.Backlog.Unassigned != 3 {
		t.Fatalf("backlog %+v", overview.Backlog)
	}
	if len(overview.Daily) != 7 || overview.Daily[0].Date != "2026-10-01" || overview.Daily[0].ConversationsCreated != 1 ||
		overview.Daily[1].ConversationsResolved != 2 || overview.Daily[1].ResolvedByAI != 1 {
		t.Fatalf("daily %+v", overview.Daily)
	}
	assertSeconds(t, "day one median", overview.Daily[0].MedianFirstHumanResponseS, 600)
	if len(overview.Channels) != 3 || overview.Channels[0].Channel != "widget" || overview.Channels[0].ConversationsCreated != 3 {
		t.Fatalf("channels %+v", overview.Channels)
	}
	csat := overview.CSAT
	if csat.Distribution != [5]int{1, 0, 1, 0, 1} || csat.AIResponses != 1 || csat.HumanResponses != 2 ||
		len(csat.RecentComments) != 2 || csat.RecentComments[0].Comment != "Took a week" {
		t.Fatalf("csat %+v", csat)
	}
	if sla := overview.SLA; sla.Conversations != 3 || sla.FirstResponse.Met != 1 || sla.FirstResponse.Missed != 1 ||
		sla.NextResponse.Met != 1 || sla.NextResponse.Missed != 1 || sla.Resolution.Missed != 1 || sla.Resolution.Met != 0 {
		t.Fatalf("sla %+v", overview.SLA)
	}
	assertRate(t, "first response hit rate", overview.SLA.FirstResponse.HitRate, 0.5)
	if len(overview.Teammates) != 2 {
		t.Fatalf("teammates %+v", overview.Teammates)
	}
	for _, mate := range overview.Teammates {
		switch mate.UserID {
		case f.userOne:
			if mate.Name != "Uma One" || mate.Resolved != 2 || mate.RepliesSent != 2 || mate.CSATResponses != 1 {
				t.Fatalf("user one %+v", mate)
			}
			assertSeconds(t, "user one frt", mate.FirstHumanResponse.MedianSeconds, 600)
			assertRate(t, "user one csat", mate.CSATAverage, 5)
		case f.userTwo:
			// c5's reply was sent in the previous period, and userOne resolved it.
			if mate.Resolved != 0 || mate.RepliesSent != 1 || mate.CSATResponses != 1 {
				t.Fatalf("user two %+v", mate)
			}
			assertSeconds(t, "user two frt", mate.FirstHumanResponse.MedianSeconds, 7200)
		default:
			t.Fatalf("unexpected teammate %+v", mate)
		}
	}
}

func TestSupportReportRespectsInboxAccess(t *testing.T) { checkSupportReportRespectsInboxAccess(t, newSupportReportFixture(t)) }

func checkSupportReportRespectsInboxAccess(t *testing.T, f *supportReportFixture) {
	t.Helper()
	overview, err := f.svc.Overview(f.ctx(model.RoleMember, f.memberID, f.userTwo), f.request())
	if err != nil {
		t.Fatal(err)
	}
	if overview.Summary.ConversationsCreated != 4 || overview.Backlog.Open != 2 {
		t.Fatalf("member sees restricted inbox: %+v %+v", overview.Summary, overview.Backlog)
	}
	req := f.request()
	req.MailboxIDs = []string{f.restrictedMailboxID}
	overview, err = f.svc.Overview(f.ctx(model.RoleOwner, "", f.userOne), req)
	if err != nil {
		t.Fatal(err)
	}
	if overview.Summary.ConversationsCreated != 1 {
		t.Fatalf("inbox filter: %+v", overview.Summary)
	}
	req.MailboxIDs = nil
	req.Channels = []string{"email"}
	overview, err = f.svc.Overview(f.ctx(model.RoleOwner, "", f.userOne), req)
	if err != nil {
		t.Fatal(err)
	}
	if overview.Summary.ConversationsCreated != 1 || overview.Summary.CSATResponses != 0 {
		t.Fatalf("channel filter: %+v", overview.Summary)
	}
}

func TestSupportReportRejectsInvalidRanges(t *testing.T) {
	f := newSupportReportFixture(t)
	ctx := f.ctx(model.RoleOwner, "", f.userOne)
	for name, req := range map[string]model.SupportReportRequest{
		"reversed":  {WorkspaceID: f.workspaceID, From: reportBase, To: reportBase.Add(-time.Hour)},
		"too long":  {WorkspaceID: f.workspaceID, From: reportBase, To: reportBase.AddDate(1, 1, 0)},
		"empty":     {WorkspaceID: f.workspaceID},
		"bad zone":  {WorkspaceID: f.workspaceID, From: reportBase, To: reportBase.AddDate(0, 0, 1), Timezone: "Mars/Base"},
		"no tenant": {From: reportBase, To: reportBase.AddDate(0, 0, 1)},
	} {
		if _, err := f.svc.Overview(ctx, req); err == nil {
			t.Fatalf("%s: accepted", name)
		} else if name != "bad zone" && !errors.Is(err, ErrSupportReportInvalidRange) {
			t.Fatalf("%s: %v", name, err)
		}
	}
}

func TestSupportReportDailyUsesTimezone(t *testing.T) { checkSupportReportDailyUsesTimezone(t, newSupportReportFixture(t)) }

func checkSupportReportDailyUsesTimezone(t *testing.T, f *supportReportFixture) {
	t.Helper()
	req := f.request()
	req.Timezone = "America/Los_Angeles"
	overview, err := f.svc.Overview(f.ctx(model.RoleOwner, "", f.userOne), req)
	if err != nil {
		t.Fatal(err)
	}
	// 2026-10-01T00:00Z is Sep 30 in Los Angeles, so the series starts there.
	if overview.Daily[0].Date != "2026-09-30" || overview.Range.Timezone != "America/Los_Angeles" {
		t.Fatalf("daily %+v", overview.Daily[0])
	}
}

func TestSupportReportCSVExport(t *testing.T) { checkSupportReportCSVExport(t, newSupportReportFixture(t)) }

func checkSupportReportCSVExport(t *testing.T, f *supportReportFixture) {
	t.Helper()
	var out bytes.Buffer
	if err := f.svc.ExportConversationsCSV(f.ctx(model.RoleOwner, "", f.userOne), f.request(), &out); err != nil {
		t.Fatal(err)
	}
	records, err := csv.NewReader(&out).ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 6 {
		t.Fatalf("rows %d: %v", len(records), records)
	}
	header := map[string]int{}
	for i, name := range records[0] {
		header[name] = i
	}
	byNumber := map[string][]string{}
	for _, record := range records[1:] {
		byNumber[record[header["conversation_number"]]] = record
	}
	first := byNumber["1"]
	if first[header["first_human_response_seconds"]] != "600" || first[header["csat_rating"]] != "5" ||
		first[header["assignee"]] != "Uma One" || first[header["resolved_by"]] != "human" || first[header["handed_off"]] != "true" {
		t.Fatalf("c1 row %v", first)
	}
	third := byNumber["3"]
	if third[header["subject"]] != `'=HYPERLINK("x")` || third[header["customer_email"]] != "ada@example.com" {
		t.Fatalf("c3 row %v", third)
	}
	if byNumber["4"][header["first_human_response_seconds"]] != "" {
		t.Fatalf("teammate-started conversation reported a first response: %v", byNumber["4"])
	}
}
