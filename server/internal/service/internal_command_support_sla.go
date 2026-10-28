package service

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/helpin-ai/helpin/server/internal/authorization"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

// SetSupportSLAService wires the SLA tier commands, so integrations such as a
// billing sync can keep company tiers current.
func (s *InternalCommandService) SetSupportSLAService(sla *SupportSLAService) {
	s.supportSLAService = sla
}

func (s *InternalCommandService) registerSupportSLACommands() {
	s.register(InternalCommandDefinition{
		Name: "support.list_sla_tiers", Module: "support", SupportedTargetTypes: []string{"workspace"},
		Tool: mustCommandToolMetadata("support.list_sla_tiers"), Execute: s.executeSupportListSLATiers,
	})
	// A tier changes what a company is promised, so only support admins set it.
	s.register(InternalCommandDefinition{
		Name: "support.set_company_sla_tier", Module: "support", Mutating: true, SupportedTargetTypes: []string{"workspace"},
		RequiredPermissionsAll: []authorization.Permission{authorization.PermSupportAdmin},
		Tool:                   mustCommandToolMetadata("support.set_company_sla_tier"), Execute: s.executeSupportSetCompanySLATier,
	})
}

func (s *InternalCommandService) executeSupportListSLATiers(ctx context.Context, meta model.InternalCommandContext, _ json.RawMessage) (json.RawMessage, error) {
	if s.supportSLAService == nil {
		return nil, errCommandInput("SLA tiers are unavailable")
	}
	tiers, err := s.supportSLAService.ListTiers(ctx, meta.WorkspaceID)
	if err != nil {
		return nil, err
	}
	items := make([]map[string]any, 0, len(tiers))
	for _, tier := range tiers {
		items = append(items, map[string]any{"id": tier.ID, "name": tier.Name, "description": tier.Description, "company_count": tier.CompanyCount})
	}
	return mustJSON(map[string]any{"tiers": items}), nil
}

func (s *InternalCommandService) executeSupportSetCompanySLATier(ctx context.Context, meta model.InternalCommandContext, input json.RawMessage) (json.RawMessage, error) {
	if s.supportSLAService == nil {
		return nil, errCommandInput("SLA tiers are unavailable")
	}
	if strings.TrimSpace(meta.ActorRole) == "" {
		return nil, errCommandInput("setting an SLA tier requires a workspace member with support admin access")
	}
	var req struct {
		CompanyID string  `json:"company_id"`
		Tier      *string `json:"tier"`
	}
	if err := json.Unmarshal(input, &req); err != nil {
		return nil, errCommandInput("invalid input")
	}
	companyID := strings.TrimSpace(req.CompanyID)
	if companyID == "" {
		return nil, errCommandInput("company_id is required")
	}
	var tier *model.SupportSLATier
	if req.Tier != nil && strings.TrimSpace(*req.Tier) != "" {
		wanted := strings.TrimSpace(*req.Tier)
		tiers, err := s.supportSLAService.ListTiers(ctx, meta.WorkspaceID)
		if err != nil {
			return nil, err
		}
		var names []string
		for i := range tiers {
			names = append(names, tiers[i].Name)
			if tiers[i].ID == wanted || strings.EqualFold(tiers[i].Name, wanted) {
				tier = &tiers[i]
			}
		}
		if tier == nil {
			return nil, errCommandInput("unknown SLA tier %q; tiers are: %s", wanted, strings.Join(names, ", "))
		}
	}
	setReq := model.SetSupportSLATierRequest{CompanyIDs: []string{companyID}}
	if tier != nil {
		setReq.TierID = &tier.ID
	}
	updated, err := s.supportSLAService.SetCompanyTier(ctx, meta.WorkspaceID, setReq)
	switch {
	case errors.Is(err, ErrSupportSLACompanyNotFound):
		return nil, errCommandNotFound("company")
	case errors.Is(err, ErrSupportSLAPolicyInvalid), errors.Is(err, repository.ErrSupportSLATierNotFound):
		return nil, errCommandInput("%s", strings.TrimPrefix(err.Error(), ErrSupportSLAPolicyInvalid.Error()+": "))
	case err != nil:
		return nil, err
	}
	result := map[string]any{"company_id": companyID, "changed": updated > 0, "tier": nil}
	if tier != nil {
		result["tier"] = map[string]any{"id": tier.ID, "name": tier.Name}
	}
	return mustJSON(result), nil
}
