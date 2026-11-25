package service

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"net/url"
	"slices"
	"strings"
	"sync"
	"testing"

	"golang.org/x/net/html"

	"github.com/helpin-ai/helpin/server/internal/model"
)

type fakeQuickstart struct {
	proposals    map[string]*model.HelpcenterBrandProposal
	intercom     map[string]string // checked address → home address
	platform     string            // platform of those help centers; Intercom when empty
	jobs         []model.DocsImportJob
	sources      []model.SupportContentSource
	settings     model.SupportInboxSettings
	updates      []model.UpdateInstallationSettingsRequest
	linked       []string
	imports      int
	website      string
	brandApplied string
	brandSources []string // every address branding was read from
	chunks       int64
	latestTitle  string
	articles     []model.PublicHelpCenterArticleRef
	articleText  map[string]string
	articleReads []string
	listings     int
	mu           sync.Mutex
}

func (f *fakeQuickstart) ProposeBrand(_ context.Context, source string) (*model.HelpcenterBrandProposal, error) {
	f.mu.Lock()
	f.brandSources = append(f.brandSources, source)
	f.mu.Unlock()
	if p, ok := f.proposals[source]; ok {
		copied := *p
		return &copied, nil
	}
	return nil, fmt.Errorf("%w: the site did not respond", ErrHelpcenterBrandInvalid)
}

func (f *fakeQuickstart) ApplyBrandProposal(_ context.Context, _ string, p *model.HelpcenterBrandProposal) (*HelpcenterBrandApplyResult, error) {
	f.brandApplied = p.BrandName
	favicon := "https://cdn.example/favicon.png"
	return &HelpcenterBrandApplyResult{Config: &model.DocsHelpcenterConfig{BrandColor: p.BrandColor, FaviconURL: &favicon}}, nil
}

func (f *fakeQuickstart) CheckPublicHelpCenter(_ context.Context, raw string) (*model.PublicHelpCenterSite, error) {
	if home, ok := f.intercom[raw]; ok {
		return &model.PublicHelpCenterSite{URL: home, Name: "Acme Help Center", Platform: cmp.Or(f.platform, model.PublicHelpCenterPlatformIntercom)}, nil
	}
	return nil, fmt.Errorf("%w: not a help center", ErrPublicImportInvalid)
}

func (f *fakeQuickstart) StartPublicHelpCenter(ctx context.Context, req model.PublicHelpCenterImportRequest, workspaceID, _ string) (string, error) {
	found, err := f.CheckPublicHelpCenter(ctx, req.URL)
	if err != nil {
		return "", err
	}
	f.imports++
	space := "space-1"
	f.jobs = append([]model.DocsImportJob{{ID: "job-1", WorkspaceID: workspaceID, SpaceID: &space, Source: found.Platform + publicImportSourceSuffix, Status: model.DocsImportStatusRunning, Total: 40, Completed: 3}}, f.jobs...)
	return "job-1", nil
}

func (f *fakeQuickstart) GetStatus(_ context.Context, jobID, _ string) (*model.DocsImportJob, error) {
	for i := range f.jobs {
		if f.jobs[i].ID == jobID {
			return &f.jobs[i], nil
		}
	}
	return nil, nil
}

func (f *fakeQuickstart) ListJobs(context.Context, string) ([]model.DocsImportJob, error) {
	return f.jobs, nil
}

func (f *fakeQuickstart) PublicHelpCenterArticles(_ context.Context, rawURL string) (*model.PublicHelpCenterImportPreview, []model.PublicHelpCenterArticleRef, error) {
	if len(f.articles) == 0 {
		return nil, nil, fmt.Errorf("%w: not Intercom", ErrPublicImportInvalid)
	}
	f.mu.Lock()
	f.listings++
	f.mu.Unlock()
	return &model.PublicHelpCenterImportPreview{URL: rawURL, Source: docsImportSourceIntercomPublic, Articles: len(f.articles), Collections: 2}, f.articles, nil
}

func (f *fakeQuickstart) ReadPublicArticle(_ context.Context, source, articleURL string) (string, string, error) {
	if source != docsImportSourceIntercomPublic {
		return "", "", fmt.Errorf("%w: unknown source %q", ErrPublicImportInvalid, source)
	}
	f.mu.Lock()
	f.articleReads = append(f.articleReads, articleURL)
	f.mu.Unlock()
	text, ok := f.articleText[articleURL]
	if !ok {
		return "", "", fmt.Errorf("%w: no article", ErrPublicImportInvalid)
	}
	return "", text, nil
}

func (f *fakeQuickstart) Update(_ context.Context, _ string, req model.UpdateWorkspaceRequest, _ string) (*model.Workspace, error) {
	f.website = *req.WebsiteURL
	return &model.Workspace{}, nil
}

func (f *fakeQuickstart) BindDefaultSupportAgent(context.Context, string) error {
	agent := "agent-1"
	f.settings.AIAgentID = &agent
	return nil
}

func (f *fakeQuickstart) GetInstallation(context.Context, string) (*model.SupportWidgetInstallation, *model.SupportInboxSettings, error) {
	settings := f.settings
	return &model.SupportWidgetInstallation{}, &settings, nil
}

func (f *fakeQuickstart) UpdateInstallationSettings(_ context.Context, _ string, req model.UpdateInstallationSettingsRequest) (*model.SupportWidgetInstallation, *model.SupportInboxSettings, error) {
	f.updates = append(f.updates, req)
	if req.AIEnabled != nil {
		f.settings.AIEnabled, f.settings.AIResponseMode = *req.AIEnabled, *req.AIResponseMode
	}
	return nil, &f.settings, nil
}

func (f *fakeQuickstart) ApplyWebsiteOriginDefaults(context.Context, string) error { return nil }

func (f *fakeQuickstart) List(context.Context, string) ([]model.SupportContentSource, error) {
	return f.sources, nil
}

func (f *fakeQuickstart) Create(_ context.Context, _ string, req model.CreateSupportContentSourceRequest) (*model.SupportContentSource, error) {
	source := model.SupportContentSource{ID: fmt.Sprintf("source-%d", len(f.sources)+1), Name: req.Name, StartURL: req.StartURL, SourceType: model.ContentSourceTypeWebsite, SyncStatus: model.KnowledgeSourceSyncQueued}
	f.sources = append(f.sources, source)
	return &source, nil
}

func (f *fakeQuickstart) Set(_ context.Context, _, _ string, ids []string) error {
	f.linked = ids
	return nil
}

func (f *fakeQuickstart) LatestTitleInSpace(context.Context, string, string) (string, error) {
	return f.latestTitle, nil
}

func (f *fakeQuickstart) CountBySpaceID(context.Context, string, string) (int64, error) {
	return f.chunks, nil
}

func newQuickstartForTest(f *fakeQuickstart) *SupportQuickstartService {
	return NewSupportQuickstartService(SupportQuickstartDeps{
		Brand: f, Imports: f, Workspaces: f, Support: f, ContentSources: f, AgentContentSources: f, Chunks: f, Documents: f,
	})
}

func TestSupportQuickstartStartImportsIntercomOnce(t *testing.T) {
	f := &fakeQuickstart{intercom: map[string]string{"https://help.acme.com": "https://help.acme.com/en"}}
	svc := newQuickstartForTest(f)
	req := model.SupportQuickstartStartRequest{WebsiteURL: "acme.com", CompanyName: "Acme", HelpCenterURL: "help.acme.com"}
	for range 2 {
		if _, err := svc.Start(context.Background(), "ws-1", "user-1", req); err != nil {
			t.Fatalf("Start() error = %v", err)
		}
	}
	if f.imports != 1 {
		t.Errorf("imports started = %d, want 1 across two runs", f.imports)
	}
	if len(f.sources) != 1 || f.sources[0].StartURL != "https://acme.com" {
		t.Errorf("crawls = %+v, want only the website", f.sources)
	}
	last := f.updates[len(f.updates)-1]
	if len(last.WidgetHelpSpaceIDs) != 1 || last.WidgetHelpSpaceIDs[0] != "space-1" || *last.WidgetName != "Acme" {
		t.Errorf("widget = %+v, want the imported space and the company name", last)
	}
	if *last.AIResponseMode != "ai_first" || len(f.linked) != 1 || f.website != "https://acme.com" {
		t.Errorf("AI mode %q, linked %v, website %q", *last.AIResponseMode, f.linked, f.website)
	}
}

func TestSupportQuickstartStartCopiesAnyPlatformThatCanBeRead(t *testing.T) {
	tests := []struct {
		platform  string
		wantBrand string
	}{
		// An Intercom help center publishes its branding; for the others the
		// website says more.
		{platform: model.PublicHelpCenterPlatformIntercom, wantBrand: "https://help.acme.com"},
		{platform: "zendesk", wantBrand: "https://acme.com"},
		{platform: "mintlify", wantBrand: "https://acme.com"},
	}
	for _, tt := range tests {
		t.Run(tt.platform, func(t *testing.T) {
			f := &fakeQuickstart{platform: tt.platform, intercom: map[string]string{"https://help.acme.com": "https://help.acme.com/home"}}
			status, err := newQuickstartForTest(f).Start(context.Background(), "ws-1", "user-1",
				model.SupportQuickstartStartRequest{WebsiteURL: "acme.com", CompanyName: "Acme", HelpCenterURL: "help.acme.com"})
			if err != nil {
				t.Fatalf("Start() error = %v", err)
			}
			if f.imports != 1 || len(f.sources) != 1 || f.sources[0].StartURL != "https://acme.com" {
				t.Errorf("imports %d, crawls %+v; want the help center copied and only the website crawled", f.imports, f.sources)
			}
			if status.Articles == nil || status.Articles.Total != 40 {
				t.Errorf("status = %+v, want the copy's progress", status)
			}
			if got := f.brandSources[len(f.brandSources)-1]; got != tt.wantBrand {
				t.Errorf("branding read from %q, want %q", got, tt.wantBrand)
			}
		})
	}
}

func TestHelpCenterCandidates(t *testing.T) {
	tests := []struct {
		name, website, linked string
		want                  []string
	}{
		{
			name: "a company website and its help link", website: "https://www.acme.com", linked: "https://support.acme.com/hc",
			want: []string{"https://support.acme.com/hc", "https://help.acme.com", "https://knowledge.acme.com", "https://docs.acme.com"},
		},
		{
			name: "an address that is itself the help center", website: "https://kb.acme.com",
			want: []string{
				"https://kb.acme.com", "https://help.acme.com", "https://support.acme.com", "https://knowledge.acme.com", "https://docs.acme.com",
			},
		},
		{
			name: "a help center under a path", website: "https://acme.com/docs", linked: "https://acme.com/docs/",
			want: []string{
				"https://acme.com/docs", "https://help.acme.com", "https://support.acme.com", "https://knowledge.acme.com", "https://docs.acme.com",
			},
		},
		{
			// help.zendesk.com and support.zendesk.com are Zendesk's own.
			name: "an address on a help center platform's domain", website: "https://acme.zendesk.com",
			want: []string{"https://acme.zendesk.com"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := helpCenterCandidates(tt.website, tt.linked); !slices.Equal(got, tt.want) {
				t.Errorf("helpCenterCandidates = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSupportQuickstartStartCrawlsOtherHelpCenterInDraftMode(t *testing.T) {
	f := &fakeQuickstart{}
	status, err := newQuickstartForTest(f).Start(context.Background(), "ws-1", "user-1", model.SupportQuickstartStartRequest{
		WebsiteURL: "acme.com", HelpCenterURL: "https://acme.zendesk.com/hc", AIMode: model.QuickstartAIModeDraft,
	})
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	if f.imports != 0 || len(f.sources) != 2 || len(f.linked) != 2 {
		t.Errorf("imports %d, crawls %+v, linked %v; want both sites crawled and linked", f.imports, f.sources, f.linked)
	}
	if f.brandApplied != "Acme" || status.AIMode != model.QuickstartAIModeDraft || !status.Started || status.Done {
		t.Errorf("brand %q, status %+v", f.brandApplied, status)
	}
}

func TestSupportQuickstartStartRejectsBadInput(t *testing.T) {
	tests := []model.SupportQuickstartStartRequest{
		{WebsiteURL: ""},
		{WebsiteURL: "localhost"},
		{WebsiteURL: "https://user:pw@acme.com"},
		{WebsiteURL: "acme.com", AIMode: "sometimes"},
		{WebsiteURL: "acme.com", CompanyName: strings.Repeat("a", quickstartNameMaxLength+1)},
	}
	for _, req := range tests {
		if _, err := newQuickstartForTest(&fakeQuickstart{}).Start(context.Background(), "ws", "u", req); !errors.Is(err, ErrSupportQuickstartInvalid) {
			t.Errorf("Start(%+v) error = %v, want ErrSupportQuickstartInvalid", req, err)
		}
	}
}

func TestSupportQuickstartStatusReportsReadiness(t *testing.T) {
	space := "space-1"
	f := &fakeQuickstart{
		jobs:    []model.DocsImportJob{{Source: docsImportSourceIntercomPublic, Status: model.DocsImportStatusDone, SpaceID: &space, Completed: 40, Total: 40}},
		sources: []model.SupportContentSource{{Name: "Acme website", SourceType: model.ContentSourceTypeWebsite, SyncStatus: model.KnowledgeSourceSyncReady, IndexedPages: 12}},
		chunks:  0,
	}
	status, err := newQuickstartForTest(f).Status(context.Background(), "ws-1")
	if err != nil {
		t.Fatalf("Status() error = %v", err)
	}
	if !status.Done || !status.AgentReady || status.Articles.Status != "done" || status.AIMode != model.QuickstartAIModeOff {
		t.Errorf("Status() = %+v, want done and ready from the website pages", status)
	}
}

func TestSupportQuickstartStatusNamesTheArticleBeingCopied(t *testing.T) {
	space := "space-1"
	f := &fakeQuickstart{
		jobs:        []model.DocsImportJob{{Source: docsImportSourceIntercomPublic, Status: model.DocsImportStatusRunning, SpaceID: &space, Completed: 12, Total: 40}},
		latestTitle: "How refunds work",
	}
	status, err := newQuickstartForTest(f).Status(context.Background(), "ws-1")
	if err != nil {
		t.Fatalf("Status() error = %v", err)
	}
	if status.Done || status.Articles.Current != "How refunds work" || status.Articles.Imported != 12 {
		t.Errorf("Status() articles = %+v, want the copy in progress with its latest article", status.Articles)
	}
	f.jobs[0].Status = model.DocsImportStatusDone
	if status, _ = newQuickstartForTest(f).Status(context.Background(), "ws-1"); status.Articles.Current != "" {
		t.Errorf("finished copy still names an article: %q", status.Articles.Current)
	}
}

func TestHelpCenterLink(t *testing.T) {
	base, _ := url.Parse("https://www.acme.com/")
	tests := []struct {
		name, anchor, want string
		score              int
	}{
		{name: "help subdomain", anchor: `<a href="https://help.acme.com/en/?utm=x">Learn</a>`, want: "https://help.acme.com/en/", score: 3},
		{name: "labelled platform", anchor: `<a href="https://acme.intercom.help">Help Center</a>`, want: "https://acme.intercom.help", score: 4},
		{name: "help path", anchor: `<a href="/support/articles">Read more</a>`, want: "https://www.acme.com/support/articles", score: 2},
		{name: "other company", anchor: `<a href="https://help.other.com">Help</a>`, score: 0},
		{name: "pricing", anchor: `<a href="/pricing">Pricing</a>`, score: 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			doc, err := html.Parse(strings.NewReader(tt.anchor))
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			var anchor *html.Node
			var find func(*html.Node)
			find = func(n *html.Node) {
				if n.Type == html.ElementNode && n.Data == "a" {
					anchor = n
				}
				for c := n.FirstChild; c != nil; c = c.NextSibling {
					find(c)
				}
			}
			find(doc)
			got, score := helpCenterLink(anchor, base)
			if score != tt.score || (tt.score > 0 && got != tt.want) {
				t.Errorf("helpCenterLink() = %q, %d; want %q, %d", got, score, tt.want, tt.score)
			}
		})
	}
}

func TestSupportQuickstartStartCrawlsHelpPathApartFromWebsite(t *testing.T) {
	f := &fakeQuickstart{}
	if _, err := newQuickstartForTest(f).Start(context.Background(), "ws-1", "user-1", model.SupportQuickstartStartRequest{
		WebsiteURL: "https://www.acme.com/", HelpCenterURL: "https://acme.com/docs/",
	}); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	if len(f.sources) != 2 {
		t.Errorf("crawls = %+v, want the website and its /docs help center", f.sources)
	}
}
