package service

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"github.com/helpin-ai/helpin/server/internal/agentcontract"
	"github.com/helpin-ai/helpin/server/internal/model"
)

func isScheduledSupportFollowUpRun(run *model.AgentRun) bool {
	if run == nil || run.TargetType != "support_conversation" || strings.TrimSpace(run.TargetID) == "" {
		return false
	}
	var input model.AgentRunInputPayload
	if json.Unmarshal(run.Input, &input) != nil || input.Trigger == nil || input.Trigger.Source != model.AgentRunTriggerSourceSystem || input.Trigger.TriggerType != supportFollowUpTriggerType {
		return false
	}
	var context struct {
		FollowUpID string `json:"follow_up_id"`
	}
	return json.Unmarshal(input.Trigger.Context, &context) == nil && strings.TrimSpace(context.FollowUpID) != ""
}

// The scheduled outcome is a product-owned operation whose executor is fenced
// to its assigned episode and run. Older/custom support agents need this new
// operation for scheduled work without changing their saved general permissions.
func runtimeAgentForScheduledSupportFollowUp(run *model.AgentRun, agent AgentRuntimeAgent) (AgentRuntimeAgent, error) {
	if !isScheduledSupportFollowUpRun(run) {
		return agent, nil
	}
	tools := agentcontract.NormalizeToolNames(agent.AllowedTools)
	for _, name := range []string{"get_support_conversation", "list_conversation_messages"} {
		if !slices.Contains(tools, name) {
			return AgentRuntimeAgent{}, fmt.Errorf("scheduled support follow-up requires agent tool %q", name)
		}
	}
	if !slices.Contains(tools, "finish_support_follow_up") {
		tools = append(tools, "finish_support_follow_up")
	}
	agent.AllowedTools = tools
	return agent, nil
}

func validateScheduledSupportFollowUpTools(run *model.AgentRun, tools []string) error {
	if !isScheduledSupportFollowUpRun(run) {
		return nil
	}
	for _, name := range []string{"get_support_conversation", "list_conversation_messages", "finish_support_follow_up"} {
		if !slices.Contains(tools, name) {
			return fmt.Errorf("scheduled support follow-up requires executable runtime tool %q", name)
		}
	}
	return nil
}
