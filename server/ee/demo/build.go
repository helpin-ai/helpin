//go:build ee

package demo

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	eerepository "github.com/helpin-ai/helpin/server/ee/repository"
	"github.com/helpin-ai/helpin/server/internal/model"
	coreservice "github.com/helpin-ai/helpin/server/internal/service"
)

// Run builds queued demos until ctx ends. Each build is leased, so several
// API instances can run this loop without advancing the same build twice.
func (s *Service) Run(ctx context.Context, interval time.Duration) {
	if s == nil || !s.cfg.Enabled() {
		return
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		s.drain(ctx)
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

// drain advances every build that is due.
func (s *Service) drain(ctx context.Context) {
	for ctx.Err() == nil {
		demo, err := s.deps.Store.ClaimNext(ctx, s.now(), demoLease)
		if err != nil {
			if ctx.Err() == nil {
				s.logger.ErrorContext(ctx, "claim demo build", "error", err)
			}
			return
		}
		if demo == nil {
			return
		}
		s.advance(ctx, demo)
	}
}

// demoStepResult says what a finished step leads to.
type demoStepResult struct {
	next string
	// wait means the step is waiting on outside work and should run again
	// after a short delay.
	wait bool
}

// advance runs a build's steps until one has to wait, one fails, or the
// build is done. The build is saved after every step.
func (s *Service) advance(ctx context.Context, demo *eerepository.DemoWorkspace) {
	now := s.now()
	if demo.StartedAt == nil {
		demo.StartedAt = &now
	}
	demo.Status = eerepository.DemoStatusBuilding
	if now.Sub(*demo.StartedAt) > demoBuildTimeout {
		s.fail(ctx, demo, "The demo took too long to build and was stopped. Run create_demo again to continue.")
		return
	}
	for ctx.Err() == nil {
		step, ok := s.steps()[demo.Step]
		if !ok {
			s.fail(ctx, demo, "The demo is in an unknown state and cannot continue.")
			return
		}
		result, err := step(ctx, demo)
		if err != nil {
			s.stepFailed(ctx, demo, err)
			return
		}
		if result.wait {
			s.release(ctx, demo, demoPollDelay)
			return
		}
		demo.Step, demo.Attempts = result.next, 0
		if demo.Step == demoStepDone {
			done := s.now()
			demo.Status, demo.CompletedAt, demo.LeaseUntil, demo.Error = eerepository.DemoStatusReady, &done, nil, nil
			s.save(ctx, demo)
			s.logger.InfoContext(ctx, "demo build ready", "demo_id", demo.ID, "workspace_id", derefDemoString(demo.WorkspaceID))
			return
		}
		until := s.now().Add(demoLease)
		demo.LeaseUntil = &until
		if !s.save(ctx, demo) {
			return
		}
	}
}

func (s *Service) steps() map[string]func(context.Context, *eerepository.DemoWorkspace) (demoStepResult, error) {
	return map[string]func(context.Context, *eerepository.DemoWorkspace) (demoStepResult, error){
		demoStepOrganization: s.stepOrganization,
		demoStepWorkspace:    s.stepWorkspace,
		demoStepBranding:     s.stepBranding,
		demoStepImport:       s.stepImport,
		demoStepHomepage:     s.stepHomepage,
		demoStepWidget:       s.stepWidget,
		demoStepCRM:          s.stepCRM,
	}
}

func (s *Service) save(ctx context.Context, demo *eerepository.DemoWorkspace) bool {
	// A build's state must be stored even while the server shuts down.
	if err := s.deps.Store.Save(context.WithoutCancel(ctx), demo); err != nil {
		s.logger.ErrorContext(ctx, "save demo build", "demo_id", demo.ID, "error", err)
		return false
	}
	return true
}

// release hands the build back to the queue for another pass after delay.
func (s *Service) release(ctx context.Context, demo *eerepository.DemoWorkspace, delay time.Duration) {
	until := s.now().Add(delay)
	demo.LeaseUntil = &until
	s.save(ctx, demo)
}

// stepFailed retries a step that may succeed later and ends the build on one
// that cannot.
func (s *Service) stepFailed(ctx context.Context, demo *eerepository.DemoWorkspace, err error) {
	if ctx.Err() != nil {
		// Shutting down: leave the build leased; it resumes when the lease ends.
		return
	}
	var permanent *demoPermanentError
	if errors.As(err, &permanent) {
		s.fail(ctx, demo, permanent.message)
		return
	}
	demo.Attempts++
	s.logger.WarnContext(ctx, "demo build step failed", "demo_id", demo.ID, "step", demo.Step, "attempt", demo.Attempts, "error", err)
	if demo.Attempts >= demoMaxAttempts {
		s.fail(ctx, demo, fmt.Sprintf("The demo could not be finished (it stopped at the %s step). Run create_demo again to continue from there.", demo.Step))
		return
	}
	// 10s, 30s, 90s.
	backoff := 10 * time.Second
	for range demo.Attempts - 1 {
		backoff *= 3
	}
	s.release(ctx, demo, backoff)
}

func (s *Service) fail(ctx context.Context, demo *eerepository.DemoWorkspace, message string) {
	done := s.now()
	demo.Status, demo.Error, demo.CompletedAt, demo.LeaseUntil = eerepository.DemoStatusFailed, &message, &done, nil
	s.save(ctx, demo)
	s.setCompanyProperties(context.WithoutCancel(ctx), demo.CompanyID, demo.HomeWorkspaceID, map[string]any{PropertyStatus: "failed"})
	s.logger.WarnContext(ctx, "demo build failed", "demo_id", demo.ID, "step", demo.Step, "reason", message)
}

// owner returns the account that owns demo workspaces.
func (s *Service) owner(ctx context.Context) (*model.User, error) {
	user, err := s.deps.Users.GetByEmail(ctx, s.cfg.OwnerEmail)
	if err != nil {
		return nil, fmt.Errorf("find demo owner: %w", err)
	}
	if user == nil {
		return nil, demoPermanent("The account that owns demos does not exist yet. Ask an administrator to finish the demo setup.")
	}
	return user, nil
}

// stepOrganization finds the owner's demo organization, or creates it.
func (s *Service) stepOrganization(ctx context.Context, demo *eerepository.DemoWorkspace) (demoStepResult, error) {
	owner, err := s.owner(ctx)
	if err != nil {
		return demoStepResult{}, err
	}
	organizations, err := s.deps.Organizations.List(ctx, owner.ID)
	if err != nil {
		return demoStepResult{}, fmt.Errorf("list owner organizations: %w", err)
	}
	for _, organization := range organizations {
		if organization.Role == model.RoleOwner && strings.EqualFold(strings.TrimSpace(organization.Name), s.cfg.OrganizationName) {
			demo.OrganizationID = &organization.ID
			return demoStepResult{next: demoStepWorkspace}, nil
		}
	}
	created, err := s.deps.Organizations.Create(ctx, model.CreateOrganizationRequest{
		Name: s.cfg.OrganizationName, Slug: demoSlug(s.cfg.OrganizationName),
	}, owner.ID)
	if err != nil {
		return demoStepResult{}, fmt.Errorf("create demo organization: %w", err)
	}
	demo.OrganizationID = &created.ID
	s.logger.InfoContext(ctx, "demo organization created", "organization_id", created.ID)
	return demoStepResult{next: demoStepWorkspace}, nil
}

// stepWorkspace creates the demo workspace, owned by the demo owner. No
// website is set on it: a website would put the company's own site on the
// chat widget's allowlist.
func (s *Service) stepWorkspace(ctx context.Context, demo *eerepository.DemoWorkspace) (demoStepResult, error) {
	if demo.WorkspaceID != nil {
		return demoStepResult{next: demoStepBranding}, nil
	}
	owner, err := s.owner(ctx)
	if err != nil {
		return demoStepResult{}, err
	}
	workspace, err := s.deps.Workspaces.Create(ctx, model.CreateWorkspaceRequest{
		Name: demo.CompanyName, Slug: demoSlug(demo.CompanyName) + "-demo", OrganizationID: derefDemoString(demo.OrganizationID),
	}, owner.ID)
	if err != nil {
		return demoStepResult{}, fmt.Errorf("create demo workspace: %w", err)
	}
	demo.WorkspaceID = &workspace.ID
	return demoStepResult{next: demoStepBranding}, nil
}

// stepBranding publishes the demo help center under its own subdomain in
// demo mode and applies the branding the company's help center shows.
func (s *Service) stepBranding(ctx context.Context, demo *eerepository.DemoWorkspace) (demoStepResult, error) {
	workspaceID := derefDemoString(demo.WorkspaceID)
	subdomain, err := s.pickSubdomain(ctx, demo)
	if err != nil {
		return demoStepResult{}, err
	}
	enabled, hosted := true, model.HelpcenterPublicURLModeHostedSubdomain
	seoTitle := demo.CompanyName + " Help Center"
	req := model.UpdateDocsHelpcenterConfigRequest{
		Subdomain: &subdomain, PublicURLMode: &hosted, BrandName: &demo.CompanyName, SEOTitle: &seoTitle,
		// Demo mode keeps the copy out of search engines and labels it as a
		// demo with a link to the real help center.
		DemoMode: &enabled, IsPublished: &enabled, ChatWidgetEnabled: &enabled, AIAnswersEnabled: &enabled,
	}
	if strings.HasPrefix(demo.HelpCenterURL, "https://") {
		req.OfficialDocsURL = &demo.HelpCenterURL
	}
	if _, err := s.deps.Helpcenter.UpsertConfig(ctx, workspaceID, req); err != nil {
		return demoStepResult{}, fmt.Errorf("publish demo help center: %w", err)
	}
	demo.Subdomain = &subdomain
	if s.cfg.HelpcenterHostedDomain != "" {
		demoURL := "https://" + subdomain + "." + s.cfg.HelpcenterHostedDomain
		demo.DemoURL = &demoURL
	}

	proposal, err := s.deps.Helpcenter.ProposeBrand(ctx, demo.HelpCenterURL)
	if err != nil {
		if ctx.Err() != nil {
			return demoStepResult{}, ctx.Err()
		}
		// A demo without the brand is still a demo; say what is missing.
		s.logger.WarnContext(ctx, "demo: brand not read", "demo_id", demo.ID, "error", err)
		addDemoNote(demo, "The company's branding could not be read. Set the logo, color and favicon in the help center settings.")
		return demoStepResult{next: demoStepImport}, nil
	}
	report := readDemoReport(demo)
	report.HelpCenterName = proposal.HelpCenterName
	writeDemoReport(demo, report)
	// The CRM holds the name the team uses for the company.
	proposal.BrandName = demo.CompanyName
	applied, err := s.deps.Helpcenter.ApplyBrandProposal(ctx, workspaceID, proposal)
	if err != nil {
		return demoStepResult{}, fmt.Errorf("apply demo branding: %w", err)
	}
	for _, note := range applied.Notes {
		addDemoNote(demo, note)
	}
	return demoStepResult{next: demoStepImport}, nil
}

// pickSubdomain returns "<company>-demo", or the first numbered variant that
// no other workspace's help center uses.
func (s *Service) pickSubdomain(ctx context.Context, demo *eerepository.DemoWorkspace) (string, error) {
	if demo.Subdomain != nil {
		return *demo.Subdomain, nil
	}
	base := demoSlug(demo.CompanyName) + "-demo"
	for attempt := 1; attempt <= 20; attempt++ {
		candidate := base
		if attempt > 1 {
			candidate = fmt.Sprintf("%s-%d", base, attempt)
		}
		existing, err := s.deps.Helpcenter.GetConfigBySubdomain(ctx, candidate)
		if err != nil {
			return "", fmt.Errorf("check help center subdomain: %w", err)
		}
		if existing == nil || existing.WorkspaceID == derefDemoString(demo.WorkspaceID) {
			return candidate, nil
		}
	}
	return "", demoPermanent("No free help center address was found for %s.", demo.CompanyName)
}

// stepImport copies the company's public help center into the demo and
// waits for the copy to finish.
func (s *Service) stepImport(ctx context.Context, demo *eerepository.DemoWorkspace) (demoStepResult, error) {
	workspaceID := derefDemoString(demo.WorkspaceID)
	if demo.ImportJobID == nil {
		owner, err := s.owner(ctx)
		if err != nil {
			return demoStepResult{}, err
		}
		req := model.PublicHelpCenterImportRequest{URL: demo.HelpCenterURL, ImportStatus: "published"}
		if demo.SpaceID != nil {
			// A restarted import continues in the space the first one made.
			req.TargetSpaceID = demo.SpaceID
		} else {
			spaceName := demo.CompanyName + " Help Center"
			req.NewSpaceName = &spaceName
		}
		jobID, err := s.deps.Imports.StartPublicHelpCenter(ctx, req, workspaceID, owner.ID)
		if err != nil {
			if errors.Is(err, coreservice.ErrPublicImportInvalid) {
				return demoStepResult{}, demoPermanent("%s", strings.TrimPrefix(err.Error(), coreservice.ErrPublicImportInvalid.Error()+": "))
			}
			return demoStepResult{}, fmt.Errorf("start help center import: %w", err)
		}
		demo.ImportJobID = &jobID
		// Stored before waiting, so a restart never starts a second copy.
		if !s.save(ctx, demo) {
			return demoStepResult{}, fmt.Errorf("store import job")
		}
	}
	job, err := s.deps.Imports.GetStatus(ctx, *demo.ImportJobID, workspaceID)
	if err != nil {
		return demoStepResult{}, fmt.Errorf("read import job: %w", err)
	}
	if job == nil {
		return demoStepResult{}, demoPermanent("The help center import was lost. Run create_demo again to restart it.")
	}
	switch job.Status {
	case model.DocsImportStatusPending, model.DocsImportStatusRunning:
		return demoStepResult{wait: true}, nil
	case model.DocsImportStatusInterrupted:
		return demoStepResult{}, demoPermanent("The help center import was stopped before it finished.")
	case model.DocsImportStatusFailed:
		reason := strings.TrimSpace(derefDemoString(job.Error))
		if reason == "" {
			reason = "none of its articles could be imported."
		}
		return demoStepResult{}, demoPermanent("The help center import failed: %s", reason)
	}
	demo.SpaceID = job.SpaceID
	s.recordImport(demo, job)
	if job.Completed == 0 {
		return demoStepResult{}, demoPermanent("The help center import finished without any articles.")
	}
	return demoStepResult{next: demoStepHomepage}, nil
}

// recordImport copies the import's outcome into the build's report.
func (s *Service) recordImport(demo *eerepository.DemoWorkspace, job *model.DocsImportJob) {
	report := readDemoReport(demo)
	report.ArticlesImported, report.ArticlesFailed = job.Completed, job.Failed
	var summary model.ImportSummary
	if len(job.Summary) > 0 && json.Unmarshal(job.Summary, &summary) == nil {
		report.Collections = summary.CollectionsCreated
		report.LinksRewritten, report.UnresolvedLinks = summary.LinksRewritten, summary.UnresolvedLinksTotal
		report.ImagesCopied, report.FailedImages = summary.ImagesCopied, summary.FailedImagesTotal
		if report.HelpCenterName == "" {
			report.HelpCenterName = summary.SourceName
		}
	}
	var failures []model.ImportFailure
	if len(job.Failures) > 0 && json.Unmarshal(job.Failures, &failures) == nil {
		report.FailedArticles = failures[:min(len(failures), 20)]
	}
	writeDemoReport(demo, report)
	if job.Failed > 0 {
		addDemoNote(demo, fmt.Sprintf("%d articles could not be imported. They are listed under failed_articles.", job.Failed))
	}
	if report.UnresolvedLinks > 0 {
		addDemoNote(demo, fmt.Sprintf("%d links still point at the original help center because their target was not found.", report.UnresolvedLinks))
	}
	if report.FailedImages > 0 {
		addDemoNote(demo, fmt.Sprintf("%d images could not be copied and still load from the original help center.", report.FailedImages))
	}
}

// stepHomepage puts one card per top-level collection on the demo's home
// page, in the order the collections appear.
func (s *Service) stepHomepage(ctx context.Context, demo *eerepository.DemoWorkspace) (demoStepResult, error) {
	workspaceID, spaceID := derefDemoString(demo.WorkspaceID), derefDemoString(demo.SpaceID)
	space, err := s.deps.Spaces.GetUnfiltered(ctx, spaceID)
	if err != nil {
		return demoStepResult{}, fmt.Errorf("read demo space: %w", err)
	}
	if space == nil {
		return demoStepResult{}, demoPermanent("The imported help center space no longer exists.")
	}
	collections, err := s.deps.Collections.List(ctx, spaceID)
	if err != nil {
		return demoStepResult{}, fmt.Errorf("list demo collections: %w", err)
	}
	top := make([]model.DocsCollection, 0, len(collections))
	for _, collection := range collections {
		if collection.ParentCollectionID == nil {
			top = append(top, collection)
		}
	}
	slices.SortStableFunc(top, func(a, b model.DocsCollection) int { return a.Position - b.Position })
	homepage := model.HelpcenterHomepageConfig{}
	if cfg, err := s.deps.Helpcenter.GetConfig(ctx, workspaceID); err != nil {
		return demoStepResult{}, fmt.Errorf("read demo help center: %w", err)
	} else if cfg != nil && len(cfg.HomepageConfig) > 0 {
		if err := json.Unmarshal(cfg.HomepageConfig, &homepage); err != nil {
			homepage = model.HelpcenterHomepageConfig{}
		}
	}
	homepage.FeaturedCards = make([]model.HomepageFeaturedCard, 0, len(top))
	for _, collection := range top {
		homepage.FeaturedCards = append(homepage.FeaturedCards, model.HomepageFeaturedCard{
			Title: collection.Name, Description: derefDemoString(collection.Description), Icon: derefDemoString(collection.Icon),
			LinkType: "collection", LinkValue: collection.Slug, SpaceSlug: space.Slug, PublicID: collection.PublicID,
		})
	}
	encoded, err := json.Marshal(homepage)
	if err != nil {
		return demoStepResult{}, fmt.Errorf("encode demo homepage: %w", err)
	}
	if _, err := s.deps.Helpcenter.UpsertConfig(ctx, workspaceID, model.UpdateDocsHelpcenterConfigRequest{HomepageConfig: encoded}); err != nil {
		return demoStepResult{}, fmt.Errorf("save demo homepage: %w", err)
	}
	return demoStepResult{next: demoStepWidget}, nil
}

// stepWidget turns on the chat widget's AI replies with the built-in support
// agent and gives the widget the demo's help articles and branding.
func (s *Service) stepWidget(ctx context.Context, demo *eerepository.DemoWorkspace) (demoStepResult, error) {
	workspaceID := derefDemoString(demo.WorkspaceID)
	if err := s.deps.Support.BindDefaultSupportAgent(ctx, workspaceID); err != nil {
		return demoStepResult{}, fmt.Errorf("bind support agent: %w", err)
	}
	_, settings, err := s.deps.Support.GetInstallation(ctx, workspaceID)
	if err != nil {
		return demoStepResult{}, fmt.Errorf("read chat widget: %w", err)
	}
	name, assistant := demo.CompanyName, demo.CompanyName+" Assistant"
	req := model.UpdateInstallationSettingsRequest{
		WidgetName: &name, AIDisplayName: &assistant, WidgetHelpSpaceIDs: []string{derefDemoString(demo.SpaceID)},
	}
	if cfg, err := s.deps.Helpcenter.GetConfig(ctx, workspaceID); err != nil {
		return demoStepResult{}, fmt.Errorf("read demo help center: %w", err)
	} else if cfg != nil {
		if color := strings.TrimSpace(cfg.BrandColor); color != "" && color != "#000000" {
			req.BrandColor, req.ButtonColor = &color, &color
		}
		scheme := "light"
		if cfg.ThemeMode == "dark" {
			scheme = "dark"
		}
		req.ColorScheme = &scheme
		if mark := strings.TrimSpace(derefDemoString(cfg.FaviconURL)); mark != "" {
			req.LogoURL, req.WidgetAvatarURL = &mark, &mark
		}
	}
	report := readDemoReport(demo)
	withAI := req
	if settings != nil && strings.TrimSpace(derefDemoString(settings.AIAgentID)) != "" {
		enabled, threshold := true, 0.7
		withAI.AIEnabled, withAI.AIAgentID, withAI.AIConfidenceThreshold = &enabled, settings.AIAgentID, &threshold
		if _, _, err := s.deps.Support.UpdateInstallationSettings(ctx, workspaceID, withAI); err == nil {
			report.AIRepliesEnabled = true
			writeDemoReport(demo, report)
			return demoStepResult{next: demoStepCRM}, nil
		} else if ctx.Err() != nil {
			return demoStepResult{}, ctx.Err()
		} else {
			s.logger.WarnContext(ctx, "demo: AI replies not enabled", "demo_id", demo.ID, "error", err)
		}
	}
	// The widget is still worth having without AI replies; say they are off.
	if _, _, err := s.deps.Support.UpdateInstallationSettings(ctx, workspaceID, req); err != nil {
		return demoStepResult{}, fmt.Errorf("configure chat widget: %w", err)
	}
	report.AIRepliesEnabled = false
	writeDemoReport(demo, report)
	addDemoNote(demo, "AI replies could not be turned on for the chat widget. Turn them on in the demo workspace's chat settings.")
	return demoStepResult{next: demoStepCRM}, nil
}

// stepCRM records the finished demo on the company in the home workspace.
func (s *Service) stepCRM(ctx context.Context, demo *eerepository.DemoWorkspace) (demoStepResult, error) {
	s.setCompanyProperties(ctx, demo.CompanyID, demo.HomeWorkspaceID, map[string]any{
		PropertyStatus:       "ready",
		PropertyWorkspaceID:  derefDemoString(demo.WorkspaceID),
		PropertyWorkspaceURL: derefDemoString(demo.DemoURL),
	})
	subject, body := "Demo built", demoNoteBody(demo)
	if _, err := s.deps.Activities.Create(ctx, model.CreateCRMActivityRequest{
		WorkspaceID: demo.HomeWorkspaceID, ActivityType: model.CRMActivityNote, CompanyID: &demo.CompanyID,
		Subject: &subject, Body: &body, Metadata: map[string]interface{}{"demo_id": demo.ID},
	}); err != nil {
		// The properties above already carry the result; the note is extra.
		s.logger.WarnContext(ctx, "demo: add CRM note", "demo_id", demo.ID, "error", err)
	}
	return demoStepResult{next: demoStepDone}, nil
}

// demoNoteBody is the CRM note that summarizes a finished build.
func demoNoteBody(demo *eerepository.DemoWorkspace) string {
	report := readDemoReport(demo)
	var out strings.Builder
	if demoURL := derefDemoString(demo.DemoURL); demoURL != "" {
		fmt.Fprintf(&out, "Demo help center: %s\n", demoURL)
	}
	fmt.Fprintf(&out, "Copied from: %s\n", demo.HelpCenterURL)
	fmt.Fprintf(&out, "Articles imported: %d", report.ArticlesImported)
	if report.ArticlesFailed > 0 {
		fmt.Fprintf(&out, " (%d failed)", report.ArticlesFailed)
	}
	fmt.Fprintf(&out, "\nLinks rewritten: %d\nImages copied: %d\n", report.LinksRewritten, report.ImagesCopied)
	if report.AIRepliesEnabled {
		out.WriteString("Chat widget: on, with AI replies\n")
	} else {
		out.WriteString("Chat widget: on, AI replies off\n")
	}
	if len(report.Notes) > 0 {
		out.WriteString("\nNeeds a look:\n")
		for _, note := range report.Notes {
			fmt.Fprintf(&out, "- %s\n", note)
		}
	}
	return strings.TrimSpace(out.String())
}
