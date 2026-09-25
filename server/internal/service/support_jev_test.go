package service

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/helpin-ai/helpin/server/internal/decision"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

type fakeJev struct {
	calls       int
	probability float64
	err         error
	states      []string
}

func (f *fakeJev) DecideMany(_ context.Context, state string, questions map[string]decision.Question) (*decision.Result, error) {
	f.calls++
	f.states = append(f.states, state)
	if f.err != nil {
		return nil, f.err
	}
	r := &decision.Result{Model: decision.Model, InputTokens: 20, OutputTokens: 10, Answers: map[string]decision.Answer{}}
	for name, q := range questions {
		selected := "sales"
		if name != "mailbox" {
			selected = "yes"
		}
		ps := map[string]float64{}
		for id := range q.Choices {
			ps[id] = (1 - f.probability) / float64(len(q.Choices)-1)
		}
		ps[selected] = f.probability
		r.Answers[name] = decision.Answer{Choice: selected, Probabilities: ps, ProviderConfidence: .2}
	}
	return r, nil
}
func attachJev(t *testing.T, f *supportTriageTestFixture, provider *fakeJev, routeMode, tagMode string) *SupportTagService {
	t.Helper()
	if err := f.db.AutoMigrate(&model.AIExecutionUsage{}, &model.SupportTag{}, &model.SupportConversationTag{}); err != nil {
		t.Fatal(err)
	}
	tags := NewSupportTagService(repository.NewSupportTagRepository(f.db), f.conversationRepo, nil).SetMessageRepo(f.messageRepo)
	svc, err := NewSupportJevService(SupportJevConfig{RoutingMode: routeMode, TagsMode: tagMode, RoutingThreshold: .9, TagThreshold: .95, HandoffThreshold: .95, FollowUpThreshold: .95, DailyLimit: 100}, provider, repository.NewSupportJevRepository(f.db), repository.NewAIExecutionUsageRepository(f.db), tags)
	if err != nil {
		t.Fatal(err)
	}
	f.triageSvc.SetJevService(svc)
	return tags
}
func TestJevRoutingFallback(t *testing.T) {
	cases := []struct {
		name, mode string
		p          float64
		err        error
		llmCalls   int
	}{
		{name: "default primary", p: .98, llmCalls: 0}, {name: "uncertain", p: .7, llmCalls: 1}, {name: "shadow", mode: "shadow", p: .99, llmCalls: 1}, {name: "off", mode: "off", p: .99, llmCalls: 1}, {name: "provider error", p: .99, err: errors.New("unavailable"), llmCalls: 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			llm := &scriptedSupportTriageLLM{response: `{"intent":"sales_pricing","target_mailbox_handle":"sales","confidence":0.82,"reason":"pricing"}`}
			f := newSupportTriageTestFixture(t, llm, nil)
			sales := f.createMailbox(t, "Sales", "sales", true)
			p := &fakeJev{probability: tc.p, err: tc.err}
			attachJev(t, f, p, tc.mode, "off")
			conv := f.createConversation(t, "pricing", "test@example.com", nil)
			msg := f.createCustomerReply(t, conv.ID, "What does the enterprise plan cost?")
			r, err := f.triageSvc.EvaluateAndRoute(f.ctx, f.workspaceID, conv.ID, msg.ID)
			if err != nil {
				t.Fatal(err)
			}
			if r == nil || derefString(r.SuggestedMailboxID) != sales.ID || llm.calls != tc.llmCalls {
				t.Fatalf("routing=%+v llm calls=%d", r, llm.calls)
			}
			if tc.llmCalls == 0 && derefFloat64(r.Confidence) < .9 {
				t.Fatal("concentration used as probability")
			}
		})
	}
}
func TestJevTagsPreserveHumanRemoval(t *testing.T) {
	f := newSupportTriageTestFixture(t, nil, nil)
	p := &fakeJev{probability: .99}
	tags := attachJev(t, f, p, "off", "")
	tag, err := tags.Create(f.ctx, f.workspaceID, model.CreateSupportTagRequest{Name: "Pricing"})
	if err != nil {
		t.Fatal(err)
	}
	conv := f.createConversation(t, "pricing", "test@example.com", nil)
	msg := f.createCustomerReply(t, conv.ID, "Please explain enterprise pricing")
	if _, err := f.triageSvc.EvaluateAndRoute(f.ctx, f.workspaceID, conv.ID, msg.ID); err != nil {
		t.Fatal(err)
	}
	linked, err := tags.conversationHasTag(f.ctx, f.workspaceID, conv.ID, tag.ID)
	if err != nil || !linked {
		t.Fatalf("tag missing: %v", err)
	}
	if err := tags.RemoveConversationTag(f.ctx, f.workspaceID, conv.ID, tag.ID, f.actorID); err != nil {
		t.Fatal(err)
	}
	msg = f.createCustomerReply(t, conv.ID, "I still have a pricing question")
	if _, err := f.triageSvc.EvaluateAndRoute(f.ctx, f.workspaceID, conv.ID, msg.ID); err != nil {
		t.Fatal(err)
	}
	linked, err = tags.conversationHasTag(f.ctx, f.workspaceID, conv.ID, tag.ID)
	if err != nil || linked || p.calls != 1 {
		t.Fatalf("manual removal overridden, calls %d err %v", p.calls, err)
	}
	for _, state := range p.states {
		if strings.Contains(state, "test@example.com") {
			t.Fatal("identity leaked from metadata")
		}
	}
}

func TestJevAdmissionDedupBudgetAndWorkspace(t *testing.T) {
	f := newSupportTriageTestFixture(t, nil, nil)
	p := &fakeJev{probability: .99}
	attachJev(t, f, p, "primary", "off")
	s := f.triageSvc.jev
	s.config.DailyLimit = 1
	s.workspaces = map[string]bool{f.workspaceID: true}
	conv := f.createConversation(t, "test", "test@example.com", nil)
	q := map[string]decision.Question{"mailbox": {Instructions: "route", Choices: map[string]string{"sales": "Sales", "shared": "Other"}}}
	for _, input := range []struct{ workspace, state string }{{f.workspaceID, "one"}, {f.workspaceID, "one"}, {f.workspaceID, "two"}, {"outside", "three"}} {
		if _, err := s.evaluate(f.ctx, input.workspace, conv.ID, input.state, "test", q); err != nil {
			t.Fatal(err)
		}
	}
	if p.calls != 1 {
		t.Fatalf("budget/dedup/workspace guard: %d calls", p.calls)
	}
	var usage int64
	if err := f.db.Model(&model.AIExecutionUsage{}).Count(&usage).Error; err != nil || usage != 1 {
		t.Fatalf("usage rows=%d err=%v", usage, err)
	}
}

func TestJevTagModesAndHumanOwnership(t *testing.T) {
	for _, tc := range []struct {
		name, mode string
		p          float64
		want       bool
		calls      int
	}{{"primary human owned", "primary", .99, true, 1}, {"shadow", "shadow", .99, false, 1}, {"off", "off", .99, false, 0}, {"uncertain", "primary", .8, false, 1}} {
		t.Run(tc.name, func(t *testing.T) {
			f := newSupportTriageTestFixture(t, nil, nil)
			p := &fakeJev{probability: tc.p}
			tags := attachJev(t, f, p, "off", tc.mode)
			tag, err := tags.Create(f.ctx, f.workspaceID, model.CreateSupportTagRequest{Name: "Pricing"})
			if err != nil {
				t.Fatal(err)
			}
			conv := f.createConversation(t, "pricing", "test@example.com", nil)
			conv.HumanTakeover = boolPtr(true)
			conv.AssignedUserID = &f.actorID
			if err := f.conversationRepo.Update(f.ctx, conv); err != nil {
				t.Fatal(err)
			}
			msg := f.createCustomerReply(t, conv.ID, "Explain pricing")
			r, err := f.triageSvc.EvaluateAndRoute(f.ctx, f.workspaceID, conv.ID, msg.ID)
			if err != nil || r != nil {
				t.Fatalf("human routing changed: %v", err)
			}
			linked, err := tags.conversationHasTag(f.ctx, f.workspaceID, conv.ID, tag.ID)
			if err != nil || linked != tc.want || p.calls != tc.calls {
				t.Fatalf("linked %v calls %d err %v", linked, p.calls, err)
			}
		})
	}
}

func TestJevExplicitRulePrecedence(t *testing.T) {
	f := newSupportTriageTestFixture(t, nil, nil)
	p := &fakeJev{probability: .99}
	attachJev(t, f, p, "primary", "off")
	sales := f.createMailbox(t, "Sales", "sales", true)
	_, err := f.triageSvc.CreateRule(f.ctx, f.workspaceID, f.actorID, model.CreateSupportTriageRuleRequest{Name: "Sales", Priority: 1, Channels: []string{"widget"}, Conditions: model.SupportTriageRuleConditions{PhraseContains: []string{"pricing"}}, TargetMailboxID: sales.ID})
	if err != nil {
		t.Fatal(err)
	}
	conv := f.createConversation(t, "pricing", "test@example.com", nil)
	msg := f.createCustomerReply(t, conv.ID, "pricing please")
	r, err := f.triageSvc.EvaluateAndRoute(f.ctx, f.workspaceID, conv.ID, msg.ID)
	if err != nil || r == nil || p.calls != 0 || r.ClassifierSource != model.SupportConversationTriageSourceRule {
		t.Fatalf("rule lost precedence: %+v %v calls %d", r, err, p.calls)
	}
}

func TestJevLegacyManualRemoval(t *testing.T) {
	tag := model.SupportTag{ID: "tag-1", Name: "Pricing"}
	history := []model.SupportMessage{{SenderType: "user", SystemEventType: model.SupportSystemEventTypeStrPtr(model.SystemEventTagRemoved), Content: "A teammate removed tag Pricing."}}
	if !supportTagManuallyRemoved(history, tag) {
		t.Fatal("legacy manual removal ignored")
	}
}

func TestJevTagInputExcludesInternalAndFutureMessages(t *testing.T) {
	f := newSupportTriageTestFixture(t, nil, nil)
	p := &fakeJev{probability: .99}
	tags := attachJev(t, f, p, "off", "shadow")
	if _, err := tags.Create(f.ctx, f.workspaceID, model.CreateSupportTagRequest{Name: "Pricing"}); err != nil {
		t.Fatal(err)
	}
	conv := f.createConversation(t, "pricing", "test@example.com", nil)
	msg := f.createCustomerReply(t, conv.ID, "Pricing question")
	history := []model.SupportMessage{*msg, {Content: "INTERNAL_SECRET", IsInternal: true, MessageType: "reply", CreatedAt: msg.CreatedAt}, {Content: "FUTURE_TEXT", MessageType: "reply", CreatedAt: msg.CreatedAt.Add(time.Hour)}}
	if err := f.triageSvc.jev.TagConversation(f.ctx, f.workspaceID, conv.ID, msg, history); err != nil {
		t.Fatal(err)
	}
	if len(p.states) != 1 || strings.Contains(p.states[0], "INTERNAL_SECRET") || strings.Contains(p.states[0], "FUTURE_TEXT") {
		t.Fatalf("invalid context selection")
	}
}
