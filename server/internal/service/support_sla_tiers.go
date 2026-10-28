package service

import (
	"context"
	"errors"
	"slices"
	"strings"

	"github.com/google/uuid"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

// ErrSupportSLACompanyNotFound means the company is not in the workspace.
var ErrSupportSLACompanyNotFound = errors.New("company not found")

// ErrSupportSLACustomExists means the company already has a custom SLA.
var ErrSupportSLACustomExists = errors.New("this company already has a custom SLA; edit its policies instead")

// validSupportSLAID reports whether id can name a row; uuid columns reject
// anything else with a database error.
func validSupportSLAID(id string) bool {
	_, err := uuid.Parse(id)
	return err == nil
}

// normalizePolicyCompanies validates the company tier and company
// conditions of a policy request.
func (s *SupportSLAService) normalizePolicyCompanies(ctx context.Context, workspaceID string, req model.SupportSLAPolicyRequest, policy *model.SupportSLAPolicy) error {
	tiers := map[string]bool{model.SupportSLANoTier: true}
	if len(req.CompanyTierIDs) > 0 {
		existing, err := s.repo.ListTiers(ctx, workspaceID)
		if err != nil {
			return err
		}
		for _, tier := range existing {
			tiers[tier.ID] = true
		}
	}
	var err error
	if policy.CompanyTierIDs, err = normalizeSLAList(req.CompanyTierIDs, func(v string) bool { return tiers[v] }, "SLA tier"); err != nil {
		return err
	}
	companyIDs, err := normalizeSLAList(req.CompanyIDs, validSupportSLAID, "company")
	if err != nil {
		return err
	}
	if len(companyIDs) > model.SupportSLAPolicyCompanyLimit {
		return invalidSLAPolicy("a policy can name up to %d companies", model.SupportSLAPolicyCompanyLimit)
	}
	existing, err := s.repo.ExistingCompanyIDs(ctx, workspaceID, companyIDs)
	if err != nil {
		return err
	}
	for _, id := range companyIDs {
		if !slices.Contains(existing, id) {
			return invalidSLAPolicy("unknown company %q", id)
		}
	}
	policy.CompanyIDs = companyIDs
	return nil
}

// attachPolicyCompanies fills in the names of the companies policies name.
func (s *SupportSLAService) attachPolicyCompanies(ctx context.Context, workspaceID string, policies ...*model.SupportSLAPolicy) error {
	var ids []string
	for _, policy := range policies {
		ids = append(ids, policy.CompanyIDs...)
	}
	names, err := s.repo.CompanyNames(ctx, workspaceID, ids)
	if err != nil {
		return err
	}
	for _, policy := range policies {
		policy.Companies = make([]model.SupportSLAPolicyCompany, 0, len(policy.CompanyIDs))
		for _, id := range policy.CompanyIDs {
			// A deleted company keeps its id in the condition until edited.
			name := names[id]
			if name == "" {
				name = "Deleted company"
			}
			policy.Companies = append(policy.Companies, model.SupportSLAPolicyCompany{ID: id, Name: name})
		}
	}
	return nil
}

// ListTiers returns the workspace's tiers with where each is used.
func (s *SupportSLAService) ListTiers(ctx context.Context, workspaceID string) ([]model.SupportSLATier, error) {
	tiers, err := s.repo.ListTiers(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	if tiers == nil {
		return []model.SupportSLATier{}, nil
	}
	policies, err := s.repo.ListPolicies(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	for i := range tiers {
		for _, policy := range policies {
			if slices.Contains([]string(policy.CompanyTierIDs), tiers[i].ID) {
				tiers[i].PolicyCount++
			}
		}
	}
	return tiers, nil
}

func (s *SupportSLAService) normalizeTier(ctx context.Context, workspaceID, tierID string, req model.SupportSLATierRequest) (name, description string, err error) {
	name = strings.Join(strings.Fields(req.Name), " ")
	if name == "" || len([]rune(name)) > 60 {
		return "", "", invalidSLAPolicy("tier name must be 1 to 60 characters")
	}
	if strings.EqualFold(name, model.SupportSLANoTier) {
		return "", "", invalidSLAPolicy("%q is reserved for companies without a tier", name)
	}
	description = strings.TrimSpace(req.Description)
	if len([]rune(description)) > 300 {
		return "", "", invalidSLAPolicy("tier description must be at most 300 characters")
	}
	tiers, err := s.repo.ListTiers(ctx, workspaceID)
	if err != nil {
		return "", "", err
	}
	for _, tier := range tiers {
		if tier.ID != tierID && strings.EqualFold(tier.Name, name) {
			return "", "", invalidSLAPolicy("a tier named %q already exists", tier.Name)
		}
	}
	if tierID == "" && len(tiers) >= model.SupportSLATierLimit {
		return "", "", invalidSLAPolicy("a workspace can have up to %d SLA tiers", model.SupportSLATierLimit)
	}
	return name, description, nil
}

// CreateTier adds a tier after the existing ones.
func (s *SupportSLAService) CreateTier(ctx context.Context, workspaceID string, req model.SupportSLATierRequest) (*model.SupportSLATier, error) {
	if err := s.requireFeature(ctx, workspaceID); err != nil {
		return nil, err
	}
	name, description, err := s.normalizeTier(ctx, workspaceID, "", req)
	if err != nil {
		return nil, err
	}
	tier := &model.SupportSLATier{ID: uuid.NewString(), WorkspaceID: workspaceID, Name: name, Description: description}
	if err := s.repo.CreateTier(ctx, tier); err != nil {
		return nil, err
	}
	return tier, nil
}

// UpdateTier renames a tier. Companies and policies keep using it.
func (s *SupportSLAService) UpdateTier(ctx context.Context, workspaceID, tierID string, req model.SupportSLATierRequest) (*model.SupportSLATier, error) {
	if err := s.requireFeature(ctx, workspaceID); err != nil {
		return nil, err
	}
	if !validSupportSLAID(tierID) {
		return nil, repository.ErrSupportSLATierNotFound
	}
	tier, err := s.repo.GetTier(ctx, workspaceID, tierID)
	if err != nil {
		return nil, err
	}
	if tier.Name, tier.Description, err = s.normalizeTier(ctx, workspaceID, tierID, req); err != nil {
		return nil, err
	}
	if err := s.repo.UpdateTier(ctx, tier); err != nil {
		return nil, err
	}
	return tier, nil
}

// DeleteTier removes a tier. Its companies have no tier afterwards and
// policies stop matching it; their open conversations are re-checked and a
// newly selected policy counts from now.
func (s *SupportSLAService) DeleteTier(ctx context.Context, workspaceID, tierID string) error {
	if !validSupportSLAID(tierID) {
		return repository.ErrSupportSLATierNotFound
	}
	now := s.now()
	companyIDs, err := s.repo.DeleteTier(ctx, workspaceID, tierID, now)
	if err != nil {
		return err
	}
	if err := s.repo.EnqueueCompanyConversations(ctx, workspaceID, companyIDs, now); err != nil {
		return err
	}
	return s.repo.EnqueueOpenRecords(ctx, workspaceID, now)
}

// ReorderTiers sets the display order of tiers.
func (s *SupportSLAService) ReorderTiers(ctx context.Context, workspaceID string, tierIDs []string) ([]model.SupportSLATier, error) {
	if err := s.repo.ReorderTiers(ctx, workspaceID, tierIDs); err != nil {
		return nil, err
	}
	return s.ListTiers(ctx, workspaceID)
}

// SetCompanyTier gives companies a tier, or clears it, and re-checks their
// open conversations. It returns how many companies changed.
func (s *SupportSLAService) SetCompanyTier(ctx context.Context, workspaceID string, req model.SetSupportSLATierRequest) (int, error) {
	if err := s.requireFeature(ctx, workspaceID); err != nil {
		return 0, err
	}
	companyIDs, err := normalizeSLAList(req.CompanyIDs, func(string) bool { return true }, "company")
	if err != nil {
		return 0, err
	}
	if len(companyIDs) == 0 || len(companyIDs) > model.SupportSLASetTierLimit {
		return 0, invalidSLAPolicy("choose 1 to %d companies", model.SupportSLASetTierLimit)
	}
	if slices.ContainsFunc(companyIDs, func(id string) bool { return !validSupportSLAID(id) }) {
		return 0, ErrSupportSLACompanyNotFound
	}
	existing, err := s.repo.ExistingCompanyIDs(ctx, workspaceID, companyIDs)
	if err != nil {
		return 0, err
	}
	if len(existing) != len(companyIDs) {
		return 0, ErrSupportSLACompanyNotFound
	}
	var tierID *string
	if req.TierID != nil && strings.TrimSpace(*req.TierID) != "" {
		if !validSupportSLAID(strings.TrimSpace(*req.TierID)) {
			return 0, repository.ErrSupportSLATierNotFound
		}
		tier, err := s.repo.GetTier(ctx, workspaceID, strings.TrimSpace(*req.TierID))
		if err != nil {
			return 0, err
		}
		tierID = &tier.ID
	}
	now := s.now()
	changed, err := s.repo.SetCompanyTier(ctx, workspaceID, companyIDs, tierID, now)
	if err != nil {
		return 0, err
	}
	return len(changed), s.repo.EnqueueCompanyConversations(ctx, workspaceID, changed, now)
}

// supportSLACustomFor reports whether policy is a custom SLA of companyID:
// a policy that names only that company and no tier.
func supportSLACustomFor(policy model.SupportSLAPolicy, companyID string) bool {
	return len(policy.CompanyIDs) == 1 && policy.CompanyIDs[0] == companyID && len(policy.CompanyTierIDs) == 0
}

// supportSLAMayApplyToCompany reports whether policy can apply to the
// company's conversations, whatever their priority, inbox or channel.
func supportSLAMayApplyToCompany(policy model.SupportSLAPolicy, company *repository.SupportSLACompany) bool {
	if len(policy.CompanyIDs) > 0 && !slices.Contains([]string(policy.CompanyIDs), company.ID) {
		return false
	}
	return len(policy.CompanyTierIDs) == 0 || slices.Contains([]string(policy.CompanyTierIDs), supportSLACompanyTier(company))
}

func (s *SupportSLAService) company(ctx context.Context, workspaceID, companyID string) (*repository.SupportSLACompany, error) {
	if !validSupportSLAID(strings.TrimSpace(companyID)) {
		return nil, ErrSupportSLACompanyNotFound
	}
	company, err := s.repo.GetCompany(ctx, workspaceID, strings.TrimSpace(companyID))
	if err != nil {
		return nil, err
	}
	if company == nil {
		return nil, ErrSupportSLACompanyNotFound
	}
	return company, nil
}

// CompanySLA returns a company's tier and the enabled policies that can
// apply to its conversations, in evaluation order.
func (s *SupportSLAService) CompanySLA(ctx context.Context, workspaceID, companyID string) (*model.SupportCompanySLA, error) {
	company, err := s.company(ctx, workspaceID, companyID)
	if err != nil {
		return nil, err
	}
	result := &model.SupportCompanySLA{CompanyID: company.ID, TierID: company.TierID, Policies: []model.SupportCompanySLAItem{}}
	if company.TierID != nil {
		if tier, err := s.repo.GetTier(ctx, workspaceID, *company.TierID); err == nil {
			result.TierName = &tier.Name
		} else if !errors.Is(err, repository.ErrSupportSLATierNotFound) {
			return nil, err
		}
	}
	policies, err := s.workspacePolicies(ctx, newSupportSLABatch(), workspaceID)
	if err != nil {
		return nil, err
	}
	// A disabled custom policy still counts, so the company can remove it.
	all, err := s.repo.ListPolicies(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	for _, policy := range all {
		result.HasCustom = result.HasCustom || supportSLACustomFor(policy, company.ID)
	}
	for _, policy := range policies {
		if !supportSLAMayApplyToCompany(policy, company) {
			continue
		}
		custom := supportSLACustomFor(policy, company.ID)
		result.Policies = append(result.Policies, model.SupportCompanySLAItem{
			ID: policy.ID, Name: policy.Name, Custom: custom, Priorities: policy.Priorities,
			FirstResponseMinutes: policy.FirstResponseMinutes, NextResponseMinutes: policy.NextResponseMinutes,
			ResolutionMinutes: policy.ResolutionMinutes, BusinessHours: policy.BusinessHours,
		})
	}
	return result, nil
}

// CustomizeCompanySLA copies the enabled policies that apply to a company
// into policies for that company only, placed first. Like any new policy,
// they apply to the company's conversations created from now on.
func (s *SupportSLAService) CustomizeCompanySLA(ctx context.Context, workspaceID, actorID, companyID string) ([]model.SupportSLAPolicy, error) {
	if err := s.requireFeature(ctx, workspaceID); err != nil {
		return nil, err
	}
	company, err := s.company(ctx, workspaceID, companyID)
	if err != nil {
		return nil, err
	}
	policies, err := s.repo.ListPolicies(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	names, err := s.repo.CompanyNames(ctx, workspaceID, []string{company.ID})
	if err != nil {
		return nil, err
	}
	now := s.now()
	var copies []model.SupportSLAPolicy
	for _, policy := range policies {
		if supportSLACustomFor(policy, company.ID) {
			return nil, ErrSupportSLACustomExists
		}
		if !policy.Enabled || !supportSLAMayApplyToCompany(policy, company) {
			continue
		}
		custom := policy
		custom.ID = uuid.NewString()
		custom.Name = truncateRunes(names[company.ID]+" · "+policy.Name, 119)
		custom.CompanyTierIDs = model.DocsStringArray{}
		custom.CompanyIDs = model.DocsStringArray{company.ID}
		custom.ActiveSince, custom.ConditionsChangedAt, custom.CreatedByID = now, nil, nil
		if strings.TrimSpace(actorID) != "" {
			custom.CreatedByID = &actorID
		}
		copies = append(copies, custom)
	}
	if len(copies) == 0 {
		return nil, invalidSLAPolicy("no SLA policy applies to this company yet; create one first")
	}
	if len(policies)+len(copies) > model.SupportSLAPolicyLimit {
		return nil, invalidSLAPolicy("a workspace can have up to %d SLA policies", model.SupportSLAPolicyLimit)
	}
	if err := s.repo.CreatePoliciesFirst(ctx, workspaceID, copies); err != nil {
		return nil, err
	}
	return s.ListPolicies(ctx, workspaceID)
}

// RemoveCompanySLA deletes the company's custom policies. Its open
// conversations move to the next matching policy.
func (s *SupportSLAService) RemoveCompanySLA(ctx context.Context, workspaceID, companyID string) error {
	company, err := s.company(ctx, workspaceID, companyID)
	if err != nil {
		return err
	}
	policies, err := s.repo.ListPolicies(ctx, workspaceID)
	if err != nil {
		return err
	}
	var custom []string
	for _, policy := range policies {
		if supportSLACustomFor(policy, company.ID) {
			custom = append(custom, policy.ID)
		}
	}
	if len(custom) == 0 {
		return repository.ErrSupportSLAPolicyNotFound
	}
	if err := s.repo.DeletePolicies(ctx, workspaceID, custom); err != nil {
		return err
	}
	return s.repo.EnqueueOpenRecords(ctx, workspaceID, s.now())
}
