//go:build ee

// Package demo builds demo workspaces: a workspace for a company, filled
// from that company's public help center and branded to match, with the chat
// widget and support agent turned on. It lives apart from ee/service because
// it drives the product's services, which that package must not import.
package demo

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"regexp"
	"slices"
	"strings"
	"time"

	eerepository "github.com/helpin-ai/helpin/server/ee/repository"
	"github.com/helpin-ai/helpin/server/internal/model"
	coreservice "github.com/helpin-ai/helpin/server/internal/service"
)

// Company custom properties a demo build keeps up to date in the home
// workspace's CRM.
const (
	PropertyStatus       = "demo_status"
	PropertyWorkspaceID  = "demo_workspace_id"
	PropertyWorkspaceURL = "demo_workspace_url"
	// PropertyHelpCenterURL is read when a request names no address.
	PropertyHelpCenterURL = "help_center_url"
)

// Build steps, in order. A build stores the step it is on, so a restarted
// server continues where it stopped.
const (
	demoStepOrganization = "organization"
	demoStepWorkspace    = "workspace"
	demoStepBranding     = "branding"
	demoStepImport       = "import"
	demoStepHomepage     = "homepage"
	demoStepWidget       = "widget"
	demoStepCRM          = "crm"
	demoStepDone         = "done"
)

const (
	// demoLease must outlast the slowest single step (reading a slow help
	// center and its website, then copying three images), or a second API
	// instance would start the same step again.
	demoLease        = 15 * time.Minute
	demoPollDelay    = 5 * time.Second
	demoMaxAttempts  = 4
	demoBuildTimeout = 3 * time.Hour
)

// ErrUnavailable means demo provisioning is not offered to the workspace.
var ErrUnavailable = errors.New("demo provisioning is not available in this workspace")

// InputError is a problem with a request that the caller can fix. Its
// message is safe to show.
type InputError struct{ Message string }

func (e *InputError) Error() string { return e.Message }

func demoInputError(format string, args ...any) error {
	return &InputError{Message: fmt.Sprintf(format, args...)}
}

// demoPermanentError ends a build with a message for whoever asked for it.
// Trying the same step again would not help.
type demoPermanentError struct{ message string }

func (e *demoPermanentError) Error() string { return e.message }

func demoPermanent(format string, args ...any) error {
	return &demoPermanentError{message: fmt.Sprintf(format, args...)}
}

// Config says who may ask for demos and who owns them.
type Config struct {
	// HomeWorkspaceIDs lists the workspaces allowed to ask for demos. Empty
	// turns the feature off.
	HomeWorkspaceIDs []string
	// OwnerEmail is the account that owns the demo organization and every
	// demo workspace.
	OwnerEmail string
	// OrganizationName names the organization that holds demo workspaces. It
	// is created for the owner on first use.
	OrganizationName string
	// HelpcenterHostedDomain is the root that hosted help centers live under.
	HelpcenterHostedDomain string
	// MaxUnfinished caps how many builds one home workspace may have queued
	// or running.
	MaxUnfinished int
}

// Enabled reports whether demos can be built at all.
func (c Config) Enabled() bool {
	return len(c.HomeWorkspaceIDs) > 0 && strings.TrimSpace(c.OwnerEmail) != ""
}

// AllowsWorkspace reports whether a workspace may ask for demos.
func (c Config) AllowsWorkspace(workspaceID string) bool {
	return c.Enabled() && workspaceID != "" && slices.Contains(c.HomeWorkspaceIDs, workspaceID)
}

func (c Config) normalized() Config {
	c.OwnerEmail = strings.TrimSpace(c.OwnerEmail)
	if c.OrganizationName = strings.TrimSpace(c.OrganizationName); c.OrganizationName == "" {
		c.OrganizationName = "Demos"
	}
	c.HelpcenterHostedDomain = strings.ToLower(strings.TrimSpace(c.HelpcenterHostedDomain))
	if c.MaxUnfinished <= 0 {
		c.MaxUnfinished = 5
	}
	return c
}

type demoStore interface {
	CreateOrGet(ctx context.Context, demo *eerepository.DemoWorkspace) (*eerepository.DemoWorkspace, bool, error)
	GetByID(ctx context.Context, homeWorkspaceID, id string) (*eerepository.DemoWorkspace, error)
	GetByCompany(ctx context.Context, homeWorkspaceID, companyID string) (*eerepository.DemoWorkspace, error)
	CountUnfinished(ctx context.Context, homeWorkspaceID string) (int64, error)
	ClaimNext(ctx context.Context, now time.Time, lease time.Duration) (*eerepository.DemoWorkspace, error)
	Save(ctx context.Context, demo *eerepository.DemoWorkspace) error
	Requeue(ctx context.Context, homeWorkspaceID, id string) error
}

type demoUsers interface {
	GetByEmail(ctx context.Context, email string) (*model.User, error)
}

type demoOrganizations interface {
	List(ctx context.Context, userID string) ([]model.OrganizationWithRole, error)
	Create(ctx context.Context, req model.CreateOrganizationRequest, ownerID string) (*model.OrganizationWithRole, error)
}

type demoWorkspaces interface {
	Create(ctx context.Context, req model.CreateWorkspaceRequest, ownerID string) (*model.WorkspaceWithRole, error)
}

type demoHelpcenter interface {
	ProposeBrand(ctx context.Context, sourceURL string) (*model.HelpcenterBrandProposal, error)
	ApplyBrandProposal(ctx context.Context, workspaceID string, proposal *model.HelpcenterBrandProposal) (*coreservice.HelpcenterBrandApplyResult, error)
	UpsertConfig(ctx context.Context, workspaceID string, req model.UpdateDocsHelpcenterConfigRequest) (*model.DocsHelpcenterConfig, error)
	GetConfig(ctx context.Context, workspaceID string) (*model.DocsHelpcenterConfig, error)
	GetConfigBySubdomain(ctx context.Context, subdomain string) (*model.DocsHelpcenterConfig, error)
}

type demoImports interface {
	CheckPublicHelpCenter(ctx context.Context, rawURL string) (homeURL, name string, err error)
	StartPublicHelpCenter(ctx context.Context, req model.PublicHelpCenterImportRequest, workspaceID, userID string) (string, error)
	GetStatus(ctx context.Context, jobID, workspaceID string) (*model.DocsImportJob, error)
}

type demoSpaces interface {
	GetUnfiltered(ctx context.Context, id string) (*model.DocsSpaceWithTeams, error)
}

type demoCollections interface {
	List(ctx context.Context, spaceID string) ([]model.DocsCollection, error)
}

type demoSupport interface {
	BindDefaultSupportAgent(ctx context.Context, workspaceID string) error
	GetInstallation(ctx context.Context, workspaceID string) (*model.SupportWidgetInstallation, *model.SupportInboxSettings, error)
	UpdateInstallationSettings(ctx context.Context, workspaceID string, req model.UpdateInstallationSettingsRequest) (*model.SupportWidgetInstallation, *model.SupportInboxSettings, error)
}

type demoCompanies interface {
	GetByID(ctx context.Context, id string) (*model.CRMCompany, error)
	GetByDomain(ctx context.Context, workspaceID, domain string) (*model.CRMCompany, error)
	Update(ctx context.Context, company *model.CRMCompany) error
}

type demoActivities interface {
	Create(ctx context.Context, req model.CreateCRMActivityRequest) (*model.CRMActivity, error)
}

// Deps are the product services a demo build drives.
type Deps struct {
	Store         demoStore
	Users         demoUsers
	Organizations demoOrganizations
	Workspaces    demoWorkspaces
	Helpcenter    demoHelpcenter
	Imports       demoImports
	Spaces        demoSpaces
	Collections   demoCollections
	Support       demoSupport
	Companies     demoCompanies
	Activities    demoActivities
}

// Service builds demo workspaces: it creates a workspace for
// a company in the demo organization, copies the company's public help
// center into it, applies the company's branding, turns on the chat widget
// with the support agent, and records the result on the company in the CRM.
type Service struct {
	cfg    Config
	deps   Deps
	now    func() time.Time
	logger *slog.Logger
}

// NewService creates the service. It builds nothing until
// Run is started.
func NewService(cfg Config, deps Deps) *Service {
	return &Service{
		cfg: cfg.normalized(), deps: deps, now: time.Now,
		logger: slog.Default().With("service", "demo_provisioning"),
	}
}

// AllowsWorkspace reports whether a workspace may ask for demos.
func (s *Service) AllowsWorkspace(workspaceID string) bool {
	return s != nil && s.cfg.AllowsWorkspace(workspaceID)
}

// Report is what a build found and did, for whoever checks the result.
type Report struct {
	HelpCenterName   string                `json:"help_center_name,omitempty"`
	Collections      int                   `json:"collections"`
	ArticlesImported int                   `json:"articles_imported"`
	ArticlesFailed   int                   `json:"articles_failed"`
	LinksRewritten   int                   `json:"links_rewritten"`
	UnresolvedLinks  int                   `json:"unresolved_links"`
	ImagesCopied     int                   `json:"images_copied"`
	FailedImages     int                   `json:"failed_images"`
	FailedArticles   []model.ImportFailure `json:"failed_articles,omitempty"`
	AIRepliesEnabled bool                  `json:"ai_replies_enabled"`
	// Notes lists, in plain words, everything that needs a look by hand.
	Notes []string `json:"notes"`
}

// View is a build as tools report it.
type View struct {
	ID            string     `json:"id"`
	Status        string     `json:"status"`
	Step          string     `json:"step"`
	Error         string     `json:"error,omitempty"`
	CompanyID     string     `json:"company_id"`
	CompanyName   string     `json:"company_name"`
	Domain        string     `json:"domain,omitempty"`
	HelpCenterURL string     `json:"help_center_url"`
	DemoURL       string     `json:"demo_url,omitempty"`
	WorkspaceID   string     `json:"workspace_id,omitempty"`
	ImportJobID   string     `json:"import_job_id,omitempty"`
	Report        Report     `json:"report"`
	CreatedAt     time.Time  `json:"created_at"`
	CompletedAt   *time.Time `json:"completed_at,omitempty"`
}

func demoView(demo *eerepository.DemoWorkspace) *View {
	return &View{
		ID: demo.ID, Status: demo.Status, Step: demo.Step, Error: derefDemoString(demo.Error),
		CompanyID: demo.CompanyID, CompanyName: demo.CompanyName, Domain: demo.Domain,
		HelpCenterURL: demo.HelpCenterURL, DemoURL: derefDemoString(demo.DemoURL),
		WorkspaceID: derefDemoString(demo.WorkspaceID), ImportJobID: derefDemoString(demo.ImportJobID),
		Report: readDemoReport(demo), CreatedAt: demo.CreatedAt, CompletedAt: demo.CompletedAt,
	}
}

func readDemoReport(demo *eerepository.DemoWorkspace) Report {
	var report Report
	if len(demo.Report) > 0 {
		if err := json.Unmarshal(demo.Report, &report); err != nil {
			report = Report{}
		}
	}
	if report.Notes == nil {
		report.Notes = []string{}
	}
	return report
}

func writeDemoReport(demo *eerepository.DemoWorkspace, report Report) {
	encoded, err := json.Marshal(report)
	if err != nil {
		return
	}
	demo.Report = encoded
}

func addDemoNote(demo *eerepository.DemoWorkspace, note string) {
	report := readDemoReport(demo)
	if !slices.Contains(report.Notes, note) {
		report.Notes = append(report.Notes, note)
	}
	writeDemoReport(demo, report)
}

func derefDemoString(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

// CreateInput names the company to build a demo for.
type CreateInput struct {
	// CompanyID or Domain identifies the company in the home workspace's CRM.
	CompanyID string
	Domain    string
	// HelpCenterURL is the company's public help center. Empty reads the
	// company's "help_center_url" property.
	HelpCenterURL string
}

// Create queues a demo build for a company, or returns the build the company
// already has. A failed build is queued again from the step it stopped at.
// created reports whether this call started work.
func (s *Service) Create(ctx context.Context, homeWorkspaceID, userID string, input CreateInput) (view *View, created bool, err error) {
	if !s.AllowsWorkspace(homeWorkspaceID) {
		return nil, false, ErrUnavailable
	}
	company, err := s.findCompany(ctx, homeWorkspaceID, input)
	if err != nil {
		return nil, false, err
	}
	existing, err := s.deps.Store.GetByCompany(ctx, homeWorkspaceID, company.ID)
	if err != nil {
		return nil, false, err
	}
	if existing != nil {
		if existing.Status != eerepository.DemoStatusFailed {
			return demoView(existing), false, nil
		}
		if err := s.deps.Store.Requeue(ctx, homeWorkspaceID, existing.ID); err != nil {
			return nil, false, err
		}
		s.setCompanyProperties(ctx, company.ID, homeWorkspaceID, map[string]any{PropertyStatus: "building"})
		requeued, err := s.deps.Store.GetByID(ctx, homeWorkspaceID, existing.ID)
		if err != nil || requeued == nil {
			return nil, false, fmt.Errorf("reload requeued demo: %w", err)
		}
		if err := s.forgetDeadImport(ctx, requeued); err != nil {
			return nil, false, err
		}
		s.logger.InfoContext(ctx, "demo build requeued", "demo_id", existing.ID, "home_workspace_id", homeWorkspaceID, "step", requeued.Step)
		return demoView(requeued), true, nil
	}

	address := strings.TrimSpace(input.HelpCenterURL)
	if address == "" {
		address, _ = company.CustomProperties[PropertyHelpCenterURL].(string)
	}
	if strings.TrimSpace(address) == "" {
		return nil, false, demoInputError("No help center address is known for %s. Pass help_center_url, or set the company's %q property.", company.Name, PropertyHelpCenterURL)
	}
	homeURL, _, err := s.deps.Imports.CheckPublicHelpCenter(ctx, address)
	if err != nil {
		if errors.Is(err, coreservice.ErrPublicImportInvalid) {
			return nil, false, demoInputError("%s", strings.TrimPrefix(err.Error(), coreservice.ErrPublicImportInvalid.Error()+": "))
		}
		return nil, false, fmt.Errorf("check help center: %w", err)
	}
	unfinished, err := s.deps.Store.CountUnfinished(ctx, homeWorkspaceID)
	if err != nil {
		return nil, false, err
	}
	if unfinished >= int64(s.cfg.MaxUnfinished) {
		return nil, false, demoInputError("%d demos are already being built. Wait for one to finish, then try again.", unfinished)
	}
	demo, isNew, err := s.deps.Store.CreateOrGet(ctx, &eerepository.DemoWorkspace{
		HomeWorkspaceID: homeWorkspaceID, CompanyID: company.ID, CompanyName: strings.TrimSpace(company.Name),
		Domain: normalizeDemoDomain(derefDemoString(company.Domain)), HelpCenterURL: homeURL,
		Status: eerepository.DemoStatusQueued, Step: demoStepOrganization, Report: json.RawMessage(`{}`), RequestedBy: userID,
	})
	if err != nil {
		return nil, false, err
	}
	if isNew {
		s.setCompanyProperties(ctx, company.ID, homeWorkspaceID, map[string]any{PropertyStatus: "building"})
		s.logger.InfoContext(ctx, "demo build queued", "demo_id", demo.ID, "home_workspace_id", homeWorkspaceID, "company_id", company.ID)
	}
	return demoView(demo), isNew, nil
}

// forgetDeadImport lets a requeued build start its import again. An import
// that failed, was stopped, or was lost stays that way, so the build would
// only read the same result and fail again. The space the import filled is
// kept: the new import goes into it and skips the articles already there.
func (s *Service) forgetDeadImport(ctx context.Context, demo *eerepository.DemoWorkspace) error {
	if demo.Step != demoStepImport || demo.ImportJobID == nil {
		return nil
	}
	job, err := s.deps.Imports.GetStatus(ctx, *demo.ImportJobID, derefDemoString(demo.WorkspaceID))
	if err != nil {
		return fmt.Errorf("read import job: %w", err)
	}
	if job != nil && (job.Status == model.DocsImportStatusPending || job.Status == model.DocsImportStatusRunning) {
		return nil
	}
	if job != nil && job.SpaceID != nil {
		demo.SpaceID = job.SpaceID
	}
	demo.ImportJobID = nil
	return s.deps.Store.Save(ctx, demo)
}

// Get returns one build of the home workspace by its ID, or by the company's
// ID or domain.
func (s *Service) Get(ctx context.Context, homeWorkspaceID string, demoID string, input CreateInput) (*View, error) {
	if !s.AllowsWorkspace(homeWorkspaceID) {
		return nil, ErrUnavailable
	}
	var demo *eerepository.DemoWorkspace
	var err error
	if demoID = strings.TrimSpace(demoID); demoID != "" {
		demo, err = s.deps.Store.GetByID(ctx, homeWorkspaceID, demoID)
	} else {
		var company *model.CRMCompany
		if company, err = s.findCompany(ctx, homeWorkspaceID, input); err == nil {
			demo, err = s.deps.Store.GetByCompany(ctx, homeWorkspaceID, company.ID)
		}
	}
	if err != nil {
		return nil, err
	}
	if demo == nil {
		return nil, demoInputError("No demo was found. Start one with create_demo.")
	}
	return demoView(demo), nil
}

// findCompany resolves the company a request names, inside the home
// workspace only.
func (s *Service) findCompany(ctx context.Context, homeWorkspaceID string, input CreateInput) (*model.CRMCompany, error) {
	companyID, domain := strings.TrimSpace(input.CompanyID), normalizeDemoDomain(input.Domain)
	if companyID == "" && domain == "" {
		return nil, demoInputError("Pass company_id or domain to say which company the demo is for.")
	}
	var company *model.CRMCompany
	var err error
	if companyID != "" {
		company, err = s.deps.Companies.GetByID(ctx, companyID)
	} else {
		company, err = s.deps.Companies.GetByDomain(ctx, homeWorkspaceID, domain)
	}
	if err != nil {
		return nil, fmt.Errorf("find company: %w", err)
	}
	if company == nil || company.WorkspaceID != homeWorkspaceID {
		if companyID != "" {
			return nil, demoInputError("No company with that ID exists in this workspace.")
		}
		return nil, demoInputError("No company with the domain %s exists in this workspace. Add it to the CRM first.", domain)
	}
	return company, nil
}

// setCompanyProperties merges values into a company's custom properties. The
// CRM record is a courtesy copy of the build's state, so a failure is logged
// and never fails the build.
func (s *Service) setCompanyProperties(ctx context.Context, companyID, homeWorkspaceID string, values map[string]any) {
	company, err := s.deps.Companies.GetByID(ctx, companyID)
	if err != nil || company == nil || company.WorkspaceID != homeWorkspaceID {
		s.logger.WarnContext(ctx, "demo: company not available for update", "company_id", companyID, "error", err)
		return
	}
	properties := model.JSONB{}
	for key, value := range company.CustomProperties {
		properties[key] = value
	}
	for key, value := range values {
		properties[key] = value
	}
	company.CustomProperties = properties
	if err := s.deps.Companies.Update(ctx, company); err != nil {
		s.logger.WarnContext(ctx, "demo: update company properties", "company_id", companyID, "error", err)
	}
}

var _demoSlugInvalid = regexp.MustCompile(`[^a-z0-9]+`)

// demoSlug turns a company name into the stem of a workspace slug and help
// center subdomain.
func demoSlug(name string) string {
	slug := strings.Trim(_demoSlugInvalid.ReplaceAllString(strings.ToLower(name), "-"), "-")
	if len(slug) > 40 {
		slug = strings.Trim(slug[:40], "-")
	}
	if slug == "" {
		slug = "company"
	}
	return slug
}

// normalizeDemoDomain reduces "https://www.Acme.com/pricing" to "acme.com".
func normalizeDemoDomain(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return ""
	}
	if !strings.Contains(value, "://") {
		value = "https://" + value
	}
	parsed, err := url.Parse(value)
	if err != nil {
		return ""
	}
	return strings.TrimPrefix(parsed.Hostname(), "www.")
}
