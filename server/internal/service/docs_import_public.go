package service

import (
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
	"github.com/helpin-ai/helpin/server/internal/temporalapp"
)

// docsImportSourceIntercomPublic marks jobs, redirects, and article
// checkpoints of an import that read a public Intercom help center. It is
// kept apart from the token-based Intercom import because the two do not
// share article identifiers.
const docsImportSourceIntercomPublic = "intercom_public"

// ErrPublicImportInvalid wraps a problem with a public help center import
// that the person starting it can fix. Its message is safe to show.
var ErrPublicImportInvalid = errors.New("invalid help center import")

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
	switch {
	case err == nil:
		return nil
	case errors.Is(err, ErrPublicImportInvalid):
		return err
	case errors.Is(err, intercom.ErrInvalidPublicURL):
		return fmt.Errorf("%w: enter the address of a help center, such as https://help.example.com", ErrPublicImportInvalid)
	case errors.Is(err, intercom.ErrNotPublicHelpCenter):
		return fmt.Errorf("%w: this address is not an Intercom help center. Only public Intercom help centers can be imported by address", ErrPublicImportInvalid)
	case errors.Is(err, intercom.ErrPublicHelpCenterRestricted):
		return fmt.Errorf("%w: this help center asks visitors to sign in, so it cannot be read by address. Use an Intercom access token instead", ErrPublicImportInvalid)
	case errors.Is(err, intercom.ErrNotFound):
		return fmt.Errorf("%w: nothing was found at this address. Check it and try again", ErrPublicImportInvalid)
	case errors.Is(err, intercom.ErrPublicHelpCenterUnreachable):
		return fmt.Errorf("%w: the help center did not respond. Check the address and try again in a moment", ErrPublicImportInvalid)
	}
	return err
}

// CheckPublicHelpCenter confirms that an address is a public help center this
// service can import, with a single request, and returns its home address
// and name. Callers use it to refuse a bad address before starting work.
func (s *DocsImportService) CheckPublicHelpCenter(ctx context.Context, rawURL string) (homeURL, name string, err error) {
	site, err := s.publicIntercomClient().LoadTheme(ctx, rawURL)
	if err != nil {
		return "", "", publicImportError(err)
	}
	return site.BaseURL, site.Name, nil
}

// PreviewPublicHelpCenter reads a public help center and counts what an
// import would bring in. Nothing is written.
func (s *DocsImportService) PreviewPublicHelpCenter(ctx context.Context, req model.PublicHelpCenterImportRequest) (*model.PublicHelpCenterImportPreview, error) {
	site, err := s.publicIntercomClient().LoadHelpCenter(ctx, req.URL, nil)
	if err != nil {
		return nil, publicImportError(err)
	}
	flat := flattenPublicCollections(site.Collections)
	preview := &model.PublicHelpCenterImportPreview{
		Platform:       model.PublicHelpCenterPlatformIntercom,
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

func countPublicArticles(collection intercom.PublicCollection) int {
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
	// One request confirms the address before anything is created.
	homeURL, _, err := s.CheckPublicHelpCenter(ctx, req.URL)
	if err != nil {
		return "", err
	}
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
	stored := model.DocsImportStartRequest{SourceURL: homeURL, TargetSpaceID: &spaceID, ImportStatus: req.ImportStatus}
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
		ID: uuid.NewString(), WorkspaceID: workspaceID, SpaceID: &spaceID, Source: docsImportSourceIntercomPublic,
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
		"job_id", job.ID, "workspace_id", workspaceID, "space_id", spaceID, "source_host", hostOfURL(homeURL))
	return job.ID, nil
}

// publicImportCollection is one source collection in import order.
type publicImportCollection struct {
	source intercom.PublicCollection
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
func flattenPublicCollections(tree []intercom.PublicCollection) []publicImportCollection {
	var out []publicImportCollection
	var walk func(nodes []intercom.PublicCollection, parentID, parentTarget string, depth int)
	walk = func(nodes []intercom.PublicCollection, parentID, parentTarget string, depth int) {
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
