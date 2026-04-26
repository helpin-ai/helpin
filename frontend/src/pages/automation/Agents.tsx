import { useCallback, useEffect, useMemo, useState } from 'react';
import { useNavigate } from '@tanstack/react-router';
import { Collapsible } from 'radix-ui';
import { formatDistanceToNow } from 'date-fns';
import { toast } from 'sonner';
import { cn } from '@/lib/utils';
import {
  BotIcon,
  ArrowDown01Icon,
  ArrowRight01Icon,
  ArrowUpRight01Icon,
  ArrowExpandIcon,
  HelpCircleIcon,
  LayoutGridIcon,
  LayoutTable01Icon,
  PlusSignIcon,
  UserGroupIcon,
  Cancel01Icon,
  ZapIcon,
  WorkflowSquare01Icon,
  Loading01Icon,
  BookOpen01Icon,
  SourceCodeIcon,
  MoreHorizontalIcon,
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
import { gitService } from '@/lib/services/gitService';
import { docsService } from '@/lib/services/docsService';
import { pmWorkflowService } from '@/lib/services/pmWorkflowService';
import { AGENT_RUNTIME_LABELS } from '@/lib/agentRuntime';
import { buildAutomationActivityPath, buildAutomationFlowsPath, buildAutomationRunsPath } from '@/lib/automationUi';
import type {
  Agent,
  AgentExecutionConfig,
  AgentPresetDefinition,
  AgentPresetKey,
  AgentApprovalMode,
  AgentInvocationMode,
  AgentModelProvider,
  AgentModelProviderOption,
  AgentTemplate,
  AgentReasoningEffort,
  AgentRun,
  AgentRuntimeKind,
  AgentServiceTier,
  AgentSkillRef,
  AgentTriggerUsage,
  AgentTriggerUsageSummary,
  AgentTargetType,
  CreateAgentFromTemplateRequest,
  CreateWorkspaceAgentPresetVersionRequest,
  GitRepository,
  UpdateWorkspaceAgentPresetVersionRequest,
  CreateAgentRequest,
  SkillCatalogResponse,
  ToolCatalogEntry,
  ToolCatalogResponse,
  UpdateAgentRequest,
  WorkflowWithStates,
} from '@/lib/pmTypes';
import type { DocsCollection, DocsSpace } from '@/lib/docsTypes';
import { Button } from '@/components/ui/button';
import { Badge } from '@/components/ui/badge';
import { Card, CardContent, CardHeader } from '@/components/ui/card';
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
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';

// ---------------------------------------------------------------------------
// Constants
// ---------------------------------------------------------------------------

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
    model: 'gpt-5.5',
  },
  review_agent: {
    label: 'Review Agent',
    description: 'Reviews work, runs tests, and checks quality without repo mutation.',
    runtime_kind: 'codex',
    default_invocation_mode: 'autonomous',
    supported_modes: ['autonomous', 'interactive'],
    provider: 'openai',
    model: 'gpt-5.5',
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

const TEMPLATE_RELEASE_KIND_OPTIONS = [
  { value: 'minor', label: 'Minor releases' },
  { value: 'major', label: 'Major releases' },
  { value: 'patch', label: 'Patch releases' },
  { value: 'prerelease', label: 'Prereleases' },
  { value: 'any', label: 'Any release' },
] as const;

const NONE_OPTION_VALUE = '__none__';
const COMPETITIVE_INTEL_TEMPLATE_KEY = 'competitive_intelligence_digest';
const COMPETITIVE_INTEL_FLOW_KEY = 'competitive_intel_scheduled';
const COMPETITIVE_INTEL_SYSTEM_PROMPT_TEMPLATE = `You are a competitive intelligence agent for {{target_company}}.

Configured digest:
- target_company: {{target_company}}
- target_domain: {{target_domain}}
- competitors: {{competitors}}
- lookback_days: {{lookback_days}}
- destination_team_id: {{destination_team_id}}
- destination_state_id: {{destination_state_id}}
- schedule_preset: {{schedule_preset}}

Treat these configured values as already resolved and authoritative. Do not plan or perform discovery of configuration variables, workspace context, teams, stages, cadence, or lookback settings.

Use the configured competitor list when it is not empty. If no competitors are configured, discover competitors with web search and cite sources.

For each competitor, first use web_search_exa to find official changelog, release notes, product updates, blog, docs, or roadmap pages. Then use fetch_url on exact source URLs to verify page content and dates. If search is thin, use crawl_url on the competitor's official website or docs host with changelog/update keywords before marking no_public_changelog.

Create exactly one marketing digest task with create_task. Pass destination_team_id directly as team_id. Pass destination_state_id directly as state_id only when it is configured; otherwise let the team default stage apply.

Raw configuration:
{{raw_configuration_json}}`;
const COMPETITIVE_INTEL_SCHEDULE_OPTIONS = [
  { value: 'daily', label: 'Daily' },
  { value: 'weekly', label: 'Weekly' },
] as const;
const DEPENDENCY_AUDITOR_TEMPLATE_KEY = 'dependency_auditor';
const DEPENDENCY_AUDITOR_FLOW_KEY = 'dependency_audit_cron';
const DEPENDENCY_AUDITOR_SYSTEM_PROMPT_TEMPLATE = `You are an autonomous dependency auditor for the selected repository.

Configured audit:
- ecosystems: {{ecosystems}}
- include_indirect: {{include_indirect}}
- destination_team_id: {{destination_team_id}}
- destination_state_id: {{destination_state_id}}
- max_tasks: {{max_tasks}}
- schedule_preset: {{schedule_preset}}

Treat these configured values as already resolved and authoritative. Do not plan or perform discovery of configuration variables, workspace context, teams, stages, cadence, or repository selection.

Scan only the configured ecosystems. Ignore indirect or transitive dependencies unless include_indirect is true. Do not modify files.

Repositories may contain more than one language ecosystem. Scan every selected ecosystem in the same run and deduplicate tasks within each ecosystem identity.

Create at most one task per outdated direct dependency with create_task, up to max_tasks. Pass destination_team_id directly as team_id. Pass destination_state_id directly as state_id only when it is configured; otherwise let the team default stage apply.

Raw configuration:
{{raw_configuration_json}}`;
const DEPENDENCY_AUDITOR_ECOSYSTEM_OPTIONS = [
  { value: 'go', label: 'Go' },
  { value: 'rust', label: 'Rust' },
  { value: 'python', label: 'Python' },
  { value: 'node', label: 'Node / JS' },
  { value: 'java', label: 'Java / JVM' },
] as const;
const DEPENDENCY_AUDITOR_SCHEDULE_OPTIONS = COMPETITIVE_INTEL_SCHEDULE_OPTIONS;
type DependencyAuditorEcosystem = typeof DEPENDENCY_AUDITOR_ECOSYSTEM_OPTIONS[number]['value'];

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
  approval_mode: AgentApprovalMode;
  max_concurrent_runs: string;
  default_invocation_mode: AgentInvocationMode;
}

interface ReleaseNotesTemplateFormData {
  repository_id: string;
  release_kind: 'minor' | 'major' | 'patch' | 'prerelease' | 'any';
  include_prerelease: boolean;
  tag_pattern: string;
  space_id: string;
  collection_id: string;
}

interface CompetitiveIntelTemplateFormData {
  target_company: string;
  target_domain: string;
  competitors_text: string;
  schedule_preset: 'daily' | 'weekly';
  lookback_days: string;
  destination_team_id: string;
  destination_state_id: string;
}

interface DependencyAuditorTemplateFormData {
  repository_id: string;
  ecosystems: DependencyAuditorEcosystem[];
  include_indirect: boolean;
  schedule_preset: 'daily' | 'weekly';
  destination_team_id: string;
  destination_state_id: string;
  max_tasks: string;
}

interface TemplateDraft {
  template: AgentTemplate;
  createStarterFlow: boolean;
}

const CUSTOM_AGENT_TARGET_OPTIONS: Array<{ value: AgentTargetType; label: string; description: string }> = [
  { value: 'task', label: 'Task', description: 'Run on tasks and task planning loops.' },
  { value: 'epic', label: 'Epic', description: 'Run on epics and planning loops.' },
  { value: 'repository', label: 'Repository', description: 'Run directly against a synced repository without requiring a linked task.' },
  { value: 'workspace', label: 'Workspace', description: 'Run without a fixed entity target and gather context across the workspace.' },
  { value: 'crm_deal', label: 'CRM Deal', description: 'Run on CRM deal records.' },
  { value: 'document', label: 'Document', description: 'Run on documents and docs-backed context.' },
  { value: 'support_conversation', label: 'Support Conversation', description: 'Run on support inbox conversations.' },
];

const RUN_NOW_SUPPORTED_TARGETS = new Set<AgentTargetType>([
  'task',
  'epic',
  'repository',
  'workspace',
  'support_conversation',
]);

const RUN_NOW_TARGET_ID_LABELS: Partial<Record<AgentTargetType, string>> = {
  task: 'Task ID',
  epic: 'Epic ID',
  support_conversation: 'Conversation ID',
};

const RUN_NOW_TARGET_ID_PLACEHOLDERS: Partial<Record<AgentTargetType, string>> = {
  task: 'Paste a task ID',
  epic: 'Paste an epic ID',
  support_conversation: 'Paste a support conversation ID',
};

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
    model_placeholder: 'gpt-5.5',
    supports_reasoning_effort: true,
    supported_reasoning_efforts: REASONING_EFFORT_OPTIONS,
    supports_service_tier: true,
    supported_service_tiers: SERVICE_TIER_OPTIONS,
  },
  {
    value: 'openrouter',
    label: 'OpenRouter',
    model_placeholder: 'openai/gpt-5.5',
    supports_reasoning_effort: true,
    supported_reasoning_efforts: REASONING_EFFORT_OPTIONS,
    supports_service_tier: false,
  },
];

// Shared pill used in the fleet card + list row. An empty model resolves to
// "Auto" at runtime — render that as a neutral value (not a red-flag state).
function AgentModelPill({ provider, model }: { provider?: AgentModelProvider | null; model?: string | null }) {
  const trimmed = model?.trim();
  const effectiveProvider = provider ?? 'openai';
  return (
    <span className="inline-flex items-center gap-2 text-sm">
      <ProviderIcon provider={effectiveProvider} className="h-4 w-4 shrink-0" />
      <span className="font-mono text-xs text-foreground">
        {trimmed || <span className="text-muted-foreground">Auto</span>}
      </span>
    </span>
  );
}

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

function isAgentTargetType(value: string): value is AgentTargetType {
  return CUSTOM_AGENT_TARGET_OPTIONS.some((target) => target.value === value);
}

function labelForAgentTarget(target: AgentTargetType) {
  return CUSTOM_AGENT_TARGET_OPTIONS.find((option) => option.value === target)?.label ?? target;
}

function runNowTargetOptions(agent: Agent | null): AgentTargetType[] {
  if (!agent) return [];
  return normalizeTargetList(
    (agent.allowed_targets ?? [])
      .filter(isAgentTargetType)
      .filter((target) => RUN_NOW_SUPPORTED_TARGETS.has(target)),
  );
}

function defaultRunNowTarget(agent: Agent | null): AgentTargetType | '' {
  const targets = runNowTargetOptions(agent);
  return targets.includes('workspace') ? 'workspace' : (targets[0] ?? '');
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
    model: form.model.trim(),
    execution_config: buildExecutionConfigPayload(form),
    system_prompt: form.system_prompt.trim() || undefined,
    team_id: form.team_id,
    allowed_tools: normalizeToolList(form.allowed_tools),
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
    model: agent.model ?? preset?.model ?? '',
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
    approval_mode: 'never',
    max_concurrent_runs: agent.max_concurrent_runs?.toString() ?? '1',
    default_invocation_mode: normalizeDefaultInvocationMode(
      defaultInvocationMode,
      agent.runtime_kind || runtimeKind,
      preset?.default_invocation_mode ?? PRESET_FALLBACKS[presetKey].default_invocation_mode,
    ),
  };
}

function buildTemplateAgentForm(template: AgentTemplate): AgentFormData {
  const runtimeKind = template.runtime_kind || 'native_sdk';
  const provider = normalizeProviderForRuntime(runtimeKind, 'anthropic');
  return {
    name: template.name,
    preset_key: DEFAULT_SYSTEM_PRESET_KEY,
    preset_version_key: fallbackPresetVersionKey(DEFAULT_SYSTEM_PRESET_KEY),
    runtime_kind: runtimeKind,
    supported_modes: supportedModesForForm(runtimeKind),
    provider,
    model: '',
    ...deriveExecutionConfigFields(runtimeKind, provider, template.execution_config),
    system_prompt: template.system_prompt ?? '',
    instruction_preamble: '',
    instruction_skills: [],
    monthly_token_budget: template.monthly_token_budget?.toString() ?? '',
    team_id: '',
    allowed_targets: normalizeTargetList(template.allowed_targets ?? ['task']),
    allowed_tools: normalizeToolList(template.allowed_tools ?? []),
    skills: template.skills ?? [],
    approval_mode: template.approval_mode ?? 'never',
    max_concurrent_runs: '1',
    default_invocation_mode: normalizeDefaultInvocationMode(template.default_invocation_mode, runtimeKind, 'autonomous'),
  };
}

function competitiveIntelCompetitorsFromText(value: string) {
  return value
    .split(/\r?\n|,/)
    .map((item) => item.trim())
    .filter(Boolean);
}

function renderCompetitiveIntelSystemPrompt(templatePrompt: string | undefined, form: CompetitiveIntelTemplateFormData) {
  const competitors = competitiveIntelCompetitorsFromText(form.competitors_text);
  const targetDomain = form.target_domain.trim() || 'not configured';
  const competitorsLabel = competitors.length > 0 ? competitors.join(', ') : 'none configured; discover competitors during this run';
  const destinationState = form.destination_state_id === NONE_OPTION_VALUE || !form.destination_state_id
    ? "not configured; use the team's default stage"
    : form.destination_state_id;
  const rawConfig = {
    target_company: form.target_company.trim(),
    target_domain: form.target_domain.trim() || undefined,
    competitors,
    schedule_preset: form.schedule_preset,
    lookback_days: Number.parseInt(form.lookback_days, 10),
    destination_team_id: form.destination_team_id,
    destination_state_id: form.destination_state_id === NONE_OPTION_VALUE ? undefined : form.destination_state_id,
  };
  const replacements: Record<string, string> = {
    target_company: form.target_company.trim(),
    target_domain: targetDomain,
    competitors: competitorsLabel,
    lookback_days: form.lookback_days,
    destination_team_id: form.destination_team_id,
    destination_state_id: destinationState,
    schedule_preset: form.schedule_preset,
    raw_configuration_json: `\`\`\`json\n${JSON.stringify(rawConfig, null, 2)}\n\`\`\``,
  };
  let rendered = (templatePrompt?.trim() || COMPETITIVE_INTEL_SYSTEM_PROMPT_TEMPLATE).trim();
  for (const [key, value] of Object.entries(replacements)) {
    rendered = rendered.split(`{{${key}}}`).join(value);
  }
  return rendered;
}

function renderDependencyAuditorSystemPrompt(templatePrompt: string | undefined, form: DependencyAuditorTemplateFormData) {
  const destinationState = form.destination_state_id === NONE_OPTION_VALUE || !form.destination_state_id
    ? "not configured; use the team's default stage"
    : form.destination_state_id;
  const maxTasks = Number.parseInt(form.max_tasks, 10);
  const rawConfig = {
    ecosystems: form.ecosystems,
    include_indirect: form.include_indirect,
    schedule_preset: form.schedule_preset,
    destination_team_id: form.destination_team_id,
    destination_state_id: form.destination_state_id === NONE_OPTION_VALUE ? undefined : form.destination_state_id,
    max_tasks: maxTasks,
  };
  const replacements: Record<string, string> = {
    ecosystems: form.ecosystems.join(', '),
    include_indirect: String(form.include_indirect),
    destination_team_id: form.destination_team_id,
    destination_state_id: destinationState,
    schedule_preset: form.schedule_preset,
    max_tasks: form.max_tasks,
    raw_configuration_json: `\`\`\`json\n${JSON.stringify(rawConfig, null, 2)}\n\`\`\``,
  };
  let rendered = (templatePrompt?.trim() || DEPENDENCY_AUDITOR_SYSTEM_PROMPT_TEMPLATE).trim();
  for (const [key, value] of Object.entries(replacements)) {
    rendered = rendered.split(`{{${key}}}`).join(value);
  }
  return rendered;
}

function templateTargetLabel(target: string) {
  switch (target) {
    case 'repository':
      return 'Repository';
    case 'task':
      return 'Task';
    case 'epic':
      return 'Epic';
    case 'workspace':
      return 'Workspace';
    default:
      return target.replace(/_/g, ' ');
  }
}

function templateTriggerLabel(triggerType: string) {
  switch (triggerType) {
    case 'github.release_published':
      return 'GitHub release';
    case 'cron':
      return 'Schedule';
    default:
      return triggerType.replace(/_/g, ' ');
  }
}

// ---------------------------------------------------------------------------
// Inline helper: label + optional tooltip
// ---------------------------------------------------------------------------

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

function DrawerConfigSection({
  title,
  description,
  children,
  defaultOpen = true,
}: {
  title: string;
  description?: string;
  children: React.ReactNode;
  defaultOpen?: boolean;
}) {
  return (
    <Collapsible.Root defaultOpen={defaultOpen} className="rounded-xl border border-border/60 bg-card">
      <Collapsible.Trigger asChild>
        <button type="button" className="group flex w-full items-center gap-3 px-4 py-3 text-left">
          <ArrowRight01Icon className="h-3.5 w-3.5 text-muted-foreground transition-transform group-data-[state=open]:rotate-90" />
          <span className="flex-1 text-sm font-medium">{title}</span>
          {description && (
            <span className="hidden max-w-[20rem] truncate text-xs text-muted-foreground group-data-[state=open]:hidden sm:block">
              {description}
            </span>
          )}
        </button>
      </Collapsible.Trigger>
      <Collapsible.Content>
        <div className="space-y-4 border-t border-border/60 p-4">
          {description && <p className="text-xs leading-relaxed text-muted-foreground sm:hidden">{description}</p>}
          {children}
        </div>
      </Collapsible.Content>
    </Collapsible.Root>
  );
}

function ToolMultiSelectPopover({
  open,
  onOpenChange,
  tools,
  selectedTools,
  disabled,
  onToggleTool,
  onClearTools,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  tools: ToolCatalogEntry[];
  selectedTools: string[];
  disabled?: boolean;
  onToggleTool: (toolName: string) => void;
  onClearTools: () => void;
}) {
  const selectedSet = new Set(selectedTools);

  return (
    <Popover open={open} onOpenChange={onOpenChange}>
      <PopoverTrigger asChild>
        <Button
          type="button"
          variant="outline"
          size="sm"
          className="h-8 gap-1.5 px-2 text-[11px]"
          disabled={tools.length === 0 || disabled}
        >
          <PlusSignIcon className="h-3.5 w-3.5" />
          Select tools
          {selectedTools.length > 0 && (
            <span className="rounded-full bg-muted px-1.5 py-0 text-[10px] text-muted-foreground">
              {selectedTools.length}
            </span>
          )}
        </Button>
      </PopoverTrigger>
      <PopoverContent
        align="end"
        className="w-[28rem] overflow-hidden p-0"
        onWheelCapture={(event) => event.stopPropagation()}
      >
        <Command>
          <CommandInput placeholder="Search tools..." />
          <CommandList className="max-h-72 overscroll-contain">
            <CommandEmpty>
              {tools.length === 0 ? 'Tool catalog unavailable.' : 'No tools match.'}
            </CommandEmpty>
            <CommandGroup heading={`${selectedTools.length} selected`}>
              {tools.map((tool) => {
                const selected = selectedSet.has(tool.name);
                return (
                  <CommandItem
                    key={tool.name}
                    value={`${tool.name} ${tool.category} ${tool.description}`}
                    onSelect={() => onToggleTool(tool.name)}
                    data-checked={selected ? 'true' : undefined}
                    aria-label={`${selected ? 'Remove' : 'Add'} ${tool.name}`}
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
                );
              })}
            </CommandGroup>
          </CommandList>
        </Command>
        <div className="flex items-center justify-between border-t border-border/60 px-2 py-2">
          <Button
            type="button"
            variant="ghost"
            size="sm"
            className="h-7 px-2 text-[11px] text-muted-foreground"
            onClick={onClearTools}
            disabled={selectedTools.length === 0}
          >
            Clear
          </Button>
          <Button
            type="button"
            variant="outline"
            size="sm"
            className="h-7 px-2 text-[11px]"
            onClick={() => onOpenChange(false)}
          >
            Done
          </Button>
        </div>
      </PopoverContent>
    </Popover>
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
        Active triggers that can start this agent.
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
  recentRuns: number;
  recentCompleted: number;
  recentFailed: number;
  lastRun?: AgentRun;
  lastFiveStatuses: AgentRun['status'][];
}

function trimSummaryText(value?: string, fallback = 'No description yet.') {
  const normalized = value?.replace(/\s+/g, ' ').trim();
  if (!normalized) return fallback;
  const sentence = normalized.split(/(?<=[.!?])\s+/)[0] ?? normalized;
  return sentence.length > 160 ? `${sentence.slice(0, 157)}...` : sentence;
}

function agentRoleLabel(agent: Agent, presets: AgentPresetDefinition[]) {
  if (agent.is_system) {
    return presetLabel(fallbackPresetKey(agent), presets);
  }
  if (agent.source_template_key === 'release_notes_writer') {
    return 'Release Notes Writer';
  }
  const role = agent.role?.trim();
  return role || 'Custom agent';
}

function agentPurpose(agent: Agent, presets: AgentPresetDefinition[]) {
  if (agent.is_system) {
    const preset = presetMetaForSelection(
      fallbackPresetKey(agent),
      agent.preset_version_key,
      presets,
    );
    return trimSummaryText(
      preset?.description ?? PRESET_FALLBACKS[fallbackPresetKey(agent)].description,
      'Built-in workspace agent.',
    );
  }
  return trimSummaryText(
    agent.planning_notes || agent.system_prompt || agent.role,
    'Custom agent for workspace-specific execution.',
  );
}

// An empty model resolves to "Auto" (the runtime picks a default). That is a
// valid configured state — do not flag it amber. Only custom agents that were
// created but never saved with a model + provider would be truly unconfigured,
// which the creation flow prevents today, so this is effectively always false.
function needsModelConfiguration(_agent: Agent) {
  return false;
}

function isUnusedAgent(stats?: AgentRunStats) {
  return (stats?.recentRuns ?? 0) === 0;
}

function isFailingAgent(stats?: AgentRunStats) {
  return Boolean(stats?.recentFailed) || stats?.lastRun?.status === 'failed';
}

function needsAttention(agent: Agent, stats?: AgentRunStats) {
  return needsModelConfiguration(agent) || isUnusedAgent(stats) || isFailingAgent(stats);
}

function formatLastRunTime(run?: AgentRun) {
  if (!run) return 'Never';
  const date = run.completed_at || run.started_at || run.created_at;
  return formatDistanceToNow(new Date(date), { addSuffix: true });
}

function lastRunStatusLabel(run?: AgentRun) {
  if (!run) return 'Never run';
  switch (run.status) {
    case 'completed':
      return 'Completed';
    case 'failed':
      return 'Failed';
    case 'paused':
      return 'Paused';
    case 'running':
      return 'Running';
    case 'queued':
      return 'Queued';
    case 'cancelled':
      return 'Cancelled';
    default:
      return run.status;
  }
}

function lastRunStatusClass(run?: AgentRun) {
  if (!run) return 'border-amber-500/40 bg-amber-500/10 text-amber-700 dark:text-amber-400';
  switch (run.status) {
    case 'completed':
      return 'border-emerald-500/30 bg-emerald-500/10 text-emerald-700 dark:text-emerald-400';
    case 'failed':
      return 'border-rose-500/30 bg-rose-500/10 text-rose-700 dark:text-rose-400';
    case 'paused':
      return 'border-amber-500/30 bg-amber-500/10 text-amber-700 dark:text-amber-400';
    case 'running':
      return 'border-sky-500/30 bg-sky-500/10 text-sky-700 dark:text-sky-400';
    default:
      return 'border-border/70 bg-muted/40 text-muted-foreground';
  }
}

function attentionDotClass(agent: Agent, stats?: AgentRunStats) {
  if (needsModelConfiguration(agent)) return 'bg-amber-500';
  if (isFailingAgent(stats)) return 'bg-rose-500';
  if (isUnusedAgent(stats)) return 'bg-amber-500';
  return 'bg-emerald-500';
}

function FlowRefs({
  usage,
  workspaceSlug,
}: {
  usage?: AgentTriggerUsageSummary | null;
  workspaceSlug?: string;
}) {
  const items = usage?.items ?? [];
  if (items.length === 0) {
    return <span className="text-xs text-muted-foreground">Not used by a flow yet</span>;
  }

  const [first, ...rest] = items;
  const href = resolveManagePath(first.manage_path, workspaceSlug) ?? buildAutomationFlowsPath(workspaceSlug);

  return (
    <div className="min-w-0 text-xs text-muted-foreground">
      <a
        href={href}
        className="inline-flex max-w-full items-center gap-1 truncate text-foreground underline decoration-border underline-offset-4 hover:text-primary"
        onClick={(event) => event.stopPropagation()}
      >
        <span className="truncate">{first.title}</span>
        <ArrowUpRight01Icon className="h-3 w-3 shrink-0" />
      </a>
      {rest.length > 0 ? <span className="ml-1 text-muted-foreground">+{rest.length}</span> : null}
    </div>
  );
}

function RunBars5({ statuses }: { statuses: AgentRun['status'][] }) {
  const values = statuses.length > 0 ? statuses : ['queued', 'queued', 'queued', 'queued', 'queued'];
  return (
    <div className="flex items-center gap-1">
      {values.slice(0, 5).map((status, index) => (
        <span
          key={`${status}-${index}`}
          className={cn(
            'h-3 w-1.5 rounded-sm',
            status === 'completed'
              ? 'bg-emerald-500'
              : status === 'failed'
                ? 'bg-rose-500'
                : status === 'paused'
                  ? 'bg-amber-500'
                  : 'bg-border',
          )}
        />
      ))}
    </div>
  );
}

function AgentCard({
  agent,
  stats,
  usage,
  workspaceSlug,
  presets,
  onOpen,
  onRunNow,
  canEdit,
}: {
  agent: Agent;
  stats?: AgentRunStats;
  usage?: AgentTriggerUsageSummary | null;
  workspaceSlug?: string;
  presets: AgentPresetDefinition[];
  onOpen: (agent: Agent) => void;
  onRunNow: (agent: Agent) => void;
  canEdit: boolean;
}) {
  const role = agentRoleLabel(agent, presets);
  const purpose = agentPurpose(agent, presets);
  const attention = needsAttention(agent, stats);

  return (
    <Card
      className={cn(
        'group cursor-pointer border-border/70 transition-shadow hover:shadow-md',
        attention && 'border-amber-500/30 shadow-amber-500/5',
      )}
      onClick={() => onOpen(agent)}
    >
      <CardHeader className="space-y-3 pb-3">
        <div className="flex items-start gap-3">
          <AgentAvatar agent={agent} className="h-10 w-10 rounded-none border-0 bg-transparent shadow-none" genericBare />
          <div className="min-w-0 flex-1 space-y-1">
            <div className="flex flex-wrap items-center gap-2">
              <span className={cn('h-2 w-2 rounded-full', attentionDotClass(agent, stats))} />
              <h3 className="truncate text-sm font-semibold">{agent.name}</h3>
              {agent.is_system ? <Badge variant="outline" className="text-[10px]">System</Badge> : null}
              {agent.source_template_key ? <Badge variant="secondary" className="text-[10px]">Template</Badge> : null}
            </div>
            <p className="text-xs text-muted-foreground">{role}</p>
            <p className="line-clamp-2 text-sm text-muted-foreground">{purpose}</p>
          </div>
        </div>
      </CardHeader>
      <CardContent className="space-y-4 pt-0">
        <div className="grid gap-3 sm:grid-cols-2">
          <div className="space-y-1">
            <p className="text-[10px] font-medium uppercase tracking-[0.14em] text-muted-foreground">Model</p>
            <AgentModelPill provider={agent.provider} model={agent.model} />
          </div>
          <div className="space-y-1">
            <p className="text-[10px] font-medium uppercase tracking-[0.14em] text-muted-foreground">Mode</p>
            <p className="text-sm text-muted-foreground">{INVOCATION_MODE_LABELS[agent.default_invocation_mode]}</p>
          </div>
        </div>

        <div className="space-y-2 rounded-xl border border-border/60 bg-muted/20 p-3">
          <div className="flex items-center justify-between gap-3">
            <div className="flex items-center gap-2">
              <RunBars5 statuses={stats?.lastFiveStatuses ?? []} />
              <span className="text-sm">
                <span className="font-mono text-foreground">{stats?.recentRuns ?? 0}</span>
                <span className="text-muted-foreground"> runs · 7d</span>
              </span>
            </div>
            {stats?.lastRun ? (
              <Badge variant="outline" className={cn('text-[11px]', lastRunStatusClass(stats.lastRun))}>
                {lastRunStatusLabel(stats.lastRun)}
              </Badge>
            ) : null}
          </div>
          {stats?.lastRun ? (
            <p className="font-mono text-[11px] text-muted-foreground">{formatLastRunTime(stats.lastRun)}</p>
          ) : (
            <p className="text-[11px] text-muted-foreground">No runs in the last 7 days.</p>
          )}
        </div>

        <div className="space-y-1">
          <p className="text-[10px] font-medium uppercase tracking-[0.14em] text-muted-foreground">Used By</p>
          <FlowRefs usage={usage} workspaceSlug={workspaceSlug} />
        </div>

        {canEdit ? (
          <div className="flex justify-end gap-2">
            {!agent.is_system ? (
              <Button
                type="button"
                variant="outline"
                size="sm"
                className="h-8 px-2.5 text-xs"
                onClick={(event) => {
                  event.stopPropagation();
                  onRunNow(agent);
                }}
              >
                <ZapIcon className="mr-1.5 h-3.5 w-3.5" />
                Run now
              </Button>
            ) : null}
            <Button
              type="button"
              variant="ghost"
              size="sm"
              className="h-8 px-2.5 text-xs"
              onClick={(event) => {
                event.stopPropagation();
                onOpen(agent);
              }}
            >
              Open
            </Button>
          </div>
        ) : null}
      </CardContent>
    </Card>
  );
}

// ---------------------------------------------------------------------------
// AgentRow (list view)
// ---------------------------------------------------------------------------

function AgentRow({
  agent,
  stats,
  usage,
  workspaceSlug,
  presets,
  onOpen,
  onRunNow,
  canEdit,
}: {
  agent: Agent;
  stats?: AgentRunStats;
  usage?: AgentTriggerUsageSummary | null;
  workspaceSlug?: string;
  presets: AgentPresetDefinition[];
  onOpen: (agent: Agent) => void;
  onRunNow: (agent: Agent) => void;
  canEdit: boolean;
}) {
  const role = agentRoleLabel(agent, presets);
  const purpose = agentPurpose(agent, presets);
  const attention = needsAttention(agent, stats);

  return (
    <div
      className={cn(
        'grid cursor-pointer items-center gap-4 border-b border-border/60 px-4 py-3 transition-colors last:border-b-0 hover:bg-muted/30 lg:grid-cols-[minmax(0,3.2fr)_minmax(170px,0.95fr)_110px_120px_150px_170px_112px]',
        attention && 'bg-amber-500/[0.03]',
      )}
      onClick={() => onOpen(agent)}
      title={purpose}
    >
      <div className="min-w-0">
        <div className="flex items-center gap-3">
          <AgentAvatar agent={agent} className="h-8 w-8 rounded-none border-0 bg-transparent shadow-none" genericBare />
          <div className="min-w-0 flex-1">
            <div className="flex flex-wrap items-center gap-2">
              <span className={cn('h-2 w-2 rounded-full', attentionDotClass(agent, stats))} />
              <span className="truncate text-sm font-medium">{agent.name}</span>
              {agent.is_system ? <Badge variant="outline" className="text-[10px]">System</Badge> : null}
              {agent.source_template_key ? <Badge variant="secondary" className="text-[10px]">Template</Badge> : null}
            </div>
            <p className="mt-0.5 truncate text-xs text-muted-foreground">{role}</p>
          </div>
        </div>
      </div>

      <div className="space-y-1">
        <p className="text-[10px] font-medium uppercase tracking-[0.14em] text-muted-foreground lg:hidden">Model</p>
        <AgentModelPill provider={agent.provider} model={agent.model} />
      </div>

      <div className="space-y-1">
        <p className="text-[10px] font-medium uppercase tracking-[0.14em] text-muted-foreground lg:hidden">Mode</p>
        <span className="text-sm text-muted-foreground">{INVOCATION_MODE_LABELS[agent.default_invocation_mode]}</span>
      </div>

      <div className="space-y-1">
        <p className="text-[10px] font-medium uppercase tracking-[0.14em] text-muted-foreground lg:hidden">Runs · 7d</p>
        <div className="flex items-center gap-2">
          <RunBars5 statuses={stats?.lastFiveStatuses ?? []} />
          <span className="font-mono text-sm">{stats?.recentRuns ?? 0}</span>
        </div>
      </div>

      <div className="space-y-1">
        <p className="text-[10px] font-medium uppercase tracking-[0.14em] text-muted-foreground lg:hidden">Last Run</p>
        {stats?.lastRun ? (
          <>
            <Badge variant="outline" className={cn('text-[11px]', lastRunStatusClass(stats.lastRun))}>
              {lastRunStatusLabel(stats.lastRun)}
            </Badge>
            <p className="font-mono text-[11px] text-muted-foreground">{formatLastRunTime(stats.lastRun)}</p>
          </>
        ) : (
          <p className="text-xs text-muted-foreground">Never</p>
        )}
      </div>

      <div className="space-y-1">
        <p className="text-[10px] font-medium uppercase tracking-[0.14em] text-muted-foreground lg:hidden">Used By</p>
        <FlowRefs usage={usage} workspaceSlug={workspaceSlug} />
      </div>

      <div className="flex items-start justify-end text-muted-foreground">
        {!agent.is_system && canEdit ? (
          <Button
            type="button"
            variant="outline"
            size="sm"
            className="h-8 px-2.5 text-xs"
            onClick={(event) => {
              event.stopPropagation();
              onRunNow(agent);
            }}
          >
            <ZapIcon className="mr-1.5 h-3.5 w-3.5" />
            Run now
          </Button>
        ) : (
          <ArrowRight01Icon className="mt-0.5 h-4 w-4" />
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
  const navigate = useNavigate();
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
  const accessibleTeamIds = useMemo(
    () => new Set(accessibleTeams.map((team) => team.id)),
    [accessibleTeams],
  );
  const visibleTeams = isAdmin ? teams : accessibleTeams;

  const [viewMode, setViewMode] = useState<'list' | 'cards'>('list');
  const [runStats, setRunStats] = useState<Record<string, AgentRunStats>>({});
  const [agentUsageMap, setAgentUsageMap] = useState<Record<string, AgentTriggerUsageSummary | null>>({});
  const [agentTemplates, setAgentTemplates] = useState<AgentTemplate[]>([]);
  const [templateDialogOpen, setTemplateDialogOpen] = useState(false);
  const [templateSetupDialogOpen, setTemplateSetupDialogOpen] = useState(false);
  const [templateDraft, setTemplateDraft] = useState<TemplateDraft | null>(null);
  const [templateResourcesLoading, setTemplateResourcesLoading] = useState(false);
  const [repositories, setRepositories] = useState<GitRepository[]>([]);
  const [docsSpaces, setDocsSpaces] = useState<DocsSpace[]>([]);
  const [docsCollections, setDocsCollections] = useState<DocsCollection[]>([]);
  const [competitiveTeamWorkflow, setCompetitiveTeamWorkflow] = useState<WorkflowWithStates | null>(null);
  const [templateForm, setTemplateForm] = useState<ReleaseNotesTemplateFormData>({
    repository_id: '',
    release_kind: 'minor',
    include_prerelease: false,
    tag_pattern: '',
    space_id: '',
    collection_id: NONE_OPTION_VALUE,
  });
  const [competitiveTemplateForm, setCompetitiveTemplateForm] = useState<CompetitiveIntelTemplateFormData>({
    target_company: '',
    target_domain: '',
    competitors_text: '',
    schedule_preset: 'weekly',
    lookback_days: '7',
    destination_team_id: '',
    destination_state_id: NONE_OPTION_VALUE,
  });
  const [dependencyAuditorTemplateForm, setDependencyAuditorTemplateForm] = useState<DependencyAuditorTemplateFormData>({
    repository_id: '',
    ecosystems: ['go', 'rust', 'python', 'node', 'java'],
    include_indirect: false,
    schedule_preset: 'weekly',
    destination_team_id: '',
    destination_state_id: NONE_OPTION_VALUE,
    max_tasks: '20',
  });

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
  const [systemPromptEditorOpen, setSystemPromptEditorOpen] = useState(false);
  const [saving, setSaving] = useState(false);
  const [deleteConfirmOpen, setDeleteConfirmOpen] = useState(false);
  const [versionDraftOpen, setVersionDraftOpen] = useState(false);
  const [versionLabelDraft, setVersionLabelDraft] = useState('');
  const [versionDescriptionDraft, setVersionDescriptionDraft] = useState('');
  const [creatingVersion, setCreatingVersion] = useState(false);
  const [workspaceVersionPendingDelete, setWorkspaceVersionPendingDelete] = useState<AgentPresetDefinition | null>(null);
  const [deletingVersion, setDeletingVersion] = useState(false);
  const [workspaceVersionBeingRenamed, setWorkspaceVersionBeingRenamed] = useState<AgentPresetDefinition | null>(null);
  const [renameLabelDraft, setRenameLabelDraft] = useState('');
  const [renameDescriptionDraft, setRenameDescriptionDraft] = useState('');
  const [renamingVersion, setRenamingVersion] = useState(false);
  const [runNowOpen, setRunNowOpen] = useState(false);
  const [runNowAgent, setRunNowAgent] = useState<Agent | null>(null);
  const [runNowTargetType, setRunNowTargetType] = useState<AgentTargetType | ''>('');
  const [runNowTargetId, setRunNowTargetId] = useState('');
  const [runNowAdditionalContext, setRunNowAdditionalContext] = useState('');
  const [runNowBaseBranch, setRunNowBaseBranch] = useState('');
  const [runNowRepositoriesLoading, setRunNowRepositoriesLoading] = useState(false);
  const [runNowSubmitting, setRunNowSubmitting] = useState(false);

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

  const loadAgentTemplates = useCallback(async () => {
    if (!workspaceId) return;
    const res = await automationService.listAgentTemplates(workspaceId);
    if (!res.error) {
      setAgentTemplates(res.data ?? []);
    }
  }, [workspaceId]);

  const loadTemplateResources = useCallback(async () => {
    if (!workspaceId) return;
    setTemplateResourcesLoading(true);
    const [reposRes, spacesRes] = await Promise.all([
      gitService.listRepositories(workspaceId),
      docsService.listSpaces(workspaceId),
    ]);
    if (reposRes.error) {
      toast.error('Failed to load repositories', { description: reposRes.error });
    } else {
      setRepositories(reposRes.data ?? []);
    }
    if (spacesRes.error) {
      toast.error('Failed to load docs spaces', { description: spacesRes.error });
    } else {
      setDocsSpaces(spacesRes.data ?? []);
    }
    setTemplateResourcesLoading(false);
  }, [workspaceId]);

  const loadRepositoriesForRunNow = useCallback(async () => {
    if (!workspaceId) return;
    setRunNowRepositoriesLoading(true);
    const res = await gitService.listRepositories(workspaceId);
    if (res.error) {
      toast.error('Failed to load repositories', { description: res.error });
    } else {
      const repos = res.data ?? [];
      setRepositories(repos);
      const firstRunnableRepo = repos.find((repo) => repo.selected && repo.active && !repo.archived);
      if (firstRunnableRepo) {
        setRunNowTargetId((current) => current || firstRunnableRepo.id);
        setRunNowBaseBranch((current) => current || firstRunnableRepo.default_branch);
      }
    }
    setRunNowRepositoriesLoading(false);
  }, [workspaceId]);

  const loadCollectionsForSpace = useCallback(async (spaceId: string) => {
    if (!workspaceId || !spaceId) {
      setDocsCollections([]);
      return;
    }
    const res = await docsService.listCollections(workspaceId, spaceId);
    if (res.error) {
      toast.error('Failed to load collections', { description: res.error });
      setDocsCollections([]);
      return;
    }
    setDocsCollections(res.data ?? []);
  }, [workspaceId]);

  const loadCompetitiveTeamWorkflow = useCallback(async (teamId: string) => {
    if (!workspaceId || !teamId) {
      setCompetitiveTeamWorkflow(null);
      return;
    }
    const res = await pmWorkflowService.resolveTeamWorkflow(workspaceId, teamId);
    if (res.error) {
      toast.error('Failed to load team stages', { description: res.error });
      setCompetitiveTeamWorkflow(null);
      return;
    }
    setCompetitiveTeamWorkflow(res.data ?? null);
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

  const openRunNowDialog = useCallback((agent: Agent) => {
    const defaultTarget = defaultRunNowTarget(agent);
    const firstRunnableRepo = repositories.find((repo) => repo.selected && repo.active && !repo.archived);
    setRunNowAgent(agent);
    setRunNowTargetType(defaultTarget);
    setRunNowTargetId(
      defaultTarget === 'workspace'
        ? (workspaceId ?? '')
        : defaultTarget === 'repository'
          ? (firstRunnableRepo?.id ?? '')
          : '',
    );
    setRunNowAdditionalContext('');
    setRunNowBaseBranch(defaultTarget === 'repository' ? (firstRunnableRepo?.default_branch ?? '') : '');
    setRunNowOpen(true);
    if (defaultTarget === 'repository' && !firstRunnableRepo) {
      void loadRepositoriesForRunNow();
    }
  }, [loadRepositoriesForRunNow, repositories, workspaceId]);

  const handleRunNowTargetChange = useCallback((value: string) => {
    if (!isAgentTargetType(value)) return;
    const firstRunnableRepo = repositories.find((repo) => repo.selected && repo.active && !repo.archived);
    setRunNowTargetType(value);
    setRunNowTargetId(
      value === 'workspace'
        ? (workspaceId ?? '')
        : value === 'repository'
          ? (firstRunnableRepo?.id ?? '')
          : '',
    );
    setRunNowBaseBranch(value === 'repository' ? (firstRunnableRepo?.default_branch ?? '') : '');
    if (value === 'repository' && !firstRunnableRepo) {
      void loadRepositoriesForRunNow();
    }
  }, [loadRepositoriesForRunNow, repositories, workspaceId]);

  const handleRunNow = useCallback(async () => {
    if (!workspaceId || !runNowAgent || !runNowTargetType) return;
    const targetId = runNowTargetType === 'workspace' ? workspaceId : runNowTargetId.trim();
    if (!targetId) {
      toast.error(`Choose a ${labelForAgentTarget(runNowTargetType).toLowerCase()} target`);
      return;
    }

    setRunNowSubmitting(true);
    const res = await agentService.startRun(workspaceId, {
      agent_id: runNowAgent.id,
      target_type: runNowTargetType,
      target_id: targetId,
      additional_context: runNowAdditionalContext.trim() || undefined,
      base_branch: runNowBaseBranch.trim() || undefined,
    });
    setRunNowSubmitting(false);

    if (res.error || !res.data) {
      toast.error('Failed to start agent run', { description: res.error ?? 'No run was returned.' });
      return;
    }

    toast.success('Agent run started');
    setRunNowOpen(false);
    setRunNowAgent(null);
    await navigate({
      to: buildAutomationRunsPath(workspace?.slug, { run_id: res.data.id }),
    });
  }, [
    navigate,
    runNowAdditionalContext,
    runNowAgent,
    runNowBaseBranch,
    runNowTargetId,
    runNowTargetType,
    workspace?.slug,
    workspaceId,
  ]);

  useEffect(() => {
    loadAgents();
    loadProviderOptions();
    loadPresets();
    loadToolCatalog();
    loadSkillCatalog();
    loadAgentTemplates();
  }, [loadAgents, loadProviderOptions, loadPresets, loadToolCatalog, loadSkillCatalog, loadAgentTemplates]);

  // Fetch fleet-level run stats and trigger usage once, then derive agent rows from that shared data.
  useEffect(() => {
    if (!workspaceId || agents.length === 0) {
      setRunStats({});
      setAgentUsageMap({});
      return;
    }

    const fetchFleetData = async () => {
      const sevenDaysAgo = Date.now() - 7 * 24 * 60 * 60 * 1000;
      const results: Record<string, AgentRunStats> = {};
      for (const agent of agents) {
        results[agent.id] = {
          recentRuns: 0,
          recentCompleted: 0,
          recentFailed: 0,
          lastRun: undefined,
          lastFiveStatuses: [],
        };
      }

      const [runsRes, usageEntries] = await Promise.all([
        automationService.listWorkspaceRuns(workspaceId, 1, 500),
        Promise.all(
          agents.map(async (agent) => {
            const res = await automationService.getAgentUsage(workspaceId, agent.id);
            return [agent.id, res.error ? null : (res.data ?? null)] as const;
          }),
        ),
      ]);

      const runs = runsRes.error ? [] : (runsRes.data?.data ?? []);
      for (const run of runs) {
        const stats = results[run.agent_id];
        if (!stats) continue;

        if (!stats.lastRun) {
          stats.lastRun = run;
        }

        if (stats.lastFiveStatuses.length < 5) {
          stats.lastFiveStatuses.push(run.status);
        }

        const createdAt = new Date(run.created_at).getTime();
        if (Number.isNaN(createdAt) || createdAt < sevenDaysAgo) continue;

        stats.recentRuns += 1;
        if (run.status === 'completed') stats.recentCompleted += 1;
        if (run.status === 'failed') stats.recentFailed += 1;
      }

      setRunStats(results);

      setAgentUsageMap(Object.fromEntries(usageEntries));
    };

    void fetchFleetData();
  }, [workspaceId, agents]);

  const openCreateDialog = () => {
    setEditingAgent(null);
    setTemplateDraft(null);
    setAgentUsage(null);
    setAdvancedOpen(false);
    setAutomationOpen(false);
    setToolPickerOpen(false);
    setSystemDrawerOpen(false);
    setVersionDraftOpen(false);
    setVersionLabelDraft('');
    setVersionDescriptionDraft('');
    setTemplateSetupDialogOpen(false);
    setForm(createEmptyCustomForm());
    setDialogOpen(true);
  };

  const openTemplateLibrary = async () => {
    setTemplateDialogOpen(true);
    if (agentTemplates.length === 0) {
      await loadAgentTemplates();
    }
  };

  const openCreateFromTemplateDrawer = async (template: AgentTemplate) => {
    setEditingAgent(null);
    const defaultStarterFlowEnabled = template.starter_flows?.some((flow) => flow.default_enabled) ?? false;
    setTemplateDraft({ template, createStarterFlow: defaultStarterFlowEnabled });
    setAgentUsage(null);
    setAdvancedOpen(false);
    setAutomationOpen(false);
    setToolPickerOpen(false);
    setSystemDrawerOpen(false);
    setVersionDraftOpen(false);
    setVersionLabelDraft('');
    setVersionDescriptionDraft('');
    setForm(buildTemplateAgentForm(template));
    setTemplateDialogOpen(false);
    setTemplateForm({
      repository_id: '',
      release_kind: 'minor',
      include_prerelease: false,
      tag_pattern: '',
      space_id: '',
      collection_id: NONE_OPTION_VALUE,
    });
    const defaultMarketingTeam = visibleTeams.find((team) => (
      team.team_type === 'marketing'
      || team.handle?.toLowerCase() === 'marketing'
      || team.name.toLowerCase() === 'marketing'
    ));
    const defaultDependencyTeam = visibleTeams.find((team) => {
      const handle = team.handle?.toLowerCase() ?? '';
      const name = team.name.toLowerCase();
      return team.team_type === 'engineering'
        || handle === 'engineering'
        || handle === 'platform'
        || handle === 'backend'
        || name === 'engineering'
        || name === 'platform'
        || name === 'backend';
    }) ?? visibleTeams[0];
    setCompetitiveTemplateForm({
      target_company: '',
      target_domain: '',
      competitors_text: '',
      schedule_preset: 'weekly',
      lookback_days: '7',
      destination_team_id: defaultMarketingTeam?.id ?? '',
      destination_state_id: NONE_OPTION_VALUE,
    });
    setDependencyAuditorTemplateForm({
      repository_id: '',
      ecosystems: ['go', 'rust', 'python', 'node', 'java'],
      include_indirect: false,
      schedule_preset: 'weekly',
      destination_team_id: defaultDependencyTeam?.id ?? '',
      destination_state_id: NONE_OPTION_VALUE,
      max_tasks: '20',
    });
    setCompetitiveTeamWorkflow(null);
    setDocsCollections([]);
    if (template.starter_flows?.length) {
      setTemplateSetupDialogOpen(true);
    } else {
      setDialogOpen(true);
    }
    if (template.key === 'release_notes_writer' && (repositories.length === 0 || docsSpaces.length === 0)) {
      await loadTemplateResources();
    }
    if (template.key === DEPENDENCY_AUDITOR_TEMPLATE_KEY && repositories.length === 0) {
      await loadTemplateResources();
    }
    if (template.key === COMPETITIVE_INTEL_TEMPLATE_KEY && defaultMarketingTeam?.id) {
      await loadCompetitiveTeamWorkflow(defaultMarketingTeam.id);
    }
    if (template.key === DEPENDENCY_AUDITOR_TEMPLATE_KEY && defaultDependencyTeam?.id) {
      await loadCompetitiveTeamWorkflow(defaultDependencyTeam.id);
    }
  };

  const cancelTemplateSetup = () => {
    setTemplateSetupDialogOpen(false);
    setTemplateDraft(null);
    setDocsCollections([]);
    setCompetitiveTeamWorkflow(null);
  };

  const handleTemplateSetupContinue = () => {
    if (!templateDraft) return;
    if (templateDraft.createStarterFlow && templateDraft.template.key === 'release_notes_writer') {
      if (!templateForm.repository_id) {
        toast.error('Select a repository');
        return;
      }
      if (!templateForm.space_id) {
        toast.error('Select a docs space');
        return;
      }
    }
    if (templateDraft.createStarterFlow && templateDraft.template.key === COMPETITIVE_INTEL_TEMPLATE_KEY) {
      if (!competitiveTemplateForm.target_company.trim()) {
        toast.error('Enter a target company');
        return;
      }
      if (!competitiveTemplateForm.destination_team_id) {
        toast.error('Select a task team');
        return;
      }
      const lookbackDays = Number.parseInt(competitiveTemplateForm.lookback_days, 10);
      if (!Number.isFinite(lookbackDays) || lookbackDays < 1 || lookbackDays > 30) {
        toast.error('Lookback window must be between 1 and 30 days');
        return;
      }
    }
    if (templateDraft.createStarterFlow && templateDraft.template.key === DEPENDENCY_AUDITOR_TEMPLATE_KEY) {
      if (!dependencyAuditorTemplateForm.repository_id) {
        toast.error('Select a repository');
        return;
      }
      if (dependencyAuditorTemplateForm.ecosystems.length === 0) {
        toast.error('Select at least one ecosystem');
        return;
      }
      if (!dependencyAuditorTemplateForm.destination_team_id) {
        toast.error('Select a task team');
        return;
      }
      const maxTasks = Number.parseInt(dependencyAuditorTemplateForm.max_tasks, 10);
      if (!Number.isFinite(maxTasks) || maxTasks < 1 || maxTasks > 100) {
        toast.error('Maximum tasks must be between 1 and 100');
        return;
      }
    }
    if (templateDraft.createStarterFlow && templateDraft.template.key === COMPETITIVE_INTEL_TEMPLATE_KEY) {
      const renderedPrompt = renderCompetitiveIntelSystemPrompt(templateDraft.template.system_prompt ?? form.system_prompt, competitiveTemplateForm);
      setForm((current) => ({
        ...current,
        system_prompt: renderedPrompt,
      }));
    }
    if (templateDraft.createStarterFlow && templateDraft.template.key === DEPENDENCY_AUDITOR_TEMPLATE_KEY) {
      const renderedPrompt = renderDependencyAuditorSystemPrompt(templateDraft.template.system_prompt ?? form.system_prompt, dependencyAuditorTemplateForm);
      setForm((current) => ({
        ...current,
        system_prompt: renderedPrompt,
      }));
    }
    setTemplateSetupDialogOpen(false);
    setDialogOpen(true);
  };

  const openEditDialog = (agent: Agent) => {
    setEditingAgent(agent);
    setTemplateDraft(null);
    setAgentUsage(null);
    void loadAgentUsage(agent.id);
    setAdvancedOpen(hasConfiguredAdvancedFields(agent, presets));
    setAutomationOpen(agent.is_system ? false : agent.approval_mode !== 'preset_default');
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
        if (res.data) {
          setAgents((current) => current.map((agent) => (agent.id === res.data?.id ? res.data : agent)));
        }
        setDialogOpen(false);
        setSystemDrawerOpen(false);
        if (editingAgent.is_system) {
          await loadPresets();
        }
        await loadAgents();
      } else {
        toast.error(editingAgent.is_system ? 'Failed to save built-in agent' : 'Failed to save custom agent', {
          description: res.error,
        });
      }
    } else {
      if (templateDraft) {
        if (templateDraft.createStarterFlow && templateDraft.template.key === 'release_notes_writer') {
          if (!templateForm.repository_id) {
            toast.error('Select a repository');
            setSaving(false);
            return;
          }
          if (!templateForm.space_id) {
            toast.error('Select a docs space');
            setSaving(false);
            return;
          }
        }
        if (templateDraft.createStarterFlow && templateDraft.template.key === COMPETITIVE_INTEL_TEMPLATE_KEY) {
          if (!competitiveTemplateForm.target_company.trim()) {
            toast.error('Enter a target company');
            setSaving(false);
            return;
          }
          if (!competitiveTemplateForm.destination_team_id) {
            toast.error('Select a task team');
            setSaving(false);
            return;
          }
          const lookbackDays = Number.parseInt(competitiveTemplateForm.lookback_days, 10);
          if (!Number.isFinite(lookbackDays) || lookbackDays < 1 || lookbackDays > 30) {
            toast.error('Lookback window must be between 1 and 30 days');
            setSaving(false);
            return;
          }
        }
        if (templateDraft.createStarterFlow && templateDraft.template.key === DEPENDENCY_AUDITOR_TEMPLATE_KEY) {
          if (!dependencyAuditorTemplateForm.repository_id) {
            toast.error('Select a repository');
            setSaving(false);
            return;
          }
          if (dependencyAuditorTemplateForm.ecosystems.length === 0) {
            toast.error('Select at least one ecosystem');
            setSaving(false);
            return;
          }
          if (!dependencyAuditorTemplateForm.destination_team_id) {
            toast.error('Select a task team');
            setSaving(false);
            return;
          }
          const maxTasks = Number.parseInt(dependencyAuditorTemplateForm.max_tasks, 10);
          if (!Number.isFinite(maxTasks) || maxTasks < 1 || maxTasks > 100) {
            toast.error('Maximum tasks must be between 1 and 100');
            setSaving(false);
            return;
          }
        }

        const selectedRepo = repositories.find((repo) => repo.id === templateForm.repository_id);
        const selectedDependencyRepo = repositories.find((repo) => repo.id === dependencyAuditorTemplateForm.repository_id);
        const competitors = competitiveIntelCompetitorsFromText(competitiveTemplateForm.competitors_text);
        const templateFlow = templateDraft.createStarterFlow
          ? templateDraft.template.key === 'release_notes_writer'
            ? {
                repository_id: templateForm.repository_id,
                repo_full_name: selectedRepo?.full_name,
                release_kinds: templateForm.release_kind === 'any' ? undefined : [templateForm.release_kind],
                include_prerelease: templateForm.include_prerelease,
                tag_pattern: templateForm.tag_pattern.trim() || undefined,
                space_id: templateForm.space_id,
                collection_id: templateForm.collection_id === NONE_OPTION_VALUE ? undefined : templateForm.collection_id,
              }
            : templateDraft.template.key === COMPETITIVE_INTEL_TEMPLATE_KEY
              ? {
                  flow_key: COMPETITIVE_INTEL_FLOW_KEY,
                  flow_input: {
                    target_company: competitiveTemplateForm.target_company.trim(),
                    target_domain: competitiveTemplateForm.target_domain.trim() || undefined,
                    competitors,
                    schedule_preset: competitiveTemplateForm.schedule_preset,
                    lookback_days: Number.parseInt(competitiveTemplateForm.lookback_days, 10),
                    destination_team_id: competitiveTemplateForm.destination_team_id,
                    destination_state_id: competitiveTemplateForm.destination_state_id === NONE_OPTION_VALUE
                      ? undefined
                      : competitiveTemplateForm.destination_state_id,
                  },
                }
              : templateDraft.template.key === DEPENDENCY_AUDITOR_TEMPLATE_KEY
                ? {
                    flow_key: DEPENDENCY_AUDITOR_FLOW_KEY,
                    repository_id: dependencyAuditorTemplateForm.repository_id,
                    repo_full_name: selectedDependencyRepo?.full_name,
                    flow_input: {
                      ecosystems: dependencyAuditorTemplateForm.ecosystems,
                      include_indirect: dependencyAuditorTemplateForm.include_indirect,
                      schedule_preset: dependencyAuditorTemplateForm.schedule_preset,
                      destination_team_id: dependencyAuditorTemplateForm.destination_team_id,
                      destination_state_id: dependencyAuditorTemplateForm.destination_state_id === NONE_OPTION_VALUE
                        ? undefined
                        : dependencyAuditorTemplateForm.destination_state_id,
                      max_tasks: Number.parseInt(dependencyAuditorTemplateForm.max_tasks, 10),
                    },
                  }
              : undefined
          : undefined;
        const payload = {
          name: form.name.trim(),
          team_id: form.team_id || undefined,
          overrides: {
            role: templateDraft.template.default_role,
            runtime_kind: form.runtime_kind,
            skills: form.skills,
            provider: form.provider,
            model: form.model.trim(),
            monthly_token_budget: form.monthly_token_budget.trim()
              ? Number.parseInt(form.monthly_token_budget, 10)
              : 0,
            execution_config: buildExecutionConfigPayload(form),
            system_prompt: form.system_prompt,
            allowed_tools: normalizeToolList(form.allowed_tools),
            allowed_targets: normalizeTargetList(form.allowed_targets),
            approval_mode: form.approval_mode,
            max_concurrent_runs: form.max_concurrent_runs ? Number.parseInt(form.max_concurrent_runs, 10) : 1,
            default_invocation_mode: form.default_invocation_mode,
          },
          create_flow: templateDraft.createStarterFlow,
          flow: templateFlow,
        } satisfies CreateAgentFromTemplateRequest;
        const res = await automationService.createAgentFromTemplate(workspaceId, templateDraft.template.id, payload);
        if (!res.error) {
          setDialogOpen(false);
          setTemplateDraft(null);
          await loadAgents();
          toast.success(res.data?.flow ? 'Agent and starter flow created' : 'Agent created');
        } else {
          toast.error('Failed to create agent from template', { description: res.error });
        }
      } else {
        const payload = buildCreatePayload(workspaceId, form, advancedOpen);
        const res = await automationService.createAgent(workspaceId, payload);
        if (!res.error) {
          setDialogOpen(false);
          await loadAgents();
          toast.success('Agent created');
        } else {
          toast.error('Failed to create agent', { description: res.error });
        }
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
      model: form.model.trim(),
      execution_config: buildExecutionConfigPayload(form),
      system_prompt: form.system_prompt.trim() || undefined,
      instruction_preamble: form.instruction_preamble.trim() || undefined,
      instruction_skills: form.instruction_skills,
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

  const handleSaveWorkspaceVersion = async (options?: { silent?: boolean }) => {
    if (!workspaceId || !editingAgent?.is_system || !selectedPreset?.id) return false;
    if (!options?.silent) setSaving(true);
    const payload: UpdateWorkspaceAgentPresetVersionRequest = {
      label: selectedPreset.version_label,
      description: selectedPreset.description,
      runtime_kind: form.runtime_kind,
      provider: form.provider,
      model: form.model.trim(),
      execution_config: buildExecutionConfigPayload(form),
      system_prompt: form.system_prompt.trim() || undefined,
      instruction_preamble: form.instruction_preamble,
      instruction_skills: form.instruction_skills,
      allowed_tools: normalizeToolList(form.allowed_tools),
      supported_modes: form.supported_modes,
      default_invocation_mode: form.default_invocation_mode,
    };
    const res = await agentService.updatePresetVersion(workspaceId, selectedPreset.id, payload);
    if (res.error) {
      toast.error('Failed to save version', { description: res.error });
      if (!options?.silent) setSaving(false);
      return false;
    }
    await loadPresets();
    if (!options?.silent) {
      toast.success('Version saved');
      setSaving(false);
    }
    return true;
  };

  const handleSaveAndPin = async () => {
    setSaving(true);
    const saved = await handleSaveWorkspaceVersion({ silent: true });
    if (!saved) {
      setSaving(false);
      return;
    }
    await handleSave();
  };

  const handleDeleteWorkspaceVersion = async () => {
    if (!workspaceId || !workspaceVersionPendingDelete?.id) return;
    setDeletingVersion(true);
    const res = await agentService.deletePresetVersion(workspaceId, workspaceVersionPendingDelete.id);
    if (res.error) {
      toast.error('Failed to delete version', { description: res.error });
      setDeletingVersion(false);
      return;
    }
    const deletedKey = workspaceVersionPendingDelete.version_key;
    await loadPresets();
    setWorkspaceVersionPendingDelete(null);
    setDeletingVersion(false);
    if (form.preset_version_key === deletedKey && editingAgent) {
      // Selected row was just deleted — fall back to the agent's currently-pinned version or the product default.
      setForm(buildSystemAgentForm(editingAgent, presets));
    }
    toast.success('Workspace version deleted');
  };

  const handleRenameWorkspaceVersion = async () => {
    if (!workspaceId || !workspaceVersionBeingRenamed?.id) return;
    const label = renameLabelDraft.trim();
    if (!label) return;
    setRenamingVersion(true);
    const res = await agentService.updatePresetVersion(workspaceId, workspaceVersionBeingRenamed.id, {
      label,
      description: renameDescriptionDraft.trim(),
    });
    if (res.error) {
      toast.error('Failed to rename version', { description: res.error });
      setRenamingVersion(false);
      return;
    }
    await loadPresets();
    setWorkspaceVersionBeingRenamed(null);
    setRenameLabelDraft('');
    setRenameDescriptionDraft('');
    setRenamingVersion(false);
    toast.success('Version renamed');
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
  const sortedAgents = [...visibleAgents].sort((left, right) => {
    const leftAttention = needsAttention(left, runStats[left.id]) ? 1 : 0;
    const rightAttention = needsAttention(right, runStats[right.id]) ? 1 : 0;
    if (leftAttention !== rightAttention) return rightAttention - leftAttention;
    return left.name.localeCompare(right.name);
  });
  const runNowTargets = runNowTargetOptions(runNowAgent);
  const runnableRepositories = repositories.filter((repo) => repo.selected && repo.active && !repo.archived);
  const selectedRunNowRepository = repositories.find((repo) => repo.id === runNowTargetId);
  const runNowCanSubmit = Boolean(
    workspaceId
      && runNowAgent
      && runNowTargetType
      && (runNowTargetType === 'workspace' || runNowTargetId.trim()),
  );

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
  const isEditingWorkspaceVersion = editingSystemAgent
    && !versionDraftOpen
    && selectedPreset?.scope === 'workspace'
    && Boolean(selectedPreset?.id);
  const systemVersionReadOnly = editingSystemAgent && !versionDraftOpen && !isEditingWorkspaceVersion;
  const effectiveTargets =
    editingSystemAgent
      ? (selectedPreset?.allowed_target_types ?? form.allowed_targets)
      : (form.allowed_targets.length > 0 ? form.allowed_targets : ['task']);
  const supportedModes = form.supported_modes.length > 0 ? form.supported_modes : supportedModesForForm(form.runtime_kind);
  const availableRuntimeKinds = editingSystemAgent ? allowedRuntimeKindsForPreset(form.preset_key) : (['opencode', 'native_sdk'] as AgentRuntimeKind[]);
  const visibleProviderOptions = availableProvidersForRuntime(form.runtime_kind, providerOptions);
  const selectedProviderOption = visibleProviderOptions.find((option) => option.value === form.provider);
  const templateStarterFlow = templateDraft?.template.starter_flows?.find((flow) => flow.key === 'github_release_notes')
    ?? templateDraft?.template.starter_flows?.[0];
  const supportsReasoningEffort = form.runtime_kind === 'codex' && Boolean(selectedProviderOption?.supports_reasoning_effort);
  const supportsServiceTier = form.runtime_kind === 'codex' && Boolean(selectedProviderOption?.supports_service_tier);
  const codexUsesPresetCapabilities = form.runtime_kind === 'codex';
  const isTemplateCreate = Boolean(templateDraft && !editingAgent);
  const createDrawerTitle = editingAgent
    ? 'Edit Custom Agent'
    : templateDraft
      ? `Create ${templateDraft.template.name}`
      : 'Create Custom Agent';
  const createDrawerSubtitle = editingAgent
    ? 'Tune this custom agent directly. It is not pinned to a product preset.'
    : templateDraft
      ? 'Start from a packaged template, review the defaults, and create the agent with an optional automation flow.'
      : 'Define a reusable agent with its own instructions, runtime, tools, targets, and limits.';
  const selectedTeamName = visibleTeams.find((team) => team.id === form.team_id)?.name ?? 'Workspace-wide';
  const starterFlowEnabled = Boolean(templateDraft?.createStarterFlow);
  const createDrawerMissingRequirements = (() => {
    const missing: string[] = [];
    if (!form.name.trim()) {
      missing.push('agent name');
    }
    if (starterFlowEnabled && templateDraft?.template.key === 'release_notes_writer') {
      if (!templateForm.repository_id) missing.push('repository');
      if (!templateForm.space_id) missing.push('docs space');
    }
    if (starterFlowEnabled && templateDraft?.template.key === COMPETITIVE_INTEL_TEMPLATE_KEY) {
      if (!competitiveTemplateForm.target_company.trim()) missing.push('target company');
      if (!competitiveTemplateForm.destination_team_id) missing.push('task team');
      const lookbackDays = Number.parseInt(competitiveTemplateForm.lookback_days, 10);
      if (!Number.isFinite(lookbackDays) || lookbackDays < 1 || lookbackDays > 30) {
        missing.push('valid lookback window');
      }
    }
    if (starterFlowEnabled && templateDraft?.template.key === DEPENDENCY_AUDITOR_TEMPLATE_KEY) {
      if (!dependencyAuditorTemplateForm.repository_id) missing.push('repository');
      if (dependencyAuditorTemplateForm.ecosystems.length === 0) missing.push('ecosystem');
      if (!dependencyAuditorTemplateForm.destination_team_id) missing.push('task team');
      const maxTasks = Number.parseInt(dependencyAuditorTemplateForm.max_tasks, 10);
      if (!Number.isFinite(maxTasks) || maxTasks < 1 || maxTasks > 100) {
        missing.push('valid task limit');
      }
    }
    return missing;
  })();
  const createDrawerReady = createDrawerMissingRequirements.length === 0;
  const createDrawerStatus = createDrawerReady
    ? editingAgent
      ? 'Ready to save'
      : starterFlowEnabled
        ? 'Ready to create agent and automation flow'
        : 'Ready to create agent'
    : `${createDrawerMissingRequirements.length} required ${createDrawerMissingRequirements.length === 1 ? 'field' : 'fields'} remaining: ${createDrawerMissingRequirements.join(', ')}`;
  const createDrawerPrimaryLabel = saving
    ? 'Saving...'
    : editingAgent
      ? 'Save Changes'
      : starterFlowEnabled
        ? 'Create Agent + Flow'
        : isTemplateCreate
          ? 'Create Agent'
          : 'Create Custom Agent';
  const toolCatalogEntries = toolCatalog?.tools ?? [];
  const toggleTool = (toolName: string) => {
    setForm((current) => {
      const selected = current.allowed_tools.includes(toolName);
      return {
        ...current,
        allowed_tools: selected
          ? current.allowed_tools.filter((tool) => tool !== toolName)
          : normalizeToolList([...current.allowed_tools, toolName]),
      };
    });
  };
  const removeTool = (toolName: string) => {
    setForm((current) => ({
      ...current,
      allowed_tools: current.allowed_tools.filter((tool) => tool !== toolName),
    }));
  };
  const clearTools = () => {
    setForm((current) => ({
      ...current,
      allowed_tools: [],
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
      model: nextPreset.model ?? '',
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
    <div className="max-w-7xl mx-auto space-y-4">
      <div className="flex flex-col gap-3 md:flex-row md:items-start md:justify-between">
        <div className="space-y-1">
          <h1 className="text-xl font-semibold">Agents</h1>
          <p className="text-sm text-muted-foreground">
            Your fleet of built-in and custom agents. See what each one does, whether it is configured, and which flows depend on it.
          </p>
        </div>
        {sortedAgents.length > 0 && (
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
              <>
                <Button size="sm" variant="outline" onClick={() => void openTemplateLibrary()}>
                  <BookOpen01Icon className="mr-1.5 h-4 w-4" />
                  Use Template
                </Button>
                <Button size="sm" onClick={openCreateDialog}>
                  <PlusSignIcon className="mr-1.5 h-4 w-4" />
                  New Custom Agent
                </Button>
              </>
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
            <div className="mb-8 flex flex-wrap items-center justify-center gap-2">
              <Button variant="outline" className="gap-2" onClick={() => void openTemplateLibrary()}>
                <BookOpen01Icon className="h-4 w-4" />
                Use Template
              </Button>
              <Button className="gap-2" onClick={openCreateDialog}>
                <PlusSignIcon className="h-4 w-4" />
                New Custom Agent
              </Button>
            </div>
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
      {sortedAgents.length > 0 && viewMode === 'list' && (
        <div className="rounded-xl border border-border/70 overflow-hidden">
          <div className="hidden items-center gap-4 border-b border-border/70 bg-muted/30 px-4 py-2 text-[11px] font-medium uppercase tracking-[0.14em] text-muted-foreground lg:grid lg:grid-cols-[minmax(0,3.2fr)_minmax(170px,0.95fr)_110px_120px_150px_170px_112px]">
            <div>Agent · Role</div>
            <div>Model</div>
            <div>Mode</div>
            <div>Runs · 7d</div>
            <div>Last run</div>
            <div>Used by</div>
            <div className="text-right">Action</div>
          </div>
          {sortedAgents.map((agent) => (
            <AgentRow
              key={agent.id}
              agent={agent}
              stats={runStats[agent.id]}
              usage={agentUsageMap[agent.id]}
              workspaceSlug={workspace?.slug}
              presets={presets}
              onOpen={openEditDialog}
              onRunNow={openRunNowDialog}
              canEdit={canEdit}
            />
          ))}
        </div>
      )}

      {sortedAgents.length > 0 && viewMode === 'cards' && (
        <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
          {sortedAgents.map((agent) => (
            <AgentCard
              key={agent.id}
              agent={agent}
              stats={runStats[agent.id]}
              usage={agentUsageMap[agent.id]}
              workspaceSlug={workspace?.slug}
              presets={presets}
              onOpen={openEditDialog}
              onRunNow={openRunNowDialog}
              canEdit={canEdit}
            />
          ))}
        </div>
      )}

      <Dialog
        open={runNowOpen}
        onOpenChange={(open) => {
          setRunNowOpen(open);
          if (!open) {
            setRunNowAgent(null);
          }
        }}
      >
        <DialogContent className="sm:max-w-[560px]">
          <DialogHeader>
            <DialogTitle>Run agent now</DialogTitle>
            <DialogDescription>
              {runNowAgent ? `Start ${runNowAgent.name} manually with a concrete target and optional instructions.` : 'Start this agent manually.'}
            </DialogDescription>
          </DialogHeader>

          <div className="space-y-4">
            {runNowTargets.length === 0 ? (
              <div className="rounded-lg border border-dashed border-border/70 px-4 py-3 text-sm text-muted-foreground">
                This agent does not have a manually runnable target enabled.
              </div>
            ) : (
              <>
                <div className="space-y-2">
                  <Label htmlFor="run-now-target-type">Target</Label>
                  <Select value={runNowTargetType} onValueChange={handleRunNowTargetChange}>
                    <SelectTrigger id="run-now-target-type">
                      <SelectValue placeholder="Choose a target" />
                    </SelectTrigger>
                    <SelectContent>
                      {runNowTargets.map((target) => (
                        <SelectItem key={target} value={target}>
                          {labelForAgentTarget(target)}
                        </SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                </div>

                {runNowTargetType === 'workspace' ? (
                  <div className="rounded-lg border border-border/70 bg-muted/20 px-3 py-2">
                    <p className="text-xs font-medium uppercase tracking-[0.14em] text-muted-foreground">Workspace</p>
                    <p className="mt-1 text-sm">{workspace.name}</p>
                  </div>
                ) : null}

                {runNowTargetType === 'repository' ? (
                  <div className="space-y-3">
                    <div className="space-y-2">
                      <Label htmlFor="run-now-repository">Repository</Label>
                      <Select
                        value={runNowTargetId}
                        onValueChange={(repoId) => {
                          setRunNowTargetId(repoId);
                          const repo = repositories.find((item) => item.id === repoId);
                          setRunNowBaseBranch(repo?.default_branch ?? '');
                        }}
                        disabled={runNowRepositoriesLoading || runnableRepositories.length === 0}
                      >
                        <SelectTrigger id="run-now-repository">
                          <SelectValue placeholder={runNowRepositoriesLoading ? 'Loading repositories...' : 'Choose a repository'} />
                        </SelectTrigger>
                        <SelectContent>
                          {runnableRepositories.map((repo) => (
                            <SelectItem key={repo.id} value={repo.id}>
                              {repo.full_name}
                            </SelectItem>
                          ))}
                        </SelectContent>
                      </Select>
                      {!runNowRepositoriesLoading && runnableRepositories.length === 0 ? (
                        <p className="text-xs text-muted-foreground">No selected repositories are available for agent runs.</p>
                      ) : null}
                    </div>
                    <div className="space-y-2">
                      <Label htmlFor="run-now-base-branch">Base branch</Label>
                      <Input
                        id="run-now-base-branch"
                        value={runNowBaseBranch}
                        onChange={(event) => setRunNowBaseBranch(event.target.value)}
                        placeholder={selectedRunNowRepository?.default_branch || 'Repository default branch'}
                      />
                    </div>
                  </div>
                ) : null}

                {runNowTargetType && !['workspace', 'repository'].includes(runNowTargetType) ? (
                  <div className="space-y-2">
                    <Label htmlFor="run-now-target-id">{RUN_NOW_TARGET_ID_LABELS[runNowTargetType] ?? 'Target ID'}</Label>
                    <Input
                      id="run-now-target-id"
                      value={runNowTargetId}
                      onChange={(event) => setRunNowTargetId(event.target.value)}
                      placeholder={RUN_NOW_TARGET_ID_PLACEHOLDERS[runNowTargetType] ?? 'Paste a target ID'}
                    />
                  </div>
                ) : null}

                <div className="space-y-2">
                  <Label htmlFor="run-now-context">Run instructions</Label>
                  <Textarea
                    id="run-now-context"
                    value={runNowAdditionalContext}
                    onChange={(event) => setRunNowAdditionalContext(event.target.value)}
                    placeholder="Add anything this run should focus on."
                    rows={4}
                  />
                </div>
              </>
            )}
          </div>

          <DialogFooter>
            <Button
              type="button"
              variant="outline"
              onClick={() => setRunNowOpen(false)}
              disabled={runNowSubmitting}
            >
              Cancel
            </Button>
            <Button
              type="button"
              onClick={() => void handleRunNow()}
              disabled={!runNowCanSubmit || runNowSubmitting}
            >
              {runNowSubmitting ? <Loading01Icon className="mr-1.5 h-3.5 w-3.5 animate-spin" /> : <ZapIcon className="mr-1.5 h-3.5 w-3.5" />}
              Run now
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

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
          <SheetHeader className="border-b border-border/60 bg-muted/20 py-4 pl-6 pr-14">
            <div className="flex items-center gap-4">
              <AgentAvatar agent={editingAgent ?? undefined} className="h-11 w-11 shrink-0 rounded-none border-0 bg-transparent shadow-none" genericBare />
              <div className="min-w-0 flex-1">
                <SheetTitle className="text-lg">{editingAgent?.name ?? 'Built-in Agent'}</SheetTitle>
                <SheetDescription className="mt-0.5 text-xs">
                  Built-in {selectedPreset ? presetLabel(form.preset_key, presets) : 'agent'}
                  {currentSystemPreset && (
                    <>
                      {' · pinned to '}
                      <span className="font-medium text-foreground">{currentSystemPreset.version_label}</span>
                    </>
                  )}
                </SheetDescription>
              </div>
              {editingAgent && (
                <Button
                  type="button"
                  size="sm"
                  variant="ghost"
                  className="shrink-0"
                  onClick={() => {
                    window.open(resolveTriggerHistoryPath(workspace?.slug, editingAgent.id), '_blank');
                  }}
                >
                  View runs
                </Button>
              )}
            </div>
          </SheetHeader>

          <div className="grid min-h-0 flex-1 lg:grid-cols-[20rem_minmax(0,1fr)]">
            {/* ──────── LEFT: VERSIONS PANE ──────── */}
            <aside className="flex min-h-0 flex-col border-b border-border/60 bg-muted/20 lg:border-b-0 lg:border-r">
              <div className="flex items-center justify-between gap-2 border-b border-border/60 px-4 py-3">
                <div>
                  <p className="text-xs font-semibold uppercase tracking-wider text-muted-foreground">Versions</p>
                  <p className="text-[10px] text-muted-foreground">{selectedPresetVersions.length} available</p>
                </div>
                <Button
                  type="button"
                  size="sm"
                  variant="ghost"
                  className="h-7 gap-1 px-2 text-[11px]"
                  onClick={() => {
                    setVersionDraftOpen(true);
                    setVersionLabelDraft(`${selectedPreset?.version_label ?? 'Version'} Copy`);
                    setVersionDescriptionDraft(selectedPreset?.description ?? '');
                  }}
                >
                  <PlusSignIcon className="h-3.5 w-3.5" />
                  New
                </Button>
              </div>
              <div className="max-h-[30vh] overflow-y-auto px-2 py-2 lg:max-h-none lg:flex-1">
                <div className="space-y-1">
                  {selectedPresetVersions.map((presetVersion) => {
                    const isSelected = presetVersion.version_key === form.preset_version_key;
                    const isCurrent = presetVersion.version_key === currentSystemVersionKey;
                    const isDraftSelection = isSelected && !isCurrent;
                    const isWorkspace = presetVersion.scope === 'workspace';
                    return (
                      <div
                        key={presetVersion.version_key}
                        className={cn(
                          'group relative rounded-lg border transition-colors focus-within:ring-2 focus-within:ring-primary/40',
                          isSelected
                            ? 'border-primary/40 bg-background shadow-sm ring-1 ring-primary/20'
                            : 'border-transparent hover:bg-background hover:shadow-sm',
                        )}
                      >
                        <button
                          type="button"
                          onClick={() => selectSystemPresetVersion(presetVersion.version_key)}
                          className="flex w-full items-start gap-2 px-3 py-2 text-left"
                        >
                          <span className={cn(
                            'mt-1.5 inline-block h-1.5 w-1.5 shrink-0 rounded-full',
                            isCurrent ? 'bg-emerald-500' : 'bg-transparent',
                          )} />
                          <div className="min-w-0 flex-1 space-y-0.5">
                            <p className="truncate text-sm font-medium">{presetVersion.version_label}</p>
                            <p className="line-clamp-1 text-[11px] leading-snug text-muted-foreground">
                              {presetVersion.description || 'No description'}
                            </p>
                            <div className="flex flex-wrap gap-1 pt-1">
                              <Badge variant={isWorkspace ? 'secondary' : 'outline'} className="text-[9px] px-1.5 py-0">
                                {isWorkspace ? 'Workspace' : 'Product'}
                              </Badge>
                              {isCurrent && (
                                <Badge variant="outline" className="bg-emerald-500/10 text-[9px] px-1.5 py-0 text-emerald-700 dark:text-emerald-400">Pinned</Badge>
                              )}
                              {isDraftSelection && (
                                <Badge variant="secondary" className="text-[9px] px-1.5 py-0">Preview</Badge>
                              )}
                            </div>
                          </div>
                        </button>
                        {isWorkspace && presetVersion.id && (
                          <DropdownMenu>
                            <DropdownMenuTrigger asChild>
                              <Button
                                type="button"
                                variant="ghost"
                                size="icon"
                                className="absolute right-1.5 top-1.5 h-6 w-6 text-muted-foreground opacity-50 transition-opacity hover:opacity-100 group-hover:opacity-100 data-[state=open]:opacity-100"
                                aria-label={`Actions for ${presetVersion.version_label}`}
                              >
                                <MoreHorizontalIcon className="h-3.5 w-3.5" />
                              </Button>
                            </DropdownMenuTrigger>
                            <DropdownMenuContent align="end" className="w-44">
                              <DropdownMenuItem
                                onSelect={() => {
                                  setWorkspaceVersionBeingRenamed(presetVersion);
                                  setRenameLabelDraft(presetVersion.version_label);
                                  setRenameDescriptionDraft(presetVersion.description ?? '');
                                }}
                              >
                                Rename
                              </DropdownMenuItem>
                              <DropdownMenuItem
                                onSelect={() => {
                                  selectSystemPresetVersion(presetVersion.version_key);
                                  setVersionDraftOpen(true);
                                  setVersionLabelDraft(`${presetVersion.version_label} Copy`);
                                  setVersionDescriptionDraft(presetVersion.description ?? '');
                                }}
                              >
                                Duplicate
                              </DropdownMenuItem>
                              <DropdownMenuItem
                                disabled={isCurrent}
                                className="text-destructive focus:text-destructive"
                                onSelect={() => setWorkspaceVersionPendingDelete(presetVersion)}
                              >
                                Delete version
                              </DropdownMenuItem>
                            </DropdownMenuContent>
                          </DropdownMenu>
                        )}
                      </div>
                    );
                  })}
                </div>
              </div>
            </aside>

            {/* ──────── RIGHT: SELECTED VERSION DETAIL ──────── */}
            <div className="min-h-0 overflow-y-auto px-6 py-6">
              <div className="mx-auto w-full max-w-4xl space-y-6">
                {/* Selected version: header + behaviour strip, visually grouped */}
                <section className="overflow-hidden rounded-xl border border-border/60 bg-card">
                  <div className="flex flex-wrap items-start justify-between gap-3 px-5 py-4">
                    <div className="min-w-0 flex-1 space-y-1.5">
                      <div className="flex flex-wrap items-center gap-2">
                        <h2 className="text-lg font-semibold leading-none">{selectedPreset?.version_label ?? 'Version'}</h2>
                        {selectedPreset && (
                          <Badge variant={selectedPreset.scope === 'workspace' ? 'secondary' : 'outline'} className="text-[10px]">
                            {selectedPreset.scope === 'workspace' ? 'Workspace' : 'Product'}
                          </Badge>
                        )}
                        {selectedPreset?.version_key === currentSystemVersionKey ? (
                          <Badge variant="outline" className="border-emerald-500/30 bg-emerald-500/10 text-[10px] text-emerald-700 dark:text-emerald-400">Pinned</Badge>
                        ) : hasPendingSystemVersionSelection ? (
                          <Badge variant="outline" className="border-amber-500/40 bg-amber-500/10 text-[10px] text-amber-700 dark:text-amber-400">Previewing — not pinned</Badge>
                        ) : null}
                      </div>
                      {selectedPreset?.description && (
                        <p className="text-sm text-muted-foreground">{selectedPreset.description}</p>
                      )}
                    </div>
                    <Button
                      type="button"
                      size="sm"
                      variant="outline"
                      className="shrink-0"
                      onClick={() => {
                        setVersionDraftOpen(true);
                        setVersionLabelDraft(`${selectedPreset?.version_label ?? 'Version'} Copy`);
                        setVersionDescriptionDraft(selectedPreset?.description ?? '');
                      }}
                    >
                      Duplicate &amp; edit
                    </Button>
                  </div>
                  <dl className="grid grid-cols-2 divide-x divide-y divide-border/40 border-t border-border/40 bg-muted/20 sm:grid-cols-3 lg:grid-cols-6">
                    <div className="space-y-1 p-3">
                      <dt className="text-[10px] font-medium uppercase tracking-wider text-muted-foreground/70">Runtime</dt>
                      <dd className="truncate text-sm font-medium">{AGENT_RUNTIME_LABELS[form.runtime_kind] ?? form.runtime_kind}</dd>
                    </div>
                    <div className="space-y-1 p-3">
                      <dt className="text-[10px] font-medium uppercase tracking-wider text-muted-foreground/70">Model</dt>
                      <dd className="truncate text-sm font-medium" title={form.model}>{form.model || 'Auto'}</dd>
                    </div>
                    <div className="space-y-1 p-3">
                      <dt className="text-[10px] font-medium uppercase tracking-wider text-muted-foreground/70">Mode</dt>
                      <dd className="truncate text-sm font-medium">{INVOCATION_MODE_LABELS[form.default_invocation_mode]}</dd>
                    </div>
                    <div className="space-y-1 p-3">
                      <dt className="text-[10px] font-medium uppercase tracking-wider text-muted-foreground/70">Tools · Skills</dt>
                      <dd className="truncate text-sm font-medium">{form.allowed_tools.length} · {form.instruction_skills.length}</dd>
                    </div>
                    <div className="space-y-1 p-3">
                      <dt className="text-[10px] font-medium uppercase tracking-wider text-muted-foreground/70">Targets</dt>
                      <dd className="truncate text-sm font-medium" title={effectiveTargets.join(', ') || '—'}>
                        {effectiveTargets.length > 0 ? effectiveTargets.join(', ') : '—'}
                      </dd>
                    </div>
                    <div className="space-y-1 p-3">
                      <dt className="text-[10px] font-medium uppercase tracking-wider text-muted-foreground/70">Last run</dt>
                      <dd className="truncate text-sm font-medium" title={(editingAgent && runStats[editingAgent.id]?.lastRun?.created_at) || undefined}>
                        {editingAgent && runStats[editingAgent.id]?.lastRun?.created_at
                          ? formatDistanceToNow(new Date(runStats[editingAgent.id].lastRun!.created_at), { addSuffix: true })
                          : 'Never'}
                      </dd>
                    </div>
                  </dl>
                </section>

                {/* ──────── VERSION DETAILS ──────── */}
                <section className="space-y-3">
                  <div className="flex items-center justify-between gap-3">
                    <h3 className="text-xs font-semibold uppercase tracking-wider text-muted-foreground">Version details</h3>
                    <Badge variant={systemVersionReadOnly ? 'outline' : 'secondary'} className="text-[10px]">
                      {versionDraftOpen
                        ? 'New draft'
                        : isEditingWorkspaceVersion
                          ? 'Editable'
                          : 'Read-only'}
                    </Badge>
                  </div>
                  <div className="space-y-2">
                  {/* 01A — Agent Identity (Preamble) */}
                  {(form.instruction_preamble || !systemVersionReadOnly) && (
                  <Collapsible.Root defaultOpen className="rounded-xl border border-border/60 bg-card">
                    <Collapsible.Trigger asChild>
                      <button type="button" className="group flex w-full items-center gap-3 px-4 py-3 text-left">
                        <ArrowRight01Icon className="h-3.5 w-3.5 text-muted-foreground transition-transform group-data-[state=open]:rotate-90" />
                        <span className="flex-1 text-sm font-medium">Agent identity</span>
                        <span className="text-xs text-muted-foreground group-data-[state=open]:hidden">Preamble</span>
                      </button>
                    </Collapsible.Trigger>
                    <Collapsible.Content>
                      <div className="border-t border-border/60 p-4">
                        {systemVersionReadOnly ? (
                          <p className="whitespace-pre-wrap rounded-md border-l-2 border-border bg-muted/30 px-3 py-2 text-sm leading-relaxed text-foreground/90">{form.instruction_preamble || <span className="text-muted-foreground">No preamble.</span>}</p>
                        ) : (
                          <Textarea
                            value={form.instruction_preamble}
                            onChange={(e) => setForm((current) => ({ ...current, instruction_preamble: e.target.value }))}
                            placeholder="e.g. You are Epic Planner. You run the full PRD-to-tasks loop inside a single interactive run."
                            rows={3}
                            className="resize-none border border-dashed border-border/60 bg-muted/30 px-3 py-2 text-sm shadow-none focus-visible:border-primary focus-visible:bg-background focus-visible:ring-1 focus-visible:ring-primary"
                          />
                        )}
                      </div>
                    </Collapsible.Content>
                  </Collapsible.Root>
                  )}

                  {/* 01B — Instruction Skills */}
                  {(form.instruction_skills.length > 0 || !systemVersionReadOnly) && (
                  <Collapsible.Root defaultOpen className="rounded-xl border border-border/60 bg-card">
                    <Collapsible.Trigger asChild>
                      <button type="button" className="group flex w-full items-center gap-3 px-4 py-3 text-left">
                        <ArrowRight01Icon className="h-3.5 w-3.5 text-muted-foreground transition-transform group-data-[state=open]:rotate-90" />
                        <span className="flex-1 text-sm font-medium">Skills</span>
                        <span className="text-xs text-muted-foreground group-data-[state=open]:hidden">{form.instruction_skills.length} module{form.instruction_skills.length === 1 ? '' : 's'}</span>
                      </button>
                    </Collapsible.Trigger>
                    <Collapsible.Content>
                      <div className="space-y-3 border-t border-border/60 p-4">
                        <div className="space-y-2">
                          {form.instruction_skills.map((skillKey, idx) => {
                            const entry = skillCatalogEntries.find((s) => s.key === skillKey);
                            return (
                              <div key={skillKey} className="flex items-start gap-3 rounded-lg border border-border/50 bg-background px-3 py-2.5">
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
                            <PopoverContent
                              align="start"
                              className="w-[28rem] overflow-hidden p-0"
                              onWheelCapture={(event) => event.stopPropagation()}
                            >
                              <Command>
                                <CommandInput placeholder="Search skills..." />
                                <CommandList className="max-h-72 overscroll-contain">
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
                    </Collapsible.Content>
                  </Collapsible.Root>
                  )}

                  {/* 01C — Compiled System Prompt (Preview) */}
                  <Collapsible.Root open={compiledPromptOpen} onOpenChange={setCompiledPromptOpen} className="rounded-xl border border-border/60 bg-card">
                    <Collapsible.Trigger asChild>
                      <button type="button" className="group flex w-full items-center gap-3 px-4 py-3 text-left">
                        <ArrowRight01Icon className="h-3.5 w-3.5 text-muted-foreground transition-transform group-data-[state=open]:rotate-90" />
                        <SourceCodeIcon className="h-3.5 w-3.5 text-muted-foreground" />
                        <span className="flex-1 text-sm font-medium">Compiled system prompt</span>
                        <Badge variant="outline" className="text-[9px] px-1.5 py-0">Preview</Badge>
                      </button>
                    </Collapsible.Trigger>
                        <Collapsible.Content>
                          <div className="border-t border-border/60 p-4">
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

                  {/* 02 — Run Mode */}
                  <Collapsible.Root defaultOpen={false} className="rounded-xl border border-border/60 bg-card">
                    <Collapsible.Trigger asChild>
                      <button type="button" className="group flex w-full items-center gap-3 px-4 py-3 text-left">
                        <ArrowRight01Icon className="h-3.5 w-3.5 text-muted-foreground transition-transform group-data-[state=open]:rotate-90" />
                        <span className="flex-1 text-sm font-medium">Run mode</span>
                        <span className="text-xs text-muted-foreground group-data-[state=open]:hidden">{INVOCATION_MODE_LABELS[form.default_invocation_mode]}</span>
                      </button>
                    </Collapsible.Trigger>
                    <Collapsible.Content>
                      <div className="border-t border-border/60 px-4 py-3">
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
                    </Collapsible.Content>
                  </Collapsible.Root>

                  {/* 03 — Execution */}
                  <Collapsible.Root defaultOpen={false} className="rounded-xl border border-border/60 bg-card">
                    <Collapsible.Trigger asChild>
                      <button type="button" className="group flex w-full items-center gap-3 px-4 py-3 text-left">
                        <ArrowRight01Icon className="h-3.5 w-3.5 text-muted-foreground transition-transform group-data-[state=open]:rotate-90" />
                        <span className="flex-1 text-sm font-medium">Execution</span>
                        <span className="text-xs text-muted-foreground group-data-[state=open]:hidden">{AGENT_RUNTIME_LABELS[form.runtime_kind] ?? form.runtime_kind} · {form.model || 'Auto'}</span>
                      </button>
                    </Collapsible.Trigger>
                    <Collapsible.Content>
                      <div className="border-t border-border/60 p-4">
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
                    </Collapsible.Content>
                  </Collapsible.Root>

                  {/* 04 — Allowed Tools */}
                  <Collapsible.Root defaultOpen={false} className="rounded-xl border border-border/60 bg-card">
                    <Collapsible.Trigger asChild>
                      <button type="button" className="group flex w-full items-center gap-3 px-4 py-3 text-left">
                        <ArrowRight01Icon className="h-3.5 w-3.5 text-muted-foreground transition-transform group-data-[state=open]:rotate-90" />
                        <span className="flex-1 text-sm font-medium">Allowed tools</span>
                        <span className="text-xs text-muted-foreground group-data-[state=open]:hidden">{form.allowed_tools.length} enabled</span>
                      </button>
                    </Collapsible.Trigger>
                    <Collapsible.Content>
                      <div className="space-y-3 border-t border-border/60 p-4">
                        <div className="flex items-center justify-end gap-2">
                          <ToolMultiSelectPopover
                            open={toolPickerOpen}
                            onOpenChange={setToolPickerOpen}
                            tools={toolCatalogEntries}
                            selectedTools={form.allowed_tools}
                            disabled={codexUsesPresetCapabilities || systemVersionReadOnly}
                            onToggleTool={toggleTool}
                            onClearTools={clearTools}
                          />
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
                    </Collapsible.Content>
                  </Collapsible.Root>
                  </div>
                </section>

                {/* ──────── AGENT SETTINGS (agent-scoped) ──────── */}
                <section className="space-y-3">
                  <div className="flex items-baseline justify-between gap-3">
                    <div>
                      <h3 className="text-xs font-semibold uppercase tracking-wider text-muted-foreground">Agent settings</h3>
                      <p className="text-[11px] text-muted-foreground">Applies to {editingAgent?.name ?? 'this agent'}, not the pinned version.</p>
                    </div>
                    {editingAgent && (
                      <button
                        type="button"
                        className="text-[11px] font-medium text-muted-foreground underline-offset-2 hover:text-foreground hover:underline"
                        onClick={() => {
                          window.open(resolveTriggerHistoryPath(workspace?.slug, editingAgent.id), '_blank');
                        }}
                      >
                        {agentUsage?.items?.length ?? 0} trigger{(agentUsage?.items?.length ?? 0) === 1 ? '' : 's'} · view runs →
                      </button>
                    )}
                  </div>
                  <div className="rounded-xl border border-border/60 bg-card p-4">
                    <div className="flex items-baseline justify-between gap-3">
                      <p className="text-sm font-medium">Monthly usage limit</p>
                      {editingAgent && (() => {
                        const used = editingAgent.tokens_used_this_month ?? 0;
                        const budget = form.monthly_token_budget ? Number.parseInt(form.monthly_token_budget, 10) : 0;
                        const pct = budget > 0 ? Math.min(100, Math.round((used / budget) * 100)) : 0;
                        return (
                          <p className="text-[11px] text-muted-foreground">
                            {budget > 0
                              ? `$${used.toLocaleString()} of $${budget.toLocaleString()} this month (${pct}%)`
                              : `$${used.toLocaleString()} spent this month · no limit set`}
                          </p>
                        );
                      })()}
                    </div>
                    {editingAgent && (() => {
                      const used = editingAgent.tokens_used_this_month ?? 0;
                      const budget = form.monthly_token_budget ? Number.parseInt(form.monthly_token_budget, 10) : 0;
                      const pct = budget > 0 ? Math.min(100, Math.round((used / budget) * 100)) : 0;
                      if (budget <= 0) return null;
                      return (
                        <div className="mt-2 h-1 w-full overflow-hidden rounded-full bg-muted">
                          <div
                            className={cn(
                              'h-full rounded-full transition-all',
                              pct >= 90 ? 'bg-destructive' : pct >= 70 ? 'bg-amber-500' : 'bg-primary',
                            )}
                            style={{ width: `${pct}%` }}
                          />
                        </div>
                      );
                    })()}
                    <div className="mt-3 flex flex-wrap gap-1.5">
                      {[
                        { label: 'No limit', value: '' },
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
                      <Input
                        id="agent-budget"
                        type="number"
                        value={form.monthly_token_budget}
                        onChange={(e) => setForm((current) => ({ ...current, monthly_token_budget: e.target.value }))}
                        placeholder="Custom…"
                        className="h-7 w-28 text-xs"
                      />
                    </div>
                  </div>
                </section>

              </div>
            </div>
          </div>

          <SheetFooter className="border-t border-border/60 bg-background py-4 pl-6 pr-20 sm:flex-row sm:justify-between">
            <div className="text-xs text-muted-foreground">
              {versionDraftOpen
                ? 'Configure the new version, then create it.'
                : isEditingWorkspaceVersion && hasPendingSystemVersionSelection
                  ? `Save and pin ${selectedPreset?.version_label} to ${editingAgent?.name ?? 'this agent'}.`
                  : isEditingWorkspaceVersion
                    ? `Editing ${selectedPreset?.version_label}. Save changes to apply.`
                    : hasPendingSystemVersionSelection
                      ? `Pin ${selectedPreset?.version_label ?? 'this version'} to ${editingAgent?.name ?? 'this agent'}?`
                      : 'No changes to save.'}
            </div>
            <div className="flex gap-2">
              <Button variant="outline" size="sm" onClick={() => setSystemDrawerOpen(false)}>
                {hasPendingSystemVersionSelection || versionDraftOpen || isEditingWorkspaceVersion ? 'Cancel' : 'Close'}
              </Button>
              {versionDraftOpen ? (
                <Button
                  size="sm"
                  disabled={creatingVersion || !versionLabelDraft.trim()}
                  onClick={handleCreatePresetVersion}
                >
                  {creatingVersion ? 'Creating…' : 'Create version'}
                </Button>
              ) : isEditingWorkspaceVersion && hasPendingSystemVersionSelection ? (
                <Button
                  size="sm"
                  disabled={saving}
                  onClick={handleSaveAndPin}
                >
                  {saving ? 'Saving & pinning…' : 'Save & pin'}
                </Button>
              ) : isEditingWorkspaceVersion ? (
                <Button
                  size="sm"
                  disabled={saving}
                  onClick={() => handleSaveWorkspaceVersion()}
                >
                  {saving ? 'Saving…' : 'Save version'}
                </Button>
              ) : (
                <Button
                  size="sm"
                  disabled={saving || !hasPendingSystemVersionSelection}
                  onClick={handleSave}
                >
                  {saving
                    ? 'Pinning…'
                    : hasPendingSystemVersionSelection
                      ? `Pin ${selectedPreset?.version_label ?? 'version'}`
                      : 'Pin to agent'}
                </Button>
              )}
            </div>
          </SheetFooter>
        </SheetContent>
      </Sheet>

      {/* ---- New workspace version dialog ---- */}
      <Dialog
        open={versionDraftOpen}
        onOpenChange={(open) => {
          if (!open) {
            setVersionDraftOpen(false);
            setVersionLabelDraft('');
            setVersionDescriptionDraft('');
            if (editingAgent) {
              setForm(buildSystemAgentForm(editingAgent, presets));
            }
          }
        }}
      >
        <DialogContent className="sm:max-w-[520px]">
          <DialogHeader>
            <DialogTitle>New workspace version</DialogTitle>
            <DialogDescription>
              Creates a new workspace version from the current configuration. The built-in agent will be pinned to the new version automatically.
            </DialogDescription>
          </DialogHeader>
          <div className="space-y-4">
            <div className="space-y-2">
              <FieldLabel htmlFor="preset-version-label">Version label</FieldLabel>
              <Input
                id="preset-version-label"
                value={versionLabelDraft}
                onChange={(e) => setVersionLabelDraft(e.target.value)}
                placeholder="e.g. Engineering tuned"
                autoFocus
              />
            </div>
            <div className="space-y-2">
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
          <DialogFooter>
            <Button
              type="button"
              variant="outline"
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
              Cancel
            </Button>
            <Button
              type="button"
              size="sm"
              disabled={creatingVersion || !versionLabelDraft.trim()}
              onClick={handleCreatePresetVersion}
            >
              {creatingVersion ? 'Creating…' : 'Create version'}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <Dialog
        open={templateSetupDialogOpen && Boolean(templateDraft)}
        onOpenChange={(open) => {
          if (!open) {
            cancelTemplateSetup();
          } else {
            setTemplateSetupDialogOpen(true);
          }
        }}
      >
        <DialogContent className="max-h-[88vh] overflow-y-auto sm:max-w-[820px]">
          <DialogHeader>
            <DialogTitle>{templateDraft ? `Set up ${templateDraft.template.name}` : 'Set up template'}</DialogTitle>
            <DialogDescription>
              Configure the template-specific values first. The next drawer is the normal custom agent editor.
            </DialogDescription>
          </DialogHeader>

          {templateDraft?.template.key === 'release_notes_writer' && (
            <div className="space-y-5 rounded-xl border border-border/60 bg-card p-5">
              <div className="flex items-start justify-between gap-4">
                <div className="space-y-1">
                  <p className="text-sm font-semibold">{templateStarterFlow?.label ?? 'Starter flow'}</p>
                  <p className="text-[11px] leading-relaxed text-muted-foreground">
                    {templateStarterFlow?.description ?? 'Create a starter automation flow when this agent is created.'}
                  </p>
                </div>
                <label className="flex shrink-0 items-center gap-2 rounded-lg border border-border/60 bg-muted/20 px-3 py-2">
                  <span className="text-xs font-medium">Create recommended automation flow</span>
                  <Switch
                    checked={templateDraft.createStarterFlow}
                    onCheckedChange={(checked) => setTemplateDraft((current) => (
                      current ? { ...current, createStarterFlow: checked } : current
                    ))}
                  />
                </label>
              </div>

              {templateDraft.createStarterFlow && (
                <div className="space-y-4">
                  <div className="grid gap-4 sm:grid-cols-2">
                    <div className="grid gap-2">
                      <FieldLabel htmlFor="template-setup-repository">Repository</FieldLabel>
                      <Select
                        value={templateForm.repository_id || undefined}
                        onValueChange={(value) => setTemplateForm((current) => ({ ...current, repository_id: value }))}
                      >
                        <SelectTrigger id="template-setup-repository">
                          <SelectValue placeholder={templateResourcesLoading ? 'Loading repositories...' : 'Select a repository'} />
                        </SelectTrigger>
                        <SelectContent>
                          {repositories.map((repo) => (
                            <SelectItem key={repo.id} value={repo.id}>
                              {repo.full_name}
                            </SelectItem>
                          ))}
                        </SelectContent>
                      </Select>
                    </div>

                    <div className="grid gap-2">
                      <FieldLabel htmlFor="template-setup-release-kind">Release type</FieldLabel>
                      <Select
                        value={templateForm.release_kind}
                        onValueChange={(value: ReleaseNotesTemplateFormData['release_kind']) => {
                          setTemplateForm((current) => ({ ...current, release_kind: value }));
                        }}
                      >
                        <SelectTrigger id="template-setup-release-kind">
                          <SelectValue />
                        </SelectTrigger>
                        <SelectContent>
                          {TEMPLATE_RELEASE_KIND_OPTIONS.map((option) => (
                            <SelectItem key={option.value} value={option.value}>
                              {option.label}
                            </SelectItem>
                          ))}
                        </SelectContent>
                      </Select>
                    </div>
                  </div>

                  <div className="grid gap-4 sm:grid-cols-[minmax(0,1fr)_auto] sm:items-end">
                    <div className="grid gap-2">
                      <FieldLabel htmlFor="template-setup-tag-pattern">Tag pattern</FieldLabel>
                      <Input
                        id="template-setup-tag-pattern"
                        value={templateForm.tag_pattern}
                        onChange={(event) => setTemplateForm((current) => ({ ...current, tag_pattern: event.target.value }))}
                        placeholder="Optional, e.g. v*"
                      />
                    </div>
                    <div className="flex items-center justify-between gap-3 rounded-lg border border-border/60 bg-muted/20 px-3 py-2.5">
                      <div className="space-y-0.5">
                        <p className="text-sm font-medium">Prereleases</p>
                        <p className="text-[11px] text-muted-foreground">Include beta and rc tags</p>
                      </div>
                      <Switch
                        checked={templateForm.include_prerelease}
                        onCheckedChange={(checked) => setTemplateForm((current) => ({ ...current, include_prerelease: checked }))}
                      />
                    </div>
                  </div>

                  <div className="grid gap-4 sm:grid-cols-2">
                    <div className="grid gap-2">
                      <FieldLabel htmlFor="template-setup-space">Docs space</FieldLabel>
                      <Select
                        value={templateForm.space_id || undefined}
                        onValueChange={(value) => {
                          setTemplateForm((current) => ({
                            ...current,
                            space_id: value,
                            collection_id: NONE_OPTION_VALUE,
                          }));
                          void loadCollectionsForSpace(value);
                        }}
                      >
                        <SelectTrigger id="template-setup-space">
                          <SelectValue placeholder={templateResourcesLoading ? 'Loading spaces...' : 'Select a docs space'} />
                        </SelectTrigger>
                        <SelectContent>
                          {docsSpaces.map((space) => (
                            <SelectItem key={space.id} value={space.id}>
                              {space.name}
                            </SelectItem>
                          ))}
                        </SelectContent>
                      </Select>
                    </div>

                    <div className="grid gap-2">
                      <FieldLabel htmlFor="template-setup-collection">Collection</FieldLabel>
                      <Select
                        value={templateForm.collection_id}
                        onValueChange={(value) => setTemplateForm((current) => ({ ...current, collection_id: value }))}
                        disabled={!templateForm.space_id}
                      >
                        <SelectTrigger id="template-setup-collection">
                          <SelectValue placeholder={templateForm.space_id ? 'Optional collection' : 'Select a docs space first'} />
                        </SelectTrigger>
                        <SelectContent>
                          <SelectItem value={NONE_OPTION_VALUE}>Space root</SelectItem>
                          {docsCollections.map((collection) => (
                            <SelectItem key={collection.id} value={collection.id}>
                              {collection.name}
                            </SelectItem>
                          ))}
                        </SelectContent>
                      </Select>
                    </div>
                  </div>
                </div>
              )}
            </div>
          )}

          {templateDraft?.template.key === COMPETITIVE_INTEL_TEMPLATE_KEY && (
            <div className="space-y-5 rounded-xl border border-border/60 bg-card p-5">
              <div className="flex items-start justify-between gap-4">
                <div className="space-y-1">
                  <p className="text-sm font-semibold">{templateStarterFlow?.label ?? 'Starter flow'}</p>
                  <p className="text-[11px] leading-relaxed text-muted-foreground">
                    {templateStarterFlow?.description ?? 'Create a starter automation flow when this agent is created.'}
                  </p>
                </div>
                <label className="flex shrink-0 items-center gap-2 rounded-lg border border-border/60 bg-muted/20 px-3 py-2">
                  <span className="text-xs font-medium">Create recommended automation flow</span>
                  <Switch
                    checked={templateDraft.createStarterFlow}
                    onCheckedChange={(checked) => setTemplateDraft((current) => (
                      current ? { ...current, createStarterFlow: checked } : current
                    ))}
                  />
                </label>
              </div>

              {templateDraft.createStarterFlow && (
                <div className="space-y-4">
                  <div className="grid gap-4 sm:grid-cols-2">
                    <div className="grid gap-2">
                      <FieldLabel htmlFor="competitive-setup-target-company">Target company</FieldLabel>
                      <Input
                        id="competitive-setup-target-company"
                        value={competitiveTemplateForm.target_company}
                        onChange={(event) => setCompetitiveTemplateForm((current) => ({
                          ...current,
                          target_company: event.target.value,
                        }))}
                        placeholder="Usermaven"
                      />
                    </div>

                    <div className="grid gap-2">
                      <FieldLabel htmlFor="competitive-setup-target-domain">Target domain</FieldLabel>
                      <Input
                        id="competitive-setup-target-domain"
                        value={competitiveTemplateForm.target_domain}
                        onChange={(event) => setCompetitiveTemplateForm((current) => ({
                          ...current,
                          target_domain: event.target.value,
                        }))}
                        placeholder="usermaven.com"
                      />
                    </div>
                  </div>

                  <div className="grid gap-2">
                    <FieldLabel htmlFor="competitive-setup-competitors">Known competitors</FieldLabel>
                    <Textarea
                      id="competitive-setup-competitors"
                      value={competitiveTemplateForm.competitors_text}
                      onChange={(event) => setCompetitiveTemplateForm((current) => ({
                        ...current,
                        competitors_text: event.target.value,
                      }))}
                      placeholder={'jasper.ai\nwritesonic.ai'}
                      className="min-h-[92px]"
                    />
                    <p className="text-[11px] text-muted-foreground">
                      Optional. Use one domain or company per line; if empty, the agent discovers competitors during each run.
                    </p>
                  </div>

                  <div className="grid gap-4 sm:grid-cols-4">
                    <div className="grid gap-2">
                      <FieldLabel htmlFor="competitive-setup-cadence">Run cadence</FieldLabel>
                      <Select
                        value={competitiveTemplateForm.schedule_preset}
                        onValueChange={(value) => {
                          setCompetitiveTemplateForm((current) => ({
                            ...current,
                            schedule_preset: value === 'daily' ? 'daily' : 'weekly',
                          }));
                        }}
                      >
                        <SelectTrigger id="competitive-setup-cadence">
                          <SelectValue />
                        </SelectTrigger>
                        <SelectContent>
                          {COMPETITIVE_INTEL_SCHEDULE_OPTIONS.map((option) => (
                            <SelectItem key={option.value} value={option.value}>
                              {option.label}
                            </SelectItem>
                          ))}
                        </SelectContent>
                      </Select>
                    </div>

                    <div className="grid gap-2">
                      <FieldLabel htmlFor="competitive-setup-lookback">Lookback days</FieldLabel>
                      <Input
                        id="competitive-setup-lookback"
                        type="number"
                        min={1}
                        max={30}
                        value={competitiveTemplateForm.lookback_days}
                        onChange={(event) => setCompetitiveTemplateForm((current) => ({
                          ...current,
                          lookback_days: event.target.value,
                        }))}
                      />
                    </div>

                    <div className="grid gap-2">
                      <FieldLabel htmlFor="competitive-setup-team">Task team</FieldLabel>
                      <Select
                        value={competitiveTemplateForm.destination_team_id || undefined}
                        onValueChange={(value) => {
                          setCompetitiveTemplateForm((current) => ({
                            ...current,
                            destination_team_id: value,
                            destination_state_id: NONE_OPTION_VALUE,
                          }));
                          void loadCompetitiveTeamWorkflow(value);
                        }}
                      >
                        <SelectTrigger id="competitive-setup-team">
                          <SelectValue placeholder="Select a team" />
                        </SelectTrigger>
                        <SelectContent>
                          {visibleTeams.map((team) => (
                            <SelectItem key={team.id} value={team.id}>
                              {team.name}
                            </SelectItem>
                          ))}
                        </SelectContent>
                      </Select>
                    </div>

                    <div className="grid gap-2">
                      <FieldLabel htmlFor="competitive-setup-state">Task stage</FieldLabel>
                      <Select
                        value={competitiveTemplateForm.destination_state_id}
                        onValueChange={(value) => setCompetitiveTemplateForm((current) => ({
                          ...current,
                          destination_state_id: value,
                        }))}
                        disabled={!competitiveTemplateForm.destination_team_id}
                      >
                        <SelectTrigger id="competitive-setup-state">
                          <SelectValue placeholder={competitiveTemplateForm.destination_team_id ? 'Default stage' : 'Select a team first'} />
                        </SelectTrigger>
                        <SelectContent>
                          <SelectItem value={NONE_OPTION_VALUE}>Team default</SelectItem>
                          {(competitiveTeamWorkflow?.states ?? []).map((state) => (
                            <SelectItem key={state.id} value={state.id}>
                              {state.name}
                            </SelectItem>
                          ))}
                        </SelectContent>
                      </Select>
                    </div>
                  </div>
                </div>
              )}
            </div>
          )}

          {templateDraft?.template.key === DEPENDENCY_AUDITOR_TEMPLATE_KEY && (
            <div className="space-y-5 rounded-xl border border-border/60 bg-card p-5">
              <div className="flex items-start justify-between gap-4">
                <div className="space-y-1">
                  <p className="text-sm font-semibold">{templateStarterFlow?.label ?? 'Starter flow'}</p>
                  <p className="text-[11px] leading-relaxed text-muted-foreground">
                    {templateStarterFlow?.description ?? 'Create a starter automation flow when this agent is created.'}
                  </p>
                </div>
                <label className="flex shrink-0 items-center gap-2 rounded-lg border border-border/60 bg-muted/20 px-3 py-2">
                  <span className="text-xs font-medium">Create recommended automation flow</span>
                  <Switch
                    checked={templateDraft.createStarterFlow}
                    onCheckedChange={(checked) => setTemplateDraft((current) => (
                      current ? { ...current, createStarterFlow: checked } : current
                    ))}
                  />
                </label>
              </div>

              {templateDraft.createStarterFlow && (
                <div className="space-y-4">
                  <div className="grid gap-4 sm:grid-cols-2">
                    <div className="grid gap-2">
                      <FieldLabel htmlFor="dependency-setup-repository">Repository</FieldLabel>
                      <Select
                        value={dependencyAuditorTemplateForm.repository_id || undefined}
                        onValueChange={(value) => setDependencyAuditorTemplateForm((current) => ({
                          ...current,
                          repository_id: value,
                        }))}
                      >
                        <SelectTrigger id="dependency-setup-repository">
                          <SelectValue placeholder={templateResourcesLoading ? 'Loading repositories...' : 'Select a repository'} />
                        </SelectTrigger>
                        <SelectContent>
                          {repositories.map((repo) => (
                            <SelectItem key={repo.id} value={repo.id}>
                              {repo.full_name}
                            </SelectItem>
                          ))}
                        </SelectContent>
                      </Select>
                    </div>

                    <div className="grid gap-2">
                      <FieldLabel htmlFor="dependency-setup-cadence">Run cadence</FieldLabel>
                      <Select
                        value={dependencyAuditorTemplateForm.schedule_preset}
                        onValueChange={(value) => {
                          setDependencyAuditorTemplateForm((current) => ({
                            ...current,
                            schedule_preset: value === 'daily' ? 'daily' : 'weekly',
                          }));
                        }}
                      >
                        <SelectTrigger id="dependency-setup-cadence">
                          <SelectValue />
                        </SelectTrigger>
                        <SelectContent>
                          {DEPENDENCY_AUDITOR_SCHEDULE_OPTIONS.map((option) => (
                            <SelectItem key={option.value} value={option.value}>
                              {option.label}
                            </SelectItem>
                          ))}
                        </SelectContent>
                      </Select>
                    </div>
                  </div>

                  <div className="grid gap-2">
                    <FieldLabel>Ecosystems</FieldLabel>
                    <div className="grid gap-2 sm:grid-cols-3">
                      {DEPENDENCY_AUDITOR_ECOSYSTEM_OPTIONS.map((option) => {
                        const checked = dependencyAuditorTemplateForm.ecosystems.includes(option.value);
                        return (
                          <label
                            key={option.value}
                            className={cn(
                              'flex items-center justify-between rounded-lg border px-3 py-2.5 text-sm transition-colors',
                              checked ? 'border-primary/35 bg-primary/5' : 'border-border/60 bg-muted/20',
                            )}
                          >
                            <span className="font-medium">{option.label}</span>
                            <Switch
                              checked={checked}
                              onCheckedChange={(nextChecked) => setDependencyAuditorTemplateForm((current) => {
                                const currentValues = new Set(current.ecosystems);
                                if (nextChecked) {
                                  currentValues.add(option.value);
                                } else {
                                  currentValues.delete(option.value);
                                }
                                return {
                                  ...current,
                                  ecosystems: DEPENDENCY_AUDITOR_ECOSYSTEM_OPTIONS
                                    .map((item) => item.value)
                                    .filter((value) => currentValues.has(value)),
                                };
                              })}
                            />
                          </label>
                        );
                      })}
                    </div>
                  </div>

                  <div className="grid gap-4 sm:grid-cols-[minmax(0,1fr)_auto] sm:items-end">
                    <div className="grid gap-2">
                      <FieldLabel htmlFor="dependency-setup-max-tasks">Maximum tasks per run</FieldLabel>
                      <Input
                        id="dependency-setup-max-tasks"
                        type="number"
                        min={1}
                        max={100}
                        value={dependencyAuditorTemplateForm.max_tasks}
                        onChange={(event) => setDependencyAuditorTemplateForm((current) => ({
                          ...current,
                          max_tasks: event.target.value,
                        }))}
                      />
                    </div>
                    <div className="flex items-center justify-between gap-3 rounded-lg border border-border/60 bg-muted/20 px-3 py-2.5">
                      <div className="space-y-0.5">
                        <p className="text-sm font-medium">Indirect dependencies</p>
                        <p className="text-[11px] text-muted-foreground">Include transitive entries</p>
                      </div>
                      <Switch
                        checked={dependencyAuditorTemplateForm.include_indirect}
                        onCheckedChange={(checked) => setDependencyAuditorTemplateForm((current) => ({
                          ...current,
                          include_indirect: checked,
                        }))}
                      />
                    </div>
                  </div>

                  <div className="grid gap-4 sm:grid-cols-2">
                    <div className="grid gap-2">
                      <FieldLabel htmlFor="dependency-setup-team">Task team</FieldLabel>
                      <Select
                        value={dependencyAuditorTemplateForm.destination_team_id || undefined}
                        onValueChange={(value) => {
                          setDependencyAuditorTemplateForm((current) => ({
                            ...current,
                            destination_team_id: value,
                            destination_state_id: NONE_OPTION_VALUE,
                          }));
                          void loadCompetitiveTeamWorkflow(value);
                        }}
                      >
                        <SelectTrigger id="dependency-setup-team">
                          <SelectValue placeholder="Select a team" />
                        </SelectTrigger>
                        <SelectContent>
                          {visibleTeams.map((team) => (
                            <SelectItem key={team.id} value={team.id}>
                              {team.name}
                            </SelectItem>
                          ))}
                        </SelectContent>
                      </Select>
                    </div>

                    <div className="grid gap-2">
                      <FieldLabel htmlFor="dependency-setup-state">Task stage</FieldLabel>
                      <Select
                        value={dependencyAuditorTemplateForm.destination_state_id}
                        onValueChange={(value) => setDependencyAuditorTemplateForm((current) => ({
                          ...current,
                          destination_state_id: value,
                        }))}
                        disabled={!dependencyAuditorTemplateForm.destination_team_id}
                      >
                        <SelectTrigger id="dependency-setup-state">
                          <SelectValue placeholder={dependencyAuditorTemplateForm.destination_team_id ? 'Default stage' : 'Select a team first'} />
                        </SelectTrigger>
                        <SelectContent>
                          <SelectItem value={NONE_OPTION_VALUE}>Team default</SelectItem>
                          {(competitiveTeamWorkflow?.states ?? []).map((state) => (
                            <SelectItem key={state.id} value={state.id}>
                              {state.name}
                            </SelectItem>
                          ))}
                        </SelectContent>
                      </Select>
                    </div>
                  </div>
                </div>
              )}
            </div>
          )}

          <DialogFooter>
            <Button type="button" variant="outline" onClick={cancelTemplateSetup}>
              Back
            </Button>
            <Button type="button" onClick={handleTemplateSetupContinue}>
              Continue to agent settings
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* ---- Create / Edit drawer ---- */}
      <Sheet
        open={dialogOpen}
        onOpenChange={(open) => {
          setDialogOpen(open);
          if (!open) {
            setToolPickerOpen(false);
            setSkillPickerOpen(false);
            setSystemPromptEditorOpen(false);
            setTemplateSetupDialogOpen(false);
            setTemplateDraft(null);
            setDocsCollections([]);
          }
        }}
      >
        <SheetContent side="right" className="w-full gap-0 p-0 data-[side=right]:w-[88vw] data-[side=right]:sm:max-w-[88vw] xl:data-[side=right]:w-[1280px] xl:data-[side=right]:max-w-[1280px]">
          <SheetHeader className="border-b border-border/60 bg-muted/20 py-4 pl-6 pr-14">
            <div className="flex items-center gap-4">
              <AgentAvatar agent={editingAgent ?? undefined} className="h-11 w-11 shrink-0 rounded-none border-0 bg-transparent shadow-none" genericBare />
              <div className="min-w-0 flex-1">
                <SheetTitle className="text-lg">{createDrawerTitle}</SheetTitle>
                <SheetDescription className="mt-0.5 max-w-3xl text-xs">
                  {createDrawerSubtitle}
                </SheetDescription>
              </div>
              {templateDraft && (
                <Badge variant="secondary" className="shrink-0 text-[10px]">
                  Template
                </Badge>
              )}
            </div>
          </SheetHeader>

          <div className="min-h-0 flex-1 overflow-y-auto px-6 py-6">
            <div className="mx-auto w-full max-w-5xl space-y-4">
              <section className="overflow-hidden rounded-xl border border-border/60 bg-card">
                <div className="flex flex-wrap items-start justify-between gap-3 px-5 py-4">
                  <div className="min-w-0 flex-1 space-y-1.5">
                    <div className="flex flex-wrap items-center gap-2">
                      <h2 className="text-lg font-semibold leading-none">{templateDraft ? templateDraft.template.name : form.name || 'Custom agent'}</h2>
                      <Badge variant={templateDraft ? 'secondary' : 'outline'} className="text-[10px]">
                        {templateDraft ? 'Template-based' : editingAgent ? 'Custom agent' : 'New custom'}
                      </Badge>
                      {starterFlowEnabled && (
                        <Badge variant="outline" className="border-emerald-500/30 bg-emerald-500/10 text-[10px] text-emerald-700 dark:text-emerald-400">
                          Automation flow
                        </Badge>
                      )}
                    </div>
                    <p className="text-sm text-muted-foreground">
                      {templateDraft?.template.description
                        || 'A direct custom agent configuration owned by this workspace.'}
                    </p>
                  </div>
                  <div className="rounded-lg border border-border/50 bg-muted/20 px-3 py-2 text-right">
                    <p className="text-[10px] font-semibold uppercase tracking-wider text-muted-foreground/70">Status</p>
                    <p className={cn('text-xs font-medium', createDrawerReady ? 'text-emerald-700 dark:text-emerald-400' : 'text-amber-700 dark:text-amber-400')}>
                      {createDrawerReady ? 'Ready' : `${createDrawerMissingRequirements.length} missing`}
                    </p>
                  </div>
                </div>
                <dl className="grid grid-cols-2 divide-x divide-y divide-border/40 border-t border-border/40 bg-muted/20 sm:grid-cols-3 lg:grid-cols-6">
                  <div className="space-y-1 p-3">
                    <dt className="text-[10px] font-medium uppercase tracking-wider text-muted-foreground/70">Runtime</dt>
                    <dd className="truncate text-sm font-medium">{AGENT_RUNTIME_LABELS[form.runtime_kind] ?? form.runtime_kind}</dd>
                  </div>
                  <div className="space-y-1 p-3">
                    <dt className="text-[10px] font-medium uppercase tracking-wider text-muted-foreground/70">Model</dt>
                    <dd className="truncate text-sm font-medium" title={form.model}>{form.model || 'Auto'}</dd>
                  </div>
                  <div className="space-y-1 p-3">
                    <dt className="text-[10px] font-medium uppercase tracking-wider text-muted-foreground/70">Mode</dt>
                    <dd className="truncate text-sm font-medium">{INVOCATION_MODE_LABELS[form.default_invocation_mode]}</dd>
                  </div>
                  <div className="space-y-1 p-3">
                    <dt className="text-[10px] font-medium uppercase tracking-wider text-muted-foreground/70">Tools · Skills</dt>
                    <dd className="truncate text-sm font-medium">{form.allowed_tools.length} · {form.skills.length}</dd>
                  </div>
                  <div className="space-y-1 p-3">
                    <dt className="text-[10px] font-medium uppercase tracking-wider text-muted-foreground/70">Targets</dt>
                    <dd className="truncate text-sm font-medium" title={effectiveTargets.join(', ') || '—'}>
                      {effectiveTargets.length > 0 ? effectiveTargets.map(templateTargetLabel).join(', ') : '—'}
                    </dd>
                  </div>
                  <div className="space-y-1 p-3">
                    <dt className="text-[10px] font-medium uppercase tracking-wider text-muted-foreground/70">Team</dt>
                    <dd className="truncate text-sm font-medium" title={selectedTeamName}>{selectedTeamName}</dd>
                  </div>
                </dl>
              </section>

              <DrawerConfigSection title="Identity" description="Name, team, and operating scope">
                <div className="grid gap-4 md:grid-cols-2">
                  <div className="space-y-2">
                    <FieldLabel htmlFor="agent-name">Name</FieldLabel>
                    <Input
                      id="agent-name"
                      value={form.name}
                      onChange={(e) => setForm((current) => ({ ...current, name: e.target.value }))}
                      placeholder="e.g. Code Reviewer, Sales Assistant"
                    />
                  </div>

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
                </div>
              </DrawerConfigSection>

              <DrawerConfigSection title="Behavior" description="Instructions and reusable skills">
                <div className="space-y-2">
                  <div className="flex items-center justify-between gap-3">
                    <FieldLabel
                      htmlFor="agent-system-prompt"
                      tooltip="Instructions stored on the agent itself. For planners, keep the planning behavior here rather than in a separate planner-only field."
                    >
                      System instructions
                    </FieldLabel>
                    <Button
                      type="button"
                      variant="ghost"
                      size="sm"
                      className="h-7 gap-1.5 px-2 text-[11px]"
                      onClick={() => setSystemPromptEditorOpen(true)}
                    >
                      <ArrowExpandIcon className="h-3.5 w-3.5" />
                      Expand
                    </Button>
                  </div>
                  <Textarea
                    id="agent-system-prompt"
                    value={form.system_prompt}
                    onChange={(e) => setForm((current) => ({ ...current, system_prompt: e.target.value }))}
                    placeholder="Agent instructions"
                    rows={5}
                    className="resize-none"
                  />
                </div>

                <div className="space-y-3">
                  <div className="flex items-center justify-between gap-2">
                    <div>
                      <p className="text-sm font-medium">Skills</p>
                      <p className="text-[11px] text-muted-foreground">Behavioral instruction modules attached at runtime.</p>
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
                      <PopoverContent
                        align="end"
                        className="w-[28rem] overflow-hidden p-0"
                        onWheelCapture={(event) => event.stopPropagation()}
                      >
                        <Command>
                          <CommandInput placeholder="Search skills..." />
                          <CommandList className="max-h-72 overscroll-contain">
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
              </DrawerConfigSection>

            {false && templateDraft?.template.key === 'release_notes_writer' && (
              <div className="rounded-xl border border-border/60 bg-card p-5">
                <div className="flex items-start justify-between gap-4">
                  <div className="space-y-1">
                    <p className="text-sm font-semibold">{templateStarterFlow?.label ?? 'Starter flow'}</p>
                    <p className="text-[11px] leading-relaxed text-muted-foreground">
                      {templateStarterFlow?.description ?? 'Create a starter automation flow when this agent is created.'}
                    </p>
                  </div>
                  <label className="flex shrink-0 items-center gap-2 rounded-lg border border-border/60 bg-muted/20 px-3 py-2">
                    <span className="text-xs font-medium">Create recommended automation flow</span>
                    <Switch
                      checked={Boolean(templateDraft?.createStarterFlow)}
                      onCheckedChange={(checked) => setTemplateDraft((current) => (
                        current ? { ...current, createStarterFlow: checked } : current
                      ))}
                    />
                  </label>
                </div>

                {templateDraft?.createStarterFlow && (
                  <div className="mt-5 space-y-4">
                    <div className="grid gap-4 sm:grid-cols-2">
                      <div className="grid gap-2">
                        <FieldLabel htmlFor="template-repository">Repository</FieldLabel>
                        <Select
                          value={templateForm.repository_id || undefined}
                          onValueChange={(value) => setTemplateForm((current) => ({ ...current, repository_id: value }))}
                        >
                          <SelectTrigger id="template-repository">
                            <SelectValue placeholder={templateResourcesLoading ? 'Loading repositories...' : 'Select a repository'} />
                          </SelectTrigger>
                          <SelectContent>
                            {repositories.map((repo) => (
                              <SelectItem key={repo.id} value={repo.id}>
                                {repo.full_name}
                              </SelectItem>
                            ))}
                          </SelectContent>
                        </Select>
                      </div>

                      <div className="grid gap-2">
                        <FieldLabel htmlFor="template-release-kind">Release type</FieldLabel>
                        <Select
                          value={templateForm.release_kind}
                          onValueChange={(value: ReleaseNotesTemplateFormData['release_kind']) => {
                            setTemplateForm((current) => ({ ...current, release_kind: value }));
                          }}
                        >
                          <SelectTrigger id="template-release-kind">
                            <SelectValue />
                          </SelectTrigger>
                          <SelectContent>
                            {TEMPLATE_RELEASE_KIND_OPTIONS.map((option) => (
                              <SelectItem key={option.value} value={option.value}>
                                {option.label}
                              </SelectItem>
                            ))}
                          </SelectContent>
                        </Select>
                      </div>
                    </div>

                    <div className="grid gap-4 sm:grid-cols-[minmax(0,1fr)_auto] sm:items-end">
                      <div className="grid gap-2">
                        <FieldLabel htmlFor="template-tag-pattern">Tag pattern</FieldLabel>
                        <Input
                          id="template-tag-pattern"
                          value={templateForm.tag_pattern}
                          onChange={(event) => setTemplateForm((current) => ({ ...current, tag_pattern: event.target.value }))}
                          placeholder="Optional, e.g. v*"
                        />
                      </div>
                      <div className="flex items-center justify-between gap-3 rounded-lg border border-border/60 bg-muted/20 px-3 py-2.5">
                        <div className="space-y-0.5">
                          <p className="text-sm font-medium">Prereleases</p>
                          <p className="text-[11px] text-muted-foreground">Include beta and rc tags</p>
                        </div>
                        <Switch
                          checked={templateForm.include_prerelease}
                          onCheckedChange={(checked) => setTemplateForm((current) => ({ ...current, include_prerelease: checked }))}
                        />
                      </div>
                    </div>

                    <div className="grid gap-4 sm:grid-cols-2">
                      <div className="grid gap-2">
                        <FieldLabel htmlFor="template-space">Docs space</FieldLabel>
                        <Select
                          value={templateForm.space_id || undefined}
                          onValueChange={(value) => {
                            setTemplateForm((current) => ({
                              ...current,
                              space_id: value,
                              collection_id: NONE_OPTION_VALUE,
                            }));
                            void loadCollectionsForSpace(value);
                          }}
                        >
                          <SelectTrigger id="template-space">
                            <SelectValue placeholder={templateResourcesLoading ? 'Loading spaces...' : 'Select a docs space'} />
                          </SelectTrigger>
                          <SelectContent>
                            {docsSpaces.map((space) => (
                              <SelectItem key={space.id} value={space.id}>
                                {space.name}
                              </SelectItem>
                            ))}
                          </SelectContent>
                        </Select>
                      </div>

                      <div className="grid gap-2">
                        <FieldLabel htmlFor="template-collection">Collection</FieldLabel>
                        <Select
                          value={templateForm.collection_id}
                          onValueChange={(value) => setTemplateForm((current) => ({ ...current, collection_id: value }))}
                          disabled={!templateForm.space_id}
                        >
                          <SelectTrigger id="template-collection">
                            <SelectValue placeholder={templateForm.space_id ? 'Optional collection' : 'Select a docs space first'} />
                          </SelectTrigger>
                          <SelectContent>
                            <SelectItem value={NONE_OPTION_VALUE}>Space root</SelectItem>
                            {docsCollections.map((collection) => (
                              <SelectItem key={collection.id} value={collection.id}>
                                {collection.name}
                              </SelectItem>
                            ))}
                          </SelectContent>
                        </Select>
                      </div>
                    </div>
                  </div>
                )}
              </div>
            )}

            {false && templateDraft?.template.key === COMPETITIVE_INTEL_TEMPLATE_KEY && (
              <div className="rounded-xl border border-border/60 bg-card p-5">
                <div className="flex items-start justify-between gap-4">
                  <div className="space-y-1">
                    <p className="text-sm font-semibold">{templateStarterFlow?.label ?? 'Starter flow'}</p>
                    <p className="text-[11px] leading-relaxed text-muted-foreground">
                      {templateStarterFlow?.description ?? 'Create a starter automation flow when this agent is created.'}
                    </p>
                  </div>
                  <label className="flex shrink-0 items-center gap-2 rounded-lg border border-border/60 bg-muted/20 px-3 py-2">
                    <span className="text-xs font-medium">Create recommended automation flow</span>
                    <Switch
                      checked={Boolean(templateDraft?.createStarterFlow)}
                      onCheckedChange={(checked) => setTemplateDraft((current) => (
                        current ? { ...current, createStarterFlow: checked } : current
                      ))}
                    />
                  </label>
                </div>

                {templateDraft?.createStarterFlow && (
                  <div className="mt-5 space-y-4">
                    <div className="grid gap-4 sm:grid-cols-2">
                      <div className="grid gap-2">
                        <FieldLabel htmlFor="competitive-target-company">Target company</FieldLabel>
                        <Input
                          id="competitive-target-company"
                          value={competitiveTemplateForm.target_company}
                          onChange={(event) => setCompetitiveTemplateForm((current) => ({
                            ...current,
                            target_company: event.target.value,
                          }))}
                          placeholder="Usermaven"
                        />
                      </div>

                      <div className="grid gap-2">
                        <FieldLabel htmlFor="competitive-target-domain">Target domain</FieldLabel>
                        <Input
                          id="competitive-target-domain"
                          value={competitiveTemplateForm.target_domain}
                          onChange={(event) => setCompetitiveTemplateForm((current) => ({
                            ...current,
                            target_domain: event.target.value,
                          }))}
                          placeholder="usermaven.com"
                        />
                      </div>
                    </div>

                    <div className="grid gap-2">
                      <FieldLabel htmlFor="competitive-competitors">Known competitors</FieldLabel>
                      <Textarea
                        id="competitive-competitors"
                        value={competitiveTemplateForm.competitors_text}
                        onChange={(event) => setCompetitiveTemplateForm((current) => ({
                          ...current,
                          competitors_text: event.target.value,
                        }))}
                        placeholder={'jasper.ai\nwritesonic.ai'}
                        className="min-h-[92px]"
                      />
                      <p className="text-[11px] text-muted-foreground">
                        Optional. Use one domain or company per line; if empty, the agent discovers competitors during each run.
                      </p>
                    </div>

                    <div className="grid gap-4 sm:grid-cols-4">
                      <div className="grid gap-2">
                        <FieldLabel htmlFor="competitive-cadence">Run cadence</FieldLabel>
                        <Select
                          value={competitiveTemplateForm.schedule_preset}
                          onValueChange={(value) => {
                            setCompetitiveTemplateForm((current) => ({
                              ...current,
                              schedule_preset: value === 'daily' ? 'daily' : 'weekly',
                            }));
                          }}
                        >
                          <SelectTrigger id="competitive-cadence">
                            <SelectValue />
                          </SelectTrigger>
                          <SelectContent>
                            {COMPETITIVE_INTEL_SCHEDULE_OPTIONS.map((option) => (
                              <SelectItem key={option.value} value={option.value}>
                                {option.label}
                              </SelectItem>
                            ))}
                          </SelectContent>
                        </Select>
                      </div>

                      <div className="grid gap-2">
                        <FieldLabel htmlFor="competitive-lookback">Lookback days</FieldLabel>
                        <Input
                          id="competitive-lookback"
                          type="number"
                          min={1}
                          max={30}
                          value={competitiveTemplateForm.lookback_days}
                          onChange={(event) => setCompetitiveTemplateForm((current) => ({
                            ...current,
                            lookback_days: event.target.value,
                          }))}
                        />
                      </div>

                      <div className="grid gap-2">
                        <FieldLabel htmlFor="competitive-team">Task team</FieldLabel>
                        <Select
                          value={competitiveTemplateForm.destination_team_id || undefined}
                          onValueChange={(value) => {
                            setCompetitiveTemplateForm((current) => ({
                              ...current,
                              destination_team_id: value,
                              destination_state_id: NONE_OPTION_VALUE,
                            }));
                            void loadCompetitiveTeamWorkflow(value);
                          }}
                        >
                          <SelectTrigger id="competitive-team">
                            <SelectValue placeholder="Select a team" />
                          </SelectTrigger>
                          <SelectContent>
                            {visibleTeams.map((team) => (
                              <SelectItem key={team.id} value={team.id}>
                                {team.name}
                              </SelectItem>
                            ))}
                          </SelectContent>
                        </Select>
                      </div>

                      <div className="grid gap-2">
                        <FieldLabel htmlFor="competitive-state">Task stage</FieldLabel>
                        <Select
                          value={competitiveTemplateForm.destination_state_id}
                          onValueChange={(value) => setCompetitiveTemplateForm((current) => ({
                            ...current,
                            destination_state_id: value,
                          }))}
                          disabled={!competitiveTemplateForm.destination_team_id}
                        >
                          <SelectTrigger id="competitive-state">
                            <SelectValue placeholder={competitiveTemplateForm.destination_team_id ? 'Default stage' : 'Select a team first'} />
                          </SelectTrigger>
                          <SelectContent>
                            <SelectItem value={NONE_OPTION_VALUE}>Team default</SelectItem>
                            {(competitiveTeamWorkflow?.states ?? []).map((state) => (
                              <SelectItem key={state.id} value={state.id}>
                                {state.name}
                              </SelectItem>
                            ))}
                          </SelectContent>
                        </Select>
                      </div>
                    </div>
                  </div>
                )}
              </div>
            )}

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

            <DrawerConfigSection title="Capabilities" description="Targets, tools, and supported modes" defaultOpen={false}>
              <div className="grid gap-5 sm:grid-cols-2">
                <div className="space-y-2">
                  <FieldLabel tooltip="Choose which target types this custom agent is allowed to run against.">
                    Allowed targets
                  </FieldLabel>
                  <div className="grid gap-2">
                    {CUSTOM_AGENT_TARGET_OPTIONS.map((target) => {
                      const active = form.allowed_targets.includes(target.value);
                      return (
                        <button
                          key={target.value}
                          type="button"
                          onClick={() => toggleTarget(target.value)}
                          className={cn(
                            'rounded-lg border px-3 py-2 text-left transition-colors',
                            active
                              ? 'border-primary bg-primary/10 text-foreground'
                              : 'border-border bg-background hover:bg-muted/40',
                          )}
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
                </div>

                <div className="space-y-4">
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

                  <Separator />

                  <div className="space-y-3">
                    <div className="flex items-center justify-between gap-2">
                      <p className="text-[10px] font-semibold uppercase tracking-wider text-muted-foreground/70">Available tools</p>
                      <ToolMultiSelectPopover
                        open={toolPickerOpen}
                        onOpenChange={setToolPickerOpen}
                        tools={toolCatalogEntries}
                        selectedTools={form.allowed_tools}
                        disabled={codexUsesPresetCapabilities}
                        onToggleTool={toggleTool}
                        onClearTools={clearTools}
                      />
                    </div>
                    <p className="text-[11px] leading-relaxed text-muted-foreground">
                      {codexUsesPresetCapabilities
                        ? 'Codex currently uses the preset capability set as-is. Custom tool overrides are disabled for this runtime.'
                        : 'Choose from the workspace tool catalog. Selected tools become this agent&apos;s allowed tool list.'}
                    </p>
                    <div className="flex max-h-40 flex-wrap gap-1.5 overflow-y-auto rounded-lg border border-border/50 bg-muted/20 p-2.5">
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
              </div>
            </DrawerConfigSection>

            <Collapsible.Root open={automationOpen} onOpenChange={setAutomationOpen}>
              <Collapsible.Trigger asChild>
                <Button type="button" variant="ghost" className="flex w-full items-center justify-between px-2">
                  <span className="flex items-center gap-2 text-sm">
                    {automationOpen ? <ArrowDown01Icon className="h-4 w-4" /> : <ArrowRight01Icon className="h-4 w-4" />}
                    Approval & Limits
                  </span>
                </Button>
              </Collapsible.Trigger>
              <Collapsible.Content className="space-y-4 rounded-md border bg-muted/30 p-3 mt-2">
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
          <SheetFooter className="border-t border-border/60 bg-background py-4 pl-6 pr-20 sm:flex-row sm:justify-between">
            <div className="flex min-w-0 items-center gap-3">
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
              <p className={cn('truncate text-xs', createDrawerReady ? 'text-muted-foreground' : 'text-amber-700 dark:text-amber-400')}>
                {createDrawerStatus}
              </p>
            </div>
            <div className="flex gap-2">
              <Button variant="outline" size="sm" onClick={() => setDialogOpen(false)}>
                Cancel
              </Button>
              <Button
                size="sm"
                disabled={saving || !createDrawerReady}
                onClick={handleSave}
              >
                {createDrawerPrimaryLabel}
              </Button>
            </div>
          </SheetFooter>
        </SheetContent>
      </Sheet>

      <Dialog open={systemPromptEditorOpen} onOpenChange={setSystemPromptEditorOpen}>
        <DialogContent className="z-[140] gap-0 overflow-hidden p-0 sm:max-w-5xl">
          <DialogHeader className="border-b border-border/70 px-6 py-4">
            <DialogTitle>System instructions</DialogTitle>
            <DialogDescription>
              Edit the prompt stored on {form.name.trim() || 'this custom agent'}.
            </DialogDescription>
          </DialogHeader>
          <div className="p-6">
            <Textarea
              id="agent-system-prompt-expanded"
              value={form.system_prompt}
              onChange={(e) => setForm((current) => ({ ...current, system_prompt: e.target.value }))}
              placeholder="Agent instructions"
              className="min-h-[62vh] resize-none font-mono text-xs leading-relaxed"
              autoFocus
            />
          </div>
          <DialogFooter className="border-t border-border/70 px-6 py-4">
            <Button type="button" size="sm" onClick={() => setSystemPromptEditorOpen(false)}>
              Done
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <ConfirmDialog
        open={deleteConfirmOpen}
        onOpenChange={setDeleteConfirmOpen}
        title="Delete agent"
        description="This will permanently remove this agent and its direct configuration. This action cannot be undone."
        confirmLabel="Delete"
        variant="destructive"
        onConfirm={handleDelete}
      />

      <ConfirmDialog
        open={workspaceVersionPendingDelete !== null}
        onOpenChange={(open) => !open && setWorkspaceVersionPendingDelete(null)}
        title="Delete workspace version"
        description={
          workspaceVersionPendingDelete
            ? `This will permanently delete "${workspaceVersionPendingDelete.version_label}". Agents pinned to this version will need to be re-pinned. This action cannot be undone.`
            : ''
        }
        confirmLabel={deletingVersion ? 'Deleting…' : 'Delete version'}
        variant="destructive"
        onConfirm={handleDeleteWorkspaceVersion}
      />

      <Dialog
        open={templateDialogOpen}
        onOpenChange={(open) => {
          setTemplateDialogOpen(open);
        }}
      >
        <DialogContent className="sm:max-w-[820px]">
          <DialogHeader>
            <DialogTitle>Agent Templates</DialogTitle>
            <DialogDescription>
              Start from a prebuilt agent and optionally create its starter automation flow.
            </DialogDescription>
          </DialogHeader>
          <div className="grid gap-3 sm:grid-cols-2">
            {agentTemplates.map((template) => {
              const starterFlow = template.starter_flows?.[0];
              return (
                <button
                  key={template.id}
                  type="button"
                  className="rounded-xl border border-border/60 p-4 text-left transition-colors hover:bg-muted/30"
                  onClick={() => void openCreateFromTemplateDrawer(template)}
                >
                  <div className="flex items-start justify-between gap-3">
                    <div className="min-w-0 flex-1 space-y-3">
                      <div className="flex items-center gap-2">
                        <span className="truncate text-sm font-semibold">{template.name}</span>
                        <Badge variant="secondary" className="text-[10px]">
                          Template
                        </Badge>
                      </div>
                      <p className="line-clamp-2 text-xs text-muted-foreground">{template.description || 'No description provided.'}</p>
                      <div className="flex flex-wrap gap-1.5">
                        {(template.allowed_targets ?? []).slice(0, 2).map((target) => (
                          <Badge key={target} variant="outline" className="text-[10px]">
                            {templateTargetLabel(target)}
                          </Badge>
                        ))}
                        {starterFlow && (
                          <Badge variant="outline" className="text-[10px]">
                            {templateTriggerLabel(starterFlow.trigger_type)}
                          </Badge>
                        )}
                        {starterFlow?.output_type === 'docs_document' && (
                          <Badge variant="outline" className="text-[10px]">
                            Creates docs
                          </Badge>
                        )}
                      </div>
                      <div className="space-y-2 border-t border-border/50 pt-3">
                        <div className="space-y-0.5">
                          <p className="text-[10px] font-semibold uppercase tracking-wider text-muted-foreground/70">
                            Recommended trigger
                          </p>
                          {starterFlow ? (
                            <div className="min-w-0">
                              <p className="truncate text-xs font-medium text-foreground">{starterFlow.label}</p>
                              <p className="mt-0.5 text-[11px] text-muted-foreground">
                                {templateTriggerLabel(starterFlow.trigger_type)}
                                {starterFlow.default_enabled ? ' · created by default' : ''}
                              </p>
                            </div>
                          ) : (
                            <p className="text-xs text-muted-foreground">Manual runs</p>
                          )}
                        </div>
                      </div>
                    </div>
                    <ArrowRight01Icon className="mt-0.5 h-4 w-4 shrink-0 text-muted-foreground" />
                  </div>
                </button>
              );
            })}
            {agentTemplates.length === 0 && (
              <div className="rounded-xl border border-dashed border-border/60 p-5 text-sm text-muted-foreground">
                No agent templates are available in this workspace yet.
              </div>
            )}
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setTemplateDialogOpen(false)}>
              Close
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <Dialog
        open={workspaceVersionBeingRenamed !== null}
        onOpenChange={(open) => {
          if (!open) {
            setWorkspaceVersionBeingRenamed(null);
            setRenameLabelDraft('');
            setRenameDescriptionDraft('');
          }
        }}
      >
        <DialogContent className="sm:max-w-[480px]">
          <DialogHeader>
            <DialogTitle>Rename workspace version</DialogTitle>
            <DialogDescription>
              Update the label or description for this workspace version. Changes apply immediately.
            </DialogDescription>
          </DialogHeader>
          <div className="space-y-4">
            <div className="space-y-2">
              <FieldLabel htmlFor="rename-version-label">Label</FieldLabel>
              <Input
                id="rename-version-label"
                value={renameLabelDraft}
                onChange={(e) => setRenameLabelDraft(e.target.value)}
                autoFocus
              />
            </div>
            <div className="space-y-2">
              <FieldLabel htmlFor="rename-version-description">Description</FieldLabel>
              <Textarea
                id="rename-version-description"
                value={renameDescriptionDraft}
                onChange={(e) => setRenameDescriptionDraft(e.target.value)}
                rows={3}
              />
            </div>
          </div>
          <DialogFooter>
            <Button
              type="button"
              variant="outline"
              size="sm"
              onClick={() => {
                setWorkspaceVersionBeingRenamed(null);
                setRenameLabelDraft('');
                setRenameDescriptionDraft('');
              }}
            >
              Cancel
            </Button>
            <Button
              type="button"
              size="sm"
              disabled={renamingVersion || !renameLabelDraft.trim()}
              onClick={handleRenameWorkspaceVersion}
            >
              {renamingVersion ? 'Saving…' : 'Save'}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}
