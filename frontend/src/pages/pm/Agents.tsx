import { useCallback, useEffect, useState } from 'react';
import { Collapsible } from 'radix-ui';
import { Bot, ChevronDown, ChevronRight, Plus } from 'lucide-react';
import { ConfirmDialog } from '@/components/pm/ConfirmDialog';
import { useTitle } from '@/hooks/useTitle';
import { useWorkspaceStore } from '@/stores/workspaceStore';
import { agentService } from '@/lib/services/agentService';
import type {
  Agent,
  AgentClass,
  AgentRuntimeKind,
  AgentTriggerMode,
  CreateAgentRequest,
  UpdateAgentRequest,
} from '@/lib/pmTypes';
import { Button } from '@/components/ui/button';
import { Badge } from '@/components/ui/badge';
import { Card, CardContent, CardHeader } from '@/components/ui/card';
import { Progress } from '@/components/ui/progress';
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Textarea } from '@/components/ui/textarea';
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';

const STATUS_DOT: Record<string, string> = {
  idle: 'bg-green-500',
  working: 'bg-amber-500',
  error: 'bg-red-500',
  paused: 'bg-gray-400',
};

const RUNTIME_KIND_OPTIONS: AgentRuntimeKind[] = ['native_claude', 'claude_code', 'openclaw', 'zeroclaw'];
const ADVANCED_DEFAULT_RUNTIME: Record<AgentClass, AgentRuntimeKind> = {
  product_planner: 'native_claude',
  engineer: 'native_claude',
  reviewer: 'native_claude',
  support: 'native_claude',
  human: 'native_claude',
};
const ENGINE_TRIGGER_MODE_OPTIONS: AgentTriggerMode[] = ['manual', 'auto_on_assignment', 'auto_on_event'];
const AGENT_CLASS_LABELS: Record<AgentClass, string> = {
  product_planner: 'Product Planner',
  engineer: 'Engineer',
  reviewer: 'Reviewer',
  support: 'Support',
  human: 'Human',
};
const AGENT_CLASS_DESCRIPTIONS: Record<AgentClass, string> = {
  product_planner: 'Epic-only PRD, spec, and story planning.',
  engineer: 'Story-only implementation and delivery.',
  reviewer: 'Story-only review, testing, and readiness checks.',
  support: 'Support-ticket triage and draft replies.',
  human: 'Non-runnable placeholder for explicit handoffs.',
};

interface AgentFormData {
  name: string;
  agent_class: AgentClass;
  runtime_kind: AgentRuntimeKind;
  trigger_mode: AgentTriggerMode;
  backing_user_id: string;
  skills: string;
  model: string;
  system_prompt: string;
  planning_notes: string;
  monthly_token_budget: string;
}

function createEmptyForm(agentClass: AgentClass = 'engineer'): AgentFormData {
  return {
    name: '',
    agent_class: agentClass,
    runtime_kind: ADVANCED_DEFAULT_RUNTIME[agentClass],
    trigger_mode: defaultTriggerModeForClass(agentClass),
    backing_user_id: '',
    skills: '',
    model: '',
    system_prompt: '',
    planning_notes: '',
    monthly_token_budget: '',
  };
}

function isLLMAgentClass(agentClass: AgentClass): boolean {
  return agentClass !== 'human';
}

function showsTriggerMode(agentClass: AgentClass): boolean {
  return agentClass === 'engineer' || agentClass === 'reviewer';
}

function defaultTriggerModeForClass(agentClass: AgentClass): AgentTriggerMode {
  return agentClass === 'engineer' || agentClass === 'reviewer' ? 'manual' : 'manual';
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

function buildCreatePayload(workspaceId: string, form: AgentFormData, advancedOpen: boolean): CreateAgentRequest {
  return {
    workspace_id: workspaceId,
    name: form.name.trim(),
    agent_class: form.agent_class,
    backing_user_id: form.agent_class === 'human' ? form.backing_user_id.trim() : undefined,
    trigger_mode: showsTriggerMode(form.agent_class) ? form.trigger_mode : undefined,
    model: isLLMAgentClass(form.agent_class) ? form.model.trim() : undefined,
    planning_notes: form.agent_class === 'product_planner' ? form.planning_notes : undefined,
    ...buildAdvancedFields(form, advancedOpen),
  };
}

function buildUpdatePayload(form: AgentFormData, advancedOpen: boolean): UpdateAgentRequest {
  return {
    name: form.name.trim(),
    agent_class: form.agent_class,
    backing_user_id: form.agent_class === 'human' ? form.backing_user_id.trim() : undefined,
    trigger_mode: showsTriggerMode(form.agent_class) ? form.trigger_mode : undefined,
    model: isLLMAgentClass(form.agent_class) ? form.model.trim() : undefined,
    planning_notes: form.agent_class === 'product_planner' ? form.planning_notes : undefined,
    ...buildAdvancedFields(form, advancedOpen),
  };
}

function AgentCard({
  agent,
  onEdit,
}: {
  agent: Agent;
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
          <span
            className={`mt-1 h-2.5 w-2.5 shrink-0 rounded-full ${STATUS_DOT[agent.status] ?? STATUS_DOT.paused}`}
            title={agent.status}
          />
        </div>
        <div className="flex items-center gap-2">
          <Badge
            variant="secondary"
            className={
              agent.agent_kind === 'llm'
                ? 'bg-purple-100 text-purple-700'
                : 'bg-blue-100 text-blue-700'
            }
          >
            {agent.agent_kind === 'llm' ? 'LLM' : 'Human'}
          </Badge>
          <span className="text-xs text-muted-foreground">
            {AGENT_CLASS_LABELS[agent.agent_class] ?? agent.agent_class}
          </span>
        </div>
      </CardHeader>
      <CardContent className="space-y-2 pt-0">
        {agent.agent_kind === 'llm' && agent.model && (
          <p className="text-xs text-muted-foreground">Model: {agent.model}</p>
        )}
        {showsTriggerMode(agent.agent_class) && (
          <p className="text-xs text-muted-foreground">Trigger: {agent.trigger_mode}</p>
        )}
        {budgetPct !== null && (
          <div className="space-y-1">
            <div className="flex justify-between text-[11px] text-muted-foreground">
              <span>Token usage</span>
              <span>{budgetPct}%</span>
            </div>
            <Progress value={budgetPct} className="h-1.5" />
          </div>
        )}
        {agent.active_story_id && (
          <p className="truncate text-xs text-muted-foreground">
            Active story: {agent.active_story_id.slice(0, 8)}...
          </p>
        )}
      </CardContent>
    </Card>
  );
}

export function AgentsPage() {
  useTitle('Agents');
  const workspace = useWorkspaceStore((state) => state.currentWorkspace);
  const workspaceId = workspace?.id;

  const [agents, setAgents] = useState<Agent[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const [dialogOpen, setDialogOpen] = useState(false);
  const [editingAgent, setEditingAgent] = useState<Agent | null>(null);
  const [form, setForm] = useState<AgentFormData>(createEmptyForm());
  const [advancedOpen, setAdvancedOpen] = useState(false);
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

  useEffect(() => {
    loadAgents();
  }, [loadAgents]);

  const openCreateDialog = () => {
    setEditingAgent(null);
    setAdvancedOpen(false);
    setForm(createEmptyForm());
    setDialogOpen(true);
  };

  const openEditDialog = (agent: Agent) => {
    setEditingAgent(agent);
    setAdvancedOpen(false);
    setForm({
      name: agent.name,
      agent_class: agent.agent_class,
      runtime_kind: agent.runtime_kind,
      trigger_mode: agent.trigger_mode,
      backing_user_id: agent.backing_user_id ?? '',
      skills: agent.skills.join(', '),
      model: agent.model ?? '',
      system_prompt: agent.system_prompt ?? '',
      planning_notes: agent.planning_notes ?? '',
      monthly_token_budget: agent.monthly_token_budget?.toString() ?? '',
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
        <Button size="sm" onClick={openCreateDialog}>
          <Plus className="mr-1.5 h-4 w-4" />
          Create Agent
        </Button>
      </div>

      {loading && <p className="text-sm text-muted-foreground">Loading agents...</p>}
      {error && <p className="text-sm text-destructive">{error}</p>}

      {!loading && agents.length === 0 && !error && (
        <div className="flex flex-col items-center justify-center gap-3 rounded-lg border border-dashed py-16">
          <Bot className="h-10 w-10 text-muted-foreground/50" />
          <p className="text-sm text-muted-foreground">No agents yet.</p>
          <Button size="sm" variant="outline" onClick={openCreateDialog}>
            <Plus className="mr-1.5 h-4 w-4" />
            Create Agent
          </Button>
        </div>
      )}

      {agents.length > 0 && (
        <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
          {agents.map((agent) => (
            <AgentCard key={agent.id} agent={agent} onEdit={openEditDialog} />
          ))}
        </div>
      )}

      <Dialog open={dialogOpen} onOpenChange={setDialogOpen}>
        <DialogContent className="sm:max-w-md">
          <DialogHeader>
            <DialogTitle>{editingAgent ? 'Edit Agent' : 'Create Agent'}</DialogTitle>
          </DialogHeader>
          <div className="space-y-4">
            <p className="text-sm text-muted-foreground">
              Choose the agent class based on where it will run. Teampulse handles permissions and runtime defaults automatically.
            </p>

            <div className="space-y-1.5">
              <Label htmlFor="agent-name">Name</Label>
              <Input
                id="agent-name"
                value={form.name}
                onChange={(e) => setForm((current) => ({ ...current, name: e.target.value }))}
                placeholder="Agent name"
              />
            </div>

            <div className="space-y-1.5">
              <Label>Agent Class</Label>
              <Select
                value={form.agent_class}
                onValueChange={(value) => {
                  const nextClass = value as AgentClass;
                  setForm((current) => nextFormForClass(current, nextClass));
                  if (nextClass === 'human') {
                    setAdvancedOpen(false);
                  }
                }}
              >
                <SelectTrigger>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {(Object.keys(AGENT_CLASS_LABELS) as AgentClass[]).map((agentClass) => (
                    <SelectItem key={agentClass} value={agentClass}>
                      {AGENT_CLASS_LABELS[agentClass]}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
              <p className="text-xs text-muted-foreground">{AGENT_CLASS_DESCRIPTIONS[form.agent_class]}</p>
            </div>

            {form.agent_class === 'human' ? (
              <div className="space-y-1.5">
                <Label htmlFor="agent-backing-user">Backing User ID</Label>
                <Input
                  id="agent-backing-user"
                  value={form.backing_user_id}
                  onChange={(e) => setForm((current) => ({ ...current, backing_user_id: e.target.value }))}
                  placeholder="Workspace user ID"
                />
              </div>
            ) : (
              <div className="space-y-1.5">
                <Label htmlFor="agent-model">Model</Label>
                <Input
                  id="agent-model"
                  value={form.model}
                  onChange={(e) => setForm((current) => ({ ...current, model: e.target.value }))}
                  placeholder="e.g. claude-sonnet-4-20250514"
                />
                <p className="text-xs text-muted-foreground">
                  Leave blank to use the runtime default model.
                </p>
              </div>
            )}

            {showsTriggerMode(form.agent_class) && (
              <div className="space-y-1.5">
                <Label>Trigger Mode</Label>
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
                        {triggerMode}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>
            )}

            {form.agent_class === 'product_planner' && (
              <div className="space-y-1.5">
                <Label htmlFor="agent-planning-notes">Planning Notes</Label>
                <Textarea
                  id="agent-planning-notes"
                  value={form.planning_notes}
                  onChange={(e) => setForm((current) => ({ ...current, planning_notes: e.target.value }))}
                  placeholder="Optional planner preferences or context that should be appended to the workspace methodology."
                  rows={4}
                />
              </div>
            )}

            {form.agent_class !== 'human' && (
              <Collapsible.Root open={advancedOpen} onOpenChange={setAdvancedOpen}>
                <Collapsible.Trigger asChild>
                  <Button type="button" variant="ghost" className="flex w-full items-center justify-between px-2">
                    <span className="flex items-center gap-2 text-sm">
                      {advancedOpen ? <ChevronDown className="h-4 w-4" /> : <ChevronRight className="h-4 w-4" />}
                      Advanced
                    </span>
                    {advancedConfigured && !advancedOpen && (
                      <span className="text-xs text-muted-foreground">Configured</span>
                    )}
                  </Button>
                </Collapsible.Trigger>
                <Collapsible.Content className="space-y-4 rounded-md border bg-muted/30 p-3">
                  <div className="space-y-1.5">
                    <Label>Runtime Kind</Label>
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

                  <div className="space-y-1.5">
                    <Label htmlFor="agent-skills">Skills</Label>
                    <Input
                      id="agent-skills"
                      value={form.skills}
                      onChange={(e) => setForm((current) => ({ ...current, skills: e.target.value }))}
                      placeholder="Comma-separated skill pack IDs"
                    />
                  </div>

                  <div className="space-y-1.5">
                    <Label htmlFor="agent-prompt">
                      {form.agent_class === 'product_planner' ? 'Advanced Planner Prompt' : 'System Prompt'}
                    </Label>
                    <Textarea
                      id="agent-prompt"
                      value={form.system_prompt}
                      onChange={(e) => setForm((current) => ({ ...current, system_prompt: e.target.value }))}
                      placeholder={
                        form.agent_class === 'product_planner'
                          ? 'Optional secondary notes. Workspace planning methodology remains authoritative.'
                          : 'Optional additional instructions for this agent.'
                      }
                      rows={4}
                    />
                    {form.agent_class === 'product_planner' && (
                      <p className="text-xs text-muted-foreground">
                        This is secondary to the workspace planning methodology and stage rules.
                      </p>
                    )}
                  </div>

                  <div className="space-y-1.5">
                    <Label htmlFor="agent-budget">Monthly Token Budget</Label>
                    <Input
                      id="agent-budget"
                      type="number"
                      value={form.monthly_token_budget}
                      onChange={(e) => setForm((current) => ({ ...current, monthly_token_budget: e.target.value }))}
                      placeholder="Optional budget limit"
                    />
                  </div>
                </Collapsible.Content>
              </Collapsible.Root>
            )}

            <div className="flex justify-between pt-2">
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
                <Button
                  variant="outline"
                  size="sm"
                  onClick={() => setDialogOpen(false)}
                >
                  Cancel
                </Button>
                <Button
                  size="sm"
                  disabled={saving || !form.name.trim()}
                  onClick={handleSave}
                >
                  {saving ? 'Saving...' : editingAgent ? 'Update' : 'Create'}
                </Button>
              </div>
            </div>
          </div>
        </DialogContent>
      </Dialog>

      <ConfirmDialog
        open={deleteConfirmOpen}
        onOpenChange={setDeleteConfirmOpen}
        title="Delete agent"
        description="This will permanently delete this agent and all its configuration. This action cannot be undone."
        confirmLabel="Delete"
        variant="destructive"
        onConfirm={handleDelete}
      />
    </div>
  );
}
