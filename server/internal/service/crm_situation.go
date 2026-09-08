package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/google/uuid"

	"github.com/helpin-ai/helpin/server/internal/authorization"
	"github.com/helpin-ai/helpin/server/internal/model"
)

var (
	// ErrCRMSituationForbidden denies missing, inactive or mismatched workspace access.
	ErrCRMSituationForbidden = errors.New("customer work access denied")
	// ErrCRMSituationNotFound does not distinguish missing and foreign records.
	ErrCRMSituationNotFound = errors.New("customer situation not found")
	// ErrCRMSituationInput represents invalid customer-work input without internal details.
	ErrCRMSituationInput = errors.New("invalid customer work input")
)

type crmSituationStore interface {
	Create(context.Context, model.CRMSituation, []model.CRMSituationReference) (*model.CRMSituation, bool, error)
	ResolveOwner(context.Context, string, model.CreateCRMSituationRequest) (*string, error)
	GetByID(context.Context, string, string) (*model.CRMSituationItem, error)
	List(context.Context, string, string, model.CRMSituationListFilters) (*model.CRMSituationList, error)
	ApplyCommand(context.Context, string, string, string, model.CRMSituationCommandRequest, string,
		func(model.CRMSituation) (model.CRMSituationWorkState, error)) (*model.CRMSituationCommandResult, error)
	History(context.Context, string, string, int64, int) (*model.CRMSituationHistory, error)
	ListInbox(context.Context, string, string, model.CRMSignalInboxFilters) (*model.CRMSignalInboxList, error)
	InboxRecommendation(context.Context, string, string) (*model.CRMInboxRecommendation, error)
}

type crmSituationAuthorizer interface {
	Can(*authorization.Actor, authorization.Permission) bool
}

// CRMSituationService owns validation and manual customer-work creation, not execution.
type CRMSituationService struct {
	store      crmSituationStore
	authz      crmSituationAuthorizer
	actions    *CRMSuggestionService
	automation interface {
		AfterSituationCommand(context.Context, string, string, string, model.CRMSituationCommandResult) error
	}
}

// SetActions connects existing CRM decisions; it does not introduce an executor.
func (s *CRMSituationService) SetActions(actions *CRMSuggestionService) { s.actions = actions }

// SetAutomationLifecycle connects cancellation after the canonical transaction has fenced authority.
func (s *CRMSituationService) SetAutomationLifecycle(automation *CRMPlaybookExecutionService) {
	s.automation = automation
}

// NewCRMSituationService binds explicit storage and the existing authorization boundary.
func NewCRMSituationService(store crmSituationStore, authz crmSituationAuthorizer) *CRMSituationService {
	return &CRMSituationService{store: store, authz: authz}
}

// Create persists manual work. Referencing a signal never activates its automation.
func (s *CRMSituationService) Create(
	ctx context.Context, workspaceID string, req model.CreateCRMSituationRequest,
) (*model.CRMSituation, bool, error) {
	actor, err := s.authorize(ctx, workspaceID, authorization.PermCRMEdit)
	if err != nil {
		return nil, false, err
	}
	req, err = normalizeSituationCreation(req)
	if err != nil {
		return nil, false, err
	}
	encoded, err := json.Marshal(req)
	if err != nil {
		return nil, false, fmt.Errorf("encode situation creation intent: %w", err)
	}
	fingerprint := sha256.Sum256(encoded)
	owner := req.OwnerMemberID
	if req.OwnerMode == "routing" {
		owner, err = s.store.ResolveOwner(ctx, workspaceID, req)
		if err != nil {
			return nil, false, fmt.Errorf("resolve situation owner: %w", err)
		}
	}
	nextOwner := req.NextActionOwnerMemberID
	if nextOwner == nil {
		nextOwner = owner
	}
	return s.store.Create(ctx, model.CRMSituation{
		ID: uuid.NewString(), WorkspaceID: workspaceID, CreationKey: req.CreationKey,
		CreationFingerprint: hex.EncodeToString(fingerprint[:]),
		Title:               req.Title, Objective: req.Objective, CommercialMotion: req.CommercialMotion,
		CompanyID: req.CompanyID, ContactID: req.ContactID, DealID: req.DealID,
		OwnerMemberID: owner, NextActionOwnerMemberID: nextOwner,
		Lifecycle: model.CRMSituationOpen, Attention: model.CRMSituationNeedsContext,
		NextStep: req.NextStep, Priority: req.Priority, CreatedByMemberID: &actor.WorkspaceMemberID,
	}, req.References)
}

// List returns authoritative server-filtered work, totals and category counts.
func (s *CRMSituationService) List(
	ctx context.Context, workspaceID string, filters model.CRMSituationListFilters,
) (*model.CRMSituationList, error) {
	actor, err := s.authorize(ctx, workspaceID, authorization.PermCRMRead)
	if err != nil {
		return nil, err
	}
	filters, err = normalizeSituationFilters(filters)
	if err != nil {
		return nil, err
	}
	return s.store.List(ctx, workspaceID, actor.WorkspaceMemberID, filters)
}

// GetByID reads customer work without marking evidence reviewed or approving actions.
func (s *CRMSituationService) GetByID(ctx context.Context, workspaceID, id string) (*model.CRMSituationItem, error) {
	if _, err := s.authorize(ctx, workspaceID, authorization.PermCRMRead); err != nil {
		return nil, err
	}
	if !validSituationID(id) {
		return nil, ErrCRMSituationInput
	}
	item, err := s.store.GetByID(ctx, workspaceID, id)
	if err != nil {
		return nil, err
	}
	if item == nil {
		return nil, ErrCRMSituationNotFound
	}
	return item, nil
}

func (s *CRMSituationService) authorize(
	ctx context.Context, workspaceID string, permission authorization.Permission,
) (*authorization.Actor, error) {
	actor := authorization.GetActor(ctx)
	if actor == nil || !validSituationID(workspaceID) || actor.WorkspaceID != workspaceID ||
		!validSituationID(actor.WorkspaceMemberID) || actor.Status != model.WorkspaceMemberStatusActive ||
		s.authz == nil || !s.authz.Can(actor, permission) {
		return nil, ErrCRMSituationForbidden
	}
	return actor, nil
}

func normalizeSituationCreation(req model.CreateCRMSituationRequest) (model.CreateCRMSituationRequest, error) {
	req.CreationKey = strings.TrimSpace(req.CreationKey)
	req.Title, req.Objective = strings.TrimSpace(req.Title), strings.TrimSpace(req.Objective)
	req.NextStep = strings.TrimSpace(req.NextStep)
	if req.OwnerMode == "" {
		req.OwnerMode = "routing"
	}
	if len(req.CreationKey) < 1 || len(req.CreationKey) > 200 || strings.HasPrefix(req.CreationKey, "source:") || len(req.Title) < 1 || len(req.Title) > 240 ||
		len(req.Objective) < 1 || len(req.Objective) > 4000 || len(req.NextStep) > 1000 ||
		req.Priority < 0 || req.Priority > 100 || math.IsNaN(req.Priority) || len(req.References) > 100 ||
		model.CRMSituationCategoryForMotion(req.CommercialMotion) == "" ||
		(req.CompanyID == nil && req.ContactID == nil && req.DealID == nil) {
		return req, ErrCRMSituationInput
	}
	for _, id := range []*string{req.CompanyID, req.ContactID, req.DealID, req.OwnerMemberID, req.NextActionOwnerMemberID} {
		if id != nil && !validSituationID(*id) {
			return req, ErrCRMSituationInput
		}
	}
	if (req.OwnerMode == "member" && req.OwnerMemberID == nil) ||
		(req.OwnerMode != "member" && req.OwnerMemberID != nil) ||
		(req.OwnerMode != "member" && req.OwnerMode != "routing" && req.OwnerMode != "unassigned") {
		return req, ErrCRMSituationInput
	}
	references := make([]model.CRMSituationReference, 0, len(req.References))
	seen := map[string]bool{}
	for _, ref := range req.References {
		if !validSituationID(ref.SourceID) ||
			(ref.Kind != model.CRMSituationReferenceSignal && ref.Kind != model.CRMSituationReferenceSuggestion) {
			return req, ErrCRMSituationInput
		}
		key := ref.Kind + ":" + ref.SourceID
		if !seen[key] {
			seen[key] = true
			references = append(references, model.CRMSituationReference{Kind: ref.Kind, SourceID: ref.SourceID})
		}
	}
	sort.Slice(references, func(i, j int) bool {
		return references[i].Kind+references[i].SourceID < references[j].Kind+references[j].SourceID
	})
	req.References = references
	return req, nil
}

func normalizeSituationFilters(filters model.CRMSituationListFilters) (model.CRMSituationListFilters, error) {
	if filters.Scope == "" {
		filters.Scope = "mine"
	}
	if filters.State == "" {
		filters.State = "needs_attention"
	}
	if filters.Category == "" {
		filters.Category = "all"
	}
	if filters.Page == 0 {
		filters.Page = 1
	}
	if filters.PageSize == 0 {
		filters.PageSize = 25
	}
	filters.Search = strings.TrimSpace(filters.Search)
	validScope := filters.Scope == "mine" || filters.Scope == "my_teams" || filters.Scope == "unassigned" || filters.Scope == "all"
	validState := filters.State == "needs_attention" || filters.State == "open" || filters.State == "waiting" ||
		filters.State == "paused" || filters.State == "closed" || filters.State == "all"
	validCategory := filters.Category == "all"
	for _, category := range model.CRMSituationCategories() {
		validCategory = validCategory || filters.Category == category.Key
	}
	if (filters.PlaybookID != "" && !validSituationID(filters.PlaybookID)) || !validScope || !validState || !validCategory || filters.Page < 1 || filters.Page > 1000000 ||
		filters.PageSize < 1 || filters.PageSize > 100 || len(filters.Search) > 500 {
		return filters, ErrCRMSituationInput
	}
	if filters.Query != nil && (len(filters.Query.Rules) > 30 ||
		(filters.Query.Logic != "" && filters.Query.Logic != model.QueryFilterLogicAnd && filters.Query.Logic != model.QueryFilterLogicOr)) {
		return filters, ErrCRMSituationInput
	}
	return filters, nil
}

func validSituationID(id string) bool {
	parsed, err := uuid.Parse(id)
	return err == nil && parsed != uuid.Nil && parsed.String() == id
}
