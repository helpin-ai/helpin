package agentcontract

import (
	"strings"
)

const (
	InteractionKindRequestUserInput = "request_user_input"
	InteractionKindApprovalRequest  = "approval_request"
	InteractionKindReviewCheckpoint = "review_checkpoint"

	InteractionTransportTypeToolCall      = "tool_call"
	InteractionTransportTypeFencedJSON    = "fenced_json"
	InteractionTransportTypeRuntimeBridge = "runtime_bridge"

	requestUserInputSchemaV1 = "request_user_input_v1"
	approvalRequestSchemaV1  = "approval_request_v1"
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

// AggregateSkillPolicies keeps runtime enforcement separate from prompt
// composition. Prompt modules may be compiled into one version-owned system
// prompt while their interaction requirements remain machine-readable.
func AggregateSkillPolicies(policies ...SkillPolicy) SkillPolicy {
	var aggregate SkillPolicy
	for _, policy := range policies {
		if policy.AllowImplicitInvocation != nil {
			value := *policy.AllowImplicitInvocation
			if aggregate.AllowImplicitInvocation == nil || !value {
				aggregate.AllowImplicitInvocation = &value
			}
		}
		aggregate.CompletionRequiresInteractionKinds = append(aggregate.CompletionRequiresInteractionKinds, policy.CompletionRequiresInteractionKinds...)
		aggregate.InteractionContracts = append(aggregate.InteractionContracts, policy.InteractionContracts...)
	}
	aggregate.CompletionRequiresInteractionKinds = SortedUniqueStrings(aggregate.CompletionRequiresInteractionKinds)
	aggregate.InteractionContracts = NormalizeInteractionContracts(aggregate.InteractionContracts)
	return aggregate
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
		if runtimeKind == "native_sdk" && transport.Type == InteractionTransportTypeToolCall && IsHelpinMCPToolAlias(transport.ToolName) {
			transport.ToolName = HelpinMCPRuntimeToolName(transport.ToolName)
		}
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
