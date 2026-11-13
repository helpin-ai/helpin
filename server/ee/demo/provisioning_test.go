//go:build ee

package demo

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	eerepository "github.com/helpin-ai/helpin/server/ee/repository"
	"github.com/helpin-ai/helpin/server/internal/model"
	coreservice "github.com/helpin-ai/helpin/server/internal/service"
)

const (
	demoHomeWorkspace = "home-ws"
	demoOwnerID       = "owner-1"
)

// demoFakes stands in for every product service a build drives and records
// what the build asked of them.
type demoFakes struct {
	now time.Time

	demos map[string]*eerepository.DemoWorkspace

	ownerMissing  bool
	organizations []model.OrganizationWithRole
	orgsCreated   int
	workspaces    []model.CreateWorkspaceRequest
	workspaceErrs int

	configs       map[string]*model.DocsHelpcenterConfig
	takenSubs     map[string]string
	brandErr      error
	brandNotes    []string
	appliedBrand  *model.HelpcenterBrandProposal
	checkErr      error
	importStarts  []model.PublicHelpCenterImportRequest
	importStartEr error
	importJob     *model.DocsImportJob
	nextImportJob *model.DocsImportJob
	importPolls   int
	importDoneAt  int

	collections []model.DocsCollection
	settings    model.SupportInboxSettings
	widgetReqs  []model.UpdateInstallationSettingsRequest
	aiRejected  bool

	companies  map[string]*model.CRMCompany
	activities []model.CreateCRMActivityRequest
}

func newDemoFakes() *demoFakes {
	return &demoFakes{
		now:       time.Date(2026, 11, 11, 10, 0, 0, 0, time.UTC),
		demos:     map[string]*eerepository.DemoWorkspace{},
		configs:   map[string]*model.DocsHelpcenterConfig{},
		takenSubs: map[string]string{},
		companies: map[string]*model.CRMCompany{
			"company-1": {ID: "company-1", WorkspaceID: demoHomeWorkspace, Name: "Acme Inc", Domain: strPtr("acme.com"),
				CustomProperties: model.JSONB{"demo_tier": "A", "help_center_url": "https://help.acme.com/en/"}},
			"company-other": {ID: "company-other", WorkspaceID: "other-ws", Name: "Elsewhere", Domain: strPtr("elsewhere.com")},
		},
		collections: []model.DocsCollection{
			{ID: "c2", Name: "Billing", Slug: "billing", PublicID: "pub2", Position: 1},
			{ID: "c1", Name: "Basics", Slug: "basics", PublicID: "pub1", Position: 0, Description: strPtr("Start here")},
			{ID: "c3", Name: "Refunds", Slug: "refunds", PublicID: "pub3", Position: 0, ParentCollectionID: strPtr("c2")},
		},
		importDoneAt: 2,
	}
}

func strPtr(value string) *string { return &value }

func (f *demoFakes) service() *Service {
	svc := NewService(Config{
		HomeWorkspaceIDs: []string{demoHomeWorkspace}, OwnerEmail: "owner@example.com", HelpcenterHostedDomain: "helpin.test",
	}, Deps{
		Store: f, Users: f, Organizations: demoFakeOrganizations{f}, Workspaces: demoFakeWorkspaces{f}, Helpcenter: f, Imports: f,
		Spaces: f, Collections: demoFakeCollections{f}, Support: f, Companies: demoFakeCompanies{f}, Activities: demoFakeActivities{f},
	})
	svc.now = func() time.Time { return f.now }
	return svc
}

// build runs the worker until the demo is finished or rounds run out,
// moving the clock past every wait and backoff.
func (f *demoFakes) build(t *testing.T, svc *Service, rounds int) {
	t.Helper()
	for range rounds {
		svc.drain(context.Background())
		finished := true
		for _, demo := range f.demos {
			finished = finished && demo.Finished()
		}
		if finished {
			return
		}
		f.now = f.now.Add(2 * time.Minute)
	}
}

// --- store ---

func (f *demoFakes) CreateOrGet(_ context.Context, demo *eerepository.DemoWorkspace) (*eerepository.DemoWorkspace, bool, error) {
	for _, existing := range f.demos {
		if existing.HomeWorkspaceID == demo.HomeWorkspaceID && existing.CompanyID == demo.CompanyID {
			return existing, false, nil
		}
	}
	demo.ID = fmt.Sprintf("demo-%d", len(f.demos)+1)
	demo.CreatedAt = f.now
	f.demos[demo.ID] = demo
	return demo, true, nil
}

func (f *demoFakes) GetByID(_ context.Context, homeWorkspaceID, id string) (*eerepository.DemoWorkspace, error) {
	if demo, ok := f.demos[id]; ok && demo.HomeWorkspaceID == homeWorkspaceID {
		return demo, nil
	}
	return nil, nil
}

func (f *demoFakes) GetByCompany(_ context.Context, homeWorkspaceID, companyID string) (*eerepository.DemoWorkspace, error) {
	for _, demo := range f.demos {
		if demo.HomeWorkspaceID == homeWorkspaceID && demo.CompanyID == companyID {
			return demo, nil
		}
	}
	return nil, nil
}

func (f *demoFakes) CountUnfinished(_ context.Context, homeWorkspaceID string) (int64, error) {
	var count int64
	for _, demo := range f.demos {
		if demo.HomeWorkspaceID == homeWorkspaceID && !demo.Finished() {
			count++
		}
	}
	return count, nil
}

func (f *demoFakes) ClaimNext(_ context.Context, now time.Time, lease time.Duration) (*eerepository.DemoWorkspace, error) {
	for _, demo := range f.demos {
		if !demo.Finished() && (demo.LeaseUntil == nil || !demo.LeaseUntil.After(now)) {
			until := now.Add(lease)
			demo.LeaseUntil = &until
			return demo, nil
		}
	}
	return nil, nil
}

func (f *demoFakes) Save(_ context.Context, demo *eerepository.DemoWorkspace) error {
	f.demos[demo.ID] = demo
	return nil
}

func (f *demoFakes) Requeue(_ context.Context, homeWorkspaceID, id string) error {
	if demo, ok := f.demos[id]; ok && demo.HomeWorkspaceID == homeWorkspaceID && demo.Status == eerepository.DemoStatusFailed {
		demo.Status, demo.Error, demo.Attempts, demo.LeaseUntil, demo.CompletedAt = eerepository.DemoStatusQueued, nil, 0, nil, nil
		demo.StartedAt = nil
	}
	return nil
}

// --- product services ---

func (f *demoFakes) GetByEmail(context.Context, string) (*model.User, error) {
	if f.ownerMissing {
		return nil, nil
	}
	return &model.User{ID: demoOwnerID, Email: "owner@example.com"}, nil
}

type demoFakeOrganizations struct{ f *demoFakes }

func (o demoFakeOrganizations) List(context.Context, string) ([]model.OrganizationWithRole, error) {
	return o.f.organizations, nil
}

func (o demoFakeOrganizations) Create(_ context.Context, req model.CreateOrganizationRequest, _ string) (*model.OrganizationWithRole, error) {
	o.f.orgsCreated++
	org := model.OrganizationWithRole{Organization: model.Organization{ID: "org-1", Name: req.Name, Slug: req.Slug}, Role: model.RoleOwner}
	o.f.organizations = append(o.f.organizations, org)
	return &org, nil
}

type demoFakeWorkspaces struct{ f *demoFakes }

func (w demoFakeWorkspaces) Create(_ context.Context, req model.CreateWorkspaceRequest, ownerID string) (*model.WorkspaceWithRole, error) {
	if w.f.workspaceErrs > 0 {
		w.f.workspaceErrs--
		return nil, errors.New("database is busy")
	}
	if ownerID != demoOwnerID {
		return nil, fmt.Errorf("workspace created for %q, want the demo owner", ownerID)
	}
	w.f.workspaces = append(w.f.workspaces, req)
	return &model.WorkspaceWithRole{Workspace: model.Workspace{ID: fmt.Sprintf("demo-ws-%d", len(w.f.workspaces)), Name: req.Name, Slug: req.Slug}}, nil
}

type demoFakeCollections struct{ f *demoFakes }

func (c demoFakeCollections) List(context.Context, string) ([]model.DocsCollection, error) {
	return c.f.collections, nil
}

func (f *demoFakes) ProposeBrand(context.Context, string) (*model.HelpcenterBrandProposal, error) {
	if f.brandErr != nil {
		return nil, f.brandErr
	}
	return &model.HelpcenterBrandProposal{HelpCenterName: "Acme Help Center", BrandName: "Acme", BrandColor: "#0b17bf", ThemeMode: "dark"}, nil
}

func (f *demoFakes) ApplyBrandProposal(_ context.Context, workspaceID string, proposal *model.HelpcenterBrandProposal) (*coreservice.HelpcenterBrandApplyResult, error) {
	f.appliedBrand = proposal
	cfg := f.configs[workspaceID]
	cfg.BrandColor, cfg.ThemeMode, cfg.FaviconURL = proposal.BrandColor, proposal.ThemeMode, strPtr("https://files/fav.png")
	return &coreservice.HelpcenterBrandApplyResult{Config: cfg, Notes: f.brandNotes}, nil
}

func (f *demoFakes) UpsertConfig(_ context.Context, workspaceID string, req model.UpdateDocsHelpcenterConfigRequest) (*model.DocsHelpcenterConfig, error) {
	cfg := f.configs[workspaceID]
	if cfg == nil {
		cfg = &model.DocsHelpcenterConfig{WorkspaceID: workspaceID}
		f.configs[workspaceID] = cfg
	}
	if req.Subdomain != nil {
		cfg.Subdomain = *req.Subdomain
	}
	if req.BrandName != nil {
		cfg.BrandName = *req.BrandName
	}
	if req.DemoMode != nil {
		cfg.DemoMode = *req.DemoMode
	}
	if req.IsPublished != nil {
		cfg.IsPublished = *req.IsPublished
	}
	if req.OfficialDocsURL != nil {
		cfg.OfficialDocsURL = *req.OfficialDocsURL
	}
	if req.HomepageConfig != nil {
		cfg.HomepageConfig = req.HomepageConfig
	}
	return cfg, nil
}

func (f *demoFakes) GetConfig(_ context.Context, workspaceID string) (*model.DocsHelpcenterConfig, error) {
	return f.configs[workspaceID], nil
}

func (f *demoFakes) GetConfigBySubdomain(_ context.Context, subdomain string) (*model.DocsHelpcenterConfig, error) {
	if owner, taken := f.takenSubs[subdomain]; taken {
		return &model.DocsHelpcenterConfig{WorkspaceID: owner, Subdomain: subdomain}, nil
	}
	return nil, nil
}

func (f *demoFakes) CheckPublicHelpCenter(_ context.Context, rawURL string) (string, string, error) {
	if f.checkErr != nil {
		return "", "", f.checkErr
	}
	return strings.TrimRight(rawURL, "/") + "/", "Acme Help Center", nil
}

func (f *demoFakes) StartPublicHelpCenter(_ context.Context, req model.PublicHelpCenterImportRequest, workspaceID, userID string) (string, error) {
	if f.importStartEr != nil {
		return "", f.importStartEr
	}
	f.importStarts = append(f.importStarts, req)
	if f.nextImportJob != nil {
		f.importJob, f.nextImportJob = f.nextImportJob, nil
	}
	if f.importJob == nil {
		summary, _ := json.Marshal(model.ImportSummary{CollectionsCreated: 3, LinksRewritten: 12, UnresolvedLinksTotal: 1, ImagesCopied: 40, FailedImagesTotal: 2, SourceName: "Acme Help Center"})
		f.importJob = &model.DocsImportJob{ID: "job-1", WorkspaceID: workspaceID, SpaceID: strPtr("space-1"), Status: model.DocsImportStatusRunning,
			Total: 20, Completed: 19, Failed: 1, Summary: summary, Failures: json.RawMessage(`[{"article_id":"9","title":"Broken","error":"gone"}]`)}
	}
	return f.importJob.ID, nil
}

func (f *demoFakes) GetStatus(_ context.Context, jobID, workspaceID string) (*model.DocsImportJob, error) {
	if f.importJob == nil || f.importJob.ID != jobID || f.importJob.WorkspaceID != workspaceID {
		return nil, nil
	}
	f.importPolls++
	job := *f.importJob
	if job.Status == model.DocsImportStatusRunning && f.importPolls >= f.importDoneAt {
		job.Status = model.DocsImportStatusDone
	}
	return &job, nil
}

func (f *demoFakes) GetUnfiltered(_ context.Context, id string) (*model.DocsSpaceWithTeams, error) {
	return &model.DocsSpaceWithTeams{DocsSpace: model.DocsSpace{ID: id, Slug: "acme-inc-help-center"}}, nil
}

func (f *demoFakes) BindDefaultSupportAgent(context.Context, string) error {
	f.settings.AIAgentID = strPtr("agent-echo")
	return nil
}

func (f *demoFakes) GetInstallation(context.Context, string) (*model.SupportWidgetInstallation, *model.SupportInboxSettings, error) {
	settings := f.settings
	return &model.SupportWidgetInstallation{}, &settings, nil
}

func (f *demoFakes) UpdateInstallationSettings(_ context.Context, _ string, req model.UpdateInstallationSettingsRequest) (*model.SupportWidgetInstallation, *model.SupportInboxSettings, error) {
	if f.aiRejected && req.AIEnabled != nil {
		return nil, nil, errors.New("AI replies need a plan")
	}
	f.widgetReqs = append(f.widgetReqs, req)
	return &model.SupportWidgetInstallation{}, &model.SupportInboxSettings{}, nil
}

type demoFakeCompanies struct{ f *demoFakes }

func (c demoFakeCompanies) GetByID(_ context.Context, id string) (*model.CRMCompany, error) {
	return c.f.companies[id], nil
}

func (c demoFakeCompanies) GetByDomain(_ context.Context, workspaceID, domain string) (*model.CRMCompany, error) {
	for _, company := range c.f.companies {
		if company.WorkspaceID == workspaceID && company.Domain != nil && *company.Domain == domain {
			return company, nil
		}
	}
	return nil, nil
}

func (c demoFakeCompanies) Update(_ context.Context, company *model.CRMCompany) error {
	c.f.companies[company.ID] = company
	return nil
}

type demoFakeActivities struct{ f *demoFakes }

func (a demoFakeActivities) Create(_ context.Context, req model.CreateCRMActivityRequest) (*model.CRMActivity, error) {
	a.f.activities = append(a.f.activities, req)
	return &model.CRMActivity{}, nil
}

// --- tests ---

func TestDemoCreateValidatesTheRequest(t *testing.T) {
	tests := []struct {
		name      string
		workspace string
		input     CreateInput
		prepare   func(*demoFakes)
		wantErr   string
		forbidden bool
	}{
		{name: "workspace not allowed", workspace: "other-ws", input: CreateInput{Domain: "acme.com"}, forbidden: true},
		{name: "no company named", workspace: demoHomeWorkspace, wantErr: "company_id or domain"},
		{name: "unknown domain", workspace: demoHomeWorkspace, input: CreateInput{Domain: "nobody.com"}, wantErr: "nobody.com"},
		{name: "company of another workspace", workspace: demoHomeWorkspace, input: CreateInput{CompanyID: "company-other"}, wantErr: "No company with that ID"},
		{
			name: "no help center address", workspace: demoHomeWorkspace, input: CreateInput{CompanyID: "company-1"},
			prepare: func(f *demoFakes) { delete(f.companies["company-1"].CustomProperties, "help_center_url") },
			wantErr: "help_center_url",
		},
		{
			name: "address is not a help center", workspace: demoHomeWorkspace, input: CreateInput{CompanyID: "company-1"},
			prepare: func(f *demoFakes) {
				f.checkErr = fmt.Errorf("%w: this address is not an Intercom help center", coreservice.ErrPublicImportInvalid)
			},
			wantErr: "not an Intercom help center",
		},
		{
			name: "too many builds in progress", workspace: demoHomeWorkspace, input: CreateInput{CompanyID: "company-1"},
			prepare: func(f *demoFakes) {
				for i := range 5 {
					id := fmt.Sprintf("busy-%d", i)
					f.demos[id] = &eerepository.DemoWorkspace{ID: id, HomeWorkspaceID: demoHomeWorkspace, CompanyID: id, Status: eerepository.DemoStatusBuilding}
				}
			},
			wantErr: "already being built",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fakes := newDemoFakes()
			if tt.prepare != nil {
				tt.prepare(fakes)
			}
			before := len(fakes.demos)
			_, _, err := fakes.service().Create(context.Background(), tt.workspace, "user-1", tt.input)
			if tt.forbidden {
				if !errors.Is(err, ErrUnavailable) {
					t.Fatalf("error = %v, want ErrUnavailable", err)
				}
			} else {
				var input *InputError
				if !errors.As(err, &input) || !strings.Contains(input.Message, tt.wantErr) {
					t.Fatalf("error = %v, want an input error containing %q", err, tt.wantErr)
				}
			}
			if len(fakes.demos) != before {
				t.Fatalf("a refused request stored a build")
			}
			if status := fakes.companies["company-1"].CustomProperties[PropertyStatus]; status != nil {
				t.Fatalf("a refused request set demo_status to %v", status)
			}
		})
	}
}

func TestDemoBuildsEndToEnd(t *testing.T) {
	fakes := newDemoFakes()
	fakes.brandNotes = []string{"The favicon could not be copied. Upload it in the branding settings."}
	// Another workspace already uses the first-choice address.
	fakes.takenSubs["acme-inc-demo"] = "someone-else"
	svc := fakes.service()

	// The company is found by a messy domain; the address comes from the CRM.
	view, created, err := svc.Create(context.Background(), demoHomeWorkspace, "user-1", CreateInput{Domain: "https://www.ACME.com/pricing"})
	if err != nil || !created {
		t.Fatalf("Create = %+v, %v, %v", view, created, err)
	}
	if view.Status != eerepository.DemoStatusQueued || view.HelpCenterURL != "https://help.acme.com/en/" || view.Domain != "acme.com" {
		t.Fatalf("queued view = %+v", view)
	}
	company := fakes.companies["company-1"]
	if company.CustomProperties[PropertyStatus] != "building" || company.CustomProperties["demo_tier"] != "A" {
		t.Fatalf("company properties after create = %v", company.CustomProperties)
	}

	// Asking again returns the same build and starts nothing new.
	again, createdAgain, err := svc.Create(context.Background(), demoHomeWorkspace, "user-1", CreateInput{CompanyID: "company-1"})
	if err != nil || createdAgain || again.ID != view.ID || len(fakes.demos) != 1 {
		t.Fatalf("second Create = %+v, %v, %v", again, createdAgain, err)
	}

	fakes.build(t, svc, 10)
	got, err := svc.Get(context.Background(), demoHomeWorkspace, view.ID, CreateInput{})
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Status != eerepository.DemoStatusReady || got.Step != demoStepDone || got.Error != "" || got.CompletedAt == nil {
		t.Fatalf("finished view = %+v", got)
	}
	if got.DemoURL != "https://acme-inc-demo-2.helpin.test" || got.WorkspaceID != "demo-ws-1" || got.ImportJobID != "job-1" {
		t.Fatalf("demo address = %q, workspace = %q, job = %q", got.DemoURL, got.WorkspaceID, got.ImportJobID)
	}

	// One organization and one workspace, owned by the demo owner, with no
	// website that would allowlist the company's own site for the widget.
	if fakes.orgsCreated != 1 || len(fakes.workspaces) != 1 {
		t.Fatalf("organizations created = %d, workspaces = %d", fakes.orgsCreated, len(fakes.workspaces))
	}
	workspace := fakes.workspaces[0]
	if workspace.Name != "Acme Inc" || workspace.Slug != "acme-inc-demo" || workspace.OrganizationID != "org-1" || workspace.WebsiteURL != nil {
		t.Fatalf("workspace request = %+v", workspace)
	}
	cfg := fakes.configs["demo-ws-1"]
	if !cfg.DemoMode || !cfg.IsPublished || cfg.Subdomain != "acme-inc-demo-2" || cfg.OfficialDocsURL != "https://help.acme.com/en/" || cfg.BrandName != "Acme Inc" {
		t.Fatalf("help center config = %+v", cfg)
	}
	// The CRM's name for the company wins over the one the site publishes.
	if fakes.appliedBrand == nil || fakes.appliedBrand.BrandName != "Acme Inc" || fakes.appliedBrand.BrandColor != "#0b17bf" {
		t.Fatalf("applied brand = %+v", fakes.appliedBrand)
	}
	if len(fakes.importStarts) != 1 || fakes.importStarts[0].URL != "https://help.acme.com/en/" ||
		*fakes.importStarts[0].NewSpaceName != "Acme Inc Help Center" || fakes.importStarts[0].ImportStatus != "published" {
		t.Fatalf("import starts = %+v", fakes.importStarts)
	}
	var homepage model.HelpcenterHomepageConfig
	if err := json.Unmarshal(cfg.HomepageConfig, &homepage); err != nil {
		t.Fatal(err)
	}
	// Top-level collections only, in display order.
	if len(homepage.FeaturedCards) != 2 || homepage.FeaturedCards[0].Title != "Basics" || homepage.FeaturedCards[1].Title != "Billing" ||
		homepage.FeaturedCards[0].LinkType != "collection" || homepage.FeaturedCards[0].LinkValue != "basics" ||
		homepage.FeaturedCards[0].PublicID != "pub1" || homepage.FeaturedCards[0].SpaceSlug != "acme-inc-help-center" ||
		homepage.FeaturedCards[0].Description != "Start here" {
		t.Fatalf("featured cards = %+v", homepage.FeaturedCards)
	}
	if len(fakes.widgetReqs) != 1 {
		t.Fatalf("widget updates = %d", len(fakes.widgetReqs))
	}
	widget := fakes.widgetReqs[0]
	if widget.AIEnabled == nil || !*widget.AIEnabled || *widget.AIAgentID != "agent-echo" || *widget.AIConfidenceThreshold != 0.7 ||
		*widget.WidgetName != "Acme Inc" || *widget.AIDisplayName != "Acme Inc Assistant" || widget.WidgetHelpSpaceIDs[0] != "space-1" ||
		*widget.BrandColor != "#0b17bf" || *widget.ColorScheme != "dark" || *widget.LogoURL != "https://files/fav.png" || widget.AllowedOrigins != nil {
		t.Fatalf("widget request = %+v", widget)
	}

	report := got.Report
	if report.ArticlesImported != 19 || report.ArticlesFailed != 1 || report.Collections != 3 || report.LinksRewritten != 12 ||
		report.UnresolvedLinks != 1 || report.ImagesCopied != 40 || report.FailedImages != 2 || !report.AIRepliesEnabled ||
		report.HelpCenterName != "Acme Help Center" || len(report.FailedArticles) != 1 {
		t.Fatalf("report = %+v", report)
	}
	if len(report.Notes) != 4 {
		t.Fatalf("notes = %v, want the brand note plus failed articles, unresolved links and failed images", report.Notes)
	}

	properties := fakes.companies["company-1"].CustomProperties
	if properties[PropertyStatus] != "ready" || properties[PropertyWorkspaceID] != "demo-ws-1" ||
		properties[PropertyWorkspaceURL] != "https://acme-inc-demo-2.helpin.test" || properties["demo_tier"] != "A" {
		t.Fatalf("company properties = %v", properties)
	}
	if len(fakes.activities) != 1 || *fakes.activities[0].CompanyID != "company-1" || fakes.activities[0].WorkspaceID != demoHomeWorkspace ||
		!strings.Contains(*fakes.activities[0].Body, "Articles imported: 19 (1 failed)") || !strings.Contains(*fakes.activities[0].Body, "Needs a look:") {
		t.Fatalf("CRM note = %+v", fakes.activities)
	}

	// The build is reachable by company too, and never from another workspace.
	if byDomain, err := svc.Get(context.Background(), demoHomeWorkspace, "", CreateInput{Domain: "acme.com"}); err != nil || byDomain.ID != view.ID {
		t.Fatalf("Get by domain = %+v, %v", byDomain, err)
	}
	if _, err := svc.Get(context.Background(), "other-ws", view.ID, CreateInput{}); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("Get from another workspace error = %v", err)
	}
}

func TestDemoReusesTheOwnersOrganization(t *testing.T) {
	fakes := newDemoFakes()
	fakes.organizations = []model.OrganizationWithRole{
		{Organization: model.Organization{ID: "org-member", Name: "Demos"}, Role: model.RoleMember},
		{Organization: model.Organization{ID: "org-existing", Name: "demos"}, Role: model.RoleOwner},
	}
	svc := fakes.service()
	if _, _, err := svc.Create(context.Background(), demoHomeWorkspace, "user-1", CreateInput{CompanyID: "company-1"}); err != nil {
		t.Fatal(err)
	}
	fakes.build(t, svc, 10)
	if fakes.orgsCreated != 0 || fakes.workspaces[0].OrganizationID != "org-existing" {
		t.Fatalf("organizations created = %d, workspace organization = %q", fakes.orgsCreated, fakes.workspaces[0].OrganizationID)
	}
}

func TestDemoRetriesABriefFailureWithoutRepeatingFinishedSteps(t *testing.T) {
	fakes := newDemoFakes()
	fakes.workspaceErrs = 2
	svc := fakes.service()
	view, _, err := svc.Create(context.Background(), demoHomeWorkspace, "user-1", CreateInput{CompanyID: "company-1"})
	if err != nil {
		t.Fatal(err)
	}
	svc.drain(context.Background())
	demo := fakes.demos[view.ID]
	if demo.Status != eerepository.DemoStatusBuilding || demo.Step != demoStepWorkspace || demo.Attempts != 1 || demo.LeaseUntil.Sub(fakes.now) != 10*time.Second {
		t.Fatalf("after first failure: status=%s step=%s attempts=%d wait=%v", demo.Status, demo.Step, demo.Attempts, demo.LeaseUntil.Sub(fakes.now))
	}
	// Not due yet: nothing happens.
	svc.drain(context.Background())
	if demo.Attempts != 1 {
		t.Fatalf("the build was retried before its backoff ended")
	}
	fakes.build(t, svc, 10)
	if demo.Status != eerepository.DemoStatusReady || fakes.orgsCreated != 1 || len(fakes.workspaces) != 1 {
		t.Fatalf("status=%s organizations=%d workspaces=%d", demo.Status, fakes.orgsCreated, len(fakes.workspaces))
	}
}

func TestDemoFailsAfterRepeatedFailuresAndContinuesOnRequest(t *testing.T) {
	fakes := newDemoFakes()
	fakes.workspaceErrs = demoMaxAttempts
	svc := fakes.service()
	view, _, err := svc.Create(context.Background(), demoHomeWorkspace, "user-1", CreateInput{CompanyID: "company-1"})
	if err != nil {
		t.Fatal(err)
	}
	fakes.build(t, svc, 10)
	demo := fakes.demos[view.ID]
	if demo.Status != eerepository.DemoStatusFailed || !strings.Contains(*demo.Error, "workspace step") || strings.Contains(*demo.Error, "database is busy") {
		t.Fatalf("status=%s error=%v; want a failure that names the step and hides internals", demo.Status, demo.Error)
	}
	if fakes.companies["company-1"].CustomProperties[PropertyStatus] != "failed" {
		t.Fatalf("company demo_status = %v", fakes.companies["company-1"].CustomProperties[PropertyStatus])
	}

	// create_demo again continues from the failed step, keeping the
	// organization that was already found.
	again, created, err := svc.Create(context.Background(), demoHomeWorkspace, "user-1", CreateInput{CompanyID: "company-1"})
	if err != nil || !created || again.ID != view.ID || again.Status != eerepository.DemoStatusQueued || again.Step != demoStepWorkspace {
		t.Fatalf("retry Create = %+v, %v, %v", again, created, err)
	}
	fakes.build(t, svc, 10)
	if demo.Status != eerepository.DemoStatusReady || fakes.orgsCreated != 1 || len(fakes.workspaces) != 1 {
		t.Fatalf("after retry: status=%s organizations=%d workspaces=%d", demo.Status, fakes.orgsCreated, len(fakes.workspaces))
	}
}

func TestDemoFailureCases(t *testing.T) {
	tests := []struct {
		name     string
		prepare  func(*demoFakes)
		wantStep string
		wantErr  string
	}{
		{
			name: "owner account missing", prepare: func(f *demoFakes) { f.ownerMissing = true },
			wantStep: demoStepOrganization, wantErr: "account that owns demos",
		},
		{
			name: "import failed", wantStep: demoStepImport, wantErr: "import failed: the help center did not respond",
			prepare: func(f *demoFakes) {
				f.importJob = &model.DocsImportJob{ID: "job-1", WorkspaceID: "demo-ws-1", Status: model.DocsImportStatusFailed, Error: strPtr("the help center did not respond")}
			},
		},
		{
			name: "import brought nothing", wantStep: demoStepImport, wantErr: "without any articles",
			prepare: func(f *demoFakes) {
				f.importJob = &model.DocsImportJob{ID: "job-1", WorkspaceID: "demo-ws-1", SpaceID: strPtr("space-1"), Status: model.DocsImportStatusDone}
			},
		},
		{
			name: "import refused the address", wantStep: demoStepImport, wantErr: "asks visitors to sign in",
			prepare: func(f *demoFakes) {
				f.importStartEr = fmt.Errorf("%w: this help center asks visitors to sign in", coreservice.ErrPublicImportInvalid)
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fakes := newDemoFakes()
			tt.prepare(fakes)
			svc := fakes.service()
			view, _, err := svc.Create(context.Background(), demoHomeWorkspace, "user-1", CreateInput{CompanyID: "company-1"})
			if err != nil {
				t.Fatal(err)
			}
			fakes.build(t, svc, 10)
			demo := fakes.demos[view.ID]
			if demo.Status != eerepository.DemoStatusFailed || demo.Step != tt.wantStep || demo.Error == nil || !strings.Contains(*demo.Error, tt.wantErr) {
				t.Fatalf("status=%s step=%s error=%v; want failed at %s with %q", demo.Status, demo.Step, demo.Error, tt.wantStep, tt.wantErr)
			}
			if len(fakes.activities) != 0 {
				t.Fatalf("a failed build added a CRM note")
			}
		})
	}
}

func TestDemoRestartsADeadImportIntoTheSameSpace(t *testing.T) {
	fakes := newDemoFakes()
	fakes.importJob = &model.DocsImportJob{ID: "job-1", WorkspaceID: "demo-ws-1", SpaceID: strPtr("space-1"), Status: model.DocsImportStatusFailed, Error: strPtr("the help center did not respond")}
	svc := fakes.service()
	view, _, err := svc.Create(context.Background(), demoHomeWorkspace, "user-1", CreateInput{CompanyID: "company-1"})
	if err != nil {
		t.Fatal(err)
	}
	fakes.build(t, svc, 10)
	demo := fakes.demos[view.ID]
	if demo.Status != eerepository.DemoStatusFailed || demo.Step != demoStepImport {
		t.Fatalf("status=%s step=%s", demo.Status, demo.Step)
	}

	// Days later the help center is back. Continuing must start a fresh
	// import, in the space the first one made, and must not be judged
	// against the first run's time limit.
	fakes.now = fakes.now.Add(72 * time.Hour)
	summary, _ := json.Marshal(model.ImportSummary{CollectionsCreated: 3})
	fakes.nextImportJob = &model.DocsImportJob{ID: "job-2", WorkspaceID: "demo-ws-1", SpaceID: strPtr("space-1"), Status: model.DocsImportStatusDone, Total: 5, Completed: 5, Summary: summary}
	again, created, err := svc.Create(context.Background(), demoHomeWorkspace, "user-1", CreateInput{CompanyID: "company-1"})
	if err != nil || !created || again.ImportJobID != "" {
		t.Fatalf("retry Create = %+v, %v, %v; want the dead import forgotten", again, created, err)
	}
	fakes.build(t, svc, 10)
	if demo.Status != eerepository.DemoStatusReady || derefDemoString(demo.ImportJobID) != "job-2" {
		t.Fatalf("after retry: status=%s error=%v job=%v", demo.Status, derefDemoString(demo.Error), derefDemoString(demo.ImportJobID))
	}
	if len(fakes.importStarts) != 2 || fakes.importStarts[0].NewSpaceName == nil ||
		derefDemoString(fakes.importStarts[1].TargetSpaceID) != "space-1" || fakes.importStarts[1].NewSpaceName != nil {
		t.Fatalf("restarted import = %+v; want it in the existing space", fakes.importStarts)
	}
	if len(fakes.workspaces) != 1 {
		t.Fatalf("workspaces created = %d", len(fakes.workspaces))
	}
}

func TestDemoKeepsARunningImportOnRequeue(t *testing.T) {
	fakes := newDemoFakes()
	svc := fakes.service()
	demo := &eerepository.DemoWorkspace{ID: "demo-1", HomeWorkspaceID: demoHomeWorkspace, CompanyID: "company-1", CompanyName: "Acme Inc",
		Status: eerepository.DemoStatusQueued, Step: demoStepImport, WorkspaceID: strPtr("demo-ws-1"), ImportJobID: strPtr("job-1")}
	fakes.importJob = &model.DocsImportJob{ID: "job-1", WorkspaceID: "demo-ws-1", Status: model.DocsImportStatusRunning}
	fakes.importDoneAt = 1 << 30
	if err := svc.forgetDeadImport(context.Background(), demo); err != nil || derefDemoString(demo.ImportJobID) != "job-1" {
		t.Fatalf("a running import was forgotten: %v, %v", demo.ImportJobID, err)
	}
}

func TestDemoDegradesInsteadOfFailing(t *testing.T) {
	fakes := newDemoFakes()
	fakes.brandErr = errors.New("site unreachable")
	fakes.aiRejected = true
	svc := fakes.service()
	view, _, err := svc.Create(context.Background(), demoHomeWorkspace, "user-1", CreateInput{CompanyID: "company-1"})
	if err != nil {
		t.Fatal(err)
	}
	fakes.build(t, svc, 10)
	got, err := svc.Get(context.Background(), demoHomeWorkspace, view.ID, CreateInput{})
	if err != nil || got.Status != eerepository.DemoStatusReady {
		t.Fatalf("Get = %+v, %v", got, err)
	}
	if got.Report.AIRepliesEnabled || len(fakes.widgetReqs) != 1 || fakes.widgetReqs[0].AIEnabled != nil || *fakes.widgetReqs[0].WidgetName != "Acme Inc" {
		t.Fatalf("widget = %+v, report = %+v", fakes.widgetReqs, got.Report)
	}
	notes := strings.Join(got.Report.Notes, "\n")
	if !strings.Contains(notes, "branding could not be read") || !strings.Contains(notes, "AI replies could not be turned on") {
		t.Fatalf("notes = %q", notes)
	}
	if !strings.Contains(*fakes.activities[0].Body, "AI replies off") {
		t.Fatalf("CRM note = %q", *fakes.activities[0].Body)
	}
}

func TestDemoStopsABuildThatRunsTooLong(t *testing.T) {
	fakes := newDemoFakes()
	fakes.importDoneAt = 1 << 30 // never finishes
	svc := fakes.service()
	view, _, err := svc.Create(context.Background(), demoHomeWorkspace, "user-1", CreateInput{CompanyID: "company-1"})
	if err != nil {
		t.Fatal(err)
	}
	fakes.build(t, svc, 120) // four hours of two-minute rounds
	demo := fakes.demos[view.ID]
	if demo.Status != eerepository.DemoStatusFailed || !strings.Contains(*demo.Error, "took too long") || len(fakes.importStarts) != 1 {
		t.Fatalf("status=%s error=%v import starts=%d", demo.Status, demo.Error, len(fakes.importStarts))
	}
}

func TestDemoMCPTools(t *testing.T) {
	fakes := newDemoFakes()
	svc := fakes.service()
	tools := svc.MCPTools()
	if len(tools) != 2 || tools[0].Definition.Name != ToolCreate || tools[1].Definition.Name != ToolGet {
		t.Fatalf("tools = %+v", tools)
	}
	for _, tool := range tools {
		if !tool.Available(demoHomeWorkspace) || tool.Available("other-ws") || tool.Available("") {
			t.Fatalf("%s availability is not limited to the home workspace", tool.Definition.Name)
		}
	}
	if !tools[0].Definition.Mutating || tools[1].Definition.Mutating {
		t.Fatal("create_demo must be a write and get_demo a read")
	}
	call := coreservice.MCPToolCall{WorkspaceID: demoHomeWorkspace, UserID: "user-1"}

	result, err := tools[0].Execute(context.Background(), call, json.RawMessage(`{"domain":"acme.com"}`))
	if err != nil || !strings.Contains(result.Summary, "Building a demo for Acme Inc") {
		t.Fatalf("create_demo = %+v, %v", result, err)
	}
	demoID := result.Data.(*View).ID
	fakes.build(t, svc, 10)
	result, err = tools[1].Execute(context.Background(), call, json.RawMessage(`{"demo_id":"`+demoID+`"}`))
	if err != nil || result.Summary != "The demo for Acme Inc is ready." || result.Links["demo"] != "https://acme-inc-demo.helpin.test" {
		t.Fatalf("get_demo = %+v, %v", result, err)
	}

	// A problem the caller can fix is worded for the caller.
	_, err = tools[0].Execute(context.Background(), call, json.RawMessage(`{"domain":"nobody.com"}`))
	var toolErr *coreservice.MCPToolError
	if !errors.As(err, &toolErr) || toolErr.Code != "invalid_request" || !strings.Contains(toolErr.Message, "nobody.com") {
		t.Fatalf("create_demo error = %v", err)
	}
	// A call from a workspace that may not build demos is refused.
	_, err = tools[0].Execute(context.Background(), coreservice.MCPToolCall{WorkspaceID: "other-ws"}, json.RawMessage(`{"domain":"acme.com"}`))
	if !errors.Is(err, coreservice.ErrMCPForbidden) {
		t.Fatalf("create_demo from another workspace error = %v", err)
	}
}

func TestDemoConfig(t *testing.T) {
	if (Config{}).Enabled() || (Config{HomeWorkspaceIDs: []string{"a"}}).Enabled() ||
		(Config{OwnerEmail: "o@example.com"}).Enabled() {
		t.Fatal("demo provisioning is on without both a home workspace and an owner")
	}
	cfg := Config{HomeWorkspaceIDs: []string{"a"}, OwnerEmail: " o@example.com "}.normalized()
	if !cfg.Enabled() || cfg.OrganizationName != "Demos" || cfg.MaxUnfinished != 5 || cfg.OwnerEmail != "o@example.com" ||
		!cfg.AllowsWorkspace("a") || cfg.AllowsWorkspace("b") {
		t.Fatalf("config = %+v", cfg)
	}
	var disabled *Service
	if disabled.AllowsWorkspace("a") {
		t.Fatal("a nil service allows a workspace")
	}
}
