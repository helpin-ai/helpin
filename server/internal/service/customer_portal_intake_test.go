package service

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

type capturingEventRecorder struct{ events []SupportEventInput }

func (r *capturingEventRecorder) RecordEventBestEffort(input SupportEventInput) {
	r.events = append(r.events, input)
}

func TestRecordIntakeEventValidation(t *testing.T) {
	svc, db, _, ws := setupCustomerPortalService(t)
	ctx := context.Background()
	intake, a, b := uuid.NewString(), uuid.NewString(), uuid.NewString()
	many := []string{uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString()}
	for _, tt := range []struct {
		name, intake, event string
		ids                 []string
	}{
		{"malformed intake", "not-a-uuid", model.PortalIntakeEventShown, []string{a}},
		{"unknown event", intake, "clicked", []string{a}},
		{"submissions come with the request", intake, model.PortalIntakeEventSubmitted, []string{a}},
		{"shown without articles", intake, model.PortalIntakeEventShown, nil},
		{"opened two articles", intake, model.PortalIntakeEventOpened, []string{a, b}},
		{"malformed article", intake, model.PortalIntakeEventOpened, []string{"article"}},
		{"too many articles", intake, model.PortalIntakeEventShown, many},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if err := svc.RecordIntakeEvent(ctx, ws, tt.intake, tt.event, tt.ids); !errors.Is(err, ErrPortalIntakeInvalid) {
				t.Fatalf("err = %v, want invalid", err)
			}
		})
	}
	if err := svc.RecordIntakeEvent(ctx, ws, intake, model.PortalIntakeEventShown, []string{a, b, a}); err != nil {
		t.Fatal(err)
	}
	if err := svc.RecordIntakeEvent(ctx, ws, intake, model.PortalIntakeEventOpened, []string{a}); err != nil {
		t.Fatal(err)
	}
	var shown model.SupportPortalIntakeEvent
	if err := db.Where("event_type = ?", model.PortalIntakeEventShown).Take(&shown).Error; err != nil {
		t.Fatal(err)
	}
	if shown.ArticleCount != 2 || shown.WorkspaceID != ws.ID {
		t.Fatalf("shown event = %+v (duplicates must collapse)", shown)
	}
}

func TestCreateRequestRecordsIntake(t *testing.T) {
	svc, db, _, ws := setupCustomerPortalService(t)
	ctx := context.Background()
	recorder := &capturingEventRecorder{}
	svc.inbox.SetSupportEventRecorder(recorder)
	setPortalContact(t, db, ws.ID, "customer@example.com", portalAccess(model.PortalAccessAllowed))
	access := signInPortal(t, svc, ws, issuePortalLink(t, db, ws.ID, "customer@example.com", time.Now().Add(-time.Hour)))
	suggested, opened := uuid.NewString(), uuid.NewString()

	bad := &model.PortalIntake{IntakeID: "nope"}
	if _, err := svc.CreateRequest(ctx, access, "Export fails", "On Safari", nil, bad); !errors.Is(err, ErrPortalIntakeInvalid) {
		t.Fatalf("malformed intake err = %v", err)
	}

	intake := &model.PortalIntake{IntakeID: uuid.NewString(), SuggestedArticleIDs: []string{suggested, opened}, OpenedArticleIDs: []string{opened}, SearchCompleted: true}
	request, err := svc.CreateRequest(ctx, access, "Export fails", "On Safari", nil, intake)
	if err != nil {
		t.Fatal(err)
	}
	var conversationID string
	if err := db.Table("support_portal_request_references").Where("reference = ?", request.Reference).Pluck("conversation_id", &conversationID).Error; err != nil {
		t.Fatal(err)
	}
	var first model.SupportMessage
	if err := db.Where("conversation_id = ? AND sender_type = 'customer'", conversationID).Take(&first).Error; err != nil {
		t.Fatal(err)
	}
	var metadata struct {
		Identity string             `json:"portal_identity_id"`
		Intake   model.PortalIntake `json:"portal_intake"`
	}
	if err := json.Unmarshal([]byte(first.Metadata), &metadata); err != nil {
		t.Fatal(err)
	}
	if metadata.Identity != access.Identity.ID || len(metadata.Intake.OpenedArticleIDs) != 1 || metadata.Intake.OpenedArticleIDs[0] != opened {
		t.Fatalf("first message metadata = %s", first.Metadata)
	}
	var submitted model.SupportPortalIntakeEvent
	if err := db.Where("event_type = ?", model.PortalIntakeEventSubmitted).Take(&submitted).Error; err != nil {
		t.Fatal(err)
	}
	if submitted.IntakeID != intake.IntakeID || submitted.ArticleCount != 2 || !submitted.SearchCompleted || derefString(submitted.ConversationID) != conversationID {
		t.Fatalf("submitted event = %+v", submitted)
	}
	if len(recorder.events) != 0 {
		t.Fatalf("a request with suggestions reported a coverage gap: %+v", recorder.events)
	}

	portalContext, err := svc.ConversationPortalContext(ctx, ws.ID, conversationID)
	if err != nil {
		t.Fatal(err)
	}
	if portalContext.Intake == nil || portalContext.Intake.SuggestedCount != 2 || portalContext.Company != nil {
		t.Fatalf("portal context = %+v", portalContext)
	}
}

// A request written after the help center suggested nothing is a
// missing-article signal for coverage review.
func TestCreateRequestWithoutSuggestionsSignalsCoverageGap(t *testing.T) {
	svc, db, _, ws := setupCustomerPortalService(t)
	ctx := context.Background()
	recorder := &capturingEventRecorder{}
	svc.inbox.SetSupportEventRecorder(recorder)
	setPortalContact(t, db, ws.ID, "customer@example.com", portalAccess(model.PortalAccessAllowed))
	access := signInPortal(t, svc, ws, issuePortalLink(t, db, ws.ID, "customer@example.com", time.Now().Add(-time.Hour)))
	if _, err := svc.CreateRequest(ctx, access, "Webhook retries are missing", "We never see retries", nil, &model.PortalIntake{IntakeID: uuid.NewString(), SearchCompleted: true}); err != nil {
		t.Fatal(err)
	}
	if len(recorder.events) != 1 {
		t.Fatalf("coverage events = %+v", recorder.events)
	}
	event := recorder.events[0]
	if event.EventType != model.SupportEventWidgetSearchPerformed || event.SourceSignal != "no_results" || event.Channel != "portal" || event.IssueSummary != "Webhook retries are missing" || event.ConversationID == nil {
		t.Fatalf("coverage event = %+v", event)
	}
	if classifyEvent(&model.SupportEvent{EventType: event.EventType, SourceSignal: event.SourceSignal, IssueSummary: event.IssueSummary}) == nil {
		t.Fatal("coverage review ignores the portal signal")
	}

	// Without an intake the form never searched; nothing is reported.
	if _, err := svc.CreateRequest(ctx, access, "Another question", "No intake", nil, nil); err != nil {
		t.Fatal(err)
	}
	if len(recorder.events) != 1 {
		t.Fatalf("a request without intake reported a gap: %+v", recorder.events)
	}
	// Skipped, failed, and still-pending searches carry an intake but no
	// completion evidence. They must not be treated as empty search results.
	if _, err := svc.CreateRequest(ctx, access, "Search never completed", "No search result", nil, &model.PortalIntake{IntakeID: uuid.NewString()}); err != nil {
		t.Fatal(err)
	}
	if len(recorder.events) != 1 {
		t.Fatalf("an incomplete search reported a gap: %+v", recorder.events)
	}
}

func TestReportPortalIntake(t *testing.T) {
	db := newTestDB(t)
	repo := repository.NewSupportConversationRepository(db)
	ctx := context.Background()
	now := time.Date(2026, 10, 27, 12, 0, 0, 0, time.UTC)
	from, to := now.Add(-24*time.Hour), now
	if summary, err := repo.ReportPortalIntake(ctx, "ws", from, to, now); err != nil || summary != nil {
		t.Fatalf("empty period = %+v, %v", summary, err)
	}
	add := func(intake, event string, articles int, at time.Time) {
		t.Helper()
		mustExec(t, db, `INSERT INTO support_portal_intake_events (id, workspace_id, intake_id, event_type, article_count, search_completed, occurred_at) VALUES (?, 'ws', ?, ?, ?, ?, ?)`,
			uuid.NewString(), intake, event, articles, intake == "nothing-found", at)
	}
	hour := func(h int) time.Time { return now.Add(time.Duration(-h) * time.Hour) }
	// Deflected: shown, opened, never submitted.
	add("deflected", "shown", 3, hour(5))
	add("deflected", "opened", 1, hour(5).Add(time.Minute))
	// Submitted after opening.
	add("read-then-asked", "shown", 2, hour(4))
	add("read-then-asked", "opened", 1, hour(4).Add(time.Minute))
	add("read-then-asked", "submitted", 2, hour(4).Add(5*time.Minute))
	// Nothing found, submitted.
	add("nothing-found", "submitted", 0, hour(3))
	// A submission with no evidence of a successful search is unknown.
	add("search-not-completed", "submitted", 0, hour(3))
	// Opened moments ago: still inside the deflection window.
	add("still-reading", "shown", 1, now.Add(-10*time.Minute))
	add("still-reading", "opened", 1, now.Add(-5*time.Minute))
	// Started before the period.
	add("earlier", "shown", 1, hour(30))
	add("earlier", "opened", 1, hour(30))
	// Another workspace.
	mustExec(t, db, `INSERT INTO support_portal_intake_events (id, workspace_id, intake_id, event_type, article_count, occurred_at) VALUES (?, 'other', 'x', 'opened', 1, ?)`, uuid.NewString(), hour(1))

	summary, err := repo.ReportPortalIntake(ctx, "ws", from, to, now)
	if err != nil {
		t.Fatal(err)
	}
	want := model.SupportReportPortalIntake{WithSuggestions: 3, Opened: 3, SubmittedAfterOpening: 1, LikelyDeflected: 1, SubmittedWithoutSuggestions: 1}
	if summary == nil || *summary != want {
		t.Fatalf("summary = %+v, want %+v", summary, want)
	}
}
