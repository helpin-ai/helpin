package service

import (
	"context"
	"errors"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/helpin-ai/helpin/server/internal/decision"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/observability"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

type fakeJev struct {
	calls       int
	probability float64
	err         error
	states      []string
	during      func()
}

func (f *fakeJev) DecideMany(_ context.Context, state string, questions map[string]decision.Question) (*decision.Result, error) {
	f.calls++
	f.states = append(f.states, state)
	if f.during != nil {
		f.during()
	}
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

func TestSupportTaggingDoesNotRunOnRoutingPath(t *testing.T) {
	f := newSupportTriageTestFixture(t, nil, func(settings *model.SupportInboxSettings) {
		settings.TriageEnabled = false
	})
	p := &fakeJev{err: errors.New("tagging provider must not run on the reply path")}
	tags := attachJev(t, f, p, "off", "primary")
	if _, err := tags.Create(f.ctx, f.workspaceID, model.CreateSupportTagRequest{Name: "Pricing"}); err != nil {
		t.Fatal(err)
	}
	conv := f.createConversation(t, "pricing", "test@example.com", nil)
	msg := f.createCustomerReply(t, conv.ID, "Please explain enterprise pricing")
	if _, err := f.triageSvc.EvaluateAndRoute(f.ctx, f.workspaceID, conv.ID, msg.ID); err != nil {
		t.Fatal(err)
	}
	if p.calls != 0 {
		t.Fatal("routing still waits for the tagging provider")
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
	runSupportTagJob(t, f, msg)
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
	runSupportTagJob(t, f, msg)
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
			runSupportTagJob(t, f, msg)
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
	if _, err := f.triageSvc.jev.selectConversationTags(f.ctx, f.workspaceID, conv.ID, msg, history); err != nil {
		t.Fatal(err)
	}
	if len(p.states) != 1 || strings.Contains(p.states[0], "INTERNAL_SECRET") || strings.Contains(p.states[0], "FUTURE_TEXT") {
		t.Fatalf("invalid context selection")
	}
}

func TestSupportJevReusesSuccessfulDecisionWithoutUsageOrBudget(t *testing.T) {
	f := newSupportTriageTestFixture(t, nil, nil)
	p := &fakeJev{probability: .99}
	attachJev(t, f, p, "primary", "off")
	s := f.triageSvc.jev
	s.config.DailyLimit = 1
	conv := f.createConversation(t, "pricing", "test@example.com", nil)
	questions := map[string]decision.Question{"mailbox": {Instructions: "route", Choices: map[string]string{"sales": "Sales", "shared": "Other"}}}
	for range 2 {
		result, err := s.evaluate(f.ctx, f.workspaceID, conv.ID, "Pricing question", "routing", questions)
		if err != nil || result == nil || result.Answers["mailbox"].Choice != "sales" {
			t.Fatalf("cached decision unavailable: result=%+v error=%v", result, err)
		}
	}
	var count int64
	if err := f.db.Model(&model.AIExecutionUsage{}).Count(&count).Error; err != nil || count != 1 || p.calls != 1 {
		t.Fatalf("cache duplicated usage: rows=%d calls=%d error=%v", count, p.calls, err)
	}
}

func TestSupportJevRetriesAfterCooldownAndStillCountsFailedAttempts(t *testing.T) {
	for _, limit := range []int{1, 2} {
		f := newSupportTriageTestFixture(t, nil, nil)
		p := &fakeJev{probability: .99, err: errors.New("temporary outage")}
		attachJev(t, f, p, "primary", "off")
		s := f.triageSvc.jev
		s.config.DailyLimit = limit
		conv := f.createConversation(t, "pricing", "test@example.com", nil)
		questions := map[string]decision.Question{"mailbox": {Instructions: "route", Choices: map[string]string{"sales": "Sales", "shared": "Other"}}}
		evaluate := func() (*decision.Result, error) {
			return s.evaluate(f.ctx, f.workspaceID, conv.ID, "Pricing question", "routing", questions)
		}
		if result, err := evaluate(); err == nil || result != nil {
			t.Fatalf("provider failure accepted: %+v %v", result, err)
		}
		p.err = nil
		if result, err := evaluate(); err != nil || result != nil || p.calls != 1 {
			t.Fatalf("cooldown bypassed: %+v %v calls=%d", result, err, p.calls)
		}
		if err := f.db.Model(&model.SupportConversationTriageEvent{}).Where("conversation_id = ? AND event_type = ?", conv.ID, "jev_decision").Update("created_at", time.Now().UTC().Add(-2*time.Minute)).Error; err != nil {
			t.Fatal(err)
		}
		result, err := evaluate()
		if err != nil || (result != nil) != (limit == 2) || p.calls != limit {
			t.Fatalf("retry/cap mismatch: limit=%d result=%+v error=%v calls=%d", limit, result, err, p.calls)
		}
	}
}

func TestSupportJevPolicyChangesInvalidateCachedDecision(t *testing.T) {
	for _, change := range []string{"mode", "threshold"} {
		t.Run(change, func(t *testing.T) {
			f := newSupportTriageTestFixture(t, nil, nil)
			p := &fakeJev{probability: .99}
			attachJev(t, f, p, "shadow", "off")
			s := f.triageSvc.jev
			conv := f.createConversation(t, "pricing", "test@example.com", nil)
			questions := map[string]decision.Question{"mailbox": {Instructions: "route", Choices: map[string]string{"sales": "Sales", "shared": "Other"}}}
			if _, err := s.evaluate(f.ctx, f.workspaceID, conv.ID, "Pricing question", "routing", questions); err != nil {
				t.Fatal(err)
			}
			if change == "mode" {
				s.config.RoutingMode = "primary"
			} else {
				s.config.RoutingThreshold = .98
			}
			if result, err := s.evaluate(f.ctx, f.workspaceID, conv.ID, "Pricing question", "routing", questions); err != nil || result == nil || p.calls != 2 {
				t.Fatalf("policy change reused/suppressed old assessment: %+v %v calls=%d", result, err, p.calls)
			}
		})
	}
}

func TestSupportJevMetricsSeparateOperationsAndCacheHits(t *testing.T) {
	f := newSupportTriageTestFixture(t, nil, nil)
	p := &fakeJev{probability: .99}
	attachJev(t, f, p, "primary", "primary")
	s := f.triageSvc.jev
	m := observability.NewMetrics()
	s.SetMetrics(m)
	conv := f.createConversation(t, "pricing", "test@example.com", nil)
	questions := map[string]decision.Question{"mailbox": {Instructions: "route", Choices: map[string]string{"sales": "Sales", "shared": "Other"}}}
	for operation, identity := range map[string]string{"routing": "routing", "tagging": "tags:private-message", "handoff": "handoff:v1:private-message", "follow_up": "follow_up:v1:private-episode"} {
		for range 2 {
			if _, err := s.evaluate(f.ctx, f.workspaceID, conv.ID, "Private evidence", identity, questions); err != nil {
				t.Fatal(err)
			}
		}
		w := httptest.NewRecorder()
		m.Handler().ServeHTTP(w, httptest.NewRequest("GET", "/metrics", nil))
		for _, outcome := range []string{"success", "cached"} {
			if !strings.Contains(w.Body.String(), `helpin_ai_decisions_total{operation="`+operation+`",outcome="`+outcome+`"} 1`) {
				t.Fatalf("missing %s/%s metric", operation, outcome)
			}
		}
		if strings.Contains(w.Body.String(), "private-") || strings.Contains(w.Body.String(), "Private evidence") {
			t.Fatal("unbounded decision labels expose evidence")
		}
	}
}

func TestSupportJevRejectsInvalidProviderResult(t *testing.T) {
	f := newSupportTriageTestFixture(t, nil, nil)
	p := &fakeJev{probability: 1.2}
	attachJev(t, f, p, "primary", "off")
	conv := f.createConversation(t, "pricing", "test@example.com", nil)
	questions := map[string]decision.Question{"mailbox": {Instructions: "route", Choices: map[string]string{"sales": "Sales", "shared": "Other"}}}
	result, err := f.triageSvc.jev.evaluate(f.ctx, f.workspaceID, conv.ID, "Pricing", "routing", questions)
	if err == nil || result != nil {
		t.Fatalf("invalid probability was actionable: %+v %v", result, err)
	}
	var event model.SupportConversationTriageEvent
	if err := f.db.Where("event_type = ?", "jev_decision").First(&event).Error; err != nil {
		t.Fatal(err)
	}
	if event.Payload["status"] != "provider_error" {
		t.Fatalf("invalid provider output cached as successful: %+v", event.Payload)
	}
}

func TestSupportJevRejectsInvalidCachedResult(t *testing.T) {
	for _, invalid := range []any{nil, "invalid", &decision.Result{Model: decision.Model}} {
		f := newSupportTriageTestFixture(t, nil, nil)
		p := &fakeJev{probability: .99}
		attachJev(t, f, p, "primary", "off")
		conv := f.createConversation(t, "pricing", "test@example.com", nil)
		questions := map[string]decision.Question{"mailbox": {Instructions: "route", Choices: map[string]string{"sales": "Sales", "shared": "Other"}}}
		evaluate := func() (*decision.Result, error) {
			return f.triageSvc.jev.evaluate(f.ctx, f.workspaceID, conv.ID, "Pricing", "routing", questions)
		}
		if _, err := evaluate(); err != nil {
			t.Fatal(err)
		}
		if err := f.db.Model(&model.SupportConversationTriageEvent{}).Where("event_type = ?", "jev_decision").Update("payload", model.JSONB{"status": "ok", "result": invalid}).Error; err != nil {
			t.Fatal(err)
		}
		if result, err := evaluate(); err == nil || result != nil || p.calls != 1 {
			t.Fatalf("invalid cached result was actionable or recalled provider: %+v %v calls=%d", result, err, p.calls)
		}
	}
}
