package worker

import (
	"encoding/json"
	"fmt"
	"strings"
)

func toolListDeals(ctx *ExecutionContext, input json.RawMessage) (string, error) {
	if ctx.Services == nil || ctx.Services.ListDeals == nil {
		return "", fmt.Errorf("CRM deal access is not available for this agent")
	}
	var params struct {
		Limit int `json:"limit"`
	}
	_ = json.Unmarshal(input, &params)
	if params.Limit <= 0 || params.Limit > 50 {
		params.Limit = 20
	}

	deals, err := ctx.Services.ListDeals(ctx.Context, ctx.WorkspaceID, params.Limit)
	if err != nil {
		return "", fmt.Errorf("list deals: %w", err)
	}
	if len(deals) == 0 {
		return "No deals found.", nil
	}

	type dealSummary struct {
		ID     string   `json:"id"`
		Name   string   `json:"name"`
		Stage  string   `json:"stage,omitempty"`
		Amount *float64 `json:"amount,omitempty"`
	}
	summaries := make([]dealSummary, len(deals))
	for i, d := range deals {
		stageName := ""
		if d.Stage != nil {
			stageName = d.Stage.Name
		}
		summaries[i] = dealSummary{ID: d.ID, Name: d.Name, Stage: stageName, Amount: d.Amount}
	}
	result, _ := json.MarshalIndent(summaries, "", "  ")
	return string(result), nil
}

func toolUpdateDealStage(ctx *ExecutionContext, input json.RawMessage) (string, error) {
	if ctx.Services == nil || ctx.Services.UpdateDealStage == nil {
		return "", fmt.Errorf("CRM deal access is not available for this agent")
	}
	var params struct {
		DealID  string `json:"deal_id"`
		StageID string `json:"stage_id"`
	}
	if err := json.Unmarshal(input, &params); err != nil {
		return "", fmt.Errorf("parse input: %w", err)
	}
	if strings.TrimSpace(params.DealID) == "" || strings.TrimSpace(params.StageID) == "" {
		return "", fmt.Errorf("deal_id and stage_id are required")
	}
	if err := ctx.Services.UpdateDealStage(ctx.Context, params.DealID, params.StageID); err != nil {
		return "", fmt.Errorf("update deal stage: %w", err)
	}
	return fmt.Sprintf("Deal %s moved to stage %s.", params.DealID, params.StageID), nil
}

func toolAddDealNote(ctx *ExecutionContext, input json.RawMessage) (string, error) {
	if ctx.Services == nil || ctx.Services.AddDealNote == nil {
		return "", fmt.Errorf("CRM deal access is not available for this agent")
	}
	var params struct {
		DealID  string `json:"deal_id"`
		Content string `json:"content"`
	}
	if err := json.Unmarshal(input, &params); err != nil {
		return "", fmt.Errorf("parse input: %w", err)
	}
	if strings.TrimSpace(params.DealID) == "" || strings.TrimSpace(params.Content) == "" {
		return "", fmt.Errorf("deal_id and content are required")
	}
	if err := ctx.Services.AddDealNote(ctx.Context, ctx.WorkspaceID, params.DealID, ctx.AgentID, params.Content); err != nil {
		return "", fmt.Errorf("add deal note: %w", err)
	}
	return "Note added to deal.", nil
}

func toolListContacts(ctx *ExecutionContext, input json.RawMessage) (string, error) {
	if ctx.Services == nil || ctx.Services.ListContacts == nil {
		return "", fmt.Errorf("CRM contact access is not available for this agent")
	}
	var params struct {
		Limit int `json:"limit"`
	}
	_ = json.Unmarshal(input, &params)
	if params.Limit <= 0 || params.Limit > 50 {
		params.Limit = 20
	}

	contacts, err := ctx.Services.ListContacts(ctx.Context, ctx.WorkspaceID, params.Limit)
	if err != nil {
		return "", fmt.Errorf("list contacts: %w", err)
	}
	if len(contacts) == 0 {
		return "No contacts found.", nil
	}

	type contactSummary struct {
		ID        string  `json:"id"`
		FirstName string  `json:"first_name"`
		LastName  *string `json:"last_name,omitempty"`
		Email     *string `json:"email,omitempty"`
		JobTitle  *string `json:"job_title,omitempty"`
	}
	summaries := make([]contactSummary, len(contacts))
	for i, c := range contacts {
		summaries[i] = contactSummary{ID: c.ID, FirstName: c.FirstName, LastName: c.LastName, Email: c.Email, JobTitle: c.JobTitle}
	}
	result, _ := json.MarshalIndent(summaries, "", "  ")
	return string(result), nil
}

func toolListBuyerSignals(ctx *ExecutionContext, input json.RawMessage) (string, error) {
	if ctx.Services == nil || ctx.Services.ListBuyerSignals == nil {
		return "", fmt.Errorf("CRM signal access is not available for this agent")
	}
	var params struct {
		DealID *string `json:"deal_id"`
		Limit  int     `json:"limit"`
	}
	_ = json.Unmarshal(input, &params)
	if params.Limit <= 0 || params.Limit > 50 {
		params.Limit = 20
	}

	signals, err := ctx.Services.ListBuyerSignals(ctx.Context, ctx.WorkspaceID, params.DealID, params.Limit)
	if err != nil {
		return "", fmt.Errorf("list buyer signals: %w", err)
	}
	if len(signals) == 0 {
		return "No buyer signals found.", nil
	}

	type signalSummary struct {
		ID         string  `json:"id"`
		SignalType string  `json:"signal_type"`
		Summary    string  `json:"summary"`
		Confidence float64 `json:"confidence"`
		DealID     *string `json:"deal_id,omitempty"`
		ContactID  *string `json:"contact_id,omitempty"`
	}
	summaries := make([]signalSummary, len(signals))
	for i, s := range signals {
		summaries[i] = signalSummary{ID: s.ID, SignalType: s.SignalType, Summary: s.Summary, Confidence: s.Confidence, DealID: s.DealID, ContactID: s.ContactID}
	}
	result, _ := json.MarshalIndent(summaries, "", "  ")
	return string(result), nil
}
