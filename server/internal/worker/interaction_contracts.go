package worker

import (
	"fmt"
	"strings"
)

const (
	InteractionKindRequestUserInput = "request_user_input"
	InteractionKindReviewCheckpoint = "review_checkpoint"

	InteractionTransportTypeToolCall      = "tool_call"
	InteractionTransportTypeFencedJSON    = "fenced_json"
	InteractionTransportTypeRuntimeBridge = "runtime_bridge"

	requestUserInputSchemaV1 = "request_user_input_v1"
	reviewCheckpointSchemaV1 = "review_checkpoint_v1"
)

func NormalizeInteractionContracts(contracts []SkillInteractionContract) []SkillInteractionContract {
	if len(contracts) == 0 {
		return nil
	}

	merged := make(map[string]SkillInteractionContract, len(contracts))
	order := make([]string, 0, len(contracts))
	for _, contract := range contracts {
		kind := strings.TrimSpace(contract.Kind)
		if kind == "" {
			continue
		}
		contract.Kind = kind
		if _, exists := merged[kind]; !exists {
			order = append(order, kind)
			merged[kind] = normalizeInteractionContract(contract)
			continue
		}
		merged[kind] = mergeInteractionContract(merged[kind], contract)
	}

	out := make([]SkillInteractionContract, 0, len(order))
	for _, kind := range order {
		out = append(out, merged[kind])
	}
	return out
}

func normalizeInteractionContract(contract SkillInteractionContract) SkillInteractionContract {
	contract.Kind = strings.TrimSpace(contract.Kind)
	contract.Schema = strings.TrimSpace(contract.Schema)
	if len(contract.Transports) == 0 {
		contract.Transports = nil
		return contract
	}

	normalized := make(map[string]SkillInteractionTransport, len(contract.Transports))
	for runtimeKind, transport := range contract.Transports {
		runtimeKind = strings.TrimSpace(runtimeKind)
		if runtimeKind == "" {
			continue
		}
		transport.Type = strings.TrimSpace(transport.Type)
		transport.ToolName = strings.TrimSpace(transport.ToolName)
		transport.BlockLabel = strings.TrimSpace(transport.BlockLabel)
		normalized[runtimeKind] = transport
	}
	if len(normalized) == 0 {
		contract.Transports = nil
		return contract
	}
	contract.Transports = normalized
	return contract
}

func mergeInteractionContract(base, incoming SkillInteractionContract) SkillInteractionContract {
	base = normalizeInteractionContract(base)
	incoming = normalizeInteractionContract(incoming)

	if base.Kind == "" {
		base.Kind = incoming.Kind
	}
	if base.Schema == "" {
		base.Schema = incoming.Schema
	}
	if len(base.Transports) == 0 && len(incoming.Transports) > 0 {
		base.Transports = make(map[string]SkillInteractionTransport, len(incoming.Transports))
	}
	for runtimeKind, incomingTransport := range incoming.Transports {
		existing := base.Transports[runtimeKind]
		if existing.Type == "" {
			existing.Type = incomingTransport.Type
		}
		if existing.ToolName == "" {
			existing.ToolName = incomingTransport.ToolName
		}
		if existing.BlockLabel == "" {
			existing.BlockLabel = incomingTransport.BlockLabel
		}
		base.Transports[runtimeKind] = existing
	}
	return base
}

func (p SkillPolicy) InteractionContract(kind string) (SkillInteractionContract, bool) {
	kind = strings.TrimSpace(kind)
	if kind == "" {
		return SkillInteractionContract{}, false
	}
	for _, contract := range NormalizeInteractionContracts(p.InteractionContracts) {
		if strings.TrimSpace(contract.Kind) == kind {
			return contract, true
		}
	}
	return SkillInteractionContract{}, false
}

func reviewCheckpointRuntimeInstructions(policy SkillPolicy, runtimeKind string) []string {
	contract, transport, ok := reviewCheckpointTransport(policy, runtimeKind)
	if !ok {
		return nil
	}
	switch strings.TrimSpace(transport.Type) {
	case InteractionTransportTypeFencedJSON:
		label := ReviewCheckpointFencedBlockLabel(policy, runtimeKind)
		schema := strings.TrimSpace(contract.Schema)
		if schema == "" {
			schema = reviewCheckpointSchemaV1
		}
		return []string{
			fmt.Sprintf("For review turns in this runtime, you cannot call Helpin interaction tools directly. When you are ready to hand control back, keep the human-readable review text, then end with a fenced code block labeled `%s` containing JSON only.", label),
			fmt.Sprintf("Use the `%s` schema for that block: {\"phase\":\"...\",\"title\":\"...\",\"summary\":\"...\",\"findings\":[{\"title\":\"...\",\"body\":\"...\",\"priority\":\"P1|P2|P3\",\"confidence\":0.0,\"code_location\":\"path:line\"}],\"overall_correctness\":\"correct|incorrect|partially_correct\",\"overall_explanation\":\"...\",\"overall_confidence_score\":0.0}.", schema),
			fmt.Sprintf("If there are no discrete findings, still include the `%s` block with an empty `findings` array and the overall verdict fields.", label),
		}
	default:
		return nil
	}
}

func RequestUserInputUsesRuntimeBridge(policy SkillPolicy, runtimeKind string) bool {
	contract, transport, ok := requestUserInputTransport(policy, runtimeKind)
	if !ok {
		// Preserve current behavior for runtimes that already have a working bridge
		// even when older skills have not declared the contract yet.
		switch strings.TrimSpace(runtimeKind) {
		case "codex", "opencode":
			return true
		default:
			return false
		}
	}
	if strings.TrimSpace(contract.Kind) == "" {
		return false
	}
	return strings.TrimSpace(transport.Type) == InteractionTransportTypeRuntimeBridge
}

func ReviewCheckpointFencedBlockLabel(policy SkillPolicy, runtimeKind string) string {
	_, transport, ok := reviewCheckpointTransport(policy, runtimeKind)
	if !ok || strings.TrimSpace(transport.Type) != InteractionTransportTypeFencedJSON {
		return "helpin-review"
	}
	label := strings.TrimSpace(transport.BlockLabel)
	if label == "" {
		return "helpin-review"
	}
	return label
}

func reviewCheckpointTransport(policy SkillPolicy, runtimeKind string) (SkillInteractionContract, SkillInteractionTransport, bool) {
	contract, ok := policy.InteractionContract(InteractionKindReviewCheckpoint)
	if !ok {
		return SkillInteractionContract{}, SkillInteractionTransport{}, false
	}
	transport, ok := contract.Transports[strings.TrimSpace(runtimeKind)]
	if !ok {
		return contract, SkillInteractionTransport{}, false
	}
	return contract, transport, true
}

func requestUserInputTransport(policy SkillPolicy, runtimeKind string) (SkillInteractionContract, SkillInteractionTransport, bool) {
	contract, ok := policy.InteractionContract(InteractionKindRequestUserInput)
	if !ok {
		return SkillInteractionContract{}, SkillInteractionTransport{}, false
	}
	transport, ok := contract.Transports[strings.TrimSpace(runtimeKind)]
	if !ok {
		return contract, SkillInteractionTransport{}, false
	}
	return contract, transport, true
}
