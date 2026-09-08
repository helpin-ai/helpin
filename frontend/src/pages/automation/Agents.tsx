import { lazy, Suspense, useCallback, useEffect, useMemo, useState, type ReactNode } from 'react';
import { useNavigate } from '@tanstack/react-router';
import { Collapsible } from 'radix-ui';
import { formatDistanceToNow } from 'date-fns';
import { toast } from 'sonner';
import { cn } from '@/lib/utils';
import { isAgentRunLifecycleEvent } from '@/lib/agentRunRealtime';
import {
  BotIcon,
  ArrowDown01Icon,
  ArrowRight01Icon,
  ExpandIcon,
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
  MoreHorizontalIcon,
  Key01Icon,
  MessagePreview01Icon,
  SecurityCheckIcon,
} from '@/lib/icons';
import { AgentAvatar } from '@/components/agents/AgentAvatar';
import { UpgradeRequiredDialog } from '@/components/billing/UpgradeRequiredDialog';
import { AutomationShell } from '@/components/automation/AutomationShell';
import { ToolMultiSelectPopover } from '@/components/automation/ToolMultiSelectPopover';
import { QuietEmptyState, QuietIconAction, QuietPrimaryAction } from '@/components/design-system/quiet';
import { ConfirmDialog } from '@/components/pm/ConfirmDialog';
import { useAccessibleTeams } from '@/hooks/useAccessibleTeams';
import { useTitle } from '@/hooks/useTitle';
import { useWorkspaceStore } from '@/stores/workspaceStore';
import { useWorkspaceAccess, usePermissions } from '@/hooks/queries/useSession';
import { BeaconPlaybookUsage } from '@/components/crm/playbooks/BeaconPlaybookUsage';
import { useWorkspaceSettings } from '@/hooks/queries/useSettings';
import { useAutomationAgentFleet } from '@/hooks/queries/useAutomation';
import { automationService } from '@/lib/services/automationService';
import { agentService } from '@/lib/services/agentService';
import { gitService } from '@/lib/services/gitService';
import { docsService } from '@/lib/services/docsService';
import { pmWorkflowService } from '@/lib/services/pmWorkflowService';
import {
  parseNativeToolStepLimit,
} from '@/lib/agentRuntime';
import { AGENT_APPROVAL_OPTIONS, agentApprovalDescription } from '@/lib/agentApproval';
import { buildAutomationActivityPath, buildAutomationFlowsPath } from '@/lib/automationUi';
import { getAgentTokenUsageTotal } from '@/lib/agentTokenUsage';
import { CRM_AGENT_TARGET_OPTIONS } from '@/lib/agentCRMTargets';
import { AGENT_MODEL_TIER_OPTIONS, agentModelTierLabel } from '@/lib/agentModelTier';
import { buildSettingsRoutePath } from '@/lib/settingsSections';
import { getUpgradeRequiredReason, type UpgradeRequiredReason } from '@/lib/upgradeRequired';
import { ACTIVE_RUN_STATUSES, getAgentRunDisplayStatus } from '@/components/pm/agentRunConstants';
import type {
  Agent,
  AgentAnalyticsResponse,
  AgentExecutionConfig,
  AgentIconKey,
  AgentPresetDefinition,
  AgentPresetKey,
  AgentVersion,
  AgentApprovalMode,
  AgentInvocationMode,
  AgentModelProvider,
  AgentModelTier,
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
  SkillCatalogEntry,
  SkillCatalogResponse,
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
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs';
import {
  buildCustomAgentCreatePayload,
  createDefaultCustomAgentForm,
  defaultModelForAgentProvider,
} from './customAgentCreateModel';

const CodingSessionDrawer = lazy(async () => {
  const module = await import('@/components/pm/CodingSession/CodingSessionDrawer');
  return { default: module.CodingSessionDrawer };
});

const CustomAgentCreatePanel = lazy(async () => {
  const module = await import('./CustomAgentCreatePanel');
  return { default: module.CustomAgentCreatePanel };
});

const AgentRunNowDialog = lazy(async () => {
  const module = await import('./agents/AgentRunNowDialog');
  return { default: module.AgentRunNowDialog };
});

// ---------------------------------------------------------------------------
// Constants
// ---------------------------------------------------------------------------

const DEFAULT_SYSTEM_PRESET_KEY: AgentPresetKey = 'code_builder';
const DEFAULT_PRESET_MODEL = {
  provider: 'openai' as const,
  model: defaultModelForAgentProvider('openai'),
};
const PRESET_FALLBACKS: Record<AgentPresetKey, {
  label: string;
  default_role: string;
  description: string;
  runtime_kind: AgentRuntimeKind;
  default_invocation_mode: AgentInvocationMode;
  supported_modes: AgentInvocationMode[];
  provider?: AgentModelProvider;
  model?: string;
}> = {
  epic_planner: {
    ...DEFAULT_PRESET_MODEL,
    label: 'Epic Planner',
    default_role: 'Epic Planner',
    description: 'Interactive product planning for epics, PRDs, docs, and tasks.',
    runtime_kind: 'codex',
    default_invocation_mode: 'interactive',
    supported_modes: ['autonomous', 'interactive'],
  },
  task_planner: {
    ...DEFAULT_PRESET_MODEL,
    label: 'Coding Task Planner',
    default_role: 'Coding Task Planner',
    description: 'Interactive decomposition and refinement for tasks and execution plans.',
    runtime_kind: 'codex',
    default_invocation_mode: 'interactive',
    supported_modes: ['autonomous', 'interactive'],
  },
  crm_operator: {
    ...DEFAULT_PRESET_MODEL,
    label: 'Beacon',
    default_role: 'CRM Operator',
    description: 'Cross-app CRM execution across deals, contacts, docs, and support context.',
    runtime_kind: 'codex',
    default_invocation_mode: 'interactive',
    supported_modes: ['autonomous', 'interactive'],
  },
  support_agent: {
    ...DEFAULT_PRESET_MODEL,
    label: 'Support Agent',
    default_role: 'Support Agent',
    description: 'Handles support conversations and drafts replies with review controls.',
    runtime_kind: 'codex',
    default_invocation_mode: 'autonomous',
    supported_modes: ['autonomous', 'interactive'],
  },
  documentation_agent: {
    ...DEFAULT_PRESET_MODEL,
    label: 'Quill',
    default_role: 'Documentation Agent',
    description: 'Keeps internal docs, public help docs, and API docs accurate and organized.',
    runtime_kind: 'codex',
    default_invocation_mode: 'interactive',
    supported_modes: ['autonomous', 'interactive'],
  },
  marketer: {
    ...DEFAULT_PRESET_MODEL,
    label: 'Mira',
    default_role: 'Marketer',
    description: 'Plans growth work, writes marketing assets, audits funnels, and creates campaigns from workspace context.',
    runtime_kind: 'native_sdk',
    default_invocation_mode: 'interactive',
    supported_modes: ['autonomous', 'interactive'],
  },
  code_builder: {
    ...DEFAULT_PRESET_MODEL,
    label: 'Code Builder',
    default_role: 'Code Builder',
    description: 'Writes code, implements features, and fixes bugs in the repo.',
    runtime_kind: 'codex',
    default_invocation_mode: 'autonomous',
    supported_modes: ['autonomous', 'interactive'],
  },
  review_agent: {
    ...DEFAULT_PRESET_MODEL,
    label: 'QA & Code Reviewer',
    default_role: 'QA & Code Reviewer',
    description: 'Reviews work, runs tests, and checks quality without repo mutation.',
    runtime_kind: 'codex',
    default_invocation_mode: 'autonomous',
    supported_modes: ['autonomous', 'interactive'],
  },
  command_agent: {
    ...DEFAULT_PRESET_MODEL,
    label: 'Sub-agent',
    default_role: 'Sub-agent',
    description: 'Runs one delegated task with a limited tool set.',
    runtime_kind: 'codex',
    default_invocation_mode: 'autonomous',
    supported_modes: ['autonomous', 'interactive'],
  },
};

const PRESET_FALLBACK_DEFAULT: (typeof PRESET_FALLBACKS)[AgentPresetKey] = {
  ...DEFAULT_PRESET_MODEL,
  label: 'Agent',
  default_role: 'Automation Agent',
  description: '',
  runtime_kind: 'codex',
  default_invocation_mode: 'autonomous',
  supported_modes: ['autonomous', 'interactive'],
};

function presetFallback(presetKey: AgentPresetKey) {
  return PRESET_FALLBACKS[presetKey] ?? PRESET_FALLBACK_DEFAULT;
}

const INVOCATION_MODE_LABELS: Record<AgentInvocationMode, string> = {
  autonomous: 'Autonomous',
  interactive: 'Interactive',
};

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
const COMPETITIVE_INTEL_TEMPLATE_KEY = 'competitors_changelog_tracking_report';
const COMPETITIVE_INTEL_FLOW_KEY = 'competitors_changelog_scheduled';
const COMPETITIVE_INTEL_SYSTEM_PROMPT_TEMPLATE = `You are a competitors changelog tracking agent for {{target_company}}.

Configured report:
- target_company: {{target_company}}
- target_domain: {{target_domain}}
- competitors: {{competitors}}
- lookback_days: {{lookback_days}}
- destination_team_id: {{destination_team_id}}
- destination_state_id: {{destination_state_id}}
- schedule_preset: {{schedule_preset}}

Treat these configured values as already resolved and authoritative. Do not plan or perform discovery of configuration variables, workspace context, teams, stages, cadence, or lookback settings.

Use the configured competitor list when it is not empty. If no competitors are configured, discover competitors with web search and cite sources.

For each competitor, first use web_search (or the runtime's built-in web search when available) to find official changelog, release notes, product updates, blog, docs, or roadmap pages. Then use fetch_url on exact source URLs to verify page content and dates. If search is thin, use crawl_url on the competitor's official website or docs host with changelog/update keywords before marking no_public_changelog. A missing search provider is a tool limitation, not evidence that a competitor has no public changelog.

Create exactly one competitors changelog tracking report task with create_task. Pass destination_team_id directly as team_id. Pass destination_state_id directly as state_id only when it is configured; otherwise let the team default stage apply.

Raw configuration:
{{raw_configuration_json}}`;
const COMPETITIVE_INTEL_SCHEDULE_OPTIONS = [
  { value: 'daily', label: 'Daily' },
  { value: 'weekly', label: 'Weekly' },
] as const;
const DEPENDENCY_AUDITOR_TEMPLATE_KEY = 'engineering_dependency_auditor';
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
const SECURITY_TRIAGE_TEMPLATE_KEY = 'engineering_security_triage';
const SECURITY_TRIAGE_FLOW_KEY = 'engineering_security_triage_cron';
const SECURITY_TRIAGE_SYSTEM_PROMPT_TEMPLATE = `You are Sentinel, an autonomous security triage agent for the selected repository.

Configured security triage:
- scanners: {{scanners}}
- severity_threshold: {{severity_threshold}}
- include_low_info: {{include_low_info}}
- destination_team_id: {{destination_team_id}}
- destination_state_id: {{destination_state_id}}
- max_tasks: {{max_tasks}}
- schedule_preset: {{schedule_preset}}

Treat these configured values as already resolved and authoritative. Do not plan or perform discovery of configuration variables, workspace context, teams, stages, cadence, or repository selection.

Run only the configured scanners through read-only commands. Do not modify files.

Triage raw findings against repository code and configuration. Suppress false positives and non-actionable findings. Create tasks only for applicable findings at or above severity_threshold. If include_low_info is false, do not create tasks for low or informational findings.

Group related findings by root cause, such as shared scanner rule, vulnerable dependency, secret type, misconfiguration pattern, sink, or remediation path. Create at most max_tasks remediation tasks with create_task. Pass destination_team_id directly as team_id. Pass destination_state_id directly as state_id only when it is configured; otherwise let the team default stage apply.

Raw configuration:
{{raw_configuration_json}}`;
const SECURITY_TRIAGE_SCANNER_OPTIONS = [
  { value: 'semgrep', label: 'Semgrep' },
  { value: 'trivy', label: 'Trivy' },
  { value: 'gitleaks', label: 'Gitleaks' },
] as const;
const SECURITY_TRIAGE_SEVERITY_OPTIONS = [
  { value: 'critical', label: 'Critical only' },
  { value: 'high', label: 'High and critical' },
  { value: 'medium', label: 'Medium and above' },
] as const;
const SECURITY_TRIAGE_SCHEDULE_OPTIONS = COMPETITIVE_INTEL_SCHEDULE_OPTIONS;
type SecurityTriageScanner = typeof SECURITY_TRIAGE_SCANNER_OPTIONS[number]['value'];
type SecurityTriageSeverity = typeof SECURITY_TRIAGE_SEVERITY_OPTIONS[number]['value'];

// ---------------------------------------------------------------------------
// Form helpers
// ---------------------------------------------------------------------------

interface AgentFormData {
  name: string;
  icon_key: AgentIconKey;
  preset_key: AgentPresetKey;
  preset_version_key: string;
  runtime_kind: AgentRuntimeKind;
  model_tier: AgentModelTier;
  supported_modes: AgentInvocationMode[];
  provider: AgentModelProvider;
  model: string;
  reasoning_effort: AgentReasoningEffort | '';
  service_tier: AgentServiceTier | '';
  max_tool_steps: string;
  system_prompt: string;
  instruction_preamble: string;
  instruction_skills: string[];
  available_skill_keys: string[];
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

interface SecurityTriageTemplateFormData {
  repository_id: string;
  scanners: SecurityTriageScanner[];
  severity_threshold: SecurityTriageSeverity;
  include_low_info: boolean;
  schedule_preset: 'daily' | 'weekly';
  destination_team_id: string;
  destination_state_id: string;
  max_tasks: string;
}

interface TemplateDraft {
  template: AgentTemplate;
  createStarterFlow: boolean;
}

export const CUSTOM_AGENT_TARGET_OPTIONS: Array<{ value: AgentTargetType; label: string; description: string }> = [
  { value: 'task', label: 'Tasks', description: 'Run on tasks and task planning loops.' },
  { value: 'epic', label: 'Epics', description: 'Run on epics and planning loops.' },
  { value: 'sprint', label: 'Sprints', description: 'Run on sprints and sprint planning loops.' },
  { value: 'objective', label: 'Objectives', description: 'Run on objectives and outcome planning loops.' },
  ...CRM_AGENT_TARGET_OPTIONS,
  { value: 'document', label: 'Docs', description: 'Run on documents and docs-backed context.' },
  { value: 'support_conversation', label: 'Support', description: 'Run on support inbox conversations.' },
  { value: 'workspace', label: 'Workspace', description: 'Run without a fixed entity target and gather context across the workspace.' },
  { value: 'repository', label: 'Code repo', description: 'Run directly against a synced repository without requiring a linked task.' },
];

const RUN_NOW_SUPPORTED_TARGETS = new Set<AgentTargetType>([
  'task',
  'epic',
  'sprint',
  'objective',
  'repository',
  'workspace',
  'support_conversation',
  'crm_deal',
  'crm_contact',
  'crm_company',
]);

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

const CODEX_PROVIDER_MISSING_MESSAGE = 'Add OpenAI, OpenRouter, or enable Codex ChatGPT auth.';
const PROVIDER_REQUIRED_MODEL_MESSAGE = 'Select a compatible AI provider first.';

export function getAgentProviderConfigState(
  runtimeKind: AgentRuntimeKind,
  provider: AgentModelProvider,
  providerOptions: AgentModelProviderOption[],
) {
  const visibleProviderOptions = availableProvidersForRuntime(runtimeKind, providerOptions);
  const selectedProviderOption = visibleProviderOptions.find((option) => option.value === provider);
  const hasCompatibleProvider = visibleProviderOptions.length > 0;
  const modelDisabled = !selectedProviderOption;
  return {
    providerOptions: visibleProviderOptions,
    selectedProviderOption,
    hasCompatibleProvider,
    providerDisabled: !hasCompatibleProvider,
    modelDisabled,
    providerMessage: !hasCompatibleProvider && runtimeKind === 'codex'
      ? CODEX_PROVIDER_MISSING_MESSAGE
      : '',
    modelMessage: modelDisabled ? PROVIDER_REQUIRED_MODEL_MESSAGE : '',
  };
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

function presetRuntimeKindForSelection(
  presetKey: AgentPresetKey,
  presetVersionKey: string,
  presets: AgentPresetDefinition[],
): AgentRuntimeKind {
  return presetMetaForSelection(presetKey, presetVersionKey, presets)?.runtime_kind ?? presetFallback(presetKey).runtime_kind;
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

function stableJSON(value: unknown): string {
  return JSON.stringify(value ?? null);
}

function stableConfigJSON(value: unknown): string {
  if (
    value &&
    typeof value === 'object' &&
    !Array.isArray(value) &&
    Object.keys(value as Record<string, unknown>).length === 0
  ) {
    return JSON.stringify(null);
  }
  return stableJSON(value);
}

function normalizeTeamIdList(teamIds: string[]): string[] {
  const seen = new Set<string>();
  return teamIds.reduce<string[]>((result, teamId) => {
    const normalized = teamId.trim();
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

export function runNowTargetOptions(agent: Agent | null): AgentTargetType[] {
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

function agentRunNowBlocker(agent: Agent, stats?: AgentRunStats) {
  if (agent.is_system) return 'Built-in agents run from their product surface.';
  const lastRun = stats?.lastRun;
  if (lastRun && ACTIVE_RUN_STATUSES.has(lastRun.status)) {
    const displayStatus = getAgentRunDisplayStatus(lastRun);
    if (displayStatus === 'awaiting_approval') return 'Agent has a run waiting for approval.';
    if (displayStatus === 'awaiting_auth') return 'Agent has a run waiting for sign-in.';
    if (displayStatus === 'awaiting_input') return 'Agent has a run waiting for input.';
    if (lastRun.status === 'queued') return 'Agent already has a queued run.';
    if (lastRun.status === 'running') return 'Agent is already running.';
    return 'Agent already has an active run.';
  }
  if (runNowTargetOptions(agent).length === 0) {
    return 'This agent does not have a manually runnable target enabled.';
  }
  return '';
}

function createEmptyCustomForm(): AgentFormData {
  return createDefaultCustomAgentForm();
}

function deriveExecutionConfigFields(
  runtimeKind: AgentRuntimeKind,
  provider: AgentModelProvider,
  executionConfig?: AgentExecutionConfig,
): Pick<AgentFormData, 'reasoning_effort' | 'service_tier' | 'max_tool_steps'> {
  const normalizedProvider = normalizeProviderForRuntime(runtimeKind, provider);
  const reasoningEffort = runtimeKind === 'codex' ? (executionConfig?.reasoning_effort ?? '') : '';
  const serviceTier = runtimeKind === 'codex' && normalizedProvider === 'openai'
    ? (executionConfig?.service_tier ?? '')
    : '';
  const maxToolSteps = runtimeKind === 'native_sdk' && executionConfig?.max_tool_steps
    ? String(executionConfig.max_tool_steps)
    : '';
  return {
    reasoning_effort: reasoningEffort,
    service_tier: serviceTier,
    max_tool_steps: maxToolSteps,
  };
}

function buildExecutionConfigPayload(form: AgentFormData): AgentExecutionConfig | undefined {
  const config: AgentExecutionConfig = {};
  if (form.runtime_kind === 'codex') {
    if (form.reasoning_effort) {
      config.reasoning_effort = form.reasoning_effort;
    }
    if (form.provider === 'openai' && form.service_tier) {
      config.service_tier = form.service_tier;
    }
  }
  if (form.runtime_kind === 'native_sdk') {
    const maxToolSteps = parseNativeToolStepLimit(form.max_tool_steps);
    if (maxToolSteps !== undefined) {
      config.max_tool_steps = maxToolSteps;
    }
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
    return Boolean(
      agent.monthly_token_budget
      || agent.execution_config?.reasoning_effort
      || agent.execution_config?.service_tier
      || agent.execution_config?.max_tool_steps,
    );
  }
  const presetKey = fallbackPresetKey(agent);
  const presetVersionKey = agent.preset_version_key ?? fallbackPresetVersionKey(presetKey);
  return (
    agent.runtime_kind !== presetRuntimeKindForSelection(presetKey, presetVersionKey, presets) ||
    Boolean(
      agent.monthly_token_budget
      || agent.execution_config?.reasoning_effort
      || agent.execution_config?.service_tier
      || agent.execution_config?.max_tool_steps,
    )
  );
}

function buildUpdatePayload(
  form: AgentFormData,
  advancedOpen: boolean,
  presets: AgentPresetDefinition[],
  agent: Agent | null,
): UpdateAgentRequest {
  const preset = agent?.is_system ? presetMetaForSelection(form.preset_key, form.preset_version_key, presets) : null;
  const defaultRuntimeKind = agent?.is_system
    ? (preset?.runtime_kind ?? presetFallback(form.preset_key).runtime_kind)
    : 'codex';
  const provider = normalizeProviderForRuntime(form.runtime_kind, form.provider);
  const teamIds = form.teamAccessMode === 'specific_teams' ? normalizeTeamIdList(form.team_ids) : [];
  const advancedPayload: UpdateAgentRequest = advancedOpen
    ? {
        runtime_kind: form.runtime_kind,
        monthly_token_budget: form.monthly_token_budget.trim()
          ? Number.parseInt(form.monthly_token_budget, 10)
          : 0,
      }
    : form.runtime_kind !== defaultRuntimeKind
      ? { runtime_kind: form.runtime_kind }
      : {};

  if (agent?.is_system) {
    return {
      name: form.name.trim(),
      preset_key: form.preset_key,
      preset_version_key: form.preset_version_key,
      provider: provider || undefined,
      model: form.model.trim() || defaultModelForAgentProvider(provider),
      execution_config: buildExecutionConfigPayload(form),
      monthly_token_budget: form.monthly_token_budget.trim()
        ? Number.parseInt(form.monthly_token_budget, 10)
        : 0,
      ...advancedPayload,
    };
  }

  return {
    name: form.name.trim(),
    icon_key: form.icon_key,
    trigger_mode: 'manual',
    model_tier: form.model_tier,
    system_prompt: form.system_prompt.trim() || undefined,
    team_ids: teamIds,
    allowed_tools: normalizeToolList(form.allowed_tools),
    allowed_targets: normalizeTargetList(form.allowed_targets),
    skills: form.skills,
    approval_mode: form.approval_mode,
    max_concurrent_runs: form.max_concurrent_runs ? Number.parseInt(form.max_concurrent_runs, 10) : 1,
    default_invocation_mode: form.default_invocation_mode,
  };
}

function buildSystemAgentForm(agent: Agent, presets: AgentPresetDefinition[]): AgentFormData {
  const presetKey = fallbackPresetKey(agent);
  const preset = presetMetaForSelection(presetKey, agent.preset_version_key, presets);
  const runtimeKind = preset?.runtime_kind ?? agent.runtime_kind ?? presetFallback(presetKey).runtime_kind;
  const supportedModes = agent.supported_modes && agent.supported_modes.length > 0
    ? agent.supported_modes
    : (preset?.supported_modes ?? supportedModesForForm(runtimeKind));
  const defaultInvocationMode = agent.default_invocation_mode?.trim()
    ? agent.default_invocation_mode
    : (preset?.default_invocation_mode ?? presetFallback(presetKey).default_invocation_mode);
  const provider = normalizeProviderForRuntime(
    agent.runtime_kind || runtimeKind,
    agent.provider ?? preset?.provider ?? presetFallback(presetKey).provider ?? 'anthropic',
  );
  return {
    name: agent.name,
    icon_key: agent.icon_key ?? 'violet_star',
    preset_key: presetKey,
    preset_version_key: agent.preset_version_key?.trim() || preset?.version_key || fallbackPresetVersionKey(presetKey),
    runtime_kind: agent.runtime_kind || runtimeKind,
    model_tier: agent.model_tier ?? preset?.model_tier ?? 'large',
    supported_modes: supportedModes,
    provider,
    model: agent.model?.trim() || preset?.model?.trim() || defaultModelForAgentProvider(provider),
    ...deriveExecutionConfigFields(
      agent.runtime_kind || runtimeKind,
      provider,
      agent.execution_config ?? preset?.execution_config,
    ),
    system_prompt: agent.system_prompt || preset?.system_prompt || preset?.instruction_preamble?.trim() || '',
    instruction_preamble: preset?.instruction_preamble ?? '',
    instruction_skills: preset?.instruction_skills ?? [],
    available_skill_keys: preset?.available_skills ?? [],
    monthly_token_budget: agent.monthly_token_budget?.toString() ?? '',
    team_id: '',
    teamAccessMode: 'all_teams',
    team_ids: [],
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
      preset?.default_invocation_mode ?? presetFallback(presetKey).default_invocation_mode,
    ),
  };
}

function buildTemplateAgentForm(template: AgentTemplate): AgentFormData {
  const runtimeKind = template.runtime_kind || 'native_sdk';
  const provider = normalizeProviderForRuntime(runtimeKind, 'anthropic');
  return {
    name: template.name,
    icon_key: 'violet_star',
    preset_key: DEFAULT_SYSTEM_PRESET_KEY,
    preset_version_key: fallbackPresetVersionKey(DEFAULT_SYSTEM_PRESET_KEY),
    runtime_kind: runtimeKind,
    model_tier: 'large',
    supported_modes: supportedModesForForm(runtimeKind),
    provider,
    model: defaultModelForAgentProvider(provider),
    ...deriveExecutionConfigFields(runtimeKind, provider, template.execution_config),
    system_prompt: template.system_prompt ?? '',
    instruction_preamble: '',
    instruction_skills: [],
    available_skill_keys: [],
    monthly_token_budget: template.monthly_token_budget?.toString() ?? '',
    team_id: '',
    teamAccessMode: 'all_teams',
    team_ids: [],
    allowed_targets: normalizeTargetList(template.allowed_targets ?? ['task']),
    allowed_tools: normalizeToolList(template.allowed_tools ?? []),
    skills: template.skills ?? [],
    approval_mode: template.approval_mode ?? 'never',
    max_concurrent_runs: '1',
    default_invocation_mode: normalizeDefaultInvocationMode(template.default_invocation_mode, runtimeKind, 'autonomous'),
  };
}

function buildCustomAgentForm(agent: Agent): AgentFormData {
  const presetKey = fallbackPresetKey(agent);
  const runtimeKind = agent.runtime_kind;
  const provider = normalizeProviderForRuntime(runtimeKind, agent.provider ?? 'anthropic');
  return {
    name: agent.name,
    icon_key: agent.icon_key ?? 'violet_star',
    preset_key: presetKey,
    preset_version_key: fallbackPresetVersionKey(presetKey),
    runtime_kind: runtimeKind,
    model_tier: agent.model_tier ?? 'large',
    supported_modes: supportedModesForForm(runtimeKind),
    provider,
    model: agent.model?.trim() || defaultModelForAgentProvider(provider),
    ...deriveExecutionConfigFields(
      runtimeKind,
      provider,
      agent.execution_config,
    ),
    system_prompt: agent.system_prompt ?? '',
    instruction_preamble: '',
    instruction_skills: [],
    available_skill_keys: [],
    monthly_token_budget: agent.monthly_token_budget?.toString() ?? '',
    team_id: agent.team_id ?? '',
    teamAccessMode: agent.team_ids?.length || agent.team_id ? 'specific_teams' : 'all_teams',
    team_ids: agent.team_ids?.length ? agent.team_ids : agent.team_id ? [agent.team_id] : [],
    allowed_targets: normalizeTargetList(agent.allowed_targets as AgentTargetType[]),
    allowed_tools: normalizeToolList(agent.allowed_tools),
    skills: agent.skills ?? [],
    approval_mode: agent.approval_mode ?? 'preset_default',
    max_concurrent_runs: agent.max_concurrent_runs?.toString() ?? '1',
    default_invocation_mode: normalizeDefaultInvocationMode(agent.default_invocation_mode, runtimeKind),
  };
}

function buildCustomAgentVersionForm(agent: Agent, version: AgentVersion): AgentFormData {
  const runtimeKind = version.runtime_kind || agent.runtime_kind || 'native_sdk';
  const provider = normalizeProviderForRuntime(runtimeKind, version.provider ?? agent.provider ?? 'anthropic');
  return {
    ...buildCustomAgentForm(agent),
    runtime_kind: runtimeKind,
    model_tier: version.model_tier ?? agent.model_tier ?? 'large',
    supported_modes: version.supported_modes?.length ? version.supported_modes : supportedModesForForm(runtimeKind),
    provider,
    model: version.model?.trim() || defaultModelForAgentProvider(provider),
    ...deriveExecutionConfigFields(runtimeKind, provider, version.execution_config),
    system_prompt: version.system_prompt ?? '',
    instruction_preamble: '',
    instruction_skills: [],
    available_skill_keys: [],
    allowed_targets: normalizeTargetList(version.allowed_targets ?? ['task']),
    allowed_tools: normalizeToolList(version.allowed_tools ?? []),
    skills: version.skills ?? [],
    default_invocation_mode: normalizeDefaultInvocationMode(version.default_invocation_mode, runtimeKind),
  };
}

function comparableCustomAgentForm(form: AgentFormData) {
  const provider = normalizeProviderForRuntime(form.runtime_kind, form.provider);
  const teamIDs = form.teamAccessMode === 'specific_teams'
    ? normalizeTeamIdList(form.team_ids.length > 0 ? form.team_ids : form.team_id ? [form.team_id] : [])
    : [];
  return {
    name: form.name.trim(),
    model_tier: form.model_tier,
    icon_key: form.icon_key,
    runtime_kind: form.runtime_kind,
    provider,
    model: form.model.trim(),
    reasoning_effort: form.runtime_kind === 'codex' ? form.reasoning_effort : '',
    service_tier: form.runtime_kind === 'codex' && provider === 'openai' ? form.service_tier : '',
    max_tool_steps: form.runtime_kind === 'native_sdk' ? parseNativeToolStepLimit(form.max_tool_steps) ?? 0 : 0,
    system_prompt: form.system_prompt.trim(),
    monthly_token_budget: normalizeTokenBudgetFormValue(form.monthly_token_budget),
    team_ids: teamIDs,
    allowed_targets: normalizeTargetList(form.allowed_targets).sort(),
    allowed_tools: normalizeToolList(form.allowed_tools).sort(),
    skills: [...form.skills]
      .map((skill) => ({
        key: skill.key,
        skill_id: skill.skill_id ?? '',
        version_key: skill.version_key ?? '',
      }))
      .sort((a, b) => `${a.key}:${a.skill_id}:${a.version_key}`.localeCompare(`${b.key}:${b.skill_id}:${b.version_key}`)),
    approval_mode: form.approval_mode,
    max_concurrent_runs: normalizePositiveIntegerFormValue(form.max_concurrent_runs, 1),
    default_invocation_mode: form.default_invocation_mode,
  };
}

function agentSkillIdentity(skill: Pick<SkillCatalogEntry, 'id' | 'key'> | AgentSkillRef) {
  if ('id' in skill && skill.id) return skill.id;
  if ('skill_id' in skill && skill.skill_id) return skill.skill_id;
  return skill.key;
}

function agentSkillDisplayName(skill: Pick<SkillCatalogEntry, 'title' | 'key'> | undefined, fallbackKey: string) {
  return skill?.title?.trim() || fallbackKey;
}

function customAgentFormDirtyKey(form: AgentFormData) {
  return JSON.stringify(comparableCustomAgentForm(form));
}

function normalizeTokenBudgetFormValue(value: string) {
  const parsed = Number.parseInt(value, 10);
  return Number.isFinite(parsed) && parsed > 0 ? parsed : 0;
}

const MONTHLY_TOKEN_LIMIT_PRESETS = [
  { label: 'No limit', value: 0 },
  { label: '1M', value: 1_000_000 },
  { label: '5M', value: 5_000_000 },
  { label: '10M', value: 10_000_000 },
  { label: '25M', value: 25_000_000 },
] as const;

function formatRate(numerator: number, denominator: number) {
  if (denominator <= 0) return '—';
  return `${Math.round((numerator / denominator) * 100)}%`;
}

const AGENT_RUN_TARGET_LABELS: Partial<Record<AgentTargetType, string>> = {
  task: 'Task',
  support_conversation: 'Support conversation',
  support_coverage_gap: 'Coverage gap',
  epic: 'Epic',
  sprint: 'Sprint',
  objective: 'Objective',
  document: 'Document',
  crm_deal: 'Deal',
  crm_contact: 'Contact',
  crm_company: 'Company',
  repository: 'Repository',
  workspace: 'Workspace',
};

function titleCaseWords(value: string) {
  return value
    .replace(/_/g, ' ')
    .split(' ')
    .filter(Boolean)
    .map((word) => word.charAt(0).toUpperCase() + word.slice(1))
    .join(' ');
}

function agentRunTargetTypeLabel(targetType: string) {
  return AGENT_RUN_TARGET_LABELS[targetType as AgentTargetType] ?? titleCaseWords(targetType);
}

function readNestedString(value: unknown, path: string[]) {
  let current = value;
  for (const key of path) {
    if (!current || typeof current !== 'object') return '';
    current = (current as Record<string, unknown>)[key];
  }
  return typeof current === 'string' ? current.trim() : '';
}

function agentRunTriggerLabel(run: AgentRun) {
  const trigger = run.input?.trigger;
  const triggerType = readNestedString(trigger, ['trigger_type']);
  const source = readNestedString(trigger, ['source']);

  if (triggerType === 'task.state_entered') return 'State changed';
  if (triggerType === 'agent_run.approved') return 'Run approved';
  if (triggerType === 'cron' || source === 'cron') return 'Scheduled run';
  if (triggerType === 'command_bar' || source === 'command_bar') return 'Command bar';
  if (triggerType === 'manual' || source === 'manual') return 'Manual run';
  if (triggerType === 'github.pull_request_merged' || triggerType === 'gitlab.merge_request_merged') return 'PR merged';
  if (triggerType === 'github.pull_request_opened' || triggerType === 'gitlab.merge_request_opened') return 'PR opened';
  if (triggerType === 'github.pull_request_closed' || triggerType === 'gitlab.merge_request_closed') return 'PR closed';
  if (triggerType === 'github.pull_request_review_requested') return 'PR review requested';
  if (triggerType === 'github.push' || triggerType === 'gitlab.push') return 'Push';
  if (triggerType === 'github.release_published' || triggerType === 'gitlab.release_published') return 'Release published';
  if (triggerType === 'github.check_suite_completed') return 'Check suite completed';
  if (triggerType === 'gitlab.pipeline_completed') return 'Pipeline completed';
  if (source === 'automation_rule') return 'Automation flow';
  return run.invocation_mode === 'autonomous' ? 'Autonomous run' : 'Interactive run';
}

function compactUniqueText(parts: string[]) {
  const seen = new Set<string>();
  const result: string[] = [];
  for (const part of parts) {
    const normalized = part.trim();
    const key = normalized.toLowerCase();
    if (!normalized || seen.has(key)) continue;
    seen.add(key);
    result.push(normalized);
  }
  return result;
}

export function getAgentRecentRunSummary(run: AgentRun) {
  const targetTypeLabel = agentRunTargetTypeLabel(run.target_type);
  const taskKey = run.target_info?.task_key?.trim();
  const targetTitle = run.target_info?.title?.trim();
  const repoTitle = run.repo_full_name?.trim();
  const fallbackTitle = `${targetTypeLabel} run`;
  const title = taskKey && targetTitle
    ? `${taskKey} · ${targetTitle}`
    : taskKey || targetTitle || repoTitle || fallbackTitle;
  const branch = run.base_branch?.trim() || run.working_branch?.trim();
  const subtitleParts = compactUniqueText([
    title.toLowerCase() === targetTypeLabel.toLowerCase() ? '' : targetTypeLabel,
    agentRunTriggerLabel(run),
    branch && run.target_type === 'repository' ? branch : '',
  ]);
  const subtitle = subtitleParts.join(' · ') || agentRunTriggerLabel(run);

  return { title, subtitle };
}

function normalizePositiveIntegerFormValue(value: string, fallback: number) {
  const parsed = Number.parseInt(value, 10);
  return Number.isFinite(parsed) && parsed > 0 ? parsed : fallback;
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

function renderSecurityTriageSystemPrompt(templatePrompt: string | undefined, form: SecurityTriageTemplateFormData) {
  const destinationState = form.destination_state_id === NONE_OPTION_VALUE || !form.destination_state_id
    ? "not configured; use the team's default stage"
    : form.destination_state_id;
  const maxTasks = Number.parseInt(form.max_tasks, 10);
  const rawConfig = {
    scanners: form.scanners,
    severity_threshold: form.severity_threshold,
    include_low_info: form.include_low_info,
    schedule_preset: form.schedule_preset,
    destination_team_id: form.destination_team_id,
    destination_state_id: form.destination_state_id === NONE_OPTION_VALUE ? undefined : form.destination_state_id,
    max_tasks: maxTasks,
  };
  const replacements: Record<string, string> = {
    scanners: form.scanners.join(', '),
    severity_threshold: form.severity_threshold,
    include_low_info: String(form.include_low_info),
    destination_team_id: form.destination_team_id,
    destination_state_id: destinationState,
    schedule_preset: form.schedule_preset,
    max_tasks: form.max_tasks,
    raw_configuration_json: `\`\`\`json\n${JSON.stringify(rawConfig, null, 2)}\n\`\`\``,
  };
  let rendered = (templatePrompt?.trim() || SECURITY_TRIAGE_SYSTEM_PROMPT_TEMPLATE).trim();
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
  const params = new URLSearchParams();
  if (agentID) params.set('agent_id', agentID);
  if (item?.execution_search?.binding_id) params.set('binding_id', item.execution_search.binding_id);
  if (item?.execution_search?.trigger_type) params.set('trigger_type', item.execution_search.trigger_type);
  if (item?.execution_search?.source) params.set('source', item.execution_search.source);
  if (item?.execution_search?.reference_id) params.set('reference_id', item.execution_search.reference_id);
  if (item?.execution_search?.status) params.set('status', item.execution_search.status);
  const query = params.toString();
  return `${base}${query ? `?${query}` : ''}#trigger-executions`;
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
  recentTokens: number;
  lastRun?: AgentRun;
  attentionRun?: AgentRun;
  attentionRunCount: number;
  lastFiveStatuses: AgentRun['status'][];
  recentRunItems: AgentRun[];
}

function trimSummaryText(value?: string, fallback = 'No description yet.') {
  const normalized = value?.replace(/\s+/g, ' ').trim();
  if (!normalized) return fallback;
  const sentence = normalized.split(/(?<=[.!?])\s+/)[0] ?? normalized;
  return sentence.length > 160 ? `${sentence.slice(0, 157)}...` : sentence;
}

function agentRoleLabel(agent: Agent, presets: AgentPresetDefinition[]) {
  if (agent.is_system) {
    const presetKey = fallbackPresetKey(agent);
    if (presetKey === 'task_planner') return 'Coding Task Planner';
    if (presetKey === 'review_agent') return 'QA & Code Reviewer';
    const preset = presetMetaForSelection(presetKey, agent.preset_version_key, presets);
    return preset?.default_role ?? presetMetaForKey(presetKey, presets)?.default_role ?? presetFallback(presetKey).default_role;
  }
  return 'Custom agent';
}

function agentPurpose(agent: Agent, presets: AgentPresetDefinition[]) {
  if (agent.is_system) {
    const preset = presetMetaForSelection(
      fallbackPresetKey(agent),
      agent.preset_version_key,
      presets,
    );
    return trimSummaryText(
      preset?.description ?? presetFallback(fallbackPresetKey(agent)).description,
      'Built-in workspace agent.',
    );
  }
  return 'Custom agent';
}

// Legacy blank models resolve to the provider default and are not an error.
function needsModelConfiguration() {
  return false;
}

function isUnusedAgent(stats?: AgentRunStats) {
  return (stats?.recentRuns ?? 0) === 0;
}

function isFailingAgent(stats?: AgentRunStats) {
  return Boolean(stats?.recentFailed) || stats?.lastRun?.status === 'failed';
}

function needsAttention(stats?: AgentRunStats) {
  return needsModelConfiguration() || Boolean(stats?.attentionRunCount) || isUnusedAgent(stats) || isFailingAgent(stats);
}

export function sortAgentsForDisplay(agents: Agent[]) {
  return [...agents].sort((left, right) => {
    if (left.is_system !== right.is_system) return left.is_system ? -1 : 1;
    return left.name.localeCompare(right.name);
  });
}

function formatLastRunTime(run?: AgentRun) {
  if (!run) return 'Never';
  const date = run.completed_at || run.started_at || run.created_at;
  return formatDistanceToNow(new Date(date), { addSuffix: true });
}

function formatAgentTimestamp(value?: string) {
  if (!value) return '—';
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return '—';
  return formatDistanceToNow(date, { addSuffix: true });
}

function formatAgentTimestampTitle(value?: string) {
  if (!value) return undefined;
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return undefined;
  return date.toLocaleString();
}

function lastRunStatusLabel(run?: AgentRun) {
  if (!run) return 'Never run';
  const displayStatus = getAgentRunDisplayStatus(run);
  switch (displayStatus) {
    case 'completed':
      return 'Completed';
    case 'failed':
      return 'Failed';
    case 'awaiting_approval':
      return 'Needs approval';
    case 'awaiting_auth':
      return 'Needs sign-in';
    case 'awaiting_input':
      return 'Needs input';
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

function attentionRunLabel(run: AgentRun) {
  const displayStatus = getAgentRunDisplayStatus(run);
  if (displayStatus === 'awaiting_approval') return 'Needs approval';
  if (displayStatus === 'awaiting_auth') return 'Needs sign-in';
  return 'Needs input';
}

function AttentionRunBadge({
  stats,
  onOpenRun,
}: {
  stats?: AgentRunStats;
  onOpenRun: (runId: string) => void;
}) {
  const run = stats?.attentionRun;
  if (!run) return null;

  const displayStatus = getAgentRunDisplayStatus(run);
  const Icon = displayStatus === 'awaiting_approval'
    ? SecurityCheckIcon
    : displayStatus === 'awaiting_auth'
      ? Key01Icon
      : MessagePreview01Icon;
  const label = attentionRunLabel(run);
  const badgeLabel = stats.attentionRunCount > 1 ? `${label} +${stats.attentionRunCount - 1}` : label;

  return (
    <Badge
      asChild
      variant="outline"
      className="border-amber-500/40 bg-amber-500/10 text-[10px] text-amber-700 hover:bg-amber-500/15 dark:text-amber-400"
    >
      <button
        type="button"
        title={`${label}. Open run.`}
        onClick={(event) => {
          event.stopPropagation();
          onOpenRun(run.id);
        }}
      >
        <Icon className="h-3 w-3" />
        {badgeLabel}
      </button>
    </Badge>
  );
}

function AgentStatusBadge({
  stats,
  onOpenRun,
}: {
  stats?: AgentRunStats;
  onOpenRun: (runId: string) => void;
}) {
  if (stats?.attentionRun) {
    return <AttentionRunBadge stats={stats} onOpenRun={onOpenRun} />;
  }

  if (stats?.lastRun) {
    const label = lastRunStatusLabel(stats.lastRun);
    return (
      <Badge
        asChild
        variant="outline"
        className={cn('h-5 px-1.5 text-[10px]', lastRunStatusClass(stats.lastRun))}
      >
        <button
          type="button"
          title={`${label}. Open latest run.`}
          onClick={(event) => {
            event.stopPropagation();
            onOpenRun(stats.lastRun!.id);
          }}
        >
          {label}
        </button>
      </Badge>
    );
  }

  return (
    <Badge variant="outline" className="h-5 border-border/70 bg-muted/30 px-1.5 text-[10px] text-muted-foreground">
      Idle
    </Badge>
  );
}

function FlowRefs({
  usage,
  workspaceSlug,
  agentId,
}: {
  usage?: AgentTriggerUsageSummary | null;
  workspaceSlug?: string;
  agentId: string;
}) {
  const items = usage?.items ?? [];
  const count = items.length;
  const flowsPath = buildAutomationFlowsPath(workspaceSlug, { agent_id: agentId });
  if (items.length === 0) {
    return (
      <a
        href={flowsPath}
        className="inline-flex w-fit rounded-md px-1.5 py-0.5 text-xs text-muted-foreground hover:bg-muted hover:text-foreground"
        onClick={(event) => event.stopPropagation()}
      >
        0 flows
      </a>
    );
  }

  return (
    <Tooltip>
      <TooltipTrigger asChild>
        <a
          href={flowsPath}
          className="inline-flex w-fit items-center rounded-md px-1.5 py-0.5 text-xs font-medium text-foreground hover:bg-muted"
          onClick={(event) => event.stopPropagation()}
        >
          {count} {count === 1 ? 'flow' : 'flows'}
        </a>
      </TooltipTrigger>
      <TooltipContent
        side="top"
        align="start"
        className="z-[1000] max-w-64 items-start py-2"
      >
        <div className="space-y-1">
          <div className="space-y-0.5">
            {items.slice(0, 8).map((item) => (
              <p key={item.id} className="truncate text-xs text-background/90">
                {item.title}
              </p>
            ))}
          </div>
          {items.length > 8 ? (
            <p className="text-[11px] text-background/70">+{items.length - 8} more</p>
          ) : null}
        </div>
      </TooltipContent>
    </Tooltip>
  );
}

function agentFlowUsages(usage?: AgentTriggerUsageSummary | null) {
  return (usage?.items ?? []).filter((item) => item.reference_type === 'automation_rule' && item.reference_id);
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

type AgentAnalyticsRange = '7d' | '30d' | '90d' | '12m';

const AGENT_ANALYTICS_RANGES: Array<{ value: AgentAnalyticsRange; label: string }> = [
  { value: '7d', label: '7d' },
  { value: '30d', label: '30d' },
  { value: '90d', label: '90d' },
  { value: '12m', label: '12m' },
];

export function getAgentAnalyticsSummary(analytics?: AgentAnalyticsResponse | null, fallbackRange = '30d') {
  const series = analytics?.series ?? [];
  const runs = series.reduce((sum, point) => sum + point.runs, 0);
  const completed = series.reduce((sum, point) => sum + point.completed, 0);
  const failed = series.reduce((sum, point) => sum + point.failed, 0);
  const needsAttention = series.reduce((sum, point) => sum + point.needs_attention, 0);
  const tokens = series.reduce((sum, point) => sum + point.tokens, 0);

  return {
    rangeLabel: analytics?.range || fallbackRange,
    runs,
    completed,
    failed,
    needsAttention,
    tokens,
    successRate: formatRate(completed, runs),
    avgTokensPerRun: runs > 0 ? Math.round(tokens / runs) : 0,
  };
}

export function getAgentTokenUsageSummary(agent?: Agent | null) {
  const monthlyTokensUsed = agent?.tokens_used_this_month ?? 0;
  const recordedTotalTokens = agent?.tokens_used_total ?? 0;
  const totalTokensUsed = Math.max(recordedTotalTokens, monthlyTokensUsed);

  return {
    monthlyTokensUsed,
    totalTokensUsed,
    totalTokensAdjusted: totalTokensUsed !== recordedTotalTokens,
  };
}

function AgentRunsTrendChart({ analytics }: { analytics?: AgentAnalyticsResponse | null }) {
  const series = analytics?.series ?? [];
  const width = 720;
  const height = 220;
  const padX = 34;
  const padY = 24;
  const plotWidth = width - padX * 2;
  const plotHeight = height - padY * 2;
  const maxValue = Math.max(1, ...series.flatMap((point) => [point.runs, point.completed, point.failed, point.needs_attention]));
  const xFor = (idx: number) => padX + (series.length <= 1 ? plotWidth / 2 : (idx / (series.length - 1)) * plotWidth);
  const yFor = (value: number) => padY + plotHeight - (value / maxValue) * plotHeight;
  const pathFor = (key: 'runs' | 'completed' | 'failed' | 'needs_attention') => {
    if (series.length === 0) return '';
    return series
      .map((point, idx) => `${idx === 0 ? 'M' : 'L'} ${xFor(idx).toFixed(1)} ${yFor(point[key]).toFixed(1)}`)
      .join(' ');
  };
  const totalRuns = series.reduce((sum, point) => sum + point.runs, 0);
  const axisLabels = series.length > 0
    ? [series[0], series[Math.floor(series.length / 2)], series[series.length - 1]].filter(Boolean)
    : [];

  if (series.length === 0 || totalRuns === 0) {
    return (
      <div className="flex h-56 items-center justify-center rounded-lg border border-dashed border-border/70 bg-muted/20 text-sm text-muted-foreground">
        No runs in this range.
      </div>
    );
  }

  return (
    <div className="overflow-hidden rounded-lg border border-border/60 bg-card">
      <div className="flex flex-wrap items-center gap-3 border-b border-border/60 px-4 py-3 text-xs">
        <span className="inline-flex items-center gap-1.5"><span className="h-2 w-2 rounded-full bg-foreground" />Runs</span>
        <span className="inline-flex items-center gap-1.5"><span className="h-2 w-2 rounded-full bg-emerald-500" />Completed</span>
        <span className="inline-flex items-center gap-1.5"><span className="h-2 w-2 rounded-full bg-rose-500" />Failed</span>
        <span className="inline-flex items-center gap-1.5"><span className="h-2 w-2 rounded-full bg-amber-500" />Needs attention</span>
      </div>
      <div className="p-4">
        <svg viewBox={`0 0 ${width} ${height}`} role="img" aria-label="Agent run trends" className="h-60 w-full">
          {[0, 0.5, 1].map((ratio) => (
            <line
              key={ratio}
              x1={padX}
              x2={width - padX}
              y1={padY + plotHeight * ratio}
              y2={padY + plotHeight * ratio}
              className="stroke-border/70"
              strokeDasharray={ratio === 1 ? undefined : '4 6'}
            />
          ))}
          <path d={pathFor('runs')} fill="none" className="stroke-foreground" strokeWidth="2.5" strokeLinecap="round" strokeLinejoin="round" />
          <path d={pathFor('completed')} fill="none" className="stroke-emerald-500" strokeWidth="2.5" strokeLinecap="round" strokeLinejoin="round" />
          <path d={pathFor('failed')} fill="none" className="stroke-rose-500" strokeWidth="2.5" strokeLinecap="round" strokeLinejoin="round" />
          <path d={pathFor('needs_attention')} fill="none" className="stroke-amber-500" strokeWidth="2.5" strokeLinecap="round" strokeLinejoin="round" />
          {axisLabels.map((point) => {
            const idx = series.indexOf(point);
            return (
              <text key={`${point.period}-${idx}`} x={xFor(idx)} y={height - 4} textAnchor={idx === 0 ? 'start' : idx === series.length - 1 ? 'end' : 'middle'} className="fill-muted-foreground text-[11px]">
                {point.period}
              </text>
            );
          })}
          <text x={padX} y={14} className="fill-muted-foreground text-[11px]">{maxValue.toLocaleString()}</text>
          <text x={padX} y={height - 22} className="fill-muted-foreground text-[11px]">0</text>
        </svg>
      </div>
    </div>
  );
}

function AgentCard({
  agent,
  stats,
  usage,
  presets,
  onOpen,
  onOpenRun,
  onRunNow,
  onDelete,
  canEdit,
  workspaceSlug,
}: {
  agent: Agent;
  stats?: AgentRunStats;
  usage?: AgentTriggerUsageSummary | null;
  presets: AgentPresetDefinition[];
  onOpen: (agent: Agent) => void;
  onOpenRun: (runId: string) => void;
  onRunNow: (agent: Agent) => void;
  onDelete: (agent: Agent) => void;
  canEdit: boolean;
  workspaceSlug?: string;
}) {
  const role = agentRoleLabel(agent, presets);
  const purpose = agentPurpose(agent, presets);
  const attention = needsAttention(stats);
  const runNowBlocker = agentRunNowBlocker(agent, stats);
  const showRunNow = canEdit && !agent.is_system;
  const canRunNow = showRunNow && !runNowBlocker;

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
              <h3 className="truncate text-sm font-semibold">{agent.name}</h3>
              <AgentStatusBadge stats={stats} onOpenRun={onOpenRun} />
              {agent.is_system ? <Badge variant="outline" className="text-[10px]">System</Badge> : null}
            </div>
            <p className="text-xs text-muted-foreground">{role}</p>
            <p className="line-clamp-2 text-sm text-muted-foreground">{purpose}</p>
          </div>
          {canEdit ? (
            <DropdownMenu>
              <DropdownMenuTrigger asChild>
                <Button
                  type="button"
                  variant="ghost"
                  size="icon"
                  className="h-8 w-8 shrink-0 text-muted-foreground"
                  aria-label="More agent actions"
                  onClick={(event) => event.stopPropagation()}
                >
                  <MoreHorizontalIcon className="h-4 w-4" />
                </Button>
              </DropdownMenuTrigger>
              <DropdownMenuContent align="end" onClick={(event) => event.stopPropagation()}>
                {showRunNow ? (
                  <DropdownMenuItem
                    disabled={!canRunNow}
                    title={runNowBlocker || undefined}
                    onClick={() => onRunNow(agent)}
                  >
                    Run now
                  </DropdownMenuItem>
                ) : null}
                <DropdownMenuItem asChild>
                  <a href={buildAutomationActivityPath(workspaceSlug, { page: 1, agent_id: agent.id })}>View runs</a>
                </DropdownMenuItem>
                {stats?.lastRun ? (
                  <DropdownMenuItem onClick={() => onOpenRun(stats.lastRun!.id)}>
                    Open latest run
                  </DropdownMenuItem>
                ) : null}
                <DropdownMenuItem onClick={() => onOpen(agent)}>
                  Edit
                </DropdownMenuItem>
                {!agent.is_system ? (
                  <DropdownMenuItem
                    className="text-destructive focus:text-destructive"
                    onClick={() => onDelete(agent)}
                  >
                    Delete
                  </DropdownMenuItem>
                ) : null}
              </DropdownMenuContent>
            </DropdownMenu>
          ) : null}
        </div>
      </CardHeader>
      <CardContent className="space-y-4 pt-0">
        <div className="grid gap-3 sm:grid-cols-2">
          <div className="space-y-1">
            <p className="text-[10px] font-medium uppercase tracking-[0.14em] text-muted-foreground">Model size</p>
            <p className="text-sm text-muted-foreground">{agentModelTierLabel(agent.model_tier)}</p>
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
                <span className="text-muted-foreground"> runs in 7d</span>
              </span>
            </div>
          </div>
          {stats?.lastRun ? (
            <p className="font-mono text-[11px] text-muted-foreground">Last run {formatLastRunTime(stats.lastRun)}</p>
          ) : (
            <p className="text-[11px] text-muted-foreground">No runs in the last 7 days.</p>
          )}
        </div>

        <div className="space-y-1">
          <p className="text-[10px] font-medium uppercase tracking-[0.14em] text-muted-foreground">Used in flows</p>
          <FlowRefs usage={usage} workspaceSlug={workspaceSlug} agentId={agent.id} />
        </div>

        {canEdit ? (
          <div className="flex justify-end gap-2">
            {showRunNow ? (
              <Tooltip>
                <TooltipTrigger asChild>
                  <span
                    className={cn(
                      'inline-flex opacity-0 transition-opacity group-hover:opacity-100 group-focus-within:opacity-100 focus-within:opacity-100',
                      canRunNow ? 'cursor-pointer' : 'cursor-not-allowed',
                    )}
                    onClick={(event) => event.stopPropagation()}
                  >
                    <Button
                      type="button"
                      variant="outline"
                      size="sm"
                      disabled={!canRunNow}
                      className={cn(
                        'h-8 px-2.5 text-xs',
                        canRunNow ? 'cursor-pointer' : 'pointer-events-none cursor-not-allowed',
                      )}
                      onClick={(event) => {
                        event.stopPropagation();
                        onRunNow(agent);
                      }}
                    >
                      <ZapIcon className="mr-1.5 h-3.5 w-3.5" />
                      Run now
                    </Button>
                  </span>
                </TooltipTrigger>
                {runNowBlocker ? <TooltipContent>{runNowBlocker}</TooltipContent> : null}
              </Tooltip>
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

const AGENTS_LIST_GRID_CLASS =
  'lg:grid-cols-[minmax(12rem,1.6fr)_17rem_6.5rem_8.25rem_4rem_2.5rem]';

export function AgentsListTable({ children }: { children: ReactNode }) {
  return (
    <div className="overflow-x-auto">
      <div className="min-w-[64rem]">{children}</div>
    </div>
  );
}

export function AgentsListHeader() {
  return (
    <div
      className={cn(
        'hidden items-center gap-4 border-b border-border px-[14px] pb-[9px] text-xs font-medium uppercase tracking-wide text-muted-foreground lg:grid',
        AGENTS_LIST_GRID_CLASS,
      )}
    >
      <div>Agent</div>
      <div>Config</div>
      <div>Runs · 7d</div>
      <div>Last run</div>
      <div className="whitespace-nowrap">Used in flows</div>
      <div />
    </div>
  );
}

export function AgentActions({
  agent,
  stats,
  workspaceSlug,
  onOpen,
  onOpenRun,
  onRunNow,
  onDelete,
  canEdit,
}: {
  agent: Agent;
  stats?: AgentRunStats;
  workspaceSlug?: string;
  onOpen: (agent: Agent) => void;
  onOpenRun: (runId: string) => void;
  onRunNow: (agent: Agent) => void;
  onDelete: (agent: Agent) => void;
  canEdit: boolean;
}) {
  const runNowBlocker = agentRunNowBlocker(agent, stats);
  const showRunNow = canEdit && !agent.is_system;
  const canRunNow = showRunNow && !runNowBlocker;
  const runsPath = buildAutomationActivityPath(workspaceSlug, { page: 1, agent_id: agent.id });

  return (
    <div className="flex items-center justify-end gap-1">
      <DropdownMenu>
        <DropdownMenuTrigger asChild>
          <Button
            type="button"
            variant="ghost"
            size="icon"
            className="h-8 w-8 text-muted-foreground"
            aria-label="More agent actions"
            onClick={(event) => event.stopPropagation()}
          >
            <MoreHorizontalIcon className="h-4 w-4" />
          </Button>
        </DropdownMenuTrigger>
        <DropdownMenuContent
          align="end"
          onClick={(event) => event.stopPropagation()}
        >
          {showRunNow ? (
            <DropdownMenuItem
              disabled={!canRunNow}
              title={runNowBlocker || undefined}
              onClick={() => onRunNow(agent)}
            >
              Run now
            </DropdownMenuItem>
          ) : null}
          <DropdownMenuItem asChild>
            <a href={runsPath}>View runs</a>
          </DropdownMenuItem>
          {canEdit ? (
            <DropdownMenuItem onClick={() => onOpen(agent)}>
              Edit
            </DropdownMenuItem>
          ) : null}
          {stats?.lastRun ? (
            <DropdownMenuItem onClick={() => onOpenRun(stats.lastRun!.id)}>
              Open latest run
            </DropdownMenuItem>
          ) : null}
          {canEdit && !agent.is_system ? (
            <DropdownMenuItem
              className="text-destructive focus:text-destructive"
              onClick={() => onDelete(agent)}
            >
              Delete
            </DropdownMenuItem>
          ) : null}
        </DropdownMenuContent>
      </DropdownMenu>
    </div>
  );
}

export function canEditWorkspacePresetVersionDescription(preset: Pick<AgentPresetDefinition, 'id' | 'scope'> | null | undefined) {
  return preset?.scope === 'workspace' && Boolean(preset.id);
}

export function getVersionToolEditingState({
  versionReadOnly,
}: {
  versionReadOnly: boolean;
  runtimeKind: AgentRuntimeKind;
}) {
  if (versionReadOnly) {
    return {
      canEdit: false,
      disabledReason: 'Duplicate this version to edit tools.',
    };
  }
  return {
    canEdit: true,
    disabledReason: '',
  };
}

function versionReadOnlyHelperText(area: string) {
  return `Duplicate this version to edit ${area}.`;
}

export function AgentRow({
  agent,
  stats,
  usage,
  presets,
  onOpen,
  onOpenRun,
  onRunNow,
  onDelete,
  canEdit,
  workspaceSlug,
}: {
  agent: Agent;
  stats?: AgentRunStats;
  usage?: AgentTriggerUsageSummary | null;
  presets: AgentPresetDefinition[];
  onOpen: (agent: Agent) => void;
  onOpenRun: (runId: string) => void;
  onRunNow: (agent: Agent) => void;
  onDelete: (agent: Agent) => void;
  canEdit: boolean;
  workspaceSlug?: string;
}) {
  const role = agentRoleLabel(agent, presets);
  const purpose = agentPurpose(agent, presets);
  const invocationLabel = INVOCATION_MODE_LABELS[agent.default_invocation_mode];

  return (
    <div
      role="button"
      tabIndex={0}
      aria-label={`Open ${agent.name}`}
      className={cn(
        'group grid cursor-pointer items-center gap-4 border-b border-border/60 px-[14px] py-[13px] outline-none transition-colors duration-100 hover:bg-muted/50 focus-visible:bg-muted/50 focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-ring',
        AGENTS_LIST_GRID_CLASS,
      )}
      onClick={() => onOpen(agent)}
      onKeyDown={(event) => {
        if (event.target !== event.currentTarget || (event.key !== 'Enter' && event.key !== ' ')) return;
        event.preventDefault();
        onOpen(agent);
      }}
    >
      <div className="min-w-0" title={purpose}>
        <div className="flex items-center gap-3">
          <AgentAvatar agent={agent} className="h-8 w-8 rounded-none border-0 bg-transparent shadow-none" genericBare />
          <div className="min-w-0 flex-1">
            <div className="flex flex-wrap items-center gap-2">
              <span className="truncate text-sm font-medium">{agent.name}</span>
            </div>
            <div className="mt-0.5 flex min-w-0 flex-wrap items-center gap-1.5">
              <span className="truncate text-xs text-muted-foreground">{role}</span>
              {agent.is_system ? <Badge variant="outline" className="h-5 px-1.5 text-[10px]">System</Badge> : null}
            </div>
          </div>
        </div>
      </div>

      <div className="min-w-0 space-y-1">
        <p className="text-[10px] font-medium uppercase tracking-[0.14em] text-muted-foreground lg:hidden">Config</p>
        <p className="truncate text-sm font-medium text-foreground">{agentModelTierLabel(agent.model_tier)}</p>
        <p className="truncate text-xs text-muted-foreground">{invocationLabel}</p>
      </div>

      <div className="min-w-0 space-y-1">
        <p className="text-[10px] font-medium uppercase tracking-[0.14em] text-muted-foreground lg:hidden">Runs · 7d</p>
        <div className="flex min-w-0 flex-wrap items-center gap-2">
          <RunBars5 statuses={stats?.lastFiveStatuses ?? []} />
          <span className="text-sm tabular-nums text-foreground">{stats?.recentRuns ?? 0}</span>
        </div>
      </div>

      <div className="min-w-0 space-y-1">
        <p className="text-[10px] font-medium uppercase tracking-[0.14em] text-muted-foreground lg:hidden">Last run</p>
        <AgentStatusBadge stats={stats} onOpenRun={onOpenRun} />
        {stats?.lastRun ? (
          <p className="text-xs text-muted-foreground">{formatLastRunTime(stats.lastRun)}</p>
        ) : (
          <p className="text-xs text-muted-foreground">Never</p>
        )}
      </div>

      <div className="space-y-1">
        <p className="text-[10px] font-medium uppercase tracking-[0.14em] text-muted-foreground lg:hidden">Used in flows</p>
        <FlowRefs usage={usage} workspaceSlug={workspaceSlug} agentId={agent.id} />
      </div>

      <AgentActions
        agent={agent}
        stats={stats}
        workspaceSlug={workspaceSlug}
        onOpen={onOpen}
        onOpenRun={onOpenRun}
        onRunNow={onRunNow}
        onDelete={onDelete}
        canEdit={canEdit}
      />
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
  const repositoriesSettingsHref = workspace?.slug ? buildSettingsRoutePath(workspace.slug, 'repositories') : undefined;
  const toolConnectionsHref = workspace?.slug ? buildSettingsRoutePath(workspace.slug, 'external-mcp') : undefined;
  const { data: access } = useWorkspaceAccess(workspaceId ?? '');
  const { canEdit } = usePermissions(access);
  const { teams: accessibleTeams, isAdmin } = useAccessibleTeams(workspaceId ?? '');

  const [presets, setPresets] = useState<AgentPresetDefinition[]>([]);
  const [toolCatalog, setToolCatalog] = useState<ToolCatalogResponse | null>(null);
  const [skillCatalog, setSkillCatalog] = useState<SkillCatalogResponse | null>(null);
  const agentFleetQuery = useAutomationAgentFleet(workspaceId ?? '');
  const agents = useMemo(
    () => (agentFleetQuery.data?.agents ?? []).map((item) => item.agent),
    [agentFleetQuery.data?.agents],
  );
  const runStats = useMemo<Record<string, AgentRunStats>>(
    () => Object.fromEntries((agentFleetQuery.data?.agents ?? []).map((item) => [item.agent.id, {
      recentRuns: item.stats.recent_runs,
      recentCompleted: item.stats.recent_completed,
      recentFailed: item.stats.recent_failed,
      recentTokens: item.stats.recent_tokens,
      lastRun: item.stats.last_run,
      attentionRun: item.stats.attention_run,
      attentionRunCount: item.stats.attention_count,
      lastFiveStatuses: item.stats.recent_run_items.map((run) => run.status),
      recentRunItems: item.stats.recent_run_items,
    }])),
    [agentFleetQuery.data?.agents],
  );
  const agentUsageMap = useMemo<Record<string, AgentTriggerUsageSummary | null>>(
    () => Object.fromEntries((agentFleetQuery.data?.agents ?? []).map((item) => [item.agent.id, item.usage])),
    [agentFleetQuery.data?.agents],
  );
  const loading = agentFleetQuery.isPending;
  const refetchAgentFleet = agentFleetQuery.refetch;
  const error = agentFleetQuery.error instanceof Error
    ? agentFleetQuery.error.message
    : agentFleetQuery.error ? String(agentFleetQuery.error) : null;

  const { data: settings } = useWorkspaceSettings(workspaceId ?? '');
  const teams = settings?.teams ?? [];
  const accessibleTeamIds = useMemo(
    () => new Set(accessibleTeams.map((team) => team.id)),
    [accessibleTeams],
  );
  const visibleTeams = isAdmin ? teams : accessibleTeams;

  const [viewMode, setViewMode] = useState<'list' | 'cards'>('list');
  const agentTemplates: AgentTemplate[] = [];
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
  const [securityTriageTemplateForm, setSecurityTriageTemplateForm] = useState<SecurityTriageTemplateFormData>({
    repository_id: '',
    scanners: ['semgrep', 'trivy', 'gitleaks'],
    severity_threshold: 'medium',
    include_low_info: false,
    schedule_preset: 'weekly',
    destination_team_id: '',
    destination_state_id: NONE_OPTION_VALUE,
    max_tasks: '20',
  });

  const [dialogOpen, setDialogOpen] = useState(false);
  const [systemDrawerOpen, setSystemDrawerOpen] = useState(false);
  const [systemDrawerTab, setSystemDrawerTab] = useState<'analytics' | 'limits' | 'versions'>('versions');
  const [tokenLimitMode, setTokenLimitMode] = useState<'preset' | 'custom'>('preset');
  const [agentAnalyticsRange, setAgentAnalyticsRange] = useState<AgentAnalyticsRange>('30d');
  const [agentAnalytics, setAgentAnalytics] = useState<AgentAnalyticsResponse | null>(null);
  const [agentAnalyticsLoading, setAgentAnalyticsLoading] = useState(false);
  const [editingAgent, setEditingAgent] = useState<Agent | null>(null);
  const [agentUsage, setAgentUsage] = useState<AgentTriggerUsageSummary | null>(null);
  const [agentUsageLoading, setAgentUsageLoading] = useState(false);
  const [form, setForm] = useState<AgentFormData>(createEmptyCustomForm());
  const [advancedOpen, setAdvancedOpen] = useState(false);
  const [automationOpen, setAutomationOpen] = useState(false);
  const [compiledPromptOpen, setCompiledPromptOpen] = useState(false);
  const [toolPickerOpen, setToolPickerOpen] = useState(false);
  const [skillPickerOpen, setSkillPickerOpen] = useState(false);
  const [versionSkillsOpen, setVersionSkillsOpen] = useState(false);
  const [versionToolsOpen, setVersionToolsOpen] = useState(false);
  const [systemPromptEditorOpen, setSystemPromptEditorOpen] = useState(false);
  const [saving, setSaving] = useState(false);
  const [deleteConfirmOpen, setDeleteConfirmOpen] = useState(false);
  const [discardCustomEditConfirmOpen, setDiscardCustomEditConfirmOpen] = useState(false);
  const [versionDraftOpen, setVersionDraftOpen] = useState(false);
  const [versionLabelDraft, setVersionLabelDraft] = useState('');
  const [versionDescriptionDraft, setVersionDescriptionDraft] = useState('');
  const [creatingVersion, setCreatingVersion] = useState(false);
  const [workspaceVersionPendingDelete, setWorkspaceVersionPendingDelete] = useState<AgentPresetDefinition | null>(null);
  const [deletingVersion, setDeletingVersion] = useState(false);
  const [workspaceVersionBeingRenamed, setWorkspaceVersionBeingRenamed] = useState<AgentPresetDefinition | null>(null);
  const [customAgentVersions, setCustomAgentVersions] = useState<AgentVersion[]>([]);
  const [selectedCustomVersionID, setSelectedCustomVersionID] = useState('');
  const [customVersionsLoading, setCustomVersionsLoading] = useState(false);
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
  const [selectedRunId, setSelectedRunId] = useState<string | null>(null);
  const [runDrawerOpen, setRunDrawerOpen] = useState(false);
  const [upgradeDialogReason, setUpgradeDialogReason] = useState<UpgradeRequiredReason | null>(null);

  const showUpgradeDialogForError = useCallback((error: unknown) => {
    const reason = getUpgradeRequiredReason(error);
    if (!reason) return false;
    setUpgradeDialogReason(reason);
    return true;
  }, []);

  const loadAgents = useCallback(async () => {
    if (!workspaceId) return;
    await refetchAgentFleet();
  }, [refetchAgentFleet, workspaceId]);

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

  const loadAgentAnalytics = useCallback(async (agentId: string, range: AgentAnalyticsRange) => {
    if (!workspaceId) return;
    setAgentAnalyticsLoading(true);
    setAgentAnalytics(null);
    const res = await automationService.getAgentAnalytics(workspaceId, agentId, range);
    if (!res.error) {
      setAgentAnalytics(res.data ?? null);
    } else {
      setAgentAnalytics(null);
      toast.error('Failed to load agent analytics', { description: res.error });
    }
    setAgentAnalyticsLoading(false);
  }, [workspaceId]);

  const loadCustomAgentVersions = useCallback(async (agent: Agent) => {
    if (!workspaceId || agent.is_system) {
      setCustomAgentVersions([]);
      setSelectedCustomVersionID('');
      return;
    }
    setCustomVersionsLoading(true);
    const res = await automationService.listAgentVersions(workspaceId, agent.id);
    if (res.error) {
      toast.error('Failed to load agent versions', { description: res.error });
      setCustomAgentVersions([]);
      setSelectedCustomVersionID('');
    } else {
      const versions = res.data ?? [];
      setCustomAgentVersions(versions);
      const activeVersionID = agent.active_version_id ?? versions.find((version) => version.version_key === 'default')?.id ?? versions[0]?.id ?? '';
      setSelectedCustomVersionID(activeVersionID);
      const activeVersion = versions.find((version) => version.id === activeVersionID);
      if (activeVersion) {
        setForm(buildCustomAgentVersionForm(agent, activeVersion));
      }
    }
    setCustomVersionsLoading(false);
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
      if (showUpgradeDialogForError(res.error)) return;
      toast.error('Failed to start agent run', { description: res.error ?? 'No run was returned.' });
      return;
    }

    toast.success('Agent run started');
    setRunNowOpen(false);
    setRunNowAgent(null);
    await navigate({
      to: buildAutomationActivityPath(workspace?.slug, { run_id: res.data.id }),
    });
  }, [
    navigate,
    runNowAdditionalContext,
    runNowAgent,
    runNowBaseBranch,
    runNowTargetId,
    runNowTargetType,
    showUpgradeDialogForError,
    workspace?.slug,
    workspaceId,
  ]);

  const openRunDetails = useCallback((runId: string) => {
    setSelectedRunId(runId);
    setRunDrawerOpen(true);
  }, []);

  useEffect(() => {
    loadPresets();
  }, [loadPresets]);

  useEffect(() => {
    if (!systemDrawerOpen || systemDrawerTab !== 'analytics' || !editingAgent?.id) return;
    void loadAgentAnalytics(editingAgent.id, agentAnalyticsRange);
  }, [agentAnalyticsRange, editingAgent?.id, loadAgentAnalytics, systemDrawerOpen, systemDrawerTab]);

  useEffect(() => {
    if (!systemDrawerOpen || systemDrawerTab !== 'analytics' || !editingAgent?.id) return;
    let refreshTimer: ReturnType<typeof setTimeout> | null = null;
    const handler = (event: Event) => {
      if (!isAgentRunLifecycleEvent(event) || refreshTimer) return;
      refreshTimer = setTimeout(() => {
        refreshTimer = null;
        void loadAgentAnalytics(editingAgent.id, agentAnalyticsRange);
      }, 400);
    };
    window.addEventListener('agent_run-created', handler);
    window.addEventListener('agent_run-updated', handler);
    return () => {
      if (refreshTimer) clearTimeout(refreshTimer);
      window.removeEventListener('agent_run-created', handler);
      window.removeEventListener('agent_run-updated', handler);
    };
  }, [agentAnalyticsRange, editingAgent?.id, loadAgentAnalytics, systemDrawerOpen, systemDrawerTab]);

  const openCreateDialog = () => {
    void Promise.all([loadToolCatalog(), loadSkillCatalog()]);
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

  const openCreateFromTemplateDrawer = async (template: AgentTemplate) => {
    void Promise.all([loadToolCatalog(), loadSkillCatalog()]);
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
    setSecurityTriageTemplateForm({
      repository_id: '',
      scanners: ['semgrep', 'trivy', 'gitleaks'],
      severity_threshold: 'medium',
      include_low_info: false,
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
    if (template.key === SECURITY_TRIAGE_TEMPLATE_KEY && repositories.length === 0) {
      await loadTemplateResources();
    }
    if (template.key === COMPETITIVE_INTEL_TEMPLATE_KEY && defaultMarketingTeam?.id) {
      await loadCompetitiveTeamWorkflow(defaultMarketingTeam.id);
    }
    if (template.key === DEPENDENCY_AUDITOR_TEMPLATE_KEY && defaultDependencyTeam?.id) {
      await loadCompetitiveTeamWorkflow(defaultDependencyTeam.id);
    }
    if (template.key === SECURITY_TRIAGE_TEMPLATE_KEY && defaultDependencyTeam?.id) {
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
    if (templateDraft.createStarterFlow && templateDraft.template.key === SECURITY_TRIAGE_TEMPLATE_KEY) {
      if (!securityTriageTemplateForm.repository_id) {
        toast.error('Select a repository');
        return;
      }
      if (securityTriageTemplateForm.scanners.length === 0) {
        toast.error('Select at least one scanner');
        return;
      }
      if (!securityTriageTemplateForm.destination_team_id) {
        toast.error('Select a task team');
        return;
      }
      const maxTasks = Number.parseInt(securityTriageTemplateForm.max_tasks, 10);
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
    if (templateDraft.createStarterFlow && templateDraft.template.key === SECURITY_TRIAGE_TEMPLATE_KEY) {
      const renderedPrompt = renderSecurityTriageSystemPrompt(templateDraft.template.system_prompt ?? form.system_prompt, securityTriageTemplateForm);
      setForm((current) => ({
        ...current,
        system_prompt: renderedPrompt,
      }));
    }
    setTemplateSetupDialogOpen(false);
    setDialogOpen(true);
  };

  const openEditDialog = (agent: Agent) => {
    void Promise.all([loadToolCatalog(), loadSkillCatalog()]);
    setEditingAgent(agent);
    setTemplateDraft(null);
    setAgentUsage(null);
    setAgentAnalytics(null);
    void loadAgentUsage(agent.id);
    setAdvancedOpen(hasConfiguredAdvancedFields(agent, presets));
    setAutomationOpen(agent.is_system ? false : agent.approval_mode !== 'preset_default');
    setToolPickerOpen(false);
    setVersionDraftOpen(false);
    setTokenLimitMode('preset');
    setVersionLabelDraft('');
    setVersionDescriptionDraft('');
    if (agent.is_system) {
      setForm(buildSystemAgentForm(agent, presets));
      setSystemDrawerTab('versions');
      setSystemDrawerOpen(true);
      setDialogOpen(false);
      return;
    }
    setForm(buildCustomAgentForm(agent));
    setSystemDrawerTab('versions');
    setSystemDrawerOpen(true);
    setDialogOpen(false);
    void loadCustomAgentVersions(agent);
  };

  const openDeleteDialog = (agent: Agent) => {
    if (agent.is_system) return;
    setEditingAgent(agent);
    setDeleteConfirmOpen(true);
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
        if (templateDraft.createStarterFlow && templateDraft.template.key === SECURITY_TRIAGE_TEMPLATE_KEY) {
          if (!securityTriageTemplateForm.repository_id) {
            toast.error('Select a repository');
            setSaving(false);
            return;
          }
          if (securityTriageTemplateForm.scanners.length === 0) {
            toast.error('Select at least one scanner');
            setSaving(false);
            return;
          }
          if (!securityTriageTemplateForm.destination_team_id) {
            toast.error('Select a task team');
            setSaving(false);
            return;
          }
          const maxTasks = Number.parseInt(securityTriageTemplateForm.max_tasks, 10);
          if (!Number.isFinite(maxTasks) || maxTasks < 1 || maxTasks > 100) {
            toast.error('Maximum tasks must be between 1 and 100');
            setSaving(false);
            return;
          }
        }

        const selectedRepo = repositories.find((repo) => repo.id === templateForm.repository_id);
        const selectedDependencyRepo = repositories.find((repo) => repo.id === dependencyAuditorTemplateForm.repository_id);
        const selectedSecurityRepo = repositories.find((repo) => repo.id === securityTriageTemplateForm.repository_id);
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
              : templateDraft.template.key === SECURITY_TRIAGE_TEMPLATE_KEY
                ? {
                    flow_key: SECURITY_TRIAGE_FLOW_KEY,
                    repository_id: securityTriageTemplateForm.repository_id,
                    repo_full_name: selectedSecurityRepo?.full_name,
                    flow_input: {
                      scanners: securityTriageTemplateForm.scanners,
                      severity_threshold: securityTriageTemplateForm.severity_threshold,
                      include_low_info: securityTriageTemplateForm.include_low_info,
                      schedule_preset: securityTriageTemplateForm.schedule_preset,
                      destination_team_id: securityTriageTemplateForm.destination_team_id,
                      destination_state_id: securityTriageTemplateForm.destination_state_id === NONE_OPTION_VALUE
                        ? undefined
                        : securityTriageTemplateForm.destination_state_id,
                      max_tasks: Number.parseInt(securityTriageTemplateForm.max_tasks, 10),
                    },
                  }
              : undefined
          : undefined;
        const payload = {
          name: form.name.trim(),
          team_id: form.team_id || undefined,
          overrides: {
            role: templateDraft.template.default_role,
            icon_key: form.icon_key,
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
          if (showUpgradeDialogForError(res.error)) {
            setSaving(false);
            return;
          }
          toast.error('Failed to create agent from template', { description: res.error });
        }
      } else {
        const payload = buildCustomAgentCreatePayload(workspaceId, form, advancedOpen);
        const res = await automationService.createAgent(workspaceId, payload);
        if (!res.error) {
          setDialogOpen(false);
          await loadAgents();
          toast.success('Agent created');
        } else {
          if (showUpgradeDialogForError(res.error)) {
            setSaving(false);
            return;
          }
          toast.error('Failed to create agent', { description: res.error });
        }
      }
    }
    setSaving(false);
  };

  const handleCreatePresetVersion = async () => {
    if (!workspaceId || !editingAgent || !versionLabelDraft.trim()) return;
    setCreatingVersion(true);
    if (!editingAgent.is_system) {
      const res = await automationService.createAgentVersion(workspaceId, editingAgent.id, {
        label: versionLabelDraft.trim(),
        description: versionDescriptionDraft.trim() || undefined,
        source_version_id: selectedCustomVersionID || undefined,
        model_tier: form.model_tier,
        system_prompt: form.system_prompt.trim() || undefined,
        skills: form.skills,
        allowed_tools: normalizeToolList(form.allowed_tools),
        allowed_targets: normalizeTargetList(form.allowed_targets),
        supported_modes: form.supported_modes,
        default_invocation_mode: form.default_invocation_mode,
      });
      if (!res.error && res.data) {
        await loadCustomAgentVersions(editingAgent);
        setSelectedCustomVersionID(res.data.id);
        setForm(buildCustomAgentVersionForm(editingAgent, res.data));
        setVersionDraftOpen(false);
        setVersionLabelDraft('');
        setVersionDescriptionDraft('');
      } else if (res.error) {
        toast.error('Failed to save custom version', { description: res.error });
      }
      setCreatingVersion(false);
      return;
    }
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
      instruction_skills: form.instruction_skills,
      available_skills: form.available_skill_keys,
      allowed_tools: normalizeToolList(form.allowed_tools),
      allowed_targets: normalizeTargetList(form.allowed_targets),
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
        system_prompt: res.data?.system_prompt || current.system_prompt,
        instruction_preamble: res.data?.instruction_preamble ?? current.instruction_preamble,
        instruction_skills: res.data?.instruction_skills ?? current.instruction_skills,
        available_skill_keys: res.data?.available_skills ?? current.available_skill_keys,
        allowed_targets: normalizeTargetList(res.data?.allowed_target_types ?? current.allowed_targets),
      }));
      setVersionDraftOpen(false);
      setVersionLabelDraft('');
      setVersionDescriptionDraft('');
    } else if (res.error) {
      toast.error('Failed to save custom version', { description: res.error });
    }
    setCreatingVersion(false);
  };

  const handleSaveWorkspaceVersion = async (options?: { silent?: boolean }) => {
    if (!workspaceId || !editingAgent) return false;
    if (!editingAgent.is_system) {
      if (!selectedCustomVersion) return false;
      if (!options?.silent) setSaving(true);
      const res = await automationService.updateAgentVersion(workspaceId, editingAgent.id, selectedCustomVersion.id, {
        label: selectedCustomVersion.label,
        description: selectedCustomVersion.description,
        model_tier: form.model_tier,
        system_prompt: form.system_prompt.trim() || undefined,
        skills: form.skills,
        allowed_tools: normalizeToolList(form.allowed_tools),
        allowed_targets: normalizeTargetList(form.allowed_targets),
        supported_modes: form.supported_modes,
        default_invocation_mode: form.default_invocation_mode,
      });
      if (res.error) {
        toast.error('Failed to save version', { description: res.error });
        if (!options?.silent) setSaving(false);
        return false;
      }
      if (res.data) {
        setCustomAgentVersions((current) => current.map((version) => (version.id === res.data?.id ? res.data : version)));
      }
      if (editingAgent.active_version_id === selectedCustomVersion.id) {
        await loadAgents();
      }
      if (!options?.silent) {
        toast.success('Version saved');
        setSaving(false);
      }
      return true;
    }
    if (!selectedPreset?.id) return false;
    if (!options?.silent) setSaving(true);
    const payload: UpdateWorkspaceAgentPresetVersionRequest = {
      label: selectedPreset.version_label,
      description: selectedPreset.description,
      runtime_kind: form.runtime_kind,
      provider: form.provider,
      model: form.model.trim(),
      execution_config: buildExecutionConfigPayload(form),
      system_prompt: form.system_prompt.trim() || undefined,
      instruction_skills: form.instruction_skills,
      available_skills: form.available_skill_keys,
      allowed_tools: normalizeToolList(form.allowed_tools),
      allowed_targets: normalizeTargetList(form.allowed_targets),
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

  const handleSaveAgentSettings = async () => {
    if (!workspaceId || !editingAgent) return;
    setSaving(true);
    const res = await automationService.updateAgent(workspaceId, editingAgent.id, {
      monthly_token_budget: form.monthly_token_budget.trim()
        ? Number.parseInt(form.monthly_token_budget, 10)
        : 0,
    });
    if (res.error) {
      toast.error('Failed to save agent settings', { description: res.error });
      setSaving(false);
      return;
    }
    if (res.data) {
      setEditingAgent(res.data);
      setForm((current) => ({
        ...current,
        monthly_token_budget: res.data?.monthly_token_budget?.toString() ?? '',
      }));
    }
    await loadAgents();
    toast.success('Agent settings saved');
    setSaving(false);
  };

  const handleSetActiveVersion = async (versionKey: string) => {
    if (!workspaceId || !editingAgent || !versionKey) return;
    if (!editingAgent.is_system) {
      if (versionKey === editingAgent.active_version_id) return;
      setSaving(true);
      const res = await automationService.activateAgentVersion(workspaceId, editingAgent.id, versionKey);
      if (res.error) {
        toast.error('Failed to set active version', { description: res.error });
        setSaving(false);
        return;
      }
      if (res.data) {
        setEditingAgent(res.data);
        setForm(buildCustomAgentForm(res.data));
        await loadCustomAgentVersions(res.data);
      }
      await loadAgents();
      setSaving(false);
      toast.success('Active version updated');
      return;
    }
    if (!versionKey || versionKey === currentSystemVersionKey) return;
    setSaving(true);
    const res = await automationService.updateAgent(workspaceId, editingAgent.id, {
      preset_key: form.preset_key,
      preset_version_key: versionKey,
    });
    if (res.error) {
      toast.error('Failed to set active version', { description: res.error });
      setSaving(false);
      return;
    }
    if (res.data) {
      setEditingAgent(res.data);
      setForm(buildSystemAgentForm(res.data, presets));
    }
    await loadPresets();
    await loadAgents();
    setVersionDraftOpen(false);
    setVersionLabelDraft('');
    setVersionDescriptionDraft('');
    setSaving(false);
    toast.success('Active version updated');
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
      // Selected row was just deleted — fall back to the agent's currently active version or the product default.
      setForm(buildSystemAgentForm(editingAgent, presets));
    }
    toast.success('Custom version deleted');
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
    const flows = agentFlowUsages(agentUsageMap[editingAgent.id]);
    for (const flow of flows) {
      if (!flow.reference_id) continue;
      const flowRes = await automationService.deleteFlow(workspaceId, flow.reference_id);
      if (flowRes.error) {
        toast.error('Failed to delete linked flow', { description: flowRes.error });
        setSaving(false);
        return;
      }
    }
    const res = await automationService.deleteAgent(workspaceId, editingAgent.id);
    if (!res.error) {
      setDialogOpen(false);
      setDeleteConfirmOpen(false);
      setEditingAgent(null);
      await loadAgents();
      toast.success(flows.length > 0 ? 'Agent and flows deleted' : 'Agent deleted');
    } else {
      toast.error('Failed to delete agent', { description: res.error });
    }
    setSaving(false);
  };

  const visibleAgents = agents.filter((agent) => {
    if (!agent.team_id) {
      return true;
    }
    return accessibleTeamIds.has(agent.team_id);
  });
  const sortedAgents = sortAgentsForDisplay(visibleAgents);
  const deleteAgentFlows = editingAgent ? agentFlowUsages(agentUsageMap[editingAgent.id]) : [];
  const deleteAgentFlowCount = deleteAgentFlows.length;
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
  const editingCustomAgent = Boolean(editingAgent && !editingAgent.is_system);
  const selectedPreset = editingSystemAgent ? presetMetaForSelection(form.preset_key, form.preset_version_key, presets) : null;
  const selectedPresetVersions = editingSystemAgent ? presetVersionsForKey(form.preset_key, presets) : [];
  const selectedCustomVersion = editingCustomAgent
    ? customAgentVersions.find((version) => version.id === selectedCustomVersionID) ?? null
    : null;
  const currentSystemVersionKey = editingSystemAgent
    ? (editingAgent?.preset_version_key?.trim() || selectedPreset?.version_key || fallbackPresetVersionKey(form.preset_key))
    : '';
  const selectedSystemVersionKey = editingSystemAgent
    ? (form.preset_version_key?.trim() || selectedPreset?.version_key || fallbackPresetVersionKey(form.preset_key))
    : '';
  const hasPendingSystemVersionSelection = editingSystemAgent
    && Boolean(selectedSystemVersionKey)
    && Boolean(currentSystemVersionKey)
    && selectedSystemVersionKey !== currentSystemVersionKey;
  const hasAgentSettingsChanges = Boolean(editingAgent && (
    normalizeTokenBudgetFormValue(form.monthly_token_budget) !== normalizeTokenBudgetFormValue(editingAgent.monthly_token_budget?.toString() ?? '')
  ));
  const selectedAgentStats = editingAgent ? runStats[editingAgent.id] : undefined;
  const tokenUsageSummary = getAgentTokenUsageSummary(editingAgent);
  const monthlyTokenLimit = normalizeTokenBudgetFormValue(form.monthly_token_budget);
  const monthlyTokensUsed = tokenUsageSummary.monthlyTokensUsed;
  const monthlyTokenPercent = monthlyTokenLimit > 0 ? Math.min(100, Math.round((monthlyTokensUsed / monthlyTokenLimit) * 100)) : 0;
  const monthlyLimitMatchesPreset = MONTHLY_TOKEN_LIMIT_PRESETS.some((preset) => preset.value === monthlyTokenLimit);
  const showCustomTokenLimitInput = tokenLimitMode === 'custom' || !monthlyLimitMatchesPreset;
  const analyticsSummary = getAgentAnalyticsSummary(agentAnalytics, agentAnalyticsRange);
  const recentRunItems = selectedAgentStats?.recentRunItems ?? [];
  const selectedVersionLabel = editingSystemAgent ? selectedPreset?.version_label : selectedCustomVersion?.label;
  const selectedVersionDescription = editingSystemAgent ? selectedPreset?.description : selectedCustomVersion?.description;
  const selectedVersionCreatedAt = editingSystemAgent ? selectedPreset?.created_at : selectedCustomVersion?.created_at;
  const selectedVersionIsActive = editingSystemAgent
    ? selectedPreset?.version_key === currentSystemVersionKey
    : selectedCustomVersion?.id === editingAgent?.active_version_id;
  const selectedVersionActivationKey = editingSystemAgent ? selectedSystemVersionKey : selectedCustomVersionID;
  const canSetSelectedVersionActive = editingSystemAgent
    ? hasPendingSystemVersionSelection
    : Boolean(selectedCustomVersionID && selectedCustomVersionID !== editingAgent?.active_version_id);
  const canEditSelectedVersionDescription = editingSystemAgent && canEditWorkspacePresetVersionDescription(selectedPreset);
  const isEditingWorkspaceVersion = editingSystemAgent && !versionDraftOpen && selectedPreset?.scope === 'workspace' && Boolean(selectedPreset?.id);
  const isEditingCustomVersion = editingCustomAgent && !versionDraftOpen && Boolean(selectedCustomVersion);
  const systemVersionReadOnly = editingSystemAgent && !versionDraftOpen && !isEditingWorkspaceVersion;
  const versionReadOnly = editingSystemAgent ? systemVersionReadOnly : false;
  const versionSystemPromptValue = versionReadOnly
    ? selectedPreset?.system_prompt?.trim() || form.system_prompt
    : form.system_prompt;
  const hasWorkspaceVersionChanges = Boolean(isEditingWorkspaceVersion && selectedPreset && (
    form.runtime_kind !== selectedPreset.runtime_kind ||
    form.provider !== (selectedPreset.provider ?? '') ||
    form.model.trim() !== (selectedPreset.model ?? '') ||
    stableConfigJSON(buildExecutionConfigPayload(form)) !== stableConfigJSON(selectedPreset.execution_config) ||
    form.system_prompt.trim() !== (selectedPreset.system_prompt || selectedPreset.instruction_preamble || '').trim() ||
    stableJSON(form.instruction_skills) !== stableJSON(selectedPreset.instruction_skills ?? []) ||
    stableJSON(form.available_skill_keys) !== stableJSON(selectedPreset.available_skills ?? []) ||
    stableJSON(normalizeToolList(form.allowed_tools)) !== stableJSON(normalizeToolList(selectedPreset.allowed_tools ?? [])) ||
    stableJSON(normalizeTargetList(form.allowed_targets)) !== stableJSON(normalizeTargetList(selectedPreset.allowed_target_types ?? [])) ||
    stableJSON(form.supported_modes) !== stableJSON(selectedPreset.supported_modes) ||
    form.default_invocation_mode !== selectedPreset.default_invocation_mode
  ));
  const hasCustomVersionChanges = Boolean(isEditingCustomVersion && selectedCustomVersion && (
    form.model_tier !== (selectedCustomVersion.model_tier ?? editingAgent?.model_tier ?? 'large') ||
    form.system_prompt.trim() !== (selectedCustomVersion.system_prompt ?? '') ||
    stableJSON(form.skills) !== stableJSON(selectedCustomVersion.skills ?? []) ||
    stableJSON(normalizeToolList(form.allowed_tools)) !== stableJSON(normalizeToolList(selectedCustomVersion.allowed_tools ?? [])) ||
    stableJSON(normalizeTargetList(form.allowed_targets)) !== stableJSON(normalizeTargetList(selectedCustomVersion.allowed_targets ?? [])) ||
    stableJSON(form.supported_modes) !== stableJSON(selectedCustomVersion.supported_modes ?? []) ||
    form.default_invocation_mode !== selectedCustomVersion.default_invocation_mode
  ));
  const hasVersionChanges = hasWorkspaceVersionChanges || hasCustomVersionChanges;
  const effectiveTargets =
    editingSystemAgent
      ? (selectedPreset?.allowed_target_types ?? form.allowed_targets)
      : (form.allowed_targets.length > 0 ? form.allowed_targets : ['task']);
  const supportedModes = form.supported_modes.length > 0 ? form.supported_modes : supportedModesForForm(form.runtime_kind);
  const templateStarterFlow = templateDraft?.template.starter_flows?.find((flow) => flow.key === 'github_release_notes')
    ?? templateDraft?.template.starter_flows?.[0];
  const versionToolEditingState = getVersionToolEditingState({ versionReadOnly, runtimeKind: form.runtime_kind });
  const isBlankCustomCreate = !editingAgent && !templateDraft;
  const isCustomEdit = Boolean(editingAgent && !editingAgent.is_system && !templateDraft);
  const isTemplateCreate = Boolean(templateDraft && !editingAgent);
  const createDrawerTitle = editingAgent
    ? 'Custom Agent'
    : templateDraft
      ? `Create ${templateDraft.template.name}`
      : 'Create Custom Agent';
  const createDrawerSubtitle = editingAgent
    ? 'Tune this custom agent directly. It is not tied to a product preset.'
    : templateDraft
      ? 'Start from a packaged template, review the defaults, and create the agent with an optional automation flow.'
      : 'Define a reusable agent with its own instructions, tools, targets, and limits.';
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
    if (starterFlowEnabled && templateDraft?.template.key === SECURITY_TRIAGE_TEMPLATE_KEY) {
      if (!securityTriageTemplateForm.repository_id) missing.push('repository');
      if (securityTriageTemplateForm.scanners.length === 0) missing.push('scanner');
      if (!securityTriageTemplateForm.destination_team_id) missing.push('task team');
      const maxTasks = Number.parseInt(securityTriageTemplateForm.max_tasks, 10);
      if (!Number.isFinite(maxTasks) || maxTasks < 1 || maxTasks > 100) {
        missing.push('valid task limit');
      }
    }
    return missing;
  })();
  const createDrawerReady = createDrawerMissingRequirements.length === 0;
  const customEditHasChanges = useMemo(() => {
    if (!editingAgent || editingAgent.is_system) return true;
    return customAgentFormDirtyKey(form) !== customAgentFormDirtyKey(buildCustomAgentForm(editingAgent));
  }, [editingAgent, form]);
  const createDrawerCanSave = createDrawerReady && (!editingAgent || editingAgent.is_system || customEditHasChanges);
  const closeCustomAgentDrawer = useCallback(() => {
    setDialogOpen(false);
    setToolPickerOpen(false);
    setSkillPickerOpen(false);
    setSystemPromptEditorOpen(false);
    setTemplateSetupDialogOpen(false);
    setTemplateDraft(null);
    setDocsCollections([]);
  }, []);
  const requestCloseCustomAgentDrawer = useCallback(() => {
    if (editingAgent && !editingAgent.is_system && customEditHasChanges) {
      setDiscardCustomEditConfirmOpen(true);
      return;
    }
    closeCustomAgentDrawer();
  }, [closeCustomAgentDrawer, customEditHasChanges, editingAgent]);
  const createDrawerStatus = createDrawerReady
    ? editingAgent
      ? customEditHasChanges || editingAgent.is_system
        ? 'Ready to save'
        : 'No changes to save'
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
  const skillCatalogEntries = skillCatalog?.skills ?? [];
  const attachedSkillIdentities = new Set(form.skills.map(agentSkillIdentity));
  const availableSkillEntries = skillCatalogEntries.filter((s) => !attachedSkillIdentities.has(agentSkillIdentity(s)));
  const availableSystemSkillItems = form.available_skill_keys.map((key) => ({ key, identity: key }));
  const versionSkillItems = editingCustomAgent
    ? form.skills.map((skill) => ({ key: skill.key, identity: agentSkillIdentity(skill) }))
    : availableSystemSkillItems;
  const attachedVersionSkillIdentities = new Set(versionSkillItems.map((skill) => skill.identity));
  const coreSkillKeySet = new Set(form.instruction_skills);
  const availableVersionSkillEntries = skillCatalogEntries.filter((s) => (
    !attachedVersionSkillIdentities.has(editingCustomAgent ? agentSkillIdentity(s) : s.key)
    && (editingCustomAgent || !coreSkillKeySet.has(s.key))
  ));
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
  const removeSkill = (identity: string) => {
    setForm((current) => ({ ...current, skills: current.skills.filter((s) => agentSkillIdentity(s) !== identity) }));
  };
  const addVersionInstructionSkill = (entry: { id?: string; key: string; source_kind?: string; required_tools?: string[] }) => {
    const missingTools = (entry.required_tools ?? []).filter((t) => !form.allowed_tools.includes(t));
    if (missingTools.length > 0) {
      toast.warning(`Skill "${entry.key}" requires tools not yet allowed: ${missingTools.join(', ')}`);
    }
    setForm((current) => {
      if (editingCustomAgent) {
        const ref: AgentSkillRef = { key: entry.key };
        if (entry.id) ref.skill_id = entry.id;
        const identity = agentSkillIdentity(ref);
        if (current.skills.some((skill) => agentSkillIdentity(skill) === identity)) return current;
        return {
          ...current,
          skills: [...current.skills, ref],
        };
      }
      if (current.available_skill_keys.includes(entry.key)) return current;
      return {
        ...current,
        available_skill_keys: [...current.available_skill_keys, entry.key],
      };
    });
    setSkillPickerOpen(false);
  };
  const removeVersionInstructionSkill = (identity: string) => {
    setForm((current) => {
      if (editingCustomAgent) {
        return {
          ...current,
          skills: current.skills.filter((skill) => agentSkillIdentity(skill) !== identity),
        };
      }
      return {
        ...current,
        available_skill_keys: current.available_skill_keys.filter((key) => key !== identity),
      };
    });
  };
  const toggleTarget = (target: AgentTargetType) => {
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
      nextPreset.provider ?? presetFallback(form.preset_key).provider ?? form.provider,
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
      system_prompt: nextPreset.system_prompt || nextPreset.instruction_preamble?.trim() || '',
      instruction_preamble: nextPreset.instruction_preamble ?? '',
      instruction_skills: nextPreset.instruction_skills ?? [],
      available_skill_keys: nextPreset.available_skills ?? [],
      allowed_tools: normalizeToolList(nextPreset.allowed_tools ?? []),
      allowed_targets: normalizeTargetList(nextPreset.allowed_target_types ?? []),
      approval_mode: 'never',
      default_invocation_mode: normalizeDefaultInvocationMode(
        nextPreset.default_invocation_mode,
        nextPreset.runtime_kind,
        nextPreset.default_invocation_mode,
      ),
    }));
  };
  const selectCustomAgentVersion = (versionID: string) => {
    if (!editingAgent || editingAgent.is_system) return;
    const nextVersion = customAgentVersions.find((version) => version.id === versionID);
    if (!nextVersion) return;
    setVersionDraftOpen(false);
    setVersionLabelDraft('');
    setVersionDescriptionDraft('');
    setSelectedCustomVersionID(nextVersion.id);
    setForm(buildCustomAgentVersionForm(editingAgent, nextVersion));
  };

  const renderSystemDrawer = () => (
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
            setCustomAgentVersions([]);
            setSelectedCustomVersionID('');
            setAgentAnalytics(null);
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
                  {editingAgent?.is_system
                    ? editingAgent ? agentRoleLabel(editingAgent, presets) : 'Agent'
                    : 'Custom agent'}
                </SheetDescription>
              </div>
            </div>
          </SheetHeader>

          <Tabs value={systemDrawerTab} onValueChange={(value) => setSystemDrawerTab(value as 'analytics' | 'limits' | 'versions')} className="min-h-0 flex-1 gap-0 overflow-hidden">
            <div className="border-b border-border/60 bg-background px-6 py-2">
              <TabsList variant="line">
                <TabsTrigger value="versions" className="focus-visible:border-transparent focus-visible:outline-none focus-visible:ring-0">Versions</TabsTrigger>
                <TabsTrigger value="analytics" className="focus-visible:border-transparent focus-visible:outline-none focus-visible:ring-0">Analytics</TabsTrigger>
                <TabsTrigger value="limits" className="focus-visible:border-transparent focus-visible:outline-none focus-visible:ring-0">Limits</TabsTrigger>
              </TabsList>
            </div>

            <TabsContent value="analytics" className="mt-0 min-h-0 overflow-y-auto px-6 pb-12 pt-6">
              <div className="mx-auto w-full max-w-6xl space-y-6">
                <section className="space-y-3">
                  <div className="flex flex-wrap items-center justify-between gap-3">
                    <div>
                      <h3 className="text-xs font-semibold uppercase tracking-wider text-muted-foreground">Agent analytics</h3>
                      <p className="mt-1 text-sm text-muted-foreground">Usage and run health for this agent.</p>
                    </div>
                    <div className="flex rounded-md border border-border p-0.5">
                      {AGENT_ANALYTICS_RANGES.map((range) => (
                        <button
                          key={range.value}
                          type="button"
                          onClick={() => setAgentAnalyticsRange(range.value)}
                          className={cn(
                            'h-7 rounded px-2.5 text-xs font-medium transition-colors',
                            agentAnalyticsRange === range.value
                              ? 'bg-muted text-foreground'
                              : 'text-muted-foreground hover:text-foreground',
                          )}
                        >
                          {range.label}
                        </button>
                      ))}
                    </div>
                  </div>

                  <div className="grid gap-3 sm:grid-cols-3">
                    <div className="rounded-lg border border-border/60 bg-card p-4">
                      <p className="text-[10px] font-medium uppercase tracking-wider text-muted-foreground/70">Runs · {analyticsSummary.rangeLabel}</p>
                      <p className="mt-2 text-2xl font-semibold">{analyticsSummary.runs.toLocaleString()}</p>
                      <p className="mt-1 text-xs text-muted-foreground">
                        {analyticsSummary.completed.toLocaleString()} completed · {analyticsSummary.failed.toLocaleString()} failed
                      </p>
                    </div>
                    <div className="rounded-lg border border-border/60 bg-card p-4">
                      <p className="text-[10px] font-medium uppercase tracking-wider text-muted-foreground/70">Success rate · {analyticsSummary.rangeLabel}</p>
                      <p className="mt-2 text-2xl font-semibold">{analyticsSummary.successRate}</p>
                      <p className="mt-1 text-xs text-muted-foreground">{analyticsSummary.needsAttention.toLocaleString()} runs need attention</p>
                    </div>
                    <div className="rounded-lg border border-border/60 bg-card p-4">
                      <p className="text-[10px] font-medium uppercase tracking-wider text-muted-foreground/70">Avg tokens / run · {analyticsSummary.rangeLabel}</p>
                      <p className="mt-2 text-2xl font-semibold">{analyticsSummary.avgTokensPerRun > 0 ? analyticsSummary.avgTokensPerRun.toLocaleString() : '-'}</p>
                      <p className="mt-1 text-xs text-muted-foreground">Efficiency signal across recent runs</p>
                    </div>
                  </div>
                </section>

                <section className="space-y-3">
                  <div className="flex flex-wrap items-center justify-between gap-3">
                    <div>
                      <h3 className="text-sm font-medium">Run trends</h3>
                      <p className="mt-1 text-xs text-muted-foreground">Runs, completions, failures, and paused runs over time.</p>
                    </div>
                  </div>
                  {agentAnalyticsLoading ? (
                    <div className="flex h-56 items-center justify-center rounded-lg border border-border/60 bg-card text-sm text-muted-foreground">
                      <Loading01Icon className="mr-2 h-4 w-4 animate-spin" />
                      Loading trends...
                    </div>
                  ) : (
                    <AgentRunsTrendChart analytics={agentAnalytics} />
                  )}
                </section>

                <section className="rounded-lg border border-border/60 bg-card">
                  <div className="flex flex-wrap items-center justify-between gap-3 border-b border-border/60 px-4 py-3">
                    <div>
                      <h3 className="text-sm font-medium">Recent runs</h3>
                      <p className="mt-1 text-xs text-muted-foreground">Last five runs with status, target, tokens, and time.</p>
                    </div>
                    {editingAgent && (
                      <Button
                        type="button"
                        size="sm"
                        variant="outline"
                        onClick={() => window.open(resolveTriggerHistoryPath(workspace?.slug, editingAgent.id), '_blank')}
                      >
                        View all
                      </Button>
                    )}
                  </div>
                  {recentRunItems.length > 0 ? (
                    <div className="divide-y divide-border/60">
                      {recentRunItems.map((run) => {
                        const summary = getAgentRecentRunSummary(run);
                        return (
                          <button
                            key={run.id}
                            type="button"
                            onClick={() => openRunDetails(run.id)}
                            className="grid w-full gap-3 px-4 py-3 text-left transition-colors hover:bg-muted/40 sm:grid-cols-[8rem_minmax(0,1fr)_8rem_8rem]"
                          >
                            <div>
                              <Badge variant="outline" className={cn('h-5 px-1.5 text-[10px]', lastRunStatusClass(run))}>
                                {lastRunStatusLabel(run)}
                              </Badge>
                            </div>
                            <div className="min-w-0">
                              <p className="truncate text-sm font-medium" title={summary.title}>{summary.title}</p>
                              <p className="mt-0.5 truncate text-xs text-muted-foreground" title={summary.subtitle}>{summary.subtitle}</p>
                            </div>
                            <div>
                              <p className="text-[10px] font-medium uppercase tracking-wider text-muted-foreground/70">Tokens</p>
                              <p className="font-mono text-sm">{getAgentTokenUsageTotal(run).toLocaleString()}</p>
                            </div>
                            <div>
                              <p className="text-[10px] font-medium uppercase tracking-wider text-muted-foreground/70">When</p>
                              <p className="text-sm text-muted-foreground">{formatLastRunTime(run)}</p>
                            </div>
                          </button>
                        );
                      })}
                    </div>
                  ) : (
                    <p className="px-4 py-6 text-sm text-muted-foreground">No runs yet.</p>
                  )}
                </section>
              </div>
            </TabsContent>

            <TabsContent value="limits" className="mt-0 min-h-0 overflow-y-auto px-6 pb-12 pt-6">
              <div className="mx-auto w-full max-w-4xl space-y-6">
                <section className="space-y-3">
                  <div>
                    <h3 className="text-xs font-semibold uppercase tracking-wider text-muted-foreground">Agent limits</h3>
                    <p className="mt-1 text-sm text-muted-foreground">Usage limits that apply across all versions of this agent.</p>
                  </div>
                  <div className="grid gap-3 sm:grid-cols-3">
                    <div className="rounded-lg border border-border/60 bg-card p-4">
                      <p className="text-[10px] font-medium uppercase tracking-wider text-muted-foreground/70">Monthly limit</p>
                      <p className="mt-2 text-2xl font-semibold">{monthlyTokenLimit > 0 ? monthlyTokenLimit.toLocaleString() : '-'}</p>
                      <p className="mt-1 text-xs text-muted-foreground">Tokens allowed this month.</p>
                    </div>
                    <div className="rounded-lg border border-border/60 bg-card p-4">
                      <p className="text-[10px] font-medium uppercase tracking-wider text-muted-foreground/70">Used this month</p>
                      <p className="mt-2 text-2xl font-semibold">{monthlyTokensUsed.toLocaleString()}</p>
                      <p className="mt-1 text-xs text-muted-foreground">{monthlyTokenLimit > 0 ? `${monthlyTokenPercent}% of limit` : 'No limit set'}</p>
                    </div>
                    <div className="rounded-lg border border-border/60 bg-card p-4">
                      <p className="text-[10px] font-medium uppercase tracking-wider text-muted-foreground/70">Total tokens</p>
                      <p className="mt-2 text-2xl font-semibold">{tokenUsageSummary.totalTokensUsed.toLocaleString()}</p>
                      <p className="mt-1 text-xs text-muted-foreground">
                        {tokenUsageSummary.totalTokensAdjusted ? 'At least this many tokens used.' : 'Across all recorded runs.'}
                      </p>
                    </div>
                  </div>
                </section>

                <section className="rounded-lg border border-border/60 bg-card p-4">
                  <div className="flex items-start justify-between gap-3">
                    <div>
                      <h3 className="text-sm font-medium">Monthly token limit</h3>
                    </div>
                    {hasAgentSettingsChanges && (
                      <Button type="button" size="sm" disabled={saving} onClick={handleSaveAgentSettings}>
                        {saving ? 'Applying…' : 'Apply'}
                      </Button>
                    )}
                  </div>
                  <div className="mt-4 space-y-3">
                    <Label className="text-[10px] font-medium uppercase tracking-wider text-muted-foreground/70">
                      Preset limits
                    </Label>
                    <div className="grid gap-2 sm:grid-cols-3 lg:grid-cols-6">
                      {MONTHLY_TOKEN_LIMIT_PRESETS.map((preset) => {
                        const active = tokenLimitMode !== 'custom' && monthlyTokenLimit === preset.value;
                        return (
                          <button
                            key={preset.label}
                            type="button"
                            onClick={() => {
                              setTokenLimitMode('preset');
                              setForm((current) => ({
                                ...current,
                                monthly_token_budget: preset.value > 0 ? String(preset.value) : '',
                              }));
                            }}
                            className={cn(
                              'rounded-md border px-3 py-2 text-left text-sm font-medium transition-colors',
                              active
                                ? 'border-primary bg-primary/10 text-foreground'
                                : 'border-border bg-background text-muted-foreground hover:bg-muted/50 hover:text-foreground',
                            )}
                          >
                            {preset.label}
                          </button>
                        );
                      })}
                      <button
                        type="button"
                        onClick={() => setTokenLimitMode('custom')}
                        className={cn(
                          'rounded-md border px-3 py-2 text-left text-sm font-medium transition-colors',
                          showCustomTokenLimitInput
                            ? 'border-primary bg-primary/10 text-foreground'
                            : 'border-border bg-background text-muted-foreground hover:bg-muted/50 hover:text-foreground',
                        )}
                      >
                        Custom
                      </button>
                    </div>
                    {showCustomTokenLimitInput && (
                      <div className="max-w-sm">
                        <Label htmlFor="agent-token-limit" className="text-[10px] font-medium uppercase tracking-wider text-muted-foreground/70">
                          Custom monthly token limit
                        </Label>
                        <Input
                          id="agent-token-limit"
                          type="number"
                          min={0}
                          value={form.monthly_token_budget}
                          onChange={(e) => {
                            setTokenLimitMode('custom');
                            setForm((current) => ({ ...current, monthly_token_budget: e.target.value }));
                          }}
                          placeholder="Enter token limit"
                          className="mt-1 h-9"
                        />
                      </div>
                    )}
                  </div>
                  {monthlyTokenLimit > 0 ? (
                    <>
                      <div className="mt-4 h-2 overflow-hidden rounded-full bg-muted">
                        <div
                          className={cn(
                            'h-full rounded-full transition-all',
                            monthlyTokenPercent >= 90 ? 'bg-destructive' : monthlyTokenPercent >= 70 ? 'bg-amber-500' : 'bg-primary',
                          )}
                          style={{ width: `${monthlyTokenPercent}%` }}
                        />
                      </div>
                      <p className="mt-2 text-xs text-muted-foreground">
                        {monthlyTokensUsed.toLocaleString()} of {monthlyTokenLimit.toLocaleString()} tokens used this month.
                      </p>
                    </>
                  ) : (
                    <p className="mt-3 text-xs text-muted-foreground">No monthly limit is set.</p>
                  )}
                </section>
              </div>
            </TabsContent>

            <TabsContent value="versions" className="mt-0 flex min-h-0 overflow-hidden">
            <div className="grid min-h-0 flex-1 lg:grid-cols-[20rem_minmax(0,1fr)]">
            {/* ──────── LEFT: VERSIONS PANE ──────── */}
            <aside className="flex min-h-0 flex-col border-b border-border/60 bg-muted/20 lg:border-b-0 lg:border-r">
              <div className="flex items-center justify-between gap-2 border-b border-border/60 px-4 py-3">
                <div>
                  <p className="text-xs font-semibold uppercase tracking-wider text-muted-foreground">Versions</p>
                  <p className="text-[10px] text-muted-foreground">
                    {editingSystemAgent ? selectedPresetVersions.length : customAgentVersions.length} available
                  </p>
                </div>
                <Button
                  type="button"
                  size="sm"
                  variant="ghost"
                  className="h-7 gap-1 px-2 text-[11px]"
                  onClick={() => {
                    setVersionDraftOpen(true);
                    setVersionLabelDraft(`${editingSystemAgent ? selectedPreset?.version_label ?? 'Version' : selectedCustomVersion?.label ?? 'Version'} Copy`);
                    setVersionDescriptionDraft(editingSystemAgent ? selectedPreset?.description ?? '' : selectedCustomVersion?.description ?? '');
                  }}
                >
                  <PlusSignIcon className="h-3.5 w-3.5" />
                  New
                </Button>
              </div>
              <div className="max-h-[30vh] overflow-y-auto px-2 py-2 lg:max-h-none lg:flex-1">
                <div className="space-y-1">
                  {editingSystemAgent ? selectedPresetVersions.map((presetVersion) => {
                    const isSelected = presetVersion.version_key === form.preset_version_key;
                    const isCurrent = presetVersion.version_key === currentSystemVersionKey;
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
                              {isCurrent && (
                                <Badge variant="outline" className="bg-emerald-500/10 text-[9px] px-1.5 py-0 text-emerald-700 dark:text-emerald-400">Active</Badge>
                              )}
                            </div>
                          </div>
                        </button>
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
                            {isWorkspace && presetVersion.id && (
                              <DropdownMenuItem
                                onSelect={() => {
                                  setWorkspaceVersionBeingRenamed(presetVersion);
                                  setRenameLabelDraft(presetVersion.version_label);
                                  setRenameDescriptionDraft(presetVersion.description ?? '');
                                }}
                              >
                                Rename
                              </DropdownMenuItem>
                            )}
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
                            {isWorkspace && presetVersion.id && (
                              <DropdownMenuItem
                                disabled={isCurrent}
                                className="text-destructive focus:text-destructive"
                                onSelect={() => setWorkspaceVersionPendingDelete(presetVersion)}
                              >
                                Delete version
                              </DropdownMenuItem>
                            )}
                          </DropdownMenuContent>
                        </DropdownMenu>
                      </div>
                    );
                  }) : customAgentVersions.map((version) => {
                    const isSelected = version.id === selectedCustomVersionID;
                    const isCurrent = version.id === editingAgent?.active_version_id;
                    return (
                      <div
                        key={version.id}
                        className={cn(
                          'group relative rounded-lg border transition-colors focus-within:ring-2 focus-within:ring-primary/40',
                          isSelected
                            ? 'border-primary/40 bg-background shadow-sm ring-1 ring-primary/20'
                            : 'border-transparent hover:bg-background hover:shadow-sm',
                        )}
                      >
                        <button
                          type="button"
                          onClick={() => selectCustomAgentVersion(version.id)}
                          className="flex w-full items-start gap-2 px-3 py-2 text-left"
                        >
                          <span className={cn(
                            'mt-1.5 inline-block h-1.5 w-1.5 shrink-0 rounded-full',
                            isCurrent ? 'bg-emerald-500' : 'bg-transparent',
                          )} />
                          <div className="min-w-0 flex-1 space-y-0.5">
                            <p className="truncate text-sm font-medium">{version.label}</p>
                            <p className="line-clamp-1 text-[11px] leading-snug text-muted-foreground">
                              {version.description || 'No description'}
                            </p>
                            <div className="flex flex-wrap gap-1 pt-1">
                              {isCurrent && (
                                <Badge variant="outline" className="bg-emerald-500/10 text-[9px] px-1.5 py-0 text-emerald-700 dark:text-emerald-400">Active</Badge>
                              )}
                            </div>
                          </div>
                        </button>
                        <DropdownMenu>
                          <DropdownMenuTrigger asChild>
                            <Button
                              type="button"
                              variant="ghost"
                              size="icon"
                              className="absolute right-1.5 top-1.5 h-6 w-6 text-muted-foreground opacity-50 transition-opacity hover:opacity-100 group-hover:opacity-100 data-[state=open]:opacity-100"
                              aria-label={`Actions for ${version.label}`}
                            >
                              <MoreHorizontalIcon className="h-3.5 w-3.5" />
                            </Button>
                          </DropdownMenuTrigger>
                          <DropdownMenuContent align="end" className="w-44">
                            <DropdownMenuItem
                              onSelect={() => {
                                selectCustomAgentVersion(version.id);
                                setVersionDraftOpen(true);
                                setVersionLabelDraft(`${version.label} Copy`);
                                setVersionDescriptionDraft(version.description ?? '');
                              }}
                            >
                              Duplicate
                            </DropdownMenuItem>
                          </DropdownMenuContent>
                        </DropdownMenu>
                      </div>
                    );
                  })}
                  {editingCustomAgent && customVersionsLoading && (
                    <p className="px-3 py-2 text-xs text-muted-foreground">Loading versions...</p>
                  )}
                </div>
              </div>
            </aside>

            {/* ──────── RIGHT: SELECTED VERSION DETAIL ──────── */}
            <div className="min-h-0 overflow-y-auto px-6 pb-12 pt-6">
              <div className="mx-auto w-full max-w-4xl space-y-6">
                {/* Selected version: header + behaviour strip, visually grouped */}
                <section className="overflow-hidden rounded-xl border border-border/60 bg-card">
                  <div className="flex flex-wrap items-start justify-between gap-3 px-5 py-4">
                    <div className="min-w-0 flex-1 space-y-1.5">
                      <div className="flex flex-wrap items-center gap-2">
                        <h2 className="text-lg font-semibold leading-none">{selectedVersionLabel ?? 'Version'}</h2>
                        {selectedVersionIsActive ? (
                          <Badge variant="outline" className="border-emerald-500/30 bg-emerald-500/10 text-[10px] text-emerald-700 dark:text-emerald-400">Active</Badge>
                        ) : null}
                      </div>
                      {(selectedVersionDescription || canEditSelectedVersionDescription) && (
                        <div className="group/description flex flex-wrap items-center gap-2">
                          <p className={cn(
                            'min-w-0 flex-1 whitespace-normal break-words text-sm text-muted-foreground',
                            !selectedVersionDescription && 'italic',
                          )}>
                            {selectedVersionDescription || 'No description'}
                          </p>
                          {canEditSelectedVersionDescription && selectedPreset?.id ? (
                            <Button
                              type="button"
                              variant="ghost"
                              size="sm"
                              className="h-6 px-1.5 text-xs opacity-0 transition-opacity group-hover/description:opacity-100 group-focus-within/description:opacity-100 focus-visible:opacity-100"
                              onClick={() => {
                                setWorkspaceVersionBeingRenamed(selectedPreset);
                                setRenameLabelDraft(selectedPreset.version_label);
                                setRenameDescriptionDraft(selectedPreset.description ?? '');
                              }}
                            >
                              Edit
                            </Button>
                          ) : null}
                        </div>
                      )}
                    </div>
                    <div className="flex shrink-0 flex-wrap gap-2">
                      <Button
                        type="button"
                        size="sm"
                        disabled={!canSetSelectedVersionActive || saving || hasVersionChanges}
                        onClick={() => {
                          void handleSetActiveVersion(selectedVersionActivationKey);
                        }}
                      >
                        Set as active
                      </Button>
                      <Button
                        type="button"
                        size="sm"
                        variant="outline"
                        onClick={() => {
                          setVersionDraftOpen(true);
                          setVersionLabelDraft(`${selectedVersionLabel ?? 'Version'} Copy`);
                          setVersionDescriptionDraft(selectedVersionDescription ?? '');
                        }}
                      >
                        Duplicate
                      </Button>
                    </div>
                  </div>
                  <dl className="grid grid-cols-2 divide-x divide-y divide-border/40 border-t border-border/40 bg-muted/20 sm:grid-cols-5">
                    <div className="space-y-1 p-3">
                      <dt className="text-[10px] font-medium uppercase tracking-wider text-muted-foreground/70">Model size</dt>
                      <dd className="truncate text-sm font-medium">{agentModelTierLabel(form.model_tier)}</dd>
                    </div>
                    <div className="space-y-1 p-3">
                      <dt className="text-[10px] font-medium uppercase tracking-wider text-muted-foreground/70">Mode</dt>
                      <dd className="truncate text-sm font-medium">{INVOCATION_MODE_LABELS[form.default_invocation_mode]}</dd>
                    </div>
                    <div className="space-y-1 p-3">
                      <dt className="text-[10px] font-medium uppercase tracking-wider text-muted-foreground/70">Created</dt>
                      <dd className="truncate text-sm font-medium" title={formatAgentTimestampTitle(selectedVersionCreatedAt)}>
                        {selectedVersionCreatedAt ? formatAgentTimestamp(selectedVersionCreatedAt) : 'Built-in default'}
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
                    <Badge variant={versionReadOnly ? 'outline' : 'secondary'} className="text-[10px]">
	                      {versionDraftOpen
	                        ? 'New version'
	                        : isEditingWorkspaceVersion || isEditingCustomVersion
	                          ? 'Editable'
	                          : 'Read-only'}
                    </Badge>
                  </div>
                  <div className="space-y-2">
                  {/* 01A — System Prompt */}
                  <Collapsible.Root open={compiledPromptOpen} onOpenChange={setCompiledPromptOpen} className="rounded-xl border border-border/60 bg-card">
                    <Collapsible.Trigger asChild>
                      <button type="button" className="group flex w-full items-center gap-3 px-4 py-3 text-left">
                        <ArrowRight01Icon className="h-3.5 w-3.5 text-muted-foreground transition-transform group-data-[state=open]:rotate-90" />
                        <span className="flex-1 text-sm font-medium">System prompt</span>
                      </button>
                    </Collapsible.Trigger>
                    <Collapsible.Content>
                      <div className="space-y-4 border-t border-border/60 p-4">
                        <div>
                          <Textarea
                            value={versionSystemPromptValue}
                            readOnly={versionReadOnly}
                            onChange={(e) => setForm((current) => ({ ...current, system_prompt: e.target.value }))}
                            rows={12}
                            placeholder="Write the base system instructions that define how this agent should behave."
                            className={cn(
                              'max-h-[36vh] overflow-y-auto rounded-none font-mono text-xs shadow-none [field-sizing:fixed]',
                              versionReadOnly
                                ? 'border-0 bg-transparent px-0 py-1 focus-visible:ring-0'
                                : 'resize-y border border-dashed border-border/60 bg-muted/30 px-3 py-2 focus-visible:border-primary focus-visible:bg-background focus-visible:ring-1 focus-visible:ring-primary',
                            )}
                          />
                        </div>
                      </div>
                    </Collapsible.Content>
                  </Collapsible.Root>

                  {/* 01B — Available Skills */}
                  {(versionSkillItems.length > 0 || !versionReadOnly) && (
                    <Collapsible.Root open={versionSkillsOpen} onOpenChange={setVersionSkillsOpen} className="rounded-xl border border-border/60 bg-card">
                      <div className="flex items-center gap-2 px-4 py-3">
                        <Collapsible.Trigger asChild>
                          <button type="button" className="group flex min-w-0 flex-1 items-center gap-3 text-left">
                            <ArrowRight01Icon className="h-3.5 w-3.5 text-muted-foreground transition-transform group-data-[state=open]:rotate-90" />
                            <span className="flex-1 text-sm font-medium">Available skills</span>
                            {!versionSkillsOpen && (
                              <span className="text-xs text-muted-foreground">{versionSkillItems.length} available</span>
                            )}
                          </button>
                        </Collapsible.Trigger>
                        {versionSkillsOpen && (
                          <Popover open={skillPickerOpen} onOpenChange={setSkillPickerOpen}>
                            <PopoverTrigger asChild>
                              <Button
                                type="button"
                                variant="outline"
                                size="sm"
                                className="h-8 gap-1.5 px-2 text-[11px]"
                                disabled={versionReadOnly || availableVersionSkillEntries.length === 0}
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
                                <CommandGroup heading={`${availableVersionSkillEntries.length} available`}>
                                  {availableVersionSkillEntries.map((skill) => (
                                    <CommandItem
                                      key={skill.key}
                                      value={`${skill.key} ${skill.title} ${skill.description}`}
                                      onSelect={() => addVersionInstructionSkill(skill)}
                                      className="cursor-pointer items-start py-2"
                                    >
                                      <div className="min-w-0 flex-1 space-y-0.5">
                                        <div className="flex items-center gap-2">
                                          <span className="text-xs font-medium text-foreground">{agentSkillDisplayName(skill, skill.key)}</span>
                                          <span className="font-mono text-[10px] text-muted-foreground">{skill.key}</span>
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
                      )}
                    </div>
                    <Collapsible.Content>
                      <div className="space-y-3 border-t border-border/60 p-4">
                        <p className="text-xs text-muted-foreground">
                          These are skills the agent can choose to use when they are relevant to the task.
                        </p>
                        {versionReadOnly ? (
                          <p className="text-xs text-amber-700 dark:text-amber-400">{versionReadOnlyHelperText('skills')}</p>
                        ) : null}
                        <div className="space-y-2">
                          {versionSkillItems.map((skill, idx) => {
                            const entry = skillCatalogEntries.find((s) => (
                              editingCustomAgent ? agentSkillIdentity(s) === skill.identity : s.key === skill.key
                            ));
                            return (
                              <div key={skill.identity} className="flex items-start gap-3 rounded-lg border border-border/50 bg-background px-3 py-2.5">
                                <span className="mt-0.5 flex h-5 w-5 shrink-0 items-center justify-center rounded bg-muted text-[10px] font-semibold text-muted-foreground">{idx + 1}</span>
                                <div className="min-w-0 flex-1">
                                  <div className="flex items-center gap-2">
                                    <BookOpen01Icon className="h-3.5 w-3.5 shrink-0 text-muted-foreground" />
                                    <span className="text-xs font-medium">{agentSkillDisplayName(entry, skill.key)}</span>
                                    <span className="font-mono text-[10px] text-muted-foreground">{skill.key}</span>
                                    {entry && (
                                      <Badge variant="outline" className="text-[9px] px-1.5 py-0">{entry.source_kind === 'built_in' ? 'built-in' : entry.source_kind}</Badge>
                                    )}
                                  </div>
                                  {entry && (
                                    <p className="mt-0.5 text-xs text-muted-foreground line-clamp-2">{entry.description}</p>
                                  )}
                                </div>
                                {!versionReadOnly && (
                                  <button
                                    type="button"
                                    className="mt-0.5 rounded-sm p-1 text-muted-foreground transition-colors hover:bg-muted hover:text-destructive"
                                    onClick={() => removeVersionInstructionSkill(skill.identity)}
                                    aria-label={`Remove ${skill.key}`}
                                  >
                                    <Cancel01Icon className="h-3.5 w-3.5" />
                                  </button>
                                )}
                              </div>
                            );
                          })}
                        </div>
                        {versionSkillItems.length === 0 && (
                          <p className="rounded-md border border-dashed border-border/60 px-3 py-2 text-sm text-muted-foreground">No selectable skills configured.</p>
                        )}
                      </div>
                    </Collapsible.Content>
                  </Collapsible.Root>
                  )}

                  {/* 01D — Allowed Tools */}
                  <Collapsible.Root open={versionToolsOpen} onOpenChange={setVersionToolsOpen} className="rounded-xl border border-border/60 bg-card">
                    <div className="flex items-center gap-2 px-4 py-3">
                      <Collapsible.Trigger asChild>
                        <button type="button" className="group flex min-w-0 flex-1 items-center gap-3 text-left">
                          <ArrowRight01Icon className="h-3.5 w-3.5 text-muted-foreground transition-transform group-data-[state=open]:rotate-90" />
                          <span className="flex-1 text-sm font-medium">Allowed tools</span>
                          {!versionToolsOpen && (
                            <span className="text-xs text-muted-foreground">{form.allowed_tools.length} enabled</span>
                          )}
                        </button>
                      </Collapsible.Trigger>
                      {versionToolsOpen && (
                        <ToolMultiSelectPopover
                          open={toolPickerOpen}
                          onOpenChange={setToolPickerOpen}
                          tools={toolCatalogEntries}
                          selectedTools={form.allowed_tools}
                          connectionsHref={toolConnectionsHref}
                          disabled={!versionToolEditingState.canEdit}
                          disabledReason={versionToolEditingState.disabledReason}
                          onToggleTool={toggleTool}
                        />
                      )}
                    </div>
                    <Collapsible.Content>
                      <div className="space-y-3 border-t border-border/60 p-4">
                        <p className="text-xs text-muted-foreground">
                          Allowed tools are the actions and data sources this version may call. Skills can guide when to use tools, but tools control what the agent can actually do.
                        </p>
                        {versionToolEditingState.disabledReason ? (
                          <p className="text-xs text-amber-700 dark:text-amber-400">{versionToolEditingState.disabledReason}</p>
                        ) : null}
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
                                        {versionToolEditingState.canEdit && (
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
                                      {versionToolEditingState.canEdit && (
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

                  {/* 01E — Working Areas */}
                  <Collapsible.Root defaultOpen={false} className="rounded-xl border border-border/60 bg-card">
                    <Collapsible.Trigger asChild>
                      <button type="button" className="group flex w-full items-center gap-3 px-4 py-3 text-left">
                        <ArrowRight01Icon className="h-3.5 w-3.5 text-muted-foreground transition-transform group-data-[state=open]:rotate-90" />
                        <span className="flex-1 text-sm font-medium">Working areas</span>
                        <span className="text-xs text-muted-foreground group-data-[state=open]:hidden">{form.allowed_targets.length} selected</span>
                      </button>
                    </Collapsible.Trigger>
                    <Collapsible.Content>
                      <div className="space-y-3 border-t border-border/60 p-4">
                        <p className="text-xs text-muted-foreground">
                          Working areas control where this agent appears as a runnable option.
                        </p>
                        {versionReadOnly ? (
                          <p className="text-xs text-amber-700 dark:text-amber-400">{versionReadOnlyHelperText('working areas')}</p>
                        ) : null}
                        <div className="grid gap-2 sm:grid-cols-2 md:grid-cols-3">
                          {CUSTOM_AGENT_TARGET_OPTIONS.map((target) => {
                            const active = form.allowed_targets.includes(target.value);
                            return (
                              <button
                                key={target.value}
                                type="button"
                                disabled={versionReadOnly}
                                onClick={() => toggleTarget(target.value)}
                                className={cn(
                                  'rounded-md border px-3 py-2 text-left text-sm transition-colors disabled:cursor-default',
                                  active
                                    ? 'border-primary bg-primary/10 text-foreground'
                                    : versionReadOnly
                                      ? 'border-border bg-card text-muted-foreground'
                                      : 'border-border bg-card hover:bg-muted/40',
                                )}
                              >
                                {target.label}
                              </button>
                            );
                          })}
                        </div>
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
                      <div className="space-y-2 border-t border-border/60 px-4 py-3">
                        <Select
                          value={form.default_invocation_mode}
                          disabled={versionReadOnly}
                          onValueChange={(value) => {
                            setForm((current) => ({
                              ...current,
                              supported_modes: supportedModesForForm(current.runtime_kind),
                              default_invocation_mode: value as AgentInvocationMode,
                            }));
                          }}
                        >
                          <SelectTrigger className="h-9 max-w-sm" aria-label="Run mode">
                            <SelectValue />
                          </SelectTrigger>
                          <SelectContent>
                            {supportedModesForForm(form.runtime_kind).map((mode) => (
                              <SelectItem key={mode} value={mode}>
                                {INVOCATION_MODE_LABELS[mode]}
                              </SelectItem>
                            ))}
                          </SelectContent>
                        </Select>
                        <p className="text-xs text-muted-foreground">
                          {form.default_invocation_mode === 'interactive'
                            ? 'Can ask follow-up questions or request approval.'
                            : 'Runs autonomously end-to-end.'}
                        </p>
                        {versionReadOnly ? (
                          <p className="text-xs text-amber-700 dark:text-amber-400">{versionReadOnlyHelperText('run mode')}</p>
                        ) : null}
                      </div>
                    </Collapsible.Content>
                  </Collapsible.Root>

                  {/* 03 — Execution */}
                  <Collapsible.Root defaultOpen={false} className="rounded-xl border border-border/60 bg-card">
                    <Collapsible.Trigger asChild>
                      <button type="button" className="group flex w-full items-center gap-3 px-4 py-3 text-left">
                        <ArrowRight01Icon className="h-3.5 w-3.5 text-muted-foreground transition-transform group-data-[state=open]:rotate-90" />
                        <span className="flex-1 text-sm font-medium">Model size</span>
                        <span className="text-xs text-muted-foreground group-data-[state=open]:hidden">{agentModelTierLabel(form.model_tier)}</span>
                      </button>
                    </Collapsible.Trigger>
                    <Collapsible.Content>
                      <div className="border-t border-border/60 p-4">
                        <div className="max-w-lg space-y-2">
                          <FieldLabel>Model size</FieldLabel>
                          <Select
                            value={form.model_tier}
                            disabled={editingSystemAgent || versionReadOnly}
                            onValueChange={(value) => setForm((current) => ({ ...current, model_tier: value as AgentModelTier }))}
                          >
                            <SelectTrigger className="h-9" aria-label="Model size">
                              <SelectValue />
                            </SelectTrigger>
                            <SelectContent>
                              {AGENT_MODEL_TIER_OPTIONS.map((tier) => (
                                <SelectItem key={tier.value} value={tier.value}>{tier.label}</SelectItem>
                              ))}
                            </SelectContent>
                          </Select>
                          <p className="text-[11px] leading-relaxed text-muted-foreground">
                            {AGENT_MODEL_TIER_OPTIONS.find((tier) => tier.value === form.model_tier)?.description}
                          </p>
                          {editingSystemAgent ? (
                            <p className="text-[11px] leading-relaxed text-muted-foreground">Built-in model sizes are managed by Helpin.</p>
                          ) : null}
                        </div>
                      </div>
                    </Collapsible.Content>
                  </Collapsible.Root>

                  </div>
                </section>

              </div>
            </div>
          </div>
            </TabsContent>
          </Tabs>

          {systemDrawerTab === 'versions' && (
          <SheetFooter className="border-t border-border/60 bg-background py-4 pl-6 pr-20 sm:flex-row sm:justify-between">
            <div className="text-xs text-muted-foreground">
              {versionDraftOpen
                ? 'Create the new version in the dialog.'
	                : isEditingWorkspaceVersion || isEditingCustomVersion
	                  ? (hasVersionChanges ? `Editing ${selectedVersionLabel ?? 'version'}. Save changes to apply.` : 'No changes to save.')
	                  : 'No changes to save.'}
            </div>
            <div className="flex gap-2">
              <Button variant="outline" size="sm" onClick={() => setSystemDrawerOpen(false)}>
	                {hasVersionChanges || versionDraftOpen ? 'Cancel' : 'Close'}
              </Button>
              <Button
                size="sm"
                disabled={
                  saving
                  || !(isEditingWorkspaceVersion || isEditingCustomVersion)
                  || !hasVersionChanges
                }
                onClick={() => handleSaveWorkspaceVersion()}
              >
                {saving ? 'Saving…' : 'Save'}
              </Button>
            </div>
          </SheetFooter>
          )}
        </SheetContent>
      </Sheet>
  );

  const renderTemplateSetupDialog = () => (
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

          {templateDraft?.template.key === SECURITY_TRIAGE_TEMPLATE_KEY && (
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
                      <FieldLabel htmlFor="security-setup-repository">Repository</FieldLabel>
                      <Select
                        value={securityTriageTemplateForm.repository_id || undefined}
                        onValueChange={(value) => setSecurityTriageTemplateForm((current) => ({
                          ...current,
                          repository_id: value,
                        }))}
                      >
                        <SelectTrigger id="security-setup-repository">
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
                      <FieldLabel htmlFor="security-setup-cadence">Run cadence</FieldLabel>
                      <Select
                        value={securityTriageTemplateForm.schedule_preset}
                        onValueChange={(value) => {
                          setSecurityTriageTemplateForm((current) => ({
                            ...current,
                            schedule_preset: value === 'daily' ? 'daily' : 'weekly',
                          }));
                        }}
                      >
                        <SelectTrigger id="security-setup-cadence">
                          <SelectValue />
                        </SelectTrigger>
                        <SelectContent>
                          {SECURITY_TRIAGE_SCHEDULE_OPTIONS.map((option) => (
                            <SelectItem key={option.value} value={option.value}>
                              {option.label}
                            </SelectItem>
                          ))}
                        </SelectContent>
                      </Select>
                    </div>
                  </div>

                  <div className="grid gap-2">
                    <FieldLabel>Scanners</FieldLabel>
                    <div className="grid gap-2 sm:grid-cols-3">
                      {SECURITY_TRIAGE_SCANNER_OPTIONS.map((option) => {
                        const checked = securityTriageTemplateForm.scanners.includes(option.value);
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
                              onCheckedChange={(nextChecked) => setSecurityTriageTemplateForm((current) => {
                                const currentValues = new Set(current.scanners);
                                if (nextChecked) {
                                  currentValues.add(option.value);
                                } else {
                                  currentValues.delete(option.value);
                                }
                                return {
                                  ...current,
                                  scanners: SECURITY_TRIAGE_SCANNER_OPTIONS
                                    .map((item) => item.value)
                                    .filter((value) => currentValues.has(value)),
                                };
                              })}
                            />
                          </label>
                        );
                      })}
                    </div>
                    <p className="text-[11px] text-muted-foreground">
                      Sentinel runs selected scanners through read-only commands, then triages findings before creating tasks.
                    </p>
                  </div>

                  <div className="grid gap-4 sm:grid-cols-3">
                    <div className="grid gap-2">
                      <FieldLabel htmlFor="security-setup-severity">Minimum severity</FieldLabel>
                      <Select
                        value={securityTriageTemplateForm.severity_threshold}
                        onValueChange={(value: SecurityTriageSeverity) => setSecurityTriageTemplateForm((current) => ({
                          ...current,
                          severity_threshold: value,
                        }))}
                      >
                        <SelectTrigger id="security-setup-severity">
                          <SelectValue />
                        </SelectTrigger>
                        <SelectContent>
                          {SECURITY_TRIAGE_SEVERITY_OPTIONS.map((option) => (
                            <SelectItem key={option.value} value={option.value}>
                              {option.label}
                            </SelectItem>
                          ))}
                        </SelectContent>
                      </Select>
                    </div>

                    <div className="grid gap-2">
                      <FieldLabel htmlFor="security-setup-max-tasks">Maximum tasks per run</FieldLabel>
                      <Input
                        id="security-setup-max-tasks"
                        type="number"
                        min={1}
                        max={100}
                        value={securityTriageTemplateForm.max_tasks}
                        onChange={(event) => setSecurityTriageTemplateForm((current) => ({
                          ...current,
                          max_tasks: event.target.value,
                        }))}
                      />
                    </div>

                    <div className="flex items-center justify-between gap-3 rounded-lg border border-border/60 bg-muted/20 px-3 py-2.5">
                      <div className="space-y-0.5">
                        <p className="text-sm font-medium">Low/info findings</p>
                        <p className="text-[11px] text-muted-foreground">Summarize only by default</p>
                      </div>
                      <Switch
                        checked={securityTriageTemplateForm.include_low_info}
                        onCheckedChange={(checked) => setSecurityTriageTemplateForm((current) => ({
                          ...current,
                          include_low_info: checked,
                        }))}
                      />
                    </div>
                  </div>

                  <div className="grid gap-4 sm:grid-cols-2">
                    <div className="grid gap-2">
                      <FieldLabel htmlFor="security-setup-team">Task team</FieldLabel>
                      <Select
                        value={securityTriageTemplateForm.destination_team_id || undefined}
                        onValueChange={(value) => {
                          setSecurityTriageTemplateForm((current) => ({
                            ...current,
                            destination_team_id: value,
                            destination_state_id: NONE_OPTION_VALUE,
                          }));
                          void loadCompetitiveTeamWorkflow(value);
                        }}
                      >
                        <SelectTrigger id="security-setup-team">
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
                      <FieldLabel htmlFor="security-setup-state">Task stage</FieldLabel>
                      <Select
                        value={securityTriageTemplateForm.destination_state_id}
                        onValueChange={(value) => setSecurityTriageTemplateForm((current) => ({
                          ...current,
                          destination_state_id: value,
                        }))}
                        disabled={!securityTriageTemplateForm.destination_team_id}
                      >
                        <SelectTrigger id="security-setup-state">
                          <SelectValue placeholder={securityTriageTemplateForm.destination_team_id ? 'Default stage' : 'Select a team first'} />
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
  );

  const renderAgentEditor = () => (
<Sheet
        open={dialogOpen}
        onOpenChange={(open) => {
          if (open) {
            setDialogOpen(true);
            return;
          }
          requestCloseCustomAgentDrawer();
        }}
      >
        <SheetContent
          side="right"
          className={cn(
            'w-full gap-0 p-0',
            isBlankCustomCreate
              ? 'data-[side=right]:w-full data-[side=right]:sm:w-[880px] data-[side=right]:!max-w-[880px]'
              : 'data-[side=right]:w-[88vw] data-[side=right]:sm:max-w-[88vw] xl:data-[side=right]:w-[1280px] xl:data-[side=right]:max-w-[1280px]',
          )}
        >
          {isBlankCustomCreate || isCustomEdit ? (
            <Suspense fallback={<div className="p-6 text-sm text-muted-foreground">Loading editor…</div>}>
              <CustomAgentCreatePanel
                workspaceId={workspaceId ?? ''}
                form={form}
                onChange={setForm}
                teams={visibleTeams}
                tools={toolCatalogEntries}
                skills={skillCatalogEntries}
                advancedOpen={advancedOpen}
                onAdvancedOpenChange={setAdvancedOpen}
                onCreate={handleSave}
                saving={saving}
                mode={isCustomEdit ? 'edit' : 'create'}
                canSave={isCustomEdit ? customEditHasChanges : undefined}
                statusText={isCustomEdit ? (customEditHasChanges ? 'Unsaved changes' : 'No changes to save') : undefined}
              />
            </Suspense>
          ) : (
            <>
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
                    <p className={cn('text-xs font-medium', createDrawerCanSave ? 'text-emerald-700 dark:text-emerald-400' : createDrawerReady ? 'text-muted-foreground' : 'text-amber-700 dark:text-amber-400')}>
                      {createDrawerCanSave ? 'Ready' : createDrawerReady ? 'No changes' : `${createDrawerMissingRequirements.length} missing`}
                    </p>
                  </div>
                </div>
                <dl className="grid grid-cols-2 divide-x divide-y divide-border/40 border-t border-border/40 bg-muted/20 sm:grid-cols-4 lg:grid-cols-8">
                  <div className="space-y-1 p-3">
                    <dt className="text-[10px] font-medium uppercase tracking-wider text-muted-foreground/70">Model size</dt>
                    <dd className="truncate text-sm font-medium">{agentModelTierLabel(form.model_tier)}</dd>
                  </div>
                  <div className="space-y-1 p-3">
                    <dt className="text-[10px] font-medium uppercase tracking-wider text-muted-foreground/70">Mode</dt>
                    <dd className="truncate text-sm font-medium">{INVOCATION_MODE_LABELS[form.default_invocation_mode]}</dd>
                  </div>
                  <div className="space-y-1 p-3">
                    <dt className="text-[10px] font-medium uppercase tracking-wider text-muted-foreground/70">Tools · Available skills</dt>
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
                  <div className="space-y-1 p-3">
                    <dt className="text-[10px] font-medium uppercase tracking-wider text-muted-foreground/70">Created</dt>
                    <dd className="truncate text-sm font-medium" title={formatAgentTimestampTitle(editingAgent?.created_at)}>
                      {formatAgentTimestamp(editingAgent?.created_at)}
                    </dd>
                  </div>
                  <div className="space-y-1 p-3">
                    <dt className="text-[10px] font-medium uppercase tracking-wider text-muted-foreground/70">Last edited</dt>
                    <dd className="truncate text-sm font-medium" title={formatAgentTimestampTitle(editingAgent?.updated_at)}>
                      {formatAgentTimestamp(editingAgent?.updated_at)}
                    </dd>
                  </div>
                </dl>
              </section>

              <DrawerConfigSection title="Identity" description="Name, team, and operating scope">
                {editingAgent?.preset_key === 'crm_operator' && workspace && <BeaconPlaybookUsage ws={workspace.id} slug={workspace.slug} agentId={editingAgent.id} />}
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
                        onValueChange={(value) => setForm((current) => {
                          const teamID = value === '_none' ? '' : value;
                          return {
                            ...current,
                            team_id: teamID,
                            teamAccessMode: teamID ? 'specific_teams' : 'all_teams',
                            team_ids: teamID ? [teamID] : [],
                          };
                        })}
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
                      <ExpandIcon className="h-3.5 w-3.5" />
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
                      <p className="text-sm font-medium">Available skills</p>
                      <p className="text-[11px] text-muted-foreground">Reusable guides the agent can discover and read during a run.</p>
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
                                  key={agentSkillIdentity(skill)}
                                  value={`${skill.key} ${skill.title} ${skill.description}`}
                                  onSelect={() => addSkill(skill)}
                                  className="cursor-pointer items-start py-2"
                                >
                                  <div className="min-w-0 flex-1 space-y-0.5">
                                    <div className="flex items-center gap-2">
                                      <span className="text-xs font-medium text-foreground">{agentSkillDisplayName(skill, skill.key)}</span>
                                      <span className="font-mono text-[10px] text-muted-foreground">{skill.key}</span>
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
                        const entry = skillCatalogEntries.find((s) => agentSkillIdentity(s) === agentSkillIdentity(ref));
                        return (
                          <Badge key={agentSkillIdentity(ref)} variant="secondary" className="gap-1.5 pr-1 text-[11px]">
                            <BookOpen01Icon className="h-3 w-3 text-muted-foreground" />
                            <span>{agentSkillDisplayName(entry, ref.key)}</span>
                            <span className="font-mono text-[9px] text-muted-foreground/70">{ref.key}</span>
                            {entry?.source_kind && (
                              <span className="text-[9px] text-muted-foreground/70">{entry.source_kind === 'built_in' ? 'built-in' : entry.source_kind}</span>
                            )}
                            <button
                              type="button"
                              className="rounded-sm p-0.5 text-muted-foreground transition-colors hover:bg-background hover:text-foreground"
                              onClick={() => removeSkill(agentSkillIdentity(ref))}
                              aria-label={`Remove ${agentSkillDisplayName(entry, ref.key)}`}
                            >
                              <Cancel01Icon className="h-3 w-3" />
                            </button>
                          </Badge>
                        );
                      })}
                    </div>
                  ) : (
                    <p className="text-xs text-muted-foreground">No available skills attached. Skills provide reusable guidance without expanding the system prompt by default.</p>
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
                  <FieldLabel tooltip="Templates use a Helpin-managed model size so capability and billing remain predictable.">Model size</FieldLabel>
                  <Select
                    value={form.model_tier}
                    disabled
                  >
                    <SelectTrigger className="h-9" aria-label="Model size">
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                      {AGENT_MODEL_TIER_OPTIONS.map((tier) => (
                        <SelectItem key={tier.value} value={tier.value}>{tier.label}</SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                  <p className="text-[11px] leading-relaxed text-muted-foreground">
                    {AGENT_MODEL_TIER_OPTIONS.find((tier) => tier.value === form.model_tier)?.description}
                  </p>
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
                        connectionsHref={toolConnectionsHref}
                        onToggleTool={toggleTool}
                      />
                    </div>
                    <p className="text-[11px] leading-relaxed text-muted-foreground">
                      Choose from the workspace tool catalog. Selected tools become this agent&apos;s allowed tool list.
                    </p>
                    <div className="flex max-h-40 flex-wrap gap-1.5 overflow-y-auto rounded-lg border border-border/50 bg-muted/20 p-2.5">
                      {form.allowed_tools.length > 0 ? form.allowed_tools.map((tool) => (
                        <Badge key={tool} variant="secondary" className="gap-1 pr-1 font-mono text-[11px]">
                          <span>{tool}</span>
                          <button
                            type="button"
                            className="rounded-sm p-0.5 text-muted-foreground transition-colors hover:bg-background hover:text-foreground"
                            onClick={() => removeTool(tool)}
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
                    Run behavior
                  </span>
                </Button>
              </Collapsible.Trigger>
              <Collapsible.Content className="space-y-4 rounded-md border bg-muted/30 p-3 mt-2">
                <div className="space-y-2">
                  <FieldLabel
                    htmlFor="agent-approval-policy"
                    tooltip="Choose when this agent needs a person to approve its work."
                  >
                    Approval policy
                  </FieldLabel>
                  <Select
                    value={form.approval_mode}
                    onValueChange={(value) => setForm((current) => ({
                      ...current,
                      approval_mode: value as AgentApprovalMode,
                    }))}
                  >
                    <SelectTrigger id="agent-approval-policy">
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                      {AGENT_APPROVAL_OPTIONS.map((option) => (
                        <SelectItem key={option.value} value={option.value}>
                          {option.label}
                        </SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                  <p className="text-xs leading-relaxed text-muted-foreground">
                    {agentApprovalDescription(form.approval_mode)}
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

            {/* ---- Advanced limits ---- */}
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
                disabled={saving || !createDrawerCanSave}
                onClick={handleSave}
              >
                {createDrawerPrimaryLabel}
              </Button>
            </div>
          </SheetFooter>
            </>
          )}
        </SheetContent>
      </Sheet>
  );

  if (!workspace) {
    return <p className="text-sm text-muted-foreground">Workspace not found.</p>;
  }

  return (
    <AutomationShell
        title="Agents"
        description="Built-in and custom agents for manual runs and automated flows."
        actions={sortedAgents.length > 0 ? (
          <>
            <div className="flex items-center gap-1">
              <QuietIconAction
                aria-label="Show agents as a list"
                aria-pressed={viewMode === 'list'}
                className={viewMode === 'list' ? 'bg-quiet-hover text-quiet-text-primary' : undefined}
                onClick={() => setViewMode('list')}
              >
                <LayoutTable01Icon className="h-4 w-4" />
              </QuietIconAction>
              <QuietIconAction
                aria-label="Show agents as a grid"
                aria-pressed={viewMode === 'cards'}
                className={viewMode === 'cards' ? 'bg-quiet-hover text-quiet-text-primary' : undefined}
                onClick={() => setViewMode('cards')}
              >
                <LayoutGridIcon className="h-4 w-4" />
              </QuietIconAction>
            </div>
            {canEdit && (
              <QuietPrimaryAction className="gap-1.5" onClick={openCreateDialog}>
                <PlusSignIcon className="h-4 w-4" />
                New Custom Agent
              </QuietPrimaryAction>
            )}
          </>
        ) : undefined}
    >

      {loading && <p className="text-sm text-muted-foreground">Loading agents...</p>}
      {error && <p className="text-sm text-destructive">{error}</p>}

      {/* ---- Empty state with onboarding ---- */}
      {!loading && visibleAgents.length === 0 && !error && (
        <QuietEmptyState
          title="Create your first agent"
          description="AI-powered teammates can plan features, write code, review work, update docs, reply to customers, and manage deals—automatically or on demand."
          action={canEdit ? (
              <QuietPrimaryAction className="gap-1.5" onClick={openCreateDialog}>
                <PlusSignIcon className="h-4 w-4" />
                New Custom Agent
              </QuietPrimaryAction>
          ) : undefined}
        >
          <div className="max-w-[760px] border-t border-quiet-divider-light">
            {EMPTY_STATE_CARDS.map((card) => (
              <div key={card.title} className="grid grid-cols-[20px_minmax(0,1fr)] gap-x-3 border-b border-quiet-divider-light py-3">
                <card.icon className="mt-0.5 h-[15px] w-[15px] text-quiet-muted" />
                <div>
                  <p className="text-[13.5px] font-semibold tracking-[-0.008em] text-quiet-text-primary">{card.title}</p>
                  <p className="mt-0.5 text-[12.5px] leading-5 text-quiet-text-tertiary">{card.desc}</p>
                </div>
              </div>
            ))}
          </div>
        </QuietEmptyState>
      )}

      {/* ---- Agent list / grid ---- */}
      {sortedAgents.length > 0 && viewMode === 'list' && (
        <AgentsListTable>
          <AgentsListHeader />
          {sortedAgents.map((agent) => (
            <AgentRow
              key={agent.id}
              agent={agent}
              stats={runStats[agent.id]}
              usage={agentUsageMap[agent.id]}
              presets={presets}
              onOpen={openEditDialog}
              onOpenRun={openRunDetails}
              onRunNow={openRunNowDialog}
              onDelete={openDeleteDialog}
              canEdit={canEdit}
              workspaceSlug={workspace?.slug}
            />
          ))}
        </AgentsListTable>
      )}

      {sortedAgents.length > 0 && viewMode === 'cards' && (
        <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
          {sortedAgents.map((agent) => (
            <AgentCard
              key={agent.id}
              agent={agent}
              stats={runStats[agent.id]}
              usage={agentUsageMap[agent.id]}
              presets={presets}
              onOpen={openEditDialog}
              onOpenRun={openRunDetails}
              onRunNow={openRunNowDialog}
              onDelete={openDeleteDialog}
              canEdit={canEdit}
              workspaceSlug={workspace?.slug}
            />
          ))}
        </div>
      )}

      {runNowOpen ? (
        <Suspense fallback={null}>
          <AgentRunNowDialog
            open
            onOpenChange={(open) => {
              setRunNowOpen(open);
              if (!open) setRunNowAgent(null);
            }}
            agent={runNowAgent}
            targets={runNowTargets}
            targetLabel={labelForAgentTarget}
            targetType={runNowTargetType}
            onTargetTypeChange={handleRunNowTargetChange}
            workspaceName={workspace.name}
            workspaceId={workspace.id}
            repositories={repositories}
            runnableRepositories={runnableRepositories}
            repositoriesLoading={runNowRepositoriesLoading}
            repositoriesSettingsHref={repositoriesSettingsHref}
            targetId={runNowTargetId}
            onTargetIdChange={setRunNowTargetId}
            baseBranch={runNowBaseBranch}
            onBaseBranchChange={setRunNowBaseBranch}
            selectedRepository={selectedRunNowRepository}
            additionalContext={runNowAdditionalContext}
            onAdditionalContextChange={setRunNowAdditionalContext}
            submitting={runNowSubmitting}
            canSubmit={runNowCanSubmit}
            onSubmit={() => void handleRunNow()}
          />
        </Suspense>
      ) : null}

      {renderSystemDrawer()}

      {/* ---- New custom version dialog ---- */}
      <Dialog
        open={versionDraftOpen}
        onOpenChange={(open) => {
          if (!open) {
            setVersionDraftOpen(false);
            setVersionLabelDraft('');
            setVersionDescriptionDraft('');
	            if (editingAgent) {
	              if (editingAgent.is_system) {
	                setForm(buildSystemAgentForm(editingAgent, presets));
	              } else if (selectedCustomVersion) {
	                setForm(buildCustomAgentVersionForm(editingAgent, selectedCustomVersion));
	              }
	            }
          }
        }}
      >
        <DialogContent className="sm:max-w-[520px]">
          <DialogHeader>
            <DialogTitle>New custom version</DialogTitle>
            <DialogDescription>
              Creates a custom version from the current configuration. Review it first, then set it as active when you are ready.
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
	                  if (editingAgent.is_system) {
	                    setForm(buildSystemAgentForm(editingAgent, presets));
	                  } else if (selectedCustomVersion) {
	                    setForm(buildCustomAgentVersionForm(editingAgent, selectedCustomVersion));
	                  }
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
              {creatingVersion ? 'Creating…' : 'Create custom version'}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {renderTemplateSetupDialog()}

      {/* ---- Create / Edit drawer ---- */}
      {renderAgentEditor()}

      <Dialog open={systemPromptEditorOpen} onOpenChange={setSystemPromptEditorOpen}>
        <DialogContent className="z-[140] max-h-[88vh] gap-0 overflow-hidden p-0 sm:max-w-5xl">
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
              className="h-[62vh] overflow-y-auto resize-none font-mono text-xs leading-relaxed [field-sizing:fixed]"
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
        description={
          deleteAgentFlowCount > 0 ? (
            <span className="space-y-2">
              <span className="block">
                This agent is used by{' '}
                <span className="font-medium text-destructive">
                  {deleteAgentFlowCount} {deleteAgentFlowCount === 1 ? 'flow' : 'flows'}
                </span>
                . Deleting it will also delete those flows.
              </span>
              <span className="block">This action cannot be undone.</span>
            </span>
          ) : (
            'This will permanently remove this agent and its direct configuration. This action cannot be undone.'
          )
        }
        confirmLabel={deleteAgentFlowCount > 0 ? 'Delete agent and flows' : 'Delete'}
        variant="destructive"
        onConfirm={handleDelete}
      />

      <ConfirmDialog
        open={discardCustomEditConfirmOpen}
        onOpenChange={setDiscardCustomEditConfirmOpen}
        title="Discard unsaved changes?"
        description="You have unsaved changes to this agent. Closing now will discard them."
        confirmLabel="Discard changes"
        variant="destructive"
        onConfirm={() => {
          setDiscardCustomEditConfirmOpen(false);
          closeCustomAgentDrawer();
        }}
      />

      <ConfirmDialog
        open={workspaceVersionPendingDelete !== null}
        onOpenChange={(open) => !open && setWorkspaceVersionPendingDelete(null)}
        title="Delete custom version"
        description={
          workspaceVersionPendingDelete
            ? `This will permanently delete "${workspaceVersionPendingDelete.version_label}". Agents using this version will need a new active version. This action cannot be undone.`
            : ''
        }
        confirmLabel={deletingVersion ? 'Deleting…' : 'Delete custom version'}
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
            <DialogTitle>Edit custom version</DialogTitle>
            <DialogDescription>
              Update the label or description for this custom version. Changes apply immediately.
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

      {runDrawerOpen && selectedRunId ? (
        <Suspense fallback={null}>
          <CodingSessionDrawer
            sessionId={selectedRunId}
            open
            onOpenChange={(open) => {
              setRunDrawerOpen(open);
              if (!open) setSelectedRunId(null);
            }}
            title="Agent Run"
            description="Interactive transcript, approvals, artifacts, and session details."
          />
        </Suspense>
      ) : null}
      <UpgradeRequiredDialog
        open={upgradeDialogReason !== null}
        onOpenChange={(open) => {
          if (!open) setUpgradeDialogReason(null);
        }}
        onUpgrade={() => {
          setDialogOpen(false);
          setTemplateDialogOpen(false);
          setTemplateSetupDialogOpen(false);
          setTemplateDraft(null);
          setRunNowOpen(false);
          setRunNowAgent(null);
          setToolPickerOpen(false);
          setSkillPickerOpen(false);
          setSystemPromptEditorOpen(false);
        }}
        reason={upgradeDialogReason}
      />
    </AutomationShell>
  );
}
