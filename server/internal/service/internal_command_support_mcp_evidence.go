package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/helpin-ai/helpin/server/internal/commandtools"
	"github.com/helpin-ai/helpin/server/internal/model"
)

const supportMCPResultMaxBytes = 16 * 1024

// registerSupportMCPEvidenceCommand makes one audited read-only MCP result
// available to the reply gate. The result stays internal and never becomes a
// customer-visible source or part of the command response.
func (s *InternalCommandService) registerSupportMCPEvidenceCommand() {
	s.register(InternalCommandDefinition{
		Name: "support.register_mcp_evidence", Module: "support", Mutating: false,
		SupportedTargetTypes: supportCommandTargetTypes,
		Tool: &commandtools.RuntimeToolMetadata{
			CommandName: "support.register_mcp_evidence", Alias: "register_support_mcp_evidence", Category: "Support",
			Description: "After a read-only external MCP lookup for this support customer, register its audited result as private evidence. The lookup must select the customer by their conversation email, phone, or linked CRM contact ID. Pass the exact MCP tool name. Only the latest call from this customer turn is considered. Returns an evidence_id for send_support_reply; never returns or publishes the private result. If registration is refused, clarify or hand off.",
			InputSchema: map[string]any{"type": "object", "properties": map[string]any{"tool_name": map[string]any{"type": "string", "description": "Exact mcp__server__tool alias just called."}}, "required": []string{"tool_name"}, "additionalProperties": false},
		},
		Execute: s.executeRegisterSupportMCPEvidence,
	})
}

func (s *InternalCommandService) executeRegisterSupportMCPEvidence(ctx context.Context, meta model.InternalCommandContext, input json.RawMessage) (json.RawMessage, error) {
	var req struct {
		ToolName string `json:"tool_name"`
	}
	if err := json.Unmarshal(input, &req); err != nil {
		return nil, fmt.Errorf("parse MCP evidence input: %w", err)
	}
	alias := strings.TrimSpace(req.ToolName)
	if !strings.HasPrefix(alias, "mcp__") || strings.HasPrefix(alias, "mcp__helpin__") {
		return nil, errCommandInput("an external MCP tool name is required")
	}
	if s.agentService == nil || s.agentService.externalMCPService == nil || s.agentService.externalMCPService.repo == nil || s.supportRunEvidenceRepo == nil || s.supportAIService == nil || s.supportAIService.conversationRepo == nil {
		return nil, fmt.Errorf("support MCP evidence is not configured")
	}
	client, ok := s.agentService.agentRuntimeClient.(agentRuntimeToolCallClient)
	if !ok {
		return nil, fmt.Errorf("runtime tool-call audit is unavailable")
	}
	run, err := s.resolveCommandRun(ctx, meta)
	if err != nil {
		return nil, err
	}
	runtimeRunID, ok := agentRuntimeRunID(run)
	if !ok || run.TargetType != "support_conversation" || run.TargetID != commandConversationTargetID(meta) || (meta.AgentID != "" && meta.AgentID != run.AgentID) {
		return nil, errCommandInput("MCP evidence requires the current support conversation run")
	}
	var conv model.SupportConversation
	err = s.supportAIService.conversationRepo.DB().WithContext(ctx).Table("support_conversations").
		Select("id, workspace_id, customer_email, customer_phone, crm_contact_id").
		Where("id = ? AND workspace_id = ?", run.TargetID, run.WorkspaceID).Take(&conv).Error
	if err != nil {
		return nil, fmt.Errorf("load support customer: %w", err)
	}
	var lastCustomerMessage struct{ CreatedAt time.Time }
	err = s.supportAIService.conversationRepo.DB().WithContext(ctx).Table("support_messages").
		Select("created_at").Where("workspace_id = ? AND conversation_id = ? AND sender_type = ? AND message_type = ? AND is_internal = ?", run.WorkspaceID, run.TargetID, "customer", "reply", false).
		Order("created_at DESC").Take(&lastCustomerMessage).Error
	if err != nil || lastCustomerMessage.CreatedAt.IsZero() {
		return nil, errCommandInput("the current customer turn could not be verified")
	}
	bindings, err := s.agentService.externalMCPService.repo.ListRunBindings(ctx, run.WorkspaceID, run.ID)
	if err != nil {
		return nil, err
	}
	tools, err := s.agentService.externalMCPService.repo.ListEnabledToolsByAliases(ctx, run.WorkspaceID, []string{alias})
	if err != nil {
		return nil, err
	}
	if len(tools) != 1 || tools[0].Access != model.ExternalMCPToolAccessRead || !supportMCPAliasBound(bindings, runtimeRunID, tools[0].ServerID, alias) {
		return nil, errCommandInput("the MCP tool is not an assigned read-only tool for this run")
	}
	calls, err := client.ListToolCalls(ctx, runtimeRunID)
	if err != nil {
		return nil, fmt.Errorf("load audited MCP calls: %w", err)
	}
	cutoff := run.CreatedAt
	if lastCustomerMessage.CreatedAt.After(cutoff) {
		cutoff = lastCustomerMessage.CreatedAt
	}
	call := latestSupportMCPReadCall(calls, runtimeRunID, alias, cutoff, &conv)
	if call == nil {
		return nil, errCommandInput("no successful customer-matched read from this MCP tool was found")
	}
	evidenceID := "mcp-result:" + strings.TrimSpace(call.ID)
	row := model.SupportRunEvidence{
		WorkspaceID: run.WorkspaceID, RunID: run.ID, EvidenceID: evidenceID, ReferenceID: evidenceID,
		SourceType: "external_mcp", SourceID: strings.TrimSpace(call.ID), Title: "Verified customer account data",
		IsInternal: true, Content: string(call.Output), VectorScore: 0.95, CombinedScore: 0.95,
	}
	if err := s.supportRunEvidenceRepo.UpsertBatch(ctx, []model.SupportRunEvidence{row}); err != nil {
		return nil, fmt.Errorf("register MCP evidence: %w", err)
	}
	return mustJSON(map[string]any{"evidence_id": evidenceID, "is_internal": true, "next_action": "Cite this evidence_id for only the customer-specific facts it supports. Keep raw records and logs out of the reply."}), nil
}

func supportMCPAliasBound(bindings []model.AgentRunExternalMCPBinding, runtimeRunID, serverID, alias string) bool {
	for _, binding := range bindings {
		if binding.RuntimeRunID != runtimeRunID || binding.ServerID != serverID {
			continue
		}
		var aliases []string
		if json.Unmarshal(binding.ToolAliases, &aliases) != nil {
			continue
		}
		for _, allowed := range aliases {
			if allowed == alias {
				return true
			}
		}
	}
	return false
}

func latestSupportMCPReadCall(calls []AgentRuntimeToolCall, runtimeRunID, alias string, runCreatedAt time.Time, conv *model.SupportConversation) *AgentRuntimeToolCall {
	var latest *AgentRuntimeToolCall
	for i := range calls {
		call := &calls[i]
		if call.RunID != runtimeRunID || call.ToolName != alias || call.CreatedAt.IsZero() || (!runCreatedAt.IsZero() && call.CreatedAt.Before(runCreatedAt)) {
			continue
		}
		if latest == nil || call.CreatedAt.After(latest.CreatedAt) {
			latest = call
		}
	}
	if latest == nil || latest.Mutating || strings.TrimSpace(latest.Error) != "" || strings.TrimSpace(latest.ID) == "" || len(latest.Output) == 0 || len(latest.Output) > supportMCPResultMaxBytes || !supportMCPInputMatchesCustomer(latest.Input, conv) {
		return nil
	}
	var result any
	if json.Unmarshal(latest.Output, &result) != nil {
		return nil
	}
	switch value := result.(type) {
	case map[string]any:
		if len(value) == 0 || value["isError"] == true {
			return nil
		}
	case []any:
		if len(value) == 0 {
			return nil
		}
	case string:
		if strings.TrimSpace(value) == "" {
			return nil
		}
	default:
		return nil
	}
	return latest
}

func supportMCPInputMatchesCustomer(input json.RawMessage, conv *model.SupportConversation) bool {
	if conv == nil {
		return false
	}
	identifiers := map[string]bool{}
	for _, value := range []*string{conv.CustomerEmail, conv.CustomerPhone, conv.CRMContactID} {
		if value != nil && strings.TrimSpace(*value) != "" {
			identifiers[strings.ToLower(strings.TrimSpace(*value))] = true
		}
	}
	if len(identifiers) == 0 {
		return false
	}
	var parsed any
	if json.Unmarshal(input, &parsed) != nil {
		return false
	}
	selectorKeys := map[string]bool{
		"email": true, "customer_email": true, "customeremail": true, "user_email": true, "contact_email": true,
		"phone": true, "customer_phone": true, "customerphone": true,
		"customer_id": true, "customerid": true, "contact_id": true, "contactid": true,
		"user_id": true, "userid": true, "query": true, "search": true,
	}
	matched, mismatched := false, false
	var inspect func(any, string)
	inspect = func(value any, key string) {
		switch typed := value.(type) {
		case string:
			if selectorKeys[key] {
				if identifiers[strings.ToLower(strings.TrimSpace(typed))] {
					matched = true
				} else {
					mismatched = true
				}
			}
		case []any:
			for _, item := range typed {
				inspect(item, key)
			}
		case map[string]any:
			for childKey, item := range typed {
				inspect(item, strings.ToLower(strings.ReplaceAll(childKey, "-", "_")))
			}
		}
	}
	inspect(parsed, "")
	return matched && !mismatched
}

func supportEvidenceRowsForTurn(rows []model.SupportRunEvidence, customerMessageAt time.Time) []model.SupportRunEvidence {
	current := make([]model.SupportRunEvidence, 0, len(rows))
	for _, row := range rows {
		if row.SourceType == "external_mcp" && (customerMessageAt.IsZero() || row.CreatedAt.Before(customerMessageAt)) {
			continue
		}
		current = append(current, row)
	}
	return current
}
