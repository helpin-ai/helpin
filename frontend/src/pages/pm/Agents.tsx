import { useCallback, useEffect, useState } from 'react';
import { Bot, Plus } from 'lucide-react';
import { useTitle } from '@/hooks/useTitle';
import { useWorkspaceStore } from '@/stores/workspaceStore';
import { agentService } from '@/lib/services/agentService';
import type { Agent, AgentKind, AgentRuntimeKind, AgentTriggerMode, CreateAgentRequest, RuntimeProfile, UpdateAgentRequest } from '@/lib/pmTypes';
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
          {agent.role && (
            <span className="text-xs text-muted-foreground">{agent.role}</span>
          )}
        </div>
      </CardHeader>
      <CardContent className="space-y-2 pt-0">
        {agent.agent_kind === 'llm' && agent.model && (
          <p className="text-xs text-muted-foreground">Model: {agent.model}</p>
        )}
        <p className="text-xs text-muted-foreground">
          Runtime: {agent.runtime_kind} · Profile: {agent.capability_profile}
        </p>
        <p className="text-xs text-muted-foreground">Trigger: {agent.trigger_mode}</p>
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

interface AgentFormData {
  name: string;
  agent_kind: AgentKind;
  role: string;
  runtime_kind: AgentRuntimeKind;
  capability_profile: string;
  trigger_mode: AgentTriggerMode;
  backing_user_id: string;
  skills: string;
  model: string;
  system_prompt: string;
  monthly_token_budget: string;
}

const EMPTY_FORM: AgentFormData = {
  name: '',
  agent_kind: 'human',
  role: '',
  runtime_kind: 'native_claude',
  capability_profile: 'engineer',
  trigger_mode: 'manual',
  backing_user_id: '',
  skills: '',
  model: '',
  system_prompt: '',
  monthly_token_budget: '',
};

const RUNTIME_KIND_OPTIONS: AgentRuntimeKind[] = ['native_claude', 'claude_code', 'openclaw', 'zeroclaw'];
const TRIGGER_MODE_OPTIONS: AgentTriggerMode[] = ['manual', 'auto_on_assignment', 'auto_on_event'];

export function AgentsPage() {
  useTitle('Agents');
  const workspace = useWorkspaceStore((state) => state.currentWorkspace);
  const workspaceId = workspace?.id;

  const [agents, setAgents] = useState<Agent[]>([]);
  const [runtimeProfiles, setRuntimeProfiles] = useState<RuntimeProfile[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const [dialogOpen, setDialogOpen] = useState(false);
  const [editingAgent, setEditingAgent] = useState<Agent | null>(null);
  const [form, setForm] = useState<AgentFormData>(EMPTY_FORM);
  const [saving, setSaving] = useState(false);

  const loadAgents = useCallback(async () => {
    if (!workspaceId) return;
    setLoading(true);
    setError(null);
    const [agentsRes, profilesRes] = await Promise.all([
      agentService.list(workspaceId),
      agentService.listRuntimeProfiles(workspaceId),
    ]);
    if (agentsRes.error) {
      setError(agentsRes.error);
    } else {
      setAgents(agentsRes.data ?? []);
    }
    if (!profilesRes.error) {
      setRuntimeProfiles(profilesRes.data ?? []);
    }
    setLoading(false);
  }, [workspaceId]);

  useEffect(() => {
    loadAgents();
  }, [loadAgents]);

  const openCreateDialog = () => {
    setEditingAgent(null);
    setForm(EMPTY_FORM);
    setDialogOpen(true);
  };

  const openEditDialog = (agent: Agent) => {
    setEditingAgent(agent);
    setForm({
      name: agent.name,
      agent_kind: agent.agent_kind,
      role: agent.role,
      runtime_kind: agent.runtime_kind,
      capability_profile: agent.capability_profile,
      trigger_mode: agent.trigger_mode,
      backing_user_id: agent.backing_user_id ?? '',
      skills: agent.skills.join(', '),
      model: agent.model ?? '',
      system_prompt: agent.system_prompt ?? '',
      monthly_token_budget: agent.monthly_token_budget?.toString() ?? '',
    });
    setDialogOpen(true);
  };

  const handleSave = async () => {
    if (!workspaceId || !form.name.trim()) return;
    setSaving(true);

    if (editingAgent) {
      const payload: UpdateAgentRequest = {
        name: form.name,
        role: form.role || undefined,
        backing_user_id: form.backing_user_id || undefined,
        runtime_kind: form.runtime_kind,
        capability_profile: form.capability_profile || undefined,
        trigger_mode: form.trigger_mode,
        skills: form.skills
          .split(',')
          .map((entry) => entry.trim())
          .filter(Boolean),
        model: form.agent_kind === 'llm' ? form.model || undefined : undefined,
        system_prompt: form.agent_kind === 'llm' ? form.system_prompt || undefined : undefined,
        monthly_token_budget:
          form.agent_kind === 'llm' && form.monthly_token_budget
            ? parseInt(form.monthly_token_budget, 10)
            : undefined,
      };
      const res = await agentService.update(workspaceId, editingAgent.id, payload);
      if (!res.error) {
        setDialogOpen(false);
        loadAgents();
      }
    } else {
      const payload: CreateAgentRequest = {
        workspace_id: workspaceId,
        name: form.name,
        agent_kind: form.agent_kind,
        role: form.role,
        backing_user_id: form.backing_user_id || undefined,
        runtime_kind: form.runtime_kind,
        capability_profile: form.capability_profile || undefined,
        trigger_mode: form.trigger_mode,
        skills: form.skills
          .split(',')
          .map((entry) => entry.trim())
          .filter(Boolean),
        model: form.agent_kind === 'llm' ? form.model || undefined : undefined,
        system_prompt: form.agent_kind === 'llm' ? form.system_prompt || undefined : undefined,
        monthly_token_budget:
          form.agent_kind === 'llm' && form.monthly_token_budget
            ? parseInt(form.monthly_token_budget, 10)
            : undefined,
      };
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
            <div className="space-y-1.5">
              <Label htmlFor="agent-name">Name</Label>
              <Input
                id="agent-name"
                value={form.name}
                onChange={(e) => setForm((f) => ({ ...f, name: e.target.value }))}
                placeholder="Agent name"
              />
            </div>

            {!editingAgent && (
              <div className="space-y-1.5">
                <Label>Kind</Label>
                <Select
                  value={form.agent_kind}
                  onValueChange={(v) =>
                    setForm((f) => ({ ...f, agent_kind: v as AgentKind }))
                  }
                >
                  <SelectTrigger>
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="human">Human</SelectItem>
                    <SelectItem value="llm">LLM</SelectItem>
                  </SelectContent>
                </Select>
              </div>
            )}

            <div className="space-y-1.5">
              <Label htmlFor="agent-role">Role</Label>
              <Input
                id="agent-role"
                value={form.role}
                onChange={(e) => setForm((f) => ({ ...f, role: e.target.value }))}
                placeholder="e.g. Coder, Tester, Reviewer"
              />
            </div>

            <div className="space-y-1.5">
              <Label>Runtime Kind</Label>
              <Select
                value={form.runtime_kind}
                onValueChange={(v) => setForm((f) => ({ ...f, runtime_kind: v as AgentRuntimeKind }))}
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
              <Label>Capability Profile</Label>
              <Select
                value={form.capability_profile}
                onValueChange={(v) => setForm((f) => ({ ...f, capability_profile: v }))}
              >
                <SelectTrigger>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {runtimeProfiles.map((profile) => (
                    <SelectItem key={profile.name} value={profile.name}>
                      {profile.name}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>

            <div className="space-y-1.5">
              <Label>Trigger Mode</Label>
              <Select
                value={form.trigger_mode}
                onValueChange={(v) => setForm((f) => ({ ...f, trigger_mode: v as AgentTriggerMode }))}
              >
                <SelectTrigger>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {TRIGGER_MODE_OPTIONS.map((triggerMode) => (
                    <SelectItem key={triggerMode} value={triggerMode}>
                      {triggerMode}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>

            {form.agent_kind === 'human' && (
              <div className="space-y-1.5">
                <Label htmlFor="agent-backing-user">Backing User ID</Label>
                <Input
                  id="agent-backing-user"
                  value={form.backing_user_id}
                  onChange={(e) => setForm((f) => ({ ...f, backing_user_id: e.target.value }))}
                  placeholder="Workspace user ID"
                />
              </div>
            )}

            <div className="space-y-1.5">
              <Label htmlFor="agent-skills">Skills</Label>
              <Input
                id="agent-skills"
                value={form.skills}
                onChange={(e) => setForm((f) => ({ ...f, skills: e.target.value }))}
                placeholder="Comma-separated skill pack IDs"
              />
            </div>

            {form.agent_kind === 'llm' && (
              <>
                <div className="space-y-1.5">
                  <Label htmlFor="agent-model">Model</Label>
                  <Input
                    id="agent-model"
                    value={form.model}
                    onChange={(e) => setForm((f) => ({ ...f, model: e.target.value }))}
                    placeholder="e.g. claude-opus-4-6"
                  />
                </div>

                <div className="space-y-1.5">
                  <Label htmlFor="agent-prompt">System Prompt</Label>
                  <Textarea
                    id="agent-prompt"
                    value={form.system_prompt}
                    onChange={(e) =>
                      setForm((f) => ({ ...f, system_prompt: e.target.value }))
                    }
                    placeholder="Instructions for the agent..."
                    rows={4}
                  />
                </div>

                <div className="space-y-1.5">
                  <Label htmlFor="agent-budget">Monthly Token Budget</Label>
                  <Input
                    id="agent-budget"
                    type="number"
                    value={form.monthly_token_budget}
                    onChange={(e) =>
                      setForm((f) => ({ ...f, monthly_token_budget: e.target.value }))
                    }
                    placeholder="e.g. 1000000"
                  />
                </div>
              </>
            )}

            <div className="flex justify-between pt-2">
              <div>
                {editingAgent && (
                  <Button
                    variant="destructive"
                    size="sm"
                    disabled={saving}
                    onClick={handleDelete}
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
    </div>
  );
}
