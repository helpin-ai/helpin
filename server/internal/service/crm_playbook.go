package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/helpin-ai/helpin/server/internal/authorization"
	"github.com/helpin-ai/helpin/server/internal/model"
)

var (
	// ErrCRMPlaybookInput indicates invalid or unsupported policy settings.
	ErrCRMPlaybookInput = errors.New("invalid playbook settings")
	// ErrCRMPlaybookForbidden rejects missing, inactive or unauthorized workspace actors.
	ErrCRMPlaybookForbidden = errors.New("playbook access denied")
	// ErrCRMPlaybookNotFound hides foreign and missing identities equally.
	ErrCRMPlaybookNotFound = errors.New("playbook not found")
)

type crmPlaybookStore interface {
	Connections(context.Context, string, string, int64, int) (*model.CRMPlaybookConnections, error)
	ConnectionSource(context.Context, string, string, model.CRMPlaybookConnectionSelection) (*model.CRMPlaybookConnectionSource, error)
	PublishConnection(context.Context, string, string, string, model.PublishCRMPlaybookConnectionRequest, string, func(model.CRMPlaybookConnectionSource) (model.CRMPlaybookConnectionSnapshot, string, error)) (*model.CRMPlaybookConnectionResult, error)
	Connection(context.Context, string, string, string) (*model.CRMPlaybookConnection, error)
	Create(context.Context, model.CRMPlaybook) (*model.CRMPlaybook, bool, error)
	Command(context.Context, string, string, string, model.CRMPlaybookCommandRequest, string, func(model.CRMPlaybookDefinition) (model.CRMPlaybookDefinition, error)) (*model.CRMPlaybookCommandResult, error)
	List(context.Context, string, model.CRMPlaybookListFilters) (*model.CRMPlaybookList, error)
	Get(context.Context, string, string) (*model.CRMPlaybookItem, error)
	History(context.Context, string, string, int64, int) (*model.CRMPlaybookHistory, error)
	Versions(context.Context, string, string, int64, int) (*model.CRMPlaybookVersions, error)
	Preview(context.Context, string, string, string, string, int64, int, int) (*model.CRMPlaybookPreview, error)
	Apply(context.Context, string, string, string, model.ApplyCRMPlaybookRequest, string) (*model.CRMSituationCommandResult, error)
	AssessMilestone(context.Context, string, string, string, string, model.CRMPlaybookMilestoneRequest, string) (*model.CRMSituationCommandResult, error)
}

// CRMPlaybookService owns business policy and manual participation. It has no execution dependency.
type CRMPlaybookService struct {
	aiProfiles *AIProfileService
	store      crmPlaybookStore
	authz      crmSituationAuthorizer
	situations *CRMSituationService
}

// NewCRMPlaybookService binds existing CRM authorization and canonical Signal queries.
func NewCRMPlaybookService(store crmPlaybookStore, authz crmSituationAuthorizer, situations *CRMSituationService) *CRMPlaybookService {
	return &CRMPlaybookService{store: store, authz: authz, situations: situations}
}

// Templates returns independent draft defaults without installing or activating them.
func (s *CRMPlaybookService) Templates(ctx context.Context, ws string) ([]model.CRMPlaybookDefinition, error) {
	if _, err := s.authorize(ctx, ws, authorization.PermCRMRead); err != nil {
		return nil, err
	}
	definitions := playbookTemplates()
	for i := range definitions {
		var err error
		definitions[i], err = normalizePlaybookDefinition(definitions[i], false)
		if err != nil {
			return nil, err
		}
	}
	return definitions, nil
}

// Create saves a draft with enrollment disabled; it never prepares or performs an action.
func (s *CRMPlaybookService) Create(ctx context.Context, ws string, req model.CreateCRMPlaybookRequest) (*model.CRMPlaybook, bool, error) {
	actor, err := s.authorize(ctx, ws, authorization.PermCRMAdmin)
	if err != nil {
		return nil, false, err
	}
	req.CreationKey = strings.TrimSpace(req.CreationKey)
	if !validPlaybookKey(req.CreationKey) {
		return nil, false, ErrCRMPlaybookInput
	}
	req.Definition, err = normalizePlaybookDefinition(req.Definition, false)
	if err != nil {
		return nil, false, err
	}
	fingerprint, err := playbookFingerprint(actor.WorkspaceMemberID, req)
	if err != nil {
		return nil, false, err
	}
	now := time.Now().UTC().Truncate(time.Microsecond)
	return s.store.Create(ctx, model.CRMPlaybook{ID: uuid.NewString(), WorkspaceID: ws, CreationKey: req.CreationKey,
		CreationFingerprint: fingerprint, Draft: req.Definition, Revision: 1, CreatedByMemberID: actor.WorkspaceMemberID,
		UpdatedByMemberID: actor.WorkspaceMemberID, CreatedAt: now, UpdatedAt: now})
}

// Command edits CRM policy without changing saved Flows, Agents or active Signal versions.
func (s *CRMPlaybookService) Command(ctx context.Context, ws, id string, req model.CRMPlaybookCommandRequest) (*model.CRMPlaybookCommandResult, error) {
	actor, err := s.authorize(ctx, ws, authorization.PermCRMAdmin)
	if err != nil {
		return nil, err
	}
	req.CommandKey, req.Reason = strings.TrimSpace(req.CommandKey), strings.TrimSpace(req.Reason)
	if !validSituationID(id) || !validPlaybookKey(req.CommandKey) || req.ExpectedRevision < 1 || len(req.Reason) > 2000 {
		return nil, ErrCRMPlaybookInput
	}
	switch req.Operation {
	case "update_draft":
		if req.Definition == nil || req.AcceptingCustomers != nil {
			return nil, ErrCRMPlaybookInput
		}
		definition, err := normalizePlaybookDefinition(*req.Definition, false)
		if err != nil {
			return nil, err
		}
		req.Definition = &definition
	case "publish":
		if req.Definition != nil || req.AcceptingCustomers != nil {
			return nil, ErrCRMPlaybookInput
		}
	case "set_enrollment":
		if req.Definition != nil || req.AcceptingCustomers == nil {
			return nil, ErrCRMPlaybookInput
		}
	default:
		return nil, ErrCRMPlaybookInput
	}
	fingerprint, err := playbookFingerprint(actor.WorkspaceMemberID, req)
	if err != nil {
		return nil, err
	}
	result, err := s.store.Command(ctx, ws, id, actor.WorkspaceMemberID, req, fingerprint,
		func(d model.CRMPlaybookDefinition) (model.CRMPlaybookDefinition, error) {
			return normalizePlaybookDefinition(d, true)
		})
	if err == nil && result == nil {
		return nil, ErrCRMPlaybookNotFound
	}
	return result, err
}

// List reads configuration and canonical participant counts without side effects.
func (s *CRMPlaybookService) List(ctx context.Context, ws string, f model.CRMPlaybookListFilters) (*model.CRMPlaybookList, error) {
	if _, err := s.authorize(ctx, ws, authorization.PermCRMRead); err != nil {
		return nil, err
	}
	var err error
	f.Page, f.PageSize, err = playbookPage(f.Page, f.PageSize)
	if err != nil {
		return nil, err
	}
	f.Search = strings.TrimSpace(f.Search)
	if len(f.Search) > 500 || (f.State != "" && f.State != "all" && f.State != "draft" && f.State != "accepting" && f.State != "stopped") {
		return nil, ErrCRMPlaybookInput
	}
	return s.store.List(ctx, ws, f)
}

// Get reads a tenant-local definition; viewers cannot mutate drafts or admission policy.
func (s *CRMPlaybookService) Get(ctx context.Context, ws, id string) (*model.CRMPlaybookItem, error) {
	if _, err := s.authorize(ctx, ws, authorization.PermCRMRead); err != nil {
		return nil, err
	}
	if !validSituationID(id) {
		return nil, ErrCRMPlaybookInput
	}
	item, err := s.store.Get(ctx, ws, id)
	if err == nil && item == nil {
		return nil, ErrCRMPlaybookNotFound
	}
	return item, err
}

// History reads append-only publication, draft and admission changes.
func (s *CRMPlaybookService) History(ctx context.Context, ws, id string, before int64, limit int) (*model.CRMPlaybookHistory, error) {
	if _, err := s.Get(ctx, ws, id); err != nil {
		return nil, err
	}
	if before < 0 {
		return nil, ErrCRMPlaybookInput
	}
	_, limit, err := playbookPage(1, limit)
	if err != nil {
		return nil, err
	}
	return s.store.History(ctx, ws, id, before, limit)
}

// Versions reads published snapshots, including policies pinned by ongoing customer work.
func (s *CRMPlaybookService) Versions(ctx context.Context, ws, id string, before int64, limit int) (*model.CRMPlaybookVersions, error) {
	if _, err := s.Get(ctx, ws, id); err != nil {
		return nil, err
	}
	if before < 0 {
		return nil, ErrCRMPlaybookInput
	}
	_, limit, err := playbookPage(1, limit)
	if err != nil {
		return nil, err
	}
	return s.store.Versions(ctx, ws, id, before, limit)
}

func (s *CRMPlaybookService) authorize(ctx context.Context, ws string, permission authorization.Permission) (*authorization.Actor, error) {
	actor := authorization.GetActor(ctx)
	if actor == nil || !validSituationID(ws) || actor.WorkspaceID != ws || !validSituationID(actor.WorkspaceMemberID) ||
		actor.Status != model.WorkspaceMemberStatusActive || s.authz == nil || !s.authz.Can(actor, permission) {
		return nil, ErrCRMPlaybookForbidden
	}
	return actor, nil
}

func validPlaybookKey(key string) bool {
	return key != "" && len(key) <= 200 && !strings.HasPrefix(key, "system:")
}

func playbookFingerprint(actor string, input any) (string, error) {
	encoded, err := json.Marshal(struct {
		Actor   string
		Request any
	}{Actor: actor, Request: input})
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(encoded)
	return hex.EncodeToString(digest[:]), nil
}

func playbookPage(page, size int) (int, int, error) {
	if page == 0 {
		page = 1
	}
	if size == 0 {
		size = 25
	}
	if page < 1 || page > 1000000 || size < 1 || size > 100 {
		return page, size, ErrCRMPlaybookInput
	}
	return page, size, nil
}
