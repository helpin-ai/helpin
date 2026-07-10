package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/helpin-ai/helpin/server/internal/commandtools"
	"github.com/helpin-ai/helpin/server/internal/model"
)

// registerCRMReadCommands registers command-backed variants of the native
// read-only CRM tools (list_deals, list_contacts, list_buyer_signals).
func (s *InternalCommandService) registerCRMReadCommands() {
	s.register(InternalCommandDefinition{
		Name:     "crm.list_deals",
		Module:   "crm",
		Mutating: false,
		Tool: &commandtools.RuntimeToolMetadata{
			CommandName: "crm.list_deals",
			Alias:       "list_deals",
			Category:    "CRM",
			Description: "List CRM deals in the workspace. Returns deal name, stage, and amount.",
			InputSchema: crmListLimitSchema("Maximum number of deals to return (default 20, max 50)"),
		},
		Execute: s.executeListDeals,
	})
	s.register(InternalCommandDefinition{
		Name:     "crm.list_contacts",
		Module:   "crm",
		Mutating: false,
		Tool: &commandtools.RuntimeToolMetadata{
			CommandName: "crm.list_contacts",
			Alias:       "list_contacts",
			Category:    "CRM",
			Description: "List CRM contacts in the workspace. Returns name, email, and job title.",
			InputSchema: crmListLimitSchema("Maximum number of contacts to return (default 20, max 50)"),
		},
		Execute: s.executeListContacts,
	})
	s.register(InternalCommandDefinition{
		Name:     "crm.list_buyer_signals",
		Module:   "crm",
		Mutating: false,
		Tool: &commandtools.RuntimeToolMetadata{
			CommandName: "crm.list_buyer_signals",
			Alias:       "list_buyer_signals",
			Category:    "CRM",
			Description: "List detected buyer signals from emails, meetings, and support conversations.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"deal_id": map[string]any{
						"type":        "string",
						"description": "Optional deal ID to filter signals for a specific deal",
					},
					"limit": map[string]any{
						"type":        "integer",
						"description": "Maximum number of signals to return (default 20, max 50)",
					},
				},
			},
		},
		Execute: s.executeListBuyerSignals,
	})
}

func (s *InternalCommandService) executeListDeals(ctx context.Context, meta model.InternalCommandContext, input json.RawMessage) (json.RawMessage, error) {
	if s.crmDealService == nil {
		return nil, fmt.Errorf("CRM deal service is not configured")
	}
	limit, err := parseCRMListLimit(input)
	if err != nil {
		return nil, err
	}
	deals, _, err := s.crmDealService.List(ctx, meta.WorkspaceID, model.CRMDealListFilters{}, model.PMPagination{Page: 1, PerPage: limit})
	if err != nil {
		return nil, fmt.Errorf("list deals: %w", err)
	}
	type dealSummary struct {
		ID     string   `json:"id"`
		Name   string   `json:"name"`
		Stage  string   `json:"stage,omitempty"`
		Amount *float64 `json:"amount,omitempty"`
	}
	summaries := make([]dealSummary, 0, len(deals))
	for _, d := range deals {
		stageName := ""
		if d.Stage != nil {
			stageName = d.Stage.Name
		}
		summaries = append(summaries, dealSummary{ID: d.ID, Name: d.Name, Stage: stageName, Amount: d.Amount})
	}
	return mustJSON(summaries), nil
}

func (s *InternalCommandService) executeListContacts(ctx context.Context, meta model.InternalCommandContext, input json.RawMessage) (json.RawMessage, error) {
	if s.crmContactService == nil {
		return nil, fmt.Errorf("CRM contact service is not configured")
	}
	limit, err := parseCRMListLimit(input)
	if err != nil {
		return nil, err
	}
	contacts, _, err := s.crmContactService.List(ctx, meta.WorkspaceID, model.CRMContactListFilters{}, model.PMPagination{Page: 1, PerPage: limit})
	if err != nil {
		return nil, fmt.Errorf("list contacts: %w", err)
	}
	type contactSummary struct {
		ID        string  `json:"id"`
		FirstName string  `json:"first_name"`
		LastName  *string `json:"last_name,omitempty"`
		Email     *string `json:"email,omitempty"`
		JobTitle  *string `json:"job_title,omitempty"`
	}
	summaries := make([]contactSummary, 0, len(contacts))
	for _, c := range contacts {
		summaries = append(summaries, contactSummary{ID: c.ID, FirstName: c.FirstName, LastName: c.LastName, Email: c.Email, JobTitle: c.JobTitle})
	}
	return mustJSON(summaries), nil
}

func (s *InternalCommandService) executeListBuyerSignals(ctx context.Context, meta model.InternalCommandContext, input json.RawMessage) (json.RawMessage, error) {
	if s.crmSignalService == nil {
		return nil, fmt.Errorf("CRM signal service is not configured")
	}
	var req struct {
		DealID *string `json:"deal_id"`
		Limit  int     `json:"limit"`
	}
	if len(input) > 0 {
		if err := json.Unmarshal(input, &req); err != nil {
			return nil, fmt.Errorf("parse list buyer signals input: %w", err)
		}
	}
	limit := req.Limit
	if limit <= 0 || limit > 50 {
		limit = 20
	}
	dealID := req.DealID
	if dealID == nil && strings.TrimSpace(meta.TargetType) == "crm_deal" && strings.TrimSpace(meta.TargetID) != "" {
		dealID = stringPtrOrNil(meta.TargetID)
	}
	signals, _, err := s.crmSignalService.ListSignals(ctx, meta.WorkspaceID, model.CRMBuyerSignalListFilters{DealID: dealID}, model.PMPagination{Page: 1, PerPage: limit})
	if err != nil {
		return nil, fmt.Errorf("list buyer signals: %w", err)
	}
	type signalSummary struct {
		ID         string  `json:"id"`
		SignalType string  `json:"signal_type"`
		Summary    string  `json:"summary"`
		Confidence float64 `json:"confidence"`
		DealID     *string `json:"deal_id,omitempty"`
		ContactID  *string `json:"contact_id,omitempty"`
	}
	summaries := make([]signalSummary, 0, len(signals))
	for _, sig := range signals {
		summaries = append(summaries, signalSummary{
			ID:         sig.ID,
			SignalType: sig.SignalType,
			Summary:    sig.Summary,
			Confidence: sig.Confidence,
			DealID:     sig.DealID,
			ContactID:  sig.ContactID,
		})
	}
	return mustJSON(summaries), nil
}

func parseCRMListLimit(input json.RawMessage) (int, error) {
	var req struct {
		Limit int `json:"limit"`
	}
	if len(input) > 0 {
		if err := json.Unmarshal(input, &req); err != nil {
			return 0, fmt.Errorf("parse list input: %w", err)
		}
	}
	if req.Limit <= 0 || req.Limit > 50 {
		return 20, nil
	}
	return req.Limit, nil
}

func crmListLimitSchema(limitDescription string) map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"limit": map[string]any{
				"type":        "integer",
				"description": limitDescription,
			},
		},
	}
}
