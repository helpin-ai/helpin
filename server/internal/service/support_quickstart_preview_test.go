package service

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// acmeWithHelpCenter is a readable website that links to an Intercom help
// center with four articles in two collections.
func acmeWithHelpCenter() *fakeQuickstart {
	f := helpCenterWithArticles()
	f.proposals = map[string]*model.HelpcenterBrandProposal{"https://acme.com": {
		Platform: model.HelpcenterBrandPlatformWebsite, BrandName: "Acme", LogoURL: "https://acme.com/logo.svg",
		BrandColor: "#ff5500", HelpCenterURL: "https://help.acme.com/en/",
	}}
	f.intercom = map[string]string{"https://help.acme.com/en/": "https://help.acme.com/en"}
	return f
}

const acmePreviewReply = `{"answers":[
	{"question":"How do I start?","can_answer":true,"answer":"Create an event, then share its link.","cited_source_ids":["s1"],"confidence":0.9},
	{"question":"How am I billed?","can_answer":true,"answer":"Plans are billed monthly.","cited_source_ids":["s2"],"confidence":0.9},
	{"question":"Is there an app?","can_answer":true,"answer":"See https://acme.com/app","cited_source_ids":["s1"],"confidence":0.9}
],"gaps":["Do you have a mobile app?","How do refunds work?","Can I get my money back?","Can I export attendees to a spreadsheet?"]}`

// runPreview collects every event of a preview.
func runPreview(t *testing.T, svc *SupportQuickstartService, req model.SupportQuickstartPreviewRequest) []model.SupportQuickstartEvent {
	t.Helper()
	var events []model.SupportQuickstartEvent
	if err := svc.Preview(context.Background(), req, func(event model.SupportQuickstartEvent) { events = append(events, event) }); err != nil {
		t.Fatalf("Preview() error = %v", err)
	}
	return events
}

// eventOf returns the one event of a type, failing when it is missing.
func eventOf(t *testing.T, events []model.SupportQuickstartEvent, kind string) model.SupportQuickstartEvent {
	t.Helper()
	i := slices.IndexFunc(events, func(event model.SupportQuickstartEvent) bool { return event.Type == kind })
	if i < 0 {
		t.Fatalf("no %q event in %+v", kind, events)
	}
	return events[i]
}

func TestPreviewReportsEachStepAsItHappens(t *testing.T) {
	events := runPreview(t, newPreviewServiceForTest(acmeWithHelpCenter(), &fakeQuickstartAI{content: acmePreviewReply}, nil, nil),
		model.SupportQuickstartPreviewRequest{WebsiteURL: "acme.com"})

	var order []string
	for _, event := range events {
		if event.Type == model.QuickstartEventLog {
			order = append(order, event.Step+": "+event.Text)
		} else {
			order = append(order, event.Type)
		}
	}
	// The articles are read side by side, so only their place is checked.
	if order = slices.DeleteFunc(order, func(line string) bool {
		return strings.HasPrefix(line, "articles: Read") && line != "articles: Read “Getting started”"
	}); !slices.Equal(order, []string{
		"site: Opening acme.com", "site",
		"help_center: Looking for your help center", "help_center: Counting the articles on help.acme.com/en", "help_center",
		"articles: Read “Getting started”", "articles",
		"answers: Writing answers to the questions your customers ask", "preview",
	}) {
		t.Errorf("events = %q", order)
	}
	site := eventOf(t, events, model.QuickstartEventSite).Site
	if !site.WebsiteReadable || site.CompanyName != "Acme" || site.LogoURL != "https://acme.com/logo.svg" || site.BrandColor != "#ff5500" {
		t.Errorf("site = %+v, want Acme's brand", site)
	}
	if found := eventOf(t, events, model.QuickstartEventHelpCenter); found.HelpCenterURL != "https://help.acme.com/en" || found.HelpCenter.Articles != 4 {
		t.Errorf("help center = %+v, want 4 articles at the Intercom home", found)
	}
}

func TestPreviewKeepsAnotherHelpCenterToLearnFrom(t *testing.T) {
	f := &fakeQuickstart{proposals: map[string]*model.HelpcenterBrandProposal{"https://acme.com": {
		Platform: model.HelpcenterBrandPlatformWebsite, BrandName: "Acme", HelpCenterURL: "https://acme.zendesk.com/hc",
	}}}
	events := runPreview(t, newPreviewServiceForTest(f, nil, nil, nil), model.SupportQuickstartPreviewRequest{WebsiteURL: "https://acme.com/"})

	if found := eventOf(t, events, model.QuickstartEventHelpCenter); found.HelpCenterURL != "https://acme.zendesk.com/hc" || found.HelpCenter != nil {
		t.Errorf("help center = %+v, want the Zendesk link without counts", found)
	}
}

func TestPreviewNamesAnUnreadableSiteFromItsDomain(t *testing.T) {
	events := runPreview(t, newPreviewServiceForTest(&fakeQuickstart{}, nil, nil, nil), model.SupportQuickstartPreviewRequest{WebsiteURL: "acme-labs.co.uk"})

	site := eventOf(t, events, model.QuickstartEventSite).Site
	if site.WebsiteReadable || site.CompanyName != "Acme Labs" || eventOf(t, events, model.QuickstartEventHelpCenter).HelpCenterURL != "" {
		t.Errorf("site = %+v, want an unreadable site named from its domain and no help center", site)
	}
}

func TestPreviewTreatsAnIntercomAddressAsTheHelpCenter(t *testing.T) {
	f := helpCenterWithArticles()
	f.proposals = map[string]*model.HelpcenterBrandProposal{"https://help.acme.com": {
		Platform: model.HelpcenterBrandPlatformIntercom, BrandName: "Acme", SourceURL: "https://help.acme.com/en", WebsiteURL: "https://acme.com",
	}}
	events := runPreview(t, newPreviewServiceForTest(f, nil, nil, nil), model.SupportQuickstartPreviewRequest{WebsiteURL: "help.acme.com"})

	if site := eventOf(t, events, model.QuickstartEventSite).Site; site.WebsiteURL != "https://acme.com" {
		t.Errorf("site = %+v, want the company website the help center names", site)
	}
	if found := eventOf(t, events, model.QuickstartEventHelpCenter); found.HelpCenterURL != "https://help.acme.com/en" || found.HelpCenter == nil {
		t.Errorf("help center = %+v, want the address given, counted", found)
	}
}

func TestPreviewFindsAHelpCenterOnAnyPlatformThatCanBeRead(t *testing.T) {
	f := helpCenterWithArticles()
	f.platform = "zendesk"
	// The website links nowhere; the help center is found at the usual place.
	f.proposals = map[string]*model.HelpcenterBrandProposal{"https://acme.com": {Platform: model.HelpcenterBrandPlatformWebsite, BrandName: "Acme"}}
	f.intercom = map[string]string{"https://support.acme.com": "https://support.acme.com/hc/en-us/"}
	events := runPreview(t, newPreviewServiceForTest(f, nil, nil, nil), model.SupportQuickstartPreviewRequest{WebsiteURL: "acme.com"})

	found := eventOf(t, events, model.QuickstartEventHelpCenter)
	if found.HelpCenterURL != "https://support.acme.com/hc/en-us/" || found.HelpCenter == nil || found.HelpCenter.Articles != len(f.articles) {
		t.Errorf("help center = %+v, want the Zendesk help center with its articles counted", found)
	}
}

func TestPreviewReadsAcrossCollectionsAndKeepsOnlyGroundedWork(t *testing.T) {
	f := acmeWithHelpCenter()
	ai := &fakeQuickstartAI{content: acmePreviewReply}
	events := runPreview(t, newPreviewServiceForTest(f, ai, nil, nil), model.SupportQuickstartPreviewRequest{WebsiteURL: "acme.com"})

	read := eventOf(t, events, model.QuickstartEventArticles).Articles
	// One article from each collection first: Basics, Billing, then their second ones.
	if len(read) != 4 || read[0].Title != "Getting started" || read[1].Title != "Billing basics" {
		t.Errorf("articles read = %+v, want each collection's first article first", read)
	}
	preview := eventOf(t, events, model.QuickstartEventPreview).Preview
	if !preview.Available || len(preview.Answers) != 2 {
		t.Fatalf("preview = %+v, want the two link-free answers", preview)
	}
	if got := preview.Answers[1]; got.Question != "How am I billed?" || got.Sources[0].Title != "Billing basics" {
		t.Errorf("second answer = %+v, want it cited to Billing basics", got)
	}
	// The Refunds article covers two of them, by its title and by its summary.
	if want := []string{"Do you have a mobile app?", "Can I export attendees to a spreadsheet?"}; !slices.Equal(preview.Gaps, want) {
		t.Errorf("gaps = %q, want %q", preview.Gaps, want)
	}
	if !strings.Contains(ai.prompts[0], "- Refunds") {
		t.Errorf("prompt did not list the article titles: %q", ai.prompts[0])
	}
}

func TestPreviewAnswersFromWebsitePagesWithoutAHelpCenter(t *testing.T) {
	reply := `{"answers":[{"question":"How much is it?","can_answer":true,"answer":"Plans start at $10 a month.","cited_source_ids":["s1"],"confidence":0.9}],"gaps":["Do you have an API?"]}`
	pages := fakeQuickstartPages{"https://acme.com/pricing": "Plans start at $10 a month."}
	events := runPreview(t, newPreviewServiceForTest(&fakeQuickstart{}, &fakeQuickstartAI{content: reply}, pages, nil),
		model.SupportQuickstartPreviewRequest{WebsiteURL: "acme.com"})

	if found := eventOf(t, events, model.QuickstartEventHelpCenter); found.HelpCenterURL != "" || found.HelpCenter != nil {
		t.Errorf("help center = %+v, want none", found)
	}
	preview := eventOf(t, events, model.QuickstartEventPreview).Preview
	if len(preview.Answers) != 1 || preview.Answers[0].Sources[0].Title != "acme.com/pricing" {
		t.Errorf("answers = %+v, want one from acme.com/pricing", preview.Answers)
	}
	// Without an article list there is nothing to call a gap.
	if len(preview.Gaps) != 0 {
		t.Errorf("gaps = %q, want none", preview.Gaps)
	}
}

func TestPreviewUsesAHelpCenterAddressThePersonAdded(t *testing.T) {
	f := helpCenterWithArticles()
	events := runPreview(t, newPreviewServiceForTest(f, &fakeQuickstartAI{content: acmePreviewReply}, nil, nil),
		model.SupportQuickstartPreviewRequest{WebsiteURL: "acme.com", HelpCenterURL: "support.acme.com"})

	if found := eventOf(t, events, model.QuickstartEventHelpCenter); found.HelpCenterURL != "https://support.acme.com" || found.HelpCenter == nil || found.HelpCenter.Articles != 4 {
		t.Errorf("help center = %+v, want its articles counted", found)
	}
}

func TestPreviewSkipsTheAIWhenItCannotAnswer(t *testing.T) {
	t.Run("no AI on this server", func(t *testing.T) {
		f := acmeWithHelpCenter()
		events := runPreview(t, newPreviewServiceForTest(f, nil, nil, nil), model.SupportQuickstartPreviewRequest{WebsiteURL: "acme.com"})
		if preview := eventOf(t, events, model.QuickstartEventPreview).Preview; preview.Available || len(f.articleReads) != 0 {
			t.Errorf("preview = %+v after reading %v, want unavailable and nothing read", preview, f.articleReads)
		}
	})
	t.Run("nothing could be read", func(t *testing.T) {
		ai := &fakeQuickstartAI{}
		events := runPreview(t, newPreviewServiceForTest(&fakeQuickstart{}, ai, fakeQuickstartPages{}, nil), model.SupportQuickstartPreviewRequest{WebsiteURL: "acme.com"})
		if preview := eventOf(t, events, model.QuickstartEventPreview).Preview; !preview.Available || len(preview.Answers) != 0 || ai.calls() != 0 {
			t.Errorf("preview = %+v with %d AI calls, want no answers and no AI call", preview, ai.calls())
		}
	})
}

func TestPreviewReusesTheAnswersForTheSameSite(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	f := acmeWithHelpCenter()
	ai := &fakeQuickstartAI{content: acmePreviewReply}
	svc := newPreviewServiceForTest(f, ai, nil, rdb)

	first := runPreview(t, svc, model.SupportQuickstartPreviewRequest{WebsiteURL: "acme.com"})
	second := runPreview(t, svc, model.SupportQuickstartPreviewRequest{WebsiteURL: "acme.com"})
	if ai.calls() != 1 || len(f.articleReads) != 4 {
		t.Fatalf("AI calls = %d, article reads = %d, want 1 and 4", ai.calls(), len(f.articleReads))
	}
	want, got := eventOf(t, first, model.QuickstartEventPreview).Preview, eventOf(t, second, model.QuickstartEventPreview).Preview
	if len(got.Answers) != len(want.Answers) || len(eventOf(t, second, model.QuickstartEventArticles).Articles) != 4 {
		t.Errorf("second preview = %+v, want the first one again with its articles", got)
	}
}

func TestPreviewPausesAnswersAtTheDailyLimit(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	if err := mr.Set("onb:answer:daily:"+time.Now().UTC().Format("20060102"), fmt.Sprint(quickstartAnswerDailyLimit)); err != nil {
		t.Fatalf("seed budget: %v", err)
	}
	ai := &fakeQuickstartAI{}
	events := runPreview(t, newPreviewServiceForTest(acmeWithHelpCenter(), ai, nil, rdb), model.SupportQuickstartPreviewRequest{WebsiteURL: "acme.com"})

	failed := eventOf(t, events, model.QuickstartEventFailed)
	if failed.Step != model.QuickstartStepAnswers || !strings.Contains(failed.Text, "previews are paused") || ai.calls() != 0 {
		t.Errorf("failed = %+v with %d AI calls, want answers paused and no AI call", failed, ai.calls())
	}
	// The site and help center were still shown.
	eventOf(t, events, model.QuickstartEventSite)
	eventOf(t, events, model.QuickstartEventHelpCenter)
}

func TestPreviewRefusesAnAddressBeforeSendingAnything(t *testing.T) {
	sent := 0
	err := newPreviewServiceForTest(&fakeQuickstart{}, nil, nil, nil).Preview(context.Background(),
		model.SupportQuickstartPreviewRequest{WebsiteURL: "localhost"}, func(model.SupportQuickstartEvent) { sent++ })
	if !errors.Is(err, ErrSupportQuickstartInvalid) || sent != 0 {
		t.Fatalf("Preview() error = %v after %d events, want invalid and none", err, sent)
	}
}
