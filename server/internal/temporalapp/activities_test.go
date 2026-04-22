package temporalapp

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/agentskills"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"github.com/helpin-ai/helpin/server/internal/tiptap"
	"github.com/helpin-ai/helpin/server/internal/websocket"
	workerpkg "github.com/helpin-ai/helpin/server/internal/worker"
)

type stubInternalCommandExecutor struct {
	executeFn func(ctx context.Context, meta model.InternalCommandContext, name string, input json.RawMessage) (json.RawMessage, error)
}

type capturedEventPublisher struct {
	events []websocket.Event
}

type stubRuntimeAdapter struct {
	kind      string
	executeFn func(execCtx *workerpkg.ExecutionContext, run *model.AgentRun) error
}

func (p *capturedEventPublisher) Publish(event websocket.Event) {
	p.events = append(p.events, event)
}

func (s stubRuntimeAdapter) Kind() string {
	return s.kind
}

func (s stubRuntimeAdapter) Execute(execCtx *workerpkg.ExecutionContext, run *model.AgentRun) error {
	if s.executeFn != nil {
		return s.executeFn(execCtx, run)
	}
	return nil
}

func (s stubInternalCommandExecutor) Execute(ctx context.Context, meta model.InternalCommandContext, name string, input json.RawMessage) (json.RawMessage, error) {
	if s.executeFn != nil {
		return s.executeFn(ctx, meta, name, input)
	}
	return json.RawMessage(`{}`), nil
}

func TestShouldPersistExecutionWorkspace(t *testing.T) {
	if !shouldPersistExecutionWorkspace(&model.AgentRun{InvocationMode: model.InvocationModeInteractive}, "codex") {
		t.Fatal("expected interactive codex runs to persist their workspace across approvals")
	}
	if shouldPersistExecutionWorkspace(&model.AgentRun{InvocationMode: model.InvocationModeAutonomous}, "codex") {
		t.Fatal("did not expect autonomous codex runs to persist their workspace")
	}
	if shouldPersistExecutionWorkspace(&model.AgentRun{InvocationMode: model.InvocationModeInteractive}, "opencode") {
		t.Fatal("did not expect non-codex interactive runs to persist their workspace")
	}
}

func TestResolveNativeSelectivePlannerPathEnabled(t *testing.T) {
	testCases := []struct {
		name  string
		env   string
		run   *model.AgentRun
		agent *model.Agent
		want  bool
	}{
		{
			name: "system epic planner on native runtime is eligible",
			env:  "true",
			run: &model.AgentRun{
				RuntimeKind: "native_sdk",
				TargetType:  "epic",
			},
			agent: &model.Agent{
				IsSystem:    true,
				PresetKey:   model.AgentPresetEpicPlanner,
				RuntimeKind: "native_sdk",
			},
			want: true,
		},
		{
			name: "system task planner on native runtime is eligible",
			env:  "true",
			run: &model.AgentRun{
				RuntimeKind: "native_sdk",
				TargetType:  "task",
			},
			agent: &model.Agent{
				IsSystem:    true,
				PresetKey:   model.AgentPresetTaskPlanner,
				RuntimeKind: "native_sdk",
			},
			want: true,
		},
		{
			name: "rollout env disabled blocks path",
			env:  "false",
			run: &model.AgentRun{
				RuntimeKind: "native_sdk",
				TargetType:  "epic",
			},
			agent: &model.Agent{
				IsSystem:    true,
				PresetKey:   model.AgentPresetEpicPlanner,
				RuntimeKind: "native_sdk",
			},
			want: false,
		},
		{
			name: "custom agents stay off selective path",
			env:  "true",
			run: &model.AgentRun{
				RuntimeKind: "native_sdk",
				TargetType:  "epic",
			},
			agent: &model.Agent{
				IsSystem:    false,
				PresetKey:   model.AgentPresetEpicPlanner,
				RuntimeKind: "native_sdk",
			},
			want: false,
		},
		{
			name: "wrong target type is rejected",
			env:  "true",
			run: &model.AgentRun{
				RuntimeKind: "native_sdk",
				TargetType:  "task",
			},
			agent: &model.Agent{
				IsSystem:    true,
				PresetKey:   model.AgentPresetEpicPlanner,
				RuntimeKind: "native_sdk",
			},
			want: false,
		},
		{
			name: "non-native runtime is rejected",
			env:  "true",
			run: &model.AgentRun{
				RuntimeKind: "codex",
				TargetType:  "epic",
			},
			agent: &model.Agent{
				IsSystem:    true,
				PresetKey:   model.AgentPresetEpicPlanner,
				RuntimeKind: "native_sdk",
			},
			want: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("AGENT_NATIVE_SELECTIVE_PLANNER_ENABLED", tc.env)
			if got := resolveNativeSelectivePlannerPathEnabled(tc.run, tc.agent); got != tc.want {
				t.Fatalf("resolveNativeSelectivePlannerPathEnabled() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestExecutionRuntimeKindPrefersRunOverride(t *testing.T) {
	state := &resolvedRunState{
		run:   &model.AgentRun{RuntimeKind: "codex"},
		agent: &model.Agent{RuntimeKind: "opencode"},
	}
	if got := executionRuntimeKind(state); got != "codex" {
		t.Fatalf("expected run runtime kind override, got %q", got)
	}

	state.run.RuntimeKind = ""
	if got := executionRuntimeKind(state); got != "opencode" {
		t.Fatalf("expected agent runtime kind fallback, got %q", got)
	}
}

func TestSelectNativeActiveSkillsRequiresSelectivePathGate(t *testing.T) {
	state := &resolvedRunState{
		run: &model.AgentRun{TargetType: "epic"},
		agent: &model.Agent{
			PresetKey: model.AgentPresetEpicPlanner,
		},
		runtimeSkillRefs: model.AgentSkillRefs{
			{Key: "approval_protocol"},
			{Key: "prd_authorship"},
			{Key: "task_decomposition"},
		},
		runtimeSkillDefinitions: []workerpkg.SkillDefinition{
			{Key: "approval_protocol", SourceKind: "built_in", Instructions: "approval"},
			{Key: "prd_authorship", SourceKind: "built_in", Instructions: "prd"},
			{Key: "task_decomposition", SourceKind: "built_in", Instructions: "tasks"},
		},
		nativeSelectivePathEnabled: false,
	}

	selection := selectNativeActiveSkills(state, model.PlanningStageDraftSpec)
	if got := testAgentSkillRefKeys(selection.Refs); len(got) != 3 || got[0] != "approval_protocol" || got[1] != "prd_authorship" || got[2] != "task_decomposition" {
		t.Fatalf("expected full skill set when selective path is disabled, got %#v", got)
	}
}

func TestSplitNativePhaseGuidanceMovesLegacyInstructionsOffInitialPrompt(t *testing.T) {
	legacyInitialInstructions, phaseGuidance := splitNativePhaseGuidance("native_sdk", &resolvedRunState{
		nativeSelectivePathEnabled: true,
	}, "Run mode: interactive\nDraft the PRD first.")

	if strings.TrimSpace(legacyInitialInstructions) != "" {
		t.Fatalf("expected selective native path to suppress legacy initial instructions, got %q", legacyInitialInstructions)
	}
	if !strings.Contains(phaseGuidance, "Draft the PRD first.") {
		t.Fatalf("expected phase guidance to carry planner instructions, got %q", phaseGuidance)
	}
}

func TestSplitNativePhaseGuidanceKeepsLegacyInstructionsForNonSelectivePath(t *testing.T) {
	legacyInitialInstructions, phaseGuidance := splitNativePhaseGuidance("native_sdk", &resolvedRunState{
		nativeSelectivePathEnabled: false,
	}, "Run mode: interactive\nDraft the PRD first.")

	if !strings.Contains(legacyInitialInstructions, "Draft the PRD first.") {
		t.Fatalf("expected legacy initial instructions to remain for ungated path, got %q", legacyInitialInstructions)
	}
	if strings.TrimSpace(phaseGuidance) != "" {
		t.Fatalf("expected no phase guidance for ungated path, got %q", phaseGuidance)
	}
}

func TestBuildInitialInstructionsUsesNativeSelectiveEpicPhaseGuidance(t *testing.T) {
	activity := &AgentRunActivities{}
	state := &resolvedRunState{
		nativeSelectivePathEnabled: true,
		run: &model.AgentRun{
			WorkspaceID:    "ws-1",
			TargetType:     "epic",
			InvocationMode: model.InvocationModeInteractive,
			Input:          json.RawMessage(`{"additional_context":"Focus on launch blockers."}`),
		},
		epic: &model.PMEpic{
			ID:          "epic-1",
			WorkspaceID: "ws-1",
			Name:        "Launch readiness",
		},
	}

	instructions, err := activity.buildInitialInstructions(context.Background(), state, planningRunInput{
		AdditionalContext: "Focus on launch blockers.",
	})
	if err != nil {
		t.Fatalf("buildInitialInstructions returned error: %v", err)
	}
	for _, snippet := range []string{
		"Current planning phase: draft_spec",
		"Phase objective: move the epic to the next durable planning checkpoint",
		"Interactive approval semantics:",
		"Next-step guidance: no approved PRD exists yet.",
		"Operator notes:\nFocus on launch blockers.",
	} {
		if !strings.Contains(instructions, snippet) {
			t.Fatalf("expected native selective epic guidance to contain %q\n%s", snippet, instructions)
		}
	}
	if strings.Contains(instructions, "Use this sequence unless the human explicitly redirects you:") {
		t.Fatalf("did not expect legacy epic planner boilerplate in native selective guidance\n%s", instructions)
	}
}

func TestBuildInitialInstructionsKeepsLegacyEpicPlannerInstructionsWhenSelectivePathDisabled(t *testing.T) {
	activity := &AgentRunActivities{}
	state := &resolvedRunState{
		nativeSelectivePathEnabled: false,
		run: &model.AgentRun{
			WorkspaceID:    "ws-1",
			TargetType:     "epic",
			InvocationMode: model.InvocationModeInteractive,
			Input:          json.RawMessage(`{"additional_context":"Focus on launch blockers."}`),
		},
		epic: &model.PMEpic{
			ID:          "epic-1",
			WorkspaceID: "ws-1",
			Name:        "Launch readiness",
		},
	}

	instructions, err := activity.buildInitialInstructions(context.Background(), state, planningRunInput{
		AdditionalContext: "Focus on launch blockers.",
		AllowedTools:      []string{workerpkg.ToolPublishPRDDraft, workerpkg.ToolPublishTaskPlan},
	})
	if err != nil {
		t.Fatalf("buildInitialInstructions returned error: %v", err)
	}
	if !strings.Contains(instructions, "Use this sequence unless the human explicitly redirects you:") {
		t.Fatalf("expected legacy epic planner wording to remain when selective path is disabled\n%s", instructions)
	}
	if strings.Contains(instructions, "Current planning phase: draft_spec") {
		t.Fatalf("did not expect native selective phase header in legacy instructions\n%s", instructions)
	}
}

func TestBuildInitialInstructionsUsesNativeSelectiveTaskPhaseGuidance(t *testing.T) {
	activity := &AgentRunActivities{}
	state := &resolvedRunState{
		nativeSelectivePathEnabled: true,
		run: &model.AgentRun{
			WorkspaceID:    "ws-1",
			TargetType:     "task",
			InvocationMode: model.InvocationModeInteractive,
			Input:          json.RawMessage(`{"additional_context":"Focus on regression risk."}`),
		},
		task: &model.PMTask{
			ID:          "task-1",
			WorkspaceID: "ws-1",
			Name:        "Harden approval preview binding",
		},
	}

	instructions, err := activity.buildInitialInstructions(context.Background(), state, planningRunInput{
		AdditionalContext: "Focus on regression risk.",
	})
	if err != nil {
		t.Fatalf("buildInitialInstructions returned error: %v", err)
	}
	for _, snippet := range []string{
		"Current planning phase: task_plan_doc",
		"Phase objective: refine a task-scoped implementation planning document",
		"Approval rule: use request_approval with phase=\"task_doc\"",
		"Operator notes:\nFocus on regression risk.",
		"Task: Harden approval preview binding",
	} {
		if !strings.Contains(instructions, snippet) {
			t.Fatalf("expected native selective task guidance to contain %q\n%s", snippet, instructions)
		}
	}
	if strings.Contains(instructions, "Use this sequence unless the human explicitly redirects you:") {
		t.Fatalf("did not expect legacy task planner boilerplate in native selective guidance\n%s", instructions)
	}
}

func TestNativeEpicPlannerPhaseName(t *testing.T) {
	testCases := []struct {
		name           string
		input          planningRunInput
		hasSpecContent bool
		hasTasks       bool
		want           string
	}{
		{
			name: "approved spec with no tasks moves to task planning",
			input: planningRunInput{
				SpecVersionID: "spec-v1",
			},
			want: model.PlanningStagePlanTasks,
		},
		{
			name: "approved spec with tasks enters extension mode",
			input: planningRunInput{
				SpecVersionID: "spec-v1",
			},
			hasTasks: true,
			want:     "task_extension",
		},
		{
			name: "draft spec doc stays in prd revision",
			input: planningRunInput{
				SpecDocumentID: "doc-1",
			},
			hasSpecContent: true,
			want:           "prd_revision",
		},
		{
			name:  "no prior spec starts in draft spec",
			input: planningRunInput{},
			want:  model.PlanningStageDraftSpec,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if got := nativeEpicPlannerPhaseName(tc.input, tc.hasSpecContent, tc.hasTasks); got != tc.want {
				t.Fatalf("expected phase %q, got %q", tc.want, got)
			}
		})
	}
}

func TestNativeEpicPlannerNextStepGuidance(t *testing.T) {
	testCases := []struct {
		name           string
		input          planningRunInput
		hasSpecContent bool
		hasTasks       bool
		wantSnippet    string
	}{
		{
			name: "approved spec with no tasks skips prd drafting",
			input: planningRunInput{
				SpecVersionID: "spec-v1",
			},
			wantSnippet: "Skip PRD drafting entirely and proceed directly to task planning",
		},
		{
			name: "approved spec with tasks avoids recreation",
			input: planningRunInput{
				SpecVersionID: "spec-v1",
			},
			hasTasks:    true,
			wantSnippet: "Do not redraft the PRD or recreate existing tasks",
		},
		{
			name: "draft spec resumes current draft",
			input: planningRunInput{
				SpecDocumentID: "doc-1",
			},
			hasSpecContent: true,
			wantSnippet:    "Resume from the current draft",
		},
		{
			name:        "no prior spec follows full loop",
			input:       planningRunInput{},
			wantSnippet: "Follow the full loop from clarification through PRD drafting",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got := nativeEpicPlannerNextStepGuidance(tc.input, tc.hasSpecContent, tc.hasTasks)
			if !strings.Contains(got, tc.wantSnippet) {
				t.Fatalf("expected guidance to contain %q, got %q", tc.wantSnippet, got)
			}
		})
	}
}

func TestLatestNativeRepairInstructionRequiresSelectivePath(t *testing.T) {
	messages := []model.AgentRunMessage{
		{Role: "user", MessageType: "policy_retry", Content: "System correction: continue from your last assistant message."},
	}

	got := latestNativeRepairInstruction(&resolvedRunState{nativeSelectivePathEnabled: false}, messages)
	if got != (nativeRepairInstruction{}) {
		t.Fatalf("expected no repair instruction without selective path, got %#v", got)
	}
}

func TestLatestNativeRepairInstructionIgnoresResolvedRetry(t *testing.T) {
	messages := []model.AgentRunMessage{
		{Role: "user", MessageType: "policy_retry", Content: "System correction: continue from your last assistant message."},
		{Role: "assistant", MessageType: "assistant_turn", Content: "Retrying with the correct handoff."},
	}

	got := latestNativeRepairInstruction(&resolvedRunState{nativeSelectivePathEnabled: true}, messages)
	if got != (nativeRepairInstruction{}) {
		t.Fatalf("expected no repair instruction after a later assistant turn, got %#v", got)
	}
}

func TestClassifyNativeRepairInstructionForApprovalPreviewBinding(t *testing.T) {
	state := &resolvedRunState{nativeSelectivePathEnabled: true}
	message := &model.AgentRunMessage{
		Role:        "user",
		MessageType: "policy_retry",
		Content:     "System correction: the previous turn requested approval without binding it to a same-turn preview. Include preview_panel_key when needed.",
	}

	got := classifyNativeRepairInstruction(state, message)
	if got.Class != "approval_preview_binding" {
		t.Fatalf("expected approval preview binding class, got %#v", got)
	}
	for _, snippet := range []string{"same turn", "preview_panel_key", "final action"} {
		if !strings.Contains(strings.ToLower(got.Instructions), snippet) {
			t.Fatalf("expected approval preview repair guidance to contain %q, got %q", snippet, got.Instructions)
		}
	}
}

func TestClassifyNativeRepairInstructionForApprovalPreviewPanelKeyRequired(t *testing.T) {
	state := &resolvedRunState{nativeSelectivePathEnabled: true}
	message := &model.AgentRunMessage{
		Role:        "user",
		MessageType: "policy_retry",
		Content:     "System correction: the previous turn requested approval after publishing multiple same-turn previews but did not include preview_panel_key.",
	}

	got := classifyNativeRepairInstruction(state, message)
	if got.Class != "approval_preview_panel_key_required" {
		t.Fatalf("expected preview_panel_key-specific class, got %#v", got)
	}
	for _, snippet := range []string{"multiple previews", "preview_panel_key", "final action"} {
		if !strings.Contains(strings.ToLower(got.Instructions), snippet) {
			t.Fatalf("expected preview_panel_key repair guidance to contain %q, got %q", snippet, got.Instructions)
		}
	}
}

func TestClassifyNativeRepairInstructionForApprovalSpecificPreviewRequired(t *testing.T) {
	state := &resolvedRunState{nativeSelectivePathEnabled: true}
	message := &model.AgentRunMessage{
		Role:        "user",
		MessageType: "policy_retry",
		Content:     `System correction: the previous turn requested approval without binding it to the required same-turn prd_draft preview. Set preview_panel_key="prd_draft" on the approval handoff.`,
	}

	got := classifyNativeRepairInstruction(state, message)
	if got.Class != "approval_specific_preview_required" {
		t.Fatalf("expected specific-preview class, got %#v", got)
	}
	for _, snippet := range []string{"prd_draft", "preview_panel_key", "final action"} {
		if !strings.Contains(strings.ToLower(got.Instructions), snippet) {
			t.Fatalf("expected specific-preview repair guidance to contain %q, got %q", snippet, got.Instructions)
		}
	}
}

func TestClassifyNativeRepairInstructionForReviewCheckpointHandoff(t *testing.T) {
	state := &resolvedRunState{nativeSelectivePathEnabled: true}
	message := &model.AgentRunMessage{
		Role:        "user",
		MessageType: "policy_retry",
		Content:     "System correction: emit a review_checkpoint handoff using the runtime-appropriate mechanism.",
	}

	got := classifyNativeRepairInstruction(state, message)
	if got.Class != "review_checkpoint_handoff" {
		t.Fatalf("expected review checkpoint class, got %#v", got)
	}
	if !strings.Contains(got.Instructions, "review_checkpoint handoff") {
		t.Fatalf("expected review checkpoint repair guidance, got %q", got.Instructions)
	}
}

func TestClassifyNativeRepairInstructionForRequiredInteractionHandoff(t *testing.T) {
	state := &resolvedRunState{
		nativeSelectivePathEnabled: true,
		skillPolicy: workerpkg.SkillPolicy{
			CompletionRequiresInteractionKinds: []string{
				model.AgentRunInteractionKindApprovalRequest,
				model.AgentRunInteractionKindRequestUserInput,
			},
		},
	}
	message := &model.AgentRunMessage{
		Role:        "user",
		MessageType: "policy_retry",
		Content:     "System correction: the previous turn ended without creating the required interaction.",
	}

	got := classifyNativeRepairInstruction(state, message)
	if got.Class != "required_interaction_handoff" {
		t.Fatalf("expected required interaction class, got %#v", got)
	}
	if !strings.Contains(got.Instructions, model.AgentRunInteractionKindApprovalRequest) || !strings.Contains(got.Instructions, model.AgentRunInteractionKindRequestUserInput) {
		t.Fatalf("expected required interaction repair guidance to list active policy kinds, got %q", got.Instructions)
	}
}

func TestLatestNativeRepairInstructionFallsBackToGenericRequiredHandoff(t *testing.T) {
	state := &resolvedRunState{nativeSelectivePathEnabled: true}
	messages := []model.AgentRunMessage{
		{
			Role:        "user",
			MessageType: "policy_retry",
			Content:     "System correction: use the exact contract from the last error.",
		},
	}

	got := latestNativeRepairInstruction(state, messages)
	if got.Class != "required_interaction_handoff" {
		t.Fatalf("expected generic fallback class, got %#v", got)
	}
	if !strings.Contains(got.Instructions, "Continue from your last assistant turn") {
		t.Fatalf("expected fallback repair guidance to remain usable, got %q", got.Instructions)
	}
}

func TestApprovalPreviewRetryInstructionForSpecificPreview(t *testing.T) {
	got := approvalPreviewRetryInstruction("approval_request requires a same-turn prd_draft preview before requesting approval")
	if !strings.Contains(got, "required same-turn prd_draft preview") {
		t.Fatalf("expected retry instruction to preserve required preview key, got %q", got)
	}
	if !strings.Contains(got, `preview_panel_key="prd_draft"`) {
		t.Fatalf("expected retry instruction to preserve preview_panel_key binding, got %q", got)
	}
}

func TestApprovalPreviewRetryInstructionForMultiplePreviews(t *testing.T) {
	got := approvalPreviewRetryInstruction("approval_request requires preview_panel_key when multiple same-turn previews exist")
	if !strings.Contains(got, "multiple same-turn previews") {
		t.Fatalf("expected retry instruction to mention multiple previews, got %q", got)
	}
	if !strings.Contains(got, "preview_panel_key") {
		t.Fatalf("expected retry instruction to mention preview_panel_key, got %q", got)
	}
}

func TestNormalizedCompletionRetryInstructionForSpecificPreview(t *testing.T) {
	state := &resolvedRunState{}
	got := normalizedCompletionRetryInstruction(state, fmt.Errorf("approval_request requires a same-turn prd_draft preview before requesting approval"))
	if got.Class != "approval_specific_preview_required" {
		t.Fatalf("expected specific-preview class, got %#v", got)
	}
	if !strings.Contains(got.Instructions, `preview_panel_key="prd_draft"`) {
		t.Fatalf("expected specific-preview retry instruction, got %q", got.Instructions)
	}
}

func TestNormalizedCompletionRetryInstructionForMultiplePreviews(t *testing.T) {
	state := &resolvedRunState{}
	got := normalizedCompletionRetryInstruction(state, fmt.Errorf("approval_request requires preview_panel_key when multiple same-turn previews exist"))
	if got.Class != "approval_preview_panel_key_required" {
		t.Fatalf("expected preview-panel-key class, got %#v", got)
	}
	if !strings.Contains(got.Instructions, "multiple same-turn previews") {
		t.Fatalf("expected multiple-preview retry instruction, got %q", got.Instructions)
	}
}

func TestNormalizedCompletionRetryInstructionForRequiredInteractionFallback(t *testing.T) {
	state := &resolvedRunState{
		skillPolicy: workerpkg.SkillPolicy{
			CompletionRequiresInteractionKinds: []string{
				model.AgentRunInteractionKindApprovalRequest,
				model.AgentRunInteractionKindRequestUserInput,
			},
		},
	}
	got := normalizedCompletionRetryInstruction(state, fmt.Errorf("completion interaction missing"))
	if got.Class != "required_interaction_handoff" {
		t.Fatalf("expected required-interaction class, got %#v", got)
	}
	if !strings.Contains(got.Instructions, model.AgentRunInteractionKindApprovalRequest) || !strings.Contains(got.Instructions, model.AgentRunInteractionKindRequestUserInput) {
		t.Fatalf("expected required interaction kinds in retry instruction, got %q", got.Instructions)
	}
}

func TestNormalizedCompletionRetryInstructionForReviewAgent(t *testing.T) {
	state := &resolvedRunState{agent: &model.Agent{PresetKey: model.AgentPresetReviewAgent}}
	got := normalizedCompletionRetryInstruction(state, fmt.Errorf("completion interaction missing"))
	if got.Class != "review_checkpoint_handoff" {
		t.Fatalf("expected review checkpoint class, got %#v", got)
	}
	if !strings.Contains(got.Instructions, "review_checkpoint handoff") {
		t.Fatalf("expected review retry instruction, got %q", got.Instructions)
	}
}

func TestNormalizedCompletionRetryInstructionHandlesNilState(t *testing.T) {
	got := normalizedCompletionRetryInstruction(nil, fmt.Errorf("completion interaction missing"))
	if got.Class != "required_interaction_handoff" {
		t.Fatalf("expected generic required-interaction class, got %#v", got)
	}
	if !strings.Contains(got.Instructions, "required interaction handoffs") {
		t.Fatalf("expected generic retry instruction, got %q", got.Instructions)
	}
}

func TestLatestNativeRepairInstructionFromArtifactsUsesLatestAssistantSequence(t *testing.T) {
	messages := []model.AgentRunMessage{
		{SequenceNo: 1, Role: "user", MessageType: "prompt", Content: "Start"},
		{SequenceNo: 2, Role: "assistant", MessageType: "assistant_turn", Content: "Drafted the task plan."},
	}
	payload, _ := json.Marshal(model.NativeRepairState{
		Source:      "tool_failure",
		RepairClass: "publish_task_plan_object_shape",
		RepairHint:  "Retry publish_task_plan with one complete JSON object in content.",
	})
	artifacts := []model.AgentRunArtifact{
		{
			ArtifactType:  model.AgentRunArtifactTypeNativeRepairState,
			InlineContent: strPtr(string(payload)),
			Metadata:      buildAssistantSequenceArtifactMetadata(2),
			SequenceNo:    1,
		},
	}

	got := latestNativeRepairInstructionFromArtifacts(messages, artifacts)
	if got.Class != "publish_task_plan_object_shape" {
		t.Fatalf("expected repair artifact class, got %#v", got)
	}
	if !strings.Contains(got.Instructions, "complete JSON object") {
		t.Fatalf("expected repair hint from artifact, got %q", got.Instructions)
	}
}

func TestLatestNativeRepairInstructionFromArtifactsIgnoresStaleAssistantArtifact(t *testing.T) {
	messages := []model.AgentRunMessage{
		{SequenceNo: 1, Role: "user", MessageType: "prompt", Content: "Start"},
		{SequenceNo: 2, Role: "assistant", MessageType: "assistant_turn", Content: "Old failing turn."},
		{SequenceNo: 3, Role: "user", MessageType: "prompt", Content: "Continue"},
		{SequenceNo: 4, Role: "assistant", MessageType: "assistant_turn", Content: "Newer turn."},
	}
	payload, _ := json.Marshal(model.NativeRepairState{
		Source:      "tool_failure",
		RepairClass: "publish_task_plan_object_shape",
		RepairHint:  "Retry publish_task_plan with one complete JSON object in content.",
	})
	artifacts := []model.AgentRunArtifact{
		{
			ArtifactType:  model.AgentRunArtifactTypeNativeRepairState,
			InlineContent: strPtr(string(payload)),
			Metadata:      buildAssistantSequenceArtifactMetadata(2),
			SequenceNo:    1,
		},
	}

	got := latestNativeRepairInstructionFromArtifacts(messages, artifacts)
	if got != (nativeRepairInstruction{}) {
		t.Fatalf("expected stale repair artifact to be ignored, got %#v", got)
	}
}

func TestResolveLatestNativeRepairInstructionPrefersArtifactOverHistory(t *testing.T) {
	state := &resolvedRunState{nativeSelectivePathEnabled: true}
	blocks, _ := json.Marshal([]workerpkg.ExecutionBlock{
		{
			Type:     workerpkg.ExecutionBlockTypeToolResult,
			ToolName: workerpkg.ToolPublishTaskPlan,
			Output:   "publish_task_plan requires content.proposed_tasks to be an array of task objects",
			IsError:  true,
		},
	})
	messages := []model.AgentRunMessage{
		{SequenceNo: 1, Role: "user", MessageType: "prompt", Content: "Start"},
		{SequenceNo: 2, Role: "assistant", MessageType: "assistant_turn", Content: "Drafted the task plan."},
		{SequenceNo: 3, Role: "tool", MessageType: "tool_result", Content: "publish_task_plan requires content.proposed_tasks to be an array of task objects", ContentBlocks: blocks},
	}
	payload, _ := json.Marshal(model.NativeRepairState{
		Source:      "tool_failure",
		RepairClass: "publish_task_plan_object_shape",
		RepairHint:  "Retry publish_task_plan with one complete JSON object in content.",
	})
	artifacts := []model.AgentRunArtifact{
		{
			ArtifactType:  model.AgentRunArtifactTypeNativeRepairState,
			InlineContent: strPtr(string(payload)),
			Metadata:      buildAssistantSequenceArtifactMetadata(2),
			SequenceNo:    1,
		},
	}

	got := resolveLatestNativeRepairInstruction(state, messages, artifacts)
	if got.Class != "publish_task_plan_object_shape" {
		t.Fatalf("expected artifact-backed repair class to win, got %#v", got)
	}
}

func TestResolveLatestNativeRepairInstructionFallsBackToHistory(t *testing.T) {
	state := &resolvedRunState{nativeSelectivePathEnabled: true}
	blocks, _ := json.Marshal([]workerpkg.ExecutionBlock{
		{
			Type:     workerpkg.ExecutionBlockTypeToolResult,
			ToolName: workerpkg.ToolPublishTaskPlan,
			Output:   "publish_task_plan requires content.proposed_tasks to be an array of task objects",
			IsError:  true,
		},
	})
	messages := []model.AgentRunMessage{
		{SequenceNo: 1, Role: "user", MessageType: "prompt", Content: "Start"},
		{SequenceNo: 2, Role: "assistant", MessageType: "assistant_turn", Content: "Drafted the task plan."},
		{SequenceNo: 3, Role: "tool", MessageType: "tool_result", Content: "publish_task_plan requires content.proposed_tasks to be an array of task objects", ContentBlocks: blocks},
	}
	artifacts := []model.AgentRunArtifact{
		{
			ArtifactType:  model.AgentRunArtifactTypeNativeRepairState,
			InlineContent: strPtr(`{"source":"tool_failure","repair_class":"publish_task_plan_object_shape"}`),
			Metadata:      buildAssistantSequenceArtifactMetadata(2),
			SequenceNo:    1,
		},
	}

	got := resolveLatestNativeRepairInstruction(state, messages, artifacts)
	if got.Class != "publish_task_plan_task_array_shape" {
		t.Fatalf("expected history fallback repair class, got %#v", got)
	}
}

func TestClassifyPublishTaskPlanRepair(t *testing.T) {
	testCases := []struct {
		name        string
		output      string
		wantClass   string
		wantSnippet string
	}{
		{
			name:        "object shape",
			output:      "publish_task_plan content must be a JSON object with summary and proposed_tasks",
			wantClass:   "publish_task_plan_object_shape",
			wantSnippet: "stringified JSON blobs",
		},
		{
			name:        "task array shape",
			output:      "publish_task_plan requires content.proposed_tasks to be an array of task objects",
			wantClass:   "publish_task_plan_task_array_shape",
			wantSnippet: "array of full task objects",
		},
		{
			name:        "missing content",
			output:      `publish_task_plan is missing content; include the task plan JSON object in "content"`,
			wantClass:   "publish_task_plan_missing_content",
			wantSnippet: `full task-plan JSON object under "content"`,
		},
		{
			name:        "raw wrapper",
			output:      "publish_task_plan input must be a JSON object with structured fields; do not send a raw string wrapper",
			wantClass:   "publish_task_plan_raw_wrapper",
			wantSnippet: "not a raw wrapper string",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got := classifyPublishTaskPlanRepair(tc.output)
			if got.Class != tc.wantClass {
				t.Fatalf("expected class %q, got %#v", tc.wantClass, got)
			}
			if !strings.Contains(got.Instructions, tc.wantSnippet) {
				t.Fatalf("expected instructions to contain %q, got %q", tc.wantSnippet, got.Instructions)
			}
		})
	}
}

func TestClassifyMarkdownPreviewRepair(t *testing.T) {
	testCases := []struct {
		name        string
		toolName    string
		output      string
		wantClass   string
		wantSnippet string
	}{
		{
			name:        "task plan doc missing content",
			toolName:    workerpkg.ToolPublishTaskPlanDoc,
			output:      `publish_task_plan_doc is missing content; include markdown in "content"`,
			wantClass:   "publish_task_plan_doc_missing_content",
			wantSnippet: "title-only payloads",
		},
		{
			name:        "task plan doc markdown type",
			toolName:    workerpkg.ToolPublishTaskPlanDoc,
			output:      `publish_task_plan_doc content must be a markdown string in "content"`,
			wantClass:   "publish_task_plan_doc_markdown_type",
			wantSnippet: "plain markdown string",
		},
		{
			name:        "prd draft missing content",
			toolName:    workerpkg.ToolPublishPRDDraft,
			output:      `publish_prd_draft is missing content; include markdown in "content"`,
			wantClass:   "publish_prd_draft_missing_content",
			wantSnippet: "full PRD markdown draft",
		},
		{
			name:        "prd draft markdown type",
			toolName:    workerpkg.ToolPublishPRDDraft,
			output:      `publish_prd_draft content must be a markdown string in "content"`,
			wantClass:   "publish_prd_draft_markdown_type",
			wantSnippet: "non-string content",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got := classifyMarkdownPreviewRepair(tc.toolName, tc.output)
			if got.Class != tc.wantClass {
				t.Fatalf("expected class %q, got %#v", tc.wantClass, got)
			}
			if !strings.Contains(got.Instructions, tc.wantSnippet) {
				t.Fatalf("expected instructions to contain %q, got %q", tc.wantSnippet, got.Instructions)
			}
		})
	}
}

func TestLatestNativeRepairInstructionPrefersPublishTaskPlanToolFailure(t *testing.T) {
	state := &resolvedRunState{
		nativeSelectivePathEnabled: true,
	}
	blocks, err := json.Marshal([]workerpkg.ExecutionBlock{
		{
			Type:     workerpkg.ExecutionBlockTypeToolResult,
			ToolName: workerpkg.ToolPublishTaskPlan,
			Output:   "publish_task_plan content must be a JSON object with summary and proposed_tasks",
			IsError:  true,
		},
	})
	if err != nil {
		t.Fatalf("marshal blocks: %v", err)
	}
	messages := []model.AgentRunMessage{
		{Role: "assistant", MessageType: "assistant_turn", Content: "Trying to publish the plan."},
		{Role: "tool", MessageType: "tool_result", Content: "publish_task_plan content must be a JSON object with summary and proposed_tasks", ContentBlocks: blocks},
		{Role: "user", MessageType: "policy_retry", Content: "System correction: continue from your last assistant turn."},
	}

	got := latestNativeRepairInstruction(state, messages)
	if got.Class != "publish_task_plan_object_shape" {
		t.Fatalf("expected publish_task_plan repair class, got %#v", got)
	}
	if !strings.Contains(got.Instructions, `"summary"`) || !strings.Contains(got.Instructions, `"proposed_tasks"`) {
		t.Fatalf("expected publish_task_plan repair guidance, got %q", got.Instructions)
	}
}

func TestLatestNativeRepairInstructionPrefersPublishTaskPlanDocToolFailure(t *testing.T) {
	state := &resolvedRunState{
		nativeSelectivePathEnabled: true,
	}
	blocks, err := json.Marshal([]workerpkg.ExecutionBlock{
		{
			Type:     workerpkg.ExecutionBlockTypeToolResult,
			ToolName: workerpkg.ToolPublishTaskPlanDoc,
			Output:   `publish_task_plan_doc is missing content; include markdown in "content"`,
			IsError:  true,
		},
	})
	if err != nil {
		t.Fatalf("marshal blocks: %v", err)
	}
	messages := []model.AgentRunMessage{
		{Role: "assistant", MessageType: "assistant_turn", Content: "Trying to publish the planning doc."},
		{Role: "tool", MessageType: "tool_result", Content: `publish_task_plan_doc is missing content; include markdown in "content"`, ContentBlocks: blocks},
		{Role: "user", MessageType: "policy_retry", Content: "System correction: continue from your last assistant turn."},
	}

	got := latestNativeRepairInstruction(state, messages)
	if got.Class != "publish_task_plan_doc_missing_content" {
		t.Fatalf("expected publish_task_plan_doc repair class, got %#v", got)
	}
	if !strings.Contains(got.Instructions, "full task planning markdown draft") {
		t.Fatalf("expected publish_task_plan_doc repair guidance, got %q", got.Instructions)
	}
}

func TestLatestNativeRepairInstructionPrefersPublishPRDDraftToolFailure(t *testing.T) {
	state := &resolvedRunState{
		nativeSelectivePathEnabled: true,
	}
	blocks, err := json.Marshal([]workerpkg.ExecutionBlock{
		{
			Type:     workerpkg.ExecutionBlockTypeToolResult,
			ToolName: workerpkg.ToolPublishPRDDraft,
			Output:   `publish_prd_draft content must be a markdown string in "content"`,
			IsError:  true,
		},
	})
	if err != nil {
		t.Fatalf("marshal blocks: %v", err)
	}
	messages := []model.AgentRunMessage{
		{Role: "assistant", MessageType: "assistant_turn", Content: "Trying to publish the PRD draft."},
		{Role: "tool", MessageType: "tool_result", Content: `publish_prd_draft content must be a markdown string in "content"`, ContentBlocks: blocks},
		{Role: "user", MessageType: "policy_retry", Content: "System correction: continue from your last assistant turn."},
	}

	got := latestNativeRepairInstruction(state, messages)
	if got.Class != "publish_prd_draft_markdown_type" {
		t.Fatalf("expected publish_prd_draft repair class, got %#v", got)
	}
	if !strings.Contains(got.Instructions, "full PRD markdown draft") {
		t.Fatalf("expected publish_prd_draft repair guidance, got %q", got.Instructions)
	}
}

func TestLatestUnresolvedNativeToolFailureIgnoresOlderToolFailureAfterLaterAssistant(t *testing.T) {
	oldBlocks, err := json.Marshal([]workerpkg.ExecutionBlock{
		{
			Type:     workerpkg.ExecutionBlockTypeToolResult,
			ToolName: workerpkg.ToolPublishTaskPlan,
			Output:   "publish_task_plan content must be a JSON object with summary and proposed_tasks",
			IsError:  true,
		},
	})
	if err != nil {
		t.Fatalf("marshal old blocks: %v", err)
	}
	newBlocks, err := json.Marshal([]workerpkg.ExecutionBlock{
		{
			Type:     workerpkg.ExecutionBlockTypeToolResult,
			ToolName: workerpkg.ToolPublishTaskPlan,
			Output:   "publish_task_plan requires content.proposed_tasks to be an array of task objects",
			IsError:  true,
		},
	})
	if err != nil {
		t.Fatalf("marshal new blocks: %v", err)
	}
	messages := []model.AgentRunMessage{
		{Role: "assistant", MessageType: "assistant_turn", Content: "Earlier publish attempt."},
		{Role: "tool", MessageType: "tool_result", Content: "publish_task_plan content must be a JSON object with summary and proposed_tasks", ContentBlocks: oldBlocks},
		{Role: "assistant", MessageType: "assistant_turn", Content: "Retried with a better payload."},
		{Role: "tool", MessageType: "tool_result", Content: "publish_task_plan requires content.proposed_tasks to be an array of task objects", ContentBlocks: newBlocks},
	}

	got := latestUnresolvedNativeToolFailure(messages)
	if got == nil {
		t.Fatal("expected latest unresolved tool failure")
	}
	if got.Output != "publish_task_plan requires content.proposed_tasks to be an array of task objects" {
		t.Fatalf("expected latest tool failure output, got %#v", got)
	}
}

func TestReplayMessagesForExecutionStripsPolicyRetryForSelectivePath(t *testing.T) {
	messages := []model.AgentRunMessage{
		{Role: "user", MessageType: "prompt", Content: "Initial request"},
		{Role: "assistant", MessageType: "assistant_turn", Content: "Assistant reply"},
		{Role: "user", MessageType: "policy_retry", Content: "System correction"},
	}

	filtered := replayMessagesForExecution(&resolvedRunState{nativeSelectivePathEnabled: true}, messages)
	if len(filtered) != 2 {
		t.Fatalf("expected policy_retry to be stripped from selective replay, got %#v", filtered)
	}
	for _, message := range filtered {
		if strings.TrimSpace(message.MessageType) == "policy_retry" {
			t.Fatalf("expected selective replay to remove policy_retry messages, got %#v", filtered)
		}
	}
}

func TestEffectiveExecutionSkillPolicyUsesActiveSelectionForSelectivePath(t *testing.T) {
	state := &resolvedRunState{
		nativeSelectivePathEnabled: true,
		skillPolicy: workerpkg.SkillPolicy{
			CompletionRequiresInteractionKinds: []string{
				model.AgentRunInteractionKindApprovalRequest,
				model.AgentRunInteractionKindReviewCheckpoint,
			},
		},
		runtimeSkillDefinitions: []workerpkg.SkillDefinition{
			{
				Key:        "approval_protocol",
				SourceKind: "built_in",
				Policy: workerpkg.SkillPolicy{
					CompletionRequiresInteractionKinds: []string{model.AgentRunInteractionKindApprovalRequest},
				},
			},
			{
				Key:        "task_decomposition",
				SourceKind: "built_in",
				Policy: workerpkg.SkillPolicy{
					CompletionRequiresInteractionKinds: []string{model.AgentRunInteractionKindReviewCheckpoint},
				},
			},
		},
	}

	policy := effectiveExecutionSkillPolicy(state, agentskills.NativeActiveSelection{
		Definitions: []workerpkg.SkillDefinition{
			{
				Key:        "approval_protocol",
				SourceKind: "built_in",
				Policy: workerpkg.SkillPolicy{
					CompletionRequiresInteractionKinds: []string{model.AgentRunInteractionKindApprovalRequest},
				},
			},
		},
	})

	if got := completionRequiredInteractionKinds(policy); len(got) != 1 {
		t.Fatalf("expected one active required interaction, got %#v", got)
	} else if _, ok := got[model.AgentRunInteractionKindApprovalRequest]; !ok {
		t.Fatalf("expected active policy to replace full policy for selective path, got %#v", got)
	}
}

func TestEffectiveExecutionSkillPolicyKeepsFullPolicyWhenSelectivePathDisabled(t *testing.T) {
	state := &resolvedRunState{
		nativeSelectivePathEnabled: false,
		skillPolicy: workerpkg.SkillPolicy{
			CompletionRequiresInteractionKinds: []string{
				model.AgentRunInteractionKindApprovalRequest,
				model.AgentRunInteractionKindReviewCheckpoint,
			},
		},
	}

	policy := effectiveExecutionSkillPolicy(state, agentskills.NativeActiveSelection{
		Definitions: []workerpkg.SkillDefinition{
			{
				Key:        "approval_protocol",
				SourceKind: "built_in",
				Policy: workerpkg.SkillPolicy{
					CompletionRequiresInteractionKinds: []string{model.AgentRunInteractionKindApprovalRequest},
				},
			},
		},
	})

	if got := completionRequiredInteractionKinds(policy); len(got) != 2 {
		t.Fatalf("expected full policy to remain active when selective path is disabled, got %#v", got)
	} else if _, ok := got[model.AgentRunInteractionKindApprovalRequest]; !ok {
		t.Fatalf("expected approval_request in full policy, got %#v", got)
	} else if _, ok := got[model.AgentRunInteractionKindReviewCheckpoint]; !ok {
		t.Fatalf("expected review_checkpoint in full policy, got %#v", got)
	}
}

func TestPersistHumanInteractionArtifactsStoresStructuredReviewFindings(t *testing.T) {
	dbName := fmt.Sprintf("file:review-findings-artifact-%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dbName), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	for _, stmt := range []string{
		`CREATE TABLE agent_run_artifacts (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			run_id TEXT NOT NULL,
			artifact_type TEXT NOT NULL,
			format TEXT NOT NULL,
			storage_mode TEXT NOT NULL,
			inline_content TEXT,
			object_key TEXT,
			metadata TEXT NOT NULL,
			sequence_no INTEGER NOT NULL,
			created_at DATETIME
		)`,
		`CREATE TABLE agent_run_interactions (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			run_id TEXT NOT NULL,
			runtime_kind TEXT NOT NULL,
			interaction_kind TEXT NOT NULL,
			status TEXT NOT NULL,
			request_schema_version TEXT NOT NULL,
			response_schema_version TEXT,
			request_id TEXT,
			thread_id TEXT,
			turn_id TEXT,
			item_id TEXT,
			approval_id TEXT,
			assistant_message_sequence_no INTEGER,
			title TEXT,
			summary TEXT,
			request_payload TEXT NOT NULL,
			response_payload TEXT,
			runtime_metadata TEXT NOT NULL,
			resolved_by TEXT,
			resolved_at DATETIME,
			created_at DATETIME,
			updated_at DATETIME
		)`,
	} {
		if err := db.Exec(stmt).Error; err != nil {
			t.Fatalf("create test table: %v", err)
		}
	}

	artifactRepo := repository.NewAgentRunArtifactRepository(db)
	interactionRepo := repository.NewAgentRunInteractionRepository(db)
	activities := &AgentRunActivities{artifactRepo: artifactRepo, interactionRepo: interactionRepo}

	run := &model.AgentRun{ID: "run-1", WorkspaceID: "ws-1", RuntimeKind: "native_sdk"}
	state := &resolvedRunState{
		run:   run,
		agent: &model.Agent{ID: "agent-1", WorkspaceID: "ws-1", RuntimeKind: "native_sdk"},
	}
	assistantMessage := &model.AgentRunMessage{SequenceNo: 7}

	result := &workerpkg.ExecutionResult{
		ToolInvocations: []model.ToolInvocation{{
			ToolName: workerpkg.ToolRequestReviewCheckpoint,
			Input: json.RawMessage(`{
				"phase":"review_findings",
				"title":"Lens review findings",
				"summary":"Two actionable regressions found.",
				"findings":[
					{
						"title":"Filter state is lost on refresh",
						"body":"The query builder selection is not restored from the URL state.",
						"priority":"p1",
						"confidence":0.93,
						"code_location":"frontend/src/pages/tasks.tsx:114"
					}
				],
				"overall_correctness":"incorrect",
				"overall_explanation":"The task behavior regresses saved filter restoration.",
				"overall_confidence_score":0.91
			}`),
		}},
	}

	if err := activities.persistHumanInteractionArtifacts(context.Background(), state, result, assistantMessage); err != nil {
		t.Fatalf("persistHumanInteractionArtifacts returned error: %v", err)
	}

	artifacts, err := artifactRepo.ListByRun(context.Background(), run.WorkspaceID, run.ID)
	if err != nil {
		t.Fatalf("list artifacts: %v", err)
	}
	if len(artifacts) != 2 {
		t.Fatalf("expected approval and review findings artifacts, got %#v", artifacts)
	}
	types := make(map[string]model.AgentRunArtifact, len(artifacts))
	for _, artifact := range artifacts {
		types[artifact.ArtifactType] = artifact
	}
	findingsArtifact, ok := types[model.AgentRunArtifactTypeReviewFindings]
	if !ok || findingsArtifact.InlineContent == nil {
		t.Fatalf("expected structured review findings artifact, got %#v", artifacts)
	}
	var findings model.ReviewFindingsArtifact
	if err := json.Unmarshal([]byte(*findingsArtifact.InlineContent), &findings); err != nil {
		t.Fatalf("unmarshal review findings artifact: %v", err)
	}
	if findings.OverallCorrectness != "incorrect" || len(findings.Findings) != 1 {
		t.Fatalf("unexpected review findings artifact %#v", findings)
	}
	if findings.Findings[0].Priority != "P1" {
		t.Fatalf("expected normalized finding priority, got %#v", findings.Findings[0])
	}
}

func TestApplyApprovedInteractivePreviewCreatesTasksFromApprovedTaskPlan(t *testing.T) {
	db := newPlannerApprovalTestDB(t)

	artifactRepo := repository.NewAgentRunArtifactRepository(db)
	epicRepo := repository.NewPMEpicRepository(db)
	taskRepo := repository.NewPMTaskRepository(db)
	runRepo := repository.NewAgentRunRepository(db)
	agentRepo := repository.NewAgentRepository(db)

	agent := &model.Agent{
		ID:                    "agent-epic-tasks",
		WorkspaceID:           "ws-1",
		Name:                  "Epic Planner",
		Status:                "running",
		RuntimeKind:           "native_sdk",
		Skills:                model.AgentSkillRefs{},
		TriggerMode:           "manual",
		AllowedTools:          json.RawMessage(`[]`),
		AllowedCommands:       json.RawMessage(`[]`),
		AllowedTargets:        json.RawMessage(`[]`),
		ApprovalMode:          "preset_default",
		MaxConcurrentRuns:     1,
		DefaultInvocationMode: model.InvocationModeInteractive,
	}
	if err := db.Create(agent).Error; err != nil {
		t.Fatalf("create agent: %v", err)
	}

	run := &model.AgentRun{
		ID:             "run-task-plan",
		WorkspaceID:    "ws-1",
		AgentID:        agent.ID,
		TargetType:     "epic",
		TargetID:       "epic-1",
		InvocationMode: model.InvocationModeInteractive,
		Status:         model.AgentRunStatusRunning,
		Input:          json.RawMessage(`{}`),
		OutputSummary:  json.RawMessage(`{}`),
	}
	if err := db.Create(run).Error; err != nil {
		t.Fatalf("create run: %v", err)
	}

	epic := &model.PMEpic{
		ID:                 "epic-1",
		WorkspaceID:        "ws-1",
		Name:               "Epic",
		PlanningState:      model.EpicPlanningStateReadyForStoryPlanning,
		SpecClarifications: json.RawMessage(`[]`),
	}
	if err := db.Create(epic).Error; err != nil {
		t.Fatalf("create epic: %v", err)
	}
	if err := db.Exec(`INSERT INTO pm_workflow_states (id, state_type) VALUES (?, ?)`, "state-1", model.PMStateTypeUnstarted).Error; err != nil {
		t.Fatalf("create workflow state: %v", err)
	}

	previewPayload, err := json.Marshal(map[string]any{
		"summary": "Breakdown",
		"proposed_tasks": []map[string]any{
			{
				"ref":                 "task_1",
				"name":                "Add tracking helper",
				"description":         "Create shared metric helper",
				"task_type":           "chore",
				"acceptance_criteria": []string{"works"},
				"dependency_refs":     []string{},
			},
			{
				"ref":                 "task_2",
				"name":                "Wire tracking into capture errors",
				"description":         "Use the helper in capture",
				"task_type":           "feature",
				"acceptance_criteria": []string{"works"},
				"dependency_refs":     []string{"task_1"},
			},
		},
	})
	if err != nil {
		t.Fatalf("marshal preview payload: %v", err)
	}
	preview := model.ApprovedRunPreview{
		Phase:    "tasks",
		PanelKey: "task_plan",
		Format:   workerpkg.PreviewFormatJSON,
		Content:  previewPayload,
	}
	previewJSON, err := json.Marshal(preview)
	if err != nil {
		t.Fatalf("marshal approved preview: %v", err)
	}
	if err := db.Create(&model.AgentRunArtifact{
		ID:            "approved-tasks-1",
		WorkspaceID:   run.WorkspaceID,
		RunID:         run.ID,
		ArtifactType:  model.AgentRunArtifactTypeApprovedPreview,
		Format:        "json",
		StorageMode:   "inline",
		InlineContent: strPtr(string(previewJSON)),
		Metadata:      json.RawMessage(`{}`),
		SequenceNo:    1,
	}).Error; err != nil {
		t.Fatalf("create approved preview artifact: %v", err)
	}

	var executed []string
	commandExecutor := stubInternalCommandExecutor{
		executeFn: func(ctx context.Context, meta model.InternalCommandContext, name string, input json.RawMessage) (json.RawMessage, error) {
			executed = append(executed, name)
			if name != "pm.create_task_batch" {
				return json.RawMessage(`{}`), nil
			}
			var payload struct {
				Tasks []model.ProposedTask `json:"tasks"`
			}
			if err := json.Unmarshal(input, &payload); err != nil {
				return nil, err
			}
			for _, planned := range payload.Tasks {
				task := &model.PMTask{
					ID:              "db-" + planned.Ref,
					WorkspaceID:     run.WorkspaceID,
					Name:            planned.Name,
					TaskType:        planned.TaskType,
					WorkflowID:      "wf-1",
					WorkflowStateID: "state-1",
					EpicID:          &epic.ID,
					Priority:        model.PMTaskPriorityNone,
					Severity:        model.PMTaskSeverityNone,
				}
				if err := taskRepo.Create(ctx, task); err != nil {
					return nil, err
				}
			}
			return mustJSON(workerpkg.CreateTaskBatchResult{
				Tasks: []workerpkg.CreateTaskBatchTaskResult{
					{Ref: "task_1", TaskID: "db-task_1", Name: "Add tracking helper"},
					{Ref: "task_2", TaskID: "db-task_2", Name: "Wire tracking into capture errors"},
				},
			}), nil
		},
	}

	activity := &AgentRunActivities{
		runRepo:         runRepo,
		artifactRepo:    artifactRepo,
		epicRepo:        epicRepo,
		taskRepo:        taskRepo,
		agentRepo:       agentRepo,
		commandExecutor: commandExecutor,
	}
	state := &resolvedRunState{
		run:  run,
		epic: epic,
	}
	input := planningRunInput{Stage: model.PlanningStagePlanTasks}

	action, err := activity.applyApprovedInteractivePreview(context.Background(), state, &input)
	if err != nil {
		t.Fatalf("applyApprovedInteractivePreview returned error: %v", err)
	}
	if action != "create_tasks" {
		t.Fatalf("expected create_tasks action, got %q", action)
	}
	if len(executed) != 1 || executed[0] != "pm.create_task_batch" {
		t.Fatalf("expected task batch command, got %#v", executed)
	}
}

func TestPrepareTaskDeliveryKeepsRunBranchOverridesOffSavedTarget(t *testing.T) {
	dbName := fmt.Sprintf("file:prepare-task-delivery-%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dbName), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	if err := db.Exec(`CREATE TABLE task_delivery_targets (
		id TEXT PRIMARY KEY,
		workspace_id TEXT NOT NULL,
		task_id TEXT NOT NULL UNIQUE,
		repository_id TEXT,
		repo_full_name TEXT,
		integration_id TEXT,
		base_branch TEXT,
		working_branch TEXT,
		delivery_state TEXT NOT NULL DEFAULT 'unconfigured',
		active_pr_number INTEGER,
		active_pr_title TEXT,
		active_pr_url TEXT,
		active_pr_status TEXT,
		last_commit_sha TEXT,
		last_run_id TEXT,
		last_synced_at DATETIME,
		created_at DATETIME,
		updated_at DATETIME
	)`).Error; err != nil {
		t.Fatalf("create task_delivery_targets: %v", err)
	}

	deliveryRepo := repository.NewTaskDeliveryTargetRepository(db)
	target := &model.TaskDeliveryTarget{
		ID:            "target-1",
		WorkspaceID:   "ws-1",
		TaskID:        "task-1",
		RepositoryID:  strPtr("repo-1"),
		BaseBranch:    strPtr("develop"),
		WorkingBranch: strPtr("hlp-42-existing"),
		DeliveryState: "ready",
	}
	if err := deliveryRepo.Save(context.Background(), target); err != nil {
		t.Fatalf("save target: %v", err)
	}

	activities := &AgentRunActivities{deliveryRepo: deliveryRepo}
	state := &resolvedRunState{
		run: &model.AgentRun{
			ID:            "run-1",
			WorkspaceID:   "ws-1",
			BaseBranch:    strPtr("release/2026.04"),
			WorkingBranch: strPtr("lens/review-hotfix"),
		},
		resolved:       workerpkg.ResolvedProfile{RequiresRepo: false},
		deliveryTarget: target,
		repository: &model.GitRepository{
			ID:            "repo-1",
			IntegrationID: "integration-1",
			FullName:      "acme/api",
			DefaultBranch: "main",
		},
		integration: &model.GitIntegration{ID: "integration-1"},
	}

	if err := activities.prepareTaskDelivery(context.Background(), state); err != nil {
		t.Fatalf("prepareTaskDelivery returned error: %v", err)
	}

	reloaded, err := deliveryRepo.GetByTask(context.Background(), "ws-1", "task-1")
	if err != nil {
		t.Fatalf("reload target: %v", err)
	}
	if got := derefString(reloaded.BaseBranch); got != "develop" {
		t.Fatalf("saved base branch = %q, want develop", got)
	}
	if got := derefString(reloaded.WorkingBranch); got != "hlp-42-existing" {
		t.Fatalf("saved working branch = %q, want hlp-42-existing", got)
	}
	if got := derefString(state.run.BaseBranch); got != "release/2026.04" {
		t.Fatalf("run base branch = %q, want release/2026.04", got)
	}
	if got := derefString(state.run.WorkingBranch); got != "lens/review-hotfix" {
		t.Fatalf("run working branch = %q, want lens/review-hotfix", got)
	}
}

func TestCheckoutRunRefChecksOutRemoteWorkingBranchWithSlashName(t *testing.T) {
	ctx := context.Background()
	remoteDir := filepath.Join(t.TempDir(), "remote.git")
	runGitCommand(t, "", "init", "--bare", remoteDir)

	seedDir := filepath.Join(t.TempDir(), "seed")
	runGitCommand(t, "", "clone", remoteDir, seedDir)
	runGitCommand(t, seedDir, "config", "user.email", "test@example.com")
	runGitCommand(t, seedDir, "config", "user.name", "Test User")
	if err := os.WriteFile(filepath.Join(seedDir, "README.md"), []byte("seed\n"), 0o644); err != nil {
		t.Fatalf("write seed file: %v", err)
	}
	runGitCommand(t, seedDir, "add", "README.md")
	runGitCommand(t, seedDir, "commit", "-m", "initial")
	runGitCommand(t, seedDir, "push", "-u", "origin", "HEAD:main")

	branchName := "use-125-verify-anonymous-visitor-tracking-end-to-end"
	runGitCommand(t, seedDir, "checkout", "-b", branchName)
	if err := os.WriteFile(filepath.Join(seedDir, "feature.txt"), []byte("feature\n"), 0o644); err != nil {
		t.Fatalf("write feature file: %v", err)
	}
	runGitCommand(t, seedDir, "add", "feature.txt")
	runGitCommand(t, seedDir, "commit", "-m", "feature")
	runGitCommand(t, seedDir, "push", "-u", "origin", branchName)

	workDir := filepath.Join(t.TempDir(), "work")
	runGitCommand(t, "", "clone", remoteDir, workDir)

	activities := &AgentRunActivities{}
	state := &resolvedRunState{
		run: &model.AgentRun{
			BaseBranch:    strPtr(branchName),
			WorkingBranch: strPtr(branchName),
		},
		repository: &model.GitRepository{
			DefaultBranch: "main",
		},
	}

	if err := activities.checkoutRunRef(ctx, workDir, state); err != nil {
		t.Fatalf("checkoutRunRef returned error: %v", err)
	}

	gotBranch := strings.TrimSpace(runGitCommand(t, workDir, "branch", "--show-current"))
	if gotBranch != branchName {
		t.Fatalf("current branch = %q, want %q", gotBranch, branchName)
	}
}

func TestSyncBaseIntoWorkingBranchMergesBaseChangesForCodex(t *testing.T) {
	ctx := context.Background()
	remoteDir := filepath.Join(t.TempDir(), "remote.git")
	runGitCommand(t, "", "init", "--bare", remoteDir)

	seedDir := filepath.Join(t.TempDir(), "seed")
	runGitCommand(t, "", "clone", remoteDir, seedDir)
	runGitCommand(t, seedDir, "config", "user.email", "test@example.com")
	runGitCommand(t, seedDir, "config", "user.name", "Test User")
	if err := os.WriteFile(filepath.Join(seedDir, "README.md"), []byte("seed\n"), 0o644); err != nil {
		t.Fatalf("write readme: %v", err)
	}
	runGitCommand(t, seedDir, "add", "README.md")
	runGitCommand(t, seedDir, "commit", "-m", "initial")
	runGitCommand(t, seedDir, "push", "-u", "origin", "HEAD:main")

	featureBranch := "tp-123-feature"
	runGitCommand(t, seedDir, "checkout", "-b", featureBranch)
	if err := os.WriteFile(filepath.Join(seedDir, "feature.txt"), []byte("feature work\n"), 0o644); err != nil {
		t.Fatalf("write feature file: %v", err)
	}
	runGitCommand(t, seedDir, "add", "feature.txt")
	runGitCommand(t, seedDir, "commit", "-m", "feature work")
	runGitCommand(t, seedDir, "push", "-u", "origin", featureBranch)

	runGitCommand(t, seedDir, "checkout", "main")
	if err := os.WriteFile(filepath.Join(seedDir, "base.txt"), []byte("base update\n"), 0o644); err != nil {
		t.Fatalf("write base file: %v", err)
	}
	runGitCommand(t, seedDir, "add", "base.txt")
	runGitCommand(t, seedDir, "commit", "-m", "base update")
	runGitCommand(t, seedDir, "push", "origin", "main")

	workDir := filepath.Join(t.TempDir(), "work")
	runGitCommand(t, "", "clone", remoteDir, workDir)

	activities := &AgentRunActivities{}
	state := &resolvedRunState{
		run: &model.AgentRun{
			RuntimeKind:   "codex",
			BaseBranch:    strPtr("main"),
			WorkingBranch: strPtr(featureBranch),
		},
		repository: &model.GitRepository{DefaultBranch: "main"},
	}

	if err := activities.checkoutRunRef(ctx, workDir, state); err != nil {
		t.Fatalf("checkoutRunRef returned error: %v", err)
	}
	if err := activities.syncBaseIntoWorkingBranch(ctx, workDir, state); err != nil {
		t.Fatalf("syncBaseIntoWorkingBranch returned error: %v", err)
	}

	if state.branchSync.Status != "merged" {
		t.Fatalf("branch sync status = %q, want merged", state.branchSync.Status)
	}
	if _, err := os.Stat(filepath.Join(workDir, "base.txt")); err != nil {
		t.Fatalf("expected base branch file to exist after merge: %v", err)
	}
	if strings.TrimSpace(runGitCommand(t, workDir, "diff", "--name-only", "--diff-filter=U")) != "" {
		t.Fatal("expected no unresolved merge conflicts after clean base sync")
	}
}

func TestSyncBaseIntoWorkingBranchLeavesConflictForCodexToResolve(t *testing.T) {
	ctx := context.Background()
	remoteDir := filepath.Join(t.TempDir(), "remote.git")
	runGitCommand(t, "", "init", "--bare", remoteDir)

	seedDir := filepath.Join(t.TempDir(), "seed")
	runGitCommand(t, "", "clone", remoteDir, seedDir)
	runGitCommand(t, seedDir, "config", "user.email", "test@example.com")
	runGitCommand(t, seedDir, "config", "user.name", "Test User")
	if err := os.WriteFile(filepath.Join(seedDir, "conflict.txt"), []byte("base\n"), 0o644); err != nil {
		t.Fatalf("write conflict seed: %v", err)
	}
	runGitCommand(t, seedDir, "add", "conflict.txt")
	runGitCommand(t, seedDir, "commit", "-m", "initial")
	runGitCommand(t, seedDir, "push", "-u", "origin", "HEAD:main")

	featureBranch := "tp-124-conflict"
	runGitCommand(t, seedDir, "checkout", "-b", featureBranch)
	if err := os.WriteFile(filepath.Join(seedDir, "conflict.txt"), []byte("feature change\n"), 0o644); err != nil {
		t.Fatalf("write feature conflict file: %v", err)
	}
	runGitCommand(t, seedDir, "add", "conflict.txt")
	runGitCommand(t, seedDir, "commit", "-m", "feature change")
	runGitCommand(t, seedDir, "push", "-u", "origin", featureBranch)

	runGitCommand(t, seedDir, "checkout", "main")
	if err := os.WriteFile(filepath.Join(seedDir, "conflict.txt"), []byte("base change\n"), 0o644); err != nil {
		t.Fatalf("write base conflict file: %v", err)
	}
	runGitCommand(t, seedDir, "add", "conflict.txt")
	runGitCommand(t, seedDir, "commit", "-m", "base change")
	runGitCommand(t, seedDir, "push", "origin", "main")

	workDir := filepath.Join(t.TempDir(), "work")
	runGitCommand(t, "", "clone", remoteDir, workDir)

	activities := &AgentRunActivities{}
	state := &resolvedRunState{
		run: &model.AgentRun{
			RuntimeKind:   "codex",
			BaseBranch:    strPtr("main"),
			WorkingBranch: strPtr(featureBranch),
		},
		repository: &model.GitRepository{DefaultBranch: "main"},
	}

	if err := activities.checkoutRunRef(ctx, workDir, state); err != nil {
		t.Fatalf("checkoutRunRef returned error: %v", err)
	}
	if err := activities.syncBaseIntoWorkingBranch(ctx, workDir, state); err != nil {
		t.Fatalf("syncBaseIntoWorkingBranch returned error: %v", err)
	}

	if state.branchSync.Status != "conflicted" {
		t.Fatalf("branch sync status = %q, want conflicted", state.branchSync.Status)
	}
	if len(state.branchSync.ConflictFiles) != 1 || state.branchSync.ConflictFiles[0] != "conflict.txt" {
		t.Fatalf("conflict files = %#v, want [conflict.txt]", state.branchSync.ConflictFiles)
	}
	if got := strings.TrimSpace(runGitCommand(t, workDir, "diff", "--name-only", "--diff-filter=U")); got != "conflict.txt" {
		t.Fatalf("unmerged files = %q, want conflict.txt", got)
	}
}

func TestSyncBaseIntoWorkingBranchDeepensShallowCloneBeforeDeclaringUnrelatedHistory(t *testing.T) {
	ctx := context.Background()
	remoteDir := filepath.Join(t.TempDir(), "remote.git")
	runGitCommand(t, "", "init", "--bare", remoteDir)

	seedDir := filepath.Join(t.TempDir(), "seed")
	runGitCommand(t, "", "clone", "file://"+remoteDir, seedDir)
	runGitCommand(t, seedDir, "config", "user.email", "test@example.com")
	runGitCommand(t, seedDir, "config", "user.name", "Test User")

	if err := os.WriteFile(filepath.Join(seedDir, "README.md"), []byte("base\n"), 0o644); err != nil {
		t.Fatalf("write seed readme: %v", err)
	}
	runGitCommand(t, seedDir, "add", "README.md")
	runGitCommand(t, seedDir, "commit", "-m", "initial main")
	runGitCommand(t, seedDir, "branch", "-M", "main")
	runGitCommand(t, seedDir, "push", "-u", "origin", "HEAD:main")

	if err := os.WriteFile(filepath.Join(seedDir, "shared.txt"), []byte("shared history\n"), 0o644); err != nil {
		t.Fatalf("write shared history file: %v", err)
	}
	runGitCommand(t, seedDir, "add", "shared.txt")
	runGitCommand(t, seedDir, "commit", "-m", "shared history")
	runGitCommand(t, seedDir, "push", "origin", "main")

	if err := os.WriteFile(filepath.Join(seedDir, "task-context.txt"), []byte("branch point\n"), 0o644); err != nil {
		t.Fatalf("write branch point file: %v", err)
	}
	runGitCommand(t, seedDir, "add", "task-context.txt")
	runGitCommand(t, seedDir, "commit", "-m", "branch point")
	runGitCommand(t, seedDir, "push", "origin", "main")

	featureBranch := "tp-125-shallow-diverged"
	runGitCommand(t, seedDir, "checkout", "-b", featureBranch)
	if err := os.WriteFile(filepath.Join(seedDir, "task.txt"), []byte("task change\n"), 0o644); err != nil {
		t.Fatalf("write task branch file: %v", err)
	}
	runGitCommand(t, seedDir, "add", "task.txt")
	runGitCommand(t, seedDir, "commit", "-m", "task change")
	runGitCommand(t, seedDir, "push", "-u", "origin", featureBranch)

	runGitCommand(t, seedDir, "checkout", "main")
	if err := os.WriteFile(filepath.Join(seedDir, "base.txt"), []byte("base change\n"), 0o644); err != nil {
		t.Fatalf("write base branch file: %v", err)
	}
	runGitCommand(t, seedDir, "add", "base.txt")
	runGitCommand(t, seedDir, "commit", "-m", "base change")
	runGitCommand(t, seedDir, "push", "origin", "main")

	workDir := filepath.Join(t.TempDir(), "work")
	runGitCommand(t, "", "clone", "--depth", "1", "--branch", "main", "file://"+remoteDir, workDir)

	activities := &AgentRunActivities{}
	state := &resolvedRunState{
		run: &model.AgentRun{
			RuntimeKind:   "codex",
			BaseBranch:    strPtr("main"),
			WorkingBranch: strPtr(featureBranch),
		},
		repository: &model.GitRepository{DefaultBranch: "main"},
	}

	if err := activities.checkoutRunRef(ctx, workDir, state); err != nil {
		t.Fatalf("checkoutRunRef returned error: %v", err)
	}
	if err := activities.syncBaseIntoWorkingBranch(ctx, workDir, state); err != nil {
		t.Fatalf("syncBaseIntoWorkingBranch returned error: %v", err)
	}

	if state.branchSync.Status != "merged" {
		t.Fatalf("branch sync status = %q, want merged", state.branchSync.Status)
	}
	if strings.TrimSpace(state.branchSync.BackupBranch) != "" {
		t.Fatalf("did not expect backup branch for shallow-history recovery, got %q", state.branchSync.BackupBranch)
	}
	if _, err := os.Stat(filepath.Join(workDir, "task.txt")); err != nil {
		t.Fatalf("expected task branch file after merge: %v", err)
	}
	if _, err := os.Stat(filepath.Join(workDir, "base.txt")); err != nil {
		t.Fatalf("expected base branch file after merge: %v", err)
	}
	if got := strings.TrimSpace(runGitCommand(t, workDir, "diff", "--name-only", "--diff-filter=U")); got != "" {
		t.Fatalf("expected no unresolved merge conflicts after deepened base sync, got %q", got)
	}
}

func TestSyncBaseIntoWorkingBranchRecreatesUnrelatedHistoryBranchFromBase(t *testing.T) {
	ctx := context.Background()
	remoteDir := filepath.Join(t.TempDir(), "remote.git")
	runGitCommand(t, "", "init", "--bare", remoteDir)

	seedDir := filepath.Join(t.TempDir(), "seed")
	runGitCommand(t, "", "clone", remoteDir, seedDir)
	runGitCommand(t, seedDir, "config", "user.email", "test@example.com")
	runGitCommand(t, seedDir, "config", "user.name", "Test User")
	if err := os.WriteFile(filepath.Join(seedDir, "README.md"), []byte("main history\n"), 0o644); err != nil {
		t.Fatalf("write seed readme: %v", err)
	}
	runGitCommand(t, seedDir, "add", "README.md")
	runGitCommand(t, seedDir, "commit", "-m", "initial main")
	runGitCommand(t, seedDir, "push", "-u", "origin", "HEAD:main")

	featureBranch := "tp-999-unrelated"
	runGitCommand(t, seedDir, "checkout", "--orphan", featureBranch)
	runGitCommand(t, seedDir, "rm", "-rf", ".")
	if err := os.WriteFile(filepath.Join(seedDir, "UNRELATED.md"), []byte("orphan history\n"), 0o644); err != nil {
		t.Fatalf("write orphan file: %v", err)
	}
	runGitCommand(t, seedDir, "add", "UNRELATED.md")
	runGitCommand(t, seedDir, "commit", "-m", "orphan root")
	runGitCommand(t, seedDir, "push", "-u", "origin", featureBranch)

	workDir := filepath.Join(t.TempDir(), "work")
	runGitCommand(t, "", "clone", remoteDir, workDir)

	activities := &AgentRunActivities{}
	state := &resolvedRunState{
		run: &model.AgentRun{
			RuntimeKind:   "codex",
			BaseBranch:    strPtr("main"),
			WorkingBranch: strPtr(featureBranch),
		},
		repository: &model.GitRepository{DefaultBranch: "main"},
	}

	if err := activities.checkoutRunRef(ctx, workDir, state); err != nil {
		t.Fatalf("checkoutRunRef returned error: %v", err)
	}
	if err := activities.syncBaseIntoWorkingBranch(ctx, workDir, state); err != nil {
		t.Fatalf("syncBaseIntoWorkingBranch returned error: %v", err)
	}
	if state.branchSync.Status != "recreated_from_base" {
		t.Fatalf("branch sync status = %q, want recreated_from_base", state.branchSync.Status)
	}
	if strings.TrimSpace(state.branchSync.BackupBranch) == "" {
		t.Fatal("expected backup branch to be recorded")
	}
	remoteWorkingSHA := strings.TrimSpace(runGitCommand(t, "", "--git-dir", remoteDir, "rev-parse", "refs/heads/"+featureBranch))
	remoteMainSHA := strings.TrimSpace(runGitCommand(t, "", "--git-dir", remoteDir, "rev-parse", "refs/heads/main"))
	if remoteWorkingSHA != remoteMainSHA {
		t.Fatalf("remote working sha = %q, want %q", remoteWorkingSHA, remoteMainSHA)
	}
	remoteBackupSHA := strings.TrimSpace(runGitCommand(t, "", "--git-dir", remoteDir, "rev-parse", "refs/heads/"+state.branchSync.BackupBranch))
	if remoteBackupSHA == remoteMainSHA {
		t.Fatalf("expected backup branch %q to preserve unrelated history", state.branchSync.BackupBranch)
	}
}

func TestSyncBaseIntoWorkingBranchFailsForUnrelatedHistoryWithActivePR(t *testing.T) {
	ctx := context.Background()
	remoteDir := filepath.Join(t.TempDir(), "remote.git")
	runGitCommand(t, "", "init", "--bare", remoteDir)

	seedDir := filepath.Join(t.TempDir(), "seed")
	runGitCommand(t, "", "clone", remoteDir, seedDir)
	runGitCommand(t, seedDir, "config", "user.email", "test@example.com")
	runGitCommand(t, seedDir, "config", "user.name", "Test User")
	if err := os.WriteFile(filepath.Join(seedDir, "README.md"), []byte("main history\n"), 0o644); err != nil {
		t.Fatalf("write seed readme: %v", err)
	}
	runGitCommand(t, seedDir, "add", "README.md")
	runGitCommand(t, seedDir, "commit", "-m", "initial main")
	runGitCommand(t, seedDir, "push", "-u", "origin", "HEAD:main")

	featureBranch := "tp-1000-unrelated-pr"
	runGitCommand(t, seedDir, "checkout", "--orphan", featureBranch)
	runGitCommand(t, seedDir, "rm", "-rf", ".")
	if err := os.WriteFile(filepath.Join(seedDir, "UNRELATED.md"), []byte("orphan history\n"), 0o644); err != nil {
		t.Fatalf("write orphan file: %v", err)
	}
	runGitCommand(t, seedDir, "add", "UNRELATED.md")
	runGitCommand(t, seedDir, "commit", "-m", "orphan root")
	runGitCommand(t, seedDir, "push", "-u", "origin", featureBranch)

	workDir := filepath.Join(t.TempDir(), "work")
	runGitCommand(t, "", "clone", remoteDir, workDir)

	prNumber := 42
	activities := &AgentRunActivities{}
	state := &resolvedRunState{
		run: &model.AgentRun{
			RuntimeKind:   "codex",
			BaseBranch:    strPtr("main"),
			WorkingBranch: strPtr(featureBranch),
		},
		repository: &model.GitRepository{DefaultBranch: "main"},
		deliveryTarget: &model.TaskDeliveryTarget{
			ActivePRNumber: &prNumber,
		},
	}

	if err := activities.checkoutRunRef(ctx, workDir, state); err != nil {
		t.Fatalf("checkoutRunRef returned error: %v", err)
	}
	err := activities.syncBaseIntoWorkingBranch(ctx, workDir, state)
	if err == nil {
		t.Fatal("expected unrelated history with active PR to fail")
	}
	if !strings.Contains(err.Error(), "active pull request") {
		t.Fatalf("expected active PR error, got %v", err)
	}
	if state.branchSync.Status != "unrelated_history" {
		t.Fatalf("branch sync status = %q, want unrelated_history", state.branchSync.Status)
	}
}

func TestPushCodexLocalCommitPushesCommittedBranch(t *testing.T) {
	ctx := context.Background()
	remoteDir := filepath.Join(t.TempDir(), "remote.git")
	runGitCommand(t, "", "init", "--bare", remoteDir)

	seedDir := filepath.Join(t.TempDir(), "seed")
	runGitCommand(t, "", "clone", remoteDir, seedDir)
	runGitCommand(t, seedDir, "config", "user.email", "test@example.com")
	runGitCommand(t, seedDir, "config", "user.name", "Test User")
	if err := os.WriteFile(filepath.Join(seedDir, "README.md"), []byte("seed\n"), 0o644); err != nil {
		t.Fatalf("write seed file: %v", err)
	}
	runGitCommand(t, seedDir, "add", "README.md")
	runGitCommand(t, seedDir, "commit", "-m", "initial")
	runGitCommand(t, seedDir, "push", "-u", "origin", "HEAD:main")

	workDir := filepath.Join(t.TempDir(), "work")
	runGitCommand(t, "", "clone", remoteDir, workDir)
	runGitCommand(t, workDir, "config", "user.email", "test@example.com")
	runGitCommand(t, workDir, "config", "user.name", "Test User")
	runGitCommand(t, workDir, "checkout", "-B", "main", "origin/main")
	runGitCommand(t, workDir, "checkout", "-b", "tp-123-implement")
	if err := os.WriteFile(filepath.Join(workDir, "README.md"), []byte("seed\nupdated\n"), 0o644); err != nil {
		t.Fatalf("write work file: %v", err)
	}
	runGitCommand(t, workDir, "add", "README.md")
	runGitCommand(t, workDir, "commit", "-m", "tp: task #123 Implement notification preferences")
	localSHA := strings.TrimSpace(runGitCommand(t, workDir, "rev-parse", "HEAD"))

	activities := &AgentRunActivities{}
	state := &resolvedRunState{
		run: &model.AgentRun{
			ID:            "run-1",
			WorkspaceID:   "ws-1",
			RuntimeKind:   "codex",
			BaseBranch:    strPtr("main"),
			WorkingBranch: strPtr("tp-123-implement"),
		},
	}
	execCtx := &workerpkg.ExecutionContext{
		Context:       ctx,
		WorkDir:       workDir,
		WorkingBranch: "tp-123-implement",
		LocalGitCommit: &workerpkg.GitCommitMetadata{
			Branch:        "tp-123-implement",
			CommitSHA:     localSHA,
			CommitMessage: "tp: task #123 Implement notification preferences",
			ChangedFiles:  []string{"README.md"},
		},
	}

	if err := activities.pushCodexLocalCommit(ctx, workDir, state, execCtx); err != nil {
		t.Fatalf("pushCodexLocalCommit returned error: %v", err)
	}

	remoteSHA := strings.TrimSpace(runGitCommand(t, "", "--git-dir", remoteDir, "rev-parse", "refs/heads/tp-123-implement"))
	if remoteSHA != localSHA {
		t.Fatalf("remote sha = %q, want %q", remoteSHA, localSHA)
	}
}

func TestEnsureGitHubPullRequestReusesExistingOpenPR(t *testing.T) {
	t.Parallel()

	var postCalled bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v3/repos/acme/rust-capture/pulls":
			if got := r.URL.Query().Get("head"); got != "acme:helpin/task-123" {
				t.Fatalf("head query = %q, want acme:helpin/task-123", got)
			}
			if got := r.URL.Query().Get("base"); got != "develop" {
				t.Fatalf("base query = %q, want develop", got)
			}
			_ = json.NewEncoder(w).Encode([]map[string]any{{
				"number":   42,
				"title":    "HLP-123: Improve delivery flow",
				"html_url": "https://example.test/pr/42",
				"head":     map[string]any{"ref": "helpin/task-123"},
				"base":     map[string]any{"ref": "develop"},
			}})
		case r.Method == http.MethodPost && r.URL.Path == "/api/v3/repos/acme/rust-capture/pulls":
			postCalled = true
			t.Fatalf("did not expect PR creation request when an open PR already exists")
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	pr, err := ensureGitHubPullRequest(context.Background(), &model.GitIntegration{
		Provider: "github",
		BaseURL:  strPtr(server.URL),
	}, "token-123", "acme/rust-capture", "helpin/task-123", "develop", "ignored", "ignored")
	if err != nil {
		t.Fatalf("ensureGitHubPullRequest returned error: %v", err)
	}
	if pr == nil {
		t.Fatal("expected PR metadata")
	}
	if !pr.Existing {
		t.Fatal("expected existing PR to be reused")
	}
	if pr.Metadata.Number != 42 {
		t.Fatalf("pr number = %d, want 42", pr.Metadata.Number)
	}
	if pr.Title != "HLP-123: Improve delivery flow" {
		t.Fatalf("title = %q", pr.Title)
	}
	if postCalled {
		t.Fatal("did not expect create PR request")
	}
}

func TestEnsureGitHubPullRequestCreatesPRWhenMissing(t *testing.T) {
	t.Parallel()

	var createPayload map[string]string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v3/repos/acme/rust-capture/pulls":
			_ = json.NewEncoder(w).Encode([]map[string]any{})
		case r.Method == http.MethodPost && r.URL.Path == "/api/v3/repos/acme/rust-capture/pulls":
			if err := json.NewDecoder(r.Body).Decode(&createPayload); err != nil {
				t.Fatalf("decode create payload: %v", err)
			}
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"number":   77,
				"title":    createPayload["title"],
				"html_url": "https://example.test/pr/77",
				"head":     map[string]any{"ref": createPayload["head"]},
				"base":     map[string]any{"ref": createPayload["base"]},
			})
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	pr, err := ensureGitHubPullRequest(context.Background(), &model.GitIntegration{
		Provider: "github",
		BaseURL:  strPtr(server.URL),
	}, "token-123", "acme/rust-capture", "helpin/task-123", "release/2026.04", "HLP-123: Improve delivery flow", "body text")
	if err != nil {
		t.Fatalf("ensureGitHubPullRequest returned error: %v", err)
	}
	if pr == nil {
		t.Fatal("expected created PR metadata")
	}
	if pr.Existing {
		t.Fatal("expected a new PR to be created")
	}
	if pr.Metadata.Number != 77 {
		t.Fatalf("pr number = %d, want 77", pr.Metadata.Number)
	}
	if createPayload["head"] != "helpin/task-123" {
		t.Fatalf("head payload = %q, want helpin/task-123", createPayload["head"])
	}
	if createPayload["base"] != "release/2026.04" {
		t.Fatalf("base payload = %q, want release/2026.04", createPayload["base"])
	}
	if createPayload["title"] != "HLP-123: Improve delivery flow" {
		t.Fatalf("title payload = %q", createPayload["title"])
	}
}

func TestRecordPushAndEnsureDeliveryPRMarksPRFailedWhenPROpenFails(t *testing.T) {
	t.Parallel()

	dbName := fmt.Sprintf("file:delivery-pr-failure-%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dbName), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	for _, stmt := range []string{
		`CREATE TABLE agent_runs (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			agent_id TEXT NOT NULL,
			task_id TEXT,
			conversation_id TEXT,
			target_type TEXT NOT NULL DEFAULT 'task',
			target_id TEXT NOT NULL,
			runtime_kind TEXT NOT NULL DEFAULT 'opencode',
			invocation_mode TEXT NOT NULL DEFAULT 'autonomous',
			parent_run_id TEXT,
			handoff_state TEXT,
			approval_state TEXT NOT NULL DEFAULT 'not_required',
			pause_reason TEXT NOT NULL DEFAULT 'none',
			triggered_by_user_id TEXT,
			status TEXT NOT NULL DEFAULT 'queued',
			workflow_id TEXT,
			workflow_run_id TEXT,
			task_queue TEXT,
			runner_pool TEXT,
			repository_id TEXT,
			repo_full_name TEXT,
			base_branch TEXT,
			working_branch TEXT,
			delivery_target_id TEXT,
			execution_stage TEXT,
			last_heartbeat_at DATETIME,
			input BLOB NOT NULL DEFAULT (CAST('{}' AS BLOB)),
			output_summary BLOB NOT NULL DEFAULT (CAST('{}' AS BLOB)),
			cached_input_tokens INTEGER NOT NULL DEFAULT 0,
			input_tokens INTEGER NOT NULL DEFAULT 0,
			output_tokens INTEGER NOT NULL DEFAULT 0,
			tokens_used INTEGER NOT NULL DEFAULT 0,
			error_message TEXT,
			started_at DATETIME,
			completed_at DATETIME,
			created_at DATETIME,
			updated_at DATETIME
		)`,
		`CREATE TABLE task_delivery_targets (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			task_id TEXT NOT NULL UNIQUE,
			repository_id TEXT,
			repo_full_name TEXT,
			integration_id TEXT,
			base_branch TEXT,
			working_branch TEXT,
			delivery_state TEXT NOT NULL DEFAULT 'unconfigured',
			active_pr_number INTEGER,
			active_pr_title TEXT,
			active_pr_url TEXT,
			active_pr_status TEXT,
			last_commit_sha TEXT,
			last_run_id TEXT,
			last_synced_at DATETIME,
			created_at DATETIME,
			updated_at DATETIME
		)`,
		`CREATE TABLE task_git_links (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			task_id TEXT NOT NULL,
			integration_id TEXT NOT NULL,
			repository_id TEXT,
			run_id TEXT,
			provider TEXT NOT NULL,
			repo TEXT NOT NULL,
			branch TEXT,
			pr_number INTEGER,
			pr_title TEXT,
			pr_url TEXT,
			pr_status TEXT,
			commit_sha TEXT,
			created_at DATETIME,
			updated_at DATETIME
		)`,
	} {
		if err := db.Exec(stmt).Error; err != nil {
			t.Fatalf("exec schema: %v", err)
		}
	}

	runRepo := repository.NewAgentRunRepository(db)
	deliveryRepo := repository.NewTaskDeliveryTargetRepository(db)
	gitLinkRepo := repository.NewTaskGitLinkRepository(db)

	run := &model.AgentRun{
		ID:          "run-1",
		WorkspaceID: "ws-1",
		AgentID:     "agent-1",
		TaskID:      strPtr("task-1"),
		TargetType:  "task",
		TargetID:    "task-1",
		BaseBranch:  strPtr("main"),
	}
	if err := db.Create(run).Error; err != nil {
		t.Fatalf("create run: %v", err)
	}

	target := &model.TaskDeliveryTarget{
		ID:            "delivery-1",
		WorkspaceID:   "ws-1",
		TaskID:        "task-1",
		RepoFullName:  strPtr("acme/rust-capture"),
		BaseBranch:    strPtr("main"),
		WorkingBranch: strPtr("helpin/task-123"),
		DeliveryState: "in_progress",
	}
	if err := db.Create(target).Error; err != nil {
		t.Fatalf("create delivery target: %v", err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v3/repos/acme/rust-capture/pulls" {
			t.Fatalf("unexpected request path %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]any{"message": "boom"})
	}))
	defer server.Close()

	activities := &AgentRunActivities{
		runRepo:      runRepo,
		deliveryRepo: deliveryRepo,
		gitLinkRepo:  gitLinkRepo,
	}
	state := &resolvedRunState{
		run:            run,
		task:           &model.PMTask{ID: "task-1", Name: "Improve delivery flow", DisplayID: 123},
		repository:     &model.GitRepository{FullName: "acme/rust-capture", DefaultBranch: "main"},
		integration:    &model.GitIntegration{Provider: "github", BaseURL: strPtr(server.URL)},
		deliveryTarget: target,
		accessToken:    "token-123",
		workspaceKey:   "HLP",
	}

	err = activities.recordPushAndEnsureDeliveryPR(context.Background(), state, "helpin/task-123", "abc123")
	if err != nil {
		t.Fatalf("expected PR creation failure to be non-fatal, got %v", err)
	}

	updated, err := deliveryRepo.GetByTask(context.Background(), "ws-1", "task-1")
	if err != nil {
		t.Fatalf("reload delivery target: %v", err)
	}
	if updated == nil {
		t.Fatal("expected delivery target after update")
	}
	if updated.DeliveryState != "pr_failed" {
		t.Fatalf("delivery state = %q, want pr_failed", updated.DeliveryState)
	}
}

func runGitCommand(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	if dir != "" {
		cmd.Dir = dir
	}
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s failed: %v\n%s", strings.Join(args, " "), err, string(output))
	}
	return string(output)
}

func newPlannerApprovalTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	dbName := fmt.Sprintf("file:planner-approval-%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dbName), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	for _, stmt := range []string{
		`CREATE TABLE agents (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			is_system BOOLEAN NOT NULL DEFAULT 0,
			name TEXT NOT NULL,
			preset_key TEXT,
			preset_version_key TEXT,
			source_preset_key TEXT,
			source_preset_version_key TEXT,
			role TEXT,
			status TEXT NOT NULL DEFAULT 'idle',
			runtime_kind TEXT NOT NULL DEFAULT 'opencode',
			skills TEXT NOT NULL DEFAULT '[]',
			trigger_mode TEXT NOT NULL DEFAULT 'manual',
			provider TEXT,
			model TEXT,
			execution_config BLOB NOT NULL DEFAULT x'7b7d',
			system_prompt TEXT,
			instruction_template_version TEXT NOT NULL DEFAULT '',
			planning_notes TEXT,
			monthly_token_budget INTEGER,
			tokens_used_this_month INTEGER NOT NULL DEFAULT 0,
			active_task_id TEXT,
			team_id TEXT,
			allowed_tools TEXT NOT NULL DEFAULT '[]',
			allowed_commands TEXT NOT NULL DEFAULT '[]',
			allowed_targets TEXT NOT NULL DEFAULT '[]',
			schedule TEXT,
			approval_mode TEXT NOT NULL DEFAULT 'preset_default',
			max_concurrent_runs INTEGER NOT NULL DEFAULT 1,
			default_invocation_mode TEXT NOT NULL DEFAULT 'autonomous',
			created_at DATETIME,
			updated_at DATETIME
		)`,
		`CREATE TABLE agent_runs (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			agent_id TEXT NOT NULL,
			task_id TEXT,
			conversation_id TEXT,
			target_type TEXT NOT NULL DEFAULT 'story',
			target_id TEXT NOT NULL,
			runtime_kind TEXT NOT NULL DEFAULT 'opencode',
			invocation_mode TEXT NOT NULL DEFAULT 'autonomous',
			parent_run_id TEXT,
			handoff_state TEXT,
			approval_state TEXT NOT NULL DEFAULT 'not_required',
			pause_reason TEXT NOT NULL DEFAULT 'none',
			triggered_by_user_id TEXT,
			status TEXT NOT NULL DEFAULT 'queued',
			workflow_id TEXT,
			workflow_run_id TEXT,
			task_queue TEXT,
			runner_pool TEXT,
			repository_id TEXT,
			repo_full_name TEXT,
			base_branch TEXT,
			working_branch TEXT,
			delivery_target_id TEXT,
			execution_stage TEXT,
			last_heartbeat_at DATETIME,
			input BLOB NOT NULL DEFAULT (CAST('{}' AS BLOB)),
			output_summary BLOB NOT NULL DEFAULT (CAST('{}' AS BLOB)),
			cached_input_tokens INTEGER NOT NULL DEFAULT 0,
			input_tokens INTEGER NOT NULL DEFAULT 0,
			output_tokens INTEGER NOT NULL DEFAULT 0,
			tokens_used INTEGER NOT NULL DEFAULT 0,
			error_message TEXT,
			started_at DATETIME,
			completed_at DATETIME,
			created_at DATETIME,
			updated_at DATETIME
		)`,
		`CREATE TABLE agent_run_artifacts (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			run_id TEXT NOT NULL,
			artifact_type TEXT NOT NULL,
			format TEXT NOT NULL DEFAULT 'text',
			storage_mode TEXT NOT NULL DEFAULT 'inline',
			inline_content TEXT,
			object_key TEXT,
			metadata TEXT NOT NULL DEFAULT '{}',
			sequence_no INTEGER NOT NULL DEFAULT 0,
			created_at DATETIME
		)`,
		`CREATE TABLE pm_epics (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			name TEXT NOT NULL,
			description TEXT,
			external_id TEXT,
			epic_state_id TEXT,
			owner_id TEXT,
			owner_member_id TEXT,
			team_id TEXT,
			planned_start_date DATETIME,
			deadline DATETIME,
			started BOOLEAN NOT NULL DEFAULT 0,
			started_at DATETIME,
			completed BOOLEAN NOT NULL DEFAULT 0,
			completed_at DATETIME,
			position INTEGER NOT NULL DEFAULT 0,
			color TEXT,
			health TEXT NOT NULL DEFAULT 'no_health',
			health_comment TEXT,
			archived BOOLEAN NOT NULL DEFAULT 0,
			spec_document_id TEXT,
			planning_repository_id TEXT,
			planning_state TEXT NOT NULL DEFAULT 'not_started',
			spec_clarifications TEXT NOT NULL DEFAULT '[]',
			spec_clarified_at DATETIME,
			spec_clarified_by TEXT,
			approved_spec_version_id TEXT,
			last_planning_run_id TEXT,
			created_by TEXT,
			created_at DATETIME,
			updated_at DATETIME
		)`,
		`CREATE TABLE pm_epic_labels (
			epic_id TEXT NOT NULL,
			label_id TEXT NOT NULL,
			created_at DATETIME
		)`,
		`CREATE TABLE pm_labels (
			id TEXT PRIMARY KEY,
			workspace_id TEXT,
			name TEXT NOT NULL,
			color TEXT,
			created_at DATETIME,
			updated_at DATETIME
		)`,
		`CREATE TABLE pm_epic_objectives (
			epic_id TEXT NOT NULL,
			objective_id TEXT NOT NULL,
			created_at DATETIME
		)`,
		`CREATE TABLE pm_objectives (
			id TEXT PRIMARY KEY,
			workspace_id TEXT,
			name TEXT NOT NULL
		)`,
		`CREATE TABLE pm_workflow_states (
			id TEXT PRIMARY KEY,
			state_type TEXT NOT NULL
		)`,
		`CREATE TABLE pm_tasks (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			display_id INTEGER NOT NULL DEFAULT 0,
			name TEXT NOT NULL,
			description TEXT,
			task_type TEXT NOT NULL DEFAULT 'feature',
			workflow_id TEXT NOT NULL,
			workflow_state_id TEXT NOT NULL,
			epic_id TEXT,
			sprint_id TEXT,
			team_id TEXT,
			owner_id TEXT,
			owner_member_id TEXT,
			requester_id TEXT,
			requester_member_id TEXT,
			estimate INTEGER,
			priority TEXT NOT NULL DEFAULT 'none',
			severity TEXT NOT NULL DEFAULT 'none',
			deadline DATETIME,
			position INTEGER NOT NULL DEFAULT 0,
			started BOOLEAN NOT NULL DEFAULT 0,
			started_at DATETIME,
			completed BOOLEAN NOT NULL DEFAULT 0,
			completed_at DATETIME,
			moved_at DATETIME,
			blocked BOOLEAN NOT NULL DEFAULT 0,
			blocker TEXT,
			archived BOOLEAN NOT NULL DEFAULT 0,
			assigned_agent_id TEXT,
			plan_document_id TEXT,
			template_id TEXT,
			recurring_template_id TEXT,
			recurring_run_id TEXT,
			recurring_occurrence_number INTEGER,
			external_id TEXT,
			slice_type TEXT,
			implementation_brief TEXT,
			created_at DATETIME,
			updated_at DATETIME
		)`,
		`CREATE TABLE docs_spaces (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			team_id TEXT,
			name TEXT NOT NULL,
			slug TEXT NOT NULL,
			icon TEXT,
			visibility TEXT NOT NULL,
			type TEXT NOT NULL,
			default_review_days INTEGER,
			description TEXT,
			position INTEGER NOT NULL DEFAULT 0,
			is_system BOOLEAN NOT NULL DEFAULT 0,
			created_by TEXT NOT NULL,
			created_at DATETIME,
			updated_at DATETIME,
			deleted_at DATETIME
		)`,
		`CREATE TABLE docs_documents (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			space_id TEXT NOT NULL,
			collection_id TEXT,
			title TEXT NOT NULL,
			status TEXT NOT NULL,
			visibility TEXT NOT NULL,
			owner_id TEXT,
			team_id TEXT,
			template_key TEXT,
			excerpt TEXT,
			icon TEXT,
			tags TEXT,
			is_pinned BOOLEAN NOT NULL DEFAULT 0,
			is_publicly_shared BOOLEAN NOT NULL DEFAULT 0,
			share_token TEXT,
			is_locked BOOLEAN NOT NULL DEFAULT 0,
			locked_by TEXT,
			last_reviewed_at DATETIME,
			next_review_at DATETIME,
			published_at DATETIME,
			created_by TEXT NOT NULL,
			created_at DATETIME,
			updated_at DATETIME,
			deleted_at DATETIME
		)`,
		`CREATE TABLE docs_contents (
			id TEXT PRIMARY KEY,
			document_id TEXT NOT NULL UNIQUE,
			content TEXT,
			content_text TEXT,
			word_count INTEGER NOT NULL DEFAULT 0,
			created_at DATETIME,
			updated_at DATETIME
		)`,
		`CREATE TABLE docs_versions (
			id TEXT PRIMARY KEY,
			document_id TEXT NOT NULL,
			content TEXT,
			content_text TEXT,
			snapshot_label TEXT,
			version_type TEXT NOT NULL,
			word_count INTEGER NOT NULL DEFAULT 0,
			created_by TEXT NOT NULL,
			created_at DATETIME
		)`,
		`CREATE TABLE docs_links (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			document_id TEXT NOT NULL,
			linked_object_type TEXT NOT NULL,
			linked_object_id TEXT NOT NULL,
			link_context TEXT NOT NULL,
			created_by TEXT NOT NULL,
			created_at DATETIME
		)`,
	} {
		if err := db.Exec(stmt).Error; err != nil {
			t.Fatalf("create test table: %v", err)
		}
	}
	return db
}

func TestValidatePlanningProposalStoriesRejectsCycle(t *testing.T) {
	stories := []model.ProposedTask{
		{Ref: "story_a", Name: "Story A", AcceptanceCriteria: []string{"A works"}, DependencyRefs: []string{"story_b"}},
		{Ref: "story_b", Name: "Story B", AcceptanceCriteria: []string{"B works"}, DependencyRefs: []string{"story_a"}},
	}

	err := validatePlanningProposalTasks(stories)
	if err == nil {
		t.Fatal("expected circular dependency error")
	}
}

func TestSelectRelevantPlanningFilesPrefersRelevantPaths(t *testing.T) {
	root := t.TempDir()
	files := map[string]string{
		"go.mod":                             "module example.com/test\n",
		"server/internal/handler/billing.go": "package handler\n",
		"server/internal/service/billing_service.go": "package service\n",
		"server/internal/model/invoice.go":           "package model\n",
		"web/src/pages/BillingPage.tsx":              "export const BillingPage = () => null\n",
		"docs/notes.md":                              "billing settings\n",
	}
	for name, content := range files {
		fullPath := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if err := os.WriteFile(fullPath, []byte(content), 0o644); err != nil {
			t.Fatalf("write file: %v", err)
		}
	}

	paths, err := selectRelevantPlanningFiles(root, "We need to improve billing invoices and billing settings.")
	if err != nil {
		t.Fatalf("selectRelevantPlanningFiles: %v", err)
	}
	if len(paths) == 0 {
		t.Fatal("expected relevant planning files")
	}
	found := false
	for _, path := range paths {
		if path == "server/internal/service/billing_service.go" || path == "server/internal/handler/billing.go" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected billing-related backend file in result, got %#v", paths)
	}
}

func TestCollectPlanningTreeBoundsDepth(t *testing.T) {
	root := t.TempDir()
	deepPath := filepath.Join(root, "server", "internal", "service", "payments", "v2", "handler.go")
	if err := os.MkdirAll(filepath.Dir(deepPath), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(deepPath, []byte("package service\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	tree, err := collectPlanningTree(root, 3, 20)
	if err != nil {
		t.Fatalf("collectPlanningTree: %v", err)
	}
	for _, item := range tree {
		if item == "        server/internal/service/payments/v2/handler.go" {
			t.Fatalf("expected deep file to be truncated by depth, got %#v", tree)
		}
	}
}

func TestValidatePlanningProposalTasksNormalizesMissingRefs(t *testing.T) {
	tasks := []model.ProposedTask{
		{Name: "Task A", AcceptanceCriteria: []string{"A works"}},
		{Name: "Task B", AcceptanceCriteria: []string{"B works"}, DependencyRefs: []string{"task_1"}},
	}

	if err := validatePlanningProposalTasks(tasks); err != nil {
		t.Fatalf("validatePlanningProposalTasks returned error: %v", err)
	}
	if tasks[0].Ref != "task_1" {
		t.Fatalf("expected first task ref to default to task_1, got %q", tasks[0].Ref)
	}
	if tasks[1].Ref != "task_2" {
		t.Fatalf("expected second task ref to default to task_2, got %q", tasks[1].Ref)
	}
}

func TestMarkdownToDocsJSONPreservesHeadingsAndBullets(t *testing.T) {
	raw := tiptap.MarkdownToJSON("# Problem\n\n- first item\n- second item\n\nPlain paragraph")

	var doc struct {
		Type    string                   `json:"type"`
		Content []map[string]interface{} `json:"content"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("unmarshal docs json: %v", err)
	}
	if doc.Type != "doc" {
		t.Fatalf("expected root type doc, got %q", doc.Type)
	}
	if len(doc.Content) < 3 {
		t.Fatalf("expected heading, list, and paragraph nodes, got %d nodes", len(doc.Content))
	}
	if doc.Content[0]["type"] != "heading" {
		t.Fatalf("expected first node to be heading, got %#v", doc.Content[0]["type"])
	}
	if doc.Content[1]["type"] != "bulletList" {
		t.Fatalf("expected second node to be bulletList, got %#v", doc.Content[1]["type"])
	}
	if doc.Content[2]["type"] != "paragraph" {
		t.Fatalf("expected third node to be paragraph, got %#v", doc.Content[2]["type"])
	}
}

func TestDecodeApprovedStoryPlanPreviewContentAcceptsStringifiedJSON(t *testing.T) {
	raw := json.RawMessage(`"{\"summary\":\"Breakdown\",\"proposed_stories\":[{\"ref\":\"story_1\",\"name\":\"Story A\",\"description\":\"Do A\",\"story_type\":\"feature\",\"acceptance_criteria\":[\"works\"]}]}"`)

	proposal, err := decodeApprovedTaskPlanPreviewContent(raw)
	if err != nil {
		t.Fatalf("decodeApprovedTaskPlanPreviewContent returned error: %v", err)
	}
	if proposal.Summary != "Breakdown" {
		t.Fatalf("expected summary Breakdown, got %q", proposal.Summary)
	}
	if len(proposal.ProposedTasks) != 1 || proposal.ProposedTasks[0].Ref != "story_1" {
		t.Fatalf("unexpected proposal stories: %#v", proposal.ProposedTasks)
	}
}

func TestDecodeApprovedStoryPlanPreviewContentAcceptsFencedJSONString(t *testing.T) {
	raw := json.RawMessage("\"Here is the plan in the required format:\\n```json\\n{\\\"summary\\\":\\\"Breakdown\\\",\\\"proposed_stories\\\":[{\\\"ref\\\":\\\"story_1\\\",\\\"name\\\":\\\"Story A\\\",\\\"description\\\":\\\"Do A\\\",\\\"story_type\\\":\\\"feature\\\",\\\"acceptance_criteria\\\":[\\\"works\\\"]}]}\\n```\"")

	proposal, err := decodeApprovedTaskPlanPreviewContent(raw)
	if err != nil {
		t.Fatalf("decodeApprovedTaskPlanPreviewContent returned error: %v", err)
	}
	if proposal.Summary != "Breakdown" {
		t.Fatalf("expected summary Breakdown, got %q", proposal.Summary)
	}
	if len(proposal.ProposedTasks) != 1 || proposal.ProposedTasks[0].Name != "Story A" {
		t.Fatalf("unexpected proposal stories: %#v", proposal.ProposedTasks)
	}
}

func TestDecodeApprovedStoryPlanPreviewContentAcceptsArrayTestStrategy(t *testing.T) {
	raw := json.RawMessage(`{
		"summary":"Breakdown",
		"proposed_stories":[{
			"ref":"story_1",
			"name":"Story A",
			"description":"Do A",
			"story_type":"feature",
			"acceptance_criteria":["works"],
			"implementation_brief":{
				"approach":"Add the metric helper",
				"files_to_modify":[{"path":"a.go","action":"modify","description":"update helper"}],
				"test_strategy":["Add parser coverage","Add integration coverage"]
			}
		}]
	}`)

	proposal, err := decodeApprovedTaskPlanPreviewContent(raw)
	if err != nil {
		t.Fatalf("decodeApprovedTaskPlanPreviewContent returned error: %v", err)
	}
	brief := proposal.ProposedTasks[0].ImplementationBrief
	if brief == nil {
		t.Fatalf("expected implementation brief, got %#v", proposal.ProposedTasks[0])
	}
	expected := "Add parser coverage\nAdd integration coverage"
	if brief.TestStrategy != expected {
		t.Fatalf("expected joined test strategy %q, got %q", expected, brief.TestStrategy)
	}
}

func TestDecodeApprovedStoryPlanPreviewContentAcceptsTitleAndTypeAliases(t *testing.T) {
	raw := json.RawMessage(`{
		"summary":"Breakdown",
		"proposed_stories":[{
			"ref":"story_1",
			"title":"Add 4xx error metrics tracking infrastructure",
			"description":"Do A",
			"type":"feature",
			"acceptance_criteria":["works"]
		}]
	}`)

	proposal, err := decodeApprovedTaskPlanPreviewContent(raw)
	if err != nil {
		t.Fatalf("decodeApprovedTaskPlanPreviewContent returned error: %v", err)
	}
	if len(proposal.ProposedTasks) != 1 {
		t.Fatalf("expected one proposed story, got %#v", proposal.ProposedTasks)
	}
	if proposal.ProposedTasks[0].Name != "Add 4xx error metrics tracking infrastructure" {
		t.Fatalf("expected title alias to populate Name, got %#v", proposal.ProposedTasks[0])
	}
	if proposal.ProposedTasks[0].TaskType != "feature" {
		t.Fatalf("expected type alias to populate TaskType, got %#v", proposal.ProposedTasks[0])
	}
}

func TestNextUnappliedApprovedPreviewPrefersNewestArtifact(t *testing.T) {
	olderPreviewJSON, err := json.Marshal(model.ApprovedRunPreview{
		Phase:    "stories",
		Format:   workerpkg.PreviewFormatJSON,
		PanelKey: "story_plan",
		Content:  json.RawMessage(`{"summary":"older","proposed_stories":[]}`),
	})
	if err != nil {
		t.Fatalf("marshal older preview: %v", err)
	}
	newerPreviewJSON, err := json.Marshal(model.ApprovedRunPreview{
		Phase:    "stories",
		Format:   workerpkg.PreviewFormatJSON,
		PanelKey: "story_plan",
		Content:  json.RawMessage(`{"summary":"newer","proposed_stories":[]}`),
	})
	if err != nil {
		t.Fatalf("marshal newer preview: %v", err)
	}

	artifacts := []model.AgentRunArtifact{
		{
			ID:            "approved-older",
			ArtifactType:  model.AgentRunArtifactTypeApprovedPreview,
			InlineContent: strPtr(string(olderPreviewJSON)),
			SequenceNo:    1,
		},
		{
			ID:            "approved-newer",
			ArtifactType:  model.AgentRunArtifactTypeApprovedPreview,
			InlineContent: strPtr(string(newerPreviewJSON)),
			SequenceNo:    2,
		},
	}

	artifact, preview, err := nextUnappliedApprovedPreview(artifacts)
	if err != nil {
		t.Fatalf("nextUnappliedApprovedPreview returned error: %v", err)
	}
	if artifact == nil || preview == nil {
		t.Fatal("expected newest approved preview")
	}
	if artifact.ID != "approved-newer" {
		t.Fatalf("expected newest artifact, got %q", artifact.ID)
	}
	var content map[string]any
	if err := json.Unmarshal(preview.Content, &content); err != nil {
		t.Fatalf("unmarshal preview content: %v", err)
	}
	if got, _ := content["summary"].(string); got != "newer" {
		t.Fatalf("expected newest preview content, got %#v", content)
	}
}

func TestDecodeApprovedStoryPlanPreviewContentToleratesOptionalFieldTypeMismatches(t *testing.T) {
	raw := json.RawMessage(`{
		"summary":"Breakdown",
		"proposed_stories":[{
			"ref":"story_1",
			"name":"Story A",
			"description":"Do A",
			"story_type":"feature",
			"estimate":"3",
			"acceptance_criteria":["works"],
			"implementation_brief":{
				"approach":"Add the metric helper",
				"files_to_modify":[{"path":"a.go","action":"modify","description":"update helper"}],
				"test_strategy":{"kind":"regression","owner":"qa"}
			}
		}]
	}`)

	proposal, err := decodeApprovedTaskPlanPreviewContent(raw)
	if err != nil {
		t.Fatalf("decodeApprovedTaskPlanPreviewContent returned error: %v", err)
	}
	if len(proposal.ProposedTasks) != 1 {
		t.Fatalf("expected one proposed story, got %#v", proposal.ProposedTasks)
	}
	if proposal.ProposedTasks[0].Estimate == nil || *proposal.ProposedTasks[0].Estimate != 3 {
		t.Fatalf("expected string estimate to decode to 3, got %#v", proposal.ProposedTasks[0].Estimate)
	}
	brief := proposal.ProposedTasks[0].ImplementationBrief
	if brief == nil {
		t.Fatalf("expected implementation brief, got %#v", proposal.ProposedTasks[0])
	}
	if brief.TestStrategy != `{"kind":"regression","owner":"qa"}` {
		t.Fatalf("expected compact JSON test strategy, got %q", brief.TestStrategy)
	}
}

func TestDecodeApprovedStoryPlanPreviewContentReturnsCanonicalShapeError(t *testing.T) {
	raw := json.RawMessage(`{"summary":"Breakdown"}`)

	_, err := decodeApprovedTaskPlanPreviewContent(raw)
	if err == nil {
		t.Fatal("expected decode error")
	}
	if !strings.Contains(err.Error(), "canonical task-plan shape {summary, proposed_tasks}") {
		t.Fatalf("expected canonical-shape error, got %v", err)
	}
}

func TestDecodeApprovedMarkdownPreviewContentReturnsRepairOrientedError(t *testing.T) {
	_, err := decodeApprovedMarkdownPreviewContent(json.RawMessage(`{"content":"not-a-string"}`), "approved story planning document preview")
	if err == nil {
		t.Fatal("expected markdown decode error")
	}
	if !strings.Contains(err.Error(), "approved story planning document preview content must be a markdown string") {
		t.Fatalf("expected repair-oriented markdown error, got %v", err)
	}
}

func TestRenderProductSpecMarkdownAppendsNormalizedResearchSources(t *testing.T) {
	rendered := renderProductSpecMarkdown("# Problem\n\nBase spec", []model.PlanningResearchSource{
		{Title: "NIST", URL: "https://example.com/nist", Note: "security baseline"},
		{Title: "NIST duplicate", URL: "https://example.com/nist"},
		{Title: "WCAG", URL: "https://example.com/wcag", PublishedAt: "2025-01-01"},
	})

	if !strings.Contains(rendered, "## Research Sources") {
		t.Fatalf("expected research sources section, got %q", rendered)
	}
	if strings.Count(rendered, "https://example.com/nist") != 1 {
		t.Fatalf("expected duplicate source URLs to be de-duplicated, got %q", rendered)
	}
	if !strings.Contains(rendered, "published 2025-01-01") {
		t.Fatalf("expected published date note in rendered markdown, got %q", rendered)
	}
}

func TestBuildDraftSpecClarificationsIncludesQuestionsAndAssumptions(t *testing.T) {
	items := buildDraftSpecClarifications(model.ProductSpecDraft{
		Assumptions:   []string{"Launch for paid plans only"},
		OpenQuestions: []string{"Who gets access on day one?"},
	})

	if len(items) != 2 {
		t.Fatalf("expected 2 clarifications, got %#v", items)
	}
	if items[0].Kind != model.SpecClarificationKindOpenQuestion {
		t.Fatalf("expected first clarification to be an open question, got %#v", items[0])
	}
	if items[1].Kind != model.SpecClarificationKindAssumption {
		t.Fatalf("expected second clarification to be an assumption, got %#v", items[1])
	}
}

func TestResolveExecutionWaitState(t *testing.T) {
	baseRun := &model.AgentRun{
		InvocationMode: model.InvocationModeInteractive,
		TargetType:     "epic",
		ApprovalState:  "not_required",
	}

	waitForApproval, waitForInput, waitForAuth := resolveExecutionWaitState(baseRun, nil, nil, nil, nil)
	if waitForApproval || waitForInput || waitForAuth {
		t.Fatalf("expected no wait state without interaction tools, got approval=%v input=%v auth=%v", waitForApproval, waitForInput, waitForAuth)
	}

	waitForApproval, waitForInput, waitForAuth = resolveExecutionWaitState(baseRun, &workerpkg.UserInputRequest{
		Questions: []workerpkg.UserInputQuestion{{ID: "q1", Question: "Who is this for?", Options: []workerpkg.UserInputOption{{Label: "A"}}}},
	}, nil, nil, nil)
	if waitForApproval || !waitForInput || waitForAuth {
		t.Fatalf("expected human input tool to pause for input, got approval=%v input=%v auth=%v", waitForApproval, waitForInput, waitForAuth)
	}

	waitForApproval, waitForInput, waitForAuth = resolveExecutionWaitState(baseRun, nil, &model.ApprovalRequest{
		Phase: "prd",
		Title: "Approve PRD",
	}, nil, nil)
	if !waitForApproval || waitForInput || waitForAuth {
		t.Fatalf("expected inline approval tool to pause for approval, got approval=%v input=%v auth=%v", waitForApproval, waitForInput, waitForAuth)
	}

	waitForApproval, waitForInput, waitForAuth = resolveExecutionWaitState(&model.AgentRun{
		InvocationMode: model.InvocationModeInteractive,
		TargetType:     "epic",
		ApprovalState:  "pending",
	}, nil, nil, nil, nil)
	if !waitForApproval || waitForInput || waitForAuth {
		t.Fatalf("expected pending approval state to wait for approval, got approval=%v input=%v auth=%v", waitForApproval, waitForInput, waitForAuth)
	}

	waitForApproval, waitForInput, waitForAuth = resolveExecutionWaitState(&model.AgentRun{
		InvocationMode: model.InvocationModeAutonomous,
		TargetType:     "story",
		ApprovalState:  "not_required",
	}, &workerpkg.UserInputRequest{
		Questions: []workerpkg.UserInputQuestion{{ID: "q1", Question: "Pick one", Options: []workerpkg.UserInputOption{{Label: "A"}}}},
	}, nil, nil, nil)
	if waitForApproval || !waitForInput || waitForAuth {
		t.Fatalf("expected autonomous human input tool to pause for input, got approval=%v input=%v auth=%v", waitForApproval, waitForInput, waitForAuth)
	}

	waitForApproval, waitForInput, waitForAuth = resolveExecutionWaitState(&model.AgentRun{
		InvocationMode: model.InvocationModeAutonomous,
		TargetType:     "story",
		ApprovalState:  "not_required",
	}, nil, &model.ApprovalRequest{
		Phase: "command_execution",
		Title: "Approve command",
	}, nil, nil)
	if !waitForApproval || waitForInput || waitForAuth {
		t.Fatalf("expected autonomous approval tool to pause for approval, got approval=%v input=%v auth=%v", waitForApproval, waitForInput, waitForAuth)
	}

	waitForApproval, waitForInput, waitForAuth = resolveExecutionWaitState(baseRun, nil, nil, nil, &model.CodexAuthState{
		State: model.CodexAuthStateRequired,
	})
	if waitForApproval || waitForInput || !waitForAuth {
		t.Fatalf("expected codex auth state to pause for authentication, got approval=%v input=%v auth=%v", waitForApproval, waitForInput, waitForAuth)
	}
}

func TestNormalizeApprovalStateAfterExecution(t *testing.T) {
	run := &model.AgentRun{ApprovalState: "not_required"}
	normalizeApprovalStateAfterExecution(run, true)
	if run.ApprovalState != "pending" {
		t.Fatalf("expected approval_state pending when waiting for approval, got %q", run.ApprovalState)
	}

	run = &model.AgentRun{ApprovalState: "rejected"}
	normalizeApprovalStateAfterExecution(run, false)
	if run.ApprovalState != "not_required" {
		t.Fatalf("expected approval_state to reset after non-approval pause, got %q", run.ApprovalState)
	}

	run = &model.AgentRun{ApprovalState: "approved"}
	normalizeApprovalStateAfterExecution(run, false)
	if run.ApprovalState != "approved" {
		t.Fatalf("expected approved state to be preserved, got %q", run.ApprovalState)
	}
}

func TestFinalRoundToolMessages(t *testing.T) {
	messages := []workerpkg.ExecutionMessage{
		{Role: "user", Content: "Initial prompt"},
		{Role: "assistant", Content: "Need clarification", Blocks: []workerpkg.ExecutionBlock{{Type: workerpkg.ExecutionBlockTypeToolCall, ToolCallID: "tool-1", ToolName: workerpkg.ToolRequestHumanInput}}},
		{Role: "tool", Content: `{"status":"paused","pause_reason":"human_input"}`, Blocks: []workerpkg.ExecutionBlock{{Type: workerpkg.ExecutionBlockTypeToolResult, ToolCallID: "tool-1", ToolName: workerpkg.ToolRequestHumanInput, Output: `{"status":"paused","pause_reason":"human_input"}`}}},
	}

	results := finalRoundToolMessages(messages)
	if len(results) != 1 {
		t.Fatalf("expected 1 trailing tool message, got %#v", results)
	}
	if results[0].Role != "tool" || results[0].Content != `{"status":"paused","pause_reason":"human_input"}` {
		t.Fatalf("unexpected tool message %#v", results[0])
	}
}

func TestFinalRoundToolMessagesUsesLastAssistantBoundary(t *testing.T) {
	messages := []workerpkg.ExecutionMessage{
		{Role: "user", Content: "Initial prompt"},
		{Role: "assistant", Content: "First round", Blocks: []workerpkg.ExecutionBlock{{Type: workerpkg.ExecutionBlockTypeToolCall, ToolCallID: "tool-1", ToolName: "read_file"}}},
		{Role: "tool", Content: "first result", Blocks: []workerpkg.ExecutionBlock{{Type: workerpkg.ExecutionBlockTypeToolResult, ToolCallID: "tool-1", ToolName: "read_file", Output: "first result"}}},
		{Role: "assistant", Content: "Second round", Blocks: []workerpkg.ExecutionBlock{{Type: workerpkg.ExecutionBlockTypeToolCall, ToolCallID: "tool-2", ToolName: workerpkg.ToolRequestHumanInput}}},
		{Role: "tool", Content: "second result", Blocks: []workerpkg.ExecutionBlock{{Type: workerpkg.ExecutionBlockTypeToolResult, ToolCallID: "tool-2", ToolName: workerpkg.ToolRequestHumanInput, Output: "second result"}}},
	}

	results := finalRoundToolMessages(messages)
	if len(results) != 1 {
		t.Fatalf("expected only final round tool results, got %#v", results)
	}
	if results[0].Content != "second result" {
		t.Fatalf("expected trailing tool result only, got %#v", results[0])
	}
}

func TestBuildPersistedAssistantRunMessageUsesCanonicalBlocksAndInvocations(t *testing.T) {
	result := &workerpkg.ExecutionResult{
		AssistantBlocks: []workerpkg.ExecutionBlock{
			{Type: workerpkg.ExecutionBlockTypeText, Text: "Planned update"},
			{Type: workerpkg.ExecutionBlockTypeToolCall, ToolCallID: "call-1", ToolName: "read_file", Input: json.RawMessage(`{"path":"a.go"}`)},
		},
		ToolInvocations: []model.ToolInvocation{
			{ToolName: "read_file", Input: json.RawMessage(`{"path":"a.go"}`), OutputSummary: "package main", DurationMs: 12},
		},
		Usage: workerpkg.ExecutionUsage{CachedInputTokens: 3, InputTokens: 11, OutputTokens: 7},
	}

	message, err := buildPersistedAssistantRunMessage(result, &model.CodingSessionStreamSnapshot{
		LiveTurnSegments: []model.CodingSessionLiveTurnSegment{
			{
				SegmentID: "assistant-1:segment:1",
				Kind:      "assistant_message",
				AssistantMessage: &model.CodingSessionLiveAssistantMessage{
					MessageID: "assistant-1",
					Content:   "Planned update",
					Status:    "completed",
				},
			},
			{
				SegmentID: "call-1",
				Kind:      "tool_call",
				ToolCall: &model.CodingSessionLiveToolCall{
					ToolCallID: "call-1",
					ToolName:   "read_file",
					Status:     "completed",
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("buildPersistedAssistantRunMessage returned error: %v", err)
	}
	if message == nil {
		t.Fatal("expected persisted assistant message")
	}
	if message.Role != "assistant" || message.MessageType != "assistant_turn" {
		t.Fatalf("unexpected assistant message envelope %#v", message)
	}
	if message.Content != "Planned update" {
		t.Fatalf("expected content derived from text block, got %#v", message.Content)
	}
	if len(message.ContentBlocks) == 0 || len(message.TurnSegments) == 0 || len(message.ToolInvocations) == 0 || len(message.TokenUsage) == 0 {
		t.Fatalf("expected canonical persisted payloads, got %#v", message)
	}
	if string(message.TokenUsage) != `{"cached_input_tokens":3,"input_tokens":11,"output_tokens":7}` {
		t.Fatalf("expected token usage payload, got %s", string(message.TokenUsage))
	}
}

func TestBuildPersistedToolResultMessagesDerivesContentFromBlocks(t *testing.T) {
	messages := []workerpkg.ExecutionMessage{
		{Role: "assistant", Content: "Need tool", Blocks: []workerpkg.ExecutionBlock{{Type: workerpkg.ExecutionBlockTypeToolCall, ToolCallID: "call-1", ToolName: "read_file"}}},
		{Role: "tool", Blocks: []workerpkg.ExecutionBlock{{Type: workerpkg.ExecutionBlockTypeToolResult, ToolCallID: "call-1", ToolName: "read_file", Output: "package main"}}},
	}

	results := buildPersistedToolResultMessages(messages)
	if len(results) != 1 {
		t.Fatalf("expected one persisted tool result, got %#v", results)
	}
	if results[0].Content != "package main" {
		t.Fatalf("expected content derived from tool result block, got %#v", results[0])
	}
	if results[0].Role != "tool" || results[0].MessageType != "tool_result" || len(results[0].ContentBlocks) == 0 {
		t.Fatalf("unexpected persisted tool result %#v", results[0])
	}
}

func TestPersistAssistantRunMessageWritesInteractionArtifacts(t *testing.T) {
	dbName := fmt.Sprintf("file:interaction-artifacts-%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dbName), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	for _, stmt := range []string{
		`CREATE TABLE agent_run_messages (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			run_id TEXT NOT NULL,
			role TEXT NOT NULL,
			content TEXT NOT NULL,
			message_type TEXT NOT NULL,
			content_blocks BLOB,
			turn_segments BLOB,
			tool_invocations BLOB,
			token_usage BLOB,
			sequence_no INTEGER NOT NULL,
			created_at DATETIME
		)`,
		`CREATE TABLE agent_run_artifacts (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			run_id TEXT NOT NULL,
			artifact_type TEXT NOT NULL,
			format TEXT NOT NULL,
			storage_mode TEXT NOT NULL,
			inline_content TEXT,
			object_key TEXT,
			metadata TEXT NOT NULL,
			sequence_no INTEGER NOT NULL,
			created_at DATETIME
		)`,
	} {
		if err := db.Exec(stmt).Error; err != nil {
			t.Fatalf("create table: %v", err)
		}
	}

	runMessageRepo := repository.NewAgentRunMessageRepository(db)
	artifactRepo := repository.NewAgentRunArtifactRepository(db)
	activities := &AgentRunActivities{runMessageRepo: runMessageRepo, artifactRepo: artifactRepo}
	run := &model.AgentRun{ID: "run-1", WorkspaceID: "ws-1"}
	state := &resolvedRunState{run: run}
	execCtx := &workerpkg.ExecutionContext{
		LastExecutionResult: &workerpkg.ExecutionResult{
			AssistantBlocks: []workerpkg.ExecutionBlock{{Type: workerpkg.ExecutionBlockTypeText, Text: "Need review"}},
			ToolInvocations: []model.ToolInvocation{
				{
					ToolName: workerpkg.ToolRequestHumanInput,
					Input: json.RawMessage(`{
						"questions":[{"id":"q1","type":"single_select","text":"Pick one","options":[{"value":"a","label":"A"}]}]
					}`),
				},
				{
					ToolName: workerpkg.ToolRequestHumanApproval,
					Input:    json.RawMessage(`{"phase":"prd","title":"Approve PRD","summary":"Review the draft"}`),
				},
			},
		},
	}

	assistantMessage, err := activities.persistAssistantRunMessage(context.Background(), state, execCtx)
	if err != nil {
		t.Fatalf("persistAssistantRunMessage returned error: %v", err)
	}
	if assistantMessage == nil {
		t.Fatal("expected assistant message to be persisted")
	}

	artifacts, err := artifactRepo.ListByRun(context.Background(), run.WorkspaceID, run.ID)
	if err != nil {
		t.Fatalf("list artifacts: %v", err)
	}
	if len(artifacts) != 2 {
		t.Fatalf("expected 2 interaction artifacts, got %#v", artifacts)
	}
	types := map[string]model.AgentRunArtifact{}
	for _, artifact := range artifacts {
		types[artifact.ArtifactType] = artifact
	}
	for _, artifactType := range []string{model.AgentRunArtifactTypeHumanInputRequest, model.AgentRunArtifactTypeHumanApprovalRequest} {
		artifact, ok := types[artifactType]
		if !ok {
			t.Fatalf("missing artifact type %q in %#v", artifactType, artifacts)
		}
		var metadata map[string]any
		if err := json.Unmarshal(artifact.Metadata, &metadata); err != nil {
			t.Fatalf("unmarshal metadata: %v", err)
		}
		if got := metadata["assistant_message_sequence_no"]; got != float64(assistantMessage.SequenceNo) {
			t.Fatalf("expected assistant sequence metadata on %q, got %#v", artifactType, metadata)
		}
	}
}

func TestHandleLiveCodexInteractivePauseIgnoresOlderRepliesWhenNoAssistantMessageIsPersisted(t *testing.T) {
	previousPollEvery := codexLivePausePollEvery
	codexLivePausePollEvery = 10 * time.Millisecond
	defer func() {
		codexLivePausePollEvery = previousPollEvery
	}()

	dbName := fmt.Sprintf("file:live-codex-pause-%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dbName), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	for _, stmt := range []string{
		`CREATE TABLE agent_runs (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			agent_id TEXT NOT NULL,
			task_id TEXT,
			conversation_id TEXT,
			target_type TEXT NOT NULL DEFAULT 'story',
			target_id TEXT NOT NULL,
			runtime_kind TEXT NOT NULL DEFAULT 'opencode',
			invocation_mode TEXT NOT NULL DEFAULT 'autonomous',
			parent_run_id TEXT,
			handoff_state TEXT,
			approval_state TEXT NOT NULL DEFAULT 'not_required',
			pause_reason TEXT NOT NULL DEFAULT 'none',
			triggered_by_user_id TEXT,
			status TEXT NOT NULL DEFAULT 'queued',
			workflow_id TEXT,
			workflow_run_id TEXT,
			task_queue TEXT,
			runner_pool TEXT,
			repository_id TEXT,
			repo_full_name TEXT,
			base_branch TEXT,
			working_branch TEXT,
			delivery_target_id TEXT,
			execution_stage TEXT,
			last_heartbeat_at DATETIME,
			input TEXT NOT NULL DEFAULT '{}',
			output_summary TEXT NOT NULL DEFAULT '{}',
			cached_input_tokens INTEGER NOT NULL DEFAULT 0,
			input_tokens INTEGER NOT NULL DEFAULT 0,
			output_tokens INTEGER NOT NULL DEFAULT 0,
			tokens_used INTEGER NOT NULL DEFAULT 0,
			error_message TEXT,
			started_at DATETIME,
			completed_at DATETIME,
			created_at DATETIME,
			updated_at DATETIME
		)`,
		`CREATE TABLE agent_run_messages (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			run_id TEXT NOT NULL,
			role TEXT NOT NULL,
			content TEXT NOT NULL,
			message_type TEXT NOT NULL,
			content_blocks BLOB,
			turn_segments BLOB,
			tool_invocations BLOB,
			token_usage BLOB,
			sequence_no INTEGER NOT NULL,
			created_at DATETIME
		)`,
	} {
		if err := db.Exec(stmt).Error; err != nil {
			t.Fatalf("create table: %v", err)
		}
	}

	runRepo := repository.NewAgentRunRepository(db)
	runMessageRepo := repository.NewAgentRunMessageRepository(db)
	activities := &AgentRunActivities{
		runRepo:        runRepo,
		runMessageRepo: runMessageRepo,
	}

	now := time.Now().UTC()
	run := &model.AgentRun{
		ID:             "run-live-codex",
		WorkspaceID:    "ws-1",
		AgentID:        "agent-1",
		TargetType:     "story",
		TargetID:       "story-1",
		RuntimeKind:    "codex",
		InvocationMode: model.InvocationModeInteractive,
		ApprovalState:  "not_required",
		PauseReason:    model.AgentRunPauseReasonNone,
		Status:         model.AgentRunStatusRunning,
		Input:          json.RawMessage(`{}`),
		OutputSummary:  json.RawMessage(`{}`),
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	if err := runRepo.Create(context.Background(), run); err != nil {
		t.Fatalf("create run: %v", err)
	}
	if err := runMessageRepo.Create(context.Background(), &model.AgentRunMessage{
		ID:          "message-old-reply",
		WorkspaceID: run.WorkspaceID,
		RunID:       run.ID,
		Role:        "user",
		Content:     "old reply",
		MessageType: "user_reply",
		SequenceNo:  1,
		CreatedAt:   now,
	}); err != nil {
		t.Fatalf("create old reply: %v", err)
	}

	result := &workerpkg.ExecutionResult{
		ToolInvocations: []model.ToolInvocation{
			{
				ToolName: workerpkg.ToolRequestHumanInput,
				Input:    json.RawMessage(`{"questions":[{"id":"q1","type":"single_select","text":"Need more info","options":[{"value":"a","label":"A","freetext":true}]}]}`),
			},
		},
	}

	type pauseOutcome struct {
		signal *workerpkg.LiveExecutionResumeSignal
		err    error
	}
	outcomeCh := make(chan pauseOutcome, 1)
	go func() {
		signal, err := activities.handleLiveCodexInteractivePause(
			context.Background(),
			&resolvedRunState{run: run},
			&workerpkg.ExecutionContext{},
			result,
			nil,
		)
		outcomeCh <- pauseOutcome{signal: signal, err: err}
	}()

	select {
	case outcome := <-outcomeCh:
		t.Fatalf("pause returned before a new reply was written: signal=%#v err=%v", outcome.signal, outcome.err)
	case <-time.After(50 * time.Millisecond):
	}

	loadedRun, err := runRepo.GetByID(context.Background(), run.WorkspaceID, run.ID)
	if err != nil {
		t.Fatalf("get paused run: %v", err)
	}
	if loadedRun == nil || loadedRun.Status != model.AgentRunStatusPaused {
		t.Fatalf("expected run to be paused while waiting for live input, got %#v", loadedRun)
	}

	if err := runMessageRepo.Create(context.Background(), &model.AgentRunMessage{
		ID:          "message-new-reply",
		WorkspaceID: run.WorkspaceID,
		RunID:       run.ID,
		Role:        "user",
		Content:     "new reply",
		MessageType: "user_reply",
		SequenceNo:  2,
		CreatedAt:   now.Add(time.Second),
	}); err != nil {
		t.Fatalf("create new reply: %v", err)
	}

	select {
	case outcome := <-outcomeCh:
		if outcome.err != nil {
			t.Fatalf("pause returned error: %v", outcome.err)
		}
		if outcome.signal == nil {
			t.Fatal("expected live resume signal")
		}
		if outcome.signal.Intent != model.AgentRunResumeIntentReply {
			t.Fatalf("expected reply intent, got %#v", outcome.signal)
		}
		if outcome.signal.Content != "new reply" {
			t.Fatalf("expected new reply content, got %#v", outcome.signal)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for live codex pause to resume from the new reply")
	}
}

func TestWaitForLiveCodexResumeSignalPrefersResolvedInteractionPayload(t *testing.T) {
	dbName := fmt.Sprintf("file:live-codex-resolved-interaction-%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dbName), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	for _, stmt := range []string{
		`CREATE TABLE agent_runs (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			agent_id TEXT NOT NULL,
			task_id TEXT,
			conversation_id TEXT,
			target_type TEXT NOT NULL DEFAULT 'story',
			target_id TEXT NOT NULL,
			runtime_kind TEXT NOT NULL DEFAULT 'opencode',
			invocation_mode TEXT NOT NULL DEFAULT 'autonomous',
			parent_run_id TEXT,
			handoff_state TEXT,
			approval_state TEXT NOT NULL DEFAULT 'not_required',
			pause_reason TEXT NOT NULL DEFAULT 'none',
			triggered_by_user_id TEXT,
			status TEXT NOT NULL DEFAULT 'queued',
			workflow_id TEXT,
			workflow_run_id TEXT,
			task_queue TEXT,
			runner_pool TEXT,
			repository_id TEXT,
			repo_full_name TEXT,
			base_branch TEXT,
			working_branch TEXT,
			delivery_target_id TEXT,
			execution_stage TEXT,
			last_heartbeat_at DATETIME,
			input TEXT NOT NULL DEFAULT '{}',
			output_summary TEXT NOT NULL DEFAULT '{}',
			cached_input_tokens INTEGER NOT NULL DEFAULT 0,
			input_tokens INTEGER NOT NULL DEFAULT 0,
			output_tokens INTEGER NOT NULL DEFAULT 0,
			tokens_used INTEGER NOT NULL DEFAULT 0,
			error_message TEXT,
			started_at DATETIME,
			completed_at DATETIME,
			created_at DATETIME,
			updated_at DATETIME
		)`,
		`CREATE TABLE agent_run_messages (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			run_id TEXT NOT NULL,
			role TEXT NOT NULL,
			content TEXT NOT NULL,
			message_type TEXT NOT NULL,
			content_blocks BLOB,
			turn_segments BLOB,
			tool_invocations BLOB,
			token_usage BLOB,
			sequence_no INTEGER NOT NULL,
			created_at DATETIME
		)`,
		`CREATE TABLE agent_run_interactions (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			run_id TEXT NOT NULL,
			runtime_kind TEXT NOT NULL,
			interaction_kind TEXT NOT NULL,
			status TEXT NOT NULL,
			request_schema_version TEXT NOT NULL,
			response_schema_version TEXT,
			request_id TEXT,
			thread_id TEXT,
			turn_id TEXT,
			item_id TEXT,
			approval_id TEXT,
			assistant_message_sequence_no INTEGER,
			title TEXT,
			summary TEXT,
			request_payload TEXT NOT NULL,
			response_payload TEXT,
			runtime_metadata TEXT NOT NULL,
			resolved_by TEXT,
			resolved_at DATETIME,
			created_at DATETIME,
			updated_at DATETIME
		)`,
	} {
		if err := db.Exec(stmt).Error; err != nil {
			t.Fatalf("create table: %v", err)
		}
	}

	runRepo := repository.NewAgentRunRepository(db)
	runMessageRepo := repository.NewAgentRunMessageRepository(db)
	interactionRepo := repository.NewAgentRunInteractionRepository(db)
	activities := &AgentRunActivities{
		runRepo:         runRepo,
		runMessageRepo:  runMessageRepo,
		interactionRepo: interactionRepo,
	}

	now := time.Now().UTC()
	run := &model.AgentRun{
		ID:             "run-live-codex-resolved",
		WorkspaceID:    "ws-1",
		AgentID:        "agent-1",
		TargetType:     "story",
		TargetID:       "story-1",
		RuntimeKind:    "codex",
		InvocationMode: model.InvocationModeInteractive,
		ApprovalState:  "pending",
		PauseReason:    model.AgentRunPauseReasonHumanApproval,
		Status:         model.AgentRunStatusPaused,
		Input:          json.RawMessage(`{}`),
		OutputSummary:  json.RawMessage(`{}`),
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	if err := runRepo.Create(context.Background(), run); err != nil {
		t.Fatalf("create run: %v", err)
	}

	responsePayload := json.RawMessage(`{"decision":"acceptForSession"}`)
	resolvedAt := now.Add(time.Second)
	responseSchemaVersion := model.AgentRunInteractionSchemaVersionCodexV2
	assistantSequenceNo := 6
	resolvedBy := "user-1"
	if err := interactionRepo.Create(context.Background(), &model.AgentRunInteraction{
		ID:                         "interaction-resolved-1",
		WorkspaceID:                run.WorkspaceID,
		RunID:                      run.ID,
		RuntimeKind:                "codex",
		InteractionKind:            model.AgentRunInteractionKindCommandExecutionApproval,
		Status:                     model.AgentRunInteractionStatusResolved,
		RequestSchemaVersion:       model.AgentRunInteractionSchemaVersionCodexV2,
		ResponseSchemaVersion:      &responseSchemaVersion,
		AssistantMessageSequenceNo: &assistantSequenceNo,
		RequestPayload:             json.RawMessage(`{"command":"git commit","availableDecisions":["accept","acceptForSession","decline","cancel"]}`),
		ResponsePayload:            responsePayload,
		RuntimeMetadata:            json.RawMessage(`{"runtime_kind":"codex"}`),
		ResolvedBy:                 &resolvedBy,
		ResolvedAt:                 &resolvedAt,
		CreatedAt:                  now,
		UpdatedAt:                  resolvedAt,
	}); err != nil {
		t.Fatalf("create interaction: %v", err)
	}

	signal, err := activities.waitForLiveCodexResumeSignal(context.Background(), run, 5, model.AgentRunPauseReasonHumanApproval)
	if err != nil {
		t.Fatalf("waitForLiveCodexResumeSignal returned error: %v", err)
	}
	if signal == nil {
		t.Fatal("expected live resume signal")
	}
	if signal.Intent != model.AgentRunResumeIntentApprove {
		t.Fatalf("expected approve intent, got %#v", signal)
	}
	if signal.Content != "" {
		t.Fatalf("expected no follow-up message content, got %#v", signal)
	}
	if got := string(signal.ResponsePayload); got != string(responsePayload) {
		t.Fatalf("expected native response payload to be preserved, got %s", got)
	}
}

func TestLoadAndPersistProviderContinuationCheckpoint(t *testing.T) {
	dbName := fmt.Sprintf("file:provider-checkpoint-%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dbName), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	for _, stmt := range []string{
		`CREATE TABLE agent_run_artifacts (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			run_id TEXT NOT NULL,
			artifact_type TEXT NOT NULL,
			format TEXT NOT NULL,
			storage_mode TEXT NOT NULL,
			inline_content TEXT,
			object_key TEXT,
			metadata TEXT NOT NULL,
			sequence_no INTEGER NOT NULL,
			created_at DATETIME
		)`,
	} {
		if err := db.Exec(stmt).Error; err != nil {
			t.Fatalf("create artifact table: %v", err)
		}
	}

	artifactRepo := repository.NewAgentRunArtifactRepository(db)
	activities := &AgentRunActivities{artifactRepo: artifactRepo}
	run := &model.AgentRun{ID: "run-1", WorkspaceID: "ws-1"}
	state := &resolvedRunState{
		run: run,
		agent: &model.Agent{
			Provider: strPtr(model.AgentModelProviderOpenAI),
		},
	}
	assistantMessage := &model.AgentRunMessage{SequenceNo: 7}
	result := &workerpkg.ExecutionResult{
		ProviderContinuation: &workerpkg.ProviderContinuation{
			Provider:           model.AgentModelProviderOpenAI,
			ResponseID:         "resp_123",
			PreviousResponseID: "resp_122",
		},
	}

	if err := activities.persistProviderResponseCheckpoint(context.Background(), state, result, assistantMessage); err != nil {
		t.Fatalf("persistProviderResponseCheckpoint returned error: %v", err)
	}

	loaded, err := activities.loadProviderContinuation(context.Background(), state)
	if err != nil {
		t.Fatalf("loadProviderContinuation returned error: %v", err)
	}
	if loaded == nil {
		t.Fatal("expected loaded continuation")
	}
	if loaded.Provider != model.AgentModelProviderOpenAI || loaded.ResponseID != "resp_123" || loaded.PreviousResponseID != "resp_122" || loaded.AfterSequenceNo != 7 {
		t.Fatalf("unexpected loaded continuation %#v", loaded)
	}

	artifacts, err := artifactRepo.ListByRun(context.Background(), run.WorkspaceID, run.ID)
	if err != nil {
		t.Fatalf("list artifacts: %v", err)
	}
	if len(artifacts) != 1 {
		t.Fatalf("expected one checkpoint artifact, got %#v", artifacts)
	}
	var metadata map[string]any
	if err := json.Unmarshal(artifacts[0].Metadata, &metadata); err != nil {
		t.Fatalf("unmarshal metadata: %v", err)
	}
	if got := metadata["assistant_message_sequence_no"]; got != float64(7) {
		t.Fatalf("expected assistant_message_sequence_no metadata, got %#v", metadata)
	}
}

func TestPersistHumanInteractionArtifactsDualWritesCodexApprovalInteraction(t *testing.T) {
	dbName := fmt.Sprintf("file:interaction-dual-write-%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dbName), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	for _, stmt := range []string{
		`CREATE TABLE agent_run_artifacts (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			run_id TEXT NOT NULL,
			artifact_type TEXT NOT NULL,
			format TEXT NOT NULL,
			storage_mode TEXT NOT NULL,
			inline_content TEXT,
			object_key TEXT,
			metadata TEXT NOT NULL,
			sequence_no INTEGER NOT NULL,
			created_at DATETIME
		)`,
		`CREATE TABLE agent_run_interactions (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			run_id TEXT NOT NULL,
			runtime_kind TEXT NOT NULL,
			interaction_kind TEXT NOT NULL,
			status TEXT NOT NULL,
			request_schema_version TEXT NOT NULL,
			response_schema_version TEXT,
			request_id TEXT,
			thread_id TEXT,
			turn_id TEXT,
			item_id TEXT,
			approval_id TEXT,
			assistant_message_sequence_no INTEGER,
			title TEXT,
			summary TEXT,
			request_payload TEXT NOT NULL,
			response_payload TEXT,
			runtime_metadata TEXT NOT NULL,
			resolved_by TEXT,
			resolved_at DATETIME,
			created_at DATETIME,
			updated_at DATETIME
		)`,
	} {
		if err := db.Exec(stmt).Error; err != nil {
			t.Fatalf("create test table: %v", err)
		}
	}

	artifactRepo := repository.NewAgentRunArtifactRepository(db)
	interactionRepo := repository.NewAgentRunInteractionRepository(db)
	wsPublisher := &capturedEventPublisher{}
	activities := &AgentRunActivities{artifactRepo: artifactRepo, interactionRepo: interactionRepo, wsPublisher: wsPublisher}

	run := &model.AgentRun{ID: "run-1", WorkspaceID: "ws-1", RuntimeKind: "codex"}
	state := &resolvedRunState{
		run:   run,
		agent: &model.Agent{ID: "agent-1", WorkspaceID: "ws-1", RuntimeKind: "codex"},
	}
	assistantMessage := &model.AgentRunMessage{SequenceNo: 5}

	approvalInput, err := json.Marshal(workerpkg.HumanApprovalRequest{
		Phase:   "command_execution",
		Title:   "Approve command execution",
		Summary: "Command: go test ./...",
	})
	if err != nil {
		t.Fatalf("marshal approval input: %v", err)
	}

	nativeRequestPayload := json.RawMessage(`{
		"threadId":"thread-1",
		"turnId":"turn-1",
		"itemId":"item-1",
		"approvalId":"approval-1",
		"command":"go test ./..."
	}`)
	approvalMetadata, err := json.Marshal(map[string]any{
		"runtime_kind":          "codex",
		"codex_request_kind":    "command_execution",
		"codex_request_id":      "7",
		"codex_thread_id":       "thread-1",
		"codex_turn_id":         "turn-1",
		"codex_item_id":         "item-1",
		"codex_approval_id":     "approval-1",
		"codex_request_payload": nativeRequestPayload,
	})
	if err != nil {
		t.Fatalf("marshal approval metadata: %v", err)
	}

	result := &workerpkg.ExecutionResult{
		ToolInvocations: []model.ToolInvocation{{
			ToolName: workerpkg.ToolRequestHumanApproval,
			Input:    approvalInput,
		}},
		HumanApprovalMetadata: approvalMetadata,
	}

	if err := activities.persistHumanInteractionArtifacts(context.Background(), state, result, assistantMessage); err != nil {
		t.Fatalf("persistHumanInteractionArtifacts returned error: %v", err)
	}

	artifacts, err := artifactRepo.ListByRun(context.Background(), run.WorkspaceID, run.ID)
	if err != nil {
		t.Fatalf("list artifacts: %v", err)
	}
	if len(artifacts) != 1 || artifacts[0].ArtifactType != model.AgentRunArtifactTypeHumanApprovalRequest {
		t.Fatalf("expected legacy approval artifact, got %#v", artifacts)
	}

	interactions, err := interactionRepo.ListByRun(context.Background(), run.WorkspaceID, run.ID)
	if err != nil {
		t.Fatalf("list interactions: %v", err)
	}
	if len(interactions) != 1 {
		t.Fatalf("expected one interaction, got %#v", interactions)
	}
	interaction := interactions[0]
	if interaction.InteractionKind != model.AgentRunInteractionKindCommandExecutionApproval {
		t.Fatalf("expected command execution interaction, got %#v", interaction)
	}
	if interaction.RequestSchemaVersion != model.AgentRunInteractionSchemaVersionCodexV2 {
		t.Fatalf("expected codex request schema version, got %#v", interaction)
	}
	if interaction.RequestID == nil || *interaction.RequestID != "7" {
		t.Fatalf("expected request id to be preserved, got %#v", interaction.RequestID)
	}
	if interaction.ApprovalID == nil || *interaction.ApprovalID != "approval-1" {
		t.Fatalf("expected approval id to be preserved, got %#v", interaction.ApprovalID)
	}
	if interaction.AssistantMessageSequenceNo == nil || *interaction.AssistantMessageSequenceNo != 5 {
		t.Fatalf("expected assistant sequence linkage, got %#v", interaction.AssistantMessageSequenceNo)
	}
	var requestPayload map[string]any
	if err := json.Unmarshal(interaction.RequestPayload, &requestPayload); err != nil {
		t.Fatalf("unmarshal request payload: %v", err)
	}
	if got, _ := requestPayload["threadId"].(string); got != "thread-1" {
		t.Fatalf("expected native codex request payload, got %#v", requestPayload)
	}
	if len(wsPublisher.events) == 0 {
		t.Fatal("expected coding session interaction event to be published")
	}
	var event model.CodingSessionEvent
	if err := json.Unmarshal(wsPublisher.events[0].Data, &event); err != nil {
		t.Fatalf("unmarshal published event: %v", err)
	}
	if event.Type != "interaction.requested" {
		t.Fatalf("expected interaction.requested event, got %#v", event)
	}
	if _, ok := event.Payload["interaction_id"].(string); !ok {
		t.Fatalf("expected interaction payload, got %#v", event.Payload)
	}
}

func TestEnsureRunConversationCreatesPromptWhenOnlyStatusMessageExists(t *testing.T) {
	dbName := fmt.Sprintf("file:run-conversation-%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dbName), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	for _, stmt := range []string{
		`CREATE TABLE agent_run_messages (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			run_id TEXT NOT NULL,
			role TEXT NOT NULL,
			content TEXT NOT NULL,
			message_type TEXT NOT NULL,
			content_blocks BLOB,
			turn_segments BLOB,
			tool_invocations BLOB,
			token_usage BLOB,
			sequence_no INTEGER NOT NULL,
			created_at DATETIME
		)`,
	} {
		if err := db.Exec(stmt).Error; err != nil {
			t.Fatalf("create message table: %v", err)
		}
	}

	runMessageRepo := repository.NewAgentRunMessageRepository(db)
	activities := &AgentRunActivities{runMessageRepo: runMessageRepo}
	run := &model.AgentRun{ID: "run-1", WorkspaceID: "ws-1"}
	state := &resolvedRunState{run: run}

	if _, err := activities.createRunMessage(context.Background(), run, "assistant", "status", "Preparing workspace and loading run context.", nil, nil, nil, nil); err != nil {
		t.Fatalf("create status message: %v", err)
	}

	history, _, _, _, err := activities.ensureRunConversation(context.Background(), state, "Operator notes:\nFocus on setup.", planningRunInput{})
	if err != nil {
		t.Fatalf("ensureRunConversation returned error: %v", err)
	}
	if len(history) != 1 {
		t.Fatalf("expected only prompt history entry, got %#v", history)
	}
	if history[0].Role != "user" || !strings.Contains(history[0].Content, "Operator notes:") {
		t.Fatalf("unexpected execution history %#v", history[0])
	}

	messages, err := runMessageRepo.ListByRun(context.Background(), run.WorkspaceID, run.ID)
	if err != nil {
		t.Fatalf("list messages: %v", err)
	}
	if len(messages) != 2 {
		t.Fatalf("expected status + prompt messages, got %#v", messages)
	}
	if messages[0].MessageType != "status" || messages[1].MessageType != "prompt" {
		t.Fatalf("unexpected message types %#v", messages)
	}
}

func TestPublishRunStreamEventPersistsAndCreateRunMessageClearsCodingSessionSnapshot(t *testing.T) {
	dbName := fmt.Sprintf("file:run-stream-snapshot-%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dbName), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	for _, stmt := range []string{
		`CREATE TABLE agent_run_messages (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			run_id TEXT NOT NULL,
			role TEXT NOT NULL,
			content TEXT NOT NULL,
			message_type TEXT NOT NULL,
			content_blocks BLOB,
			turn_segments BLOB,
			tool_invocations BLOB,
			token_usage BLOB,
			sequence_no INTEGER NOT NULL,
			created_at DATETIME
		)`,
		`CREATE TABLE coding_session_state_snapshots (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			run_id TEXT NOT NULL,
			schema_version TEXT NOT NULL,
			snapshot_payload BLOB NOT NULL DEFAULT (CAST('{}' AS BLOB)),
			created_at DATETIME,
			updated_at DATETIME,
			UNIQUE(workspace_id, run_id)
		)`,
	} {
		if err := db.Exec(stmt).Error; err != nil {
			t.Fatalf("create table: %v", err)
		}
	}

	runMessageRepo := repository.NewAgentRunMessageRepository(db)
	snapshotRepo := repository.NewCodingSessionStateSnapshotRepository(db)
	activities := &AgentRunActivities{
		runMessageRepo:      runMessageRepo,
		sessionSnapshotRepo: snapshotRepo,
	}
	run := &model.AgentRun{ID: "run-1", WorkspaceID: "ws-1"}

	activities.publishRunStreamEvent(run, workerpkg.ExecutionEvent{
		Type:      "assistant_message_started",
		MessageID: "assistant-live-1",
	})
	activities.publishRunStreamEvent(run, workerpkg.ExecutionEvent{
		Type:      "assistant_message_delta",
		MessageID: "assistant-live-1",
		Text:      "Inspecting workspace",
	})

	record, err := snapshotRepo.GetByRun(context.Background(), run.WorkspaceID, run.ID)
	if err != nil {
		t.Fatalf("get snapshot: %v", err)
	}
	if record == nil {
		t.Fatal("expected coding session snapshot to be persisted")
	}
	snapshot, err := model.DecodeCodingSessionStreamSnapshot(record.SnapshotPayload)
	if err != nil {
		t.Fatalf("decode snapshot: %v", err)
	}
	if snapshot == nil || snapshot.LiveAssistantMessage == nil || snapshot.LiveAssistantMessage.Content != "Inspecting workspace" {
		t.Fatalf("unexpected persisted snapshot %#v", snapshot)
	}
	if len(snapshot.LiveTurnSegments) != 1 || snapshot.LiveTurnSegments[0].Kind != "assistant_message" || snapshot.LiveTurnSegments[0].AssistantMessage == nil || snapshot.LiveTurnSegments[0].AssistantMessage.Content != "Inspecting workspace" {
		t.Fatalf("unexpected live turn segments %#v", snapshot.LiveTurnSegments)
	}

	if _, err := activities.createRunMessage(context.Background(), run, "assistant", "assistant_turn", "Inspecting workspace", nil, nil, nil, nil); err != nil {
		t.Fatalf("create assistant turn: %v", err)
	}

	record, err = snapshotRepo.GetByRun(context.Background(), run.WorkspaceID, run.ID)
	if err != nil {
		t.Fatalf("get snapshot after clear: %v", err)
	}
	if record != nil {
		t.Fatalf("expected snapshot to be cleared after persisted assistant turn, got %#v", record)
	}
}

func TestEnsureRunConversationBuildsTranscriptSummaryCheckpoint(t *testing.T) {
	dbName := fmt.Sprintf("file:run-summary-%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dbName), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	for _, stmt := range []string{
		`CREATE TABLE agent_run_messages (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			run_id TEXT NOT NULL,
			role TEXT NOT NULL,
			content TEXT NOT NULL,
			message_type TEXT NOT NULL,
			content_blocks BLOB,
			turn_segments BLOB,
			tool_invocations BLOB,
			token_usage BLOB,
			sequence_no INTEGER NOT NULL,
			created_at DATETIME
		)`,
		`CREATE TABLE agent_run_artifacts (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			run_id TEXT NOT NULL,
			artifact_type TEXT NOT NULL,
			format TEXT NOT NULL,
			storage_mode TEXT NOT NULL,
			inline_content TEXT,
			object_key TEXT,
			metadata TEXT NOT NULL,
			sequence_no INTEGER NOT NULL,
			created_at DATETIME
		)`,
	} {
		if err := db.Exec(stmt).Error; err != nil {
			t.Fatalf("create table: %v", err)
		}
	}

	runMessageRepo := repository.NewAgentRunMessageRepository(db)
	artifactRepo := repository.NewAgentRunArtifactRepository(db)
	activities := &AgentRunActivities{runMessageRepo: runMessageRepo, artifactRepo: artifactRepo}
	run := &model.AgentRun{ID: "run-1", WorkspaceID: "ws-1"}
	state := &resolvedRunState{run: run}

	for i := 0; i < 13; i++ {
		role := "assistant"
		messageType := "assistant_turn"
		content := fmt.Sprintf("Assistant update %d", i)
		if i%2 == 0 {
			role = "user"
			messageType = "prompt"
			content = fmt.Sprintf("User request %d", i)
		}
		if _, err := activities.createRunMessage(context.Background(), run, role, messageType, content, nil, nil, nil, nil); err != nil {
			t.Fatalf("create message %d: %v", i, err)
		}
	}

	history, _, _, _, err := activities.ensureRunConversation(context.Background(), state, "", planningRunInput{})
	if err != nil {
		t.Fatalf("ensureRunConversation returned error: %v", err)
	}
	if len(history) != 9 {
		t.Fatalf("expected summary + 8 recent messages, got %#v", history)
	}
	if history[0].Role != "user" || !strings.Contains(history[0].Content, "Resume context from earlier turns:") {
		t.Fatalf("expected synthetic summary message first, got %#v", history[0])
	}
	if history[1].SequenceNo != 6 {
		t.Fatalf("expected recent history to start after summarized cutoff, got %#v", history[1])
	}

	artifacts, err := artifactRepo.ListByRun(context.Background(), run.WorkspaceID, run.ID)
	if err != nil {
		t.Fatalf("list artifacts: %v", err)
	}
	found := false
	for _, artifact := range artifacts {
		if artifact.ArtifactType == workerpkg.TranscriptSummaryArtifactType {
			found = true
			if artifact.InlineContent == nil {
				t.Fatalf("unexpected transcript summary artifact %#v", artifact)
			}
			var checkpoint workerpkg.TranscriptSummaryCheckpoint
			if err := json.Unmarshal([]byte(*artifact.InlineContent), &checkpoint); err != nil {
				t.Fatalf("unmarshal transcript summary artifact: %v", err)
			}
			if checkpoint.CoveredThroughSequenceNo != 5 || checkpoint.SourceMessageCount != 5 || strings.TrimSpace(checkpoint.Summary) == "" {
				t.Fatalf("unexpected transcript summary checkpoint %#v", checkpoint)
			}
			var metadata map[string]any
			if err := json.Unmarshal(artifact.Metadata, &metadata); err != nil {
				t.Fatalf("unmarshal transcript summary metadata: %v", err)
			}
			if got := metadata["assistant_message_sequence_no"]; got != float64(4) {
				t.Fatalf("expected transcript summary to link to last covered assistant turn, got %#v", metadata)
			}
		}
	}
	if !found {
		t.Fatal("expected transcript summary artifact to be persisted")
	}
}

func TestCaptureTranscriptPlanningArtifactsLinksPreviewToAssistantTurn(t *testing.T) {
	dbName := fmt.Sprintf("file:preview-linkage-%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dbName), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	for _, stmt := range []string{
		`CREATE TABLE agent_run_artifacts (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			run_id TEXT NOT NULL,
			artifact_type TEXT NOT NULL,
			format TEXT NOT NULL,
			storage_mode TEXT NOT NULL,
			inline_content TEXT,
			object_key TEXT,
			metadata TEXT NOT NULL,
			sequence_no INTEGER NOT NULL,
			created_at DATETIME
		)`,
	} {
		if err := db.Exec(stmt).Error; err != nil {
			t.Fatalf("create artifact table: %v", err)
		}
	}

	artifactRepo := repository.NewAgentRunArtifactRepository(db)
	activities := &AgentRunActivities{artifactRepo: artifactRepo}
	run := &model.AgentRun{ID: "run-1", WorkspaceID: "ws-1", TargetType: "epic"}
	state := &resolvedRunState{run: run, epic: &model.PMEpic{ID: "epic-1"}}
	execCtx := &workerpkg.ExecutionContext{
		LastExecutionResult: &workerpkg.ExecutionResult{
			ToolInvocations: []model.ToolInvocation{
				{
					ToolName: workerpkg.ToolPublishPreview,
					Input: json.RawMessage(`{
						"panel_key":"prd_draft",
						"title":"PRD draft",
						"format":"markdown",
						"content":"# Draft",
						"replace":true
					}`),
				},
			},
		},
	}

	if err := activities.captureTranscriptPlanningArtifacts(context.Background(), state, execCtx, &model.AgentRunMessage{SequenceNo: 9}, planningRunInput{}); err != nil {
		t.Fatalf("captureTranscriptPlanningArtifacts returned error: %v", err)
	}

	artifacts, err := artifactRepo.ListByRun(context.Background(), run.WorkspaceID, run.ID)
	if err != nil {
		t.Fatalf("list preview artifacts: %v", err)
	}
	if len(artifacts) != 1 {
		t.Fatalf("expected one preview artifact, got %#v", artifacts)
	}
	var metadata map[string]any
	if err := json.Unmarshal(artifacts[0].Metadata, &metadata); err != nil {
		t.Fatalf("unmarshal preview metadata: %v", err)
	}
	if got := metadata["assistant_message_sequence_no"]; got != float64(9) {
		t.Fatalf("expected preview metadata to link to assistant turn, got %#v", metadata)
	}
}

func TestCaptureTranscriptPlanningArtifactsPersistsStoryPlannerPreview(t *testing.T) {
	dbName := fmt.Sprintf("file:story-preview-linkage-%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dbName), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	for _, stmt := range []string{
		`CREATE TABLE agent_run_artifacts (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			run_id TEXT NOT NULL,
			artifact_type TEXT NOT NULL,
			format TEXT NOT NULL,
			storage_mode TEXT NOT NULL,
			inline_content TEXT,
			object_key TEXT,
			metadata TEXT NOT NULL,
			sequence_no INTEGER NOT NULL,
			created_at DATETIME
		)`,
	} {
		if err := db.Exec(stmt).Error; err != nil {
			t.Fatalf("create artifact table: %v", err)
		}
	}

	artifactRepo := repository.NewAgentRunArtifactRepository(db)
	activities := &AgentRunActivities{artifactRepo: artifactRepo}
	run := &model.AgentRun{ID: "run-story-1", WorkspaceID: "ws-1", TargetType: "story"}
	state := &resolvedRunState{
		run:  run,
		task: &model.PMTask{ID: "story-1", WorkspaceID: "ws-1", Name: "Kafka health monitoring"},
	}
	execCtx := &workerpkg.ExecutionContext{
		LastExecutionResult: &workerpkg.ExecutionResult{
			ToolInvocations: []model.ToolInvocation{
				{
					ToolName: workerpkg.ToolPublishStoryPlanDoc,
					Input: json.RawMessage(`{
						"title":"Story Planning Document",
						"content":"# Outcome\n\nImplement Kafka health monitoring."
					}`),
				},
			},
		},
	}

	if err := activities.captureTranscriptPlanningArtifacts(context.Background(), state, execCtx, &model.AgentRunMessage{SequenceNo: 11}, planningRunInput{Stage: model.PlanningStageStoryPlanDoc}); err != nil {
		t.Fatalf("captureTranscriptPlanningArtifacts returned error: %v", err)
	}

	artifacts, err := artifactRepo.ListByRun(context.Background(), run.WorkspaceID, run.ID)
	if err != nil {
		t.Fatalf("list preview artifacts: %v", err)
	}
	if len(artifacts) != 1 {
		t.Fatalf("expected one preview artifact, got %#v", artifacts)
	}
	if artifacts[0].ArtifactType != workerpkg.RunPreviewArtifactType {
		t.Fatalf("expected run preview artifact, got %#v", artifacts[0])
	}

	var preview workerpkg.PublishedPreview
	if err := json.Unmarshal([]byte(derefString(artifacts[0].InlineContent)), &preview); err != nil {
		t.Fatalf("unmarshal preview artifact: %v", err)
	}
	if preview.PanelKey != "task_plan_doc" || preview.Format != workerpkg.PreviewFormatMarkdown {
		t.Fatalf("unexpected persisted story preview %#v", preview)
	}

	var metadata map[string]any
	if err := json.Unmarshal(artifacts[0].Metadata, &metadata); err != nil {
		t.Fatalf("unmarshal preview metadata: %v", err)
	}
	if got := metadata["assistant_message_sequence_no"]; got != float64(11) {
		t.Fatalf("expected preview metadata to link to assistant turn, got %#v", metadata)
	}
}

func TestPersistAssistantRunMessagePersistsRunPlanArtifact(t *testing.T) {
	dbName := fmt.Sprintf("file:run-plan-artifact-%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dbName), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	for _, stmt := range []string{
		`CREATE TABLE agent_run_messages (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			run_id TEXT NOT NULL,
			role TEXT NOT NULL,
			content TEXT,
			message_type TEXT NOT NULL DEFAULT 'message',
			content_blocks TEXT,
			turn_segments TEXT,
			tool_invocations TEXT,
			token_usage TEXT,
			sequence_no INTEGER NOT NULL DEFAULT 0,
			created_at DATETIME
		)`,
		`CREATE TABLE agent_run_artifacts (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			run_id TEXT NOT NULL,
			artifact_type TEXT NOT NULL,
			format TEXT NOT NULL,
			storage_mode TEXT NOT NULL,
			inline_content TEXT,
			object_key TEXT,
			metadata TEXT NOT NULL,
			sequence_no INTEGER NOT NULL,
			created_at DATETIME
		)`,
	} {
		if err := db.Exec(stmt).Error; err != nil {
			t.Fatalf("create table: %v", err)
		}
	}

	runMessageRepo := repository.NewAgentRunMessageRepository(db)
	artifactRepo := repository.NewAgentRunArtifactRepository(db)
	activities := &AgentRunActivities{runMessageRepo: runMessageRepo, artifactRepo: artifactRepo}

	run := &model.AgentRun{ID: "run-plan-1", WorkspaceID: "ws-1"}
	state := &resolvedRunState{run: run, agent: &model.Agent{ID: "agent-1", WorkspaceID: "ws-1"}}
	execCtx := &workerpkg.ExecutionContext{
		LastExecutionResult: &workerpkg.ExecutionResult{
			AssistantText: "Working through the plan.",
			ToolInvocations: []model.ToolInvocation{
				{
					ToolName: workerpkg.ToolUpdatePlan,
					Input: json.RawMessage(`{
						"note":"Focus on repo context first",
						"plan":[
							{"step":"Review current PRD draft","status":"completed"},
							{"step":"Inspect relevant modules","status":"in_progress"}
						]
					}`),
				},
			},
		},
	}

	assistantMessage, err := activities.persistAssistantRunMessage(context.Background(), state, execCtx)
	if err != nil {
		t.Fatalf("persistAssistantRunMessage returned error: %v", err)
	}
	if assistantMessage == nil {
		t.Fatal("expected assistant message to be persisted")
	}

	artifacts, err := artifactRepo.ListByRun(context.Background(), run.WorkspaceID, run.ID)
	if err != nil {
		t.Fatalf("list artifacts: %v", err)
	}
	found := false
	for _, artifact := range artifacts {
		if artifact.ArtifactType != model.AgentRunArtifactTypeRunPlan || artifact.InlineContent == nil {
			continue
		}
		found = true
		if !strings.Contains(*artifact.InlineContent, `"note":"Focus on repo context first"`) {
			t.Fatalf("expected run_plan inline content, got %s", *artifact.InlineContent)
		}
		var metadata map[string]any
		if err := json.Unmarshal(artifact.Metadata, &metadata); err != nil {
			t.Fatalf("unmarshal metadata: %v", err)
		}
		if got := metadata["assistant_message_sequence_no"]; got != float64(1) {
			t.Fatalf("expected assistant sequence metadata, got %#v", metadata)
		}
	}
	if !found {
		t.Fatal("expected run_plan artifact to be persisted")
	}
}

func TestPersistAssistantRunMessagePersistsNativeTurnDebugArtifact(t *testing.T) {
	dbName := fmt.Sprintf("file:native-turn-debug-%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dbName), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	for _, stmt := range []string{
		`CREATE TABLE agent_run_messages (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			run_id TEXT NOT NULL,
			role TEXT NOT NULL,
			content TEXT,
			message_type TEXT NOT NULL DEFAULT 'message',
			content_blocks TEXT,
			turn_segments TEXT,
			tool_invocations TEXT,
			token_usage TEXT,
			sequence_no INTEGER NOT NULL DEFAULT 0,
			created_at DATETIME
		)`,
		`CREATE TABLE agent_run_artifacts (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			run_id TEXT NOT NULL,
			artifact_type TEXT NOT NULL,
			format TEXT NOT NULL,
			storage_mode TEXT NOT NULL,
			inline_content TEXT,
			object_key TEXT,
			metadata TEXT NOT NULL,
			sequence_no INTEGER NOT NULL,
			created_at DATETIME
		)`,
	} {
		if err := db.Exec(stmt).Error; err != nil {
			t.Fatalf("create table: %v", err)
		}
	}

	runMessageRepo := repository.NewAgentRunMessageRepository(db)
	artifactRepo := repository.NewAgentRunArtifactRepository(db)
	activities := &AgentRunActivities{runMessageRepo: runMessageRepo, artifactRepo: artifactRepo}

	run := &model.AgentRun{ID: "native-debug-1", WorkspaceID: "ws-1"}
	state := &resolvedRunState{
		run:                        run,
		agent:                      &model.Agent{RuntimeKind: "native_sdk"},
		nativeSelectivePathEnabled: true,
	}
	execCtx := &workerpkg.ExecutionContext{
		NativeSelectivePathEnabled: true,
		PlanningStage:              model.PlanningStagePlanTasks,
		ProviderContinuation: &workerpkg.ProviderContinuation{
			ResponseID: "resp_123",
		},
		RuntimeSkillRefs: model.AgentSkillRefs{
			{Key: "general_agent_behavior"},
			{Key: "approval_protocol"},
			{SkillID: strPtr("workspace-skill-1")},
		},
		ActiveRuntimeSkillRefs: model.AgentSkillRefs{
			{Key: "approval_protocol"},
			{SkillID: strPtr("workspace-skill-1")},
		},
		SkillPolicy: workerpkg.SkillPolicy{
			CompletionRequiresInteractionKinds: []string{
				model.AgentRunInteractionKindApprovalRequest,
				model.AgentRunInteractionKindReviewCheckpoint,
			},
		},
		RepairGuidance:      "Retry with a same-turn preview binding.",
		RepairGuidanceClass: "approval_preview_binding",
		LastExecutionResult: &workerpkg.ExecutionResult{
			AssistantText: "Need to retry the approval handoff.",
		},
	}

	assistantMessage, err := activities.persistAssistantRunMessage(context.Background(), state, execCtx)
	if err != nil {
		t.Fatalf("persistAssistantRunMessage returned error: %v", err)
	}
	if assistantMessage == nil {
		t.Fatal("expected assistant message to be persisted")
	}

	artifacts, err := artifactRepo.ListByRun(context.Background(), run.WorkspaceID, run.ID)
	if err != nil {
		t.Fatalf("list artifacts: %v", err)
	}

	found := false
	for _, artifact := range artifacts {
		if artifact.ArtifactType != model.AgentRunArtifactTypeNativeTurnDebug || artifact.InlineContent == nil {
			continue
		}
		found = true

		var payload nativeTurnDebugArtifact
		if err := json.Unmarshal([]byte(*artifact.InlineContent), &payload); err != nil {
			t.Fatalf("unmarshal debug payload: %v", err)
		}
		if payload.RuntimeKind != "native_sdk" {
			t.Fatalf("expected runtime_kind native_sdk, got %q", payload.RuntimeKind)
		}
		if !payload.NativeSelectivePathEnabled {
			t.Fatal("expected native selective path enabled in payload")
		}
		if payload.PlanningStage != model.PlanningStagePlanTasks {
			t.Fatalf("expected planning stage %q, got %q", model.PlanningStagePlanTasks, payload.PlanningStage)
		}
		if payload.ContinuationMode != "response_id" {
			t.Fatalf("expected continuation mode response_id, got %q", payload.ContinuationMode)
		}
		if got := strings.Join(payload.RuntimeSkillRefs, ","); got != "general_agent_behavior,approval_protocol,workspace:workspace-skill-1" {
			t.Fatalf("unexpected runtime skill refs: %v", payload.RuntimeSkillRefs)
		}
		if got := strings.Join(payload.ActiveSkillRefs, ","); got != "approval_protocol,workspace:workspace-skill-1" {
			t.Fatalf("unexpected active skill refs: %v", payload.ActiveSkillRefs)
		}
		if got := strings.Join(payload.RequiredInteractions, ","); got != model.AgentRunInteractionKindApprovalRequest+","+model.AgentRunInteractionKindReviewCheckpoint {
			t.Fatalf("unexpected required interactions: %v", payload.RequiredInteractions)
		}
		if !payload.RepairGuidancePresent {
			t.Fatal("expected repair guidance to be marked present")
		}
		if payload.RepairGuidanceClass != "approval_preview_binding" {
			t.Fatalf("expected repair guidance class to be preserved, got %q", payload.RepairGuidanceClass)
		}

		var metadata map[string]any
		if err := json.Unmarshal(artifact.Metadata, &metadata); err != nil {
			t.Fatalf("unmarshal metadata: %v", err)
		}
		if got := metadata["assistant_message_sequence_no"]; got != float64(assistantMessage.SequenceNo) {
			t.Fatalf("expected assistant sequence metadata, got %#v", metadata)
		}
	}
	if !found {
		t.Fatal("expected native_turn_debug artifact to be persisted")
	}
}

func TestPersistAssistantRunMessagePersistsNativeRepairStateArtifact(t *testing.T) {
	dbName := fmt.Sprintf("file:native-repair-state-%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dbName), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	for _, stmt := range []string{
		`CREATE TABLE agent_run_messages (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			run_id TEXT NOT NULL,
			role TEXT NOT NULL,
			content TEXT,
			message_type TEXT NOT NULL DEFAULT 'message',
			content_blocks TEXT,
			turn_segments TEXT,
			tool_invocations TEXT,
			token_usage TEXT,
			sequence_no INTEGER NOT NULL DEFAULT 0,
			created_at DATETIME
		)`,
		`CREATE TABLE agent_run_artifacts (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			run_id TEXT NOT NULL,
			artifact_type TEXT NOT NULL,
			format TEXT NOT NULL,
			storage_mode TEXT NOT NULL,
			inline_content TEXT,
			object_key TEXT,
			metadata TEXT NOT NULL,
			sequence_no INTEGER NOT NULL,
			created_at DATETIME
		)`,
	} {
		if err := db.Exec(stmt).Error; err != nil {
			t.Fatalf("create table: %v", err)
		}
	}

	runMessageRepo := repository.NewAgentRunMessageRepository(db)
	artifactRepo := repository.NewAgentRunArtifactRepository(db)
	activities := &AgentRunActivities{runMessageRepo: runMessageRepo, artifactRepo: artifactRepo}

	run := &model.AgentRun{ID: "native-repair-1", WorkspaceID: "ws-1"}
	state := &resolvedRunState{
		run:                        run,
		agent:                      &model.Agent{RuntimeKind: "native_sdk"},
		nativeSelectivePathEnabled: true,
	}
	execCtx := &workerpkg.ExecutionContext{
		NativeSelectivePathEnabled: true,
		LastExecutionResult: &workerpkg.ExecutionResult{
			AssistantText: "Tried to publish the task plan.",
			Messages: []workerpkg.ExecutionMessage{
				{Role: "assistant", Content: "Tried to publish the task plan."},
				{
					Role:    "tool",
					Content: "publish_task_plan content must be a JSON object with summary and proposed_tasks",
					Blocks: []workerpkg.ExecutionBlock{
						{
							Type:     workerpkg.ExecutionBlockTypeToolResult,
							ToolName: workerpkg.ToolPublishTaskPlan,
							Output:   "publish_task_plan content must be a JSON object with summary and proposed_tasks",
							IsError:  true,
						},
					},
				},
			},
		},
	}

	assistantMessage, err := activities.persistAssistantRunMessage(context.Background(), state, execCtx)
	if err != nil {
		t.Fatalf("persistAssistantRunMessage returned error: %v", err)
	}
	if assistantMessage == nil {
		t.Fatal("expected assistant message to be persisted")
	}

	artifacts, err := artifactRepo.ListByRun(context.Background(), run.WorkspaceID, run.ID)
	if err != nil {
		t.Fatalf("list artifacts: %v", err)
	}

	found := false
	for _, artifact := range artifacts {
		if artifact.ArtifactType != model.AgentRunArtifactTypeNativeRepairState || artifact.InlineContent == nil {
			continue
		}
		found = true

		var payload model.NativeRepairState
		if err := json.Unmarshal([]byte(*artifact.InlineContent), &payload); err != nil {
			t.Fatalf("unmarshal repair state payload: %v", err)
		}
		if payload.Source != "tool_failure" {
			t.Fatalf("expected tool_failure source, got %#v", payload)
		}
		if payload.RepairClass != "publish_task_plan_object_shape" {
			t.Fatalf("expected publish_task_plan repair class, got %#v", payload)
		}
		if payload.ToolName != workerpkg.ToolPublishTaskPlan {
			t.Fatalf("expected publish_task_plan tool name, got %#v", payload)
		}
		if !strings.Contains(payload.RepairHint, "complete JSON object") {
			t.Fatalf("expected repair hint in payload, got %#v", payload)
		}
		if !strings.Contains(payload.ErrorSummary, "summary and proposed_tasks") {
			t.Fatalf("expected original error summary in payload, got %#v", payload)
		}

		var metadata map[string]any
		if err := json.Unmarshal(artifact.Metadata, &metadata); err != nil {
			t.Fatalf("unmarshal metadata: %v", err)
		}
		if got := metadata["assistant_message_sequence_no"]; got != float64(assistantMessage.SequenceNo) {
			t.Fatalf("expected assistant sequence metadata, got %#v", metadata)
		}
	}
	if !found {
		t.Fatal("expected native_repair_state artifact to be persisted")
	}
}

func TestRetryInvalidCompletionTurnPersistsNativeCompletionRepairState(t *testing.T) {
	dbName := fmt.Sprintf("file:native-completion-repair-%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dbName), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	for _, stmt := range []string{
		`CREATE TABLE agent_run_messages (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			run_id TEXT NOT NULL,
			role TEXT NOT NULL,
			content TEXT NOT NULL,
			message_type TEXT NOT NULL,
			content_blocks BLOB,
			turn_segments BLOB,
			tool_invocations BLOB,
			token_usage BLOB,
			sequence_no INTEGER NOT NULL,
			created_at DATETIME,
			updated_at DATETIME
		)`,
		`CREATE TABLE agent_run_artifacts (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			run_id TEXT NOT NULL,
			artifact_type TEXT NOT NULL,
			format TEXT NOT NULL,
			storage_mode TEXT NOT NULL,
			inline_content TEXT,
			object_key TEXT,
			metadata TEXT NOT NULL,
			sequence_no INTEGER NOT NULL,
			created_at DATETIME
		)`,
	} {
		if err := db.Exec(stmt).Error; err != nil {
			t.Fatalf("create table: %v", err)
		}
	}

	runMessageRepo := repository.NewAgentRunMessageRepository(db)
	artifactRepo := repository.NewAgentRunArtifactRepository(db)
	activities := &AgentRunActivities{runMessageRepo: runMessageRepo, artifactRepo: artifactRepo}

	run := &model.AgentRun{ID: "run-native-completion-repair", WorkspaceID: "ws-1"}
	state := &resolvedRunState{
		run:                        run,
		agent:                      &model.Agent{RuntimeKind: "native_sdk"},
		nativeSelectivePathEnabled: true,
	}
	assistantMessage, err := activities.createRunMessage(context.Background(), run, "assistant", "assistant_turn", "Need approval.", nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("create assistant message: %v", err)
	}

	retried, err := activities.retryInvalidCompletionTurn(context.Background(), state, assistantMessage, fmt.Errorf("approval_request requires a same-turn prd_draft preview before requesting approval"))
	if err != nil {
		t.Fatalf("retryInvalidCompletionTurn returned error: %v", err)
	}
	if !retried {
		t.Fatal("expected retryInvalidCompletionTurn to request a retry")
	}

	artifacts, err := artifactRepo.ListByRun(context.Background(), run.WorkspaceID, run.ID)
	if err != nil {
		t.Fatalf("list artifacts: %v", err)
	}
	found := false
	for _, artifact := range artifacts {
		if artifact.ArtifactType != model.AgentRunArtifactTypeNativeRepairState || artifact.InlineContent == nil {
			continue
		}
		found = true

		var payload model.NativeRepairState
		if err := json.Unmarshal([]byte(*artifact.InlineContent), &payload); err != nil {
			t.Fatalf("unmarshal repair state payload: %v", err)
		}
		if payload.Source != "completion_retry" {
			t.Fatalf("expected completion_retry source, got %#v", payload)
		}
		if payload.RepairClass != "approval_specific_preview_required" {
			t.Fatalf("expected specific preview repair class, got %#v", payload)
		}
		if !strings.Contains(payload.RepairHint, `preview_panel_key="prd_draft"`) {
			t.Fatalf("expected completion retry hint to preserve preview binding, got %#v", payload)
		}
		if !strings.Contains(payload.ErrorSummary, "same-turn prd_draft preview") {
			t.Fatalf("expected original completion error summary, got %#v", payload)
		}

		var metadata map[string]any
		if err := json.Unmarshal(artifact.Metadata, &metadata); err != nil {
			t.Fatalf("unmarshal metadata: %v", err)
		}
		if got := metadata["assistant_message_sequence_no"]; got != float64(assistantMessage.SequenceNo) {
			t.Fatalf("expected assistant sequence metadata, got %#v", metadata)
		}
	}
	if !found {
		t.Fatal("expected completion retry repair artifact to be persisted")
	}
}

func TestPersistAssistantRunMessageSkipsNativeTurnDebugArtifactOutsideSelectivePath(t *testing.T) {
	dbName := fmt.Sprintf("file:native-turn-debug-off-%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dbName), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	for _, stmt := range []string{
		`CREATE TABLE agent_run_messages (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			run_id TEXT NOT NULL,
			role TEXT NOT NULL,
			content TEXT,
			message_type TEXT NOT NULL DEFAULT 'message',
			content_blocks TEXT,
			turn_segments TEXT,
			tool_invocations TEXT,
			token_usage TEXT,
			sequence_no INTEGER NOT NULL DEFAULT 0,
			created_at DATETIME
		)`,
		`CREATE TABLE agent_run_artifacts (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			run_id TEXT NOT NULL,
			artifact_type TEXT NOT NULL,
			format TEXT NOT NULL,
			storage_mode TEXT NOT NULL,
			inline_content TEXT,
			object_key TEXT,
			metadata TEXT NOT NULL,
			sequence_no INTEGER NOT NULL,
			created_at DATETIME
		)`,
	} {
		if err := db.Exec(stmt).Error; err != nil {
			t.Fatalf("create table: %v", err)
		}
	}

	runMessageRepo := repository.NewAgentRunMessageRepository(db)
	artifactRepo := repository.NewAgentRunArtifactRepository(db)
	activities := &AgentRunActivities{runMessageRepo: runMessageRepo, artifactRepo: artifactRepo}

	run := &model.AgentRun{ID: "native-debug-2", WorkspaceID: "ws-1", RuntimeKind: "native_sdk"}
	state := &resolvedRunState{run: run, nativeSelectivePathEnabled: false}
	execCtx := &workerpkg.ExecutionContext{
		LastExecutionResult: &workerpkg.ExecutionResult{
			AssistantText: "Legacy path message.",
		},
	}

	if _, err := activities.persistAssistantRunMessage(context.Background(), state, execCtx); err != nil {
		t.Fatalf("persistAssistantRunMessage returned error: %v", err)
	}

	artifacts, err := artifactRepo.ListByRun(context.Background(), run.WorkspaceID, run.ID)
	if err != nil {
		t.Fatalf("list artifacts: %v", err)
	}
	for _, artifact := range artifacts {
		if artifact.ArtifactType == model.AgentRunArtifactTypeNativeTurnDebug {
			t.Fatalf("did not expect native_turn_debug artifact outside selective path: %#v", artifact)
		}
	}
}

func TestBuildArtifactContextEntriesIncludesLatestRunPlan(t *testing.T) {
	runPlanOld, _ := json.Marshal(workerpkg.RunPlanArtifact{
		Plan: []workerpkg.RunPlanStep{{Step: "Old", Status: workerpkg.PlanStepInProgress}},
	})
	runPlanNew, _ := json.Marshal(workerpkg.RunPlanArtifact{
		Note: "Keep the current scope tight.",
		Plan: []workerpkg.RunPlanStep{
			{Step: "Review current PRD draft", Status: workerpkg.PlanStepCompleted},
			{Step: "Inspect relevant modules", Status: workerpkg.PlanStepInProgress},
		},
	})

	entries, err := buildArtifactContextEntries([]model.AgentRunArtifact{
		{
			ID:            "artifact-1",
			ArtifactType:  model.AgentRunArtifactTypeRunPlan,
			Format:        "json",
			InlineContent: strPtr(string(runPlanOld)),
			SequenceNo:    1,
		},
		{
			ID:            "artifact-2",
			ArtifactType:  model.AgentRunArtifactTypeRunPlan,
			Format:        "json",
			InlineContent: strPtr(string(runPlanNew)),
			SequenceNo:    2,
		},
	})
	if err != nil {
		t.Fatalf("buildArtifactContextEntries returned error: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected one latest run_plan entry, got %#v", entries)
	}
	if entries[0].Source != model.AgentRunArtifactTypeRunPlan || entries[0].Label != "Current execution plan" {
		t.Fatalf("unexpected run plan entry %#v", entries[0])
	}
	if !strings.Contains(entries[0].Content, "Keep the current scope tight.") || !strings.Contains(entries[0].Content, "[>] Inspect relevant modules") {
		t.Fatalf("expected rendered run plan content, got %s", entries[0].Content)
	}
}

func TestBuildArtifactContextEntriesPrefersApprovedPreviewOverDraftForSamePanel(t *testing.T) {
	draftContent, _ := json.Marshal("# Draft PRD")
	approvedContent, _ := json.Marshal("# Approved PRD")
	approvedPreview, _ := json.Marshal(model.ApprovedRunPreview{
		Phase:        "prd",
		PanelKey:     "prd_draft",
		PreviewTitle: "PRD Draft",
		Format:       workerpkg.PreviewFormatMarkdown,
		Content:      approvedContent,
	})
	appliedMarker, _ := json.Marshal(model.AppliedApprovedRunPreview{
		ApprovedArtifactID: "artifact-approved",
		Phase:              "prd",
		Action:             "persist_prd",
		AppliedAt:          time.Now().UTC(),
	})
	runPreview, _ := json.Marshal(workerpkg.PublishedPreview{
		PanelKey: "prd_draft",
		Title:    "PRD Draft",
		Format:   workerpkg.PreviewFormatMarkdown,
		Content:  draftContent,
	})

	entries, err := buildArtifactContextEntries([]model.AgentRunArtifact{
		{
			ID:            "artifact-draft",
			ArtifactType:  workerpkg.RunPreviewArtifactType,
			Format:        "json",
			InlineContent: strPtr(string(runPreview)),
			SequenceNo:    1,
		},
		{
			ID:            "artifact-approved",
			ArtifactType:  model.AgentRunArtifactTypeApprovedPreview,
			Format:        "json",
			InlineContent: strPtr(string(approvedPreview)),
			SequenceNo:    2,
		},
		{
			ID:            "artifact-applied",
			ArtifactType:  model.AgentRunArtifactTypeApprovedPreviewApplied,
			Format:        "json",
			InlineContent: strPtr(string(appliedMarker)),
			SequenceNo:    3,
		},
	})
	if err != nil {
		t.Fatalf("buildArtifactContextEntries returned error: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected only the approved preview entry, got %#v", entries)
	}
	if entries[0].Source != model.AgentRunArtifactTypeApprovedPreview {
		t.Fatalf("expected approved preview source, got %#v", entries[0])
	}
	if entries[0].Status != "approved_and_persist_prd" {
		t.Fatalf("expected applied approved-preview status, got %#v", entries[0])
	}
	if strings.Contains(entries[0].Label, "Current preview") {
		t.Fatalf("expected stale draft preview to be omitted, got %#v", entries[0])
	}
	if !strings.Contains(entries[0].Content, "Approved PRD") {
		t.Fatalf("expected approved preview content, got %q", entries[0].Content)
	}
}

func TestBuildDurableRunFactsCollectsGenericIDsFromStateAndRunInput(t *testing.T) {
	state := &resolvedRunState{
		run: &model.AgentRun{
			ID:               "run-1",
			WorkspaceID:      "ws-1",
			AgentID:          "agent-1",
			TargetType:       "crm_deal",
			TargetID:         "deal-1",
			TaskID:           strPtr("story-1"),
			ConversationID:   strPtr("conv-1"),
			RepositoryID:     strPtr("repo-run"),
			DeliveryTargetID: strPtr("delivery-run"),
			BaseBranch:       strPtr("main"),
			WorkingBranch:    strPtr("tp-123"),
			Input: json.RawMessage(`{
				"document_id":"doc-1",
				"crm":{
					"contact_id":"contact-1",
					"pipeline_stage_id":"stage-1",
					"participant_ids":["person-1","person-2"]
				}
			}`),
		},
		agent: &model.Agent{
			ID:          "agent-1",
			WorkspaceID: "ws-1",
			TeamID:      strPtr("team-agent"),
		},
		task: &model.PMTask{
			ID:             "story-1",
			EpicID:         strPtr("epic-1"),
			TeamID:         strPtr("team-story"),
			OwnerID:        strPtr("owner-1"),
			PlanDocumentID: strPtr("story-doc-1"),
		},
		epic: &model.PMEpic{
			ID:                    "epic-1",
			SpecDocumentID:        strPtr("spec-epic"),
			ApprovedSpecVersionID: strPtr("ver-approved"),
			PlanningRepositoryID:  strPtr("repo-plan"),
		},
		conversation: &model.SupportConversation{
			ID:           "conv-1",
			CRMContactID: strPtr("contact-conversation"),
		},
		deliveryTarget: &model.TaskDeliveryTarget{
			ID:           "delivery-1",
			RepositoryID: strPtr("repo-delivery"),
		},
		repository: &model.GitRepository{
			ID:            "repo-1",
			IntegrationID: "int-1",
			FullName:      "helpin-ai/helpin",
		},
		integration: &model.GitIntegration{
			ID:             "int-1",
			InstallationID: strPtr("install-1"),
		},
		teamDefault: &model.PMTeamRepoDefault{
			ID:           "team-default-1",
			TeamID:       "team-story",
			RepositoryID: "repo-team",
		},
	}

	facts := buildDurableRunFacts(state, planningRunInput{
		SpecDocumentID: "spec-input",
		SpecVersionID:  "ver-input",
	})

	for key, expected := range map[string]string{
		"workspace_id":                "ws-1",
		"run_id":                      "run-1",
		"agent_id":                    "agent-1",
		"target_type":                 "crm_deal",
		"target_id":                   "deal-1",
		"crm_deal_id":                 "deal-1",
		"document_id":                 "doc-1",
		"crm_contact_id":              "contact-1",
		"crm_pipeline_stage_id":       "stage-1",
		"crm_participant_ids":         "person-1, person-2",
		"story_id":                    "story-1",
		"epic_id":                     "epic-1",
		"plan_document_id":            "story-doc-1",
		"spec_document_id":            "spec-input",
		"spec_version_id":             "ver-input",
		"approved_spec_version_id":    "ver-approved",
		"conversation_id":             "conv-1",
		"conversation_crm_contact_id": "contact-conversation",
		"repository_id":               "repo-run",
		"delivery_target_id":          "delivery-run",
		"repo_full_name":              "helpin-ai/helpin",
		"working_branch":              "tp-123",
		"epic_planning_repository_id": "repo-plan",
		"team_default_repository_id":  "repo-team",
	} {
		if got := facts[key]; got != expected {
			t.Fatalf("expected fact %q=%q, got %q", key, expected, got)
		}
	}
}

func TestFormatInteractivePlanningFacts(t *testing.T) {
	facts := formatInteractivePlanningFacts(planningRunInput{
		SpecDocumentID: "doc-1",
		SpecVersionID:  "ver-1",
	}, true, 3)

	for _, marker := range []string{
		"Current durable planning facts:",
		"- approved_spec_exists=true",
		"- draft_spec_exists=true",
		"- existing_task_count=3",
		"- spec_document_id=doc-1",
		"- approved_spec_version_id=ver-1",
	} {
		if !strings.Contains(facts, marker) {
			t.Fatalf("expected facts summary to contain %q\n%s", marker, facts)
		}
	}
}

func TestApplyApprovedInteractivePreviewReturnsPersistPRDAction(t *testing.T) {
	dbName := fmt.Sprintf("file:approved-preview-%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dbName), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	for _, stmt := range []string{
		`CREATE TABLE agent_run_artifacts (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			run_id TEXT NOT NULL,
			artifact_type TEXT NOT NULL,
			format TEXT NOT NULL,
			storage_mode TEXT NOT NULL,
			inline_content TEXT,
			object_key TEXT,
			metadata TEXT NOT NULL,
			sequence_no INTEGER NOT NULL,
			created_at DATETIME
		)`,
		`CREATE TABLE pm_epics (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			name TEXT NOT NULL,
			description TEXT,
			external_id TEXT,
			epic_state_id TEXT,
			owner_id TEXT,
			owner_member_id TEXT,
			team_id TEXT,
			planned_start_date DATETIME,
			deadline DATETIME,
			started BOOLEAN NOT NULL DEFAULT 0,
			started_at DATETIME,
			completed BOOLEAN NOT NULL DEFAULT 0,
			completed_at DATETIME,
			position INTEGER NOT NULL DEFAULT 0,
			color TEXT,
			health TEXT NOT NULL DEFAULT 'no_health',
			health_comment TEXT,
			archived BOOLEAN NOT NULL DEFAULT 0,
			spec_document_id TEXT,
			planning_repository_id TEXT,
			planning_state TEXT NOT NULL DEFAULT 'not_started',
			spec_clarifications TEXT NOT NULL DEFAULT '[]',
			spec_clarified_at DATETIME,
			spec_clarified_by TEXT,
			approved_spec_version_id TEXT,
			last_planning_run_id TEXT,
			created_by TEXT,
			created_at DATETIME,
			updated_at DATETIME
		)`,
		`CREATE TABLE pm_epic_labels (
			epic_id TEXT NOT NULL,
			label_id TEXT NOT NULL,
			created_at DATETIME
		)`,
		`CREATE TABLE pm_labels (
			id TEXT PRIMARY KEY,
			workspace_id TEXT,
			name TEXT NOT NULL,
			color TEXT,
			created_at DATETIME,
			updated_at DATETIME
		)`,
		`CREATE TABLE pm_epic_objectives (
			epic_id TEXT NOT NULL,
			objective_id TEXT NOT NULL,
			created_at DATETIME
		)`,
		`CREATE TABLE pm_objectives (
			id TEXT PRIMARY KEY,
			workspace_id TEXT,
			name TEXT NOT NULL
		)`,
		`CREATE TABLE pm_workflow_states (
			id TEXT PRIMARY KEY,
			state_type TEXT NOT NULL
		)`,
		`CREATE TABLE pm_tasks (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			display_id INTEGER NOT NULL DEFAULT 0,
			name TEXT NOT NULL,
			description TEXT,
			task_type TEXT NOT NULL DEFAULT 'feature',
			workflow_id TEXT NOT NULL,
			workflow_state_id TEXT NOT NULL,
			epic_id TEXT,
			sprint_id TEXT,
			team_id TEXT,
			owner_id TEXT,
			owner_member_id TEXT,
			requester_id TEXT,
			requester_member_id TEXT,
			estimate INTEGER,
			priority TEXT NOT NULL DEFAULT 'none',
			severity TEXT NOT NULL DEFAULT 'none',
			deadline DATETIME,
			position INTEGER NOT NULL DEFAULT 0,
			started BOOLEAN NOT NULL DEFAULT 0,
			started_at DATETIME,
			completed BOOLEAN NOT NULL DEFAULT 0,
			completed_at DATETIME,
			moved_at DATETIME,
			blocked BOOLEAN NOT NULL DEFAULT 0,
			blocker TEXT,
			plan_document_id TEXT,
			archived BOOLEAN NOT NULL DEFAULT 0,
			assigned_agent_id TEXT,
			template_id TEXT,
			recurring_template_id TEXT,
			recurring_run_id TEXT,
			recurring_occurrence_number INTEGER,
			external_id TEXT,
			slice_type TEXT,
			implementation_brief TEXT,
			created_at DATETIME,
			updated_at DATETIME
		)`,
		`CREATE TABLE docs_spaces (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			team_id TEXT,
			name TEXT NOT NULL,
			slug TEXT NOT NULL,
			icon TEXT,
			visibility TEXT NOT NULL,
			type TEXT NOT NULL,
			default_review_days INTEGER,
			description TEXT,
			position INTEGER NOT NULL DEFAULT 0,
			is_system BOOLEAN NOT NULL DEFAULT 0,
			created_by TEXT NOT NULL,
			created_at DATETIME,
			updated_at DATETIME,
			deleted_at DATETIME
		)`,
		`CREATE TABLE docs_documents (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			space_id TEXT NOT NULL,
			collection_id TEXT,
			title TEXT NOT NULL,
			status TEXT NOT NULL,
			visibility TEXT NOT NULL,
			owner_id TEXT,
			team_id TEXT,
			template_key TEXT,
			excerpt TEXT,
			icon TEXT,
			tags TEXT,
			is_pinned BOOLEAN NOT NULL DEFAULT 0,
			is_publicly_shared BOOLEAN NOT NULL DEFAULT 0,
			share_token TEXT,
			is_locked BOOLEAN NOT NULL DEFAULT 0,
			locked_by TEXT,
			last_reviewed_at DATETIME,
			next_review_at DATETIME,
			published_at DATETIME,
			created_by TEXT NOT NULL,
			created_at DATETIME,
			updated_at DATETIME,
			deleted_at DATETIME
		)`,
		`CREATE TABLE docs_contents (
			id TEXT PRIMARY KEY,
			document_id TEXT NOT NULL UNIQUE,
			content TEXT,
			content_text TEXT,
			word_count INTEGER NOT NULL DEFAULT 0,
			created_at DATETIME,
			updated_at DATETIME
		)`,
		`CREATE TABLE docs_versions (
			id TEXT PRIMARY KEY,
			document_id TEXT NOT NULL,
			content TEXT,
			content_text TEXT,
			snapshot_label TEXT,
			version_type TEXT NOT NULL,
			word_count INTEGER NOT NULL DEFAULT 0,
			created_by TEXT NOT NULL,
			created_at DATETIME
		)`,
		`CREATE TABLE docs_links (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			document_id TEXT NOT NULL,
			linked_object_type TEXT NOT NULL,
			linked_object_id TEXT NOT NULL,
			link_context TEXT NOT NULL,
			created_by TEXT NOT NULL,
			created_at DATETIME
		)`,
	} {
		if err := db.Exec(stmt).Error; err != nil {
			t.Fatalf("create test table: %v", err)
		}
	}

	artifactRepo := repository.NewAgentRunArtifactRepository(db)
	epicRepo := repository.NewPMEpicRepository(db)
	docsDocRepo := repository.NewDocsDocumentRepository(db)

	run := &model.AgentRun{
		ID:             "run-1",
		WorkspaceID:    "ws-1",
		AgentID:        "agent-1",
		TargetType:     "epic",
		TargetID:       "epic-1",
		InvocationMode: model.InvocationModeInteractive,
		Status:         model.AgentRunStatusRunning,
		Input:          json.RawMessage(`{}`),
		OutputSummary:  json.RawMessage(`{}`),
	}

	epic := &model.PMEpic{
		ID:                    "epic-1",
		WorkspaceID:           "ws-1",
		Name:                  "Epic",
		SpecDocumentID:        strPtr("doc-1"),
		SpecClarifications:    json.RawMessage(`[]`),
		ApprovedSpecVersionID: nil,
	}
	if err := db.Create(epic).Error; err != nil {
		t.Fatalf("create epic: %v", err)
	}
	if err := db.Exec(`INSERT INTO docs_documents (id, workspace_id, space_id, title, status, visibility, created_by, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`,
		"doc-1", "ws-1", "space-1", "Epic Product Spec", model.DocStatusDraft, model.SpaceVisibilityWorkspaceWide, run.AgentID,
	).Error; err != nil {
		t.Fatalf("create docs document: %v", err)
	}

	markdownJSON, err := json.Marshal("# Problem\n\nApproved draft")
	if err != nil {
		t.Fatalf("marshal markdown: %v", err)
	}
	preview := model.ApprovedRunPreview{
		Phase:    "prd",
		PanelKey: "prd_draft",
		Format:   workerpkg.PreviewFormatMarkdown,
		Content:  markdownJSON,
	}
	previewJSON, err := json.Marshal(preview)
	if err != nil {
		t.Fatalf("marshal preview: %v", err)
	}
	artifact := &model.AgentRunArtifact{
		ID:            "approved-1",
		WorkspaceID:   run.WorkspaceID,
		RunID:         run.ID,
		ArtifactType:  model.AgentRunArtifactTypeApprovedPreview,
		Format:        "json",
		StorageMode:   "inline",
		InlineContent: strPtr(string(previewJSON)),
		Metadata:      json.RawMessage(`{}`),
		SequenceNo:    1,
	}
	if err := db.Create(artifact).Error; err != nil {
		t.Fatalf("create approved preview artifact: %v", err)
	}

	var executedCommands []string
	commandExecutor := stubInternalCommandExecutor{
		executeFn: func(ctx context.Context, meta model.InternalCommandContext, name string, input json.RawMessage) (json.RawMessage, error) {
			executedCommands = append(executedCommands, name)
			if name == "pm.approve_epic_spec" {
				if err := db.Exec("UPDATE pm_epics SET approved_spec_version_id = ? WHERE id = ?", "ver-1", epic.ID).Error; err != nil {
					return nil, err
				}
			}
			return json.RawMessage(`{}`), nil
		},
	}

	activity := &AgentRunActivities{
		artifactRepo:    artifactRepo,
		epicRepo:        epicRepo,
		docsDocRepo:     docsDocRepo,
		commandExecutor: commandExecutor,
	}
	state := &resolvedRunState{
		run:  run,
		epic: epic,
	}
	input := planningRunInput{}

	action, err := activity.applyApprovedInteractivePreview(context.Background(), state, &input)
	if err != nil {
		t.Fatalf("applyApprovedInteractivePreview returned error: %v", err)
	}
	if action != "persist_prd" {
		t.Fatalf("expected persist_prd action, got %q", action)
	}
	if input.SpecDocumentID != "doc-1" || input.SpecVersionID != "ver-1" {
		t.Fatalf("expected planning input to be updated with approved spec ids, got %#v", input)
	}
	if len(executedCommands) != 2 || executedCommands[0] != "docs.write_document_content" || executedCommands[1] != "pm.approve_epic_spec" {
		t.Fatalf("expected docs write then epic spec approval commands, got %#v", executedCommands)
	}

	var appliedMarkers []model.AgentRunArtifact
	if err := db.Where("run_id = ? AND artifact_type = ?", run.ID, model.AgentRunArtifactTypeApprovedPreviewApplied).Find(&appliedMarkers).Error; err != nil {
		t.Fatalf("list applied markers: %v", err)
	}
	if len(appliedMarkers) != 1 {
		t.Fatalf("expected 1 approved preview applied marker, got %d", len(appliedMarkers))
	}

	updatedEpic, err := epicRepo.GetByID(context.Background(), epic.ID)
	if err != nil {
		t.Fatalf("get updated epic: %v", err)
	}
	if updatedEpic == nil || updatedEpic.Epic.SpecDocumentID == nil || updatedEpic.Epic.ApprovedSpecVersionID == nil {
		t.Fatalf("expected epic spec document/version to be set, got %#v", updatedEpic)
	}

	if updatedEpic.Epic.ApprovedSpecVersionID == nil || *updatedEpic.Epic.ApprovedSpecVersionID != "ver-1" {
		t.Fatalf("expected approved spec version to be updated, got %#v", updatedEpic.Epic.ApprovedSpecVersionID)
	}
}

func TestApplyApprovedInteractivePreviewCreatesStoriesFromApprovedStoryPlan(t *testing.T) {
	db := newPlannerApprovalTestDB(t)

	artifactRepo := repository.NewAgentRunArtifactRepository(db)
	epicRepo := repository.NewPMEpicRepository(db)
	taskRepo := repository.NewPMTaskRepository(db)
	runRepo := repository.NewAgentRunRepository(db)
	agentRepo := repository.NewAgentRepository(db)

	agent := &model.Agent{
		ID:                    "agent-epic",
		WorkspaceID:           "ws-1",
		Name:                  "Epic Planner",
		Status:                "running",
		RuntimeKind:           "native_sdk",
		Skills:                model.AgentSkillRefs{},
		TriggerMode:           "manual",
		AllowedTools:          json.RawMessage(`[]`),
		AllowedCommands:       json.RawMessage(`[]`),
		AllowedTargets:        json.RawMessage(`[]`),
		ApprovalMode:          "preset_default",
		MaxConcurrentRuns:     1,
		DefaultInvocationMode: model.InvocationModeInteractive,
	}
	if err := db.Create(agent).Error; err != nil {
		t.Fatalf("create agent: %v", err)
	}

	run := &model.AgentRun{
		ID:             "run-story-plan",
		WorkspaceID:    "ws-1",
		AgentID:        agent.ID,
		TargetType:     "epic",
		TargetID:       "epic-1",
		InvocationMode: model.InvocationModeInteractive,
		Status:         model.AgentRunStatusRunning,
		Input:          json.RawMessage(`{}`),
		OutputSummary:  json.RawMessage(`{}`),
	}
	if err := db.Create(run).Error; err != nil {
		t.Fatalf("create run: %v", err)
	}

	epic := &model.PMEpic{
		ID:                 "epic-1",
		WorkspaceID:        "ws-1",
		Name:               "Epic",
		PlanningState:      model.EpicPlanningStateReadyForStoryPlanning,
		SpecClarifications: json.RawMessage(`[]`),
	}
	if err := db.Create(epic).Error; err != nil {
		t.Fatalf("create epic: %v", err)
	}
	if err := db.Exec(`INSERT INTO pm_workflow_states (id, state_type) VALUES (?, ?)`, "state-1", model.PMStateTypeUnstarted).Error; err != nil {
		t.Fatalf("create workflow state: %v", err)
	}

	previewPayload, err := json.Marshal(map[string]any{
		"summary": "Breakdown",
		"proposed_stories": []map[string]any{
			{
				"ref":                 "story_1",
				"name":                "Add tracking helper",
				"description":         "Create shared metric helper",
				"story_type":          "chore",
				"acceptance_criteria": []string{"works"},
				"dependency_refs":     []string{},
			},
			{
				"ref":                 "story_2",
				"name":                "Wire tracking into capture errors",
				"description":         "Use the helper in capture",
				"story_type":          "feature",
				"acceptance_criteria": []string{"works"},
				"dependency_refs":     []string{"story_1"},
			},
		},
	})
	if err != nil {
		t.Fatalf("marshal preview payload: %v", err)
	}
	preview := model.ApprovedRunPreview{
		Phase:    "stories",
		PanelKey: "story_plan",
		Format:   workerpkg.PreviewFormatJSON,
		Content:  previewPayload,
	}
	previewJSON, err := json.Marshal(preview)
	if err != nil {
		t.Fatalf("marshal approved preview: %v", err)
	}
	if err := db.Create(&model.AgentRunArtifact{
		ID:            "approved-stories-1",
		WorkspaceID:   run.WorkspaceID,
		RunID:         run.ID,
		ArtifactType:  model.AgentRunArtifactTypeApprovedPreview,
		Format:        "json",
		StorageMode:   "inline",
		InlineContent: strPtr(string(previewJSON)),
		Metadata:      json.RawMessage(`{}`),
		SequenceNo:    1,
	}).Error; err != nil {
		t.Fatalf("create approved preview artifact: %v", err)
	}

	var executed []string
	commandExecutor := stubInternalCommandExecutor{
		executeFn: func(ctx context.Context, meta model.InternalCommandContext, name string, input json.RawMessage) (json.RawMessage, error) {
			executed = append(executed, name)
			if name != "pm.create_task_batch" {
				return json.RawMessage(`{}`), nil
			}
			var payload struct {
				Tasks []model.ProposedTask `json:"tasks"`
			}
			if err := json.Unmarshal(input, &payload); err != nil {
				return nil, err
			}
			for _, planned := range payload.Tasks {
				story := &model.PMTask{
					ID:              "db-" + planned.Ref,
					WorkspaceID:     run.WorkspaceID,
					Name:            planned.Name,
					TaskType:        planned.TaskType,
					WorkflowID:      "wf-1",
					WorkflowStateID: "state-1",
					EpicID:          &epic.ID,
					Priority:        model.PMTaskPriorityNone,
					Severity:        model.PMTaskSeverityNone,
				}
				if err := taskRepo.Create(ctx, story); err != nil {
					return nil, err
				}
			}
			return mustJSON(workerpkg.CreateTaskBatchResult{
				Tasks: []workerpkg.CreateTaskBatchTaskResult{
					{Ref: "story_1", TaskID: "db-story_1", Name: "Add tracking helper"},
					{Ref: "story_2", TaskID: "db-story_2", Name: "Wire tracking into capture errors"},
				},
			}), nil
		},
	}

	activity := &AgentRunActivities{
		runRepo:         runRepo,
		artifactRepo:    artifactRepo,
		epicRepo:        epicRepo,
		taskRepo:        taskRepo,
		agentRepo:       agentRepo,
		commandExecutor: commandExecutor,
	}
	state := &resolvedRunState{
		run:  run,
		epic: epic,
	}
	input := planningRunInput{Stage: model.PlanningStageStoryPlanDoc}

	action, err := activity.applyApprovedInteractivePreview(context.Background(), state, &input)
	if err != nil {
		t.Fatalf("applyApprovedInteractivePreview returned error: %v", err)
	}
	if action != "create_tasks" {
		t.Fatalf("expected create_tasks action, got %q", action)
	}
	if len(executed) != 1 || executed[0] != "pm.create_task_batch" {
		t.Fatalf("expected task batch command, got %#v", executed)
	}

	var createdStories []model.PMTask
	if err := db.Where("epic_id = ?", epic.ID).Find(&createdStories).Error; err != nil {
		t.Fatalf("list created stories: %v", err)
	}
	if len(createdStories) != 2 {
		t.Fatalf("expected 2 created stories, got %d", len(createdStories))
	}

	updatedRun, err := runRepo.GetByIDAny(context.Background(), run.ID)
	if err != nil {
		t.Fatalf("get updated run: %v", err)
	}
	if updatedRun == nil || updatedRun.Status != model.AgentRunStatusCompleted {
		t.Fatalf("expected run to complete, got %#v", updatedRun)
	}

	updatedAgent, err := agentRepo.GetByID(context.Background(), agent.WorkspaceID, agent.ID)
	if err != nil {
		t.Fatalf("get updated agent: %v", err)
	}
	if updatedAgent == nil || updatedAgent.Status != "idle" {
		t.Fatalf("expected agent to be idle, got %#v", updatedAgent)
	}

	var appliedMarkers []model.AgentRunArtifact
	if err := db.Where("run_id = ? AND artifact_type = ?", run.ID, model.AgentRunArtifactTypeApprovedPreviewApplied).Find(&appliedMarkers).Error; err != nil {
		t.Fatalf("list applied markers: %v", err)
	}
	if len(appliedMarkers) != 1 {
		t.Fatalf("expected 1 approved preview applied marker, got %d", len(appliedMarkers))
	}
}

func TestExecuteRunActivityPausesNativePlannerForReviewCheckpoint(t *testing.T) {
	t.Setenv("AGENT_NATIVE_SELECTIVE_PLANNER_ENABLED", "true")

	db := newPlannerApprovalTestDB(t)
	if err := db.Exec(`CREATE TABLE IF NOT EXISTS agent_run_messages (
		id TEXT PRIMARY KEY,
		workspace_id TEXT NOT NULL,
		run_id TEXT NOT NULL,
		role TEXT NOT NULL,
		content TEXT NOT NULL,
		message_type TEXT NOT NULL,
		content_blocks BLOB,
		turn_segments BLOB,
		tool_invocations BLOB,
		token_usage BLOB,
		sequence_no INTEGER NOT NULL,
		created_at DATETIME,
		updated_at DATETIME
	)`).Error; err != nil {
		t.Fatalf("create agent_run_messages table: %v", err)
	}
	if err := db.Exec(`CREATE TABLE IF NOT EXISTS agent_run_interactions (
		id TEXT PRIMARY KEY,
		workspace_id TEXT NOT NULL,
		run_id TEXT NOT NULL,
		runtime_kind TEXT NOT NULL,
		interaction_kind TEXT NOT NULL,
		status TEXT NOT NULL,
		request_schema_version TEXT NOT NULL,
		response_schema_version TEXT,
		request_id TEXT,
		thread_id TEXT,
		turn_id TEXT,
		item_id TEXT,
		approval_id TEXT,
		assistant_message_sequence_no INTEGER,
		title TEXT,
		summary TEXT,
		request_payload BLOB NOT NULL,
		response_payload BLOB,
		runtime_metadata BLOB,
		resolved_by TEXT,
		resolved_at DATETIME,
		created_at DATETIME,
		updated_at DATETIME
	)`).Error; err != nil {
		t.Fatalf("create agent_run_interactions table: %v", err)
	}

	runRepo := repository.NewAgentRunRepository(db)
	runMessageRepo := repository.NewAgentRunMessageRepository(db)
	interactionRepo := repository.NewAgentRunInteractionRepository(db)
	agentRepo := repository.NewAgentRepository(db)
	artifactRepo := repository.NewAgentRunArtifactRepository(db)
	epicRepo := repository.NewPMEpicRepository(db)
	docsDocRepo := repository.NewDocsDocumentRepository(db)
	docsContentRepo := repository.NewDocsContentRepository(db)
	docsLinkRepo := repository.NewDocsLinkRepository(db)
	wsPublisher := &capturedEventPublisher{}
	var capturedSelectivePathEnabled bool
	var capturedRuntimeSkillRefs model.AgentSkillRefs
	var capturedActiveRuntimeSkillRefs model.AgentSkillRefs
	var capturedActiveSkillInstructions string
	var capturedSkillPolicy workerpkg.SkillPolicy
	var capturedInitialInstructions string
	var capturedPhaseGuidance string

	now := time.Now().UTC()
	agent := &model.Agent{
		ID:                    "agent-epic",
		WorkspaceID:           "ws-1",
		IsSystem:              true,
		Name:                  "Epic Planner",
		PresetKey:             model.AgentPresetEpicPlanner,
		Role:                  "Planner",
		Status:                "idle",
		RuntimeKind:           "native_sdk",
		Skills:                model.AgentSkillRefs{},
		TriggerMode:           "manual",
		AllowedTools:          json.RawMessage(`[]`),
		AllowedCommands:       json.RawMessage(`[]`),
		AllowedTargets:        json.RawMessage(`[]`),
		ApprovalMode:          "never",
		MaxConcurrentRuns:     1,
		DefaultInvocationMode: model.InvocationModeInteractive,
		CreatedAt:             now,
		UpdatedAt:             now,
	}
	if err := db.Create(agent).Error; err != nil {
		t.Fatalf("create agent: %v", err)
	}

	epic := &model.PMEpic{
		ID:                 "epic-1",
		WorkspaceID:        "ws-1",
		Name:               "Test all the best code",
		PlanningState:      model.EpicPlanningStateAwaitingSpecApproval,
		SpecClarifications: json.RawMessage(`[]`),
		CreatedAt:          now,
		UpdatedAt:          now,
	}
	if err := db.Create(epic).Error; err != nil {
		t.Fatalf("create epic: %v", err)
	}

	run := &model.AgentRun{
		ID:             "run-review-checkpoint",
		WorkspaceID:    "ws-1",
		AgentID:        agent.ID,
		TargetType:     "epic",
		TargetID:       epic.ID,
		RuntimeKind:    "native_sdk",
		InvocationMode: model.InvocationModeInteractive,
		Status:         model.AgentRunStatusRunning,
		PauseReason:    model.AgentRunPauseReasonNone,
		ApprovalState:  "not_required",
		Input:          json.RawMessage(`{}`),
		OutputSummary:  json.RawMessage(`{}`),
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	if err := db.Create(run).Error; err != nil {
		t.Fatalf("create run: %v", err)
	}

	activities := &AgentRunActivities{
		runRepo:         runRepo,
		runMessageRepo:  runMessageRepo,
		interactionRepo: interactionRepo,
		agentRepo:       agentRepo,
		artifactRepo:    artifactRepo,
		epicRepo:        epicRepo,
		docsDocRepo:     docsDocRepo,
		docsContentRepo: docsContentRepo,
		docsLinkRepo:    docsLinkRepo,
		runtimes: workerpkg.NewRuntimeRegistry(stubRuntimeAdapter{
			kind: "native_sdk",
			executeFn: func(execCtx *workerpkg.ExecutionContext, run *model.AgentRun) error {
				capturedSelectivePathEnabled = execCtx.NativeSelectivePathEnabled
				capturedRuntimeSkillRefs = append(model.AgentSkillRefs(nil), execCtx.RuntimeSkillRefs...)
				capturedActiveRuntimeSkillRefs = append(model.AgentSkillRefs(nil), execCtx.ActiveRuntimeSkillRefs...)
				capturedActiveSkillInstructions = execCtx.ActiveSkillInstructions
				capturedSkillPolicy = execCtx.SkillPolicy
				capturedInitialInstructions = execCtx.InitialInstructions
				capturedPhaseGuidance = execCtx.PhaseGuidance
				execCtx.LastExecutionResult = &workerpkg.ExecutionResult{
					AssistantText: "PRD review checkpoint requested.",
					ToolInvocations: []model.ToolInvocation{
						{
							ToolName: workerpkg.ToolPublishPRDDraft,
							Input:    json.RawMessage(`{"content":"# PRD\n\nDraft body"}`),
						},
						{
							ToolName: workerpkg.ToolRequestReviewCheckpoint,
							Input:    json.RawMessage(`{"phase":"prd","title":"PRD Review","summary":"Review the current PRD draft."}`),
						},
					},
				}
				return nil
			},
		}),
		wsPublisher: wsPublisher,
	}

	result, err := activities.ExecuteRunActivity(context.Background(), run.ID)
	if err != nil {
		t.Fatalf("ExecuteRunActivity returned error: %v", err)
	}
	if !result.WaitForApproval {
		t.Fatalf("expected WaitForApproval, got %#v", result)
	}
	if result.AwaitingInput || result.AwaitingAuth || result.ContinueExecution {
		t.Fatalf("unexpected execute result %#v", result)
	}
	if !capturedSelectivePathEnabled {
		t.Fatal("expected native selective path flag to be threaded into execution context")
	}
	if len(capturedRuntimeSkillRefs) == 0 {
		t.Fatal("expected runtime skill refs to be threaded into execution context")
	}
	if got := testAgentSkillRefKeys(capturedActiveRuntimeSkillRefs); len(got) != 4 || got[0] != "approval_protocol" || got[1] != "prd_authorship" || got[2] != "epic_state_routing" || got[3] != "general_agent_behavior" {
		t.Fatalf("unexpected active runtime skill refs %#v", got)
	}
	if strings.TrimSpace(capturedActiveSkillInstructions) == "" {
		t.Fatalf("expected active skill instructions to be threaded into execution context, got %q", capturedActiveSkillInstructions)
	}
	if strings.TrimSpace(capturedInitialInstructions) != "" {
		t.Fatalf("expected gated native path to suppress legacy initial instructions, got %q", capturedInitialInstructions)
	}
	if strings.TrimSpace(capturedPhaseGuidance) == "" {
		t.Fatalf("expected gated native path to populate phase guidance, got %q", capturedPhaseGuidance)
	}
	if !strings.Contains(capturedPhaseGuidance, "Current planning phase:") {
		t.Fatalf("expected gated native phase guidance to use explicit phase header, got %q", capturedPhaseGuidance)
	}
	if strings.Contains(capturedPhaseGuidance, "Use this sequence unless the human explicitly redirects you:") {
		t.Fatalf("did not expect legacy monolithic planner guidance in gated native phase guidance, got %q", capturedPhaseGuidance)
	}
	if got := completionRequiredInteractionKinds(capturedSkillPolicy); len(got) != 2 {
		t.Fatalf("unexpected active skill policy %#v", got)
	} else if _, ok := got[model.AgentRunInteractionKindApprovalRequest]; !ok {
		t.Fatalf("expected approval_request in active skill policy, got %#v", got)
	} else if _, ok := got[model.AgentRunInteractionKindRequestUserInput]; !ok {
		t.Fatalf("unexpected active skill policy %#v", got)
	}

	updatedRun, err := runRepo.GetByIDAny(context.Background(), run.ID)
	if err != nil {
		t.Fatalf("get updated run: %v", err)
	}
	if updatedRun == nil {
		t.Fatal("expected updated run")
	}
	if updatedRun.Status != model.AgentRunStatusPaused {
		t.Fatalf("expected paused run, got %#v", updatedRun)
	}
	if updatedRun.PauseReason != model.AgentRunPauseReasonHumanApproval {
		t.Fatalf("expected human approval pause reason, got %#v", updatedRun)
	}
	if updatedRun.ApprovalState != "pending" {
		t.Fatalf("expected pending approval state, got %#v", updatedRun)
	}

	interactions, err := interactionRepo.ListByRun(context.Background(), run.WorkspaceID, run.ID)
	if err != nil {
		t.Fatalf("list interactions: %v", err)
	}
	if len(interactions) != 1 {
		t.Fatalf("expected one interaction, got %#v", interactions)
	}
	if interactions[0].InteractionKind != model.AgentRunInteractionKindReviewCheckpoint {
		t.Fatalf("expected review checkpoint interaction, got %#v", interactions[0])
	}

	messages, err := runMessageRepo.ListByRun(context.Background(), run.WorkspaceID, run.ID)
	if err != nil {
		t.Fatalf("list run messages: %v", err)
	}
	if len(messages) == 0 {
		t.Fatal("expected persisted run messages")
	}
	if strings.Contains(messages[0].Content, "Run mode: interactive") {
		t.Fatalf("did not expect gated native first user prompt to persist phase guidance, got %q", messages[0].Content)
	}
}

func testAgentSkillRefKeys(refs model.AgentSkillRefs) []string {
	keys := make([]string, 0, len(refs))
	for _, ref := range refs {
		keys = append(keys, ref.Key)
	}
	return keys
}

func TestExecuteRunActivityRetriesReviewAgentCompletionWithoutRequiredInteraction(t *testing.T) {
	db := newPlannerApprovalTestDB(t)

	for _, stmt := range []string{
		`CREATE TABLE agent_run_messages (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			run_id TEXT NOT NULL,
			role TEXT NOT NULL,
			content TEXT NOT NULL,
			message_type TEXT NOT NULL,
			content_blocks TEXT,
			turn_segments TEXT,
			tool_invocations TEXT,
			token_usage TEXT,
			sequence_no INTEGER NOT NULL,
			created_at DATETIME
		)`,
		`CREATE TABLE agent_run_interactions (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			run_id TEXT NOT NULL,
			runtime_kind TEXT NOT NULL,
			interaction_kind TEXT NOT NULL,
			status TEXT NOT NULL,
			request_schema_version TEXT NOT NULL,
			response_schema_version TEXT,
			request_id TEXT,
			thread_id TEXT,
			turn_id TEXT,
			item_id TEXT,
			approval_id TEXT,
			assistant_message_sequence_no INTEGER,
			title TEXT,
			summary TEXT,
			request_payload TEXT NOT NULL,
			response_payload TEXT,
			runtime_metadata TEXT NOT NULL,
			resolved_by TEXT,
			resolved_at DATETIME,
			created_at DATETIME,
			updated_at DATETIME
		)`,
	} {
		if err := db.Exec(stmt).Error; err != nil {
			t.Fatalf("create test table: %v", err)
		}
	}

	runRepo := repository.NewAgentRunRepository(db)
	runMessageRepo := repository.NewAgentRunMessageRepository(db)
	interactionRepo := repository.NewAgentRunInteractionRepository(db)
	agentRepo := repository.NewAgentRepository(db)
	artifactRepo := repository.NewAgentRunArtifactRepository(db)
	epicRepo := repository.NewPMEpicRepository(db)

	now := time.Now().UTC()
	agent := &model.Agent{
		ID:                    "agent-review",
		WorkspaceID:           "ws-1",
		IsSystem:              true,
		Name:                  "Lens",
		PresetKey:             model.AgentPresetReviewAgent,
		Role:                  "Reviewer",
		Status:                "idle",
		RuntimeKind:           "native_sdk",
		Skills:                model.AgentSkillRefs{},
		TriggerMode:           "manual",
		AllowedTools:          json.RawMessage(`[]`),
		AllowedCommands:       json.RawMessage(`[]`),
		AllowedTargets:        json.RawMessage(`[]`),
		ApprovalMode:          "never",
		MaxConcurrentRuns:     1,
		DefaultInvocationMode: model.InvocationModeInteractive,
		CreatedAt:             now,
		UpdatedAt:             now,
	}
	if err := db.Create(agent).Error; err != nil {
		t.Fatalf("create agent: %v", err)
	}

	epic := &model.PMEpic{
		ID:                 "epic-review",
		WorkspaceID:        "ws-1",
		Name:               "Review epic",
		PlanningState:      model.EpicPlanningStateNotStarted,
		SpecClarifications: json.RawMessage(`[]`),
		CreatedAt:          now,
		UpdatedAt:          now,
	}
	if err := db.Create(epic).Error; err != nil {
		t.Fatalf("create epic: %v", err)
	}

	run := &model.AgentRun{
		ID:             "run-review-no-interaction",
		WorkspaceID:    "ws-1",
		AgentID:        agent.ID,
		TargetType:     "epic",
		TargetID:       epic.ID,
		RuntimeKind:    "native_sdk",
		InvocationMode: model.InvocationModeInteractive,
		Status:         model.AgentRunStatusRunning,
		PauseReason:    model.AgentRunPauseReasonNone,
		ApprovalState:  "not_required",
		Input:          json.RawMessage(`{}`),
		OutputSummary:  json.RawMessage(`{}`),
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	if err := db.Create(run).Error; err != nil {
		t.Fatalf("create run: %v", err)
	}

	activities := &AgentRunActivities{
		runRepo:         runRepo,
		runMessageRepo:  runMessageRepo,
		interactionRepo: interactionRepo,
		agentRepo:       agentRepo,
		artifactRepo:    artifactRepo,
		epicRepo:        epicRepo,
		runtimes: workerpkg.NewRuntimeRegistry(stubRuntimeAdapter{
			kind: "native_sdk",
			executeFn: func(execCtx *workerpkg.ExecutionContext, run *model.AgentRun) error {
				execCtx.LastExecutionResult = &workerpkg.ExecutionResult{
					AssistantText: "Findings\n\nHigh: The required alert rules are missing.\nOverall correctness: incorrect.",
				}
				return nil
			},
		}),
	}

	result, err := activities.ExecuteRunActivity(context.Background(), run.ID)
	if err != nil {
		t.Fatalf("expected auto-retry instead of failure, got %v", err)
	}
	if !result.ContinueExecution || result.WaitForApproval || result.AwaitingInput || result.AwaitingAuth {
		t.Fatalf("expected continue execution retry result, got %#v", result)
	}

	updatedRun, err := runRepo.GetByIDAny(context.Background(), run.ID)
	if err != nil {
		t.Fatalf("get updated run: %v", err)
	}
	if updatedRun == nil {
		t.Fatal("expected updated run")
	}
	if updatedRun.Status != model.AgentRunStatusRunning {
		t.Fatalf("expected run to remain running for retry, got %#v", updatedRun)
	}
	if updatedRun.ErrorMessage != nil && strings.TrimSpace(*updatedRun.ErrorMessage) != "" {
		t.Fatalf("expected no terminal error on auto-retry, got %#v", updatedRun.ErrorMessage)
	}

	messages, err := runMessageRepo.ListByRun(context.Background(), run.WorkspaceID, run.ID)
	if err != nil {
		t.Fatalf("list run messages: %v", err)
	}
	foundAssistantTurn := false
	foundRetryMessage := false
	for _, message := range messages {
		if message.Role == "assistant" && message.MessageType == "assistant_turn" && strings.Contains(message.Content, "Findings") {
			foundAssistantTurn = true
		}
		if message.Role == "user" && message.MessageType == "policy_retry" && strings.Contains(message.Content, "review_checkpoint handoff") {
			foundRetryMessage = true
		}
	}
	if !foundAssistantTurn {
		t.Fatalf("expected assistant findings message to persist before failure, got %#v", messages)
	}
	if !foundRetryMessage {
		t.Fatalf("expected policy retry message to be appended, got %#v", messages)
	}
}

func TestExecuteRunActivityThreadsNativeRepairGuidanceWithoutReplayingPolicyRetry(t *testing.T) {
	t.Setenv("AGENT_NATIVE_SELECTIVE_PLANNER_ENABLED", "true")

	db := newPlannerApprovalTestDB(t)
	for _, stmt := range []string{
		`CREATE TABLE agent_run_messages (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			run_id TEXT NOT NULL,
			role TEXT NOT NULL,
			message_type TEXT NOT NULL,
			content TEXT NOT NULL,
			content_blocks BLOB,
			turn_segments BLOB,
			tool_invocations BLOB,
			token_usage BLOB,
			sequence_no INTEGER NOT NULL,
			created_at DATETIME,
			updated_at DATETIME
		)`,
		`CREATE TABLE agent_run_interactions (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			run_id TEXT NOT NULL,
			runtime_kind TEXT NOT NULL,
			interaction_kind TEXT NOT NULL,
			status TEXT NOT NULL,
			request_schema_version TEXT NOT NULL,
			response_schema_version TEXT,
			request_id TEXT,
			thread_id TEXT,
			turn_id TEXT,
			item_id TEXT,
			approval_id TEXT,
			assistant_message_sequence_no INTEGER,
			title TEXT,
			summary TEXT,
			request_payload TEXT NOT NULL,
			response_payload TEXT,
			runtime_metadata TEXT NOT NULL,
			resolved_by TEXT,
			resolved_at DATETIME,
			created_at DATETIME,
			updated_at DATETIME
		)`,
	} {
		if err := db.Exec(stmt).Error; err != nil {
			t.Fatalf("create table: %v", err)
		}
	}

	runRepo := repository.NewAgentRunRepository(db)
	runMessageRepo := repository.NewAgentRunMessageRepository(db)
	interactionRepo := repository.NewAgentRunInteractionRepository(db)
	agentRepo := repository.NewAgentRepository(db)
	epicRepo := repository.NewPMEpicRepository(db)
	docsDocRepo := repository.NewDocsDocumentRepository(db)
	docsContentRepo := repository.NewDocsContentRepository(db)
	docsLinkRepo := repository.NewDocsLinkRepository(db)

	now := time.Now().UTC()
	agent := &model.Agent{
		ID:                    "agent-native-repair",
		WorkspaceID:           "ws-1",
		IsSystem:              true,
		Name:                  "Epic Planner",
		PresetKey:             model.AgentPresetEpicPlanner,
		Role:                  "Planner",
		Status:                "idle",
		RuntimeKind:           "native_sdk",
		Skills:                model.AgentSkillRefs{},
		TriggerMode:           "manual",
		AllowedTools:          json.RawMessage(`[]`),
		AllowedCommands:       json.RawMessage(`[]`),
		AllowedTargets:        json.RawMessage(`[]`),
		ApprovalMode:          "never",
		MaxConcurrentRuns:     1,
		DefaultInvocationMode: model.InvocationModeInteractive,
		CreatedAt:             now,
		UpdatedAt:             now,
	}
	if err := db.Create(agent).Error; err != nil {
		t.Fatalf("create agent: %v", err)
	}

	epic := &model.PMEpic{
		ID:                 "epic-native-repair",
		WorkspaceID:        "ws-1",
		Name:               "Selective repair guidance",
		PlanningState:      model.EpicPlanningStateAwaitingSpecApproval,
		SpecClarifications: json.RawMessage(`[]`),
		CreatedAt:          now,
		UpdatedAt:          now,
	}
	if err := db.Create(epic).Error; err != nil {
		t.Fatalf("create epic: %v", err)
	}

	run := &model.AgentRun{
		ID:             "run-native-repair",
		WorkspaceID:    "ws-1",
		AgentID:        agent.ID,
		TargetType:     "epic",
		TargetID:       epic.ID,
		RuntimeKind:    "native_sdk",
		InvocationMode: model.InvocationModeInteractive,
		Status:         model.AgentRunStatusRunning,
		PauseReason:    model.AgentRunPauseReasonNone,
		ApprovalState:  "not_required",
		Input:          json.RawMessage(`{}`),
		OutputSummary:  json.RawMessage(`{}`),
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	if err := db.Create(run).Error; err != nil {
		t.Fatalf("create run: %v", err)
	}

	activities := &AgentRunActivities{
		runRepo:         runRepo,
		runMessageRepo:  runMessageRepo,
		interactionRepo: interactionRepo,
		agentRepo:       agentRepo,
		epicRepo:        epicRepo,
		docsDocRepo:     docsDocRepo,
		docsContentRepo: docsContentRepo,
		docsLinkRepo:    docsLinkRepo,
		runtimes: workerpkg.NewRuntimeRegistry(stubRuntimeAdapter{
			kind: "native_sdk",
			executeFn: func(execCtx *workerpkg.ExecutionContext, run *model.AgentRun) error {
				if !strings.Contains(strings.ToLower(execCtx.RepairGuidance), "same turn") || !strings.Contains(execCtx.RepairGuidance, "preview_panel_key") {
					t.Fatalf("expected repair guidance in execution context, got %q", execCtx.RepairGuidance)
				}
				for _, message := range execCtx.ConversationHistory {
					if strings.Contains(message.Content, "System correction:") {
						t.Fatalf("expected policy_retry to stay out of replay history, got %#v", execCtx.ConversationHistory)
					}
				}
				execCtx.LastExecutionResult = &workerpkg.ExecutionResult{
					AssistantText: "Need clarification before continuing.",
					ToolInvocations: []model.ToolInvocation{
						{
							ToolName: workerpkg.ToolRequestUserInput,
							Input: json.RawMessage(`{
								"questions": [
									{
										"id": "repair-q1",
										"header": "Priority",
										"question": "Which edge case should be prioritized first?",
										"options": [
											{ "label": "Preview binding" },
											{ "label": "Approval handoff" }
										]
									}
								]
							}`),
						},
					},
				}
				return nil
			},
		}),
	}

	if _, err := activities.createRunMessage(context.Background(), run, "user", "prompt", "Initial prompt", nil, nil, nil, nil); err != nil {
		t.Fatalf("create prompt message: %v", err)
	}
	if _, err := activities.createRunMessage(context.Background(), run, "assistant", "assistant_turn", "Drafted the PRD.", nil, nil, nil, nil); err != nil {
		t.Fatalf("create assistant message: %v", err)
	}
	retryContent := "System correction: the previous turn requested approval without binding it to a same-turn preview."
	if _, err := activities.createRunMessage(context.Background(), run, "user", "policy_retry", retryContent, nil, nil, nil, nil); err != nil {
		t.Fatalf("create policy retry message: %v", err)
	}

	result, err := activities.ExecuteRunActivity(context.Background(), run.ID)
	if err != nil {
		t.Fatalf("ExecuteRunActivity returned error: %v", err)
	}
	if !result.AwaitingInput {
		t.Fatalf("expected AwaitingInput, got %#v", result)
	}

	messages, err := runMessageRepo.ListByRun(context.Background(), run.WorkspaceID, run.ID)
	if err != nil {
		t.Fatalf("list run messages: %v", err)
	}
	foundPolicyRetry := false
	for _, message := range messages {
		if strings.TrimSpace(message.MessageType) == "policy_retry" && strings.Contains(message.Content, "same-turn preview") {
			foundPolicyRetry = true
			break
		}
	}
	if !foundPolicyRetry {
		t.Fatalf("expected persisted policy_retry marker to remain present, got %#v", messages)
	}
}

func TestSynthesizeCompletionInteractionFallbackUsesStructuredReviewBlockForCodex(t *testing.T) {
	dbName := fmt.Sprintf("file:review-fallback-codex-%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dbName), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	for _, stmt := range []string{
		`CREATE TABLE agent_run_artifacts (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			run_id TEXT NOT NULL,
			artifact_type TEXT NOT NULL,
			format TEXT NOT NULL,
			storage_mode TEXT NOT NULL,
			inline_content TEXT,
			object_key TEXT,
			metadata TEXT NOT NULL,
			sequence_no INTEGER NOT NULL,
			created_at DATETIME
		)`,
		`CREATE TABLE agent_run_interactions (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			run_id TEXT NOT NULL,
			runtime_kind TEXT NOT NULL,
			interaction_kind TEXT NOT NULL,
			status TEXT NOT NULL,
			request_schema_version TEXT NOT NULL,
			response_schema_version TEXT,
			request_id TEXT,
			thread_id TEXT,
			turn_id TEXT,
			item_id TEXT,
			approval_id TEXT,
			assistant_message_sequence_no INTEGER,
			title TEXT,
			summary TEXT,
			request_payload TEXT NOT NULL,
			response_payload TEXT,
			runtime_metadata TEXT NOT NULL,
			resolved_by TEXT,
			resolved_at DATETIME,
			created_at DATETIME,
			updated_at DATETIME
		)`,
	} {
		if err := db.Exec(stmt).Error; err != nil {
			t.Fatalf("create test table: %v", err)
		}
	}

	artifactRepo := repository.NewAgentRunArtifactRepository(db)
	interactionRepo := repository.NewAgentRunInteractionRepository(db)

	now := time.Now().UTC()
	state := &resolvedRunState{
		run: &model.AgentRun{
			ID:             "run-review-codex-no-interaction",
			WorkspaceID:    "ws-1",
			AgentID:        "agent-review-codex",
			TargetType:     "epic",
			TargetID:       "epic-review-codex",
			RuntimeKind:    "codex",
			InvocationMode: model.InvocationModeInteractive,
			Status:         model.AgentRunStatusRunning,
			PauseReason:    model.AgentRunPauseReasonNone,
			ApprovalState:  "not_required",
			Input:          json.RawMessage(`{}`),
			OutputSummary:  json.RawMessage(`{}`),
			CreatedAt:      now,
			UpdatedAt:      now,
		},
		agent: &model.Agent{
			ID:                    "agent-review-codex",
			WorkspaceID:           "ws-1",
			IsSystem:              true,
			Name:                  "Lens",
			PresetKey:             model.AgentPresetReviewAgent,
			Role:                  "Reviewer",
			Status:                "idle",
			RuntimeKind:           "codex",
			Skills:                model.AgentSkillRefs{},
			TriggerMode:           "manual",
			AllowedTools:          json.RawMessage(`[]`),
			AllowedCommands:       json.RawMessage(`[]`),
			AllowedTargets:        json.RawMessage(`[]`),
			ApprovalMode:          "never",
			MaxConcurrentRuns:     1,
			DefaultInvocationMode: model.InvocationModeInteractive,
			CreatedAt:             now,
			UpdatedAt:             now,
		},
		skillPolicy: workerpkg.SkillPolicy{
			CompletionRequiresInteractionKinds: []string{model.AgentRunInteractionKindReviewCheckpoint},
			InteractionContracts: []workerpkg.SkillInteractionContract{
				{
					Kind:   workerpkg.InteractionKindReviewCheckpoint,
					Schema: "review_checkpoint_v1",
					Transports: map[string]workerpkg.SkillInteractionTransport{
						"codex": {Type: workerpkg.InteractionTransportTypeFencedJSON, BlockLabel: "helpin-review"},
					},
				},
			},
		},
	}

	activities := &AgentRunActivities{
		artifactRepo:    artifactRepo,
		interactionRepo: interactionRepo,
	}
	assistantMessage := &model.AgentRunMessage{
		SequenceNo:  3,
		Role:        "assistant",
		MessageType: "assistant_turn",
		Content:     "Findings\n\nMedium: visitor_type misclassifies blank user IDs as identified.\n\n```helpin-review\n{\"title\":\"Lens review findings\",\"summary\":\"One concrete classification bug found.\",\"findings\":[{\"title\":\"Blank user IDs are treated as identified\",\"body\":\"The new visitor_type logic treats an empty user.id string as identified instead of anonymous.\",\"priority\":\"P1\",\"confidence\":0.94,\"code_location\":\"rust-capture/src/enrichment/handler.rs:284\"}],\"overall_correctness\":\"incorrect\",\"overall_explanation\":\"The new classification logic regresses visitor typing for blank IDs.\",\"overall_confidence_score\":0.92}\n```",
	}

	approval, input, err := activities.synthesizeCompletionInteractionFallback(context.Background(), state, assistantMessage)
	if err != nil {
		t.Fatalf("expected synthesized checkpoint instead of failure, got %v", err)
	}
	if input != nil {
		t.Fatalf("expected no synthesized input request, got %#v", input)
	}
	if approval == nil {
		t.Fatal("expected synthesized approval request")
	}
	if approval.Title != "Lens review findings" || approval.Summary != "One concrete classification bug found." {
		t.Fatalf("expected structured synthesized approval request, got %#v", approval)
	}
	if approval.OverallCorrectness != "incorrect" || approval.OverallExplanation == "" || len(approval.Findings) != 1 {
		t.Fatalf("expected structured review verdict, got %#v", approval)
	}
	if approval.Findings[0].ID != "finding_1" || approval.Findings[0].Title != "Blank user IDs are treated as identified" {
		t.Fatalf("expected structured finding title, got %#v", approval.Findings[0])
	}

	interactions, err := interactionRepo.ListByRun(context.Background(), state.run.WorkspaceID, state.run.ID)
	if err != nil {
		t.Fatalf("list interactions: %v", err)
	}
	if len(interactions) != 1 {
		t.Fatalf("expected one synthesized interaction, got %#v", interactions)
	}
	if interactions[0].InteractionKind != model.AgentRunInteractionKindReviewCheckpoint {
		t.Fatalf("expected synthesized review checkpoint, got %#v", interactions[0])
	}
	if interactions[0].Title == nil || *interactions[0].Title != "Lens review findings" {
		t.Fatalf("expected structured title, got %#v", interactions[0])
	}
	if interactions[0].Summary == nil || *interactions[0].Summary != "One concrete classification bug found." {
		t.Fatalf("expected structured checkpoint summary, got %#v", interactions[0])
	}

	var request model.ReviewCheckpointRequest
	if err := json.Unmarshal(interactions[0].RequestPayload, &request); err != nil {
		t.Fatalf("unmarshal synthesized approval request: %v", err)
	}
	if request.OverallCorrectness != "incorrect" || request.OverallExplanation == "" || len(request.Findings) != 1 {
		t.Fatalf("expected structured approval payload, got %#v", request)
	}
	if request.Findings[0].ID != "finding_1" || request.Findings[0].Title != "Blank user IDs are treated as identified" {
		t.Fatalf("expected structured finding title, got %#v", request.Findings[0])
	}
}

func TestSynthesizeCompletionInteractionFallbackPrefersReviewCheckpointWhenUserInputIsAlsoRequired(t *testing.T) {
	dbName := fmt.Sprintf("file:completion-fallback-review-followup-%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dbName), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	for _, stmt := range []string{
		`CREATE TABLE agent_run_artifacts (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			run_id TEXT NOT NULL,
			artifact_type TEXT NOT NULL,
			format TEXT NOT NULL,
			storage_mode TEXT NOT NULL,
			inline_content TEXT,
			object_key TEXT,
			metadata TEXT NOT NULL,
			sequence_no INTEGER NOT NULL,
			created_at DATETIME
		)`,
		`CREATE TABLE agent_run_interactions (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			run_id TEXT NOT NULL,
			runtime_kind TEXT NOT NULL,
			interaction_kind TEXT NOT NULL,
			status TEXT NOT NULL,
			request_schema_version TEXT NOT NULL,
			response_schema_version TEXT,
			request_id TEXT,
			thread_id TEXT,
			turn_id TEXT,
			item_id TEXT,
			approval_id TEXT,
			assistant_message_sequence_no INTEGER,
			title TEXT,
			summary TEXT,
			request_payload BLOB NOT NULL,
			response_payload BLOB,
			runtime_metadata BLOB NOT NULL,
			expires_at DATETIME,
			resolved_by TEXT,
			resolved_at DATETIME,
			created_at DATETIME,
			updated_at DATETIME
		)`,
	} {
		if err := db.Exec(stmt).Error; err != nil {
			t.Fatalf("create table: %v", err)
		}
	}

	artifactRepo := repository.NewAgentRunArtifactRepository(db)
	interactionRepo := repository.NewAgentRunInteractionRepository(db)
	now := time.Now().UTC()
	state := &resolvedRunState{
		run: &model.AgentRun{
			ID:             "run-review-followup",
			WorkspaceID:    "ws-1",
			AgentID:        "agent-review-codex",
			TargetType:     "epic",
			TargetID:       "epic-review-codex",
			RuntimeKind:    "codex",
			InvocationMode: model.InvocationModeInteractive,
			Status:         model.AgentRunStatusRunning,
			PauseReason:    model.AgentRunPauseReasonNone,
			ApprovalState:  "not_required",
			Input:          json.RawMessage(`{}`),
			OutputSummary:  json.RawMessage(`{}`),
			CreatedAt:      now,
			UpdatedAt:      now,
		},
		agent: &model.Agent{
			ID:                    "agent-review-codex",
			WorkspaceID:           "ws-1",
			IsSystem:              true,
			Name:                  "Lens",
			PresetKey:             model.AgentPresetReviewAgent,
			Role:                  "Reviewer",
			Status:                "idle",
			RuntimeKind:           "codex",
			Skills:                model.AgentSkillRefs{},
			TriggerMode:           "manual",
			AllowedTools:          json.RawMessage(`[]`),
			AllowedCommands:       json.RawMessage(`[]`),
			AllowedTargets:        json.RawMessage(`[]`),
			ApprovalMode:          "never",
			MaxConcurrentRuns:     1,
			DefaultInvocationMode: model.InvocationModeInteractive,
			CreatedAt:             now,
			UpdatedAt:             now,
		},
		skillPolicy: workerpkg.SkillPolicy{
			CompletionRequiresInteractionKinds: []string{
				model.AgentRunInteractionKindReviewCheckpoint,
				model.AgentRunInteractionKindRequestUserInput,
			},
			InteractionContracts: []workerpkg.SkillInteractionContract{
				{
					Kind:   workerpkg.InteractionKindReviewCheckpoint,
					Schema: "review_checkpoint_v1",
					Transports: map[string]workerpkg.SkillInteractionTransport{
						"codex": {Type: workerpkg.InteractionTransportTypeFencedJSON, BlockLabel: "helpin-review"},
					},
				},
				{
					Kind:   workerpkg.InteractionKindRequestUserInput,
					Schema: "request_user_input_v1",
					Transports: map[string]workerpkg.SkillInteractionTransport{
						"codex": {Type: workerpkg.InteractionTransportTypeRuntimeBridge},
					},
				},
			},
		},
	}

	activities := &AgentRunActivities{
		artifactRepo:    artifactRepo,
		interactionRepo: interactionRepo,
	}
	assistantMessage := &model.AgentRunMessage{
		SequenceNo:  4,
		Role:        "assistant",
		MessageType: "assistant_turn",
		Content:     "The review is clean overall. I can explain the reasoning in more detail if helpful.\n\n```helpin-review\n{\"title\":\"Lens review findings\",\"summary\":\"No issues found.\",\"findings\":[],\"overall_correctness\":\"correct\",\"overall_explanation\":\"I did not find correctness issues in this pass.\",\"overall_confidence_score\":0.88}\n```",
	}

	approval, input, err := activities.synthesizeCompletionInteractionFallback(context.Background(), state, assistantMessage)
	if err != nil {
		t.Fatalf("expected synthesized review checkpoint, got %v", err)
	}
	if input != nil {
		t.Fatalf("expected no synthesized input request, got %#v", input)
	}
	if approval == nil {
		t.Fatal("expected synthesized approval request")
	}
	if approval.Title != "Lens review findings" || approval.Summary != "No issues found." {
		t.Fatalf("unexpected synthesized approval request %#v", approval)
	}

	interactions, err := interactionRepo.ListByRun(context.Background(), state.run.WorkspaceID, state.run.ID)
	if err != nil {
		t.Fatalf("list interactions: %v", err)
	}
	if len(interactions) != 1 {
		t.Fatalf("expected one synthesized interaction, got %#v", interactions)
	}
	if interactions[0].InteractionKind != model.AgentRunInteractionKindReviewCheckpoint {
		t.Fatalf("expected synthesized review checkpoint, got %#v", interactions[0])
	}
}

func TestParseStructuredReviewApprovalRequest(t *testing.T) {
	request, ok := parseStructuredReviewApprovalRequest(workerpkg.SkillPolicy{
		InteractionContracts: []workerpkg.SkillInteractionContract{
			{
				Kind:   workerpkg.InteractionKindReviewCheckpoint,
				Schema: "review_checkpoint_v1",
				Transports: map[string]workerpkg.SkillInteractionTransport{
					"codex": {Type: workerpkg.InteractionTransportTypeFencedJSON, BlockLabel: "review-json"},
				},
			},
		},
	}, "codex", strings.TrimSpace(`
Findings

1. High: Something is wrong.

`+"```review-json\n"+`{"title":"Review findings","summary":"Two issues found.","findings":[{"title":"Broken case","body":"Details","priority":"P2","code_location":"app.rs:10"}],"overall_correctness":"incorrect","overall_explanation":"The change regresses behavior.","overall_confidence_score":0.81}`+"\n```"))
	if !ok || request == nil {
		t.Fatal("expected structured review request to parse")
	}
	if request.Title != "Review findings" || request.Summary != "Two issues found." {
		t.Fatalf("unexpected request header %#v", request)
	}
	if request.OverallCorrectness != "incorrect" || request.OverallExplanation != "The change regresses behavior." {
		t.Fatalf("unexpected overall verdict %#v", request)
	}
	if len(request.Findings) != 1 || request.Findings[0].ID != "finding_1" || request.Findings[0].CodeLocation != "app.rs:10" {
		t.Fatalf("unexpected findings %#v", request.Findings)
	}
}

func TestParseStructuredReviewApprovalRequestDefaultsToLegacyBlockLabel(t *testing.T) {
	content := strings.TrimSpace(`
Findings

1. High: Something is wrong.

` + "```helpin-review\n" + `{"title":"Review findings","summary":"Two issues found.","findings":[{"title":"Broken case","body":"Details","priority":"P2","code_location":"app.rs:10"}],"overall_correctness":"incorrect","overall_explanation":"The change regresses behavior.","overall_confidence_score":0.81}` + "\n```")

	request, ok := parseStructuredReviewApprovalRequest(workerpkg.SkillPolicy{}, "codex", content)
	if !ok || request == nil {
		t.Fatal("expected structured review request to parse")
	}
	if request.Title != "Review findings" || request.Summary != "Two issues found." {
		t.Fatalf("unexpected request header %#v", request)
	}
	if request.OverallCorrectness != "incorrect" || request.OverallExplanation != "The change regresses behavior." {
		t.Fatalf("unexpected overall verdict %#v", request)
	}
	if len(request.Findings) != 1 || request.Findings[0].ID != "finding_1" || request.Findings[0].CodeLocation != "app.rs:10" {
		t.Fatalf("unexpected findings %#v", request.Findings)
	}
}

func TestSynthesizeCompletionInteractionFallbackAllowsTerminalCleanImplementationReview(t *testing.T) {
	dbName := fmt.Sprintf("file:terminal-clean-implementation-review-%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dbName), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	for _, stmt := range []string{
		`CREATE TABLE agent_run_artifacts (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			run_id TEXT NOT NULL,
			artifact_type TEXT NOT NULL,
			format TEXT NOT NULL,
			storage_mode TEXT NOT NULL,
			inline_content TEXT,
			object_key TEXT,
			metadata TEXT NOT NULL,
			sequence_no INTEGER NOT NULL,
			created_at DATETIME
		)`,
		`CREATE TABLE agent_run_interactions (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			run_id TEXT NOT NULL,
			runtime_kind TEXT NOT NULL,
			interaction_kind TEXT NOT NULL,
			status TEXT NOT NULL,
			request_schema_version TEXT NOT NULL,
			response_schema_version TEXT,
			request_id TEXT,
			thread_id TEXT,
			turn_id TEXT,
			item_id TEXT,
			approval_id TEXT,
			assistant_message_sequence_no INTEGER,
			title TEXT,
			summary TEXT,
			request_payload TEXT NOT NULL,
			response_payload TEXT,
			runtime_metadata TEXT NOT NULL,
			resolved_by TEXT,
			resolved_at DATETIME,
			created_at DATETIME,
			updated_at DATETIME
		)`,
	} {
		if err := db.Exec(stmt).Error; err != nil {
			t.Fatalf("create table: %v", err)
		}
	}

	artifactRepo := repository.NewAgentRunArtifactRepository(db)
	interactionRepo := repository.NewAgentRunInteractionRepository(db)

	now := time.Now().UTC()
	state := &resolvedRunState{
		run: &model.AgentRun{
			ID:             "run-terminal-review",
			WorkspaceID:    "ws-1",
			RuntimeKind:    "codex",
			InvocationMode: model.InvocationModeInteractive,
			CreatedAt:      now,
			UpdatedAt:      now,
		},
		agent: &model.Agent{
			ID:                    "agent-1",
			WorkspaceID:           "ws-1",
			Name:                  "Lens",
			PresetKey:             model.AgentPresetReviewAgent,
			Role:                  "Reviewer",
			Status:                "idle",
			RuntimeKind:           "codex",
			Skills:                model.AgentSkillRefs{},
			TriggerMode:           "manual",
			AllowedTools:          json.RawMessage(`[]`),
			AllowedCommands:       json.RawMessage(`[]`),
			AllowedTargets:        json.RawMessage(`[]`),
			ApprovalMode:          "never",
			MaxConcurrentRuns:     1,
			DefaultInvocationMode: model.InvocationModeInteractive,
			CreatedAt:             now,
			UpdatedAt:             now,
		},
		skillPolicy: workerpkg.SkillPolicy{
			CompletionRequiresInteractionKinds: []string{
				model.AgentRunInteractionKindReviewCheckpoint,
				model.AgentRunInteractionKindRequestUserInput,
			},
			InteractionContracts: []workerpkg.SkillInteractionContract{
				{
					Kind:   workerpkg.InteractionKindReviewCheckpoint,
					Schema: "review_checkpoint_v1",
					Transports: map[string]workerpkg.SkillInteractionTransport{
						"codex": {Type: workerpkg.InteractionTransportTypeFencedJSON, BlockLabel: "helpin-review"},
					},
				},
			},
		},
	}

	activities := &AgentRunActivities{
		artifactRepo:    artifactRepo,
		interactionRepo: interactionRepo,
	}
	assistantMessage := &model.AgentRunMessage{
		SequenceNo:  8,
		Role:        "assistant",
		MessageType: "assistant_turn",
		Content:     "Final verification complete.\n\n```helpin-review\n{\"phase\":\"implementation\",\"title\":\"Producer alert findings already implemented\",\"summary\":\"No new code changes were required in this turn.\",\"findings\":[],\"overall_correctness\":\"correct\",\"overall_explanation\":\"The branch already reflects the approved fixes and the clean re-review passed.\",\"overall_confidence_score\":0.96}\n```",
	}

	approval, input, err := activities.synthesizeCompletionInteractionFallback(context.Background(), state, assistantMessage)
	if err != nil {
		t.Fatalf("expected terminal implementation review to avoid checkpoint synthesis, got %v", err)
	}
	if approval != nil || input != nil {
		t.Fatalf("expected no synthesized interaction, got approval=%#v input=%#v", approval, input)
	}
	if err := activities.enforceCompletionInteractionPolicy(context.Background(), state, assistantMessage); err != nil {
		t.Fatalf("expected clean implementation review to satisfy completion policy, got %v", err)
	}

	interactions, err := interactionRepo.ListByRun(context.Background(), state.run.WorkspaceID, state.run.ID)
	if err != nil {
		t.Fatalf("list interactions: %v", err)
	}
	if len(interactions) != 0 {
		t.Fatalf("expected no review interaction for terminal clean implementation review, got %#v", interactions)
	}

	artifacts, err := artifactRepo.ListByRun(context.Background(), state.run.WorkspaceID, state.run.ID)
	if err != nil {
		t.Fatalf("list artifacts: %v", err)
	}
	if len(artifacts) != 1 || artifacts[0].ArtifactType != model.AgentRunArtifactTypeReviewFindings {
		t.Fatalf("expected one persisted review findings artifact, got %#v", artifacts)
	}
}

func TestPostApprovalImplementationTurnCanCompleteWithoutAnotherReviewCheckpoint(t *testing.T) {
	dbName := fmt.Sprintf("file:post-approval-implementation-completion-%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dbName), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	for _, stmt := range []string{
		`CREATE TABLE agent_run_artifacts (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			run_id TEXT NOT NULL,
			artifact_type TEXT NOT NULL,
			format TEXT NOT NULL,
			storage_mode TEXT NOT NULL,
			inline_content TEXT,
			object_key TEXT,
			metadata TEXT NOT NULL,
			sequence_no INTEGER NOT NULL,
			created_at DATETIME
		)`,
		`CREATE TABLE agent_run_interactions (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			run_id TEXT NOT NULL,
			runtime_kind TEXT NOT NULL,
			interaction_kind TEXT NOT NULL,
			status TEXT NOT NULL,
			request_schema_version TEXT NOT NULL,
			response_schema_version TEXT,
			request_id TEXT,
			thread_id TEXT,
			turn_id TEXT,
			item_id TEXT,
			approval_id TEXT,
			assistant_message_sequence_no INTEGER,
			title TEXT,
			summary TEXT,
			request_payload TEXT NOT NULL,
			response_payload TEXT,
			runtime_metadata TEXT NOT NULL,
			resolved_by TEXT,
			resolved_at DATETIME,
			created_at DATETIME,
			updated_at DATETIME
		)`,
	} {
		if err := db.Exec(stmt).Error; err != nil {
			t.Fatalf("create table: %v", err)
		}
	}

	artifactRepo := repository.NewAgentRunArtifactRepository(db)
	interactionRepo := repository.NewAgentRunInteractionRepository(db)

	now := time.Now().UTC()
	state := &resolvedRunState{
		run: &model.AgentRun{
			ID:             "run-post-approval-impl",
			WorkspaceID:    "ws-1",
			RuntimeKind:    "codex",
			InvocationMode: model.InvocationModeInteractive,
			CreatedAt:      now,
			UpdatedAt:      now,
		},
		agent: &model.Agent{
			ID:                    "agent-1",
			WorkspaceID:           "ws-1",
			Name:                  "Lens",
			PresetKey:             model.AgentPresetReviewAgent,
			Role:                  "Reviewer",
			Status:                "idle",
			RuntimeKind:           "codex",
			Skills:                model.AgentSkillRefs{},
			TriggerMode:           "manual",
			AllowedTools:          json.RawMessage(`[]`),
			AllowedCommands:       json.RawMessage(`[]`),
			AllowedTargets:        json.RawMessage(`[]`),
			ApprovalMode:          "never",
			MaxConcurrentRuns:     1,
			DefaultInvocationMode: model.InvocationModeInteractive,
			CreatedAt:             now,
			UpdatedAt:             now,
		},
		skillPolicy: workerpkg.SkillPolicy{
			CompletionRequiresInteractionKinds: []string{
				model.AgentRunInteractionKindReviewCheckpoint,
				model.AgentRunInteractionKindRequestUserInput,
			},
			InteractionContracts: []workerpkg.SkillInteractionContract{
				{
					Kind:   workerpkg.InteractionKindReviewCheckpoint,
					Schema: "review_checkpoint_v1",
					Transports: map[string]workerpkg.SkillInteractionTransport{
						"codex": {Type: workerpkg.InteractionTransportTypeFencedJSON, BlockLabel: "helpin-review"},
					},
				},
			},
		},
	}

	assistantSequenceNo := 5
	resolvedAt := now.Add(time.Second)
	resolvedBy := "user-1"
	responseSchemaVersion := model.AgentRunInteractionSchemaVersionHelpinV1
	if err := interactionRepo.Create(context.Background(), &model.AgentRunInteraction{
		ID:                         "interaction-approved-review-1",
		WorkspaceID:                state.run.WorkspaceID,
		RunID:                      state.run.ID,
		RuntimeKind:                "codex",
		InteractionKind:            model.AgentRunInteractionKindReviewCheckpoint,
		Status:                     model.AgentRunInteractionStatusResolved,
		RequestSchemaVersion:       model.AgentRunInteractionSchemaVersionHelpinV1,
		ResponseSchemaVersion:      &responseSchemaVersion,
		AssistantMessageSequenceNo: &assistantSequenceNo,
		RequestPayload:             json.RawMessage(`{"phase":"review_findings","title":"Lens review findings","findings":[{"id":"finding_1","title":"Regression A"}]}`),
		ResponsePayload:            json.RawMessage(`{"decision":"approve","selection_mode":"selected","selected_finding_ids":["finding_1"]}`),
		RuntimeMetadata:            json.RawMessage(`{"runtime_kind":"codex"}`),
		ResolvedBy:                 &resolvedBy,
		ResolvedAt:                 &resolvedAt,
		CreatedAt:                  now,
		UpdatedAt:                  resolvedAt,
	}); err != nil {
		t.Fatalf("create interaction: %v", err)
	}

	activities := &AgentRunActivities{
		artifactRepo:    artifactRepo,
		interactionRepo: interactionRepo,
	}
	assistantMessage := &model.AgentRunMessage{
		SequenceNo:  6,
		Role:        "assistant",
		MessageType: "assistant_turn",
		Content:     "Implemented the approved fix, ran focused validation, and updated the branch summary.",
	}

	approval, input, err := activities.synthesizeCompletionInteractionFallback(context.Background(), state, assistantMessage)
	if err != nil {
		t.Fatalf("expected implementation follow-up to avoid another checkpoint, got %v", err)
	}
	if approval != nil || input != nil {
		t.Fatalf("expected no synthesized interaction, got approval=%#v input=%#v", approval, input)
	}
	if err := activities.enforceCompletionInteractionPolicy(context.Background(), state, assistantMessage); err != nil {
		t.Fatalf("expected approved implementation turn to satisfy completion policy, got %v", err)
	}
}

func TestReviewCheckpointForPlanningPhaseRequiresMatchingCurrentTurnPreview(t *testing.T) {
	dbName := fmt.Sprintf("file:review-checkpoint-preview-%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dbName), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	for _, stmt := range []string{
		`CREATE TABLE agent_run_artifacts (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			run_id TEXT NOT NULL,
			artifact_type TEXT NOT NULL,
			format TEXT NOT NULL,
			storage_mode TEXT NOT NULL,
			inline_content TEXT,
			object_key TEXT,
			metadata TEXT NOT NULL,
			sequence_no INTEGER NOT NULL,
			created_at DATETIME
		)`,
		`CREATE TABLE agent_run_interactions (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			run_id TEXT NOT NULL,
			runtime_kind TEXT NOT NULL,
			interaction_kind TEXT NOT NULL,
			status TEXT NOT NULL,
			request_schema_version TEXT NOT NULL,
			response_schema_version TEXT,
			request_id TEXT,
			thread_id TEXT,
			turn_id TEXT,
			item_id TEXT,
			approval_id TEXT,
			assistant_message_sequence_no INTEGER,
			title TEXT,
			summary TEXT,
			request_payload TEXT NOT NULL,
			response_payload TEXT,
			runtime_metadata TEXT NOT NULL,
			resolved_by TEXT,
			resolved_at DATETIME,
			created_at DATETIME,
			updated_at DATETIME
		)`,
	} {
		if err := db.Exec(stmt).Error; err != nil {
			t.Fatalf("create table: %v", err)
		}
	}

	now := time.Now().UTC()
	interactionRepo := repository.NewAgentRunInteractionRepository(db)
	artifactRepo := repository.NewAgentRunArtifactRepository(db)
	activities := &AgentRunActivities{
		artifactRepo:    artifactRepo,
		interactionRepo: interactionRepo,
	}
	state := &resolvedRunState{
		run: &model.AgentRun{
			ID:          "run-planner-preview",
			WorkspaceID: "ws-1",
			RuntimeKind: "native_sdk",
		},
		agent: &model.Agent{
			ID:          "agent-1",
			WorkspaceID: "ws-1",
			PresetKey:   model.AgentPresetReviewAgent,
			RuntimeKind: "native_sdk",
		},
		skillPolicy: workerpkg.SkillPolicy{
			CompletionRequiresInteractionKinds: []string{
				model.AgentRunInteractionKindReviewCheckpoint,
				model.AgentRunInteractionKindRequestUserInput,
			},
		},
	}

	assistantSequenceNo := 4
	if err := interactionRepo.Create(context.Background(), &model.AgentRunInteraction{
		ID:                         "interaction-1",
		WorkspaceID:                state.run.WorkspaceID,
		RunID:                      state.run.ID,
		RuntimeKind:                "native_sdk",
		InteractionKind:            model.AgentRunInteractionKindReviewCheckpoint,
		Status:                     model.AgentRunInteractionStatusPending,
		RequestSchemaVersion:       model.AgentRunInteractionSchemaVersionHelpinV1,
		AssistantMessageSequenceNo: &assistantSequenceNo,
		RequestPayload:             json.RawMessage(`{"phase":"tasks","title":"Approve task plan","summary":"Review the proposed tasks"}`),
		RuntimeMetadata:            json.RawMessage(`{"runtime_kind":"native_sdk"}`),
		CreatedAt:                  now,
		UpdatedAt:                  now,
	}); err != nil {
		t.Fatalf("create interaction: %v", err)
	}

	err = activities.enforceCompletionInteractionPolicy(context.Background(), state, &model.AgentRunMessage{
		SequenceNo:  assistantSequenceNo,
		Role:        "assistant",
		MessageType: "assistant_turn",
		Content:     "Please review the task plan.",
	})
	if err == nil {
		t.Fatal("expected missing preview validation error")
	}
	if !strings.Contains(err.Error(), "review_checkpoint requires a same-turn preview before requesting approval") {
		t.Fatalf("expected planner preview validation error, got %v", err)
	}
}

func TestReviewCheckpointWithExplicitPreviewPanelKeyRequiresMatchingCurrentTurnPreview(t *testing.T) {
	dbName := fmt.Sprintf("file:review-checkpoint-explicit-preview-%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dbName), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	for _, stmt := range []string{
		`CREATE TABLE agent_run_artifacts (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			run_id TEXT NOT NULL,
			artifact_type TEXT NOT NULL,
			format TEXT NOT NULL,
			storage_mode TEXT NOT NULL,
			inline_content TEXT,
			object_key TEXT,
			metadata TEXT NOT NULL,
			sequence_no INTEGER NOT NULL,
			created_at DATETIME
		)`,
		`CREATE TABLE agent_run_interactions (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			run_id TEXT NOT NULL,
			runtime_kind TEXT NOT NULL,
			interaction_kind TEXT NOT NULL,
			status TEXT NOT NULL,
			request_schema_version TEXT NOT NULL,
			response_schema_version TEXT,
			request_id TEXT,
			thread_id TEXT,
			turn_id TEXT,
			item_id TEXT,
			approval_id TEXT,
			assistant_message_sequence_no INTEGER,
			title TEXT,
			summary TEXT,
			request_payload TEXT NOT NULL,
			response_payload TEXT,
			runtime_metadata TEXT NOT NULL,
			resolved_by TEXT,
			resolved_at DATETIME,
			created_at DATETIME,
			updated_at DATETIME
		)`,
	} {
		if err := db.Exec(stmt).Error; err != nil {
			t.Fatalf("create table: %v", err)
		}
	}

	now := time.Now().UTC()
	interactionRepo := repository.NewAgentRunInteractionRepository(db)
	artifactRepo := repository.NewAgentRunArtifactRepository(db)
	activities := &AgentRunActivities{
		artifactRepo:    artifactRepo,
		interactionRepo: interactionRepo,
	}
	state := &resolvedRunState{
		run: &model.AgentRun{
			ID:          "run-explicit-preview",
			WorkspaceID: "ws-1",
			RuntimeKind: "native_sdk",
		},
		agent: &model.Agent{
			ID:          "agent-1",
			WorkspaceID: "ws-1",
			RuntimeKind: "native_sdk",
		},
		skillPolicy: workerpkg.SkillPolicy{
			CompletionRequiresInteractionKinds: []string{
				model.AgentRunInteractionKindReviewCheckpoint,
			},
		},
	}

	assistantSequenceNo := 7
	previewPayload, err := json.Marshal(workerpkg.PublishedPreview{
		PanelKey: "task_plan",
		Title:    "Task plan",
		Format:   workerpkg.PreviewFormatJSON,
		Content:  json.RawMessage(`{"summary":"ok","proposed_tasks":[]}`),
	})
	if err != nil {
		t.Fatalf("marshal preview: %v", err)
	}
	if err := artifactRepo.Create(context.Background(), &model.AgentRunArtifact{
		ID:            "artifact-preview",
		WorkspaceID:   state.run.WorkspaceID,
		RunID:         state.run.ID,
		ArtifactType:  workerpkg.RunPreviewArtifactType,
		Format:        "json",
		StorageMode:   "inline",
		InlineContent: strPtr(string(previewPayload)),
		Metadata:      buildAssistantSequenceArtifactMetadata(assistantSequenceNo),
		SequenceNo:    1,
		CreatedAt:     now,
	}); err != nil {
		t.Fatalf("create preview artifact: %v", err)
	}

	if err := interactionRepo.Create(context.Background(), &model.AgentRunInteraction{
		ID:                         "interaction-explicit",
		WorkspaceID:                state.run.WorkspaceID,
		RunID:                      state.run.ID,
		RuntimeKind:                "native_sdk",
		InteractionKind:            model.AgentRunInteractionKindReviewCheckpoint,
		Status:                     model.AgentRunInteractionStatusPending,
		RequestSchemaVersion:       model.AgentRunInteractionSchemaVersionHelpinV1,
		AssistantMessageSequenceNo: &assistantSequenceNo,
		RequestPayload:             json.RawMessage(`{"phase":"review","preview_panel_key":"prd_draft","title":"Approve draft","summary":"Review it"}`),
		RuntimeMetadata:            json.RawMessage(`{"runtime_kind":"native_sdk"}`),
		CreatedAt:                  now,
		UpdatedAt:                  now,
	}); err != nil {
		t.Fatalf("create interaction: %v", err)
	}

	err = activities.enforceCompletionInteractionPolicy(context.Background(), state, &model.AgentRunMessage{
		SequenceNo:  assistantSequenceNo,
		Role:        "assistant",
		MessageType: "assistant_turn",
		Content:     "Please review the draft.",
	})
	if err == nil {
		t.Fatal("expected explicit preview validation error")
	}
	if !strings.Contains(err.Error(), "review_checkpoint requires a same-turn prd_draft preview") {
		t.Fatalf("expected explicit preview validation error, got %v", err)
	}
}

func TestEnforceCompletionInteractionPolicyRequiresCurrentTurnInteraction(t *testing.T) {
	dbName := fmt.Sprintf("file:completion-policy-current-turn-%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dbName), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	if err := db.Exec(`CREATE TABLE agent_run_interactions (
		id TEXT PRIMARY KEY,
		workspace_id TEXT NOT NULL,
		run_id TEXT NOT NULL,
		runtime_kind TEXT NOT NULL,
		interaction_kind TEXT NOT NULL,
		status TEXT NOT NULL,
		request_schema_version TEXT NOT NULL,
		response_schema_version TEXT,
		request_id TEXT,
		thread_id TEXT,
		turn_id TEXT,
		item_id TEXT,
		approval_id TEXT,
		assistant_message_sequence_no INTEGER,
		title TEXT,
		summary TEXT,
		request_payload TEXT NOT NULL,
		response_payload TEXT,
		runtime_metadata TEXT NOT NULL,
		resolved_by TEXT,
		resolved_at DATETIME,
		created_at DATETIME,
		updated_at DATETIME
	)`).Error; err != nil {
		t.Fatalf("create interactions table: %v", err)
	}

	interactionRepo := repository.NewAgentRunInteractionRepository(db)
	activities := &AgentRunActivities{interactionRepo: interactionRepo}
	now := time.Now().UTC()
	run := &model.AgentRun{ID: "run-1", WorkspaceID: "ws-1"}
	state := &resolvedRunState{
		run: run,
		skillPolicy: workerpkg.SkillPolicy{
			CompletionRequiresInteractionKinds: []string{model.AgentRunInteractionKindReviewCheckpoint},
		},
	}

	previousAssistantSequenceNo := 1
	if err := interactionRepo.Create(context.Background(), &model.AgentRunInteraction{
		ID:                         "interaction-previous",
		WorkspaceID:                run.WorkspaceID,
		RunID:                      run.ID,
		RuntimeKind:                "codex",
		InteractionKind:            model.AgentRunInteractionKindReviewCheckpoint,
		Status:                     model.AgentRunInteractionStatusPending,
		RequestSchemaVersion:       model.AgentRunInteractionSchemaVersionHelpinV1,
		RequestPayload:             json.RawMessage(`{"title":"Previous review checkpoint"}`),
		RuntimeMetadata:            json.RawMessage(`{}`),
		AssistantMessageSequenceNo: &previousAssistantSequenceNo,
		CreatedAt:                  now,
		UpdatedAt:                  now,
	}); err != nil {
		t.Fatalf("create previous interaction: %v", err)
	}

	currentAssistantMessage := &model.AgentRunMessage{SequenceNo: 2}
	if err := activities.enforceCompletionInteractionPolicy(context.Background(), state, currentAssistantMessage); err == nil {
		t.Fatal("expected completion policy to reject missing current-turn interaction")
	}

	currentAssistantSequenceNo := currentAssistantMessage.SequenceNo
	if err := interactionRepo.Create(context.Background(), &model.AgentRunInteraction{
		ID:                         "interaction-current",
		WorkspaceID:                run.WorkspaceID,
		RunID:                      run.ID,
		RuntimeKind:                "codex",
		InteractionKind:            model.AgentRunInteractionKindReviewCheckpoint,
		Status:                     model.AgentRunInteractionStatusPending,
		RequestSchemaVersion:       model.AgentRunInteractionSchemaVersionHelpinV1,
		RequestPayload:             json.RawMessage(`{"title":"Current review checkpoint"}`),
		RuntimeMetadata:            json.RawMessage(`{}`),
		AssistantMessageSequenceNo: &currentAssistantSequenceNo,
		CreatedAt:                  now.Add(time.Second),
		UpdatedAt:                  now.Add(time.Second),
	}); err != nil {
		t.Fatalf("create current interaction: %v", err)
	}

	if err := activities.enforceCompletionInteractionPolicy(context.Background(), state, currentAssistantMessage); err != nil {
		t.Fatalf("expected current-turn interaction to satisfy completion policy, got %v", err)
	}
}

func TestExecuteRunActivityFailsAfterPolicyRetryStillMissesInteraction(t *testing.T) {
	db := newPlannerApprovalTestDB(t)

	for _, stmt := range []string{
		`CREATE TABLE agent_run_messages (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			run_id TEXT NOT NULL,
			role TEXT NOT NULL,
			content TEXT NOT NULL,
			message_type TEXT NOT NULL,
			content_blocks TEXT,
			turn_segments TEXT,
			tool_invocations TEXT,
			token_usage TEXT,
			sequence_no INTEGER NOT NULL,
			created_at DATETIME
		)`,
		`CREATE TABLE agent_run_interactions (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			run_id TEXT NOT NULL,
			runtime_kind TEXT NOT NULL,
			interaction_kind TEXT NOT NULL,
			status TEXT NOT NULL,
			request_schema_version TEXT NOT NULL,
			response_schema_version TEXT,
			request_id TEXT,
			thread_id TEXT,
			turn_id TEXT,
			item_id TEXT,
			approval_id TEXT,
			assistant_message_sequence_no INTEGER,
			title TEXT,
			summary TEXT,
			request_payload TEXT NOT NULL,
			response_payload TEXT,
			runtime_metadata TEXT NOT NULL,
			resolved_by TEXT,
			resolved_at DATETIME,
			created_at DATETIME,
			updated_at DATETIME
		)`,
	} {
		if err := db.Exec(stmt).Error; err != nil {
			t.Fatalf("create test table: %v", err)
		}
	}

	runRepo := repository.NewAgentRunRepository(db)
	runMessageRepo := repository.NewAgentRunMessageRepository(db)
	agentRepo := repository.NewAgentRepository(db)
	artifactRepo := repository.NewAgentRunArtifactRepository(db)
	interactionRepo := repository.NewAgentRunInteractionRepository(db)
	epicRepo := repository.NewPMEpicRepository(db)

	now := time.Now().UTC()
	agent := &model.Agent{
		ID:                    "agent-review-2",
		WorkspaceID:           "ws-1",
		IsSystem:              true,
		Name:                  "Lens",
		PresetKey:             model.AgentPresetReviewAgent,
		Role:                  "Reviewer",
		Status:                "idle",
		RuntimeKind:           "native_sdk",
		Skills:                model.AgentSkillRefs{},
		TriggerMode:           "manual",
		AllowedTools:          json.RawMessage(`[]`),
		AllowedCommands:       json.RawMessage(`[]`),
		AllowedTargets:        json.RawMessage(`[]`),
		ApprovalMode:          "never",
		MaxConcurrentRuns:     1,
		DefaultInvocationMode: model.InvocationModeInteractive,
		CreatedAt:             now,
		UpdatedAt:             now,
	}
	if err := db.Create(agent).Error; err != nil {
		t.Fatalf("create agent: %v", err)
	}

	epic := &model.PMEpic{
		ID:                 "epic-review-2",
		WorkspaceID:        "ws-1",
		Name:               "Review epic",
		PlanningState:      model.EpicPlanningStateNotStarted,
		SpecClarifications: json.RawMessage(`[]`),
		CreatedAt:          now,
		UpdatedAt:          now,
	}
	if err := db.Create(epic).Error; err != nil {
		t.Fatalf("create epic: %v", err)
	}

	run := &model.AgentRun{
		ID:             "run-review-no-interaction-2",
		WorkspaceID:    "ws-1",
		AgentID:        agent.ID,
		TargetType:     "epic",
		TargetID:       epic.ID,
		RuntimeKind:    "native_sdk",
		InvocationMode: model.InvocationModeInteractive,
		Status:         model.AgentRunStatusRunning,
		PauseReason:    model.AgentRunPauseReasonNone,
		ApprovalState:  "not_required",
		Input:          json.RawMessage(`{}`),
		OutputSummary:  json.RawMessage(`{}`),
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	if err := db.Create(run).Error; err != nil {
		t.Fatalf("create run: %v", err)
	}

	activities := &AgentRunActivities{
		runRepo:         runRepo,
		runMessageRepo:  runMessageRepo,
		agentRepo:       agentRepo,
		artifactRepo:    artifactRepo,
		interactionRepo: interactionRepo,
		epicRepo:        epicRepo,
		runtimes: workerpkg.NewRuntimeRegistry(stubRuntimeAdapter{
			kind: "native_sdk",
			executeFn: func(execCtx *workerpkg.ExecutionContext, run *model.AgentRun) error {
				execCtx.LastExecutionResult = &workerpkg.ExecutionResult{
					AssistantText: "Findings\n\nHigh: The required alert rules are missing.\nOverall correctness: incorrect.",
				}
				return nil
			},
		}),
	}

	first, err := activities.ExecuteRunActivity(context.Background(), run.ID)
	if err != nil || !first.ContinueExecution {
		t.Fatalf("expected first execution to trigger auto-retry, got result=%#v err=%v", first, err)
	}

	if _, err := activities.ExecuteRunActivity(context.Background(), run.ID); err == nil {
		t.Fatal("expected second invalid completion to fail after retry")
	} else if !strings.Contains(err.Error(), "require one of") {
		t.Fatalf("expected completion policy error, got %v", err)
	}

	updatedRun, err := runRepo.GetByIDAny(context.Background(), run.ID)
	if err != nil {
		t.Fatalf("get updated run: %v", err)
	}
	if updatedRun == nil || updatedRun.Status != model.AgentRunStatusFailed {
		t.Fatalf("expected failed run after retry exhaustion, got %#v", updatedRun)
	}
}

func TestApplyApprovedInteractivePreviewPersistsStoryDocAndLinksIt(t *testing.T) {
	db := newPlannerApprovalTestDB(t)

	artifactRepo := repository.NewAgentRunArtifactRepository(db)
	runRepo := repository.NewAgentRunRepository(db)
	agentRepo := repository.NewAgentRepository(db)
	taskRepo := repository.NewPMTaskRepository(db)
	docsSpaceRepo := repository.NewDocsSpaceRepository(db)
	docsDocRepo := repository.NewDocsDocumentRepository(db)
	docsContentRepo := repository.NewDocsContentRepository(db)
	docsVersionRepo := repository.NewDocsVersionRepository(db)
	docsLinkRepo := repository.NewDocsLinkRepository(db)

	agent := &model.Agent{
		ID:                    "agent-story",
		WorkspaceID:           "ws-1",
		Name:                  "Story Planner",
		Status:                "running",
		RuntimeKind:           "native_sdk",
		Skills:                model.AgentSkillRefs{},
		TriggerMode:           "manual",
		AllowedTools:          json.RawMessage(`[]`),
		AllowedCommands:       json.RawMessage(`[]`),
		AllowedTargets:        json.RawMessage(`[]`),
		ApprovalMode:          "preset_default",
		MaxConcurrentRuns:     1,
		DefaultInvocationMode: model.InvocationModeInteractive,
	}
	if err := db.Create(agent).Error; err != nil {
		t.Fatalf("create agent: %v", err)
	}

	docID := "doc-story-plan-1"
	if err := db.Exec(`INSERT INTO docs_documents (id, workspace_id, space_id, title, status, visibility, created_by, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`,
		docID, "ws-1", "space-1", "Track 4xx errors Plan", model.DocStatusDraft, model.SpaceVisibilityWorkspaceWide, agent.ID,
	).Error; err != nil {
		t.Fatalf("create docs document: %v", err)
	}

	story := &model.PMTask{
		ID:              "story-1",
		WorkspaceID:     "ws-1",
		Name:            "Track 4xx errors",
		DisplayID:       1,
		TaskType:        model.PMTaskTypeFeature,
		WorkflowID:      "wf-1",
		WorkflowStateID: "state-1",
		Priority:        model.PMTaskPriorityNone,
		Severity:        model.PMTaskSeverityNone,
		PlanDocumentID:  &docID,
	}
	if err := db.Create(story).Error; err != nil {
		t.Fatalf("create story: %v", err)
	}

	run := &model.AgentRun{
		ID:             "run-story-doc",
		WorkspaceID:    "ws-1",
		AgentID:        agent.ID,
		TaskID:         &story.ID,
		TargetType:     "story",
		TargetID:       story.ID,
		InvocationMode: model.InvocationModeInteractive,
		Status:         model.AgentRunStatusRunning,
		Input:          json.RawMessage(`{}`),
		OutputSummary:  json.RawMessage(`{}`),
	}
	if err := db.Create(run).Error; err != nil {
		t.Fatalf("create run: %v", err)
	}

	markdownJSON, err := json.Marshal("# Outcome\n\nAdd the helper and wire it into capture.")
	if err != nil {
		t.Fatalf("marshal markdown: %v", err)
	}
	preview := model.ApprovedRunPreview{
		Phase:           "story_doc",
		PanelKey:        "task_plan_doc",
		Format:          workerpkg.PreviewFormatMarkdown,
		Content:         markdownJSON,
		ApprovalSummary: "Approved story plan",
	}
	previewJSON, err := json.Marshal(preview)
	if err != nil {
		t.Fatalf("marshal approved preview: %v", err)
	}
	if err := db.Create(&model.AgentRunArtifact{
		ID:            "approved-story-doc-1",
		WorkspaceID:   run.WorkspaceID,
		RunID:         run.ID,
		ArtifactType:  model.AgentRunArtifactTypeApprovedPreview,
		Format:        "json",
		StorageMode:   "inline",
		InlineContent: strPtr(string(previewJSON)),
		Metadata:      json.RawMessage(`{}`),
		SequenceNo:    1,
	}).Error; err != nil {
		t.Fatalf("create approved preview artifact: %v", err)
	}

	var executed []string
	commandExecutor := stubInternalCommandExecutor{
		executeFn: func(ctx context.Context, meta model.InternalCommandContext, name string, input json.RawMessage) (json.RawMessage, error) {
			executed = append(executed, name)
			return json.RawMessage(`{}`), nil
		},
	}

	activity := &AgentRunActivities{
		runRepo:         runRepo,
		artifactRepo:    artifactRepo,
		taskRepo:        taskRepo,
		agentRepo:       agentRepo,
		docsSpaceRepo:   docsSpaceRepo,
		docsDocRepo:     docsDocRepo,
		docsContentRepo: docsContentRepo,
		docsVersionRepo: docsVersionRepo,
		docsLinkRepo:    docsLinkRepo,
		commandExecutor: commandExecutor,
	}
	state := &resolvedRunState{
		run:  run,
		task: story,
	}
	input := planningRunInput{Stage: model.PlanningStageStoryPlanDoc}

	action, err := activity.applyApprovedInteractivePreview(context.Background(), state, &input)
	if err != nil {
		t.Fatalf("applyApprovedInteractivePreview returned error: %v", err)
	}
	if action != "persist_task_doc" {
		t.Fatalf("expected persist_task_doc action, got %q", action)
	}
	if len(executed) != 1 || executed[0] != "docs.write_document_content" {
		t.Fatalf("expected only docs.write_document_content to execute, got %#v", executed)
	}

	updatedTask, err := taskRepo.GetRawByID(context.Background(), story.ID)
	if err != nil {
		t.Fatalf("get updated task: %v", err)
	}
	if updatedTask == nil || updatedTask.PlanDocumentID == nil || *updatedTask.PlanDocumentID == "" {
		t.Fatalf("expected task plan document id to be set, got %#v", updatedTask)
	}
	if input.PlanDocumentID == "" || input.PlanDocumentID != *updatedTask.PlanDocumentID {
		t.Fatalf("expected planning input plan_document_id to be set, got %#v", input)
	}

	links, err := docsLinkRepo.ListByObject(context.Background(), run.WorkspaceID, model.LinkedObjectTask, story.ID)
	if err != nil {
		t.Fatalf("list docs links: %v", err)
	}
	if len(links) != 1 {
		t.Fatalf("expected 1 story plan doc link, got %d", len(links))
	}

	updatedRun, err := runRepo.GetByIDAny(context.Background(), run.ID)
	if err != nil {
		t.Fatalf("get updated run: %v", err)
	}
	if updatedRun == nil || updatedRun.Status != model.AgentRunStatusCompleted {
		t.Fatalf("expected run to complete, got %#v", updatedRun)
	}

	updatedAgent, err := agentRepo.GetByID(context.Background(), agent.WorkspaceID, agent.ID)
	if err != nil {
		t.Fatalf("get updated agent: %v", err)
	}
	if updatedAgent == nil || updatedAgent.Status != "idle" {
		t.Fatalf("expected agent to be idle, got %#v", updatedAgent)
	}

	var appliedMarkers []model.AgentRunArtifact
	if err := db.Where("run_id = ? AND artifact_type = ?", run.ID, model.AgentRunArtifactTypeApprovedPreviewApplied).Find(&appliedMarkers).Error; err != nil {
		t.Fatalf("list applied markers: %v", err)
	}
	if len(appliedMarkers) != 1 {
		t.Fatalf("expected 1 approved preview applied marker, got %d", len(appliedMarkers))
	}
}

func TestBuildInitialInstructionsIncludesTaskPlanningDocForTaskExecutionRun(t *testing.T) {
	db := newPlannerApprovalTestDB(t)

	if err := db.Exec(`INSERT INTO docs_spaces (id, workspace_id, name, slug, visibility, type, created_by, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`,
		"space-1", "ws-1", "Product", "product", model.SpaceVisibilityWorkspaceWide, model.SpaceTypeInternal, "user-1",
	).Error; err != nil {
		t.Fatalf("create docs space: %v", err)
	}

	docID := "doc-task-prd"
	if err := db.Exec(`INSERT INTO docs_documents (id, workspace_id, space_id, title, status, visibility, created_by, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`,
		docID, "ws-1", "space-1", "Track 4xx errors", model.DocStatusDraft, model.SpaceVisibilityWorkspaceWide, "user-1",
	).Error; err != nil {
		t.Fatalf("create task planning doc: %v", err)
	}
	if err := db.Exec(`INSERT INTO docs_contents (id, document_id, content, content_text, created_at, updated_at)
		VALUES (?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`,
		"content-task-prd", docID, []byte(`{"_markdown_source":"# Task PRD\n\nImplement the linked task PRD first."}`), "Implement the linked task PRD first.",
	).Error; err != nil {
		t.Fatalf("create task planning doc content: %v", err)
	}

	extraDocID := "doc-task-extra"
	if err := db.Exec(`INSERT INTO docs_documents (id, workspace_id, space_id, title, status, visibility, created_by, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`,
		extraDocID, "ws-1", "space-1", "Error Payload Notes", model.DocStatusDraft, model.SpaceVisibilityWorkspaceWide, "user-1",
	).Error; err != nil {
		t.Fatalf("create extra linked doc: %v", err)
	}
	if err := db.Exec(`INSERT INTO docs_contents (id, document_id, content, content_text, created_at, updated_at)
		VALUES (?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`,
		"content-task-extra", extraDocID, []byte(`{"_markdown_source":"## Payload Fields\n\nCapture the upstream error payload fields."}`), "Capture the upstream error payload fields.",
	).Error; err != nil {
		t.Fatalf("create extra linked doc content: %v", err)
	}

	if err := db.Exec(`INSERT INTO docs_links (id, workspace_id, document_id, linked_object_type, linked_object_id, link_context, created_by, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)`,
		"link-task-extra", "ws-1", extraDocID, model.LinkedObjectTask, "task-1", "reference", "user-1",
	).Error; err != nil {
		t.Fatalf("create docs link: %v", err)
	}

	task := &model.PMTask{
		ID:              "task-1",
		WorkspaceID:     "ws-1",
		DisplayID:       1,
		Name:            "Track 4xx errors",
		TaskType:        model.PMTaskTypeFeature,
		WorkflowID:      "wf-1",
		WorkflowStateID: "state-1",
		Priority:        model.PMTaskPriorityNone,
		Severity:        model.PMTaskSeverityNone,
		PlanDocumentID:  &docID,
	}
	if err := db.Create(task).Error; err != nil {
		t.Fatalf("create task: %v", err)
	}

	run := &model.AgentRun{
		ID:          "run-task-exec",
		WorkspaceID: "ws-1",
		AgentID:     "agent-forge",
		TargetType:  "task",
		TargetID:    task.ID,
		Input:       json.RawMessage(`{"additional_context":"Focus on the capture pipeline."}`),
	}

	activity := &AgentRunActivities{
		docsDocRepo:     repository.NewDocsDocumentRepository(db),
		docsContentRepo: repository.NewDocsContentRepository(db),
		docsLinkRepo:    repository.NewDocsLinkRepository(db),
	}
	state := &resolvedRunState{
		run:  run,
		task: task,
	}

	instructions, err := activity.buildInitialInstructions(context.Background(), state, planningRunInput{
		AllowedTools: []string{"read_file", "run_command"},
	})
	if err != nil {
		t.Fatalf("buildInitialInstructions returned error: %v", err)
	}
	for _, snippet := range []string{
		"Operator notes:\nFocus on the capture pipeline.",
		"Canonical task planning document: Track 4xx errors [doc-task-prd]",
		"# Task PRD",
		"Implement the linked task PRD first.",
		"Other docs linked directly to this task:",
		"Error Payload Notes [doc-task-extra]",
		"## Payload Fields",
		"Capture the upstream error payload fields.",
	} {
		if !strings.Contains(instructions, snippet) {
			t.Fatalf("expected instructions to contain %q\n%s", snippet, instructions)
		}
	}
}

func TestBuildInitialInstructionsIncludesGenericRepositoryBranchContext(t *testing.T) {
	run := &model.AgentRun{
		ID:            "run-task-review",
		WorkspaceID:   "ws-1",
		TargetType:    "task",
		TargetID:      "task-1",
		BaseBranch:    strPtr("main"),
		WorkingBranch: strPtr("tp-123-review"),
		Input:         json.RawMessage(`{"additional_context":"Focus on regressions in the sprint picker."}`),
	}
	task := &model.PMTask{
		ID:          "task-1",
		WorkspaceID: "ws-1",
		Name:        "Restrict Sprint Visibility",
	}

	activity := &AgentRunActivities{}
	state := &resolvedRunState{
		run:  run,
		task: task,
	}

	instructions, err := activity.buildInitialInstructions(context.Background(), state, planningRunInput{
		AllowedTools: []string{"read_file", "run_command"},
	})
	if err != nil {
		t.Fatalf("buildInitialInstructions returned error: %v", err)
	}
	for _, snippet := range []string{
		"Repository branches: base `main`, working `tp-123-review`.",
		"Operator notes:\nFocus on regressions in the sprint picker.",
	} {
		if !strings.Contains(instructions, snippet) {
			t.Fatalf("expected generic task instructions to contain %q\n%s", snippet, instructions)
		}
	}
}

func TestResolvePlanningRunInputClearsDeletedEpicSpecReferences(t *testing.T) {
	dbName := fmt.Sprintf("file:resolve-planning-input-%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dbName), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	for _, stmt := range []string{
		`CREATE TABLE pm_epics (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			name TEXT NOT NULL,
			description TEXT,
			external_id TEXT,
			epic_state_id TEXT,
			owner_id TEXT,
			owner_member_id TEXT,
			team_id TEXT,
			planned_start_date DATETIME,
			deadline DATETIME,
			started BOOLEAN NOT NULL DEFAULT 0,
			started_at DATETIME,
			completed BOOLEAN NOT NULL DEFAULT 0,
			completed_at DATETIME,
			position INTEGER NOT NULL DEFAULT 0,
			color TEXT,
			health TEXT NOT NULL DEFAULT 'no_health',
			health_comment TEXT,
			archived BOOLEAN NOT NULL DEFAULT 0,
			spec_document_id TEXT,
			planning_repository_id TEXT,
			planning_state TEXT NOT NULL DEFAULT 'not_started',
			spec_clarifications TEXT NOT NULL DEFAULT '[]',
			spec_clarified_at DATETIME,
			spec_clarified_by TEXT,
			approved_spec_version_id TEXT,
			last_planning_run_id TEXT,
			created_by TEXT,
			created_at DATETIME,
			updated_at DATETIME
		)`,
		`CREATE TABLE pm_epic_labels (
			epic_id TEXT NOT NULL,
			label_id TEXT NOT NULL,
			created_at DATETIME
		)`,
		`CREATE TABLE pm_labels (
			id TEXT PRIMARY KEY,
			workspace_id TEXT,
			name TEXT NOT NULL,
			color TEXT,
			created_at DATETIME,
			updated_at DATETIME
		)`,
		`CREATE TABLE pm_epic_objectives (
			epic_id TEXT NOT NULL,
			objective_id TEXT NOT NULL,
			created_at DATETIME
		)`,
		`CREATE TABLE pm_objectives (
			id TEXT PRIMARY KEY,
			workspace_id TEXT,
			name TEXT NOT NULL
		)`,
		`CREATE TABLE pm_workflow_states (
			id TEXT PRIMARY KEY,
			state_type TEXT NOT NULL
		)`,
		`CREATE TABLE pm_tasks (
			id TEXT PRIMARY KEY,
			epic_id TEXT,
			workflow_state_id TEXT,
			estimate INTEGER,
			plan_document_id TEXT,
			archived BOOLEAN NOT NULL DEFAULT 0
		)`,
		`CREATE TABLE docs_documents (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			space_id TEXT NOT NULL,
			collection_id TEXT,
			title TEXT NOT NULL,
			status TEXT NOT NULL,
			visibility TEXT NOT NULL,
			owner_id TEXT,
			team_id TEXT,
			template_key TEXT,
			excerpt TEXT,
			icon TEXT,
			tags TEXT,
			is_pinned BOOLEAN NOT NULL DEFAULT 0,
			is_publicly_shared BOOLEAN NOT NULL DEFAULT 0,
			share_token TEXT,
			is_locked BOOLEAN NOT NULL DEFAULT 0,
			locked_by TEXT,
			last_reviewed_at DATETIME,
			next_review_at DATETIME,
			published_at DATETIME,
			created_by TEXT NOT NULL,
			created_at DATETIME,
			updated_at DATETIME,
			deleted_at DATETIME
		)`,
		`CREATE TABLE docs_versions (
			id TEXT PRIMARY KEY,
			document_id TEXT NOT NULL,
			content TEXT,
			content_text TEXT,
			snapshot_label TEXT,
			version_type TEXT NOT NULL,
			word_count INTEGER NOT NULL DEFAULT 0,
			created_by TEXT NOT NULL,
			created_at DATETIME
		)`,
	} {
		if err := db.Exec(stmt).Error; err != nil {
			t.Fatalf("create test table: %v", err)
		}
	}

	epicRepo := repository.NewPMEpicRepository(db)
	docsDocRepo := repository.NewDocsDocumentRepository(db)
	docsVersionRepo := repository.NewDocsVersionRepository(db)

	epic := &model.PMEpic{
		ID:                    "epic-1",
		WorkspaceID:           "ws-1",
		Name:                  "Epic",
		SpecDocumentID:        strPtr("doc-missing"),
		ApprovedSpecVersionID: strPtr("ver-missing"),
		SpecClarifications:    json.RawMessage(`[]`),
	}
	if err := db.Create(epic).Error; err != nil {
		t.Fatalf("create epic: %v", err)
	}

	activity := &AgentRunActivities{
		epicRepo:        epicRepo,
		docsDocRepo:     docsDocRepo,
		docsVersionRepo: docsVersionRepo,
	}
	state := &resolvedRunState{
		run: &model.AgentRun{
			ID:          "run-1",
			WorkspaceID: "ws-1",
			TargetType:  "epic",
			TargetID:    epic.ID,
			Input:       json.RawMessage(`{"epic_id":"epic-1","spec_document_id":"doc-stale","spec_version_id":"ver-stale"}`),
		},
		epic: epic,
	}

	input, err := activity.resolvePlanningRunInput(context.Background(), state)
	if err != nil {
		t.Fatalf("resolvePlanningRunInput returned error: %v", err)
	}
	if input.SpecDocumentID != "" || input.SpecVersionID != "" {
		t.Fatalf("expected stale spec references to be cleared, got %#v", input)
	}
	if state.epic.SpecDocumentID != nil || state.epic.ApprovedSpecVersionID != nil {
		t.Fatalf("expected in-memory epic spec state to be cleared, got %#v", state.epic)
	}

	updatedEpic, err := epicRepo.GetByID(context.Background(), epic.ID)
	if err != nil {
		t.Fatalf("get updated epic: %v", err)
	}
	if updatedEpic == nil {
		t.Fatal("expected epic after update")
	}
	if updatedEpic.Epic.SpecDocumentID != nil || updatedEpic.Epic.ApprovedSpecVersionID != nil {
		t.Fatalf("expected persisted epic spec state to be cleared, got %#v", updatedEpic.Epic)
	}
}
