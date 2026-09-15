package service

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

type answerFeedbackRecorder struct{ events []SupportEventInput }

func (r *answerFeedbackRecorder) RecordEventBestEffort(input SupportEventInput) {
	r.events = append(r.events, input)
}

func TestWidgetAnswerFeedbackOwnershipPersistenceAndIdempotence(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	seedWorkspace(t, db, "w-feedback", "Feedback", "feedback", "user-feedback")
	conversations := repository.NewSupportConversationRepository(db)
	messages := repository.NewSupportMessageRepository(db)
	sessions := repository.NewSupportInboxSessionRepository(db)
	anon := "visitor-feedback"
	conv := &model.SupportConversation{WorkspaceID: "w-feedback", AnonymousID: &anon, Subject: "Question", Status: "resolved", UpdatedAt: time.Now().Add(-72 * time.Hour)}
	if err := conversations.Create(ctx, conv); err != nil {
		t.Fatal(err)
	}
	for _, session := range []model.SupportWidgetSession{
		{WorkspaceID: conv.WorkspaceID, SessionToken: "allowed", AnonymousID: anon, ExpiresAt: time.Now().Add(24 * time.Hour)},
		{WorkspaceID: conv.WorkspaceID, SessionToken: "other", AnonymousID: "someone-else", ExpiresAt: time.Now().Add(24 * time.Hour)},
		{WorkspaceID: conv.WorkspaceID, SessionToken: "expired", AnonymousID: anon, ExpiresAt: time.Now().Add(-time.Hour)},
	} {
		if err := sessions.Create(ctx, &session); err != nil {
			t.Fatal(err)
		}
	}
	message := &model.SupportMessage{WorkspaceID: conv.WorkspaceID, ConversationID: conv.ID, SenderType: "ai", MessageType: "reply", Content: "AI answer", Metadata: `{"ai_model":"internal-model"}`}
	if err := messages.Create(ctx, message); err != nil {
		t.Fatal(err)
	}
	recorder := &answerFeedbackRecorder{}
	svc := &SupportInboxService{conversationRepo: conversations, messageRepo: messages, sessionRepo: sessions, supportEventRecorder: recorder}
	for _, token := range []string{"other", "expired", "unknown"} {
		if _, err := svc.SubmitWidgetAnswerFeedback(ctx, token, message.ID, false); err == nil {
			t.Fatalf("unauthorized vote accepted: %s", token)
		}
	}
	feedback, err := svc.SubmitWidgetAnswerFeedback(ctx, "allowed", message.ID, false)
	if err != nil {
		t.Fatal(err)
	}
	if feedback.Helpful || feedback.SubmittedAt.IsZero() {
		t.Fatalf("wrong persisted vote: %+v", feedback)
	}
	repeated, err := svc.SubmitWidgetAnswerFeedback(ctx, "allowed", message.ID, true)
	if err != nil {
		t.Fatal(err)
	}
	if repeated.Helpful || !repeated.SubmittedAt.Equal(feedback.SubmittedAt) || len(recorder.events) != 1 {
		t.Fatal("retry overwrote or duplicated the first vote")
	}
	stored, err := messages.GetByID(ctx, message.ID)
	if err != nil {
		t.Fatal(err)
	}
	if stored.VisitorFeedback() == nil || !strings.Contains(stored.Metadata, "internal-model") {
		t.Fatal("vote or original metadata lost")
	}
	refreshed, err := conversations.GetByID(ctx, conv.WorkspaceID, conv.ID, "", model.RoleOwner)
	if err != nil {
		t.Fatal(err)
	}
	if refreshed.Status != "resolved" || refreshed.UpdatedAt.Before(feedback.SubmittedAt) {
		t.Fatal("feedback changed status or failed to schedule coverage reconsideration")
	}
	public, err := svc.ListWidgetConversationMessages(ctx, conv.WorkspaceID, conv.ID)
	if err != nil {
		t.Fatal(err)
	}
	wire, _ := json.Marshal(model.PublicWidgetMessages(public))
	if !strings.Contains(string(wire), "visitor_feedback") || strings.Contains(string(wire), "internal-model") {
		t.Fatalf("incorrect public feedback projection: %s", wire)
	}
	for _, test := range []struct {
		sender   string
		internal bool
		metadata string
	}{{"user", false, "{}"}, {"ai", true, "{}"}, {"ai", false, `{"delivery_mode":"email_only"}`}, {"ai", false, `{"delayed_team_reply":true}`}} {
		candidate := &model.SupportMessage{WorkspaceID: conv.WorkspaceID, ConversationID: conv.ID, SenderType: test.sender, MessageType: "reply", Content: "Not eligible", IsInternal: test.internal, Metadata: test.metadata}
		if err := messages.Create(ctx, candidate); err != nil {
			t.Fatal(err)
		}
		if _, err := svc.SubmitWidgetAnswerFeedback(ctx, "allowed", candidate.ID, true); err == nil {
			t.Fatalf("accepted ineligible vote %+v", test)
		}
	}
}

func TestAnswerFeedbackContextAppliesOnlyToNextCustomerTurn(t *testing.T) {
	base := time.Date(2026, 9, 13, 12, 0, 0, 0, time.UTC)
	vote, _ := json.Marshal(map[string]any{"visitor_feedback": model.SupportAnswerFeedback{Helpful: false, SubmittedAt: base.Add(2 * time.Minute)}})
	history := []model.SupportMessage{{ID: "c1", SenderType: "customer", CreatedAt: base}, {ID: "a1", SenderType: "ai", MessageType: "reply", Content: "Previous answer", Metadata: string(vote), CreatedAt: base.Add(time.Minute)}}
	next := model.SupportMessage{ID: "c2", SenderType: "customer", CreatedAt: base.Add(3 * time.Minute)}
	got := supportVisitorFeedbackContext(history, next)
	if !strings.Contains(got, "Previous answer") || !strings.Contains(got, "not proof") {
		t.Fatalf("missing bounded feedback context: %s", got)
	}
	history = append(history, next)
	if got := supportVisitorFeedbackContext(history, model.SupportMessage{ID: "c3", CreatedAt: base.Add(4 * time.Minute)}); got != "" {
		t.Fatalf("old vote repeated: %s", got)
	}
	history[1].Metadata = strings.ReplaceAll(string(vote), `false`, `true`)
	if got := supportVisitorFeedbackContext(history[:2], next); got != "" {
		t.Fatal("positive vote treated as correction")
	}
}

func TestCoverageAnalysisIncludesVotesAndChangesTranscriptHash(t *testing.T) {
	base := time.Date(2026, 9, 13, 12, 0, 0, 0, time.UTC)
	messages := []model.SupportMessage{{ID: "c", SenderType: "customer", MessageType: "reply", Content: "How do I export?", CreatedAt: base}, {ID: "a", SenderType: "ai", MessageType: "reply", Content: "Use export.", Metadata: "{}", CreatedAt: base.Add(time.Minute)}}
	conversation := model.SupportConversation{ID: "conv", WorkspaceID: "ws", Subject: "Export"}
	before, err := BuildCoverageConversationAnalysisInput(conversation, messages, nil)
	if err != nil {
		t.Fatal(err)
	}
	messages[1].Metadata = `{"visitor_feedback":{"helpful":false,"submitted_at":"2026-09-13T12:02:00Z"}}`
	after, err := BuildCoverageConversationAnalysisInput(conversation, messages, nil)
	if err != nil {
		t.Fatal(err)
	}
	if after.Messages[1].VisitorFeedback == nil || after.Messages[1].VisitorFeedback.Helpful || before.TranscriptHash == after.TranscriptHash {
		t.Fatal("coverage did not receive or invalidate on a negative vote")
	}
	if !strings.Contains(coverageConversationAnalysisSystemPrompt(), "Never create or close a gap solely because of a vote") {
		t.Fatal("missing coverage feedback safeguards")
	}
}

func TestCoverageVisitorVoteEnrichesExistingGapWithoutCreatingOne(t *testing.T) {
	eventSvc, coverage, db := setupCoverageTestEnv(t)
	ctx := context.Background()
	conv, message := "conv-vote", "answer-vote"
	vote := SupportEventInput{WorkspaceID: "ws-1", EventType: model.SupportEventAIAnswerFeedback, ConversationID: &conv, MessageID: &message, SourceSignal: "not_helpful"}
	if err := eventSvc.RecordEvent(ctx, vote); err != nil {
		t.Fatal(err)
	}
	var count int64
	db.Model(&model.SupportCoverageGap{}).Count(&count)
	if count != 0 {
		t.Fatal("thumbs-down created a gap")
	}
	if err := eventSvc.RecordEvent(ctx, SupportEventInput{WorkspaceID: "ws-1", EventType: model.SupportEventDocsIssueFeedback, ConversationID: &conv, IssueKey: "export", SourceSignal: model.SupportCoverageSourceAgentFeedback}); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if err := eventSvc.RecordEvent(ctx, vote); err != nil {
			t.Fatal(err)
		}
	}
	var evidence []model.SupportGapEvidence
	if err := db.Where("evidence_type = ?", model.SupportEventAIAnswerFeedback).Find(&evidence).Error; err != nil {
		t.Fatal(err)
	}
	if len(evidence) != 1 || evidence[0].MessageID == nil || *evidence[0].MessageID != message {
		t.Fatalf("vote evidence lost or duplicated: %+v", evidence)
	}
	vote.SourceSignal = "helpful"
	if err := eventSvc.RecordEvent(ctx, vote); err != nil {
		t.Fatal(err)
	}
	gaps, _, err := coverage.ListGaps(ctx, "ws-1", model.SupportCoverageGapFilter{ShowRaw: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(gaps) != 1 || gaps[0].Status == "resolved" {
		t.Fatal("positive vote changed gap lifecycle")
	}
	otherGap := model.SupportCoverageGap{ID: "other-gap", WorkspaceID: "ws-1", Status: model.SupportCoverageGapStatusOpen, IssueKey: "billing", Metadata: json.RawMessage(`{}`)}
	if err := db.Create(&otherGap).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.SupportGapEvidence{ID: "other-evidence", GapID: otherGap.ID, WorkspaceID: "ws-1", ConversationID: &conv, SourceKey: "billing", Metadata: json.RawMessage(`{}`)}).Error; err != nil {
		t.Fatal(err)
	}
	message = "another-answer"
	vote.SourceSignal = "not_helpful"
	if err := eventSvc.RecordEvent(ctx, vote); err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&model.SupportGapEvidence{}).Where("evidence_type = ?", model.SupportEventAIAnswerFeedback).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatal("ambiguous feedback was attached to an arbitrary gap")
	}
}
