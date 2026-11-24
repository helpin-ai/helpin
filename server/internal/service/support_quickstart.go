package service

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
	"golang.org/x/net/publicsuffix"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// ErrSupportQuickstartInvalid wraps a quickstart input the person can fix.
// Its message is safe to show.
var ErrSupportQuickstartInvalid = errors.New("invalid support setup")

const (
	quickstartDetectTimeout = 20 * time.Second
	quickstartCheckTimeout  = 8 * time.Second
	quickstartCrawlLimit    = 100
	quickstartNameMaxLength = 120
)

// _quickstartCrawlExcludes keeps blog posts out of the crawl: they fill the
// page limit without answering support questions.
var _quickstartCrawlExcludes = []string{"/blog*"}

type quickstartBrand interface {
	ProposeBrand(ctx context.Context, sourceURL string) (*model.HelpcenterBrandProposal, error)
	ApplyBrandProposal(ctx context.Context, workspaceID string, proposal *model.HelpcenterBrandProposal) (*HelpcenterBrandApplyResult, error)
}

type quickstartImports interface {
	CheckPublicHelpCenter(ctx context.Context, rawURL string) (homeURL, name string, err error)
	StartPublicHelpCenter(ctx context.Context, req model.PublicHelpCenterImportRequest, workspaceID, userID string) (string, error)
	GetStatus(ctx context.Context, jobID, workspaceID string) (*model.DocsImportJob, error)
	ListJobs(ctx context.Context, workspaceID string) ([]model.DocsImportJob, error)
	PublicHelpCenterArticles(ctx context.Context, rawURL string) (*model.PublicHelpCenterImportPreview, []model.PublicHelpCenterArticleRef, error)
	ReadPublicArticle(ctx context.Context, articleURL string) (title, text string, err error)
}

type quickstartWorkspaces interface {
	Update(ctx context.Context, id string, req model.UpdateWorkspaceRequest, actorID string) (*model.Workspace, error)
}

type quickstartSupport interface {
	BindDefaultSupportAgent(ctx context.Context, workspaceID string) error
	GetInstallation(ctx context.Context, workspaceID string) (*model.SupportWidgetInstallation, *model.SupportInboxSettings, error)
	UpdateInstallationSettings(ctx context.Context, workspaceID string, req model.UpdateInstallationSettingsRequest) (*model.SupportWidgetInstallation, *model.SupportInboxSettings, error)
	ApplyWebsiteOriginDefaults(ctx context.Context, workspaceID string) error
}

type quickstartContentSources interface {
	List(ctx context.Context, workspaceID string) ([]model.SupportContentSource, error)
	Create(ctx context.Context, workspaceID string, req model.CreateSupportContentSourceRequest) (*model.SupportContentSource, error)
}

type quickstartAgentContentSources interface {
	Set(ctx context.Context, workspaceID, agentID string, contentSourceIDs []string) error
}

type quickstartChunks interface {
	CountBySpaceID(ctx context.Context, workspaceID, spaceID string) (int64, error)
}

type quickstartDocuments interface {
	LatestTitleInSpace(ctx context.Context, workspaceID, spaceID string) (string, error)
}

// SupportQuickstartDeps are the product services a support quickstart drives.
type SupportQuickstartDeps struct {
	Brand               quickstartBrand
	Imports             quickstartImports
	Workspaces          quickstartWorkspaces
	Support             quickstartSupport
	ContentSources      quickstartContentSources
	AgentContentSources quickstartAgentContentSources
	Chunks              quickstartChunks
	Documents           quickstartDocuments
	// AI answers preview questions; nil turns preview answers off.
	AI quickstartAI
	// Pages reads website pages for preview answers; nil uses the shared
	// public-only client.
	Pages WorkspaceContextFetcher
	// Redis caches preview answers and caps them per day; nil skips both.
	Redis *redis.Client
}

// SupportQuickstartService sets up customer support for a workspace from the
// company's website: it copies the help center, crawls the website for the
// AI, brands the help center and chat widget, and turns on the support agent.
type SupportQuickstartService struct {
	deps   SupportQuickstartDeps
	logger *slog.Logger
}

// NewSupportQuickstartService creates the service.
func NewSupportQuickstartService(deps SupportQuickstartDeps) *SupportQuickstartService {
	return &SupportQuickstartService{deps: deps, logger: slog.Default().With("service", "support_quickstart")}
}

// readSite reads a company website for its name and branding. A site that
// cannot be read still returns the address and a name taken from the domain.
// When the address is itself an Intercom help center, that is set as the help
// center. linked is the help center the website links to, if any.
func (s *SupportQuickstartService) readSite(ctx context.Context, website string) (site *model.SupportQuickstartDetection, linked string, err error) {
	ctx, cancel := context.WithTimeout(ctx, quickstartDetectTimeout)
	defer cancel()
	site = &model.SupportQuickstartDetection{WebsiteURL: website, CompanyName: companyNameFromURL(website)}
	proposal, err := s.deps.Brand.ProposeBrand(ctx, website)
	if err != nil {
		// An unreadable site is not an error; running out of time is.
		return site, "", ctx.Err()
	}
	site.WebsiteReadable = true
	site.CompanyName = cmp.Or(strings.TrimSpace(proposal.BrandName), site.CompanyName)
	site.LogoURL, site.FaviconURL, site.BrandColor = cmp.Or(proposal.LogoURL, proposal.LogoDarkURL), proposal.FaviconURL, proposal.BrandColor
	if proposal.Platform == model.HelpcenterBrandPlatformIntercom {
		site.HelpCenter = &model.SupportQuickstartHelpCenter{URL: proposal.SourceURL, Name: proposal.HelpCenterName, Platform: model.QuickstartHelpCenterIntercom}
		site.WebsiteURL = cmp.Or(proposal.WebsiteURL, website)
	}
	return site, proposal.HelpCenterURL, nil
}

// findHelpCenter looks for the company's help center: a public Intercom one
// at the linked address or the usual places, otherwise the linked address.
func (s *SupportQuickstartService) findHelpCenter(ctx context.Context, website, linked string) *model.SupportQuickstartHelpCenter {
	ctx, cancel := context.WithTimeout(ctx, quickstartDetectTimeout)
	defer cancel()
	if home, name := s.findIntercomHelpCenter(ctx, helpCenterCandidates(website, linked)); home != "" {
		return &model.SupportQuickstartHelpCenter{URL: home, Name: name, Platform: model.QuickstartHelpCenterIntercom}
	}
	if linked != "" {
		return &model.SupportQuickstartHelpCenter{URL: linked, Platform: model.QuickstartHelpCenterWebsite}
	}
	return nil
}

// findIntercomHelpCenter checks the candidates side by side and returns the
// first one, in candidate order, that is a public Intercom help center.
func (s *SupportQuickstartService) findIntercomHelpCenter(ctx context.Context, candidates []string) (home, name string) {
	type found struct{ home, name string }
	results := make([]found, len(candidates))
	var wg sync.WaitGroup
	for i, candidate := range candidates {
		wg.Add(1)
		go func() {
			defer wg.Done()
			checkCtx, cancel := context.WithTimeout(ctx, quickstartCheckTimeout)
			defer cancel()
			if home, name, err := s.deps.Imports.CheckPublicHelpCenter(checkCtx, candidate); err == nil {
				results[i] = found{home: home, name: name}
			}
		}()
	}
	wg.Wait()
	for _, result := range results {
		if result.home != "" {
			return result.home, result.name
		}
	}
	return "", ""
}

// Start sets up support for a workspace. Running it again is safe: an import
// already started and sites already crawled are reused.
func (s *SupportQuickstartService) Start(ctx context.Context, workspaceID, userID string, req model.SupportQuickstartStartRequest) (*model.SupportQuickstartStatus, error) {
	website, err := normalizeQuickstartURL(req.WebsiteURL, "website")
	if err != nil {
		return nil, err
	}
	name := strings.TrimSpace(req.CompanyName)
	if name == "" {
		name = companyNameFromURL(website)
	}
	if len(name) > quickstartNameMaxLength {
		return nil, fmt.Errorf("%w: The company name is too long.", ErrSupportQuickstartInvalid)
	}
	mode := cmp.Or(req.AIMode, model.QuickstartAIModeAnswer)
	if mode != model.QuickstartAIModeAnswer && mode != model.QuickstartAIModeDraft {
		return nil, fmt.Errorf("%w: Choose whether the AI answers customers or drafts replies.", ErrSupportQuickstartInvalid)
	}
	helpURL := ""
	if strings.TrimSpace(req.HelpCenterURL) != "" {
		if helpURL, err = normalizeQuickstartURL(req.HelpCenterURL, "help center"); err != nil {
			return nil, err
		}
	}

	if _, err := s.deps.Workspaces.Update(ctx, workspaceID, model.UpdateWorkspaceRequest{WebsiteURL: &website}, userID); err != nil {
		return nil, fmt.Errorf("save workspace website: %w", err)
	}
	// Reading the installation creates it; the website then fills its empty allowlist.
	if _, _, err := s.deps.Support.GetInstallation(ctx, workspaceID); err != nil {
		return nil, fmt.Errorf("read chat widget: %w", err)
	}
	if err := s.deps.Support.ApplyWebsiteOriginDefaults(ctx, workspaceID); err != nil {
		return nil, fmt.Errorf("allow the website for the chat widget: %w", err)
	}

	spaceID, imported, err := s.importHelpCenter(ctx, workspaceID, userID, name, helpURL)
	if err != nil {
		return nil, err
	}
	sourceIDs := make([]string, 0, 2)
	crawl := []struct{ label, address string }{{label: name + " website", address: website}}
	if helpURL != "" && !imported {
		crawl = append(crawl, struct{ label, address string }{label: name + " help center", address: helpURL})
	}
	for _, site := range crawl {
		source, err := s.ensureCrawl(ctx, workspaceID, site.label, site.address)
		if err != nil {
			// The AI still has whatever else was found; a site that cannot be crawled is left out.
			s.logger.WarnContext(ctx, "quickstart crawl not started", "workspace_id", workspaceID, "error", err)
			continue
		}
		sourceIDs = append(sourceIDs, source.ID)
	}

	brandSource := website
	if imported {
		brandSource = helpURL
	}
	cfg := s.applyBrand(ctx, workspaceID, name, brandSource)
	if err := s.configureWidget(ctx, workspaceID, name, mode, spaceID, cfg, sourceIDs); err != nil {
		return nil, err
	}
	s.logger.InfoContext(ctx, "support quickstart started", "workspace_id", workspaceID, "user_id", userID,
		"help_center_imported", imported, "crawled_sites", len(sourceIDs), "ai_mode", mode)
	return s.Status(ctx, workspaceID)
}

// importHelpCenter starts copying an Intercom help center unless one was
// already started. It reports the space the articles go to and whether the
// help center is being copied; any other help center is crawled instead.
func (s *SupportQuickstartService) importHelpCenter(ctx context.Context, workspaceID, userID, name, helpURL string) (string, bool, error) {
	if job, err := s.quickstartImport(ctx, workspaceID); err != nil {
		return "", false, err
	} else if job != nil {
		return derefString(job.SpaceID), true, nil
	}
	if helpURL == "" {
		return "", false, nil
	}
	spaceName := name + " Help Center"
	jobID, err := s.deps.Imports.StartPublicHelpCenter(ctx, model.PublicHelpCenterImportRequest{
		URL: helpURL, NewSpaceName: &spaceName, ImportStatus: "published",
	}, workspaceID, userID)
	if err != nil {
		if ctx.Err() != nil {
			return "", false, ctx.Err()
		}
		// Not an Intercom help center, or imports are unavailable here.
		if !errors.Is(err, ErrPublicImportInvalid) {
			s.logger.WarnContext(ctx, "quickstart help center import not started", "workspace_id", workspaceID, "error", err)
		}
		return "", false, nil
	}
	job, err := s.deps.Imports.GetStatus(ctx, jobID, workspaceID)
	if err != nil {
		return "", false, fmt.Errorf("read help center import: %w", err)
	}
	if job == nil {
		return "", true, nil
	}
	return derefString(job.SpaceID), true, nil
}

// quickstartImport returns the workspace's latest public help center import.
func (s *SupportQuickstartService) quickstartImport(ctx context.Context, workspaceID string) (*model.DocsImportJob, error) {
	jobs, err := s.deps.Imports.ListJobs(ctx, workspaceID)
	if err != nil {
		return nil, fmt.Errorf("list help center imports: %w", err)
	}
	for i := range jobs {
		if jobs[i].Source == docsImportSourceIntercomPublic {
			return &jobs[i], nil
		}
	}
	return nil, nil
}

// ensureCrawl returns the workspace's crawl of a site, starting one unless
// the same site, with or without "www.", is already crawled.
func (s *SupportQuickstartService) ensureCrawl(ctx context.Context, workspaceID, label, address string) (*model.SupportContentSource, error) {
	sources, err := s.deps.ContentSources.List(ctx, workspaceID)
	if err != nil {
		return nil, fmt.Errorf("list content sources: %w", err)
	}
	for i := range sources {
		if sources[i].SourceType == model.ContentSourceTypeWebsite && sameQuickstartPage(sources[i].StartURL, address) {
			return &sources[i], nil
		}
	}
	render := true
	return s.deps.ContentSources.Create(ctx, workspaceID, model.CreateSupportContentSourceRequest{
		Name: label, StartURL: address, CrawlLimit: quickstartCrawlLimit, CrawlDepth: 2,
		CrawlSource: "all", Formats: []string{"markdown"}, Render: &render, ExcludePatterns: _quickstartCrawlExcludes,
		CrawlPurposes: []string{"search", "ai-input"}, MaxAgeSeconds: 86400,
	})
}

// applyBrand copies the company's branding into the help center settings and
// returns the saved settings, or nil when nothing could be read.
func (s *SupportQuickstartService) applyBrand(ctx context.Context, workspaceID, name, source string) *model.DocsHelpcenterConfig {
	proposal, err := s.deps.Brand.ProposeBrand(ctx, source)
	if err != nil {
		s.logger.WarnContext(ctx, "quickstart brand not read", "workspace_id", workspaceID, "error", err)
		proposal = &model.HelpcenterBrandProposal{}
	}
	proposal.BrandName = name
	applied, err := s.deps.Brand.ApplyBrandProposal(ctx, workspaceID, proposal)
	if err != nil {
		s.logger.WarnContext(ctx, "quickstart brand not applied", "workspace_id", workspaceID, "error", err)
		return nil
	}
	return applied.Config
}

// configureWidget names and brands the chat widget, turns on the support
// agent in the chosen mode, and gives it the crawled sites. When the AI
// cannot be turned on, the widget is still configured without it.
func (s *SupportQuickstartService) configureWidget(ctx context.Context, workspaceID, name, mode, spaceID string, cfg *model.DocsHelpcenterConfig, sourceIDs []string) error {
	if err := s.deps.Support.BindDefaultSupportAgent(ctx, workspaceID); err != nil {
		return fmt.Errorf("bind support agent: %w", err)
	}
	_, settings, err := s.deps.Support.GetInstallation(ctx, workspaceID)
	if err != nil {
		return fmt.Errorf("read chat widget: %w", err)
	}
	req := model.UpdateInstallationSettingsRequest{WidgetName: &name}
	if spaceID != "" {
		req.WidgetHelpSpaceIDs = []string{spaceID}
	}
	if cfg != nil {
		if color := strings.TrimSpace(cfg.BrandColor); color != "" && color != "#000000" {
			req.BrandColor, req.ButtonColor = &color, &color
		}
		if mark := strings.TrimSpace(derefString(cfg.FaviconURL)); mark != "" {
			req.LogoURL, req.WidgetAvatarURL = &mark, &mark
		}
	}
	agentID := ""
	if settings != nil {
		agentID = strings.TrimSpace(derefString(settings.AIAgentID))
	}
	if agentID != "" {
		withAI := req
		enabled, threshold, responseMode := true, 0.7, "ai_first"
		if mode == model.QuickstartAIModeDraft {
			responseMode = "internal_note"
		}
		withAI.AIEnabled, withAI.AIAgentID, withAI.AIConfidenceThreshold, withAI.AIResponseMode = &enabled, &agentID, &threshold, &responseMode
		if _, _, err := s.deps.Support.UpdateInstallationSettings(ctx, workspaceID, withAI); err == nil {
			return s.linkSources(ctx, workspaceID, agentID, sourceIDs)
		} else if ctx.Err() != nil {
			return ctx.Err()
		} else {
			s.logger.WarnContext(ctx, "quickstart AI replies not enabled", "workspace_id", workspaceID, "error", err)
		}
	}
	if _, _, err := s.deps.Support.UpdateInstallationSettings(ctx, workspaceID, req); err != nil {
		return fmt.Errorf("configure chat widget: %w", err)
	}
	if agentID == "" {
		return nil
	}
	return s.linkSources(ctx, workspaceID, agentID, sourceIDs)
}

func (s *SupportQuickstartService) linkSources(ctx context.Context, workspaceID, agentID string, sourceIDs []string) error {
	if len(sourceIDs) == 0 {
		return nil
	}
	if err := s.deps.AgentContentSources.Set(ctx, workspaceID, agentID, sourceIDs); err != nil {
		return fmt.Errorf("give the support agent the crawled sites: %w", err)
	}
	return nil
}

// Status reports how far the workspace's support setup has come.
func (s *SupportQuickstartService) Status(ctx context.Context, workspaceID string) (*model.SupportQuickstartStatus, error) {
	out := &model.SupportQuickstartStatus{Sources: []model.SupportQuickstartSource{}, AIMode: model.QuickstartAIModeOff}
	job, err := s.quickstartImport(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	done := true
	if job != nil {
		articles := &model.SupportQuickstartArticles{Status: "importing", Imported: job.Completed, Total: job.Total, Failed: job.Failed}
		switch job.Status {
		case model.DocsImportStatusDone:
			articles.Status = "done"
		case model.DocsImportStatusFailed, model.DocsImportStatusInterrupted:
			articles.Status = "failed"
		default:
			done = false
		}
		out.Articles = articles
		if job.SpaceID != nil {
			chunks, err := s.deps.Chunks.CountBySpaceID(ctx, workspaceID, *job.SpaceID)
			if err != nil {
				return nil, fmt.Errorf("count indexed articles: %w", err)
			}
			out.AgentReady = chunks > 0
			if articles.Status == "importing" {
				// Progress detail only; the counts stand without it.
				if articles.Current, err = s.deps.Documents.LatestTitleInSpace(ctx, workspaceID, *job.SpaceID); err != nil {
					s.logger.WarnContext(ctx, "read latest copied article", "workspace_id", workspaceID, "error", err)
				}
			}
		}
	}
	sources, err := s.deps.ContentSources.List(ctx, workspaceID)
	if err != nil {
		return nil, fmt.Errorf("list content sources: %w", err)
	}
	for _, source := range sources {
		if source.SourceType != model.ContentSourceTypeWebsite {
			continue
		}
		out.Sources = append(out.Sources, model.SupportQuickstartSource{
			Name: source.Name, URL: source.StartURL, Status: source.SyncStatus, IndexedPages: source.IndexedPages,
		})
		out.AgentReady = out.AgentReady || source.IndexedPages > 0
		if !slices.Contains([]string{model.KnowledgeSourceSyncReady, model.KnowledgeSourceSyncFailed, model.KnowledgeSourceSyncDisabled}, source.SyncStatus) {
			done = false
		}
	}
	out.Started = job != nil || len(out.Sources) > 0
	out.Done = out.Started && done
	_, settings, err := s.deps.Support.GetInstallation(ctx, workspaceID)
	if err != nil {
		return nil, fmt.Errorf("read chat widget: %w", err)
	}
	if settings != nil && settings.AIEnabled {
		out.AIMode = model.QuickstartAIModeAnswer
		if settings.AIResponseMode == "internal_note" {
			out.AIMode = model.QuickstartAIModeDraft
		}
	}
	return out, nil
}

// normalizeQuickstartURL accepts "acme.com" or an http(s) address on a
// public-looking host, without credentials, and returns it as https.
func normalizeQuickstartURL(raw, label string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", fmt.Errorf("%w: Enter your %s address.", ErrSupportQuickstartInvalid, label)
	}
	if !strings.Contains(raw, "://") {
		raw = "https://" + raw
	}
	parsed, err := url.Parse(raw)
	invalid := fmt.Errorf("%w: Enter a %s address, such as acme.com.", ErrSupportQuickstartInvalid, label)
	if err != nil || parsed.User != nil || (parsed.Scheme != "https" && parsed.Scheme != "http") {
		return "", invalid
	}
	host := strings.ToLower(parsed.Hostname())
	if _, err := publicsuffix.EffectiveTLDPlusOne(host); err != nil || !strings.Contains(host, ".") || parsed.Port() != "" {
		return "", invalid
	}
	parsed.Scheme, parsed.Host, parsed.RawQuery, parsed.Fragment = "https", host, "", ""
	if parsed.Path == "/" {
		parsed.Path = ""
	}
	return parsed.String(), nil
}

// helpCenterCandidates lists the addresses worth checking for an Intercom
// help center: the website's own help link, then help. and support. on the
// company domain.
func helpCenterCandidates(website, linked string) []string {
	var out []string
	if linked != "" {
		out = append(out, linked)
	}
	parsed, err := url.Parse(website)
	if err != nil {
		return out
	}
	if site, err := publicsuffix.EffectiveTLDPlusOne(parsed.Hostname()); err == nil {
		for _, label := range []string{"help", "support"} {
			if candidate := "https://" + label + "." + site; !slices.ContainsFunc(out, func(existing string) bool { return sameQuickstartSite(existing, candidate) }) {
				out = append(out, candidate)
			}
		}
	}
	return out
}

// sameQuickstartSite reports whether two addresses are on the same host,
// ignoring "www.".
func sameQuickstartSite(a, b string) bool {
	host := func(raw string) string {
		parsed, err := url.Parse(strings.TrimSpace(raw))
		if err != nil {
			return ""
		}
		return strings.TrimPrefix(strings.ToLower(parsed.Hostname()), "www.")
	}
	return host(a) != "" && host(a) == host(b)
}

// sameQuickstartPage reports whether two addresses start the same crawl:
// same host, ignoring "www.", and same path, ignoring a trailing slash. A help
// center at acme.com/help is crawled apart from acme.com.
func sameQuickstartPage(a, b string) bool {
	path := func(raw string) string {
		parsed, err := url.Parse(strings.TrimSpace(raw))
		if err != nil {
			return ""
		}
		return strings.TrimSuffix(parsed.Path, "/")
	}
	return sameQuickstartSite(a, b) && path(a) == path(b)
}

// companyNameFromURL turns "https://www.acme-labs.com" into "Acme Labs".
func companyNameFromURL(raw string) string {
	parsed, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	host := strings.TrimPrefix(strings.ToLower(parsed.Hostname()), "www.")
	if site, err := publicsuffix.EffectiveTLDPlusOne(host); err == nil {
		host = site
	}
	label, _, _ := strings.Cut(host, ".")
	words := strings.FieldsFunc(label, func(r rune) bool { return r == '-' || r == '_' })
	for i, word := range words {
		words[i] = strings.ToUpper(word[:1]) + word[1:]
	}
	return strings.Join(words, " ")
}
