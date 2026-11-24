package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"

	"github.com/helpin-ai/helpin/server/internal/intercom"
	"github.com/helpin-ai/helpin/server/internal/llm"
	"github.com/helpin-ai/helpin/server/internal/model"
)

// fakeQuickstartAI returns one canned completion and records the prompts.
type fakeQuickstartAI struct {
	content string
	mu      sync.Mutex
	prompts []string
}

func (a *fakeQuickstartAI) ChatCompletion(_ context.Context, req llm.ChatRequest) (*llm.ChatResponse, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.prompts = append(a.prompts, req.Messages[0].Content)
	return &llm.ChatResponse{Content: a.content}, nil
}

func (a *fakeQuickstartAI) calls() int {
	a.mu.Lock()
	defer a.mu.Unlock()
	return len(a.prompts)
}

// fakeQuickstartPages serves page text by address; others fail.
type fakeQuickstartPages map[string]string

func (p fakeQuickstartPages) FetchText(_ context.Context, rawURL string) (string, error) {
	if text, ok := p[rawURL]; ok {
		return text, nil
	}
	return "", errors.New("status 404")
}

func helpCenterWithArticles() *fakeQuickstart {
	return &fakeQuickstart{
		articles: []model.PublicHelpCenterArticleRef{
			{Title: "Getting started", URL: "https://help.acme.com/en/articles/1-getting-started", Collection: "Basics"},
			{Title: "Invite your team", URL: "https://help.acme.com/en/articles/2-invite", Collection: "Basics"},
			{Title: "Billing basics", URL: "https://help.acme.com/en/articles/3-billing", Collection: "Billing"},
			{Title: "Refunds", URL: "https://help.acme.com/en/articles/4-refunds", Summary: "How to get your money back", Collection: "Billing"},
		},
		articleText: map[string]string{
			"https://help.acme.com/en/articles/1-getting-started": "Create an event, then share its link.",
			"https://help.acme.com/en/articles/2-invite":          "Invite teammates from Settings.",
			"https://help.acme.com/en/articles/3-billing":         "Plans are billed monthly.",
			"https://help.acme.com/en/articles/4-refunds":         "Refunds are issued within 5 days.",
		},
	}
}

func newPreviewServiceForTest(f *fakeQuickstart, ai quickstartAI, pages WorkspaceContextFetcher, rdb *redis.Client) *SupportQuickstartService {
	return NewSupportQuickstartService(SupportQuickstartDeps{
		Brand: f, Imports: f, Workspaces: f, Support: f, ContentSources: f, AgentContentSources: f, Chunks: f,
		AI: ai, Pages: pages, Redis: rdb,
	})
}

// oneAnswer is the agent's reply holding a single answer.
func oneAnswer(answer, cited string, confidence float64) string {
	return fmt.Sprintf(`{"answers":[{"question":"","can_answer":true,"answer":%q,"cited_source_ids":[%q],"confidence":%v}],"gaps":[]}`, answer, cited, confidence)
}

func refundQuestion() model.SupportQuickstartAnswerRequest {
	return model.SupportQuickstartAnswerRequest{WebsiteURL: "acme.com", HelpCenterURL: "help.acme.com/en", Question: "Can I get my money back?"}
}

func TestPreviewAnswerReadsTheArticlesMatchingTheQuestion(t *testing.T) {
	f := helpCenterWithArticles()
	ai := &fakeQuickstartAI{content: oneAnswer("Refunds are issued within 5 days.", "s1", 0.8)}
	svc := newPreviewServiceForTest(f, ai, nil, nil)

	got, err := svc.PreviewAnswer(context.Background(), refundQuestion())
	if err != nil {
		t.Fatalf("PreviewAnswer() error = %v", err)
	}
	if len(f.articleReads) != 1 || f.articleReads[0] != "https://help.acme.com/en/articles/4-refunds" {
		t.Errorf("read %v, want only the refunds article", f.articleReads)
	}
	if got.Question != "Can I get my money back?" || !got.CanAnswer || len(got.Sources) != 1 || got.Sources[0].Title != "Refunds" {
		t.Errorf("answer = %+v, want the visitor's question answered from Refunds", got)
	}
	if !strings.Contains(ai.prompts[0], "Refunds are issued within 5 days.") {
		t.Errorf("prompt did not include the article text: %q", ai.prompts[0])
	}
}

func TestPreviewAnswerRefusesWhatItCannotGround(t *testing.T) {
	tests := []struct{ name, reply string }{
		{name: "an answer with a link", reply: oneAnswer("See https://acme.com/refunds", "s1", 0.9)},
		{name: "a citation to a page not read", reply: oneAnswer("Refunds take 5 days.", "s9", 0.9)},
		{name: "low confidence", reply: oneAnswer("Refunds take 5 days.", "s1", 0.2)},
		{name: "no answer at all", reply: `{"answers":[],"gaps":[]}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := newPreviewServiceForTest(helpCenterWithArticles(), &fakeQuickstartAI{content: tt.reply}, nil, nil)
			got, err := svc.PreviewAnswer(context.Background(), refundQuestion())
			if err != nil {
				t.Fatalf("PreviewAnswer() error = %v", err)
			}
			if !got.Available || got.CanAnswer || got.Answer != "" || len(got.Sources) != 0 {
				t.Errorf("answer = %+v, want it refused", got)
			}
		})
	}
}

func TestPreviewAnswerUsesWebsitePagesWithoutAHelpCenter(t *testing.T) {
	ai := &fakeQuickstartAI{content: oneAnswer("Plans start at $10 a month.", "s1", 0.9)}
	pages := fakeQuickstartPages{"https://acme.com/pricing": "Plans start at $10 a month."}
	svc := newPreviewServiceForTest(&fakeQuickstart{}, ai, pages, nil)

	got, err := svc.PreviewAnswer(context.Background(), model.SupportQuickstartAnswerRequest{WebsiteURL: "acme.com", Question: "How much is it?"})
	if err != nil {
		t.Fatalf("PreviewAnswer() error = %v", err)
	}
	if !got.CanAnswer || len(got.Sources) != 1 || got.Sources[0].Title != "acme.com/pricing" {
		t.Errorf("answer = %+v, want it answered from acme.com/pricing", got)
	}
}

func TestPreviewAnswerSkipsTheAIWhenNothingCouldBeRead(t *testing.T) {
	ai := &fakeQuickstartAI{}
	svc := newPreviewServiceForTest(&fakeQuickstart{}, ai, fakeQuickstartPages{}, nil)

	got, err := svc.PreviewAnswer(context.Background(), model.SupportQuickstartAnswerRequest{WebsiteURL: "acme.com", Question: "How much is it?"})
	if err != nil {
		t.Fatalf("PreviewAnswer() error = %v", err)
	}
	if !got.Available || got.CanAnswer || ai.calls() != 0 {
		t.Errorf("answer = %+v with %d AI calls, want no answer and no AI call", got, ai.calls())
	}
}

func TestPreviewAnswerIsUnavailableWithoutAI(t *testing.T) {
	svc := newPreviewServiceForTest(helpCenterWithArticles(), nil, nil, nil)

	got, err := svc.PreviewAnswer(context.Background(), refundQuestion())
	if err != nil {
		t.Fatalf("PreviewAnswer() error = %v", err)
	}
	if got.Available {
		t.Errorf("answer = %+v, want unavailable", got)
	}
}

func TestPreviewAnswerRejectsAQuestionItCannotTake(t *testing.T) {
	svc := newPreviewServiceForTest(helpCenterWithArticles(), &fakeQuickstartAI{}, nil, nil)

	for _, question := range []string{"  ", strings.Repeat("why ", 100)} {
		_, err := svc.PreviewAnswer(context.Background(), model.SupportQuickstartAnswerRequest{WebsiteURL: "acme.com", Question: question})
		if !errors.Is(err, ErrSupportQuickstartInvalid) {
			t.Errorf("PreviewAnswer(%q) error = %v, want invalid", question, err)
		}
	}
}

func TestPreviewAnswerListsTheHelpCenterOnceForFollowUpQuestions(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	f := helpCenterWithArticles()
	svc := newPreviewServiceForTest(f, &fakeQuickstartAI{content: oneAnswer("Refunds take 5 days.", "s1", 0.9)}, nil, rdb)

	for range 2 {
		if _, err := svc.PreviewAnswer(context.Background(), refundQuestion()); err != nil {
			t.Fatalf("PreviewAnswer() error = %v", err)
		}
	}
	if f.listings != 1 {
		t.Errorf("help center listed %d times, want 1", f.listings)
	}
}

func TestPreviewAnswerStopsAtTheDailyLimit(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	key := "onb:answer:daily:" + time.Now().UTC().Format("20060102")
	if err := mr.Set(key, fmt.Sprint(quickstartAnswerDailyLimit)); err != nil {
		t.Fatalf("seed budget: %v", err)
	}
	ai := &fakeQuickstartAI{}
	svc := newPreviewServiceForTest(helpCenterWithArticles(), ai, nil, rdb)

	_, err := svc.PreviewAnswer(context.Background(), refundQuestion())
	if !errors.Is(err, ErrSupportQuickstartBusy) || ai.calls() != 0 {
		t.Fatalf("PreviewAnswer() error = %v with %d AI calls, want busy and no AI call", err, ai.calls())
	}
}

func TestWritePublicBlocksTextIncludesListsAndTables(t *testing.T) {
	blocks := []intercom.Block{
		{Type: "paragraph", Text: "Open <b>Settings</b>."},
		{Type: "orderedList", Items: []intercom.BlockItem{{Content: []intercom.Block{{Type: "paragraph", Text: "Click Billing"}}}}},
		{Type: "table", Rows: []intercom.BlockRow{{Cells: []intercom.BlockItem{{Content: []intercom.Block{{Type: "paragraph", Text: "Pro: $20"}}}}}}},
	}
	var b strings.Builder
	writePublicBlocksText(&b, blocks)
	got := b.String()
	for _, want := range []string{"Settings", "Click Billing\n", "Pro: $20\n"} {
		if !strings.Contains(got, want) {
			t.Errorf("text = %q, want it to contain %q", got, want)
		}
	}
	if strings.Contains(got, "<b>") {
		t.Errorf("text = %q, want markup removed", got)
	}
}

func TestWithoutLeadingQuestion(t *testing.T) {
	for answer, want := range map[string]string{
		"How do I set up my first event? Start from your Box office.":            "Start from your Box office.",
		"1. How are tickets charged? — We charge per ticket.\n2. Pay as you go.": "1. We charge per ticket.\n2. Pay as you go.",
		"Start from your Box office. Need more? Ask us.":                         "Start from your Box office. Need more? Ask us.",
		"Is that all?":             "Is that all?",
		"  Refunds take 5 days.  ": "Refunds take 5 days.",
	} {
		if got := withoutLeadingQuestion(answer); got != want {
			t.Errorf("withoutLeadingQuestion(%q) = %q, want %q", answer, got, want)
		}
	}
}
