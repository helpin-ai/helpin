package agentcontract

import "testing"

func TestNormalizeInteractionContractsCanonicalizesToolCallsForEveryRuntime(t *testing.T) {
	contracts := NormalizeInteractionContracts([]SkillInteractionContract{{
		Kind: InteractionKindApprovalRequest,
		Transports: map[string]SkillInteractionTransport{
			"native_sdk": {Type: InteractionTransportTypeToolCall, ToolName: "add_task_comment"},
			"codex":      {Type: InteractionTransportTypeToolCall, ToolName: HelpinMCPToolPrefix + "add_task_comment"},
			"opencode":   {Type: InteractionTransportTypeToolCall, ToolName: "add_task_comment"},
		},
	}})
	if len(contracts) != 1 {
		t.Fatalf("contracts=%d, want one", len(contracts))
	}
	for runtimeKind, transport := range contracts[0].Transports {
		if transport.ToolName != "add_task_comment" {
			t.Fatalf("%s tool name=%q, want add_task_comment", runtimeKind, transport.ToolName)
		}
	}
}
