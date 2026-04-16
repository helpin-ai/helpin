import { useCallback, useEffect, useMemo, useState } from 'react';
import { Collapsible } from 'radix-ui';
import { formatDistanceToNow } from 'date-fns';
import { toast } from 'sonner';
import { cn } from '@/lib/utils';
import {
  BotIcon,
  ArrowDown01Icon,
  ArrowRight01Icon,
  Clock01Icon,
  HelpCircleIcon,
  LayoutGridIcon,
  LayoutTable01Icon,
  PencilEdit01Icon,
  PlusSignIcon,
  UserGroupIcon,
  Cancel01Icon,
  ZapIcon,
  WorkflowSquare01Icon,
  Loading01Icon,
  BookOpen01Icon,
  SourceCodeIcon,
} from '@/lib/icons';
import { AgentAvatar } from '@/components/agents/AgentAvatar';
import { ConfirmDialog } from '@/components/pm/ConfirmDialog';
import { useAccessibleTeams } from '@/hooks/useAccessibleTeams';
import { useTitle } from '@/hooks/useTitle';
import { useWorkspaceStore } from '@/stores/workspaceStore';
import { useWorkspaceAccess, usePermissions } from '@/hooks/queries/useSession';
import { useWorkspaceSettings } from '@/hooks/queries/useSettings';
import { automationService } from '@/lib/services/automationService';
import { agentService } from '@/lib/services/agentService';
import { AGENT_RUNTIME_LABELS } from '@/lib/agentRuntime';
import { buildAutomationActivityPath, buildAutomationFlowsPath } from '@/lib/automationUi';
import type {
  Agent,
  AgentExecutionConfig,
  AgentPresetDefinition,
  AgentPresetKey,
  AgentApprovalMode,
  AgentInvocationMode,
  AgentModelProvider,
  AgentModelProviderOption,
  AgentReasoningEffort,
  AgentRun,
  AgentRuntimeKind,
  AgentServiceTier,
  AgentSkillRef,
  AgentTriggerUsage,
  AgentTriggerUsageSummary,
  AgentTargetType,
  CreateWorkspaceAgentPresetVersionRequest,
  CreateAgentRequest,
  SkillCatalogResponse,
  ToolCatalogResponse,
  UpdateAgentRequest,
} from '@/lib/pmTypes';
import { Button } from '@/components/ui/button';
import { Badge } from '@/components/ui/badge';
import { Card, CardContent, CardHeader } from '@/components/ui/card';
import { Progress } from '@/components/ui/progress';
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetFooter,
  SheetHeader,
  SheetTitle,
} from '@/components/ui/sheet';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Switch } from '@/components/ui/switch';
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from '@/components/ui/tooltip';
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';
import { Textarea } from '@/components/ui/textarea';
import { Separator } from '@/components/ui/separator';
import {
  Command,
  CommandEmpty,
  CommandGroup,
  CommandInput,
  CommandItem,
  CommandList,
} from '@/components/ui/command';
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from '@/components/ui/popover';

// ---------------------------------------------------------------------------
// Constants
// ---------------------------------------------------------------------------

const STATUS_DOT: Record<string, string> = {
  idle: 'bg-green-500',
  working: 'bg-amber-500',
  error: 'bg-red-500',
  paused: 'bg-gray-400',
};

const STATUS_LABEL: Record<string, string> = {
  idle: 'Ready',
  working: 'Running',
  error: 'Error',
  paused: 'Paused',
};

const RUNTIME_KIND_OPTIONS: AgentRuntimeKind[] = ['opencode', 'codex', 'native_sdk'];
const REASONING_EFFORT_OPTIONS: AgentReasoningEffort[] = ['none', 'minimal', 'low', 'medium', 'high', 'xhigh'];
const SERVICE_TIER_OPTIONS: AgentServiceTier[] = ['fast', 'flex'];
const DEFAULT_SYSTEM_PRESET_KEY: AgentPresetKey = 'code_builder';
const PRESET_FALLBACKS: Record<AgentPresetKey, {
  label: string;
  description: string;
  runtime_kind: AgentRuntimeKind;
  default_invocation_mode: AgentInvocationMode;
  supported_modes: AgentInvocationMode[];
  provider?: AgentModelProvider;
  model?: string;
}> = {
  epic_planner: {
    label: 'Epic Planner',
    description: 'Interactive product planning for epics, PRDs, docs, and tasks.',
    runtime_kind: 'native_sdk',
    default_invocation_mode: 'interactive',
    supported_modes: ['autonomous', 'interactive'],
  },
  task_planner: {
    label: 'Task Planner',
    description: 'Interactive decomposition and refinement for tasks and execution plans.',
    runtime_kind: 'native_sdk',
    default_invocation_mode: 'interactive',
    supported_modes: ['autonomous', 'interactive'],
  },
  story_planner: {
    label: 'Task Planner',
    description: 'Interactive decomposition and refinement for tasks and execution plans.',
    runtime_kind: 'native_sdk',
    default_invocation_mode: 'interactive',
    supported_modes: ['autonomous', 'interactive'],
  },
  crm_operator: {
    label: 'CRM Operator',
    description: 'Cross-app CRM execution across deals, contacts, docs, and support context.',
    runtime_kind: 'native_sdk',
    default_invocation_mode: 'interactive',
    supported_modes: ['autonomous', 'interactive'],
  },
  support_agent: {
    label: 'Support Agent',
    description: 'Handles support conversations and drafts replies with review controls.',
    runtime_kind: 'native_sdk',
    default_invocation_mode: 'autonomous',
    supported_modes: ['autonomous', 'interactive'],
  },
  code_builder: {
    label: 'Code Builder',
    description: 'Writes code, implements features, and fixes bugs in the repo.',
    runtime_kind: 'codex',
    default_invocation_mode: 'autonomous',
    supported_modes: ['autonomous', 'interactive'],
    provider: 'openai',
    model: 'gpt-5.4',
  },
  review_agent: {
    label: 'Review Agent',
    description: 'Reviews work, runs tests, and checks quality without repo mutation.',
    runtime_kind: 'codex',
    default_invocation_mode: 'autonomous',
    supported_modes: ['autonomous', 'interactive'],
    provider: 'openai',
    model: 'gpt-5.4',
  },
};

const INVOCATION_MODE_LABELS: Record<AgentInvocationMode, string> = {
  autonomous: 'Autonomous',
  interactive: 'Interactive',
};

const APPROVAL_MODE_OPTIONS: { value: AgentApprovalMode; label: string; description: string }[] = [
  { value: 'preset_default', label: 'Preset Default', description: 'Uses the standard setting for this agent preset' },
  { value: 'never', label: 'No — run immediately', description: 'Agent starts working right away without waiting' },
  { value: 'always', label: 'Yes — always review first', description: 'A team member must approve before the agent runs' },
];

const EMPTY_STATE_CARDS = [
  {
    icon: ZapIcon,
    title: 'Automate work',
    desc: 'Handle planning, coding, doc updates, support replies, and deal management so your team can focus on what matters.',
  },
  {
    icon: BotIcon,
    title: 'Cross-module',
    desc: 'Agents can span projects, CRM, support, and docs — just pick the capabilities they need.',
  },
  {
    icon: UserGroupIcon,
    title: 'Team-aware',
    desc: 'Assign agents to teams so they only work on relevant tasks, or let them operate workspace-wide.',
  },
];

// ---------------------------------------------------------------------------
// Form helpers
// ---------------------------------------------------------------------------

interface AgentFormData {
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
  allowed_targets: AgentTargetType[];
  allowed_tools: string[];
  skills: AgentSkillRef[];
  schedule: string;
  approval_mode: AgentApprovalMode;
  max_concurrent_runs: string;
  default_invocation_mode: AgentInvocationMode;
}

const CUSTOM_AGENT_TARGET_OPTIONS: Array<{ value: AgentTargetType; label: string; description: string }> = [
  { value: 'task', label: 'Task', description: 'Run on tasks and task planning loops.' },
  { value: 'epic', label: 'Epic', description: 'Run on epics and planning loops.' },
  { value: 'repository', label: 'Repository', description: 'Run directly against a synced repository without requiring a linked task.' },
  { value: 'crm_deal', label: 'CRM Deal', description: 'Run on CRM deal records.' },
  { value: 'document', label: 'Document', description: 'Run on documents and docs-backed context.' },
  { value: 'support_conversation', label: 'Support Conversation', description: 'Run on support inbox conversations.' },
];

function ProviderIcon({ provider, className = 'h-4 w-4' }: { provider: string; className?: string }) {
  switch (provider) {
    case 'anthropic':
      return (
        <svg className={className} viewBox="0 0 24 24" fill="currentColor" aria-hidden="true">
          <path d="M17.304 3.541h-3.672l6.696 16.918h3.672l-6.696-16.918zM6.696 3.541L0 20.459h3.672l1.344-3.541h6.86l1.344 3.541h3.672L10.196 3.541H6.696zm-.672 10.459l2.424-6.391 2.424 6.391H6.024z" />
        </svg>
      );
    case 'openai':
      return (
        <svg className={className} viewBox="0 0 24 24" fill="currentColor" aria-hidden="true">
          <path d="M22.282 9.821a5.985 5.985 0 0 0-.516-4.91 6.046 6.046 0 0 0-6.51-2.9A6.065 6.065 0 0 0 4.981 4.18a5.998 5.998 0 0 0-3.992 2.9 6.042 6.042 0 0 0 .743 7.097 5.98 5.98 0 0 0 .51 4.911 6.051 6.051 0 0 0 6.515 2.9A5.985 5.985 0 0 0 13.26 24a6.056 6.056 0 0 0 5.772-4.206 5.99 5.99 0 0 0 3.997-2.9 6.056 6.056 0 0 0-.747-7.073zM13.26 22.43a4.476 4.476 0 0 1-2.876-1.04l.141-.081 4.779-2.758a.795.795 0 0 0 .392-.681v-6.737l2.02 1.168a.071.071 0 0 1 .038.052v5.583a4.504 4.504 0 0 1-4.494 4.494zM3.6 18.304a4.47 4.47 0 0 1-.535-3.014l.142.085 4.783 2.759a.771.771 0 0 0 .78 0l5.843-3.369v2.332a.08.08 0 0 1-.033.062L9.74 19.95a4.5 4.5 0 0 1-6.14-1.646zM2.34 7.896a4.485 4.485 0 0 1 2.366-1.973V11.6a.766.766 0 0 0 .388.676l5.815 3.355-2.02 1.168a.076.076 0 0 1-.071 0l-4.83-2.786A4.504 4.504 0 0 1 2.34 7.872zm16.597 3.855l-5.833-3.387L15.119 7.2a.076.076 0 0 1 .071 0l4.83 2.791a4.494 4.494 0 0 1-.676 8.105v-5.678a.79.79 0 0 0-.407-.667zm2.01-3.023l-.141-.085-4.774-2.782a.776.776 0 0 0-.785 0L9.409 9.23V6.897a.066.066 0 0 1 .028-.061l4.83-2.787a4.5 4.5 0 0 1 6.68 4.66zm-12.64 4.135l-2.02-1.164a.08.08 0 0 1-.038-.057V6.075a4.5 4.5 0 0 1 7.375-3.453l-.142.08L8.704 5.46a.795.795 0 0 0-.393.681zm1.097-2.365l2.602-1.5 2.607 1.5v2.999l-2.597 1.5-2.607-1.5z" />
        </svg>
      );
    case 'openrouter':
      return (
        <svg className={className} viewBox="0 0 24 24" fill="currentColor" aria-hidden="true">
          <path d="M12 2C6.48 2 2 6.48 2 12s4.48 10 10 10 10-4.48 10-10S17.52 2 12 2zm-1 17.93c-3.95-.49-7-3.85-7-7.93 0-.62.08-1.21.21-1.79L9 15v1c0 1.1.9 2 2 2v1.93zm6.9-2.54c-.26-.81-1-1.39-1.9-1.39h-1v-3c0-.55-.45-1-1-1H8v-2h2c.55 0 1-.45 1-1V7h2c1.1 0 2-.9 2-2v-.41c2.93 1.19 5 4.06 5 7.41 0 2.08-.8 3.97-2.1 5.39z" />
        </svg>
      );
    default:
      return <BotIcon className={className} />;
  }
}

const FALLBACK_PROVIDER_OPTIONS: AgentModelProviderOption[] = [
  {
    value: 'anthropic',
    label: 'Anthropic',
    model_placeholder: 'claude-sonnet-4-20250514',
    supports_reasoning_effort: false,
    supports_service_tier: false,
  },
  {
    value: 'openai',
    label: 'OpenAI',
    model_placeholder: 'gpt-5.4',
    supports_reasoning_effort: true,
    supported_reasoning_efforts: REASONING_EFFORT_OPTIONS,
    supports_service_tier: true,
    supported_service_tiers: SERVICE_TIER_OPTIONS,
  },
  {
    value: 'openrouter',
    label: 'OpenRouter',
    model_placeholder: 'openai/gpt-5.4',
    supports_reasoning_effort: true,
    supported_reasoning_efforts: REASONING_EFFORT_OPTIONS,
    supports_service_tier: false,
  },
];

function allowedRuntimeKindsForPreset(presetKey: AgentPresetKey): AgentRuntimeKind[] {
  switch (presetKey) {
    case 'code_builder':
      return ['opencode', 'codex', 'native_sdk'];
    case 'review_agent':
      return ['opencode', 'codex', 'native_sdk'];
    default:
      return ['native_sdk'];
  }
}

function normalizeProviderForRuntime(
  runtimeKind: AgentRuntimeKind,
  provider: AgentModelProvider,
): AgentModelProvider {
  if (runtimeKind === 'codex' && provider === 'anthropic') {
    return 'openai';
  }
  return provider;
}

function availableProvidersForRuntime(
  runtimeKind: AgentRuntimeKind,
  providerOptions: AgentModelProviderOption[],
): AgentModelProviderOption[] {
  if (runtimeKind !== 'codex') {
    return providerOptions;
  }
  return providerOptions.filter((provider) => provider.value === 'openai' || provider.value === 'openrouter');
}

function fallbackPresetKey(agent?: Pick<Agent, 'preset_key' | 'is_system'> | null): AgentPresetKey {
  if (agent?.preset_key) return agent.preset_key;
  if (agent?.is_system) return 'epic_planner';
  return DEFAULT_SYSTEM_PRESET_KEY;
}

function fallbackPresetVersionKey(presetKey: AgentPresetKey): string {
  return `${presetKey}_default`;
}

function presetVersionsForKey(presetKey: AgentPresetKey, presets: AgentPresetDefinition[]): AgentPresetDefinition[] {
  return presets
    .filter((preset) => (preset.family_key || preset.key) === presetKey)
    .sort((left, right) => {
      if (left.is_default_version === right.is_default_version) {
        return left.version_label.localeCompare(right.version_label);
      }
      return left.is_default_version ? -1 : 1;
    });
}

function defaultPresetForKey(presetKey: AgentPresetKey, presets: AgentPresetDefinition[]): AgentPresetDefinition | null {
  const matching = presetVersionsForKey(presetKey, presets);
  return matching.find((preset) => preset.is_default_version) ?? matching[0] ?? null;
}

function presetMetaForSelection(
  presetKey: AgentPresetKey,
  presetVersionKey: string | undefined,
  presets: AgentPresetDefinition[],
): AgentPresetDefinition | null {
  const matching = presetVersionsForKey(presetKey, presets);
  if (matching.length === 0) {
    return null;
  }
  if (presetVersionKey) {
    const exact = matching.find((preset) => preset.version_key === presetVersionKey);
    if (exact) {
      return exact;
    }
  }
  return defaultPresetForKey(presetKey, presets);
}

function presetMetaForKey(presetKey: AgentPresetKey, presets: AgentPresetDefinition[]): AgentPresetDefinition | null {
  return defaultPresetForKey(presetKey, presets);
}

function presetLabel(presetKey: AgentPresetKey, presets: AgentPresetDefinition[]): string {
  return presetMetaForKey(presetKey, presets)?.label ?? PRESET_FALLBACKS[presetKey].label;
}

function presetRuntimeKindForSelection(
  presetKey: AgentPresetKey,
  presetVersionKey: string,
  presets: AgentPresetDefinition[],
): AgentRuntimeKind {
  return presetMetaForSelection(presetKey, presetVersionKey, presets)?.runtime_kind ?? PRESET_FALLBACKS[presetKey].runtime_kind;
}

function normalizeToolList(tools: string[]): string[] {
  const seen = new Set<string>();
  return tools.reduce<string[]>((result, tool) => {
    const normalized = tool.trim();
    if (!normalized || seen.has(normalized)) {
      return result;
    }
    seen.add(normalized);
    result.push(normalized);
    return result;
  }, []);
}

function normalizeTargetList(targets: AgentTargetType[]): AgentTargetType[] {
  const seen = new Set<AgentTargetType>();
  return targets.reduce<AgentTargetType[]>((result, target) => {
    if (seen.has(target)) {
      return result;
    }
    seen.add(target);
    result.push(target);
    return result;
  }, []);
}

function createEmptyCustomForm(): AgentFormData {
  return {
    name: '',
    preset_key: DEFAULT_SYSTEM_PRESET_KEY,
    preset_version_key: fallbackPresetVersionKey(DEFAULT_SYSTEM_PRESET_KEY),
    runtime_kind: 'opencode',
    supported_modes: ['autonomous'],
    provider: 'anthropic',
    model: '',
    reasoning_effort: '',
    service_tier: '',
    system_prompt: '',
    instruction_preamble: '',
    instruction_skills: [],
    monthly_token_budget: '',
    team_id: '',
    allowed_targets: ['task'],
    allowed_tools: [],
    skills: [],
    schedule: '',
    approval_mode: 'never',
    max_concurrent_runs: '1',
    default_invocation_mode: 'autonomous',
  };
}

function deriveExecutionConfigFields(
  runtimeKind: AgentRuntimeKind,
  provider: AgentModelProvider,
  executionConfig?: AgentExecutionConfig,
): Pick<AgentFormData, 'reasoning_effort' | 'service_tier'> {
  const normalizedProvider = normalizeProviderForRuntime(runtimeKind, provider);
  const reasoningEffort = runtimeKind === 'codex' ? (executionConfig?.reasoning_effort ?? '') : '';
  const serviceTier = runtimeKind === 'codex' && normalizedProvider === 'openai'
    ? (executionConfig?.service_tier ?? '')
    : '';
  return {
    reasoning_effort: reasoningEffort,
    service_tier: serviceTier,
  };
}

function buildExecutionConfigPayload(form: AgentFormData): AgentExecutionConfig | undefined {
  if (form.runtime_kind !== 'codex') {
    return undefined;
  }
  const config: AgentExecutionConfig = {};
  if (form.reasoning_effort) {
    config.reasoning_effort = form.reasoning_effort;
  }
  if (form.provider === 'openai' && form.service_tier) {
    config.service_tier = form.service_tier;
  }
  return Object.keys(config).length > 0 ? config : undefined;
}

function supportedModesForForm(runtimeKind: AgentRuntimeKind): AgentInvocationMode[] {
  if (runtimeKind === 'native_sdk' || runtimeKind === 'codex') {
    return ['autonomous', 'interactive'];
  }
  return ['autonomous'];
}

function normalizeDefaultInvocationMode(
  value: AgentInvocationMode | undefined,
  runtimeKind: AgentRuntimeKind,
  preferredMode: AgentInvocationMode = 'autonomous',
): AgentInvocationMode {
  const supportedModes = supportedModesForForm(runtimeKind);
  if (value && supportedModes.includes(value)) {
    return value;
  }
  if (supportedModes.includes(preferredMode)) {
    return preferredMode;
  }
  return 'autonomous';
}

function hasConfiguredAdvancedFields(agent: Agent | null, presets: AgentPresetDefinition[]): boolean {
  if (!agent) return false;
  if (!agent.is_system) {
    return Boolean(agent.monthly_token_budget || agent.execution_config?.reasoning_effort || agent.execution_config?.service_tier);
  }
  const presetKey = fallbackPresetKey(agent);
  const presetVersionKey = agent.preset_version_key ?? fallbackPresetVersionKey(presetKey);
  return (
    agent.runtime_kind !== presetRuntimeKindForSelection(presetKey, presetVersionKey, presets) ||
    Boolean(agent.monthly_token_budget || agent.execution_config?.reasoning_effort || agent.execution_config?.service_tier)
  );
}

function buildCreatePayload(workspaceId: string, form: AgentFormData, advancedOpen: boolean): CreateAgentRequest {
  const defaultRuntimeKind: AgentRuntimeKind = 'opencode';
  const provider = normalizeProviderForRuntime(form.runtime_kind, form.provider);
  return {
    workspace_id: workspaceId,
    name: form.name.trim(),
    provider,
    model: form.model.trim() || undefined,
    execution_config: buildExecutionConfigPayload(form),
    system_prompt: form.system_prompt.trim() || undefined,
    trigger_mode: 'manual',
    team_id: form.team_id,
    allowed_tools: normalizeToolList(form.allowed_tools),
    skills: form.skills.length > 0 ? form.skills : undefined,
    allowed_targets: normalizeTargetList(form.allowed_targets),
    schedule: form.schedule.trim(),
    approval_mode: form.approval_mode,
    max_concurrent_runs: form.max_concurrent_runs ? Number.parseInt(form.max_concurrent_runs, 10) : 1,
    default_invocation_mode: form.default_invocation_mode,
    ...(advancedOpen
      ? {
          runtime_kind: form.runtime_kind,
          monthly_token_budget: form.monthly_token_budget.trim()
            ? Number.parseInt(form.monthly_token_budget, 10)
            : 0,
        }
      : form.runtime_kind !== defaultRuntimeKind
        ? { runtime_kind: form.runtime_kind }
        : {}),
  };
}

function buildUpdatePayload(
  form: AgentFormData,
  advancedOpen: boolean,
  presets: AgentPresetDefinition[],
  agent: Agent | null,
): UpdateAgentRequest {
  const preset = agent?.is_system ? presetMetaForSelection(form.preset_key, form.preset_version_key, presets) : null;
  const defaultRuntimeKind = agent?.is_system
    ? (preset?.runtime_kind ?? PRESET_FALLBACKS[form.preset_key].runtime_kind)
    : 'opencode';
  const provider = normalizeProviderForRuntime(form.runtime_kind, form.provider);
  const payload: UpdateAgentRequest = {
    name: form.name.trim(),
    trigger_mode: 'manual',
    provider: provider || undefined,
    model: form.model.trim() || undefined,
    execution_config: buildExecutionConfigPayload(form),
    system_prompt: form.system_prompt.trim() || undefined,
    team_id: form.team_id,
    allowed_tools: normalizeToolList(form.allowed_tools),
    schedule: form.schedule.trim(),
    approval_mode: form.approval_mode,
    max_concurrent_runs: form.max_concurrent_runs ? Number.parseInt(form.max_concurrent_runs, 10) : 1,
    default_invocation_mode: form.default_invocation_mode,
    ...(advancedOpen
      ? {
          runtime_kind: form.runtime_kind,
          monthly_token_budget: form.monthly_token_budget.trim()
            ? Number.parseInt(form.monthly_token_budget, 10)
            : 0,
        }
      : form.runtime_kind !== defaultRuntimeKind
        ? { runtime_kind: form.runtime_kind }
        : {}),
  };
  if (agent?.is_system) {
    payload.preset_key = form.preset_key;
    payload.preset_version_key = form.preset_version_key;
    payload.monthly_token_budget = form.monthly_token_budget.trim()
      ? Number.parseInt(form.monthly_token_budget, 10)
      : 0;
  } else {
    payload.allowed_targets = normalizeTargetList(form.allowed_targets);
    payload.skills = form.skills;
  }
  return payload;
}

function buildSystemAgentForm(agent: Agent, presets: AgentPresetDefinition[]): AgentFormData {
  const presetKey = fallbackPresetKey(agent);
  const preset = presetMetaForSelection(presetKey, agent.preset_version_key, presets);
  const runtimeKind = preset?.runtime_kind ?? agent.runtime_kind ?? PRESET_FALLBACKS[presetKey].runtime_kind;
  const supportedModes = agent.supported_modes && agent.supported_modes.length > 0
    ? agent.supported_modes
    : (preset?.supported_modes ?? supportedModesForForm(runtimeKind));
  const defaultInvocationMode = agent.default_invocation_mode?.trim()
    ? agent.default_invocation_mode
    : (preset?.default_invocation_mode ?? PRESET_FALLBACKS[presetKey].default_invocation_mode);
  return {
    name: agent.name,
    preset_key: presetKey,
    preset_version_key: agent.preset_version_key?.trim() || preset?.version_key || fallbackPresetVersionKey(presetKey),
    runtime_kind: agent.runtime_kind || runtimeKind,
    supported_modes: supportedModes,
    provider: normalizeProviderForRuntime(
      agent.runtime_kind || runtimeKind,
      agent.provider ?? preset?.provider ?? PRESET_FALLBACKS[presetKey].provider ?? 'anthropic',
    ),
    model: agent.model ?? preset?.model ?? PRESET_FALLBACKS[presetKey].model ?? '',
    ...deriveExecutionConfigFields(
      agent.runtime_kind || runtimeKind,
      normalizeProviderForRuntime(
        agent.runtime_kind || runtimeKind,
        agent.provider ?? preset?.provider ?? PRESET_FALLBACKS[presetKey].provider ?? 'anthropic',
      ),
      agent.execution_config ?? preset?.execution_config,
    ),
    system_prompt: agent.system_prompt ?? preset?.system_prompt ?? '',
    instruction_preamble: preset?.instruction_preamble ?? '',
    instruction_skills: preset?.instruction_skills ?? [],
    monthly_token_budget: agent.monthly_token_budget?.toString() ?? '',
    team_id: '',
    allowed_targets: normalizeTargetList(
      agent.allowed_targets?.length
        ? (agent.allowed_targets as AgentTargetType[])
        : (preset?.allowed_target_types ?? []),
    ),
    allowed_tools: normalizeToolList(agent.allowed_tools?.length ? agent.allowed_tools : (preset?.allowed_tools ?? [])),
    skills: agent.skills ?? [],
    schedule: '',
    approval_mode: 'never',
    max_concurrent_runs: agent.max_concurrent_runs?.toString() ?? '1',
    default_invocation_mode: normalizeDefaultInvocationMode(
      defaultInvocationMode,
      agent.runtime_kind || runtimeKind,
      preset?.default_invocation_mode ?? PRESET_FALLBACKS[presetKey].default_invocation_mode,
    ),
  };
}

// ---------------------------------------------------------------------------
// Inline helper: label + optional tooltip
// ---------------------------------------------------------------------------

function SectionHeader({ number, title, description }: { number: string; title: string; description?: string }) {
  return (
    <div className="flex items-baseline justify-between gap-4 border-b border-border/60 pb-2">
      <div className="flex items-baseline gap-2">
        <span className="text-xs font-medium text-primary/70">{number}</span>
        <h3 className="text-sm font-semibold">{title}</h3>
      </div>
      {description && (
        <p className="shrink-0 text-xs text-muted-foreground">{description}</p>
      )}
    </div>
  );
}

function FieldLabel({ htmlFor, children, tooltip }: { htmlFor?: string; children: React.ReactNode; tooltip?: string }) {
  return (
    <div className="flex items-center gap-1.5">
      <Label htmlFor={htmlFor} className="text-[11px] font-semibold uppercase tracking-wider text-muted-foreground">{children}</Label>
      {tooltip && (
        <Tooltip>
          <TooltipTrigger asChild>
            <HelpCircleIcon className="h-3.5 w-3.5 text-muted-foreground/60 cursor-help" />
          </TooltipTrigger>
          <TooltipContent side="right" className="max-w-56 text-xs">
            {tooltip}
          </TooltipContent>
        </Tooltip>
      )}
    </div>
  );
}

function resolveManagePath(path: string | undefined, workspaceSlug?: string) {
  if (!path) return undefined;
  if (!workspaceSlug) return path.replace('$slug', '<workspace>');
  return path.replace('$slug', workspaceSlug);
}

function resolveTriggerHistoryPath(
  workspaceSlug: string | undefined,
  agentID: string | undefined,
  item?: Pick<AgentTriggerUsage, 'execution_search'>,
) {
  const base = buildAutomationActivityPath(workspaceSlug);
  const params = new URLSearchParams({ page: '1' });
  if (agentID) params.set('agent_id', agentID);
  if (item?.execution_search?.binding_id) params.set('binding_id', item.execution_search.binding_id);
  if (item?.execution_search?.trigger_type) params.set('trigger_type', item.execution_search.trigger_type);
  if (item?.execution_search?.source) params.set('source', item.execution_search.source);
  if (item?.execution_search?.reference_id) params.set('reference_id', item.execution_search.reference_id);
  if (item?.execution_search?.status) params.set('status', item.execution_search.status);
  return `${base}?${params.toString()}#trigger-executions`;
}

function AgentTriggerPanel({
  workspaceId,
  workspaceSlug,
  usage,
  loading,
  onRefresh,
}: {
  workspaceId?: string;
  workspaceSlug?: string;
  usage: AgentTriggerUsageSummary | null;
  loading: boolean;
  onRefresh: () => Promise<void>;
}) {
  const handleDeleteRule = async (ruleId: string) => {
    if (!workspaceId) return;
    const res = await automationService.deleteFlow(workspaceId, ruleId);
    if (res.error) {
      toast.error('Failed to remove trigger', { description: res.error });
      return;
    }
    toast.success('Trigger removed');
    await onRefresh();
  };

  return (
    <div className="rounded-2xl border border-border/60 bg-muted/20 p-5">
      <div className="flex items-center justify-between">
        <p className="text-[10px] font-semibold uppercase tracking-wider text-muted-foreground/70">Triggered By</p>
        {usage?.agent_id && (
          <a
            href={resolveTriggerHistoryPath(workspaceSlug, usage.agent_id)}
            className="text-[11px] text-muted-foreground transition-colors hover:text-foreground"
          >
            View history
          </a>
        )}
      </div>
      <p className="mt-1 text-[11px] text-muted-foreground">
        Active triggers and schedules that can start this agent.
      </p>

      <div className="mt-4 space-y-2">
        {loading ? (
          <div className="flex items-center gap-2 py-4 text-sm text-muted-foreground">
            <Loading01Icon className="h-3.5 w-3.5 animate-spin" />
            Loading triggers...
          </div>
        ) : usage?.items.length ? (
          usage.items.map((item) => {
            const managePath = resolveManagePath(item.manage_path, workspaceSlug);
            const historyPath = resolveTriggerHistoryPath(workspaceSlug, usage.agent_id, item);
            const removable = item.reference_type === 'automation_rule' && item.reference_id;
            const lastTriggered = item.last_triggered_at ? formatDistanceToNow(new Date(item.last_triggered_at), { addSuffix: true }) : null;
            const lastSuccess = item.last_success_at ? formatDistanceToNow(new Date(item.last_success_at), { addSuffix: true }) : null;
            const lastError = item.last_error_at ? formatDistanceToNow(new Date(item.last_error_at), { addSuffix: true }) : null;
            return (
              <div key={item.id} className="rounded-lg border border-border/60 bg-card px-3 py-2.5">
                <div className="flex items-start justify-between gap-2">
                  <div className="min-w-0 flex-1">
                    <div className="flex flex-wrap items-center gap-2">
                      <ZapIcon className="h-3.5 w-3.5 shrink-0 text-amber-500" />
                      <p className="text-sm font-medium">{item.title}</p>
                      {!item.enabled && <Badge variant="outline" className="text-[10px]">Off</Badge>}
                      {item.trigger_type && <Badge variant="outline" className="font-mono text-[10px]">{item.trigger_type}</Badge>}
                    </div>
                    {item.description && (
                      <p className="mt-1 pl-[1.375rem] text-xs text-muted-foreground">{item.description}</p>
                    )}
                    {(lastTriggered || lastSuccess || lastError) && (
                      <div className="mt-2 flex flex-wrap gap-x-3 gap-y-1 pl-[1.375rem] text-[11px] text-muted-foreground">
                        {lastTriggered && <span>Fired {lastTriggered}</span>}
                        {lastSuccess && <span className="text-emerald-600 dark:text-emerald-400">Success {lastSuccess}</span>}
                        {lastError && <span className="text-rose-600 dark:text-rose-400">Error {lastError}</span>}
                      </div>
                    )}
                    {item.last_error && (
                      <div className="mt-2 ml-[1.375rem] rounded-md border border-rose-200 bg-rose-50 px-2.5 py-1.5 text-[11px] text-rose-700 dark:border-rose-900/60 dark:bg-rose-950/30 dark:text-rose-300">
                        {item.last_error}
                      </div>
                    )}
                    {item.recent_executions && item.recent_executions.length > 0 && (
                      <div className="mt-3 ml-[1.375rem] space-y-1.5">
                        <p className="text-[10px] font-semibold uppercase tracking-wider text-muted-foreground/70">Recent</p>
                        {item.recent_executions.map((execution) => (
                          <div key={execution.execution_id} className="flex flex-wrap items-center gap-2 text-[11px]">
                            <span className={cn(
                              'inline-block h-1.5 w-1.5 rounded-full',
                              execution.status === 'completed' ? 'bg-emerald-500' : execution.status === 'failed' ? 'bg-rose-500' : 'bg-amber-500',
                            )} />
                            <span className="capitalize">{execution.status}</span>
                            <span className="text-muted-foreground">
                              {formatDistanceToNow(new Date(execution.fired_at), { addSuffix: true })}
                            </span>
                            {execution.error_message && (
                              <span className="truncate text-rose-600 dark:text-rose-400">{execution.error_message}</span>
                            )}
                          </div>
                        ))}
                      </div>
                    )}
                    <div className="mt-2 flex flex-wrap gap-3 pl-[1.375rem]">
                      <a href={historyPath} className="text-[11px] text-muted-foreground underline-offset-2 hover:text-foreground hover:underline">
                        History
                      </a>
                      {managePath && (
                        <a href={managePath} className="text-[11px] text-muted-foreground underline-offset-2 hover:text-foreground hover:underline">
                          Manage
                        </a>
                      )}
                    </div>
                  </div>
                  {removable ? (
                    <Button type="button" variant="ghost" size="sm" className="h-7 px-2 text-xs text-muted-foreground" onClick={() => handleDeleteRule(item.reference_id!)}>
                      Remove
                    </Button>
                  ) : null}
                </div>
              </div>
            );
          })
        ) : (
          /* Empty state */
          <div className="flex flex-col items-center py-5 text-center">
            <div className="flex h-10 w-10 items-center justify-center rounded-full bg-muted">
              <ZapIcon className="h-5 w-5 text-muted-foreground/60" />
            </div>
            <p className="mt-2.5 text-sm font-medium text-muted-foreground">No active triggers</p>
            <p className="mt-0.5 text-[11px] text-muted-foreground/70">
              Only manual runs can start this agent.
            </p>
            {workspaceSlug && (
              <a
                href={buildAutomationFlowsPath(workspaceSlug)}
                className="mt-3 inline-flex items-center gap-1.5 rounded-lg border border-border/60 px-3 py-1.5 text-xs font-medium transition-colors hover:bg-muted"
              >
                <WorkflowSquare01Icon className="h-3.5 w-3.5" />
                Open Automation Flows
              </a>
            )}
          </div>
        )}
      </div>

      {/* Show flows link when there ARE triggers too */}
      {usage?.items.length && workspaceSlug ? (
        <div className="mt-3 border-t border-border/40 pt-3 text-center">
          <a
            href={buildAutomationFlowsPath(workspaceSlug)}
            className="inline-flex items-center gap-1.5 text-[11px] text-muted-foreground transition-colors hover:text-foreground"
          >
            <WorkflowSquare01Icon className="h-3 w-3" />
            Manage flows
          </a>
        </div>
      ) : null}
    </div>
  );
}

// ---------------------------------------------------------------------------
// Run stats helper
// ---------------------------------------------------------------------------

interface AgentRunStats {
  total: number;
  lastRun?: AgentRun;
}

interface AgentCollectionSection {
  key: string;
  title: string;
  description: string;
  agents: Agent[];
  empty?: string;
}

function agentClassLabel(agent: Agent, presets: AgentPresetDefinition[]): string {
  if (!agent.is_system) {
    return 'Custom';
  }
  return presetLabel(fallbackPresetKey(agent), presets);
}

function formatLastRun(run?: AgentRun): string {
  if (!run) return 'Never';
  const date = run.completed_at || run.started_at || run.created_at;
  return formatDistanceToNow(new Date(date), { addSuffix: true });
}

function lastRunStatusColor(run?: AgentRun): string {
  if (!run) return '';
  switch (run.status) {
    case 'completed': return 'text-green-600';
    case 'failed': return 'text-red-500';
    case 'running': return 'text-amber-500';
    case 'cancelled': return 'text-muted-foreground';
    default: return 'text-muted-foreground';
  }
}

// ---------------------------------------------------------------------------
// AgentCard
// ---------------------------------------------------------------------------

function AgentCard({
  agent,
  presetLabel,
  teamName,
  stats,
  onEdit,
  canEdit,
}: {
  agent: Agent;
  presetLabel: string;
  teamName?: string;
  stats?: AgentRunStats;
  onEdit: (agent: Agent) => void;
  canEdit: boolean;
}) {
  const budgetPct =
    agent.monthly_token_budget
      ? Math.min(
          100,
          Math.round((agent.tokens_used_this_month / agent.monthly_token_budget) * 100)
        )
      : null;

  return (
    <Card className="group cursor-pointer transition-shadow hover:shadow-md relative">
      <CardHeader className="pb-2">
        <div className="flex items-start justify-between gap-2">
          <div className="flex min-w-0 items-center gap-2.5">
            <AgentAvatar agent={agent} className="h-9 w-9" />
            <div className="min-w-0">
              <span className="block truncate text-sm font-semibold">{agent.name}</span>
              <span className="block text-[11px] text-muted-foreground">
                {presetLabel}
              </span>
            </div>
          </div>
          <div className="flex items-center gap-1.5 shrink-0">
            <span className="text-[11px] text-muted-foreground group-hover:hidden">{STATUS_LABEL[agent.status] ?? agent.status}</span>
            <span
              className={`h-2 w-2 rounded-full group-hover:hidden ${STATUS_DOT[agent.status] ?? STATUS_DOT.paused}`}
            />
            {canEdit && (
              <Tooltip>
                <TooltipTrigger asChild>
                  <button
                    type="button"
                    className="hidden group-hover:flex p-1 rounded-md hover:bg-muted transition-colors text-muted-foreground hover:text-foreground"
                    onClick={(e) => { e.stopPropagation(); onEdit(agent); }}
                  >
                    <PencilEdit01Icon className="h-3.5 w-3.5" />
                  </button>
                </TooltipTrigger>
                <TooltipContent side="left" className="text-xs">Edit agent</TooltipContent>
              </Tooltip>
            )}
          </div>
        </div>
        <div className="flex items-center gap-2 flex-wrap">
          {agent.is_system && (
            <Badge variant="outline" className="text-[11px]">
              System
            </Badge>
          )}
          {teamName && (
            <span className="flex items-center gap-1 text-[11px] text-muted-foreground">
              <UserGroupIcon className="h-3 w-3" />
              {teamName}
            </span>
          )}
        </div>
      </CardHeader>
      <CardContent className="space-y-2 pt-0">
        {(agent.provider || agent.model) && (
          <p className="text-xs text-muted-foreground">
            {[agent.provider, agent.model].filter(Boolean).join(' / ')}
          </p>
        )}
        <div className="flex items-center gap-3 text-xs text-muted-foreground">
          <span className="flex items-center gap-1">
            <BotIcon className="h-3 w-3" />
            {INVOCATION_MODE_LABELS[agent.default_invocation_mode]}
          </span>
          {agent.schedule && (
            <span className="flex items-center gap-1">
              <Clock01Icon className="h-3 w-3" />
              Scheduled
            </span>
          )}
          {stats && (
            <span className="flex items-center gap-1">
              <ZapIcon className="h-3 w-3" />
              {stats.total > 0 ? `${stats.total} ${stats.total === 1 ? 'run' : 'runs'}` : 'No runs'}
            </span>
          )}
        </div>
        {stats?.lastRun && (
          <p className="text-[11px] text-muted-foreground">
            Last run{' '}
            <span className={lastRunStatusColor(stats.lastRun)}>
              {stats.lastRun.status}
            </span>{' '}
            {formatLastRun(stats.lastRun)}
          </p>
        )}
        {budgetPct !== null && (
          <div className="space-y-1">
            <div className="flex justify-between text-[11px] text-muted-foreground">
              <span>Usage</span>
              <span>{budgetPct}%</span>
            </div>
            <Progress value={budgetPct} className="h-1.5" />
          </div>
        )}
      </CardContent>
    </Card>
  );
}

// ---------------------------------------------------------------------------
// AgentRow (list view)
// ---------------------------------------------------------------------------

function AgentRow({
  agent,
  presetLabel,
  teamName,
  stats,
  onEdit,
  canEdit,
}: {
  agent: Agent;
  presetLabel: string;
  teamName?: string;
  stats?: AgentRunStats;
  onEdit: (agent: Agent) => void;
  canEdit: boolean;
}) {
  return (
    <div
      className="group flex items-center gap-3 px-4 py-3 border-b border-border/50 last:border-b-0 hover:bg-muted/40 transition-colors"
    >
      {/* Status dot + Name */}
      <div className="flex items-center gap-2.5 flex-1 min-w-[120px]">
        <span
          className={`h-2 w-2 shrink-0 rounded-full ${STATUS_DOT[agent.status] ?? STATUS_DOT.paused}`}
        />
        <AgentAvatar agent={agent} className="h-8 w-8" />
        <div className="flex min-w-0 items-center gap-2">
          <span className="text-sm font-medium truncate">{agent.name}</span>
          {agent.is_system && (
            <Badge variant="outline" className="text-[10px]">
              System
            </Badge>
          )}
        </div>
      </div>

      {/* Class */}
      <span className="text-xs text-muted-foreground w-20 shrink-0 truncate">
        {presetLabel}
      </span>

      {/* Team */}
      <span className="text-xs text-muted-foreground w-28 shrink-0 truncate hidden md:block">
        {teamName ?? 'All teams'}
      </span>

      {/* Provider / Model */}
      <span className="text-xs text-muted-foreground w-28 shrink-0 truncate hidden lg:block">
        {[agent.provider, agent.model].filter(Boolean).join(' / ') || '—'}
      </span>

      {/* Default mode */}
      <span className="text-xs text-muted-foreground w-24 shrink-0 truncate hidden lg:block">
        {INVOCATION_MODE_LABELS[agent.default_invocation_mode]}
      </span>

      {/* Runs */}
      <span className="text-xs text-muted-foreground w-12 shrink-0 hidden sm:block">
        {stats ? (stats.total > 0 ? stats.total : '0') : '—'}
      </span>

      {/* Last run */}
      <span className="text-xs text-muted-foreground w-28 shrink-0 truncate hidden sm:block">
        {stats?.lastRun ? (
          <span>
            <span className={lastRunStatusColor(stats.lastRun)}>{stats.lastRun.status}</span>
            {' '}
            {formatLastRun(stats.lastRun)}
          </span>
        ) : (
          'Never'
        )}
      </span>

      {/* Edit button on hover */}
      <div className="w-8 shrink-0 flex justify-center">
        {canEdit && (
          <Tooltip>
            <TooltipTrigger asChild>
              <button
                type="button"
                className="p-1.5 rounded-md opacity-0 group-hover:opacity-100 hover:bg-muted transition-all text-muted-foreground hover:text-foreground"
                onClick={(e) => { e.stopPropagation(); onEdit(agent); }}
              >
                <PencilEdit01Icon className="h-3.5 w-3.5" />
              </button>
            </TooltipTrigger>
            <TooltipContent side="left" className="text-xs">Edit agent</TooltipContent>
          </Tooltip>
        )}
      </div>
    </div>
  );
}

// ---------------------------------------------------------------------------
// AgentsPage
// ---------------------------------------------------------------------------

export function AgentsPage() {
  useTitle('Agents');
  const workspace = useWorkspaceStore((state) => state.currentWorkspace);
  const workspaceId = workspace?.id;
  const { data: access } = useWorkspaceAccess(workspaceId ?? '');
  const { canEdit } = usePermissions(access);
  const { teams: accessibleTeams, isAdmin } = useAccessibleTeams(workspaceId ?? '');

  const [agents, setAgents] = useState<Agent[]>([]);
  const [providerOptions, setProviderOptions] = useState<AgentModelProviderOption[]>(FALLBACK_PROVIDER_OPTIONS);
  const [presets, setPresets] = useState<AgentPresetDefinition[]>([]);
  const [toolCatalog, setToolCatalog] = useState<ToolCatalogResponse | null>(null);
  const [skillCatalog, setSkillCatalog] = useState<SkillCatalogResponse | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const { data: settings } = useWorkspaceSettings(workspaceId ?? '');
  const teams = settings?.teams ?? [];
  const teamMap = new Map(teams.map((t) => [t.id, t.name]));
  const accessibleTeamIds = useMemo(
    () => new Set(accessibleTeams.map((team) => team.id)),
    [accessibleTeams],
  );
  const visibleTeams = isAdmin ? teams : accessibleTeams;

  const [viewMode, setViewMode] = useState<'list' | 'cards'>('list');
  const [runStats, setRunStats] = useState<Record<string, AgentRunStats>>({});

  const [dialogOpen, setDialogOpen] = useState(false);
  const [systemDrawerOpen, setSystemDrawerOpen] = useState(false);
  const [editingAgent, setEditingAgent] = useState<Agent | null>(null);
  const [agentUsage, setAgentUsage] = useState<AgentTriggerUsageSummary | null>(null);
  const [agentUsageLoading, setAgentUsageLoading] = useState(false);
  const [form, setForm] = useState<AgentFormData>(createEmptyCustomForm());
  const [advancedOpen, setAdvancedOpen] = useState(false);
  const [automationOpen, setAutomationOpen] = useState(false);
  const [compiledPromptOpen, setCompiledPromptOpen] = useState(false);
  const [toolPickerOpen, setToolPickerOpen] = useState(false);
  const [skillPickerOpen, setSkillPickerOpen] = useState(false);
  const [saving, setSaving] = useState(false);
  const [deleteConfirmOpen, setDeleteConfirmOpen] = useState(false);
  const [versionDraftOpen, setVersionDraftOpen] = useState(false);
  const [versionLabelDraft, setVersionLabelDraft] = useState('');
  const [versionDescriptionDraft, setVersionDescriptionDraft] = useState('');
  const [creatingVersion, setCreatingVersion] = useState(false);

  const loadAgents = useCallback(async () => {
    if (!workspaceId) return;
    setLoading(true);
    setError(null);
    const agentsRes = await automationService.listAgents(workspaceId);
    if (agentsRes.error) {
      setError(agentsRes.error);
    } else {
      setAgents(agentsRes.data ?? []);
    }
    setLoading(false);
  }, [workspaceId]);

  const loadProviderOptions = useCallback(async () => {
    if (!workspaceId) return;
    const res = await agentService.listModelProviders(workspaceId);
    if (!res.error && res.data && res.data.length > 0) {
      setProviderOptions(res.data);
    }
  }, [workspaceId]);

  const loadPresets = useCallback(async () => {
    if (!workspaceId) return;
    const res = await agentService.listPresets(workspaceId);
    if (!res.error && res.data) {
      setPresets(res.data);
    }
  }, [workspaceId]);

  const loadToolCatalog = useCallback(async () => {
    if (!workspaceId) return;
    const res = await automationService.listToolCatalog(workspaceId);
    if (!res.error && res.data) {
      setToolCatalog(res.data);
    }
  }, [workspaceId]);

  const loadSkillCatalog = useCallback(async () => {
    if (!workspaceId) return;
    const res = await automationService.listSkillCatalog(workspaceId);
    if (!res.error && res.data) {
      setSkillCatalog(res.data);
    }
  }, [workspaceId]);

  const loadAgentUsage = useCallback(async (agentId: string) => {
    if (!workspaceId) return;
    setAgentUsageLoading(true);
    const res = await automationService.getAgentUsage(workspaceId, agentId);
    if (!res.error) {
      setAgentUsage(res.data ?? null);
    } else {
      setAgentUsage(null);
      toast.error('Failed to load trigger usage', { description: res.error });
    }
    setAgentUsageLoading(false);
  }, [workspaceId]);

  useEffect(() => {
    loadAgents();
    loadProviderOptions();
    loadPresets();
    loadToolCatalog();
    loadSkillCatalog();
  }, [loadAgents, loadProviderOptions, loadPresets, loadToolCatalog, loadSkillCatalog]);

  // Fetch run stats for all agents
  useEffect(() => {
    if (!workspaceId || agents.length === 0) return;
    const fetchStats = async () => {
      const results: Record<string, AgentRunStats> = {};
      await Promise.all(
        agents.map(async (agent) => {
          const res = await agentService.listRuns(workspaceId, agent.id);
          if (!res.error && res.data) {
            // Handle both paginated { data, total } and plain array responses
            const paginated = res.data;
            const runs = Array.isArray(paginated) ? paginated : (paginated.data ?? []);
            const total = Array.isArray(paginated) ? paginated.length : (paginated.total ?? 0);
            results[agent.id] = {
              total,
              lastRun: runs[0],
            };
          }
        })
      );
      setRunStats(results);
    };
    fetchStats();
  }, [workspaceId, agents]);

  const openCreateDialog = () => {
    setEditingAgent(null);
    setAgentUsage(null);
    setAdvancedOpen(false);
    setAutomationOpen(false);
    setToolPickerOpen(false);
    setSystemDrawerOpen(false);
    setVersionDraftOpen(false);
    setVersionLabelDraft('');
    setVersionDescriptionDraft('');
    setForm(createEmptyCustomForm());
    setDialogOpen(true);
  };

  const openEditDialog = (agent: Agent) => {
    setEditingAgent(agent);
    setAgentUsage(null);
    void loadAgentUsage(agent.id);
    setAdvancedOpen(hasConfiguredAdvancedFields(agent, presets));
    setAutomationOpen(agent.is_system ? false : Boolean(agent.schedule || agent.approval_mode !== 'preset_default'));
    setToolPickerOpen(false);
    setVersionDraftOpen(false);
    setVersionLabelDraft('');
    setVersionDescriptionDraft('');
    if (agent.is_system) {
      setForm(buildSystemAgentForm(agent, presets));
      setSystemDrawerOpen(true);
      setDialogOpen(false);
      return;
    }
    const presetKey = fallbackPresetKey(agent);
    const runtimeKind = agent.runtime_kind;
    setForm({
      name: agent.name,
      preset_key: presetKey,
      preset_version_key: fallbackPresetVersionKey(presetKey),
      runtime_kind: runtimeKind,
      supported_modes: supportedModesForForm(runtimeKind),
      provider: normalizeProviderForRuntime(runtimeKind, agent.provider ?? 'anthropic'),
      model: agent.model ?? '',
      ...deriveExecutionConfigFields(
        runtimeKind,
        normalizeProviderForRuntime(runtimeKind, agent.provider ?? 'anthropic'),
        agent.execution_config,
      ),
      system_prompt: agent.system_prompt ?? '',
      instruction_preamble: '',
      instruction_skills: [],
      monthly_token_budget: agent.monthly_token_budget?.toString() ?? '',
      team_id: agent.team_id ?? '',
      allowed_targets: normalizeTargetList(agent.allowed_targets as AgentTargetType[]),
      allowed_tools: normalizeToolList(agent.allowed_tools),
      skills: agent.skills ?? [],
      schedule: agent.schedule ?? '',
      approval_mode: agent.approval_mode ?? 'preset_default',
      max_concurrent_runs: agent.max_concurrent_runs?.toString() ?? '1',
      default_invocation_mode: normalizeDefaultInvocationMode(agent.default_invocation_mode, runtimeKind),
    });
    setDialogOpen(true);
  };

  const handleSave = async () => {
    if (!workspaceId || !form.name.trim()) return;
    setSaving(true);

    if (editingAgent) {
      const payload = buildUpdatePayload(form, advancedOpen, presets, editingAgent);
      const res = await automationService.updateAgent(workspaceId, editingAgent.id, payload);
      if (!res.error) {
        setDialogOpen(false);
        setSystemDrawerOpen(false);
        await loadAgents();
      } else {
        toast.error(editingAgent.is_system ? 'Failed to save built-in agent' : 'Failed to save custom agent', {
          description: res.error,
        });
      }
    } else {
      const payload = buildCreatePayload(workspaceId, form, advancedOpen);
      const res = await automationService.createAgent(workspaceId, payload);
      if (!res.error) {
        setDialogOpen(false);
        await loadAgents();
      } else {
        toast.error('Failed to create agent', { description: res.error });
      }
    }
    setSaving(false);
  };

  const handleCreatePresetVersion = async () => {
    if (!workspaceId || !editingAgent?.is_system || !versionLabelDraft.trim()) return;
    setCreatingVersion(true);
    const payload: CreateWorkspaceAgentPresetVersionRequest = {
      workspace_id: workspaceId,
      family_key: form.preset_key,
      label: versionLabelDraft.trim(),
      description: versionDescriptionDraft.trim() || undefined,
      source_version_key: form.preset_version_key,
      runtime_kind: form.runtime_kind,
      provider: form.provider,
      model: form.model.trim() || undefined,
      execution_config: buildExecutionConfigPayload(form),
      instruction_preamble: form.instruction_preamble.trim() || undefined,
      instruction_skills: form.instruction_skills.length > 0 ? form.instruction_skills : undefined,
      allowed_tools: normalizeToolList(form.allowed_tools),
      supported_modes: form.supported_modes,
      approval_mode: 'never',
      default_invocation_mode: form.default_invocation_mode,
    };
    const res = await agentService.createPresetVersion(workspaceId, payload);
    if (!res.error && res.data) {
      await loadPresets();
      setForm((current) => ({
        ...current,
        preset_version_key: res.data?.version_key ?? current.preset_version_key,
        system_prompt: res.data?.system_prompt ?? current.system_prompt,
        instruction_preamble: res.data?.instruction_preamble ?? current.instruction_preamble,
        instruction_skills: res.data?.instruction_skills ?? current.instruction_skills,
      }));
      setVersionDraftOpen(false);
      setVersionLabelDraft('');
      setVersionDescriptionDraft('');
    } else if (res.error) {
      toast.error('Failed to save workspace version', { description: res.error });
    }
    setCreatingVersion(false);
  };

  const handleDelete = async () => {
    if (!workspaceId || !editingAgent) return;
    setSaving(true);
    const res = await automationService.deleteAgent(workspaceId, editingAgent.id);
    if (!res.error) {
      setDialogOpen(false);
      await loadAgents();
    }
    setSaving(false);
  };

  if (!workspace) {
    return <p className="text-sm text-muted-foreground">Workspace not found.</p>;
  }

  const visibleAgents = agents.filter((agent) => {
    if (!agent.team_id) {
      return true;
    }
    return accessibleTeamIds.has(agent.team_id);
  });

  const builtInAgents = (() => {
    const byPreset = new Map<string, Agent>();
    for (const agent of visibleAgents) {
      if (!agent.is_system) {
        continue;
      }
      const presetKey = fallbackPresetKey(agent);
      if (!byPreset.has(presetKey)) {
        byPreset.set(presetKey, agent);
      }
    }
    return Array.from(byPreset.values());
  })();
  const customAgents = visibleAgents.filter((agent) => !agent.is_system);
  const agentSections: AgentCollectionSection[] = [
    {
      key: 'built-in',
      title: 'Built-in Presets',
      description: 'Pre-configured agents for core workflows like planning, coding, review, and support.',
      agents: builtInAgents,
      empty: 'No built-in preset agents are provisioned in this workspace yet.',
    },
    {
      key: 'custom',
      title: 'Custom Agents',
      description: 'Agents your team created with custom prompts, tools, and scheduling.',
      agents: customAgents,
      empty: 'No custom agents yet.',
    },
  ].filter((section) => section.agents.length > 0 || section.key === 'built-in');

  const advancedConfigured = hasConfiguredAdvancedFields(editingAgent, presets);
  const editingSystemAgent = Boolean(editingAgent?.is_system);
  const selectedPreset = editingSystemAgent ? presetMetaForSelection(form.preset_key, form.preset_version_key, presets) : null;
  const selectedPresetVersions = editingSystemAgent ? presetVersionsForKey(form.preset_key, presets) : [];
  const currentSystemVersionKey = editingSystemAgent
    ? (editingAgent?.preset_version_key?.trim() || selectedPreset?.version_key || fallbackPresetVersionKey(form.preset_key))
    : '';
  const selectedSystemVersionKey = editingSystemAgent
    ? (form.preset_version_key?.trim() || selectedPreset?.version_key || fallbackPresetVersionKey(form.preset_key))
    : '';
  const currentSystemPreset = editingSystemAgent
    ? presetMetaForSelection(form.preset_key, currentSystemVersionKey, presets)
    : null;
  const hasPendingSystemVersionSelection = editingSystemAgent
    && Boolean(selectedSystemVersionKey)
    && Boolean(currentSystemVersionKey)
    && selectedSystemVersionKey !== currentSystemVersionKey;
  const systemVersionReadOnly = editingSystemAgent && !versionDraftOpen;
  const effectiveTargets =
    editingSystemAgent
      ? (selectedPreset?.allowed_target_types ?? form.allowed_targets)
      : (form.allowed_targets.length > 0 ? form.allowed_targets : ['task']);
  const supportedModes = form.supported_modes.length > 0 ? form.supported_modes : supportedModesForForm(form.runtime_kind);
  const availableRuntimeKinds = editingSystemAgent ? allowedRuntimeKindsForPreset(form.preset_key) : (['opencode', 'native_sdk'] as AgentRuntimeKind[]);
  const visibleProviderOptions = availableProvidersForRuntime(form.runtime_kind, providerOptions);
  const selectedProviderOption = visibleProviderOptions.find((option) => option.value === form.provider);
  const supportsReasoningEffort = form.runtime_kind === 'codex' && Boolean(selectedProviderOption?.supports_reasoning_effort);
  const supportsServiceTier = form.runtime_kind === 'codex' && Boolean(selectedProviderOption?.supports_service_tier);
  const codexUsesPresetCapabilities = form.runtime_kind === 'codex';
  const toolCatalogEntries = toolCatalog?.tools ?? [];
  const availableToolEntries = toolCatalogEntries.filter((tool) => !form.allowed_tools.includes(tool.name));
  const addTool = (toolName: string) => {
    setForm((current) => ({
      ...current,
      allowed_tools: normalizeToolList([...current.allowed_tools, toolName]),
    }));
    setToolPickerOpen(false);
  };
  const removeTool = (toolName: string) => {
    setForm((current) => ({
      ...current,
      allowed_tools: current.allowed_tools.filter((tool) => tool !== toolName),
    }));
  };
  const skillCatalogEntries = skillCatalog?.skills ?? [];
  const attachedSkillKeys = new Set(form.skills.map((s) => s.key));
  const availableSkillEntries = skillCatalogEntries.filter((s) => !attachedSkillKeys.has(s.key));
  const addSkill = (entry: { id?: string; key: string; source_kind: string; required_tools?: string[] }) => {
    const ref: AgentSkillRef = { key: entry.key };
    if (entry.id) ref.skill_id = entry.id;
    const missingTools = (entry.required_tools ?? []).filter((t) => !form.allowed_tools.includes(t));
    if (missingTools.length > 0) {
      toast.warning(`Skill "${entry.key}" requires tools not yet allowed: ${missingTools.join(', ')}`);
    }
    setForm((current) => ({ ...current, skills: [...current.skills, ref] }));
    setSkillPickerOpen(false);
  };
  const removeSkill = (key: string) => {
    setForm((current) => ({ ...current, skills: current.skills.filter((s) => s.key !== key) }));
  };
  const toggleTarget =(target: AgentTargetType) => {
    setForm((current) => {
      const hasTarget = current.allowed_targets.includes(target);
      if (hasTarget && current.allowed_targets.length === 1) {
        toast.error('Custom agents need at least one target');
        return current;
      }
      return {
        ...current,
        allowed_targets: hasTarget
          ? current.allowed_targets.filter((value) => value !== target)
          : normalizeTargetList([...current.allowed_targets, target]),
      };
    });
  };
  const selectSystemPresetVersion = (versionKey: string) => {
    if (!editingAgent?.is_system) return;
    const nextPreset = presetMetaForSelection(form.preset_key, versionKey, presets);
    if (!nextPreset) return;
    const nextProvider = normalizeProviderForRuntime(
      nextPreset.runtime_kind,
      nextPreset.provider ?? PRESET_FALLBACKS[form.preset_key].provider ?? form.provider,
    );
    setVersionDraftOpen(false);
    setVersionLabelDraft('');
    setVersionDescriptionDraft('');
    setForm((current) => ({
      ...current,
      preset_version_key: nextPreset.version_key,
      runtime_kind: nextPreset.runtime_kind,
      supported_modes: nextPreset.supported_modes,
      provider: nextProvider,
      model: nextPreset.model ?? PRESET_FALLBACKS[form.preset_key].model ?? '',
      ...deriveExecutionConfigFields(nextPreset.runtime_kind, nextProvider, nextPreset.execution_config),
      system_prompt: nextPreset.system_prompt ?? '',
      instruction_preamble: nextPreset.instruction_preamble ?? '',
      instruction_skills: nextPreset.instruction_skills ?? [],
      allowed_tools: normalizeToolList(nextPreset.allowed_tools ?? []),
      approval_mode: 'never',
      default_invocation_mode: normalizeDefaultInvocationMode(
        nextPreset.default_invocation_mode,
        nextPreset.runtime_kind,
        nextPreset.default_invocation_mode,
      ),
    }));
  };

  return (
    <div className="max-w-5xl mx-auto space-y-4">
      <div className="flex items-center justify-between">
        <h1 className="text-xl font-semibold">Agents</h1>
        {visibleAgents.length > 0 && (
          <div className="flex items-center gap-2">
            <div className="flex items-center rounded-md border border-border">
              <button
                type="button"
                className={`p-1.5 ${viewMode === 'list' ? 'bg-muted text-foreground' : 'text-muted-foreground hover:text-foreground'}`}
                onClick={() => setViewMode('list')}
              >
                <LayoutTable01Icon className="h-4 w-4" />
              </button>
              <button
                type="button"
                className={`p-1.5 ${viewMode === 'cards' ? 'bg-muted text-foreground' : 'text-muted-foreground hover:text-foreground'}`}
                onClick={() => setViewMode('cards')}
              >
                <LayoutGridIcon className="h-4 w-4" />
              </button>
            </div>
            {canEdit && (
              <Button size="sm" onClick={openCreateDialog}>
                <PlusSignIcon className="mr-1.5 h-4 w-4" />
                New Custom Agent
              </Button>
            )}
          </div>
        )}
      </div>

      {loading && <p className="text-sm text-muted-foreground">Loading agents...</p>}
      {error && <p className="text-sm text-destructive">{error}</p>}

      {/* ---- Empty state with onboarding ---- */}
      {!loading && visibleAgents.length === 0 && !error && (
        <div className="flex flex-col items-center justify-center py-16 px-4">
          <div className="flex h-14 w-14 items-center justify-center rounded-full bg-violet-500/10 mb-5">
            <BotIcon className="h-7 w-7 text-violet-500" />
          </div>
          <h3 className="text-lg font-semibold mb-1.5">Create your first agent</h3>
          <p className="text-sm text-muted-foreground text-center max-w-md mb-6">
            AI-powered teammates that plan features, write code, review work, update docs, reply to customers, and manage deals — automatically or on demand.
          </p>
          {canEdit && (
            <Button className="gap-2 mb-8" onClick={openCreateDialog}>
              <PlusSignIcon className="h-4 w-4" />
              New Custom Agent
            </Button>
          )}
          <div className="grid grid-cols-1 sm:grid-cols-3 gap-4 w-full max-w-4xl">
            {EMPTY_STATE_CARDS.map((card) => (
              <div key={card.title} className="flex flex-col items-center text-center rounded-lg border border-border/50 bg-muted/30 p-6">
                <card.icon className="h-5 w-5 text-muted-foreground mb-3" />
                <p className="text-sm font-medium mb-1">{card.title}</p>
                <p className="text-sm text-muted-foreground leading-relaxed">{card.desc}</p>
              </div>
            ))}
          </div>
        </div>
      )}

      {/* ---- Agent list / grid ---- */}
      {visibleAgents.length > 0 && viewMode === 'list' && (
        <div className="space-y-5">
          {agentSections.map((section) => (
            <div key={section.key} className="space-y-2">
              <div className="space-y-1">
                <div className="flex items-center gap-2">
                  <h2 className="text-sm font-semibold">{section.title}</h2>
                  <Badge variant="outline" className="text-[10px]">
                    {section.agents.length}
                  </Badge>
                </div>
                <p className="text-xs text-muted-foreground">{section.description}</p>
              </div>
              <div className="rounded-lg border border-border overflow-hidden">
                <div className="flex items-center gap-3 px-4 py-2 border-b border-border bg-muted/40 text-[11px] font-medium text-muted-foreground uppercase tracking-wide">
                  <div className="flex-1 min-w-[120px]">Name</div>
                  <div className="w-20 shrink-0">Preset</div>
                  <div className="w-28 shrink-0 hidden md:block">Team</div>
                  <div className="w-28 shrink-0 hidden lg:block">Provider</div>
                  <div className="w-24 shrink-0 hidden lg:block">Mode</div>
                  <div className="w-12 shrink-0 hidden sm:block">Runs</div>
                  <div className="w-28 shrink-0 hidden sm:block">Last run</div>
                  <div className="w-8 shrink-0" />
                </div>
                {section.agents.length > 0 ? section.agents.map((agent) => (
                    <AgentRow
                      key={agent.id}
                      agent={agent}
                      presetLabel={agentClassLabel(agent, presets)}
                    teamName={agent.team_id ? teamMap.get(agent.team_id) : undefined}
                    stats={runStats[agent.id]}
                    onEdit={openEditDialog}
                    canEdit={canEdit}
                  />
                )) : (
                  <div className="px-4 py-6 text-sm text-muted-foreground">{section.empty}</div>
                )}
              </div>
            </div>
          ))}
        </div>
      )}

      {visibleAgents.length > 0 && viewMode === 'cards' && (
        <div className="space-y-5">
          {agentSections.map((section) => (
            <div key={section.key} className="space-y-2">
              <div className="space-y-1">
                <div className="flex items-center gap-2">
                  <h2 className="text-sm font-semibold">{section.title}</h2>
                  <Badge variant="outline" className="text-[10px]">
                    {section.agents.length}
                  </Badge>
                </div>
                <p className="text-xs text-muted-foreground">{section.description}</p>
              </div>
              {section.agents.length > 0 ? (
                <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
                  {section.agents.map((agent) => (
                    <AgentCard
                      key={agent.id}
                      agent={agent}
                      presetLabel={agentClassLabel(agent, presets)}
                      teamName={agent.team_id ? teamMap.get(agent.team_id) : undefined}
                      stats={runStats[agent.id]}
                      onEdit={openEditDialog}
                      canEdit={canEdit}
                    />
                  ))}
                </div>
              ) : (
                <div className="rounded-lg border border-dashed border-border/70 px-4 py-6 text-sm text-muted-foreground">
                  {section.empty}
                </div>
              )}
            </div>
          ))}
        </div>
      )}

      <Sheet
        open={systemDrawerOpen}
        onOpenChange={(open) => {
          setSystemDrawerOpen(open);
          if (!open) {
            setToolPickerOpen(false);
            setSkillPickerOpen(false);
            setCompiledPromptOpen(false);
            setVersionDraftOpen(false);
            setVersionLabelDraft('');
            setVersionDescriptionDraft('');
          }
        }}
      >
        <SheetContent side="right" className="w-full gap-0 p-0 data-[side=right]:w-[88vw] data-[side=right]:sm:max-w-[88vw] xl:data-[side=right]:w-[1280px] xl:data-[side=right]:max-w-[1280px]">
          <SheetHeader className="border-b border-border/60 bg-muted/20 px-6 py-5">
            <div className="flex items-start gap-4">
              <AgentAvatar agent={editingAgent ?? undefined} className="h-14 w-14 shrink-0" />
              <div className="min-w-0 space-y-1">
                <SheetTitle className="text-xl">{editingAgent?.name ?? 'Built-in Agent'}</SheetTitle>
                <SheetDescription className="max-w-3xl">
                  Built-in agents stay pinned to a preset family. Browse every product and workspace version here, inspect the configuration, then pin the agent to the version you want.
                </SheetDescription>
                {selectedPreset && (
                  <div className="flex flex-wrap items-center gap-2 pt-1">
                    <Badge variant="outline">{presetLabel(form.preset_key, presets)}</Badge>
                    <Badge variant={selectedPreset.scope === 'workspace' ? 'secondary' : 'outline'}>
                      {selectedPreset.scope === 'workspace' ? 'Workspace version' : 'Product version'}
                    </Badge>
                    {hasPendingSystemVersionSelection ? (
                      <Badge variant="secondary">Draft selection</Badge>
                    ) : selectedPreset.version_key === currentSystemVersionKey ? (
                      <Badge variant="secondary">Current on agent</Badge>
                    ) : null}
                  </div>
                )}
                {selectedPreset && hasPendingSystemVersionSelection && (
                  <p className="text-xs text-muted-foreground">
                    Current on agent: <span className="font-medium text-foreground">{currentSystemPreset?.version_label ?? currentSystemVersionKey}</span>. Save changes to switch to <span className="font-medium text-foreground">{selectedPreset.version_label}</span>.
                  </p>
                )}
              </div>
            </div>
          </SheetHeader>

          <div className="grid min-h-0 flex-1 lg:grid-cols-[18rem_minmax(0,1fr)]">
            <aside className="border-b border-border/60 bg-muted/20 lg:border-r lg:border-b-0">
              <div className="border-b border-border/60 px-5 py-4">
                <p className="text-sm font-semibold">Preset Versions</p>
                <p className="mt-1 text-xs leading-relaxed text-muted-foreground">
                  Product versions are shipped by Helpin. Workspace versions are copies you own locally.
                </p>
              </div>
              <div className="max-h-[28vh] overflow-y-auto px-3 py-3 lg:max-h-[calc(100vh-11rem)]">
                <div className="space-y-2">
                  {selectedPresetVersions.map((presetVersion) => {
                    const isSelected = presetVersion.version_key === form.preset_version_key;
                    const isCurrent = presetVersion.version_key === currentSystemVersionKey;
                    const isDraftSelection = isSelected && !isCurrent;
                    return (
                      <button
                        key={presetVersion.version_key}
                        type="button"
                        onClick={() => selectSystemPresetVersion(presetVersion.version_key)}
                        className={`w-full rounded-2xl border px-4 py-3 text-left transition-colors ${
                          isSelected
                            ? 'border-foreground/20 bg-background shadow-sm'
                            : 'border-border/60 bg-background/60 hover:border-border hover:bg-background'
                        }`}
                      >
                        <div className="flex items-start justify-between gap-3">
                          <div className="min-w-0">
                            <p className="truncate text-sm font-medium">{presetVersion.version_label}</p>
                            <p className="mt-1 line-clamp-2 text-xs leading-relaxed text-muted-foreground">
                              {presetVersion.description || 'No description'}
                            </p>
                          </div>
                          <div className="flex shrink-0 flex-col items-end gap-1">
                            <Badge variant={presetVersion.scope === 'workspace' ? 'secondary' : 'outline'} className="text-[10px]">
                              {presetVersion.scope === 'workspace' ? 'Workspace' : 'Product'}
                            </Badge>
                            {isCurrent && (
                              <Badge variant="outline" className="text-[10px]">
                                Current
                              </Badge>
                            )}
                            {isDraftSelection && (
                              <Badge variant="secondary" className="text-[10px]">
                                Draft
                              </Badge>
                            )}
                          </div>
                        </div>
                      </button>
                    );
                  })}
                </div>
              </div>
            </aside>

            <div className="min-h-0 overflow-y-auto px-6 py-6">
              <div className="space-y-6">
                <section className="rounded-2xl border border-amber-500/30 bg-amber-500/5 p-4 dark:bg-amber-900/10">
                  <div className="flex flex-col gap-4 lg:flex-row lg:items-start lg:justify-between">
                    <div className="space-y-2">
                      <div className="flex flex-wrap items-center gap-2">
                        <h3 className="text-base font-semibold">{selectedPreset?.version_label ?? 'Version'}</h3>
                        {selectedPreset && (
                          <Badge variant={selectedPreset.scope === 'workspace' ? 'secondary' : 'outline'}>
                            {selectedPreset.scope === 'workspace' ? 'Workspace-owned' : 'System-owned'}
                          </Badge>
                        )}
                      </div>
                      <p className="text-sm text-muted-foreground">
                        {selectedPreset?.description || 'This version has no description yet.'}
                      </p>
                      {selectedPreset?.source_version_key && (
                        <p className="text-xs text-muted-foreground">
                          Based on: <span className="font-mono">{selectedPreset.source_version_key}</span>
                        </p>
                      )}
                    </div>
                    <div className="flex flex-wrap gap-2">
                      <Button
                        type="button"
                        size="sm"
                        variant="outline"
                        onClick={() => {
                          setVersionDraftOpen(true);
                          setVersionLabelDraft(`${selectedPreset?.version_label ?? 'Version'} Copy`);
                          setVersionDescriptionDraft(selectedPreset?.description ?? '');
                        }}
                      >
                        Duplicate to workspace
                      </Button>
                    </div>
                  </div>
                </section>

                {versionDraftOpen && (
                  <section className="rounded-2xl border border-foreground/10 bg-card p-4 shadow-sm">
                    <div className="space-y-4">
                      <div>
                        <h3 className="text-base font-semibold">New Workspace Version</h3>
                        <p className="mt-1 text-sm text-muted-foreground">
                          This creates a new immutable workspace version from the configuration below. Save the agent afterwards to pin it to the new version.
                        </p>
                      </div>
                      <div className="grid gap-4 md:grid-cols-2">
                        <div className="space-y-2">
                          <FieldLabel htmlFor="preset-version-label">Version Label</FieldLabel>
                          <Input
                            id="preset-version-label"
                            value={versionLabelDraft}
                            onChange={(e) => setVersionLabelDraft(e.target.value)}
                            placeholder="e.g. Engineering tuned"
                          />
                        </div>
                        <div className="space-y-2 md:col-span-2">
                          <FieldLabel htmlFor="preset-version-description">Description</FieldLabel>
                          <Textarea
                            id="preset-version-description"
                            value={versionDescriptionDraft}
                            onChange={(e) => setVersionDescriptionDraft(e.target.value)}
                            placeholder="What changed in this version?"
                            rows={3}
                          />
                        </div>
                      </div>
                      <div className="flex flex-wrap gap-2">
                        <Button
                          type="button"
                          size="sm"
                          disabled={creatingVersion || !versionLabelDraft.trim()}
                          onClick={handleCreatePresetVersion}
                        >
                          {creatingVersion ? 'Saving version...' : 'Save workspace version'}
                        </Button>
                        <Button
                          type="button"
                          variant="ghost"
                          size="sm"
                          onClick={() => {
                            setVersionDraftOpen(false);
                            setVersionLabelDraft('');
                            setVersionDescriptionDraft('');
                            if (editingAgent) {
                              setForm(buildSystemAgentForm(editingAgent, presets));
                            }
                          }}
                        >
                          Cancel draft
                        </Button>
                      </div>
                    </div>
                  </section>
                )}

                <section className="grid gap-6 xl:grid-cols-[minmax(0,1fr)_18rem]">
                  <div className="space-y-6">
                    {/* 01A — Agent Identity (Preamble) */}
                    {(form.instruction_preamble || !systemVersionReadOnly) && (
                    <div className="space-y-3">
                      <SectionHeader number="01" title="Agent Identity" description="Role definition and high-level objective" />
                      <div className="rounded-xl border border-border/60 bg-card p-4">
                        {systemVersionReadOnly ? (
                          <>
                            <div className="mb-2 flex items-center justify-between">
                              <p className="text-xs font-mono text-muted-foreground">preamble</p>
                              <span className="text-[10px] font-semibold uppercase tracking-wider text-amber-600 dark:text-amber-400">Read-only</span>
                            </div>
                            <p className="whitespace-pre-wrap text-sm leading-relaxed text-foreground/90">{form.instruction_preamble}</p>
                          </>
                        ) : (
                          <>
                            <Textarea
                              value={form.instruction_preamble}
                              onChange={(e) => setForm((current) => ({ ...current, instruction_preamble: e.target.value }))}
                              placeholder="e.g. You are Epic Planner. You run the full PRD-to-tasks loop inside a single interactive run."
                              rows={3}
                              className="border-0 bg-transparent p-0 shadow-none focus-visible:ring-0"
                            />
                          </>
                        )}
                      </div>
                    </div>
                    )}

                    {/* 01B — Instruction Skills */}
                    {(form.instruction_skills.length > 0 || !systemVersionReadOnly) && (
                    <div className="space-y-3">
                      <SectionHeader number="01" title="Skills" description="Ordered instruction modules attached to this preset" />
                      <div className="space-y-2">
                        {form.instruction_skills.map((skillKey, idx) => {
                          const entry = skillCatalogEntries.find((s) => s.key === skillKey);
                          return (
                            <div key={skillKey} className="flex items-start gap-3 rounded-lg border border-border/50 bg-card px-3 py-2.5">
                              <span className="mt-0.5 flex h-5 w-5 shrink-0 items-center justify-center rounded bg-muted text-[10px] font-semibold text-muted-foreground">{idx + 1}</span>
                              <div className="min-w-0 flex-1">
                                <div className="flex items-center gap-2">
                                  <BookOpen01Icon className="h-3.5 w-3.5 shrink-0 text-muted-foreground" />
                                  <span className="font-mono text-xs font-medium">{skillKey}</span>
                                  {entry && (
                                    <Badge variant="outline" className="text-[9px] px-1.5 py-0">{entry.source_kind === 'built_in' ? 'built-in' : entry.source_kind}</Badge>
                                  )}
                                </div>
                                {entry && (
                                  <p className="mt-0.5 text-xs text-muted-foreground line-clamp-2">{entry.title !== skillKey ? `${entry.title} — ` : ''}{entry.description}</p>
                                )}
                              </div>
                              {!systemVersionReadOnly && (
                                <button
                                  type="button"
                                  className="mt-0.5 shrink-0 rounded p-0.5 text-muted-foreground hover:text-destructive"
                                  onClick={() => setForm((current) => ({
                                    ...current,
                                    instruction_skills: current.instruction_skills.filter((k) => k !== skillKey),
                                  }))}
                                >
                                  <Cancel01Icon className="h-3.5 w-3.5" />
                                </button>
                              )}
                            </div>
                          );
                        })}
                      </div>
                      {!systemVersionReadOnly && (
                        <Popover open={skillPickerOpen} onOpenChange={setSkillPickerOpen}>
                          <PopoverTrigger asChild>
                            <Button variant="outline" size="sm" className="h-8 gap-1.5 px-2 text-[11px]">
                              <PlusSignIcon className="h-3.5 w-3.5" />
                              Add skill
                            </Button>
                          </PopoverTrigger>
                          <PopoverContent align="start" className="w-[28rem] p-0">
                            <Command>
                              <CommandInput placeholder="Search skills..." />
                              <CommandList className="max-h-72">
                                <CommandEmpty>No more skills available.</CommandEmpty>
                                <CommandGroup heading={`${skillCatalogEntries.filter((s) => !form.instruction_skills.includes(s.key)).length} available`}>
                                  {skillCatalogEntries
                                    .filter((s) => !form.instruction_skills.includes(s.key))
                                    .map((skill) => (
                                    <CommandItem
                                      key={skill.key}
                                      value={skill.key}
                                      onSelect={() => {
                                        setForm((current) => ({
                                          ...current,
                                          instruction_skills: [...current.instruction_skills, skill.key],
                                        }));
                                        setSkillPickerOpen(false);
                                      }}
                                    >
                                      <div className="min-w-0 flex-1 space-y-0.5">
                                        <div className="flex items-center gap-2">
                                          <span className="font-mono text-xs">{skill.key}</span>
                                          <Badge variant="outline" className="text-[9px] px-1.5 py-0">{skill.source_kind === 'built_in' ? 'built-in' : skill.source_kind}</Badge>
                                        </div>
                                        <p className="text-xs text-muted-foreground">{skill.description}</p>
                                      </div>
                                    </CommandItem>
                                  ))}
                                </CommandGroup>
                              </CommandList>
                            </Command>
                          </PopoverContent>
                        </Popover>
                      )}
                    </div>
                    )}

                    {/* 01C — Compiled System Prompt (Preview) */}
                    <div className="space-y-3">
                      <Collapsible.Root open={compiledPromptOpen} onOpenChange={setCompiledPromptOpen}>
                        <Collapsible.Trigger asChild>
                          <button type="button" className="flex w-full items-center gap-2 text-left">
                            {compiledPromptOpen ? <ArrowDown01Icon className="h-3.5 w-3.5 text-muted-foreground" /> : <ArrowRight01Icon className="h-3.5 w-3.5 text-muted-foreground" />}
                            <SourceCodeIcon className="h-3.5 w-3.5 text-muted-foreground" />
                            <span className="text-xs font-medium text-muted-foreground">Compiled System Prompt</span>
                            <Badge variant="outline" className="text-[9px] px-1.5 py-0">Preview</Badge>
                          </button>
                        </Collapsible.Trigger>
                        <Collapsible.Content>
                          <div className="mt-2 rounded-xl border border-border/60 bg-card p-4">
                            <Textarea
                              value={form.system_prompt}
                              disabled
                              rows={12}
                              className="border-0 bg-transparent p-0 font-mono text-xs shadow-none focus-visible:ring-0"
                            />
                            <p className="mt-3 text-xs text-muted-foreground">
                              {systemVersionReadOnly
                                ? 'This is the full prompt sent to the model, compiled from the preamble and skills above.'
                                : 'This prompt will be recompiled from your preamble and skills when you save.'}
                            </p>
                          </div>
                        </Collapsible.Content>
                      </Collapsible.Root>
                    </div>

                    {/* 02 — Run Mode */}
                    <div className="space-y-3">
                      <SectionHeader number="02" title="Run Mode" />
                      <div className="rounded-xl border border-border/60 bg-card px-4 py-3">
                        <label className="flex items-center justify-between gap-3">
                          <div>
                            <p className="text-sm font-medium">Interactive mode</p>
                            <p className="text-xs text-muted-foreground">
                              {supportedModesForForm(form.runtime_kind).includes('interactive')
                                ? 'On \u2014 opens a live back-and-forth run. Off \u2014 runs autonomously end-to-end.'
                                : 'This runtime only supports autonomous runs.'}
                            </p>
                          </div>
                          <Switch
                            checked={form.default_invocation_mode === 'interactive'}
                            disabled={systemVersionReadOnly || !supportedModesForForm(form.runtime_kind).includes('interactive')}
                            onCheckedChange={(isInteractive) => {
                              setForm((current) => ({
                                ...current,
                                supported_modes: [isInteractive ? 'interactive' : 'autonomous'],
                                default_invocation_mode: isInteractive ? 'interactive' : 'autonomous',
                              }));
                            }}
                          />
                        </label>
                      </div>
                    </div>

                    {/* 03 — Execution */}
                    <div className="space-y-3">
                      <SectionHeader number="03" title="Execution" description="Runtime, model and inference settings" />
                      {systemVersionReadOnly && (
                        <p className="rounded-lg border border-amber-500/30 bg-amber-500/5 px-3 py-2 text-[11px] text-amber-700 dark:bg-amber-900/10 dark:text-amber-400">
                          These settings are locked to this version. Duplicate to workspace to create an editable copy with a new version.
                        </p>
                      )}
                      <div className="rounded-xl border border-border/60 bg-card p-5">
                        <div className="grid gap-5 md:grid-cols-2">
                          <div className="space-y-2">
                            <FieldLabel>Execution Engine</FieldLabel>
                            <Select
                              value={form.runtime_kind}
                              disabled={systemVersionReadOnly}
                              onValueChange={(value) =>
                                setForm((current) => {
                                  const runtimeKind = value as AgentRuntimeKind;
                                  const nextSupportedModes = supportedModesForForm(runtimeKind);
                                  const nextDefaultMode = nextSupportedModes.includes(current.default_invocation_mode)
                                    ? current.default_invocation_mode
                                    : nextSupportedModes[0];
                                  return {
                                    ...current,
                                    runtime_kind: runtimeKind,
                                    supported_modes: nextSupportedModes,
                                    provider: normalizeProviderForRuntime(runtimeKind, current.provider),
                                    ...deriveExecutionConfigFields(
                                      runtimeKind,
                                      normalizeProviderForRuntime(runtimeKind, current.provider),
                                      buildExecutionConfigPayload(current),
                                    ),
                                    default_invocation_mode: nextDefaultMode,
                                  };
                                })
                              }
                            >
                              <SelectTrigger className="h-9">
                                <SelectValue />
                              </SelectTrigger>
                              <SelectContent>
                                {RUNTIME_KIND_OPTIONS
                                  .filter((runtimeKind) => availableRuntimeKinds.includes(runtimeKind))
                                  .map((runtimeKind) => (
                                    <SelectItem key={runtimeKind} value={runtimeKind}>
                                      {AGENT_RUNTIME_LABELS[runtimeKind]}
                                    </SelectItem>
                                  ))}
                              </SelectContent>
                            </Select>
                            <p className="text-[11px] leading-relaxed text-muted-foreground">Sandbox + tool runtime used to execute the agent.</p>
                          </div>

                          <div className="space-y-2">
                            <FieldLabel>AI Provider</FieldLabel>
                            <Select
                              value={form.provider}
                              disabled={systemVersionReadOnly}
                              onValueChange={(value) =>
                                setForm((current) => {
                                  const provider = normalizeProviderForRuntime(current.runtime_kind, value as AgentModelProvider);
                                  return {
                                    ...current,
                                    provider,
                                    ...deriveExecutionConfigFields(current.runtime_kind, provider, buildExecutionConfigPayload(current)),
                                  };
                                })
                              }
                            >
                              <SelectTrigger className="h-9">
                                <SelectValue />
                              </SelectTrigger>
                              <SelectContent>
                                {visibleProviderOptions.map((provider) => (
                                  <SelectItem key={provider.value} value={provider.value}>
                                    <span className="flex items-center gap-2">
                                      <ProviderIcon provider={provider.value} className="h-3.5 w-3.5 shrink-0" />
                                      {provider.label}
                                    </span>
                                  </SelectItem>
                                ))}
                              </SelectContent>
                            </Select>
                            <p className="text-[11px] leading-relaxed text-muted-foreground">LLM vendor powering this engine.</p>
                          </div>
                        </div>

                        <Separator className="my-5" />

                        <div className="grid gap-5 md:grid-cols-2">
                          <div className="space-y-2">
                            <FieldLabel htmlFor="system-agent-model">Model</FieldLabel>
                            <Input
                              id="system-agent-model"
                              value={form.model}
                              disabled={systemVersionReadOnly}
                              onChange={(e) => setForm((current) => ({ ...current, model: e.target.value }))}
                              placeholder={selectedProviderOption?.model_placeholder ?? 'Auto'}
                              className="h-9"
                            />
                            <p className="text-[11px] leading-relaxed text-muted-foreground">The language model that powers this agent.</p>
                          </div>

                          {supportsReasoningEffort && (
                            <div className="space-y-2">
                              <FieldLabel>Reasoning Effort</FieldLabel>
                              <Select
                                value={form.reasoning_effort || '_default'}
                                disabled={systemVersionReadOnly}
                                onValueChange={(value) =>
                                  setForm((current) => ({
                                    ...current,
                                    reasoning_effort: value === '_default' ? '' : value as AgentReasoningEffort,
                                  }))
                                }
                              >
                                <SelectTrigger className="h-9 capitalize">
                                  <SelectValue />
                                </SelectTrigger>
                                <SelectContent>
                                  <SelectItem value="_default">Model default</SelectItem>
                                  {(selectedProviderOption?.supported_reasoning_efforts ?? REASONING_EFFORT_OPTIONS).map((effort) => (
                                    <SelectItem key={effort} value={effort} className="capitalize">
                                      {effort}
                                    </SelectItem>
                                  ))}
                                </SelectContent>
                              </Select>
                              <p className="text-[11px] leading-relaxed text-muted-foreground">How hard the model thinks before responding.</p>
                            </div>
                          )}

                          {supportsServiceTier && (
                            <div className="space-y-2">
                              <FieldLabel>Service Tier</FieldLabel>
                              <Select
                                value={form.service_tier || '_default'}
                                disabled={systemVersionReadOnly}
                                onValueChange={(value) =>
                                  setForm((current) => ({
                                    ...current,
                                    service_tier: value === '_default' ? '' : value as AgentServiceTier,
                                  }))
                                }
                              >
                                <SelectTrigger className="h-9 capitalize">
                                  <SelectValue />
                                </SelectTrigger>
                                <SelectContent>
                                  <SelectItem value="_default">Provider default</SelectItem>
                                  {(selectedProviderOption?.supported_service_tiers ?? SERVICE_TIER_OPTIONS).map((tier) => (
                                    <SelectItem key={tier} value={tier} className="capitalize">
                                      {tier}
                                    </SelectItem>
                                  ))}
                                </SelectContent>
                              </Select>
                              <p className="text-[11px] leading-relaxed text-muted-foreground">Fast lanes prioritize latency over cost.</p>
                            </div>
                          )}
                        </div>
                      </div>
                    </div>

                    {/* 04 — Allowed Tools */}
                    <div className="space-y-3">
                      <SectionHeader number="04" title="Allowed Tools" description="Capabilities the agent may invoke during a run" />
                      <div className="flex items-center justify-between gap-2">
                        <div className="flex items-center gap-2 text-xs text-muted-foreground">
                          <span className="font-medium">{form.allowed_tools.length} enabled</span>
                        </div>
                        <Popover open={toolPickerOpen} onOpenChange={setToolPickerOpen}>
                          <PopoverTrigger asChild>
                            <Button
                              type="button"
                              variant="outline"
                              size="sm"
                              className="h-8 gap-1.5 px-2 text-[11px]"
                              disabled={toolCatalogEntries.length === 0 || codexUsesPresetCapabilities || systemVersionReadOnly}
                            >
                              <PlusSignIcon className="h-3.5 w-3.5" />
                              Add tool
                            </Button>
                          </PopoverTrigger>
                          <PopoverContent align="end" className="w-[28rem] p-0">
                            <Command>
                              <CommandInput placeholder="Search tools..." />
                              <CommandList className="max-h-72">
                                <CommandEmpty>
                                  {toolCatalogEntries.length === 0 ? 'Tool catalog unavailable.' : 'No more tools available.'}
                                </CommandEmpty>
                                <CommandGroup heading={`${availableToolEntries.length} available`}>
                                  {availableToolEntries.map((tool) => (
                                    <CommandItem
                                      key={tool.name}
                                      value={`${tool.name} ${tool.category} ${tool.description}`}
                                      onSelect={() => addTool(tool.name)}
                                      className="cursor-pointer items-start py-2"
                                    >
                                      <div className="min-w-0 flex-1 space-y-0.5">
                                        <div className="flex items-center gap-2">
                                          <span className="font-mono text-xs text-foreground">{tool.name}</span>
                                          <Badge variant="outline" className="text-[10px]">
                                            {tool.category}
                                          </Badge>
                                        </div>
                                        <p className="text-xs leading-relaxed text-muted-foreground">{tool.description}</p>
                                      </div>
                                    </CommandItem>
                                  ))}
                                </CommandGroup>
                              </CommandList>
                            </Command>
                          </PopoverContent>
                        </Popover>
                      </div>
                      {form.allowed_tools.length > 0 ? (
                        <div className="space-y-3">
                          {(toolCatalog?.categories ?? []).map((category) => {
                            const categoryTools = form.allowed_tools.filter((toolName) => {
                              const entry = toolCatalogEntries.find((t) => t.name === toolName);
                              return entry?.category === category;
                            });
                            if (categoryTools.length === 0) return null;
                            return (
                              <div key={category}>
                                <p className="mb-1.5 text-[10px] font-semibold uppercase tracking-wider text-muted-foreground/70">{category}</p>
                                <div className="flex flex-wrap gap-1.5">
                                  {categoryTools.map((tool) => (
                                    <Badge key={tool} variant="secondary" className="gap-1 pr-1 font-mono text-[11px]">
                                      <span className="inline-block h-1.5 w-1.5 rounded-full bg-emerald-500" />
                                      <span>{tool}</span>
                                      {versionDraftOpen && !codexUsesPresetCapabilities && (
                                        <button
                                          type="button"
                                          className="rounded-sm p-0.5 text-muted-foreground transition-colors hover:bg-background hover:text-foreground"
                                          onClick={() => removeTool(tool)}
                                          aria-label={`Remove ${tool}`}
                                        >
                                          <Cancel01Icon className="h-3 w-3" />
                                        </button>
                                      )}
                                    </Badge>
                                  ))}
                                </div>
                              </div>
                            );
                          })}
                          {/* Tools without a matching category */}
                          {form.allowed_tools.filter((toolName) => {
                            const entry = toolCatalogEntries.find((t) => t.name === toolName);
                            return !entry || !(toolCatalog?.categories ?? []).includes(entry.category);
                          }).length > 0 && (
                            <div>
                              <p className="mb-1.5 text-[10px] font-semibold uppercase tracking-wider text-muted-foreground/70">Other</p>
                              <div className="flex flex-wrap gap-1.5">
                                {form.allowed_tools.filter((toolName) => {
                                  const entry = toolCatalogEntries.find((t) => t.name === toolName);
                                  return !entry || !(toolCatalog?.categories ?? []).includes(entry.category);
                                }).map((tool) => (
                                  <Badge key={tool} variant="secondary" className="gap-1 pr-1 font-mono text-[11px]">
                                    <span className="inline-block h-1.5 w-1.5 rounded-full bg-emerald-500" />
                                    <span>{tool}</span>
                                    {versionDraftOpen && !codexUsesPresetCapabilities && (
                                      <button
                                        type="button"
                                        className="rounded-sm p-0.5 text-muted-foreground transition-colors hover:bg-background hover:text-foreground"
                                        onClick={() => removeTool(tool)}
                                        aria-label={`Remove ${tool}`}
                                      >
                                        <Cancel01Icon className="h-3 w-3" />
                                      </button>
                                    )}
                                  </Badge>
                                ))}
                              </div>
                            </div>
                          )}
                        </div>
                      ) : (
                        <p className="text-sm text-muted-foreground">No tools configured</p>
                      )}
                    </div>

                  </div>

                  <div className="space-y-4">
                    <div className="rounded-2xl border border-border/60 bg-muted/20 p-5">
                      <div className="flex items-center gap-2">
                        <span className="inline-block h-2 w-2 rounded-full bg-emerald-500" />
                        <p className="text-[10px] font-semibold uppercase tracking-wider text-muted-foreground/70">Effective Configuration</p>
                      </div>

                      {/* Key-value table */}
                      <div className="mt-4 space-y-2.5 text-sm">
                        {([
                          ['Runtime', AGENT_RUNTIME_LABELS[form.runtime_kind] ?? form.runtime_kind],
                          ['Model', form.model || 'Auto'],
                          ...(supportsReasoningEffort ? [['Reasoning', form.reasoning_effort || 'Default']] : []),
                          ...(supportsServiceTier ? [['Tier', form.service_tier || 'Default']] : []),
                          ['Tools', `${form.allowed_tools.length} enabled`],
                        ] as [string, string][]).map(([label, value]) => (
                          <div key={label} className="flex items-center justify-between">
                            <span className="text-muted-foreground">{label}</span>
                            <span className="font-medium">{value}</span>
                          </div>
                        ))}
                      </div>

                      {/* Targets */}
                      <div className="mt-5">
                        <p className="text-[10px] font-semibold uppercase tracking-wider text-muted-foreground/70">Targets</p>
                        <div className="mt-2 flex flex-wrap gap-1.5">
                          {effectiveTargets.length > 0 ? effectiveTargets.map((target) => (
                            <Badge key={target} variant="outline" className="text-[11px]">
                              {target}
                            </Badge>
                          )) : (
                            <span className="text-xs text-muted-foreground">No targets</span>
                          )}
                        </div>
                      </div>

                      {/* Supported Modes */}
                      <div className="mt-4">
                        <p className="text-[10px] font-semibold uppercase tracking-wider text-muted-foreground/70">Supported Modes</p>
                        <div className="mt-2 flex flex-wrap gap-1.5">
                          {supportedModes.map((mode) => (
                            <Badge key={mode} variant="outline" className="text-[11px]">
                              {INVOCATION_MODE_LABELS[mode]}
                            </Badge>
                          ))}
                        </div>
                      </div>
                    </div>

                    {/* Monthly Usage Limit */}
                    <div className="rounded-2xl border border-border/60 bg-muted/20 p-5">
                      <p className="text-[10px] font-semibold uppercase tracking-wider text-muted-foreground/70">Monthly Usage Limit</p>

                      {/* Preset quick-select */}
                      <div className="mt-3 flex flex-wrap gap-1.5">
                        {[
                          { label: 'No Limit', value: '' },
                          { label: '$50', value: '50' },
                          { label: '$100', value: '100' },
                          { label: '$250', value: '250' },
                          { label: '$500', value: '500' },
                          { label: '$1,000', value: '1000' },
                        ].map((preset) => {
                          const isActive = form.monthly_token_budget === preset.value;
                          return (
                            <button
                              key={preset.value}
                              type="button"
                              onClick={() => setForm((current) => ({ ...current, monthly_token_budget: preset.value }))}
                              className={cn(
                                'rounded-lg border px-2.5 py-1 text-xs font-medium transition-colors',
                                isActive
                                  ? 'border-primary bg-primary/10 text-primary'
                                  : 'border-border/60 text-muted-foreground hover:border-border hover:text-foreground',
                              )}
                            >
                              {preset.label}
                            </button>
                          );
                        })}
                      </div>

                      {/* Custom input */}
                      <div className="mt-3">
                        <Input
                          id="agent-budget"
                          type="number"
                          value={form.monthly_token_budget}
                          onChange={(e) => setForm((current) => ({ ...current, monthly_token_budget: e.target.value }))}
                          placeholder="Custom amount..."
                          className="h-8 text-xs"
                        />
                      </div>

                      {/* Progress bar — usage vs limit */}
                      {editingAgent && (
                        <div className="mt-3">
                          {(() => {
                            const used = editingAgent.tokens_used_this_month ?? 0;
                            const budget = form.monthly_token_budget ? Number.parseInt(form.monthly_token_budget, 10) : 0;
                            const pct = budget > 0 ? Math.min(100, Math.round((used / budget) * 100)) : 0;
                            return (
                              <>
                                {budget > 0 && (
                                  <div className="h-1.5 w-full overflow-hidden rounded-full bg-muted">
                                    <div
                                      className={cn(
                                        'h-full rounded-full transition-all',
                                        pct >= 90 ? 'bg-destructive' : pct >= 70 ? 'bg-amber-500' : 'bg-primary',
                                      )}
                                      style={{ width: `${pct}%` }}
                                    />
                                  </div>
                                )}
                                <p className="mt-1.5 text-[11px] text-muted-foreground">
                                  {budget > 0
                                    ? `$${used.toLocaleString()} of $${budget.toLocaleString()} used this cycle (${pct}%)`
                                    : `$${used.toLocaleString()} spent this cycle. Applies to the built-in agent, not the preset version.`}
                                </p>
                              </>
                            );
                          })()}
                        </div>
                      )}
                    </div>

                    {/* Triggers */}
                    {editingAgent && (
                      <AgentTriggerPanel
                        workspaceId={workspaceId}
                        workspaceSlug={workspace?.slug}
                        usage={agentUsage}
                        loading={agentUsageLoading}
                        onRefresh={async () => {
                          await loadAgentUsage(editingAgent.id);
                        }}
                      />
                    )}
                  </div>
                </section>
              </div>
            </div>
          </div>

          <SheetFooter className="border-t border-border/60 bg-background px-6 py-4 sm:flex-row sm:justify-between">
            <div className="text-xs text-muted-foreground">
              {versionDraftOpen
                ? 'Save the workspace version first, then save the built-in agent to pin it.'
                : selectedPreset?.version_key === currentSystemVersionKey
                  ? 'This agent is already pinned to the selected version.'
                  : 'Save the built-in agent to pin it to the selected version.'}
            </div>
            <div className="flex gap-2">
              <Button variant="outline" size="sm" onClick={() => setSystemDrawerOpen(false)}>
                Close
              </Button>
              <Button
                size="sm"
                disabled={saving || versionDraftOpen}
                onClick={handleSave}
              >
                {saving ? 'Saving...' : 'Save Built-in Agent'}
              </Button>
            </div>
          </SheetFooter>
        </SheetContent>
      </Sheet>

      {/* ---- Create / Edit drawer ---- */}
      <Sheet
        open={dialogOpen}
        onOpenChange={(open) => {
          setDialogOpen(open);
          if (!open) {
            setToolPickerOpen(false);
            setSkillPickerOpen(false);
          }
        }}
      >
        <SheetContent side="right" className="w-full gap-0 p-0 data-[side=right]:w-[88vw] data-[side=right]:sm:max-w-[88vw] xl:data-[side=right]:w-[1100px] xl:data-[side=right]:max-w-[1100px]">
          <SheetHeader className="border-b border-border/60 bg-muted/20 px-6 py-5">
            <SheetTitle>{editingAgent ? 'Edit Custom Agent' : 'New Custom Agent'}</SheetTitle>
            <SheetDescription className="max-w-3xl">
              Custom agents own their prompt, runtime, tools, targets, and automation settings directly. They do not inherit from or stay pinned to any preset family.
            </SheetDescription>
          </SheetHeader>

          <div className="min-h-0 flex-1 overflow-y-auto px-6 py-6">
            <div className="space-y-4">
              <div className="rounded-md border border-border/60 bg-muted/30 px-3 py-2">
                <p className="text-sm font-medium">Custom agent</p>
                <p className="text-xs text-muted-foreground">
                  This is a fully custom agent. It does not inherit or track any preset family or preset version.
                </p>
              </div>

            {/* ---- Basics ---- */}
            <div className="space-y-2">
              <FieldLabel htmlFor="agent-name">Name</FieldLabel>
              <Input
                id="agent-name"
                value={form.name}
                onChange={(e) => setForm((current) => ({ ...current, name: e.target.value }))}
                placeholder="e.g. Code Reviewer, Sales Assistant"
              />
            </div>

            <div className="space-y-2">
              <FieldLabel
                htmlFor="agent-system-prompt"
                tooltip="Instructions stored on the agent itself. For planners, keep the planning behavior here rather than in a separate planner-only field."
              >
                System instructions
              </FieldLabel>
              <Textarea
                id="agent-system-prompt"
                value={form.system_prompt}
                onChange={(e) => setForm((current) => ({ ...current, system_prompt: e.target.value }))}
                placeholder="Agent instructions"
                rows={4}
              />
            </div>

            {/* Team selector */}
            {visibleTeams.length > 0 && (
              <div className="space-y-2">
                <FieldLabel tooltip="Assign this agent to a team so it only works on that team's tasks. Leave unassigned for workspace-wide access.">
                  Team
                </FieldLabel>
                <Select
                  value={form.team_id || '_none'}
                  onValueChange={(value) => setForm((current) => ({ ...current, team_id: value === '_none' ? '' : value }))}
                >
                  <SelectTrigger>
                    <SelectValue placeholder="All teams (workspace-wide)" />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="_none">All teams (workspace-wide)</SelectItem>
                    {visibleTeams.map((team) => (
                      <SelectItem key={team.id} value={team.id}>
                        {team.name}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>
            )}

            <div className="space-y-2">
              <FieldLabel tooltip="Choose which target types this custom agent is allowed to run against.">
                Allowed targets
              </FieldLabel>
              <div className="grid gap-2 sm:grid-cols-2">
                {CUSTOM_AGENT_TARGET_OPTIONS.map((target) => {
                  const active = form.allowed_targets.includes(target.value);
                  return (
                    <button
                      key={target.value}
                      type="button"
                      onClick={() => toggleTarget(target.value)}
                      className={`rounded-lg border px-3 py-2 text-left transition-colors ${
                        active
                          ? 'border-primary bg-primary/10 text-foreground'
                          : 'border-border bg-background hover:bg-muted/40'
                      }`}
                    >
                      <div className="flex items-center justify-between gap-2">
                        <span className="text-sm font-medium">{target.label}</span>
                        {active ? <Badge variant="secondary" className="text-[10px]">Enabled</Badge> : null}
                      </div>
                      <p className="mt-1 text-[11px] text-muted-foreground">{target.description}</p>
                    </button>
                  );
                })}
              </div>
              <p className="text-[11px] text-muted-foreground">
                Task and epic are the normal choices for planning agents. Add other targets only if the prompt and toolset are designed for them.
              </p>
            </div>

            <div className="rounded-xl border border-border/60 bg-card p-5">
              <div className="grid gap-5 sm:grid-cols-2">
                <div className="space-y-2">
                  <FieldLabel tooltip="The execution engine that runs this agent.">
                    Execution engine
                  </FieldLabel>
                  <Select
                    value={form.runtime_kind}
                    onValueChange={(value) =>
                      setForm((current) => {
                        const runtimeKind = value as AgentRuntimeKind;
                        if (!availableRuntimeKinds.includes(runtimeKind)) {
                          return current;
                        }
                        return {
                          ...current,
                          runtime_kind: runtimeKind,
                          supported_modes: supportedModesForForm(runtimeKind),
                          provider: normalizeProviderForRuntime(runtimeKind, current.provider),
                          ...deriveExecutionConfigFields(
                            runtimeKind,
                            normalizeProviderForRuntime(runtimeKind, current.provider),
                            buildExecutionConfigPayload(current),
                          ),
                          default_invocation_mode: normalizeDefaultInvocationMode(
                            current.default_invocation_mode,
                            runtimeKind,
                            'autonomous',
                          ),
                        };
                      })
                    }
                  >
                    <SelectTrigger className="h-9">
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                      {RUNTIME_KIND_OPTIONS
                        .filter((runtimeKind) => availableRuntimeKinds.includes(runtimeKind))
                        .map((runtimeKind) => (
                          <SelectItem key={runtimeKind} value={runtimeKind}>
                            {AGENT_RUNTIME_LABELS[runtimeKind]}
                          </SelectItem>
                        ))}
                    </SelectContent>
                  </Select>
                  <p className="text-[11px] leading-relaxed text-muted-foreground">Sandbox + tool runtime used to execute the agent.</p>
                </div>

                <div className="space-y-2">
                  <FieldLabel tooltip="This agent owns its default run mode. Launchers should not decide whether it is interactive or autonomous.">
                    Default run mode
                  </FieldLabel>
                  <Select
                    value={form.default_invocation_mode}
                    onValueChange={(value) =>
                      setForm((current) => ({
                        ...current,
                        default_invocation_mode: value as AgentInvocationMode,
                      }))
                    }
                  >
                    <SelectTrigger className="h-9">
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                      {supportedModes.map((mode) => (
                        <SelectItem key={mode} value={mode}>
                          {INVOCATION_MODE_LABELS[mode]}
                        </SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                  <p className="text-[11px] leading-relaxed text-muted-foreground">
                    {form.default_invocation_mode === 'interactive'
                      ? 'New runs open as a live chat and can ask follow-up questions before acting.'
                      : 'New runs execute autonomously unless you open the run drawer to inspect them.'}
                  </p>
                </div>
              </div>

              <Separator className="my-5" />

              <div className="grid gap-5 sm:grid-cols-2">
                <div className="space-y-2">
                  <FieldLabel tooltip="The AI service that powers this agent.">AI Provider</FieldLabel>
                  <Select
                    value={form.provider}
                    onValueChange={(value) =>
                      setForm((current) => {
                        const provider = normalizeProviderForRuntime(current.runtime_kind, value as AgentModelProvider);
                        return {
                          ...current,
                          provider,
                          ...deriveExecutionConfigFields(current.runtime_kind, provider, buildExecutionConfigPayload(current)),
                        };
                      })
                    }
                  >
                    <SelectTrigger className="h-9">
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                      {visibleProviderOptions.map((provider) => (
                        <SelectItem key={provider.value} value={provider.value}>
                          <span className="flex items-center gap-2">
                            <ProviderIcon provider={provider.value} className="h-3.5 w-3.5 shrink-0" />
                            {provider.label}
                          </span>
                        </SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                  <p className="text-[11px] leading-relaxed text-muted-foreground">LLM vendor powering this engine.</p>
                </div>
                <div className="space-y-2">
                  <FieldLabel
                    htmlFor="agent-model"
                    tooltip="Leave blank to use the recommended model. Only change this if you need a specific model."
                  >
                    Model
                  </FieldLabel>
                  <Input
                    id="agent-model"
                    value={form.model}
                    onChange={(e) => setForm((current) => ({ ...current, model: e.target.value }))}
                    placeholder={selectedProviderOption?.model_placeholder ?? 'Auto'}
                    className="h-9"
                  />
                  <p className="text-[11px] leading-relaxed text-muted-foreground">The language model that powers this agent.</p>
                </div>
              </div>
            </div>

            <div className="rounded-xl border border-border/60 bg-muted/30 p-5">
              <div className="space-y-1 mb-5">
                <p className="text-sm font-semibold">Effective capabilities</p>
                <p className="text-[11px] leading-relaxed text-muted-foreground">
                  These are the effective defaults produced by this agent&apos;s runtime, mode, and policy settings.
                </p>
              </div>

              <div className="grid gap-5 sm:grid-cols-3">
                <div className="space-y-1.5">
                  <p className="text-[10px] font-semibold uppercase tracking-wider text-muted-foreground/70">Runtime</p>
                  <p className="text-sm font-medium">{AGENT_RUNTIME_LABELS[form.runtime_kind] ?? form.runtime_kind}</p>
                </div>
                <div className="space-y-1.5">
                  <p className="text-[10px] font-semibold uppercase tracking-wider text-muted-foreground/70">Default mode</p>
                  <p className="text-sm font-medium">{INVOCATION_MODE_LABELS[form.default_invocation_mode]}</p>
                </div>
                <div className="space-y-1.5">
                  <p className="text-[10px] font-semibold uppercase tracking-wider text-muted-foreground/70">Targets</p>
                  <div className="flex flex-wrap gap-1.5">
                    {effectiveTargets.length > 0 ? effectiveTargets.map((target) => (
                      <Badge key={target} variant="outline" className="text-[11px]">
                        {target}
                      </Badge>
                    )) : (
                      <span className="text-sm text-muted-foreground">No targets</span>
                    )}
                  </div>
                </div>
              </div>

              <Separator className="my-5" />

              <div className="space-y-1.5">
                <p className="text-[10px] font-semibold uppercase tracking-wider text-muted-foreground/70">Supported modes</p>
                <div className="flex flex-wrap gap-1.5">
                  {supportedModes.map((mode) => (
                    <Badge key={mode} variant="outline" className="text-[11px]">
                      {INVOCATION_MODE_LABELS[mode]}
                    </Badge>
                  ))}
                </div>
              </div>

              <Separator className="my-5" />

              <div className="space-y-3">
                <div className="flex items-center justify-between gap-2">
                  <p className="text-[10px] font-semibold uppercase tracking-wider text-muted-foreground/70">Available tools</p>
                  <Popover open={toolPickerOpen} onOpenChange={setToolPickerOpen}>
                    <PopoverTrigger asChild>
                      <Button
                        type="button"
                        variant="outline"
                        size="sm"
                        className="h-8 gap-1.5 px-2 text-[11px]"
                        disabled={toolCatalogEntries.length === 0 || codexUsesPresetCapabilities}
                      >
                        <PlusSignIcon className="h-3.5 w-3.5" />
                        Add tool
                      </Button>
                    </PopoverTrigger>
                    <PopoverContent align="end" className="w-[28rem] p-0">
                      <Command>
                        <CommandInput placeholder="Search tools..." />
                        <CommandList className="max-h-72">
                          <CommandEmpty>
                            {toolCatalogEntries.length === 0 ? 'Tool catalog unavailable.' : 'No more tools available.'}
                          </CommandEmpty>
                          <CommandGroup heading={`${availableToolEntries.length} available`}>
                            {availableToolEntries.map((tool) => (
                              <CommandItem
                                key={tool.name}
                                value={`${tool.name} ${tool.category} ${tool.description}`}
                                onSelect={() => addTool(tool.name)}
                                className="cursor-pointer items-start py-2"
                              >
                                <div className="min-w-0 flex-1 space-y-0.5">
                                  <div className="flex items-center gap-2">
                                    <span className="font-mono text-xs text-foreground">{tool.name}</span>
                                    <Badge variant="outline" className="text-[10px]">
                                      {tool.category}
                                    </Badge>
                                  </div>
                                  <p className="text-xs leading-relaxed text-muted-foreground">{tool.description}</p>
                                </div>
                              </CommandItem>
                            ))}
                          </CommandGroup>
                        </CommandList>
                      </Command>
                    </PopoverContent>
                  </Popover>
                </div>
                <p className="text-[11px] leading-relaxed text-muted-foreground">
                  {codexUsesPresetCapabilities
                    ? 'Codex currently uses the preset capability set as-is. Custom tool overrides are disabled for this runtime.'
                    : 'Choose from the workspace tool catalog. Selected tools become this agent&apos;s allowed tool list.'}
                </p>
                <div className="flex max-h-40 flex-wrap gap-1.5 overflow-y-auto rounded-lg border border-border/50 bg-card/70 p-2.5">
                  {form.allowed_tools.length > 0 ? form.allowed_tools.map((tool) => (
                    <Badge key={tool} variant="secondary" className="gap-1 pr-1 font-mono text-[11px]">
                      <span>{tool}</span>
                      <button
                        type="button"
                        className="rounded-sm p-0.5 text-muted-foreground transition-colors hover:bg-background hover:text-foreground"
                        onClick={() => removeTool(tool)}
                        disabled={codexUsesPresetCapabilities}
                        aria-label={`Remove ${tool}`}
                      >
                        <Cancel01Icon className="h-3 w-3" />
                      </button>
                    </Badge>
                  )) : (
                    <span className="text-sm text-muted-foreground">No tools configured</span>
                  )}
                </div>
              </div>
            </div>

            {/* ---- Skills ---- */}
            <div className="space-y-3">
              <div className="flex items-center justify-between gap-2">
                <div>
                  <p className="text-sm font-semibold">Skills</p>
                  <p className="text-[11px] text-muted-foreground">Behavioral instruction modules attached to this agent</p>
                </div>
                <Popover open={skillPickerOpen} onOpenChange={setSkillPickerOpen}>
                  <PopoverTrigger asChild>
                    <Button
                      type="button"
                      variant="outline"
                      size="sm"
                      className="h-8 gap-1.5 px-2 text-[11px]"
                      disabled={availableSkillEntries.length === 0}
                    >
                      <PlusSignIcon className="h-3.5 w-3.5" />
                      Add skill
                    </Button>
                  </PopoverTrigger>
                  <PopoverContent align="end" className="w-[28rem] p-0">
                    <Command>
                      <CommandInput placeholder="Search skills..." />
                      <CommandList className="max-h-72">
                        <CommandEmpty>No more skills available.</CommandEmpty>
                        <CommandGroup heading={`${availableSkillEntries.length} available`}>
                          {availableSkillEntries.map((skill) => (
                            <CommandItem
                              key={skill.key}
                              value={`${skill.key} ${skill.title} ${skill.description}`}
                              onSelect={() => addSkill(skill)}
                              className="cursor-pointer items-start py-2"
                            >
                              <div className="min-w-0 flex-1 space-y-0.5">
                                <div className="flex items-center gap-2">
                                  <span className="font-mono text-xs text-foreground">{skill.key}</span>
                                  <Badge variant="outline" className="text-[10px]">
                                    {skill.source_kind === 'built_in' ? 'built-in' : skill.source_kind}
                                  </Badge>
                                </div>
                                <p className="text-xs leading-relaxed text-muted-foreground">{skill.description}</p>
                              </div>
                            </CommandItem>
                          ))}
                        </CommandGroup>
                      </CommandList>
                    </Command>
                  </PopoverContent>
                </Popover>
              </div>
              {form.skills.length > 0 ? (
                <div className="flex flex-wrap gap-1.5">
                  {form.skills.map((ref) => {
                    const entry = skillCatalogEntries.find((s) => s.key === ref.key);
                    return (
                      <Badge key={ref.key} variant="secondary" className="gap-1.5 pr-1 font-mono text-[11px]">
                        <BookOpen01Icon className="h-3 w-3 text-muted-foreground" />
                        <span>{ref.key}</span>
                        {entry?.source_kind && (
                          <span className="text-[9px] text-muted-foreground/70">{entry.source_kind === 'built_in' ? 'built-in' : entry.source_kind}</span>
                        )}
                        <button
                          type="button"
                          className="rounded-sm p-0.5 text-muted-foreground transition-colors hover:bg-background hover:text-foreground"
                          onClick={() => removeSkill(ref.key)}
                          aria-label={`Remove ${ref.key}`}
                        >
                          <Cancel01Icon className="h-3 w-3" />
                        </button>
                      </Badge>
                    );
                  })}
                </div>
              ) : (
                <p className="text-xs text-muted-foreground">No skills attached. Skills provide behavioral instructions to the agent at runtime.</p>
              )}
            </div>

            <Collapsible.Root open={automationOpen} onOpenChange={setAutomationOpen}>
              <Collapsible.Trigger asChild>
                <Button type="button" variant="ghost" className="flex w-full items-center justify-between px-2">
                  <span className="flex items-center gap-2 text-sm">
                    {automationOpen ? <ArrowDown01Icon className="h-4 w-4" /> : <ArrowRight01Icon className="h-4 w-4" />}
                    Scheduling & Approval
                  </span>
                  {form.schedule.trim() && !automationOpen && (
                    <Badge variant="outline" className="text-[11px] gap-1">
                      <Clock01Icon className="h-3 w-3" />
                      Scheduled
                    </Badge>
                  )}
                </Button>
              </Collapsible.Trigger>
              <Collapsible.Content className="space-y-4 rounded-md border bg-muted/30 p-3 mt-2">
                <div className="space-y-2">
                  <FieldLabel
                    htmlFor="agent-schedule"
                    tooltip="Use a cron expression to run this agent on a recurring schedule. For example: '0 9 * * 1-5' means weekdays at 9am UTC."
                  >
                    Recurring schedule
                  </FieldLabel>
                  <Input
                    id="agent-schedule"
                    value={form.schedule}
                    onChange={(e) => setForm((current) => ({ ...current, schedule: e.target.value }))}
                    placeholder="e.g. 0 9 * * 1-5 (weekdays at 9am)"
                  />
                  <p className="text-[11px] text-muted-foreground">
                    Leave empty if you only want to run this agent manually or via triggers.
                  </p>
                </div>

                <div className="space-y-2">
                  <FieldLabel tooltip="When set to 'always review first', a team member must approve each run before the agent starts working.">
                    Requires approval?
                  </FieldLabel>
                  <Select
                    value={form.approval_mode}
                    onValueChange={(value) => setForm((current) => ({ ...current, approval_mode: value as AgentApprovalMode }))}
                  >
                    <SelectTrigger>
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                      {APPROVAL_MODE_OPTIONS.map((opt) => (
                        <SelectItem key={opt.value} value={opt.value}>
                          {opt.label}
                        </SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                  <p className="text-[11px] text-muted-foreground">
                    {APPROVAL_MODE_OPTIONS.find((o) => o.value === form.approval_mode)?.description}
                  </p>
                </div>

                <div className="space-y-2">
                  <FieldLabel
                    htmlFor="agent-concurrency"
                    tooltip="How many tasks this agent can work on at the same time. Keep at 1 unless you need parallel processing."
                  >
                    Parallel tasks
                  </FieldLabel>
                  <Input
                    id="agent-concurrency"
                    type="number"
                    min={1}
                    max={10}
                    value={form.max_concurrent_runs}
                    onChange={(e) => setForm((current) => ({ ...current, max_concurrent_runs: e.target.value }))}
                  />
                </div>
              </Collapsible.Content>
            </Collapsible.Root>

            {editingAgent && (
              <AgentTriggerPanel
                workspaceId={workspaceId}
                workspaceSlug={workspace?.slug}
                usage={agentUsage}
                loading={agentUsageLoading}
                onRefresh={async () => {
                  await loadAgentUsage(editingAgent.id);
                }}
              />
            )}

            {/* ---- Advanced (engine internals) ---- */}
            <Collapsible.Root open={advancedOpen} onOpenChange={setAdvancedOpen}>
              <Collapsible.Trigger asChild>
                <Button type="button" variant="ghost" className="flex w-full items-center justify-between px-2">
                  <span className="flex items-center gap-2 text-sm">
                    {advancedOpen ? <ArrowDown01Icon className="h-4 w-4" /> : <ArrowRight01Icon className="h-4 w-4" />}
                    Advanced
                  </span>
                  {advancedConfigured && !advancedOpen && (
                    <Badge variant="outline" className="text-[11px]">Customised</Badge>
                  )}
                </Button>
              </Collapsible.Trigger>
              <Collapsible.Content className="space-y-4 rounded-md border bg-muted/30 p-3 mt-2">
                {supportsReasoningEffort && (
                  <div className="space-y-2">
                    <FieldLabel tooltip="Codex-only reasoning control for supported providers. Leave on model default unless you need a specific tradeoff.">
                      Reasoning effort
                    </FieldLabel>
                    <Select
                      value={form.reasoning_effort || '_default'}
                      onValueChange={(value) =>
                        setForm((current) => ({
                          ...current,
                          reasoning_effort: value === '_default' ? '' : value as AgentReasoningEffort,
                        }))
                      }
                    >
                      <SelectTrigger>
                        <SelectValue />
                      </SelectTrigger>
                      <SelectContent>
                        <SelectItem value="_default">Model default</SelectItem>
                        {(selectedProviderOption?.supported_reasoning_efforts ?? REASONING_EFFORT_OPTIONS).map((effort) => (
                          <SelectItem key={effort} value={effort}>
                            {effort}
                          </SelectItem>
                        ))}
                      </SelectContent>
                    </Select>
                  </div>
                )}
                {supportsServiceTier && (
                  <div className="space-y-2">
                    <FieldLabel tooltip="OpenAI-only Codex service tier. Fast mode trades more plan usage for lower latency.">
                      Service tier
                    </FieldLabel>
                    <Select
                      value={form.service_tier || '_default'}
                      onValueChange={(value) =>
                        setForm((current) => ({
                          ...current,
                          service_tier: value === '_default' ? '' : value as AgentServiceTier,
                        }))
                      }
                    >
                      <SelectTrigger>
                        <SelectValue />
                      </SelectTrigger>
                      <SelectContent>
                        <SelectItem value="_default">Provider default</SelectItem>
                        {(selectedProviderOption?.supported_service_tiers ?? SERVICE_TIER_OPTIONS).map((tier) => (
                          <SelectItem key={tier} value={tier}>
                            {tier}
                          </SelectItem>
                        ))}
                      </SelectContent>
                    </Select>
                  </div>
                )}
                <div className="space-y-2">
                  <FieldLabel
                    htmlFor="agent-budget"
                    tooltip="Set a monthly limit on how much this agent can process. Measured in AI tokens. Leave empty for unlimited."
                  >
                    Monthly usage limit
                  </FieldLabel>
                  <Input
                    id="agent-budget"
                    type="number"
                    value={form.monthly_token_budget}
                    onChange={(e) => setForm((current) => ({ ...current, monthly_token_budget: e.target.value }))}
                    placeholder="No limit"
                  />
                </div>
              </Collapsible.Content>
            </Collapsible.Root>
            </div>
          </div>

          {/* ---- Footer ---- */}
          <SheetFooter className="border-t border-border/60 bg-background px-6 py-4 sm:flex-row sm:justify-between">
            <div>
              {editingAgent && (
                <Button
                  variant="destructive"
                  size="sm"
                  disabled={saving}
                  onClick={() => setDeleteConfirmOpen(true)}
                >
                  Delete
                </Button>
              )}
            </div>
            <div className="flex gap-2">
              <Button variant="outline" size="sm" onClick={() => setDialogOpen(false)}>
                Cancel
              </Button>
              <Button
                size="sm"
                disabled={saving || !form.name.trim()}
                onClick={handleSave}
              >
                {saving ? 'Saving...' : editingAgent ? 'Save Changes' : 'Create Custom Agent'}
              </Button>
            </div>
          </SheetFooter>
        </SheetContent>
      </Sheet>

      <ConfirmDialog
        open={deleteConfirmOpen}
        onOpenChange={setDeleteConfirmOpen}
        title="Delete agent"
        description="This will permanently remove this agent and all its configuration. Any scheduled runs will be stopped. This action cannot be undone."
        confirmLabel="Delete"
        variant="destructive"
        onConfirm={handleDelete}
      />
    </div>
  );
}
