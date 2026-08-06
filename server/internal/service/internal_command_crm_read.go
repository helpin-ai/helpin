package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// registerCRMReadCommands registers command-backed variants of the native
// read-only CRM tools (list_deals, list_contacts, list_buyer_signals).
func (s *InternalCommandService) registerCRMReadCommands() {
	s.register(InternalCommandDefinition{
		Name:     "crm.list_deals",
		Module:   "crm",
		Mutating: false,
		Tool:     mustCommandToolMetadata("crm.list_deals"),
		Execute:  s.executeListDeals,
	})
	s.register(InternalCommandDefinition{
		Name:     "crm.list_contacts",
		Module:   "crm",
		Mutating: false,
		Tool:     mustCommandToolMetadata("crm.list_contacts"),
		Execute:  s.executeListContacts,
	})
	s.register(InternalCommandDefinition{
		Name:     "crm.list_buyer_signals",
		Module:   "crm",
		Mutating: false,
		Tool:     mustCommandToolMetadata("crm.list_buyer_signals"),
		Execute:  s.executeListBuyerSignals,
	})
}

func (s *InternalCommandService) executeListDeals(ctx context.Context, meta model.InternalCommandContext, input json.RawMessage) (json.RawMessage, error) {
	if s.crmDealService == nil {
		return nil, fmt.Errorf("CRM deal service is not configured")
	}
	limit, offset, query, err := parseCRMListLimit(input)
	if err != nil {
		return nil, err
	}
	deals, total, err := s.crmDealService.List(ctx, meta.WorkspaceID, model.CRMDealListFilters{Search: stringPtrOrNil(query)}, model.PMPagination{Page: 1, PerPage: limit, Offset: &offset})
	if err != nil {
		return nil, fmt.Errorf("list deals: %w", err)
	}
	type dealSummary struct {
		ID           string   `json:"id"`
		MarkdownLink string   `json:"markdown_link"`
		Name         string   `json:"name"`
		Stage        string   `json:"stage,omitempty"`
		Amount       *float64 `json:"amount,omitempty"`
	}
	summaries := make([]dealSummary, 0, len(deals))
	for _, d := range deals {
		stageName := ""
		if d.Stage != nil {
			stageName = d.Stage.Name
		}
		summaries = append(summaries, dealSummary{ID: d.ID, MarkdownLink: helpinMarkdownLink(d.Name, "deals", d.ID), Name: d.Name, Stage: stageName, Amount: d.Amount})
	}
	response := commandPaginationOutput(total, offset, limit, len(summaries))
	response["deals"] = summaries
	return mustJSON(response), nil
}

func (s *InternalCommandService) executeListContacts(ctx context.Context, meta model.InternalCommandContext, input json.RawMessage) (json.RawMessage, error) {
	if s.crmContactService == nil {
		return nil, fmt.Errorf("CRM contact service is not configured")
	}
	limit, offset, query, err := parseCRMListLimit(input)
	if err != nil {
		return nil, err
	}
	contacts, total, err := s.crmContactService.List(ctx, meta.WorkspaceID, model.CRMContactListFilters{Search: stringPtrOrNil(query)}, model.PMPagination{Page: 1, PerPage: limit, Offset: &offset})
	if err != nil {
		return nil, fmt.Errorf("list contacts: %w", err)
	}
	type contactSummary struct {
		ID           string  `json:"id"`
		MarkdownLink string  `json:"markdown_link"`
		FirstName    string  `json:"first_name"`
		LastName     *string `json:"last_name,omitempty"`
		Email        *string `json:"email,omitempty"`
		JobTitle     *string `json:"job_title,omitempty"`
	}
	summaries := make([]contactSummary, 0, len(contacts))
	for _, c := range contacts {
		label := strings.TrimSpace(c.FirstName + " " + commandDerefString(c.LastName))
		if label == "" {
			label = commandDerefString(c.Email)
		}
		summaries = append(summaries, contactSummary{ID: c.ID, MarkdownLink: helpinMarkdownLink(label, "contacts", c.ID), FirstName: c.FirstName, LastName: c.LastName, Email: c.Email, JobTitle: c.JobTitle})
	}
	response := commandPaginationOutput(total, offset, limit, len(summaries))
	response["contacts"] = summaries
	return mustJSON(response), nil
}

func (s *InternalCommandService) executeListBuyerSignals(ctx context.Context, meta model.InternalCommandContext, input json.RawMessage) (json.RawMessage, error) {
	if s.crmSignalService == nil {
		return nil, fmt.Errorf("CRM signal service is not configured")
	}
	var req struct {
		DealID *string `json:"deal_id"`
		Limit  int     `json:"limit"`
		Offset int     `json:"offset"`
	}
	if len(input) > 0 {
		if err := json.Unmarshal(input, &req); err != nil {
			return nil, fmt.Errorf("parse list buyer signals input: %w", err)
		}
	}
	limit := req.Limit
	if limit == 0 {
		limit = 20
	}
	if limit < 1 || limit > 50 {
		return nil, fmt.Errorf("limit must be between 1 and 50")
	}
	if req.Offset < 0 {
		return nil, fmt.Errorf("offset must be zero or greater")
	}
	dealID := req.DealID
	if dealID == nil && strings.TrimSpace(meta.TargetType) == "crm_deal" && strings.TrimSpace(meta.TargetID) != "" {
		dealID = stringPtrOrNil(meta.TargetID)
	}
	signals, total, err := s.crmSignalService.ListSignals(ctx, meta.WorkspaceID, model.CRMBuyerSignalListFilters{DealID: dealID}, model.PMPagination{Page: 1, PerPage: limit, Offset: &req.Offset})
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
	response := commandPaginationOutput(total, req.Offset, limit, len(summaries))
	response["buyer_signals"] = summaries
	return mustJSON(response), nil
}

func parseCRMListLimit(input json.RawMessage) (int, int, string, error) {
	var req struct {
		Limit  int    `json:"limit"`
		Offset int    `json:"offset"`
		Query  string `json:"query"`
	}
	if len(input) > 0 {
		if err := json.Unmarshal(input, &req); err != nil {
			return 0, 0, "", fmt.Errorf("parse list input: %w", err)
		}
	}
	if req.Limit == 0 {
		req.Limit = 20
	}
	if req.Limit < 1 || req.Limit > 50 {
		return 0, 0, "", fmt.Errorf("limit must be between 1 and 50")
	}
	if req.Offset < 0 {
		return 0, 0, "", fmt.Errorf("offset must be zero or greater")
	}
	return req.Limit, req.Offset, strings.TrimSpace(req.Query), nil
}
