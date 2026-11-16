//go:build ee

package demo

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	eerepository "github.com/helpin-ai/helpin/server/ee/repository"
	"github.com/helpin-ai/helpin/server/internal/model"
)

// buildDemo creates a demo for company-1 and runs the worker until it ends.
func buildDemo(t *testing.T, fakes *demoFakes, svc *Service, input CreateInput) *View {
	t.Helper()
	input.CompanyID = "company-1"
	view, _, err := svc.Create(context.Background(), demoHomeWorkspace, "user-1", input)
	if err != nil {
		t.Fatal(err)
	}
	fakes.build(t, svc, 60)
	got, err := svc.Get(context.Background(), demoHomeWorkspace, view.ID, CreateInput{})
	if err != nil {
		t.Fatal(err)
	}
	return got
}

func TestDemoHandoffsFinishWithIssues(t *testing.T) {
	fakes := newDemoFakes()
	fakes.decisions["Can you help me with Update your card?"] = "handoff"
	fakes.decisions["How much does Acme Inc cost?"] = "handoff"
	got := buildDemo(t, fakes, fakes.service(), CreateInput{})
	// One of two article questions answered fails the pass rule.
	if got.Status != eerepository.DemoStatusReadyWithIssues || got.Report.VerifyPassed {
		t.Fatalf("status = %s, verify passed = %v", got.Status, got.Report.VerifyPassed)
	}
	want := []string{"verify_handoffs:answer_validation_numeric_mismatch", IssueVerifyFailed}
	if !slices.Equal(got.Report.Issues, want) {
		t.Fatalf("issues = %v, want %v", got.Report.Issues, want)
	}
	if fakes.companies["company-1"].CustomProperties[PropertyStatus] != eerepository.DemoStatusReadyWithIssues {
		t.Fatalf("company demo_status = %v", fakes.companies["company-1"].CustomProperties[PropertyStatus])
	}
	if body := *fakes.activities[0].Body; !strings.Contains(body, "not answered (answer_validation_numeric_mismatch)") || strings.Contains(body, "0.9") {
		t.Fatalf("CRM note = %q", body)
	}
}

func TestDemoPricingHandoffStillPasses(t *testing.T) {
	fakes := newDemoFakes()
	fakes.decisions["How much does Acme Inc cost?"] = "handoff"
	got := buildDemo(t, fakes, fakes.service(), CreateInput{VerifyQuestions: []string{" Do you have an API? ", ""}})
	// The pass rule counts article questions only; the handoff is still listed.
	if got.Status != eerepository.DemoStatusReady || !slices.Contains(got.Report.Issues, "verify_handoffs:answer_validation_numeric_mismatch") {
		t.Fatalf("status = %s, issues = %v", got.Status, got.Report.Issues)
	}
	if last := fakes.previewStarts[len(fakes.previewStarts)-1]; last != "Do you have an API?" || len(fakes.previewStarts) != 4 {
		t.Fatalf("check questions = %q", fakes.previewStarts)
	}
}

func TestDemoIndexWaitReleasesTheBuildAndTimesOut(t *testing.T) {
	fakes := newDemoFakes()
	fakes.indexReadyAt = 1 << 30 // indexing never finishes
	svc := fakes.service()
	view, _, err := svc.Create(context.Background(), demoHomeWorkspace, "user-1", CreateInput{CompanyID: "company-1"})
	if err != nil {
		t.Fatal(err)
	}
	demo := fakes.demos[view.ID]
	for demo.Step != demoStepIndexWait {
		svc.drain(context.Background())
		fakes.now = fakes.now.Add(2 * time.Minute)
	}
	svc.drain(context.Background())
	// Waiting hands the build back instead of holding the 15-minute lease.
	if demo.LeaseUntil.Sub(fakes.now) != demoIndexPollDelay || demo.Status != eerepository.DemoStatusBuilding {
		t.Fatalf("lease = %v, status = %s", demo.LeaseUntil.Sub(fakes.now), demo.Status)
	}
	fakes.build(t, svc, 60)
	report := readDemoReport(demo)
	// After the hour the build checks the demo anyway and says why.
	if !demo.Finished() || !slices.Contains(report.Issues, IssueIndexIncomplete) || len(fakes.previewStarts) != 3 {
		t.Fatalf("status = %s, issues = %v, questions = %d", demo.Status, report.Issues, len(fakes.previewStarts))
	}
}

func TestDemoPartialIndexIsAnIssue(t *testing.T) {
	fakes := newDemoFakes()
	fakes.indexedDocs = 10 // of 19 imported
	got := buildDemo(t, fakes, fakes.service(), CreateInput{})
	if !slices.Contains(got.Report.Issues, IssueIndexIncomplete) || got.Report.Knowledge.IndexedDocuments != 10 {
		t.Fatalf("issues = %v, knowledge = %+v", got.Report.Issues, got.Report.Knowledge)
	}
}

func TestDemoWebsiteThatCannotBeCrawledDoesNotFailTheBuild(t *testing.T) {
	fakes := newDemoFakes()
	fakes.sourceErr = errors.New("crawl blocked")
	got := buildDemo(t, fakes, fakes.service(), CreateInput{})
	if got.Status != eerepository.DemoStatusReady || !slices.Contains(got.Report.Issues, IssueWebsiteCrawlFailed) ||
		!strings.Contains(strings.Join(got.Report.Notes, "\n"), "website could not be crawled") || fakes.linkedSources != nil {
		t.Fatalf("status = %s, issues = %v, notes = %v, linked = %v", got.Status, got.Report.Issues, got.Report.Notes, fakes.linkedSources)
	}
	// Without a website crawl there is no pricing question.
	if len(fakes.previewStarts) != 2 {
		t.Fatalf("check questions = %q", fakes.previewStarts)
	}
}

func TestDemoPreviewUnavailable(t *testing.T) {
	fakes := newDemoFakes()
	fakes.previewErr = errors.New("support preview requires agent tool search_knowledge")
	got := buildDemo(t, fakes, fakes.service(), CreateInput{})
	if got.Status != eerepository.DemoStatusReadyWithIssues || !slices.Equal(got.Report.Issues, []string{IssueVerifyUnavailable, IssueVerifyFailed}) {
		t.Fatalf("status = %s, issues = %v", got.Status, got.Report.Issues)
	}
}

func TestDemoWebsiteComesFromTheRequestOrTheCRM(t *testing.T) {
	tests := []struct {
		name     string
		property string
		input    string
		want     string
	}{
		{name: "domain", want: "https://acme.com"},
		{name: "CRM property", property: "www.acme.io", want: "https://www.acme.io"},
		{name: "bad CRM property falls back to the domain", property: "http://acme.io", want: "https://acme.com"},
		{name: "request wins", property: "https://acme.io", input: "https://acme.dev/en", want: "https://acme.dev/en"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fakes := newDemoFakes()
			if tt.property != "" {
				fakes.companies["company-1"].CustomProperties[PropertyCompanyWebsite] = tt.property
			}
			got := buildDemo(t, fakes, fakes.service(), CreateInput{WebsiteURL: tt.input})
			if got.Report.Website.URL != tt.want || strings.Join(fakes.websites, ",") != tt.want {
				t.Fatalf("website = %q, saved = %v, want %q", got.Report.Website.URL, fakes.websites, tt.want)
			}
		})
	}
}

func TestDemoCreateRefusesBadWebsiteAndTooManyQuestions(t *testing.T) {
	tests := []struct {
		name    string
		input   CreateInput
		wantErr string
	}{
		{name: "plain http", input: CreateInput{WebsiteURL: "http://acme.com"}, wantErr: "website_url"},
		{name: "credentials", input: CreateInput{WebsiteURL: "https://user:pw@acme.com"}, wantErr: "website_url"},
		{name: "six questions", input: CreateInput{VerifyQuestions: []string{"a", "b", "c", "d", "e", "f"}}, wantErr: "at most 5"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fakes := newDemoFakes()
			tt.input.CompanyID = "company-1"
			_, _, err := fakes.service().Create(context.Background(), demoHomeWorkspace, "user-1", tt.input)
			var input *InputError
			if !errors.As(err, &input) || !strings.Contains(input.Message, tt.wantErr) || len(fakes.demos) != 0 {
				t.Fatalf("error = %v, demos = %d", err, len(fakes.demos))
			}
		})
	}
}

func TestDemoBuildsBeyondTheCapWaitInTheQueue(t *testing.T) {
	fakes := newDemoFakes()
	svc := fakes.service()
	svc.cfg.MaxUnfinished = 1
	fakes.companies["company-2"] = &model.CRMCompany{ID: "company-2", WorkspaceID: demoHomeWorkspace, Name: "Beta", Domain: strPtr("beta.com"),
		CustomProperties: model.JSONB{"help_center_url": "https://help.beta.com/"}}
	first, _, err := svc.Create(context.Background(), demoHomeWorkspace, "user-1", CreateInput{CompanyID: "company-1"})
	if err != nil {
		t.Fatal(err)
	}
	second, created, err := svc.Create(context.Background(), demoHomeWorkspace, "user-1", CreateInput{CompanyID: "company-2"})
	if err != nil || !created || second.Status != eerepository.DemoStatusQueued {
		t.Fatalf("second Create = %+v, %v, %v; want it queued, not refused", second, created, err)
	}
	// Run until the first is waiting on its import: the second must not start.
	svc.drain(context.Background())
	if fakes.demos[first.ID].Status != eerepository.DemoStatusBuilding || fakes.demos[second.ID].Status != eerepository.DemoStatusQueued {
		t.Fatalf("first = %s, second = %s", fakes.demos[first.ID].Status, fakes.demos[second.ID].Status)
	}
	fakes.build(t, svc, 120)
	if !fakes.demos[first.ID].Finished() || !fakes.demos[second.ID].Finished() {
		t.Fatalf("first = %s, second = %s", fakes.demos[first.ID].Status, fakes.demos[second.ID].Status)
	}
}

func TestDemoRefreshRerunsTheChecksWithoutImporting(t *testing.T) {
	fakes := newDemoFakes()
	fakes.decisions["Can you help me with Update your card?"] = "handoff"
	svc := fakes.service()
	got := buildDemo(t, fakes, svc, CreateInput{})
	if got.Status != eerepository.DemoStatusReadyWithIssues {
		t.Fatalf("status = %s", got.Status)
	}

	// The answer check is fixed; the refresh must find everything in place.
	delete(fakes.decisions, "Can you help me with Update your card?")
	refreshed, err := svc.Refresh(context.Background(), demoHomeWorkspace, got.ID, CreateInput{})
	if err != nil || refreshed.Status != eerepository.DemoStatusQueued || refreshed.Step != demoStepWebsite ||
		len(refreshed.Report.Issues) != 0 || len(refreshed.Report.Verify) != 0 {
		t.Fatalf("Refresh = %+v, %v", refreshed, err)
	}
	fakes.build(t, svc, 60)
	after, _ := svc.Get(context.Background(), demoHomeWorkspace, got.ID, CreateInput{})
	if after.Status != eerepository.DemoStatusReady || len(fakes.importStarts) != 1 || len(fakes.contentSources) != 1 ||
		len(fakes.knowledgeSets) != 1 || fakes.reindexes != 1 || len(fakes.previewStarts) != 6 {
		t.Fatalf("status = %s, imports = %d, sources = %d, sets = %d, reindexes = %d, questions = %d",
			after.Status, len(fakes.importStarts), len(fakes.contentSources), len(fakes.knowledgeSets), fakes.reindexes, len(fakes.previewStarts))
	}

	// A demo that is still building cannot be refreshed.
	if _, err := svc.Refresh(context.Background(), demoHomeWorkspace, got.ID, CreateInput{}); err != nil {
		t.Fatal(err)
	}
	var input *InputError
	if _, err := svc.Refresh(context.Background(), demoHomeWorkspace, got.ID, CreateInput{}); !errors.As(err, &input) {
		t.Fatalf("second Refresh error = %v, want an input error", err)
	}
}

func TestDemoReusesAWebsiteSourceAddedByHand(t *testing.T) {
	fakes := newDemoFakes()
	fakes.contentSources = []model.SupportContentSource{{ID: "manual", SourceType: model.ContentSourceTypeWebsite,
		StartURL: "https://www.acme.com/", SyncStatus: model.KnowledgeSourceSyncReady, IndexedPages: 40}}
	got := buildDemo(t, fakes, fakes.service(), CreateInput{})
	if len(fakes.contentSources) != 1 || got.Report.Website.SourceID != "manual" || strings.Join(fakes.linkedSources, ",") != "manual" {
		t.Fatalf("sources = %+v, website = %+v", fakes.contentSources, got.Report.Website)
	}
}

func TestArticleQuestion(t *testing.T) {
	for title, want := range map[string]string{
		"How do I export?": "How do I export?",
		"Getting started.": "Can you help me with Getting started?",
		"Refund policy":    "Can you help me with Refund policy?",
	} {
		if got := articleQuestion(title); got != want {
			t.Errorf("articleQuestion(%q) = %q, want %q", title, got, want)
		}
	}
}
