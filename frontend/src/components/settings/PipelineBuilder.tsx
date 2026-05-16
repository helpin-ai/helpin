import { useMemo, useState, type ReactNode } from 'react';
import { BASE_BRANCH_TOKEN, describeMergeInto } from '@/lib/branchLabels';
import { automationRuleService } from '@/lib/services/automationRuleService';
import { StateTypeIcon } from '@/lib/pmConstants';
import type { Agent, AutomationRule, WorkflowState } from '@/lib/pmTypes';
import { Input } from '@/components/ui/input';
import { Switch } from '@/components/ui/switch';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { BotIcon, GitBranchIcon, Cancel01Icon, CheckmarkCircle02Icon, Loading01Icon, ZapIcon } from '@/lib/icons';
import { cn } from '@/lib/utils';

export function PipelineBuilder({
  workspaceId,
  workflowId,
  states,
  agents,
  rules,
  editable,
  onChanged,
}: {
  workspaceId: string;
  workflowId: string;
  states: WorkflowState[];
  agents: Agent[];
  rules: AutomationRule[];
  editable: boolean;
  onChanged: () => void;
}) {
  const [saving, setSaving] = useState<string | null>(null);

  const stateRuleMap = useMemo(() => {
    const map = new Map<string, { runRule?: AutomationRule; advanceRule?: AutomationRule; mergeRule?: AutomationRule }>();
    for (const state of states) {
      map.set(state.id, {});
    }
    for (const rule of rules) {
      const stateId = rule.trigger_config?.state_id;
      if (!stateId || !map.has(stateId)) continue;
      const entry = map.get(stateId)!;
      if (rule.trigger_type === 'task.state_entered' && rule.action_type === 'start_agent_run') {
        entry.runRule = rule;
      } else if (rule.trigger_type === 'agent_run.approved' && rule.action_type === 'move_to_state') {
        entry.advanceRule = rule;
      } else if (rule.trigger_type === 'task.state_entered' && rule.action_type === 'merge_branch') {
        entry.mergeRule = rule;
      }
    }
    return map;
  }, [states, rules]);

  const configuredCount = useMemo(() => {
    let count = 0;
    for (const entry of stateRuleMap.values()) {
      if (entry.runRule || entry.advanceRule || entry.mergeRule) count += 1;
    }
    return count;
  }, [stateRuleMap]);

  const handleAgentChange = async (stateId: string, stateName: string, agentId: string) => {
    setSaving(stateId);
    const entry = stateRuleMap.get(stateId);
    const existing = entry?.runRule;

    try {
      if (!agentId) {
        if (existing) await automationRuleService.remove(workspaceId, existing.id);
      } else if (existing) {
        await automationRuleService.update(workspaceId, existing.id, {
          action_type: 'start_agent_run',
          action_config: { agent_id: agentId },
        });
      } else {
        await automationRuleService.create(workspaceId, {
          workspace_id: workspaceId,
          name: `Run agent on ${stateName}`,
          workflow_id: workflowId,
          trigger_type: 'task.state_entered',
          trigger_config: { state_id: stateId },
          action_type: 'start_agent_run',
          action_config: { agent_id: agentId },
        });
      }
      onChanged();
    } finally {
      setSaving(null);
    }
  };

  const handleAutoAdvanceToggle = async (stateId: string, stateName: string, enabled: boolean) => {
    setSaving(stateId);
    const existing = stateRuleMap.get(stateId)?.advanceRule;
    const stateIdx = states.findIndex((s) => s.id === stateId);
    const nextState = states[stateIdx + 1];

    try {
      if (!enabled && existing) {
        await automationRuleService.remove(workspaceId, existing.id);
      } else if (enabled && !existing && nextState) {
        await automationRuleService.create(workspaceId, {
          workspace_id: workspaceId,
          name: `Auto-advance from ${stateName}`,
          workflow_id: workflowId,
          trigger_type: 'agent_run.approved',
          trigger_config: { state_id: stateId },
          action_type: 'move_to_state',
          action_config: { target_state_id: nextState.id },
        });
      }
      onChanged();
    } finally {
      setSaving(null);
    }
  };

  const handleMergeBranchToggle = async (stateId: string, stateName: string, branch: string) => {
    setSaving(stateId);
    const existing = stateRuleMap.get(stateId)?.mergeRule;

    try {
      if (!branch && existing) {
        await automationRuleService.remove(workspaceId, existing.id);
      } else if (branch && existing) {
        await automationRuleService.update(workspaceId, existing.id, { action_config: { target_branch: branch } });
      } else if (branch) {
        await automationRuleService.create(workspaceId, {
          workspace_id: workspaceId,
          name: `Merge task branch on ${stateName}`,
          workflow_id: workflowId,
          trigger_type: 'task.state_entered',
          trigger_config: { state_id: stateId },
          action_type: 'merge_branch',
          action_config: { target_branch: branch },
        });
      }
      onChanged();
    } finally {
      setSaving(null);
    }
  };

  if (states.length === 0) return null;

  return (
    <section className="space-y-3">
      <div className="flex flex-wrap items-start justify-between gap-3">
        <div className="min-w-0">
          <div className="flex items-center gap-2">
            <ZapIcon className="h-4 w-4 text-violet-500" />
            <h3 className="text-sm font-semibold">Pipeline automation</h3>
          </div>
          <p className="mt-1 text-xs text-muted-foreground">
            Configure what happens when tasks enter each workflow state.
          </p>
        </div>
        <div className="rounded-full border border-border bg-muted/30 px-2.5 py-1 text-xs text-muted-foreground">
          {configuredCount}/{states.length} configured
        </div>
      </div>

      <div className="space-y-2">
        {states.map((state, idx) => {
          const entry = stateRuleMap.get(state.id);
          const runRule = entry?.runRule;
          const selectedAgentId = (runRule?.action_config?.agent_id as string) ?? '';
          const selectedAgent = agents.find((agent) => agent.id === selectedAgentId);
          const hasAdvance = !!entry?.advanceRule;
          const mergeBranch = (entry?.mergeRule?.action_config?.target_branch as string) ?? '';
          const isLast = idx === states.length - 1;
          const isSaving = saving === state.id;
          const hasExecution = !!selectedAgentId;
          const nextState = states[idx + 1];

          return (
            <div
              key={state.id}
              className={cn(
                'rounded-lg border bg-card transition-colors',
                hasExecution || mergeBranch ? 'border-violet-300/70 dark:border-violet-800/70' : 'border-border',
                isSaving && 'opacity-70',
              )}
            >
              <div className="grid gap-3 p-3 lg:grid-cols-[minmax(170px,230px)_1fr]">
                <div className="min-w-0 space-y-2">
                  <div className="flex min-w-0 items-center gap-2">
                    <span className="flex h-6 w-6 shrink-0 items-center justify-center rounded-md bg-muted text-[11px] font-medium text-muted-foreground">
                      {idx + 1}
                    </span>
                    {state.color ? (
                      <span className="h-2.5 w-2.5 shrink-0 rounded-full" style={{ backgroundColor: state.color }} />
                    ) : (
                      <StateTypeIcon stateType={state.state_type} className="h-3.5 w-3.5 shrink-0" />
                    )}
                    <span className="truncate text-sm font-medium">{state.name}</span>
                    {isSaving ? <Loading01Icon className="h-3.5 w-3.5 shrink-0 animate-spin text-muted-foreground" /> : null}
                  </div>
                  <div className="flex flex-wrap gap-1.5 pl-8">
                    {hasExecution ? <StatusPill tone="violet">Agent</StatusPill> : null}
                    {hasAdvance ? <StatusPill tone="green">Advances</StatusPill> : null}
                    {mergeBranch ? <StatusPill tone="blue">Merge</StatusPill> : null}
                    {!hasExecution && !hasAdvance && !mergeBranch ? <StatusPill>Manual</StatusPill> : null}
                  </div>
                </div>

                <div className="grid min-w-0 gap-2 md:grid-cols-[minmax(220px,1fr)_minmax(180px,240px)] xl:grid-cols-[minmax(240px,1fr)_220px_minmax(190px,240px)]">
                  <div className="min-w-0 rounded-md border border-border/60 bg-background/60 p-2.5">
                    <div className="mb-2 flex items-center gap-1.5 text-xs font-medium">
                      <BotIcon className="h-3.5 w-3.5 text-violet-500" />
                      Run agent
                    </div>
                    {editable ? (
                      <Select
                        value={selectedAgentId || '__none__'}
                        onValueChange={(v) => void handleAgentChange(state.id, state.name, v === '__none__' ? '' : v)}
                        disabled={isSaving}
                      >
                        <SelectTrigger className="h-8 w-full text-xs">
                          <SelectValue placeholder="No agent" />
                        </SelectTrigger>
                        <SelectContent>
                          <SelectItem value="__none__">
                            <span className="text-muted-foreground">No agent</span>
                          </SelectItem>
                          {agents.map((agent) => (
                            <SelectItem key={agent.id} value={agent.id}>
                              {agent.name}
                            </SelectItem>
                          ))}
                        </SelectContent>
                      </Select>
                    ) : (
                      <p className="truncate text-xs text-muted-foreground">{selectedAgent?.name ?? 'No agent'}</p>
                    )}
                  </div>

                  <div className="rounded-md border border-border/60 bg-background/60 p-2.5">
                    <div className="mb-2 flex items-center gap-1.5 text-xs font-medium">
                      <CheckmarkCircle02Icon className="h-3.5 w-3.5 text-emerald-500" />
                      After approval
                    </div>
                    <div className="flex items-center justify-between gap-3">
                      <div className="min-w-0">
                        <p className="truncate text-xs text-muted-foreground">
                          {isLast ? 'No next state' : nextState ? `Move to ${nextState.name}` : 'Move forward'}
                        </p>
                      </div>
                      <Switch
                        checked={hasAdvance}
                        onCheckedChange={(v) => void handleAutoAdvanceToggle(state.id, state.name, v)}
                        disabled={!editable || !hasExecution || isLast || isSaving}
                      />
                    </div>
                  </div>

                  <div className="rounded-md border border-border/60 bg-background/60 p-2.5 md:col-span-2 xl:col-span-1">
                    <div className="mb-2 flex items-center gap-1.5 text-xs font-medium">
                      <GitBranchIcon className="h-3.5 w-3.5 text-sky-500" />
                      Merge branch
                    </div>
                    <MergeBranchInput
                      value={mergeBranch}
                      editable={editable && !isSaving}
                      onChange={(v) => void handleMergeBranchToggle(state.id, state.name, v)}
                    />
                  </div>
                </div>
              </div>
            </div>
          );
        })}
      </div>
    </section>
  );
}

function StatusPill({ children, tone = 'muted' }: { children: ReactNode; tone?: 'muted' | 'violet' | 'green' | 'blue' }) {
  return (
    <span
      className={cn(
        'inline-flex items-center rounded-full border px-1.5 py-0.5 text-[10px] font-medium',
        tone === 'muted' && 'border-border bg-muted/40 text-muted-foreground',
        tone === 'violet' && 'border-violet-500/20 bg-violet-500/10 text-violet-700 dark:text-violet-300',
        tone === 'green' && 'border-emerald-500/20 bg-emerald-500/10 text-emerald-700 dark:text-emerald-300',
        tone === 'blue' && 'border-sky-500/20 bg-sky-500/10 text-sky-700 dark:text-sky-300',
      )}
    >
      {children}
    </span>
  );
}

function MergeBranchInput({ value, editable, onChange }: { value: string; editable: boolean; onChange: (v: string) => void }) {
  const [editing, setEditing] = useState(false);
  const isBaseBranch = value === BASE_BRANCH_TOKEN;
  const mode = isBaseBranch ? BASE_BRANCH_TOKEN : value ? '__custom__' : '';
  const [customDraft, setCustomDraft] = useState(isBaseBranch ? '' : value);

  if (!editable) {
    return (
      <p className="truncate text-xs text-muted-foreground">
        {value ? describeMergeInto(value) : 'No merge action'}
      </p>
    );
  }

  if (!editing && !value) {
    return (
      <button
        type="button"
        className="inline-flex h-8 w-full items-center justify-start gap-1.5 rounded-md border border-dashed border-border px-2 text-xs text-muted-foreground transition-colors hover:border-border hover:bg-accent hover:text-foreground"
        onClick={() => { onChange(BASE_BRANCH_TOKEN); }}
      >
        <GitBranchIcon className="h-3.5 w-3.5" />
        Add merge action
      </button>
    );
  }

  if (!editing) {
    return (
      <div className="flex h-8 min-w-0 items-center gap-1.5 rounded-md border border-border bg-background px-2 text-xs">
        <span className="min-w-0 flex-1 truncate text-muted-foreground">
          {describeMergeInto(value)}
        </span>
        <button type="button" className="shrink-0 rounded px-1 text-muted-foreground hover:text-foreground" onClick={() => setEditing(true)}>
          Edit
        </button>
        <button type="button" className="shrink-0 text-muted-foreground hover:text-destructive" onClick={() => onChange('')} aria-label="Remove merge action">
          <Cancel01Icon className="h-3.5 w-3.5" />
        </button>
      </div>
    );
  }

  return (
    <div className="space-y-2">
      <Select
        value={mode || BASE_BRANCH_TOKEN}
        onValueChange={(v) => {
          if (v === BASE_BRANCH_TOKEN) {
            onChange(BASE_BRANCH_TOKEN);
            setEditing(false);
          } else {
            setCustomDraft('');
          }
        }}
      >
        <SelectTrigger className="h-8 text-xs">
          <SelectValue />
        </SelectTrigger>
        <SelectContent>
          <SelectItem value={BASE_BRANCH_TOKEN}>Task base branch</SelectItem>
          <SelectItem value="__custom__">Custom branch</SelectItem>
        </SelectContent>
      </Select>
      {mode === '__custom__' && (
        <Input
          className="h-8 text-xs"
          value={customDraft}
          onChange={(e) => setCustomDraft(e.target.value)}
          placeholder="Branch name"
          autoFocus
          onKeyDown={(e) => {
            if (e.key === 'Enter' && customDraft.trim()) { onChange(customDraft.trim()); setEditing(false); }
            if (e.key === 'Escape') setEditing(false);
          }}
          onBlur={() => { if (customDraft.trim()) { onChange(customDraft.trim()); setEditing(false); } }}
        />
      )}
    </div>
  );
}
