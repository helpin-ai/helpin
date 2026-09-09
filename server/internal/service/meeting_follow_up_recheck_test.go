package service

import (
	"context"
	"github.com/helpin-ai/helpin/server/internal/authorization"
	f "github.com/helpin-ai/helpin/server/internal/crmsituationtest"
	"github.com/helpin-ai/helpin/server/internal/llm"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"testing"
)

type recheckStore struct {
	items         []model.MeetingFollowUpRoutingCandidate
	calls, writes int
	changed       bool
}

func (s *recheckStore) MeetingFollowUpsToRecheck(context.Context, string, string, string) ([]model.MeetingFollowUpRoutingCandidate, error) {
	s.calls++
	return s.items, nil
}
func (s *recheckStore) RouteMeetingFollowUpForActor(context.Context, model.CRMSuggestion, string, string) (bool, error) {
	s.writes++
	return s.changed, nil
}
func recheckContext() context.Context {
	return authorization.WithActor(context.Background(), &authorization.Actor{UserID: f.SalesUser, WorkspaceID: f.Workspace, WorkspaceMemberID: f.Sales, Status: "active"})
}
func TestMeetingFollowUpRecheckReportsOutcomesWithoutWorker(t *testing.T) {
	db := f.Open(t)
	f.Exec(t, db, `CREATE TABLE crm_meeting_transcripts (id TEXT PRIMARY KEY, workspace_id TEXT, meeting_id TEXT, plain_text TEXT)`)
	f.Exec(t, db, `INSERT INTO crm_meeting_transcripts VALUES ('t',?,'meeting','Our internal sprint planning needs an engineering follow-up.')`, f.Workspace)
	meeting, missing := "meeting", "missing"
	store := &recheckStore{changed: true, items: []model.MeetingFollowUpRoutingCandidate{
		{CRMSuggestion: model.CRMSuggestion{ID: "one", WorkspaceID: f.Workspace, ObjectID: &meeting}, RoutingEligible: true},
		{CRMSuggestion: model.CRMSuggestion{ID: "two", WorkspaceID: f.Workspace, ObjectID: &missing}, RoutingEligible: true},
		{CRMSuggestion: model.CRMSuggestion{ID: "three", WorkspaceID: f.Workspace, ObjectID: &meeting}, RoutingEligible: false},
		{CRMSuggestion: model.CRMSuggestion{ID: "four", WorkspaceID: f.Workspace, ObjectID: &meeting}, RoutingEligible: true},
	}}
	provider := &scriptedMeetingIntelligenceLLM{results: []meetingIntelligenceLLMResult{{response: &llm.ChatResponse{Content: `{"scope":"internal","scope_evidence":"Our internal sprint planning needs an engineering follow-up."}`}}}}
	processor := &CRMMeetingProcessingService{repo: repository.NewCRMMeetingRepository(db), llmProvider: provider}
	svc := NewMeetingFollowUpRecheckService(store, processor)
	result, err := svc.Recheck(recheckContext(), f.Workspace, "")
	if err != nil || len(result.Items) != 3 || result.NextCursor != "three" {
		t.Fatalf("result %+v %v", result, err)
	}
	for i, want := range []string{"internal", "missing_transcript", "not_eligible"} {
		if result.Items[i].Outcome != want {
			t.Fatalf("item %d: %+v", i, result.Items[i])
		}
	}
	if store.writes != 1 {
		t.Fatalf("writes %d", store.writes)
	}
}
func TestMeetingFollowUpRecheckRejectsInvalidScopeBeforeReading(t *testing.T) {
	store := &recheckStore{}
	svc := NewMeetingFollowUpRecheckService(store, nil)
	for _, tc := range []struct {
		ctx        context.Context
		ws, cursor string
	}{{context.Background(), f.Workspace, ""}, {recheckContext(), f.ForeignWorkspace, ""}, {recheckContext(), f.Workspace, "invalid"}} {
		if _, err := svc.Recheck(tc.ctx, tc.ws, tc.cursor); err == nil {
			t.Fatal("invalid request accepted")
		}
	}
	if store.calls != 0 {
		t.Fatal("unauthorized scan")
	}
}

func TestMeetingFollowUpRecheckPreservesProgressOnBillingFailure(t *testing.T) {
	db := f.Open(t)
	f.Exec(t, db, `CREATE TABLE crm_meeting_transcripts (id TEXT PRIMARY KEY, workspace_id TEXT, meeting_id TEXT, plain_text TEXT)`)
	f.Exec(t, db, `INSERT INTO crm_meeting_transcripts VALUES ('t',?,'meeting','Our internal sprint planning needs an engineering follow-up.')`, f.Workspace)
	meeting := "meeting"
	store := &recheckStore{changed: true}
	for _, id := range []string{"first", "second", "third"} {
		store.items = append(store.items, model.MeetingFollowUpRoutingCandidate{CRMSuggestion: model.CRMSuggestion{ID: id, WorkspaceID: f.Workspace, ObjectID: &meeting}, RoutingEligible: true})
	}
	provider := &scriptedMeetingIntelligenceLLM{results: []meetingIntelligenceLLMResult{
		{response: &llm.ChatResponse{Content: `{"scope":"internal","scope_evidence":"Our internal sprint planning needs an engineering follow-up."}`}},
		{err: model.ErrAIUsageExhausted},
	}}
	processor := &CRMMeetingProcessingService{repo: repository.NewCRMMeetingRepository(db), llmProvider: provider}
	result, err := NewMeetingFollowUpRecheckService(store, processor).Recheck(recheckContext(), f.Workspace, "")
	if err == nil || result == nil || len(result.Items) != 1 || result.Items[0].Outcome != "internal" || result.NextCursor != "first" || store.writes != 1 {
		t.Fatalf("partial result %+v err %v writes %d", result, err, store.writes)
	}
}

func TestMeetingFollowUpRecheckKeepsCustomerUncertainAndChangedWork(t *testing.T) {
	for _, tc := range []struct {
		name, scope, outcome string
		changed              bool
		providerErr          error
	}{
		{"customer", "customer", "customer", true, nil},
		{"uncertain", "uncertain", "uncertain", true, nil},
		{"concurrent change", "internal", "changed", false, nil},
		{"provider failure", "", "retry_needed", false, context.DeadlineExceeded},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db := f.Open(t)
			f.Exec(t, db, `CREATE TABLE crm_meeting_transcripts (id TEXT PRIMARY KEY, workspace_id TEXT, meeting_id TEXT, plain_text TEXT)`)
			f.Exec(t, db, `INSERT INTO crm_meeting_transcripts VALUES ('t',?,'meeting','Our follow-up needs careful review before sending.')`, f.Workspace)
			mid := "meeting"
			store := &recheckStore{changed: tc.changed, items: []model.MeetingFollowUpRoutingCandidate{{CRMSuggestion: model.CRMSuggestion{ID: "id", WorkspaceID: f.Workspace, ObjectID: &mid}, RoutingEligible: true}}}
			provider := &scriptedMeetingIntelligenceLLM{results: []meetingIntelligenceLLMResult{{response: &llm.ChatResponse{Content: `{"scope":"` + tc.scope + `","scope_evidence":"Our follow-up needs careful review before sending."}`}, err: tc.providerErr}}}
			processor := &CRMMeetingProcessingService{repo: repository.NewCRMMeetingRepository(db), llmProvider: provider}
			result, err := NewMeetingFollowUpRecheckService(store, processor).Recheck(recheckContext(), f.Workspace, "")
			if err != nil || len(result.Items) != 1 || result.Items[0].Outcome != tc.outcome {
				t.Fatalf("outcome %+v %v", result, err)
			}
			if tc.providerErr != nil && store.writes != 0 {
				t.Fatal("persisted transient failure")
			}
		})
	}
}
