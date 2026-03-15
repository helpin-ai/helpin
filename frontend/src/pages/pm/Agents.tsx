import { useCallback, useEffect, useState } from 'react';
import { Collapsible } from 'radix-ui';
import {
  Bot,
  ChevronDown,
  ChevronRight,
  Clock,
  HelpCircle,
  Plus,
  Users,
  Wrench,
  Zap,
} from 'lucide-react';
import { ConfirmDialog } from '@/components/pm/ConfirmDialog';
import { useTitle } from '@/hooks/useTitle';
import { useWorkspaceStore } from '@/stores/workspaceStore';
import { useWorkspaceSettings } from '@/hooks/queries/useSettings';
import { agentService } from '@/lib/services/agentService';
import type {
  Agent,
  AgentClass,
  AgentApprovalMode,
  AgentModelProvider,
  AgentModelProviderOption,
  AgentRuntimeKind,
  AgentTriggerMode,
  CreateAgentRequest,
  UpdateAgentRequest,
} from '@/lib/pmTypes';
import { Button } from '@/components/ui/button';
import { Badge } from '@/components/ui/badge';
import { Card, CardContent, CardHeader } from '@/components/ui/card';
import { Progress } from '@/components/ui/progress';
import { Separator } from '@/components/ui/separator';
import {
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Textarea } from '@/components/ui/textarea';
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

const RUNTIME_KIND_OPTIONS: AgentRuntimeKind[] = ['opencode', 'native_claude', 'claude_code', 'openclaw', 'zeroclaw'];
const ADVANCED_DEFAULT_RUNTIME: Record<AgentClass, AgentRuntimeKind> = {
  product_planner: 'opencode',
  engineer: 'opencode',
  reviewer: 'opencode',
  support: 'opencode',
  human: 'opencode',
};
const ENGINE_TRIGGER_MODE_OPTIONS: AgentTriggerMode[] = ['manual', 'auto_on_assignment', 'auto_on_event'];

const TRIGGER_MODE_LABELS: Record<AgentTriggerMode, string> = {
  manual: 'Manually',
  auto_on_assignment: 'When assigned a story',
  auto_on_event: 'When an event occurs',
};

const AGENT_CLASS_LABELS: Record<AgentClass, string> = {
  product_planner: 'Planner',
  engineer: 'Coder',
  reviewer: 'Reviewer',
  support: 'Support',
  human: 'Team Member',
};

const AGENT_CLASS_DESCRIPTIONS: Record<AgentClass, string> = {
  product_planner: 'Plans features, writes specs, and breaks work into stories.',
  engineer: 'Writes code, implements features, and fixes bugs.',
  reviewer: 'Reviews work, runs tests, and checks quality.',
  support: 'Handles support conversations and drafts replies.',
  human: 'Represents a real person for manual handoffs.',
};

const TOOL_GROUPS = [
  {
    label: 'Code & Files',
    tools: [
      { id: 'read_file', label: 'Read files' },
      { id: 'write_file', label: 'Write files' },
      { id: 'list_directory', label: 'Browse folders' },
      { id: 'search_files', label: 'Find files' },
      { id: 'read_file_range', label: 'Read file sections' },
      { id: 'ripgrep', label: 'Search code' },
      { id: 'grep', label: 'Search text' },
      { id: 'list_symbols', label: 'Browse code structure' },
    ],
  },
  {
    label: 'Commands',
    tools: [{ id: 'run_command', label: 'Run commands' }],
  },
  {
    label: 'Git & Deployment',
    tools: [
      { id: 'create_branch', label: 'Create branches' },
      { id: 'commit_and_push', label: 'Save & publish code' },
      { id: 'open_pr', label: 'Open pull requests' },
    ],
  },
  {
    label: 'Project Management',
    tools: [
      { id: 'add_story_comment', label: 'Comment on stories' },
      { id: 'update_story_state', label: 'Update story status' },
      { id: 'list_story_checklist', label: 'View checklists' },
    ],
  },
  {
    label: 'Customer Support',
    tools: [
      { id: 'list_conversation_messages', label: 'Read conversations' },
      { id: 'draft_support_reply', label: 'Draft replies' },
      { id: 'update_conversation_status', label: 'Update status' },
    ],
  },
  {
    label: 'Sales & CRM',
    tools: [
      { id: 'list_deals', label: 'View deals' },
      { id: 'update_deal_stage', label: 'Move deals forward' },
      { id: 'add_deal_note', label: 'Add deal notes' },
      { id: 'list_contacts', label: 'View contacts' },
      { id: 'list_buyer_signals', label: 'View buying signals' },
    ],
  },
  {
    label: 'Knowledge Base',
    tools: [
      { id: 'list_documents', label: 'Browse documents' },
      { id: 'read_document', label: 'Read documents' },
      { id: 'search_documents', label: 'Search documents' },
    ],
  },
  {
    label: 'Research',
    tools: [{ id: 'web_search', label: 'Search the web' }],
  },
];

const APPROVAL_MODE_OPTIONS: { value: AgentApprovalMode; label: string; description: string }[] = [
  { value: 'class_default', label: 'Default', description: 'Uses the standard setting for this agent role' },
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
    icon: Wrench,
    title: 'Cross-module',
    desc: 'Agents can span projects, CRM, support, and docs — just pick the capabilities they need.',
  },
  {
    icon: Clock,
    title: 'Schedule or trigger',
    desc: 'Run on a schedule, on story assignment, or on demand — fully automatic or manual.',
  },
];

// ---------------------------------------------------------------------------
// Form helpers
// ---------------------------------------------------------------------------

interface AgentFormData {
  name: string;
  agent_class: AgentClass;
  runtime_kind: AgentRuntimeKind;
  trigger_mode: AgentTriggerMode;
  backing_user_id: string;
  skills: string;
  provider: AgentModelProvider;
  model: string;
  system_prompt: string;
  planning_notes: string;
  monthly_token_budget: string;
  team_id: string;
  schedule: string;
  approval_mode: AgentApprovalMode;
  max_concurrent_runs: string;
  allowed_tools: string[];
}

const FALLBACK_PROVIDER_OPTIONS: AgentModelProviderOption[] = [
  { value: 'anthropic', label: 'Anthropic', model_placeholder: 'claude-sonnet-4-20250514' },
  { value: 'openai', label: 'OpenAI', model_placeholder: 'gpt-5-mini' },
  { value: 'openrouter', label: 'OpenRouter', model_placeholder: 'openai/gpt-5-mini' },
];

function createEmptyForm(agentClass: AgentClass = 'engineer'): AgentFormData {
  return {
    name: '',
    agent_class: agentClass,
    runtime_kind: ADVANCED_DEFAULT_RUNTIME[agentClass],
    trigger_mode: defaultTriggerModeForClass(agentClass),
    backing_user_id: '',
    skills: '',
    provider: 'anthropic',
    model: '',
    system_prompt: '',
    planning_notes: '',
    monthly_token_budget: '',
    team_id: '',
    schedule: '',
    approval_mode: 'class_default',
    max_concurrent_runs: '1',
    allowed_tools: [],
  };
}

function isLLMAgentClass(agentClass: AgentClass): boolean {
  return agentClass !== 'human';
}

function showsTriggerMode(agentClass: AgentClass): boolean {
  return agentClass === 'engineer' || agentClass === 'reviewer';
}

function defaultTriggerModeForClass(_agentClass: AgentClass): AgentTriggerMode {
  return 'manual';
}

function allowedTriggerModesForClass(agentClass: AgentClass): AgentTriggerMode[] {
  return showsTriggerMode(agentClass) ? ENGINE_TRIGGER_MODE_OPTIONS : ['manual'];
}

function parseSkills(skills: string): string[] {
  return skills
    .split(',')
    .map((entry) => entry.trim())
    .filter(Boolean);
}

function hasConfiguredAdvancedFields(agent: Agent | null): boolean {
  if (!agent || agent.agent_class === 'human') return false;
  return (
    agent.runtime_kind !== ADVANCED_DEFAULT_RUNTIME[agent.agent_class] ||
    Boolean(agent.system_prompt?.trim()) ||
    agent.skills.length > 0 ||
    Boolean(agent.monthly_token_budget)
  );
}

function nextFormForClass(current: AgentFormData, nextClass: AgentClass): AgentFormData {
  const next: AgentFormData = {
    ...current,
    agent_class: nextClass,
    runtime_kind: current.runtime_kind || ADVANCED_DEFAULT_RUNTIME[nextClass],
    trigger_mode: allowedTriggerModesForClass(nextClass).includes(current.trigger_mode)
      ? current.trigger_mode
      : defaultTriggerModeForClass(nextClass),
  };

  if (nextClass === 'human') {
    return {
      ...next,
      runtime_kind: ADVANCED_DEFAULT_RUNTIME[nextClass],
      trigger_mode: 'manual',
      provider: 'anthropic',
      model: '',
      system_prompt: '',
      planning_notes: '',
      skills: '',
      monthly_token_budget: '',
    };
  }

  if (nextClass === 'product_planner') {
    return {
      ...next,
      backing_user_id: '',
      provider: 'anthropic',
      system_prompt: '',
      trigger_mode: 'manual',
    };
  }

  return {
    ...next,
    backing_user_id: '',
    planning_notes: '',
  };
}

function buildAdvancedFields(form: AgentFormData, advancedOpen: boolean): Partial<CreateAgentRequest> {
  if (!advancedOpen || form.agent_class === 'human') {
    return {};
  }

  return {
    runtime_kind: form.runtime_kind,
    system_prompt: form.system_prompt,
    skills: parseSkills(form.skills),
    monthly_token_budget: form.monthly_token_budget.trim()
      ? Number.parseInt(form.monthly_token_budget, 10)
      : 0,
  };
}

function buildAutomationFields(form: AgentFormData): Partial<CreateAgentRequest> {
  if (form.agent_class === 'human') return {};
  return {
    team_id: form.team_id || undefined,
    schedule: form.schedule.trim() || undefined,
    approval_mode: form.approval_mode,
    max_concurrent_runs: form.max_concurrent_runs ? Number.parseInt(form.max_concurrent_runs, 10) : 1,
    allowed_tools: form.allowed_tools.length > 0 ? form.allowed_tools : undefined,
  };
}

function buildCreatePayload(workspaceId: string, form: AgentFormData, advancedOpen: boolean): CreateAgentRequest {
  return {
    workspace_id: workspaceId,
    name: form.name.trim(),
    agent_class: form.agent_class,
    backing_user_id: form.agent_class === 'human' ? form.backing_user_id.trim() : undefined,
    trigger_mode: showsTriggerMode(form.agent_class) ? form.trigger_mode : undefined,
    provider: isLLMAgentClass(form.agent_class) ? form.provider : undefined,
    model: isLLMAgentClass(form.agent_class) ? form.model.trim() : undefined,
    planning_notes: form.agent_class === 'product_planner' ? form.planning_notes : undefined,
    ...buildAdvancedFields(form, advancedOpen),
    ...buildAutomationFields(form),
  };
}

function buildUpdatePayload(form: AgentFormData, advancedOpen: boolean): UpdateAgentRequest {
  return {
    name: form.name.trim(),
    agent_class: form.agent_class,
    backing_user_id: form.agent_class === 'human' ? form.backing_user_id.trim() : undefined,
    trigger_mode: showsTriggerMode(form.agent_class) ? form.trigger_mode : undefined,
    provider: isLLMAgentClass(form.agent_class) ? form.provider : undefined,
    model: isLLMAgentClass(form.agent_class) ? form.model.trim() : undefined,
    planning_notes: form.agent_class === 'product_planner' ? form.planning_notes : undefined,
    ...buildAdvancedFields(form, advancedOpen),
    ...buildAutomationFields(form),
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
// AgentCard
// ---------------------------------------------------------------------------

function AgentCard({
  agent,
  teamName,
  onEdit,
}: {
  agent: Agent;
  teamName?: string;
  onEdit: (agent: Agent) => void;
}) {
  const budgetPct =
    agent.agent_kind === 'llm' && agent.monthly_token_budget
      ? Math.min(
          100,
          Math.round((agent.tokens_used_this_month / agent.monthly_token_budget) * 100)
        )
      : null;

  return (
    <Card
      className="cursor-pointer transition-shadow hover:shadow-md"
      onClick={() => onEdit(agent)}
    >
      <CardHeader className="pb-2">
        <div className="flex items-start justify-between gap-2">
          <span className="truncate font-semibold text-sm">{agent.name}</span>
          <div className="flex items-center gap-1.5 shrink-0">
            <span className="text-[11px] text-muted-foreground">{STATUS_LABEL[agent.status] ?? agent.status}</span>
            <span
              className={`h-2 w-2 rounded-full ${STATUS_DOT[agent.status] ?? STATUS_DOT.paused}`}
            />
          </div>
        </div>
        <div className="flex items-center gap-2 flex-wrap">
          <Badge variant="secondary" className="text-[11px]">
            {AGENT_CLASS_LABELS[agent.agent_class] ?? agent.agent_class}
          </Badge>
          {teamName && (
            <span className="flex items-center gap-1 text-[11px] text-muted-foreground">
              <Users className="h-3 w-3" />
              {teamName}
            </span>
          )}
        </div>
      </CardHeader>
      <CardContent className="space-y-2 pt-0">
        {agent.agent_kind === 'llm' && (agent.provider || agent.model) && (
          <p className="text-xs text-muted-foreground">
            {[agent.provider, agent.model].filter(Boolean).join(' / ')}
          </p>
        )}
        {agent.schedule && (
          <p className="flex items-center gap-1 text-xs text-muted-foreground">
            <Clock className="h-3 w-3" />
            Scheduled
          </p>
        )}
        {(agent.allowed_tools?.length ?? 0) > 0 && (
          <p className="flex items-center gap-1 text-xs text-muted-foreground">
            <Wrench className="h-3 w-3" />
            {agent.allowed_tools!.length} capabilities
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
// AgentsPage
// ---------------------------------------------------------------------------

export function AgentsPage() {
  useTitle('Agents');
  const workspace = useWorkspaceStore((state) => state.currentWorkspace);
  const workspaceId = workspace?.id;

  const [agents, setAgents] = useState<Agent[]>([]);
  const [providerOptions, setProviderOptions] = useState<AgentModelProviderOption[]>(FALLBACK_PROVIDER_OPTIONS);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const { data: settings } = useWorkspaceSettings(workspaceId ?? '');
  const teams = settings?.teams ?? [];
  const teamMap = new Map(teams.map((t) => [t.id, t.name]));

  const [dialogOpen, setDialogOpen] = useState(false);
  const [editingAgent, setEditingAgent] = useState<Agent | null>(null);
  const [form, setForm] = useState<AgentFormData>(createEmptyForm());
  const [advancedOpen, setAdvancedOpen] = useState(false);
  const [automationOpen, setAutomationOpen] = useState(false);
  const [toolsOpen, setToolsOpen] = useState(false);
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

  useEffect(() => {
    loadAgents();
    loadProviderOptions();
  }, [loadAgents, loadProviderOptions]);

  const openCreateDialog = () => {
    setEditingAgent(null);
    setAdvancedOpen(false);
    setAutomationOpen(false);
    setToolsOpen(false);
    setForm(createEmptyForm());
    setDialogOpen(true);
  };

  const openEditDialog = (agent: Agent) => {
    setEditingAgent(agent);
    setAdvancedOpen(hasConfiguredAdvancedFields(agent));
    setAutomationOpen(Boolean(agent.schedule || agent.approval_mode !== 'class_default'));
    setToolsOpen((agent.allowed_tools?.length ?? 0) > 0);
    setForm({
      name: agent.name,
      agent_class: agent.agent_class,
      runtime_kind: agent.runtime_kind,
      trigger_mode: agent.trigger_mode,
      backing_user_id: agent.backing_user_id ?? '',
      skills: agent.skills.join(', '),
      provider: agent.provider ?? 'anthropic',
      model: agent.model ?? '',
      system_prompt: agent.system_prompt ?? '',
      planning_notes: agent.planning_notes ?? '',
      monthly_token_budget: agent.monthly_token_budget?.toString() ?? '',
      team_id: agent.team_id ?? '',
      schedule: agent.schedule ?? '',
      approval_mode: agent.approval_mode ?? 'class_default',
      max_concurrent_runs: agent.max_concurrent_runs?.toString() ?? '1',
      allowed_tools: agent.allowed_tools ?? [],
    });
    setDialogOpen(true);
  };

  const handleSave = async () => {
    if (!workspaceId || !form.name.trim()) return;
    setSaving(true);

    if (editingAgent) {
      const payload = buildUpdatePayload(form, advancedOpen);
      const res = await agentService.update(workspaceId, editingAgent.id, payload);
      if (!res.error) {
        setDialogOpen(false);
        loadAgents();
      }
    } else {
      const payload = buildCreatePayload(workspaceId, form, advancedOpen);
      const res = await agentService.create(workspaceId, payload);
      if (!res.error) {
        setDialogOpen(false);
        loadAgents();
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
      loadAgents();
    }
    setSaving(false);
  };

  if (!workspace) {
    return <p className="text-sm text-muted-foreground">Workspace not found.</p>;
  }

  const advancedConfigured = hasConfiguredAdvancedFields(editingAgent);

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <h1 className="text-xl font-semibold">Agents</h1>
        {agents.length > 0 && (
          <Button size="sm" onClick={openCreateDialog}>
            <Plus className="mr-1.5 h-4 w-4" />
            New Agent
          </Button>
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
          <Button className="gap-2 mb-8" onClick={openCreateDialog}>
            <Plus className="h-4 w-4" />
            New Agent
          </Button>
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

      {/* ---- Agent grid ---- */}
      {agents.length > 0 && (
        <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
          {agents.map((agent) => (
            <AgentCard
              key={agent.id}
              agent={agent}
              teamName={agent.team_id ? teamMap.get(agent.team_id) : undefined}
              onEdit={openEditDialog}
            />
          ))}
        </div>
      )}

      {/* ---- Create / Edit dialog ---- */}
      <Dialog open={dialogOpen} onOpenChange={setDialogOpen}>
        <DialogContent className="sm:max-w-lg max-h-[85vh] overflow-y-auto">
          <DialogHeader>
            <DialogTitle>{editingAgent ? 'Edit Agent' : 'New Agent'}</DialogTitle>
          </DialogHeader>

          <div className="space-y-4">
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
              <FieldLabel tooltip="Each role comes with sensible defaults for permissions and behavior. You can customise everything below.">
                What does this agent do?
              </FieldLabel>
              <Select
                value={form.agent_class}
                onValueChange={(value) => {
                  const nextClass = value as AgentClass;
                  setForm((current) => nextFormForClass(current, nextClass));
                  if (nextClass === 'human') {
                    setAdvancedOpen(false);
                    setAutomationOpen(false);
                    setToolsOpen(false);
                  }
                }}
              >
                <SelectTrigger>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {(Object.keys(AGENT_CLASS_LABELS) as AgentClass[]).map((agentClass) => (
                    <SelectItem key={agentClass} value={agentClass}>
                      <span className="flex flex-col">
                        <span>{AGENT_CLASS_LABELS[agentClass]}</span>
                      </span>
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
              <p className="text-xs text-muted-foreground">{AGENT_CLASS_DESCRIPTIONS[form.agent_class]}</p>
            </div>

            {/* Team selector */}
            {teams.length > 0 && (
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
                    {teams.map((team) => (
                      <SelectItem key={team.id} value={team.id}>
                        {team.name}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>
            )}

            {/* Human agent: linked member */}
            {form.agent_class === 'human' ? (
              <div className="space-y-2">
                <FieldLabel
                  htmlFor="agent-backing-user"
                  tooltip="The workspace member this agent represents. Handoffs will be routed to this person."
                >
                  Linked team member
                </FieldLabel>
                <Input
                  id="agent-backing-user"
                  value={form.backing_user_id}
                  onChange={(e) => setForm((current) => ({ ...current, backing_user_id: e.target.value }))}
                  placeholder="User ID of the team member"
                />
              </div>
            ) : (
              <>
                {/* AI provider + model */}
                <Separator />
                <div className="grid grid-cols-2 gap-3">
                  <div className="space-y-2">
                    <FieldLabel tooltip="The AI service that powers this agent.">AI Provider</FieldLabel>
                    <Select
                      value={form.provider}
                      onValueChange={(value) => setForm((current) => ({ ...current, provider: value as AgentModelProvider }))}
                    >
                      <SelectTrigger>
                        <SelectValue />
                      </SelectTrigger>
                      <SelectContent>
                        {providerOptions.map((provider) => (
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
                      placeholder={providerOptions.find((o) => o.value === form.provider)?.model_placeholder ?? 'Auto'}
                    />
                  </div>
                </div>
              </>
            )}

            {/* Trigger mode */}
            {showsTriggerMode(form.agent_class) && (
              <div className="space-y-2">
                <FieldLabel tooltip="Controls when this agent starts working. 'Manually' means you must trigger it yourself.">
                  When should it run?
                </FieldLabel>
                <Select
                  value={form.trigger_mode}
                  onValueChange={(value) => setForm((current) => ({ ...current, trigger_mode: value as AgentTriggerMode }))}
                >
                  <SelectTrigger>
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    {allowedTriggerModesForClass(form.agent_class).map((triggerMode) => (
                      <SelectItem key={triggerMode} value={triggerMode}>
                        {TRIGGER_MODE_LABELS[triggerMode] ?? triggerMode}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>
            )}

            {/* Planning guidelines */}
            {form.agent_class === 'product_planner' && (
              <div className="space-y-2">
                <FieldLabel
                  htmlFor="agent-planning-notes"
                  tooltip="Extra context for the planner — like preferred frameworks, constraints, or team conventions."
                >
                  Planning guidelines
                </FieldLabel>
                <Textarea
                  id="agent-planning-notes"
                  value={form.planning_notes}
                  onChange={(e) => setForm((current) => ({ ...current, planning_notes: e.target.value }))}
                  placeholder="e.g. Always consider mobile-first. Use our design system components."
                  rows={3}
                />
              </div>
            )}

            {/* ---- Scheduling & Approval ---- */}
            {form.agent_class !== 'human' && (
              <>
                <Separator />
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
              </>
            )}

            {/* ---- Capabilities / Tools ---- */}
            {form.agent_class !== 'human' && (
              <Collapsible.Root open={toolsOpen} onOpenChange={setToolsOpen}>
                <Collapsible.Trigger asChild>
                  <Button type="button" variant="ghost" className="flex w-full items-center justify-between px-2">
                    <span className="flex items-center gap-2 text-sm">
                      {toolsOpen ? <ChevronDown className="h-4 w-4" /> : <ChevronRight className="h-4 w-4" />}
                      Capabilities
                    </span>
                    {form.allowed_tools.length > 0 && !toolsOpen && (
                      <Badge variant="outline" className="text-[11px]">
                        {form.allowed_tools.length} selected
                      </Badge>
                    )}
                  </Button>
                </Collapsible.Trigger>
                <Collapsible.Content className="space-y-3 rounded-md border bg-muted/30 p-3 mt-2">
                  <p className="text-[11px] text-muted-foreground">
                    Choose what this agent is allowed to do. Leave all unselected to use the standard set for its role.
                  </p>
                  {TOOL_GROUPS.map((group) => {
                    const allSelected = group.tools.every((t) => form.allowed_tools.includes(t.id));
                    return (
                      <div key={group.label} className="space-y-1.5">
                        <div className="flex items-center justify-between">
                          <span className="text-xs font-medium text-muted-foreground">{group.label}</span>
                          <button
                            type="button"
                            className="text-[11px] text-primary hover:underline"
                            onClick={() => {
                              const groupIds = group.tools.map((t) => t.id);
                              setForm((current) => ({
                                ...current,
                                allowed_tools: allSelected
                                  ? current.allowed_tools.filter((id) => !groupIds.includes(id))
                                  : [...new Set([...current.allowed_tools, ...groupIds])],
                              }));
                            }}
                          >
                            {allSelected ? 'Remove all' : 'Add all'}
                          </button>
                        </div>
                        <div className="flex flex-wrap gap-1.5">
                          {group.tools.map((tool) => {
                            const selected = form.allowed_tools.includes(tool.id);
                            return (
                              <button
                                key={tool.id}
                                type="button"
                                className={`rounded-md border px-2 py-1 text-xs transition-colors ${
                                  selected
                                    ? 'border-primary bg-primary/10 text-primary'
                                    : 'border-border bg-background text-muted-foreground hover:border-primary/50'
                                }`}
                                onClick={() =>
                                  setForm((current) => ({
                                    ...current,
                                    allowed_tools: selected
                                      ? current.allowed_tools.filter((id) => id !== tool.id)
                                      : [...current.allowed_tools, tool.id],
                                  }))
                                }
                              >
                                {tool.label}
                              </button>
                            );
                          })}
                        </div>
                      </div>
                    );
                  })}
                </Collapsible.Content>
              </Collapsible.Root>
            )}

            {/* ---- Advanced (engine internals) ---- */}
            {form.agent_class !== 'human' && (
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
                      onValueChange={(value) => setForm((current) => ({ ...current, runtime_kind: value as AgentRuntimeKind }))}
                    >
                      <SelectTrigger>
                        <SelectValue />
                      </SelectTrigger>
                      <SelectContent>
                        {RUNTIME_KIND_OPTIONS.map((runtimeKind) => (
                          <SelectItem key={runtimeKind} value={runtimeKind}>
                            {runtimeKind}
                          </SelectItem>
                        ))}
                      </SelectContent>
                    </Select>
                  </div>

                  <div className="space-y-2">
                    <FieldLabel
                      htmlFor="agent-skills"
                      tooltip="Skill packs extend the agent's abilities. Separate multiple values with commas."
                    >
                      Skill packs
                    </FieldLabel>
                    <Input
                      id="agent-skills"
                      value={form.skills}
                      onChange={(e) => setForm((current) => ({ ...current, skills: e.target.value }))}
                      placeholder="e.g. testing, documentation"
                    />
                  </div>

                  <div className="space-y-2">
                    <FieldLabel
                      htmlFor="agent-prompt"
                      tooltip="Custom instructions that shape how this agent behaves. These are added to the agent's base instructions."
                    >
                      Custom instructions
                    </FieldLabel>
                    <Textarea
                      id="agent-prompt"
                      value={form.system_prompt}
                      onChange={(e) => setForm((current) => ({ ...current, system_prompt: e.target.value }))}
                      placeholder="e.g. Always write unit tests. Follow our coding style guide."
                      rows={3}
                    />
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
            )}
          </div>

          {/* ---- Footer ---- */}
          <DialogFooter className="flex-row justify-between sm:justify-between pt-2">
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
