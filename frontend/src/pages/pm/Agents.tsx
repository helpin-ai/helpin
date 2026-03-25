import { useCallback, useEffect, useState } from 'react';
import { Collapsible } from 'radix-ui';
import { formatDistanceToNow } from 'date-fns';
import {
  Bot,
  ChevronDown,
  ChevronRight,
  Clock,
  HelpCircle,
  LayoutGrid,
  LayoutList,
  Pencil,
  Plus,
  Users,
  X,
  Zap,
} from 'lucide-react';
import { ConfirmDialog } from '@/components/pm/ConfirmDialog';
import { useTitle } from '@/hooks/useTitle';
import { useWorkspaceStore } from '@/stores/workspaceStore';
import { useWorkspaceAccess, usePermissions } from '@/hooks/queries/useSession';
import { useWorkspaceSettings } from '@/hooks/queries/useSettings';
import { agentService } from '@/lib/services/agentService';
import { AGENT_RUNTIME_LABELS } from '@/lib/agentRuntime';
import type {
  Agent,
  AgentPresetDefinition,
  AgentPresetKey,
  AgentApprovalMode,
  AgentInvocationMode,
  AgentModelProvider,
  AgentModelProviderOption,
  AgentRun,
  AgentRuntimeKind,
  CreateAgentRequest,
  ToolCatalogResponse,
  UpdateAgentRequest,
} from '@/lib/pmTypes';
import { Button } from '@/components/ui/button';
import { Badge } from '@/components/ui/badge';
import { Card, CardContent, CardHeader } from '@/components/ui/card';
import { Progress } from '@/components/ui/progress';
import {
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
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
const DEFAULT_PRESET_KEY: AgentPresetKey = 'code_builder';
const PRESET_ORDER: AgentPresetKey[] = [
  'epic_planner',
  'story_planner',
  'crm_operator',
  'support_agent',
  'code_builder',
  'review_agent',
];

const PRESET_FALLBACKS: Record<AgentPresetKey, { label: string; description: string; runtime_kind: AgentRuntimeKind; default_invocation_mode: AgentInvocationMode; supported_modes: AgentInvocationMode[] }> = {
  epic_planner: {
    label: 'Epic Planner',
    description: 'Interactive product planning for epics, PRDs, docs, and stories.',
    runtime_kind: 'native_sdk',
    default_invocation_mode: 'interactive',
    supported_modes: ['autonomous', 'interactive'],
  },
  story_planner: {
    label: 'Story Planner',
    description: 'Interactive decomposition and refinement for stories and execution plans.',
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
    runtime_kind: 'opencode',
    default_invocation_mode: 'autonomous',
    supported_modes: ['autonomous'],
  },
  review_agent: {
    label: 'Review Agent',
    description: 'Reviews work, runs tests, and checks quality without repo mutation.',
    runtime_kind: 'opencode',
    default_invocation_mode: 'autonomous',
    supported_modes: ['autonomous'],
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
    icon: Zap,
    title: 'Automate work',
    desc: 'Handle planning, coding, doc updates, support replies, and deal management so your team can focus on what matters.',
  },
  {
    icon: Bot,
    title: 'Cross-module',
    desc: 'Agents can span projects, CRM, support, and docs — just pick the capabilities they need.',
  },
  {
    icon: Users,
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
  runtime_kind: AgentRuntimeKind;
  provider: AgentModelProvider;
  model: string;
  system_prompt: string;
  monthly_token_budget: string;
  team_id: string;
  allowed_tools: string[];
  schedule: string;
  approval_mode: AgentApprovalMode;
  max_concurrent_runs: string;
  default_invocation_mode: AgentInvocationMode;
}

const FALLBACK_PROVIDER_OPTIONS: AgentModelProviderOption[] = [
  { value: 'anthropic', label: 'Anthropic', model_placeholder: 'claude-sonnet-4-20250514' },
  { value: 'openai', label: 'OpenAI', model_placeholder: 'gpt-5-mini' },
  { value: 'openrouter', label: 'OpenRouter', model_placeholder: 'openai/gpt-5-mini' },
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
  return DEFAULT_PRESET_KEY;
}

function presetMetaForKey(presetKey: AgentPresetKey, presets: AgentPresetDefinition[]): AgentPresetDefinition | null {
  return presets.find((preset) => preset.key === presetKey) ?? null;
}

function presetLabel(presetKey: AgentPresetKey, presets: AgentPresetDefinition[]): string {
  return presetMetaForKey(presetKey, presets)?.label ?? PRESET_FALLBACKS[presetKey].label;
}

function presetDescription(presetKey: AgentPresetKey, presets: AgentPresetDefinition[]): string {
  return presetMetaForKey(presetKey, presets)?.description ?? PRESET_FALLBACKS[presetKey].description;
}

function presetRuntimeKind(presetKey: AgentPresetKey, presets: AgentPresetDefinition[]): AgentRuntimeKind {
  return presetMetaForKey(presetKey, presets)?.runtime_kind ?? PRESET_FALLBACKS[presetKey].runtime_kind;
}

function presetDefaultInvocationMode(presetKey: AgentPresetKey, presets: AgentPresetDefinition[]): AgentInvocationMode {
  return presetMetaForKey(presetKey, presets)?.default_invocation_mode ?? PRESET_FALLBACKS[presetKey].default_invocation_mode;
}

function presetAllowedTools(presetKey: AgentPresetKey, presets: AgentPresetDefinition[]): string[] {
  return normalizeToolList(presetMetaForKey(presetKey, presets)?.allowed_tools ?? []);
}

function presetSupportedModes(presetKey: AgentPresetKey, runtimeKind: AgentRuntimeKind, presets: AgentPresetDefinition[]): AgentInvocationMode[] {
  const preset = presetMetaForKey(presetKey, presets);
  if (preset && preset.runtime_kind === runtimeKind && preset.supported_modes.length > 0) {
    return preset.supported_modes;
  }
  return supportedModesForForm(runtimeKind);
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

function toolListsEqual(left: string[], right: string[]): boolean {
  const normalizedLeft = normalizeToolList(left);
  const normalizedRight = normalizeToolList(right);
  if (normalizedLeft.length !== normalizedRight.length) {
    return false;
  }
  return normalizedLeft.every((tool, index) => tool === normalizedRight[index]);
}

function createEmptyForm(presetKey: AgentPresetKey = DEFAULT_PRESET_KEY, preset?: AgentPresetDefinition | null): AgentFormData {
  const runtimeKind = preset?.runtime_kind ?? PRESET_FALLBACKS[presetKey].runtime_kind;
  const defaultInvocationMode = preset?.default_invocation_mode ?? PRESET_FALLBACKS[presetKey].default_invocation_mode;
  const provider = normalizeProviderForRuntime(runtimeKind, 'anthropic');
  return {
    name: '',
    preset_key: presetKey,
    runtime_kind: runtimeKind,
    provider,
    model: '',
    system_prompt: preset?.system_prompt ?? '',
    monthly_token_budget: '',
    team_id: '',
    allowed_tools: normalizeToolList(preset?.allowed_tools ?? []),
    schedule: '',
    approval_mode: preset?.approval_mode ?? 'preset_default',
    max_concurrent_runs: '1',
    default_invocation_mode: normalizeDefaultInvocationMode(defaultInvocationMode, runtimeKind),
  };
}

function supportedModesForForm(runtimeKind: AgentRuntimeKind): AgentInvocationMode[] {
  if (runtimeKind === 'native_sdk') {
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
  const presetKey = fallbackPresetKey(agent);
  return (
    agent.runtime_kind !== presetRuntimeKind(presetKey, presets) ||
    Boolean(agent.monthly_token_budget)
  );
}

function buildCreatePayload(workspaceId: string, form: AgentFormData, advancedOpen: boolean, presets: AgentPresetDefinition[]): CreateAgentRequest {
  const preset = presetMetaForKey(form.preset_key, presets);
  const defaultRuntimeKind = preset?.runtime_kind ?? PRESET_FALLBACKS[form.preset_key].runtime_kind;
  const provider = normalizeProviderForRuntime(form.runtime_kind, form.provider);
  return {
    workspace_id: workspaceId,
    name: form.name.trim(),
    preset_key: form.preset_key,
    provider,
    model: form.model.trim() || undefined,
    system_prompt: form.system_prompt.trim() || undefined,
    trigger_mode: 'manual',
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
}

function buildUpdatePayload(form: AgentFormData, advancedOpen: boolean, presets: AgentPresetDefinition[]): UpdateAgentRequest {
  const preset = presetMetaForKey(form.preset_key, presets);
  const defaultRuntimeKind = preset?.runtime_kind ?? PRESET_FALLBACKS[form.preset_key].runtime_kind;
  const provider = normalizeProviderForRuntime(form.runtime_kind, form.provider);
  return {
    name: form.name.trim(),
    preset_key: form.preset_key,
    trigger_mode: 'manual',
    provider: provider || undefined,
    model: form.model.trim() || undefined,
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
}

// ---------------------------------------------------------------------------
// Inline helper: label + optional tooltip
// ---------------------------------------------------------------------------

function FieldLabel({ htmlFor, children, tooltip }: { htmlFor?: string; children: React.ReactNode; tooltip?: string }) {
  return (
    <div className="flex items-center gap-1.5">
      <Label htmlFor={htmlFor}>{children}</Label>
      {tooltip && (
        <Tooltip>
          <TooltipTrigger asChild>
            <HelpCircle className="h-3.5 w-3.5 text-muted-foreground/60 cursor-help" />
          </TooltipTrigger>
          <TooltipContent side="right" className="max-w-56 text-xs">
            {tooltip}
          </TooltipContent>
        </Tooltip>
      )}
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
          <span className="truncate font-semibold text-sm">{agent.name}</span>
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
                    <Pencil className="h-3.5 w-3.5" />
                  </button>
                </TooltipTrigger>
                <TooltipContent side="left" className="text-xs">Edit agent</TooltipContent>
              </Tooltip>
            )}
          </div>
        </div>
        <div className="flex items-center gap-2 flex-wrap">
          <Badge variant="secondary" className="text-[11px]">
            {presetLabel}
          </Badge>
          {agent.is_system && (
            <Badge variant="outline" className="text-[11px]">
              System
            </Badge>
          )}
          {teamName && (
            <span className="flex items-center gap-1 text-[11px] text-muted-foreground">
              <Users className="h-3 w-3" />
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
            <Bot className="h-3 w-3" />
            {INVOCATION_MODE_LABELS[agent.default_invocation_mode]}
          </span>
          {agent.schedule && (
            <span className="flex items-center gap-1">
              <Clock className="h-3 w-3" />
              Scheduled
            </span>
          )}
          {stats && (
            <span className="flex items-center gap-1">
              <Zap className="h-3 w-3" />
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
                <Pencil className="h-3.5 w-3.5" />
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

  const [agents, setAgents] = useState<Agent[]>([]);
  const [providerOptions, setProviderOptions] = useState<AgentModelProviderOption[]>(FALLBACK_PROVIDER_OPTIONS);
  const [presets, setPresets] = useState<AgentPresetDefinition[]>([]);
  const [toolCatalog, setToolCatalog] = useState<ToolCatalogResponse | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const { data: settings } = useWorkspaceSettings(workspaceId ?? '');
  const teams = settings?.teams ?? [];
  const teamMap = new Map(teams.map((t) => [t.id, t.name]));

  const [viewMode, setViewMode] = useState<'list' | 'cards'>('list');
  const [runStats, setRunStats] = useState<Record<string, AgentRunStats>>({});

  const [dialogOpen, setDialogOpen] = useState(false);
  const [editingAgent, setEditingAgent] = useState<Agent | null>(null);
  const [form, setForm] = useState<AgentFormData>(createEmptyForm());
  const [advancedOpen, setAdvancedOpen] = useState(false);
  const [automationOpen, setAutomationOpen] = useState(false);
  const [toolPickerOpen, setToolPickerOpen] = useState(false);
  const [saving, setSaving] = useState(false);
  const [deleteConfirmOpen, setDeleteConfirmOpen] = useState(false);

  const loadAgents = useCallback(async () => {
    if (!workspaceId) return;
    setLoading(true);
    setError(null);
    const agentsRes = await agentService.list(workspaceId);
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
    const res = await agentService.listToolCatalog(workspaceId);
    if (!res.error && res.data) {
      setToolCatalog(res.data);
    }
  }, [workspaceId]);

  useEffect(() => {
    loadAgents();
    loadProviderOptions();
    loadPresets();
    loadToolCatalog();
  }, [loadAgents, loadProviderOptions, loadPresets, loadToolCatalog]);

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
    setAdvancedOpen(false);
    setAutomationOpen(false);
    setToolPickerOpen(false);
    setForm(createEmptyForm(DEFAULT_PRESET_KEY, presetMetaForKey(DEFAULT_PRESET_KEY, presets)));
    setDialogOpen(true);
  };

  const openEditDialog = (agent: Agent) => {
    setEditingAgent(agent);
    const presetKey = fallbackPresetKey(agent);
    const preset = presetMetaForKey(presetKey, presets);
    setAdvancedOpen(hasConfiguredAdvancedFields(agent, presets));
    setAutomationOpen(Boolean(agent.schedule || agent.approval_mode !== 'preset_default'));
    setToolPickerOpen(false);
    const runtimeKind = agent.runtime_kind;
    setForm({
      name: agent.name,
      preset_key: presetKey,
      runtime_kind: runtimeKind,
      provider: normalizeProviderForRuntime(runtimeKind, agent.provider ?? 'anthropic'),
      model: agent.model ?? '',
      system_prompt: agent.system_prompt ?? '',
      monthly_token_budget: agent.monthly_token_budget?.toString() ?? '',
      team_id: agent.team_id ?? '',
      allowed_tools: normalizeToolList(agent.allowed_tools.length > 0 ? agent.allowed_tools : (preset?.allowed_tools ?? [])),
      schedule: agent.schedule ?? '',
      approval_mode: agent.approval_mode ?? 'preset_default',
      max_concurrent_runs: agent.max_concurrent_runs?.toString() ?? '1',
      default_invocation_mode: normalizeDefaultInvocationMode(
        agent.default_invocation_mode,
        runtimeKind,
        preset?.default_invocation_mode ?? PRESET_FALLBACKS[presetKey].default_invocation_mode,
      ),
    });
    setDialogOpen(true);
  };

  const handleSave = async () => {
    if (!workspaceId || !form.name.trim()) return;
    setSaving(true);

    if (editingAgent) {
      const payload = buildUpdatePayload(form, advancedOpen, presets);
      const res = await agentService.update(workspaceId, editingAgent.id, payload);
      if (!res.error) {
        setDialogOpen(false);
        await loadAgents();
      }
    } else {
      const payload = buildCreatePayload(workspaceId, form, advancedOpen, presets);
      const res = await agentService.create(workspaceId, payload);
      if (!res.error) {
        setDialogOpen(false);
        await loadAgents();
      }
    }
    setSaving(false);
  };

  const handleDelete = async () => {
    if (!workspaceId || !editingAgent) return;
    setSaving(true);
    const res = await agentService.delete(workspaceId, editingAgent.id);
    if (!res.error) {
      setDialogOpen(false);
      await loadAgents();
    }
    setSaving(false);
  };

  if (!workspace) {
    return <p className="text-sm text-muted-foreground">Workspace not found.</p>;
  }

  const advancedConfigured = hasConfiguredAdvancedFields(editingAgent, presets);
  const editingSystemAgent = Boolean(editingAgent?.is_system);
  const selectedPreset = presetMetaForKey(form.preset_key, presets);
  const effectiveTargets =
    editingAgent && fallbackPresetKey(editingAgent) === form.preset_key && editingAgent.allowed_targets.length > 0
      ? editingAgent.allowed_targets
      : (selectedPreset?.allowed_target_types ?? []);
  const supportedModes = presetSupportedModes(form.preset_key, form.runtime_kind, presets);
  const visibleProviderOptions = availableProvidersForRuntime(form.runtime_kind, providerOptions);
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

  return (
    <div className="max-w-5xl mx-auto space-y-4">
      <div className="flex items-center justify-between">
        <h1 className="text-xl font-semibold">Agents</h1>
        {agents.length > 0 && (
          <div className="flex items-center gap-2">
            <div className="flex items-center rounded-md border border-border">
              <button
                type="button"
                className={`p-1.5 ${viewMode === 'list' ? 'bg-muted text-foreground' : 'text-muted-foreground hover:text-foreground'}`}
                onClick={() => setViewMode('list')}
              >
                <LayoutList className="h-4 w-4" />
              </button>
              <button
                type="button"
                className={`p-1.5 ${viewMode === 'cards' ? 'bg-muted text-foreground' : 'text-muted-foreground hover:text-foreground'}`}
                onClick={() => setViewMode('cards')}
              >
                <LayoutGrid className="h-4 w-4" />
              </button>
            </div>
            {canEdit && (
              <Button size="sm" onClick={openCreateDialog}>
                <Plus className="mr-1.5 h-4 w-4" />
                New Agent
              </Button>
            )}
          </div>
        )}
      </div>

      {loading && <p className="text-sm text-muted-foreground">Loading agents...</p>}
      {error && <p className="text-sm text-destructive">{error}</p>}

      {/* ---- Empty state with onboarding ---- */}
      {!loading && agents.length === 0 && !error && (
        <div className="flex flex-col items-center justify-center py-16 px-4">
          <div className="flex h-14 w-14 items-center justify-center rounded-full bg-violet-500/10 mb-5">
            <Bot className="h-7 w-7 text-violet-500" />
          </div>
          <h3 className="text-lg font-semibold mb-1.5">Create your first agent</h3>
          <p className="text-sm text-muted-foreground text-center max-w-md mb-6">
            AI-powered teammates that plan features, write code, review work, update docs, reply to customers, and manage deals — automatically or on demand.
          </p>
          {canEdit && (
            <Button className="gap-2 mb-8" onClick={openCreateDialog}>
              <Plus className="h-4 w-4" />
              New Agent
            </Button>
          )}
          <div className="grid grid-cols-1 sm:grid-cols-3 gap-4 w-full max-w-4xl">
            {EMPTY_STATE_CARDS.map((card) => (
              <div key={card.title} className="flex flex-col items-center text-center rounded-lg border border-border/50 bg-muted/30 p-6">
                <card.icon className="h-5 w-5 text-muted-foreground mb-3" />
                <p className="text-sm font-medium mb-1">{card.title}</p>
                <p className="text-[13px] text-muted-foreground leading-relaxed">{card.desc}</p>
              </div>
            ))}
          </div>
        </div>
      )}

      {/* ---- Agent list / grid ---- */}
      {agents.length > 0 && viewMode === 'list' && (
        <div className="rounded-lg border border-border overflow-hidden">
          {/* List header */}
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
          {agents.map((agent) => (
            <AgentRow
              key={agent.id}
              agent={agent}
              presetLabel={presetLabel(fallbackPresetKey(agent), presets)}
              teamName={agent.team_id ? teamMap.get(agent.team_id) : undefined}
              stats={runStats[agent.id]}
              onEdit={openEditDialog}
              canEdit={canEdit}
            />
          ))}
        </div>
      )}

      {agents.length > 0 && viewMode === 'cards' && (
        <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
          {agents.map((agent) => (
            <AgentCard
              key={agent.id}
              agent={agent}
              presetLabel={presetLabel(fallbackPresetKey(agent), presets)}
              teamName={agent.team_id ? teamMap.get(agent.team_id) : undefined}
              stats={runStats[agent.id]}
              onEdit={openEditDialog}
              canEdit={canEdit}
            />
          ))}
        </div>
      )}

      {/* ---- Create / Edit dialog ---- */}
      <Dialog
        open={dialogOpen}
        onOpenChange={(open) => {
          setDialogOpen(open);
          if (!open) {
            setToolPickerOpen(false);
          }
        }}
      >
        <DialogContent className="sm:max-w-3xl max-h-[85vh] overflow-y-auto">
          <DialogHeader>
            <DialogTitle>{editingAgent ? 'Edit Agent' : 'New Agent'}</DialogTitle>
          </DialogHeader>

          <div className="space-y-4">
            {editingSystemAgent && (
              <div className="rounded-md border border-border/60 bg-muted/30 px-3 py-2">
                <p className="text-sm font-medium">System agent</p>
                <p className="text-xs text-muted-foreground">
                  This is a built-in workspace agent. You can tune its prompt, model, and default run mode, but its preset stays fixed.
                </p>
              </div>
            )}

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
              <FieldLabel tooltip="Presets provide sensible defaults for prompt, tools, targets, runtime, and default run mode.">
                Preset
              </FieldLabel>
              <Select
                value={form.preset_key}
                disabled={editingSystemAgent}
                onValueChange={(value) => {
                  const nextPresetKey = value as AgentPresetKey;
                  const nextPreset = presetMetaForKey(nextPresetKey, presets);
                  setForm((current) => {
                    const currentDefaultRuntime = presetRuntimeKind(current.preset_key, presets);
                    const currentPresetPrompt = presetMetaForKey(current.preset_key, presets)?.system_prompt ?? '';
                    const currentPresetTools = presetAllowedTools(current.preset_key, presets);
                    const nextDefaultRuntime = nextPreset?.runtime_kind ?? PRESET_FALLBACKS[nextPresetKey].runtime_kind;
                    const nextAllowedRuntimes = allowedRuntimeKindsForPreset(nextPresetKey);
                    const nextRuntimeKind = current.runtime_kind === currentDefaultRuntime || !nextAllowedRuntimes.includes(current.runtime_kind)
                      ? nextDefaultRuntime
                      : current.runtime_kind;
                    const shouldReplacePrompt =
                      current.system_prompt.trim().length === 0 || current.system_prompt === currentPresetPrompt;
                    const shouldReplaceTools = toolListsEqual(current.allowed_tools, currentPresetTools);
                    return {
                      ...current,
                      preset_key: nextPresetKey,
                      runtime_kind: nextRuntimeKind,
                      provider: normalizeProviderForRuntime(nextRuntimeKind, current.provider),
                      system_prompt: shouldReplacePrompt ? (nextPreset?.system_prompt ?? '') : current.system_prompt,
                      allowed_tools: shouldReplaceTools ? presetAllowedTools(nextPresetKey, presets) : current.allowed_tools,
                      approval_mode: nextPreset?.approval_mode ?? current.approval_mode,
                      default_invocation_mode: normalizeDefaultInvocationMode(
                        current.default_invocation_mode,
                        nextRuntimeKind,
                        nextPreset?.default_invocation_mode ?? PRESET_FALLBACKS[nextPresetKey].default_invocation_mode,
                      ),
                    };
                  });
                }}
              >
                <SelectTrigger>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {PRESET_ORDER.map((presetKey) => (
                    <SelectItem key={presetKey} value={presetKey}>
                      <span className="flex flex-col">
                        <span>{presetLabel(presetKey, presets)}</span>
                      </span>
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
              <p className="text-xs text-muted-foreground">{presetDescription(form.preset_key, presets)}</p>
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
            {teams.length > 0 && (
              <div className="space-y-2">
                <FieldLabel tooltip="Assign this agent to a team so it only works on that team's tasks. Leave unassigned for workspace-wide access.">
                  Team
                </FieldLabel>
                <Select
                  disabled={editingSystemAgent}
                  value={form.team_id || '_none'}
                  onValueChange={(value) => setForm((current) => ({ ...current, team_id: value === '_none' ? '' : value }))}
                >
                  <SelectTrigger>
                    <SelectValue placeholder="All teams (workspace-wide)" />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="_none">All teams (workspace-wide)</SelectItem>
                    {teams.map((team) => (
                      <SelectItem key={team.id} value={team.id}>
                        {team.name}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>
            )}

            <div className="grid grid-cols-2 gap-3">
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
                  <SelectTrigger>
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
                <p className="text-[11px] text-muted-foreground">
                  {form.default_invocation_mode === 'interactive'
                    ? 'New runs open as a live chat and can ask follow-up questions before acting.'
                    : 'New runs execute autonomously unless you open the run drawer to inspect them.'}
                </p>
              </div>
              <div className="space-y-2">
                <FieldLabel tooltip="The AI service that powers this agent.">AI Provider</FieldLabel>
                <Select
                  value={form.provider}
                  onValueChange={(value) => setForm((current) => ({ ...current, provider: normalizeProviderForRuntime(current.runtime_kind, value as AgentModelProvider) }))}
                >
                  <SelectTrigger>
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    {visibleProviderOptions.map((provider) => (
                      <SelectItem key={provider.value} value={provider.value}>
                        {provider.label}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
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
                  placeholder={visibleProviderOptions.find((o) => o.value === form.provider)?.model_placeholder ?? 'Auto'}
                />
              </div>
            </div>

            <div className="space-y-3 rounded-md border bg-muted/30 p-3">
              <div className="space-y-1">
                <p className="text-sm font-medium">Effective capabilities</p>
                <p className="text-[11px] text-muted-foreground">
                  These are the effective defaults produced by the preset, runtime, mode, and policy on this agent.
                </p>
              </div>

              <div className="grid gap-3 md:grid-cols-3">
                <div className="space-y-1">
                  <p className="text-[11px] font-medium uppercase tracking-wide text-muted-foreground">Runtime</p>
                  <p className="text-sm">{AGENT_RUNTIME_LABELS[form.runtime_kind] ?? form.runtime_kind}</p>
                </div>
                <div className="space-y-1">
                  <p className="text-[11px] font-medium uppercase tracking-wide text-muted-foreground">Default mode</p>
                  <p className="text-sm">{INVOCATION_MODE_LABELS[form.default_invocation_mode]}</p>
                </div>
                <div className="space-y-1">
                  <p className="text-[11px] font-medium uppercase tracking-wide text-muted-foreground">Targets</p>
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
                <div className="space-y-1 md:col-span-3">
                  <p className="text-[11px] font-medium uppercase tracking-wide text-muted-foreground">Supported modes</p>
                  <div className="flex flex-wrap gap-1.5">
                    {supportedModes.map((mode) => (
                      <Badge key={mode} variant="outline" className="text-[11px]">
                        {INVOCATION_MODE_LABELS[mode]}
                      </Badge>
                    ))}
                  </div>
                </div>
              </div>

              <div className="space-y-1">
                <div className="flex items-center justify-between gap-2">
                  <p className="text-[11px] font-medium uppercase tracking-wide text-muted-foreground">Available tools</p>
                  <Popover open={toolPickerOpen} onOpenChange={setToolPickerOpen}>
                    <PopoverTrigger asChild>
                      <Button
                        type="button"
                        variant="outline"
                        size="sm"
                        className="h-8 gap-1.5 px-2 text-[11px]"
                        disabled={toolCatalogEntries.length === 0 || editingSystemAgent || codexUsesPresetCapabilities}
                      >
                        <Plus className="h-3.5 w-3.5" />
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
                <p className="text-[11px] text-muted-foreground">
                  {codexUsesPresetCapabilities
                    ? 'Codex currently uses the preset capability set as-is. Custom tool overrides are disabled for this runtime.'
                    : 'Choose from the workspace tool catalog. Selected tools become this agent&apos;s allowed tool list.'}
                </p>
                <div className="flex max-h-40 flex-wrap gap-1.5 overflow-y-auto rounded-md border border-border/50 bg-background/70 p-2">
                  {form.allowed_tools.length > 0 ? form.allowed_tools.map((tool) => (
                    <Badge key={tool} variant="secondary" className="gap-1 pr-1 font-mono text-[11px]">
                      <span>{tool}</span>
                      {!editingSystemAgent && (
                        <button
                          type="button"
                          className="rounded-sm p-0.5 text-muted-foreground transition-colors hover:bg-background hover:text-foreground"
                          onClick={() => removeTool(tool)}
                          disabled={codexUsesPresetCapabilities}
                          aria-label={`Remove ${tool}`}
                        >
                          <X className="h-3 w-3" />
                        </button>
                      )}
                    </Badge>
                  )) : (
                    <span className="text-sm text-muted-foreground">No tools configured</span>
                  )}
                </div>
              </div>
            </div>

            {/* ---- Scheduling & Approval ---- */}
            <Collapsible.Root open={automationOpen} onOpenChange={setAutomationOpen}>
              <Collapsible.Trigger asChild>
                <Button type="button" variant="ghost" className="flex w-full items-center justify-between px-2">
                  <span className="flex items-center gap-2 text-sm">
                    {automationOpen ? <ChevronDown className="h-4 w-4" /> : <ChevronRight className="h-4 w-4" />}
                    Scheduling & Approval
                  </span>
                  {form.schedule.trim() && !automationOpen && (
                    <Badge variant="outline" className="text-[11px] gap-1">
                      <Clock className="h-3 w-3" />
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

            {/* ---- Advanced (engine internals) ---- */}
            <Collapsible.Root open={advancedOpen} onOpenChange={setAdvancedOpen}>
              <Collapsible.Trigger asChild>
                <Button type="button" variant="ghost" className="flex w-full items-center justify-between px-2">
                  <span className="flex items-center gap-2 text-sm">
                    {advancedOpen ? <ChevronDown className="h-4 w-4" /> : <ChevronRight className="h-4 w-4" />}
                    Advanced
                  </span>
                  {advancedConfigured && !advancedOpen && (
                    <Badge variant="outline" className="text-[11px]">Customised</Badge>
                  )}
                </Button>
              </Collapsible.Trigger>
              <Collapsible.Content className="space-y-4 rounded-md border bg-muted/30 p-3 mt-2">
                <div className="space-y-2">
                  <FieldLabel tooltip="The execution engine that runs this agent. Only change this if you know what you're doing.">
                    Execution engine
                  </FieldLabel>
                  <Select
                    value={form.runtime_kind}
                    disabled={editingSystemAgent}
                    onValueChange={(value) =>
                      setForm((current) => {
                        const runtimeKind = value as AgentRuntimeKind;
                        const allowedRuntimeKinds = allowedRuntimeKindsForPreset(current.preset_key);
                        if (!allowedRuntimeKinds.includes(runtimeKind)) {
                          return current;
                        }
                        return {
                          ...current,
                          runtime_kind: runtimeKind,
                          provider: normalizeProviderForRuntime(runtimeKind, current.provider),
                          default_invocation_mode: normalizeDefaultInvocationMode(
                            current.default_invocation_mode,
                            runtimeKind,
                            presetDefaultInvocationMode(current.preset_key, presets),
                          ),
                        };
                      })
                    }
                  >
                    <SelectTrigger>
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                      {RUNTIME_KIND_OPTIONS
                        .filter((runtimeKind) => allowedRuntimeKindsForPreset(form.preset_key).includes(runtimeKind))
                        .map((runtimeKind) => (
                        <SelectItem key={runtimeKind} value={runtimeKind}>
                          {AGENT_RUNTIME_LABELS[runtimeKind]}
                        </SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                </div>

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

          {/* ---- Footer ---- */}
          <DialogFooter className="flex-row justify-between sm:justify-between pt-2">
            <div>
              {editingAgent && (
                <Button
                  variant="destructive"
                  size="sm"
                  disabled={saving || editingSystemAgent}
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
                {saving ? 'Saving...' : editingAgent ? 'Save Changes' : 'Create Agent'}
              </Button>
            </div>
          </DialogFooter>
        </DialogContent>
      </Dialog>

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
