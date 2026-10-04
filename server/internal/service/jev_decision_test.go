package service

import (
	"context"
	"errors"
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/decision"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/observability"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

type jevProductTestProvider struct {
	calls       int
	choices     map[string]string
	probability float64
	malformed   bool
	err         error
	states      []string
}

func (p *jevProductTestProvider) DecideMany(_ context.Context, state string, questions map[string]decision.Question) (*decision.Result, error) {
	p.calls++
	p.states = append(p.states, state)
	if p.err != nil {
		return nil, p.err
	}
	result := &decision.Result{Model: decision.Model, InputTokens: 12, Answers: map[string]decision.Answer{}}
	for key, q := range questions {
		choice := p.choices[key]
		if choice == "" {
			choice = "yes"
		}
		probabilities := map[string]float64{}
		for id := range q.Choices {
			probabilities[id] = (1 - p.probability) / float64(len(q.Choices)-1)
		}
		probabilities[choice] = p.probability
		result.Answers[key] = decision.Answer{Choice: choice, Probabilities: probabilities}
	}
	if p.malformed {
		result.Model = "unexpected-model"
	}
	return result, nil
}
func setupJevDecisionTest(t *testing.T, mode string) (*JevDecisionService, *jevProductTestProvider, *pmTriageTestUsage, *gorm.DB) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:jev_product_%d?mode=memory&cache=shared", time.Now().UnixNano())), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := sqlDB.Close(); err != nil {
			t.Error(err)
		}
	})
	for _, sql := range []string{
		`CREATE TABLE workspaces(id TEXT PRIMARY KEY)`,
		`INSERT INTO workspaces VALUES ('workspace'),('other')`,
		`CREATE TABLE jev_decision_attempts(id TEXT PRIMARY KEY,workspace_id TEXT,feature TEXT,source_id TEXT,input_hash TEXT,mode TEXT,status TEXT,outcome TEXT,created_at DATETIME,updated_at DATETIME)`,
	} {
		if err := db.Exec(sql).Error; err != nil {
			t.Fatal(err)
		}
	}
	provider := &jevProductTestProvider{probability: .99, choices: map[string]string{}}
	usage := &pmTriageTestUsage{}
	policies := map[string]decision.Policy{}
	for _, feature := range []string{JevMeetingRouting, JevCoverageClassification, JevCoverageTopicMatching, JevAutomationCondition, JevAnswerEvidence} {
		policies[feature] = decision.Policy{Mode: mode, Threshold: .95, DailyLimit: 10}
	}
	svc, err := NewJevDecisionService(provider, repository.NewJevDecisionRepository(db), usage, policies, []string{"workspace"})
	if err != nil {
		t.Fatal(err)
	}
	return svc, provider, usage, db
}
func jevTestRequest() JevDecisionRequest {
	return JevDecisionRequest{WorkspaceID: "workspace", Feature: JevAnswerEvidence, SourceID: "source", Version: "v1", State: "Private evidence for classification", Questions: map[string]decision.Question{"supported": {Instructions: "Is the answer supported?", Choices: map[string]string{"yes": "Supported", "no": "Unsupported"}}}}
}
func TestJevProductDecisionCacheAndUsage(t *testing.T) {
	svc, provider, usage, db := setupJevDecisionTest(t, "primary")
	for range 2 {
		result, err := svc.Decide(context.Background(), jevTestRequest())
		if err != nil {
			t.Fatal(err)
		}
		choice, _, accepted := result.Selected("supported")
		if choice != "yes" || !accepted {
			t.Fatalf("unexpected result %+v", result)
		}
	}
	if provider.calls != 1 || len(usage.entries) != 1 || usage.entries[0].FeatureKey != "jev_answer_evidence" {
		t.Fatal("cache repeated provider/usage")
	}
	var attempt model.JevDecisionAttempt
	if err := db.First(&attempt).Error; err != nil {
		t.Fatal(err)
	}
	if strings.Contains(fmt.Sprint(attempt.Outcome), "Private evidence") {
		t.Fatal("raw source stored in audit")
	}
	request := jevTestRequest()
	request.Version = "v2"
	if _, err := svc.Decide(context.Background(), request); err != nil {
		t.Fatal(err)
	}
	if provider.calls != 2 {
		t.Fatal("policy version did not invalidate cache")
	}
}
func TestJevProductDecisionModesAndBounds(t *testing.T) {
	for _, scenario := range []string{"off", "shadow", "workspace", "oversize", "low probability"} {
		t.Run(scenario, func(t *testing.T) {
			mode := "primary"
			if scenario == "off" || scenario == "shadow" {
				mode = scenario
			}
			svc, p, _, _ := setupJevDecisionTest(t, mode)
			request := jevTestRequest()
			switch scenario {
			case "workspace":
				request.WorkspaceID = "other"
			case "oversize":
				request.State = strings.Repeat("x", 16001)
			case "low probability":
				p.probability = .7
			}
			result, err := svc.Decide(context.Background(), request)
			if err != nil {
				t.Fatal(err)
			}
			if _, _, accepted := result.Selected("supported"); accepted {
				t.Fatal("non-actionable decision accepted")
			}
			wantCalls := 0
			if scenario == "shadow" || scenario == "low probability" {
				wantCalls = 1
			}
			if p.calls != wantCalls {
				t.Fatalf("calls=%d want=%d", p.calls, wantCalls)
			}
		})
	}
}
func TestJevProductDecisionFailuresSettleAndConsumeCap(t *testing.T) {
	for _, scenario := range []string{"provider", "usage", "malformed"} {
		t.Run(scenario, func(t *testing.T) {
			svc, p, usage, db := setupJevDecisionTest(t, "primary")
			svc.policies[JevAnswerEvidence] = decision.Policy{Mode: "primary", Threshold: .95, DailyLimit: 1}
			switch scenario {
			case "provider":
				p.err = errors.New("provider unavailable")
			case "usage":
				usage.err = errors.New("usage unavailable")
			case "malformed":
				p.malformed = true
			}
			if result, err := svc.Decide(context.Background(), jevTestRequest()); err == nil || result != nil {
				t.Fatal("failed decision exposed a result")
			}
			var attempt model.JevDecisionAttempt
			if err := db.First(&attempt).Error; err != nil {
				t.Fatal(err)
			}
			if attempt.Status != "failed" {
				t.Fatal("failed attempt left pending")
			}
			request := jevTestRequest()
			request.SourceID = "other-source"
			result, err := svc.Decide(context.Background(), request)
			if err != nil || result.Status != "daily_limit" || p.calls != 1 {
				t.Fatalf("failed attempt escaped cap: %+v %v", result, err)
			}
		})
	}
}

func TestJevProductDecisionMetrics(t *testing.T) {
	svc, _, _, _ := setupJevDecisionTest(t, "primary")
	m := observability.NewMetrics()
	svc.SetMetrics(m)
	for range 2 {
		if _, err := svc.Decide(context.Background(), jevTestRequest()); err != nil {
			t.Fatal(err)
		}
	}
	w := httptest.NewRecorder()
	m.Handler().ServeHTTP(w, httptest.NewRequest("GET", "/metrics", nil))
	for _, outcome := range []string{"ready", "cached"} {
		if !strings.Contains(w.Body.String(), `helpin_ai_decisions_total{operation="answer_evidence",outcome="`+outcome+`"} 1`) {
			t.Fatalf("missing answer evidence %s metric", outcome)
		}
	}
}
