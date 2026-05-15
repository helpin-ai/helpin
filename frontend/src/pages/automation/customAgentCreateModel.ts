import type {
  AgentApprovalMode,
  AgentExecutionConfig,
  AgentInvocationMode,
  AgentModelProvider,
  AgentPresetKey,
  AgentReasoningEffort,
  AgentRuntimeKind,
  AgentServiceTier,
  AgentSkillRef,
  AgentTargetType,
  CreateAgentRequest,
  CustomAgentDraft,
} from '@/lib/pmTypes';

export interface CustomAgentFormData {
  name: string;
  preset_key: AgentPresetKey;
  preset_version_key: string;
  runtime_kind: AgentRuntimeKind;
  supported_modes: AgentInvocationMode[];
  provider: AgentModelProvider;
  model: string;
  reasoning_effort: AgentReasoningEffort | '';
  service_tier: AgentServiceTier | '';
  system_prompt: string;
  instruction_preamble: string;
  instruction_skills: string[];
  monthly_token_budget: string;
  team_id: string;
  teamAccessMode: 'all_teams' | 'specific_teams';
  team_ids: string[];
  allowed_targets: AgentTargetType[];
  allowed_tools: string[];
  skills: AgentSkillRef[];
  approval_mode: AgentApprovalMode;
  max_concurrent_runs: string;
  default_invocation_mode: AgentInvocationMode;
}

export function createDefaultCustomAgentForm(): CustomAgentFormData {
  return {
    name: '',
    preset_key: 'code_builder',
    preset_version_key: 'code_builder_default',
    runtime_kind: 'native_sdk',
    supported_modes: ['autonomous', 'interactive'],
    provider: 'anthropic',
    model: '',
    reasoning_effort: '',
    service_tier: '',
    system_prompt: '',
    instruction_preamble: '',
    instruction_skills: [],
    monthly_token_budget: '',
    team_id: '',
    teamAccessMode: 'all_teams',
    team_ids: [],
    allowed_targets: ['task'],
    allowed_tools: [],
    skills: [],
    approval_mode: 'always',
    max_concurrent_runs: '1',
    default_invocation_mode: 'interactive',
  };
}

export function applyCustomAgentDraftToForm(
  form: CustomAgentFormData,
  draft: CustomAgentDraft,
): CustomAgentFormData {
  return {
    ...form,
    name: draft.name,
    runtime_kind: draft.runtime_kind,
    supported_modes: draft.runtime_kind === 'native_sdk' || draft.runtime_kind === 'codex'
      ? ['autonomous', 'interactive']
      : ['autonomous'],
    provider: draft.provider,
    model: draft.model ?? '',
    system_prompt: draft.system_prompt,
    allowed_targets: [...draft.allowed_targets],
    allowed_tools: [...draft.allowed_tools],
    skills: [...draft.skills],
    approval_mode: draft.approval_mode,
    max_concurrent_runs: String(draft.max_concurrent_runs || 1),
    default_invocation_mode: draft.default_invocation_mode,
  };
}

function buildExecutionConfigPayload(form: CustomAgentFormData): AgentExecutionConfig | undefined {
  if (form.runtime_kind !== 'codex') return undefined;
  const config: AgentExecutionConfig = {};
  if (form.reasoning_effort) {
    config.reasoning_effort = form.reasoning_effort;
  }
  if (form.provider === 'openai' && form.service_tier) {
    config.service_tier = form.service_tier;
  }
  return Object.keys(config).length > 0 ? config : undefined;
}

function normalizeMaxConcurrentRuns(value: string) {
  const parsed = Number.parseInt(value, 10);
  return Number.isFinite(parsed) && parsed > 0 ? parsed : 1;
}

function normalizeStringList(values: string[]) {
  return Array.from(new Set(values.map((value) => value.trim()).filter(Boolean)));
}

export function buildCustomAgentCreatePayload(
  workspaceId: string,
  form: CustomAgentFormData,
  advancedOpen: boolean,
): CreateAgentRequest {
  const defaultRuntimeKind: AgentRuntimeKind = 'native_sdk';
  const teamIds = form.teamAccessMode === 'specific_teams' ? normalizeStringList(form.team_ids) : [];
  return {
    workspace_id: workspaceId,
    name: form.name.trim(),
    provider: form.provider,
    model: form.model.trim() || undefined,
    execution_config: buildExecutionConfigPayload(form),
    system_prompt: form.system_prompt.trim() || undefined,
    trigger_mode: 'manual',
    team_ids: teamIds,
    allowed_tools: [...form.allowed_tools],
    skills: form.skills.length > 0 ? form.skills : undefined,
    allowed_targets: form.allowed_targets.length > 0 ? [...form.allowed_targets] : ['task'],
    approval_mode: form.approval_mode,
    max_concurrent_runs: normalizeMaxConcurrentRuns(form.max_concurrent_runs),
    default_invocation_mode: form.default_invocation_mode,
    ...(advancedOpen
      ? {
          runtime_kind: form.runtime_kind,
        }
      : form.runtime_kind !== defaultRuntimeKind
        ? { runtime_kind: form.runtime_kind }
        : {}),
  };
}

export function validateCustomAgentCreateForm(form: CustomAgentFormData): string[] {
  const missing: string[] = [];
  if (!form.name.trim()) {
    missing.push('agent name');
  }
  if (!form.system_prompt.trim()) {
    missing.push('instructions');
  }
  if (form.allowed_targets.length === 0) {
    missing.push('working area');
  }
  if (form.allowed_tools.length === 0 && form.skills.length === 0) {
    missing.push('tool or skill');
  }
  if (form.teamAccessMode === 'specific_teams' && normalizeStringList(form.team_ids).length === 0) {
    missing.push('team access');
  }
  return missing;
}

const TARGET_LABELS: Record<AgentTargetType, string> = {
  task: 'tasks',
  epic: 'epics',
  repository: 'repositories',
  workspace: 'the workspace',
  crm_deal: 'CRM deals',
  document: 'documents',
  support_conversation: 'support conversations',
  support_coverage_gap: 'support coverage gaps',
};

function joinLabels(labels: string[]) {
  if (labels.length === 0) return 'tasks';
  if (labels.length === 1) return labels[0];
  if (labels.length === 2) return `${labels[0]} and ${labels[1]}`;
  return `${labels.slice(0, -1).join(', ')}, and ${labels[labels.length - 1]}`;
}

export function summarizeCustomAgentCreate(form: CustomAgentFormData, teamName?: string) {
  const name = form.name.trim() || 'this agent';
  const scope = teamName?.trim() || 'workspace-wide';
  const targets = joinLabels(form.allowed_targets.map((target) => TARGET_LABELS[target] ?? target));
  const mode = form.default_invocation_mode === 'interactive' ? 'interactive' : 'autonomous';
  const instructions = form.system_prompt.trim() ? 'uses your saved instructions' : 'starts with blank instructions';
  return `Create ${name} for ${scope}. It can work on ${targets}, starts as an ${mode} agent, and ${instructions}.`;
}
