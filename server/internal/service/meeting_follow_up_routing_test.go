package service

import (
	"context"
	"strings"
	"testing"

	f "github.com/helpin-ai/helpin/server/internal/crmsituationtest"
	"github.com/helpin-ai/helpin/server/internal/llm"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

func TestMeetingFollowUpRoutingRequiresTranscriptEvidence(t *testing.T) {
	transcript := "Maya: Our internal sprint planning needs a separate engineering follow-up. Legal will review Acme's renewal contract."
	for _, tc := range []struct{ name, scope, evidence, want string }{
		{"internal supported", "internal", "Our internal sprint planning needs a separate engineering follow-up.", "internal"},
		{"customer supported", "customer", "Legal will review Acme's renewal contract.", "customer"},
		{"missing evidence", "internal", "", "uncertain"},
		{"fabricated evidence", "internal", "Everyone confirmed there was no customer work.", "uncertain"},
		{"unrecognized label", "team", "Our internal sprint planning needs a separate engineering follow-up.", "uncertain"},
		{"uncertain stays visible", "uncertain", "", "uncertain"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := validatedMeetingFollowUpScope(tc.scope, tc.evidence, transcript); got != tc.want {
				t.Fatalf("scope = %q, want %q", got, tc.want)
			}
		})
	}
}

type routingBackfillStore struct {
	items  []model.CRMSuggestion
	writes map[string]string
	limit  int
}

func (s *routingBackfillStore) MeetingFollowUpsToRoute(_ context.Context, limit int) ([]model.CRMSuggestion, error) {
	s.limit = limit
	return s.items, nil
}
func (s *routingBackfillStore) RouteMeetingFollowUp(_ context.Context, item model.CRMSuggestion, scope string) error {
	s.writes[item.ID] = scope
	return nil
}

func TestMeetingFollowUpBackfillKeepsTransientFailuresRetryable(t *testing.T) {
	db := f.Open(t)
	f.Exec(t, db, `CREATE TABLE crm_meeting_transcripts (id TEXT PRIMARY KEY, workspace_id TEXT, meeting_id TEXT, plain_text TEXT)`)
	f.Exec(t, db, `INSERT INTO crm_meeting_transcripts VALUES ('transcript','ws','meeting','Our internal sprint planning needs a separate engineering follow-up.')`)
	meeting, missing := "meeting", "missing"
	store := &routingBackfillStore{items: []model.CRMSuggestion{
		{ID: "failure", WorkspaceID: "ws", ObjectID: &meeting, Title: "Sprint planning"},
		{ID: "classified", WorkspaceID: "ws", ObjectID: &meeting, Title: "Sprint planning"},
		{ID: "missing-transcript", WorkspaceID: "ws", ObjectID: &missing, Title: "Sprint planning"},
	}, writes: map[string]string{}}
	provider := &scriptedMeetingIntelligenceLLM{results: []meetingIntelligenceLLMResult{
		{response: &llm.ChatResponse{Content: `{"scope":"bad"}`}},
		{response: &llm.ChatResponse{Content: `{"scope":"internal","scope_evidence":"Our internal sprint planning needs a separate engineering follow-up."}`}},
	}}
	service := &CRMMeetingProcessingService{llmProvider: provider, repo: repository.NewCRMMeetingRepository(db), followUpRouting: store}
	if err := service.BackfillFollowUpRouting(context.Background()); err != nil {
		t.Fatal(err)
	}
	if store.limit != 3 || len(store.writes) != 1 || store.writes["classified"] != "internal" {
		t.Fatalf("backfill mutated failed/missing records: %+v", store)
	}
}

func TestMeetingFollowUpRoutingUsesDraftAndMeteredTranscriptClassification(t *testing.T) {
	provider := &scriptedMeetingIntelligenceLLM{results: []meetingIntelligenceLLMResult{{response: &llm.ChatResponse{Content: `{"scope":"customer","scope_evidence":"Legal will review Acme's renewal contract."}`}}}}
	service := &CRMMeetingProcessingService{llmProvider: provider}
	meetingID, description := "meeting-1", "Ask our legal team to approve Acme's contract."
	item := model.CRMSuggestion{ID: "draft-1", WorkspaceID: "ws", ObjectID: &meetingID, Title: "Legal review", Description: &description}
	transcript := &model.CRMMeetingTranscript{PlainText: "Legal will review Acme's renewal contract. Also arrange our engineering sprint planning."}
	scope, err := service.classifyExistingFollowUp(context.Background(), item, transcript)
	if err != nil || scope != "customer" {
		t.Fatalf("scope=%q err=%v", scope, err)
	}
	if len(provider.requests) != 1 || len(provider.metering) != 1 {
		t.Fatalf("classification not metered: %#v", provider.metering)
	}
	request := provider.requests[0]
	if !strings.Contains(request.Messages[0].Content, description) || !strings.Contains(request.Messages[0].Content, transcript.PlainText) {
		t.Fatal("classifier did not receive action and meeting evidence")
	}
	if !strings.Contains(request.SystemPrompt, "Missing customer names or linked IDs NEVER proves internal") || !strings.Contains(request.SystemPrompt, "Mixed internal and customer follow-up is customer") {
		t.Fatal("classification lost conservative action-level contract")
	}
}

type routingSuggestionManager struct {
	existing []model.CRMSuggestion
	filter   model.CRMSuggestionListFilters
	created  []model.CreateCRMSuggestionRequest
}

func (m *routingSuggestionManager) List(_ context.Context, _ string, filters model.CRMSuggestionListFilters, _ model.PMPagination) ([]model.CRMSuggestion, int64, error) {
	m.filter = filters
	return m.existing, int64(len(m.existing)), nil
}
func (m *routingSuggestionManager) Create(_ context.Context, req model.CreateCRMSuggestionRequest) (*model.CRMSuggestion, error) {
	m.created = append(m.created, req)
	return &model.CRMSuggestion{}, nil
}

func TestMeetingFollowUpProjectionClassifiesWithoutDuplicatingInternalDraft(t *testing.T) {
	manager := &routingSuggestionManager{}
	service := &CRMMeetingProcessingService{suggestions: manager}
	meeting := &model.CRMMeeting{ID: "meeting", WorkspaceID: "workspace", Title: "Engineering planning"}
	transcript := &model.CRMMeetingTranscript{PlainText: "Our internal sprint planning needs a separate engineering follow-up."}
	output := &meetingIntelligenceOutput{FollowUpDraft: meetingFollowUpOutput{Subject: "Sprint planning", Body: "Arrange our engineering sprint planning.", Scope: "internal", ScopeEvidence: transcript.PlainText}}
	if err := service.projectFollowUp(context.Background(), meeting, transcript, output); err != nil {
		t.Fatal(err)
	}
	if len(manager.created) != 1 || manager.created[0].Context[model.MeetingFollowUpScopeKey] != "internal" || !manager.filter.IncludeInternalMeetingFollowUps {
		t.Fatalf("projection contract: %+v", manager)
	}
	manager.existing = []model.CRMSuggestion{{ID: "already-internal"}}
	if err := service.projectFollowUp(context.Background(), meeting, transcript, output); err != nil {
		t.Fatal(err)
	}
	if len(manager.created) != 1 {
		t.Fatal("reprocessing duplicated an internal follow-up")
	}
}

func TestMeetingFollowUpRoutingAcceptsExactDraftEvidenceAcrossTranscriptLanguages(t *testing.T) {
	draft := "Hi Saira, please compile competitor landing pages so we can improve our campaign conversion rates."
	for _, tc := range []struct{ evidence, want string }{
		{"please compile competitor landing pages so we can improve our campaign conversion rates.", "internal"},
		{"Everyone confirmed this was an internal meeting.", "uncertain"},
	} {
		got := validatedMeetingFollowUpScope("internal", tc.evidence, "ham landing pages pe kaam karenge", draft)
		if got != tc.want {
			t.Fatalf("scope %q, want %q", got, tc.want)
		}
	}
}

func TestMeetingFollowUpClassifierRetriesInvalidEvidenceInsteadOfFinalizingUncertain(t *testing.T) {
	provider := &scriptedMeetingIntelligenceLLM{results: []meetingIntelligenceLLMResult{{response: &llm.ChatResponse{Content: `{"scope":"internal","scope_evidence":"Everyone confirmed the recap was only internal work."}`}}}}
	processor := &CRMMeetingProcessingService{llmProvider: provider}
	meeting := "meeting"
	item := model.CRMSuggestion{ID: "draft", WorkspaceID: "ws", ObjectID: &meeting, Title: "Team planning", Context: model.JSONB{"draft_body": "Please prepare the next sprint backlog."}}
	_, err := processor.classifyExistingFollowUp(context.Background(), item, &model.CRMMeetingTranscript{PlainText: "Prepare the next sprint backlog for the team."})
	if err == nil {
		t.Fatal("unsupported evidence was persisted as a completed uncertain classification")
	}
}

func TestMeetingFollowUpProjectionLeavesInvalidEvidenceForAutomaticRetry(t *testing.T) {
	manager := &routingSuggestionManager{}
	processor := &CRMMeetingProcessingService{suggestions: manager}
	meeting := &model.CRMMeeting{ID: "meeting", WorkspaceID: "workspace", Title: "Team planning"}
	transcript := &model.CRMMeetingTranscript{PlainText: "Prepare the next sprint backlog."}
	output := &meetingIntelligenceOutput{FollowUpDraft: meetingFollowUpOutput{Subject: "Team planning", Body: "Please prepare the sprint backlog.", Scope: "internal", ScopeEvidence: "Everyone confirmed this was an internal team meeting."}}
	if err := processor.projectFollowUp(context.Background(), meeting, transcript, output); err != nil {
		t.Fatal(err)
	}
	if len(manager.created) != 1 {
		t.Fatal("draft was lost")
	}
	metadata := manager.created[0].Context
	if metadata[model.MeetingFollowUpScopeKey] != "uncertain" || metadata[model.MeetingFollowUpRoutingVersionKey] != nil {
		t.Fatalf("invalid evidence was marked final: %+v", metadata)
	}
}
