package service

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"go.temporal.io/api/serviceerror"
	tclient "go.temporal.io/sdk/client"

	appcrypto "github.com/helpin-ai/helpin/server/internal/crypto"
	"github.com/helpin-ai/helpin/server/internal/intercom"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/publichelp"
	"github.com/helpin-ai/helpin/server/internal/publichelp/zendesk"
	"github.com/helpin-ai/helpin/server/internal/temporalapp"
)

// docsImportSourceIntercomPublic marks jobs, redirects, and article
// checkpoints of an import that read a public Intercom help center. It is
// kept apart from the token-based Intercom import because the two do not
// share article identifiers. The other platforms' sources are named by their
// readers; see publicImportSourceSuffix.
const docsImportSourceIntercomPublic = "intercom_public"

// ErrPublicImportInvalid wraps a problem with a public help center import
// that the person starting it can fix. Its message is safe to show.
var ErrPublicImportInvalid = errors.New("invalid help center import")

// ErrPublicImportNeedsToken means the help center runs on Zendesk and
// refuses anonymous reads, so the import needs a Zendesk API token.
var ErrPublicImportNeedsToken = fmt.Errorf("%w: this Zendesk help center cannot be read without signing in. "+
	"Enter the email of a Zendesk admin and an API token to import it", ErrPublicImportInvalid)

func (s *DocsImportService) publicIntercomClient() *intercom.PublicClient {
	if s.publicIntercom == nil {
		s.publicIntercom = intercom.NewPublicClient()
	}
	return s.publicIntercom
}

// publicImportError turns a failed read of a public help center into a
// message that says what to do next. Anything unexpected is returned as is
// for the caller to log and report generically.
func publicImportError(err error) error {
	var known *publichelp.PlatformError
	switch {
	case err == nil:
		return nil
	case errors.Is(err, ErrPublicImportInvalid):
		return err
	case errors.As(err, &known) && known.Platform == zendesk.Key &&
		(errors.Is(err, publichelp.ErrRestricted) || errors.Is(err, publichelp.ErrChallenged)):
		return ErrPublicImportNeedsToken
	case errors.Is(err, intercom.ErrInvalidPublicURL), errors.Is(err, publichelp.ErrInvalidURL):
		return fmt.Errorf("%w: enter the address of a help center, such as https://help.example.com", ErrPublicImportInvalid)
	case errors.Is(err, intercom.ErrNotPublicHelpCenter), errors.Is(err, publichelp.ErrNotHelpCenter):
		return fmt.Errorf("%w: no help center that can be imported was found at this address. "+
			"Help centers on Intercom, Zendesk, Crisp, Freshdesk, Help Scout and HubSpot, and docs sites built with tools such as Mintlify, GitBook, Ferndesk and ReadMe, can be imported by address", ErrPublicImportInvalid)
	case errors.Is(err, publichelp.ErrChallenged):
		return fmt.Errorf("%w: this help center blocks automated reading, so it cannot be imported by address. "+
			"If it runs on Zendesk, enter its Zendesk address instead, such as example.zendesk.com", ErrPublicImportInvalid)
	case errors.Is(err, intercom.ErrPublicHelpCenterRestricted), errors.Is(err, publichelp.ErrRestricted):
		return fmt.Errorf("%w: this help center asks visitors to sign in, so it cannot be read by address", ErrPublicImportInvalid)
	case errors.Is(err, intercom.ErrNotFound), errors.Is(err, publichelp.ErrNotFound):
		return fmt.Errorf("%w: nothing was found at this address. Check it and try again", ErrPublicImportInvalid)
	case errors.Is(err, intercom.ErrPublicHelpCenterUnreachable), errors.Is(err, publichelp.ErrUnreachable):
		return fmt.Errorf("%w: the help center did not respond. Check the address and try again in a moment", ErrPublicImportInvalid)
	}
	return err
}

// requestCredentials are the Zendesk credentials of an import request.
func requestCredentials(req model.PublicHelpCenterImportRequest) publicCredentials {
	return publicCredentials{email: strings.TrimSpace(req.Email), token: strings.TrimSpace(req.APIToken)}
}

// CheckPublicHelpCenter confirms that an address is a public help center this
// service can import, with a request or two, and returns its home address,
// name and platform. Callers use it to refuse a bad address before starting
// work.
func (s *DocsImportService) CheckPublicHelpCenter(ctx context.Context, rawURL string) (*model.PublicHelpCenterSite, error) {
	_, site, err := s.detectPublicHelpCenter(ctx, rawURL, publicCredentials{})
	if err != nil {
		return nil, publicImportError(err)
	}
	return &model.PublicHelpCenterSite{URL: site.BaseURL, Name: site.Name, Platform: site.Platform}, nil
}

// loadPublicHelpCenter detects the help center at rawURL and reads its
// collections and article lists.
func (s *DocsImportService) loadPublicHelpCenter(ctx context.Context, rawURL string, credentials publicCredentials) (publicSource, *publichelp.Site, error) {
	source, found, err := s.detectPublicHelpCenter(ctx, rawURL, credentials)
	if err != nil {
		return nil, nil, err
	}
	site, err := source.load(ctx, found.BaseURL, nil)
	if err != nil {
		return nil, nil, err
	}
	// The home page names the help center; a listing read through an API may not.
	site.Name = cmp.Or(site.Name, found.Name)
	return source, site, nil
}

// PreviewPublicHelpCenter reads a public help center and counts what an
// import would bring in. Nothing is written.
func (s *DocsImportService) PreviewPublicHelpCenter(ctx context.Context, req model.PublicHelpCenterImportRequest) (*model.PublicHelpCenterImportPreview, error) {
	source, site, err := s.loadPublicHelpCenter(ctx, req.URL, requestCredentials(req))
	if err != nil {
		return nil, publicImportError(err)
	}
	return publicHelpCenterPreview(source, site)
}

// PublicHelpCenterArticles reads a public help center once and returns its
// counts together with every public article, each with its summary and
// top-level collection, in the order the help center shows them. Nothing is
// written.
func (s *DocsImportService) PublicHelpCenterArticles(ctx context.Context, rawURL string) (*model.PublicHelpCenterImportPreview, []model.PublicHelpCenterArticleRef, error) {
	source, site, err := s.loadPublicHelpCenter(ctx, rawURL, publicCredentials{})
	if err != nil {
		return nil, nil, publicImportError(err)
	}
	preview, err := publicHelpCenterPreview(source, site)
	if err != nil {
		return nil, nil, err
	}
	var refs []model.PublicHelpCenterArticleRef
	for _, top := range site.Collections {
		refs = appendPublicArticleRefs(refs, top, top.Name)
	}
	return preview, refs, nil
}

func publicHelpCenterPreview(source publicSource, site *publichelp.Site) (*model.PublicHelpCenterImportPreview, error) {
	flat := flattenPublicCollections(site.Collections)
	preview := &model.PublicHelpCenterImportPreview{
		Platform:       site.Platform,
		Source:         source.source(),
		URL:            site.BaseURL,
		Name:           site.Name,
		Locale:         site.Locale,
		TopCollections: make([]model.PublicHelpCenterCollectionCount, 0, len(site.Collections)),
	}
	for _, entry := range flat {
		preview.Articles += len(entry.source.Articles)
		if entry.merged() {
			preview.CollectionsMerged++
		} else {
			preview.Collections++
		}
	}
	for _, top := range site.Collections {
		preview.TopCollections = append(preview.TopCollections, model.PublicHelpCenterCollectionCount{
			Name: top.Name, Articles: countPublicArticles(top),
		})
	}
	if preview.Articles == 0 {
		return nil, fmt.Errorf("%w: this help center has no public articles to import", ErrPublicImportInvalid)
	}
	return preview, nil
}

// ReadPublicArticle returns one public help-center article as plain text.
// source is the one its listing named.
func (s *DocsImportService) ReadPublicArticle(ctx context.Context, source, articleURL string) (title, text string, err error) {
	from := s.publicSourceFor(source, publicCredentials{})
	if from == nil {
		return "", "", fmt.Errorf("%w: unknown help center platform", ErrPublicImportInvalid)
	}
	article, err := from.read(ctx, articleURL)
	if err != nil {
		return "", "", publicImportError(err)
	}
	return article.Title, article.text(), nil
}

// appendPublicArticleRefs adds a collection's articles, then its nested
// collections' articles, all named under the top-level collection.
func appendPublicArticleRefs(refs []model.PublicHelpCenterArticleRef, collection publichelp.Collection, top string) []model.PublicHelpCenterArticleRef {
	for _, article := range collection.Articles {
		refs = append(refs, model.PublicHelpCenterArticleRef{Title: article.Title, URL: article.URL, Summary: article.Summary, Collection: top})
	}
	for _, child := range collection.Children {
		refs = appendPublicArticleRefs(refs, child, top)
	}
	return refs
}

// writePublicBlocksText writes the readable text of article blocks, one block
// per line, including list items, table cells and collapsible summaries.
func writePublicBlocksText(b *strings.Builder, blocks []intercom.Block) {
	for _, block := range blocks {
		if text := strings.TrimSpace(htmlToPlainText(cmp.Or(block.Text, block.Summary.Text))); text != "" {
			b.WriteString(text)
			b.WriteString("\n")
		}
		writePublicBlocksText(b, block.Content)
		for _, item := range block.Items {
			writePublicBlocksText(b, item.Content)
		}
		for _, row := range block.Rows {
			for _, cell := range row.Cells {
				writePublicBlocksText(b, cell.Content)
			}
		}
	}
}

func countPublicArticles(collection publichelp.Collection) int {
	total := len(collection.Articles)
	for _, child := range collection.Children {
		total += countPublicArticles(child)
	}
	return total
}

// StartPublicHelpCenter creates an import job for a public help center and
// starts the durable import workflow.
func (s *DocsImportService) StartPublicHelpCenter(ctx context.Context, req model.PublicHelpCenterImportRequest, workspaceID, userID string) (string, error) {
	newSpaceName := strings.TrimSpace(derefString(req.NewSpaceName))
	targetSpaceID := strings.TrimSpace(derefString(req.TargetSpaceID))
	if newSpaceName == "" && targetSpaceID == "" {
		return "", fmt.Errorf("%w: choose a space or name a new one", ErrPublicImportInvalid)
	}
	// A request or two confirm the address before anything is created.
	credentials := requestCredentials(req)
	source, site, err := s.detectPublicHelpCenter(ctx, req.URL, credentials)
	if err != nil {
		return "", publicImportError(err)
	}
	homeURL := site.BaseURL
	if newSpaceName == "" {
		if err := s.checkIntercomTargetSpace(ctx, workspaceID, targetSpaceID); err != nil {
			if errors.Is(err, ErrIntercomImportInvalid) {
				return "", fmt.Errorf("%w: %s", ErrPublicImportInvalid, strings.TrimPrefix(err.Error(), ErrIntercomImportInvalid.Error()+": "))
			}
			return "", err
		}
	}
	if s.temporalClient == nil {
		return "", fmt.Errorf("docs import worker is not configured")
	}
	if len(s.encryptionKey) != 32 {
		return "", fmt.Errorf("docs import encryption key is not configured")
	}
	spaceID := targetSpaceID
	if newSpaceName != "" {
		space, err := s.spaceSvc.Create(ctx, workspaceID, model.CreateDocsSpaceRequest{
			Name: newSpaceName, Visibility: model.SpaceVisibilityWorkspaceWide, Type: model.SpaceTypeExternalCapable,
		}, userID)
		if err != nil {
			return "", fmt.Errorf("create space: %w", err)
		}
		spaceID = space.ID
	}
	stored := model.DocsImportStartRequest{
		SourceURL: homeURL, TargetSpaceID: &spaceID, ImportStatus: req.ImportStatus, Email: credentials.email, APIKey: credentials.token,
	}
	payload, err := json.Marshal(stored)
	if err != nil {
		return "", fmt.Errorf("marshal import payload: %w", err)
	}
	encrypted, err := appcrypto.EncryptString(string(payload), s.encryptionKey)
	if err != nil {
		return "", fmt.Errorf("encrypt import payload: %w", err)
	}
	config, err := json.Marshal(docsImportJobConfig{ImportStatus: normalizeDocsImportStatus(req.ImportStatus), SourceURL: homeURL})
	if err != nil {
		return "", fmt.Errorf("marshal import config: %w", err)
	}
	now := time.Now()
	job := &model.DocsImportJob{
		ID: uuid.NewString(), WorkspaceID: workspaceID, SpaceID: &spaceID, Source: source.source(),
		Status: model.DocsImportStatusPending, Failures: json.RawMessage("[]"), Config: config,
		StartedBy: userID, StartedAt: &now, PayloadEncrypted: &encrypted,
	}
	workflowID := temporalapp.WorkflowIDForDocsImport(job.ID)
	job.WorkflowID = &workflowID
	if err := s.importRepo.Create(ctx, job); err != nil {
		return "", fmt.Errorf("create import job: %w", err)
	}
	if _, err := s.temporalClient.ExecuteWorkflow(ctx, tclient.StartWorkflowOptions{ID: workflowID, TaskQueue: temporalapp.QueueAutomation},
		temporalapp.DocsImportWorkflow, temporalapp.DocsImportWorkflowInput{ImportID: job.ID}); err != nil {
		var alreadyStarted *serviceerror.WorkflowExecutionAlreadyStarted
		if !errors.As(err, &alreadyStarted) {
			s.failJob(ctx, job.ID, fmt.Sprintf("start docs import workflow: %v", err))
			return "", fmt.Errorf("start docs import workflow: %w", err)
		}
	}
	s.logger.InfoContext(ctx, "public help center import started",
		"job_id", job.ID, "workspace_id", workspaceID, "space_id", spaceID, "platform", site.Platform, "source_host", hostOfURL(homeURL))
	return job.ID, nil
}

// publicImportCollection is one source collection in import order.
type publicImportCollection struct {
	source publichelp.Collection
	// parentID is the source ID of the collection above, or "" at the top.
	parentID string
	depth    int
	// targetID is the source ID of the collection whose Helpin collection
	// receives this one's articles: its own, unless it is nested deeper than
	// Helpin allows.
	targetID string
}

// merged reports whether the collection is folded into one above it.
func (c publicImportCollection) merged() bool { return c.targetID != c.source.ID }

// flattenPublicCollections lists a collection tree parents-first in source
// order and folds collections below Helpin's depth limit into the nearest
// collection that fits.
func flattenPublicCollections(tree []publichelp.Collection) []publicImportCollection {
	var out []publicImportCollection
	var walk func(nodes []publichelp.Collection, parentID, parentTarget string, depth int)
	walk = func(nodes []publichelp.Collection, parentID, parentTarget string, depth int) {
		for _, node := range nodes {
			target := node.ID
			if depth > maxCollectionDepth {
				target = parentTarget
			}
			out = append(out, publicImportCollection{source: node, parentID: parentID, depth: depth, targetID: target})
			walk(node.Children, node.ID, target, depth+1)
		}
	}
	walk(tree, "", "", 0)
	return out
}
