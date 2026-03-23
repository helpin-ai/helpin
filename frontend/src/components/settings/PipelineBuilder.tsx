import { useMemo, useState } from 'react';
import { automationRuleService } from '@/lib/services/automationRuleService';
import { StateTypeIcon } from '@/lib/pmConstants';
import type { Agent, AutomationRule, WorkflowState } from '@/lib/pmTypes';
import { Input } from '@/components/ui/input';
import { Switch } from '@/components/ui/switch';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { Bot, ChevronRight, GitBranch, X } from 'lucide-react';
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

  // Build a lookup: stateId → { runRule, advanceRule, mergeRule }
  const stateRuleMap = useMemo(() => {
    const map = new Map<string, { runRule?: AutomationRule; advanceRule?: AutomationRule; mergeRule?: AutomationRule }>();
    for (const state of states) {
      map.set(state.id, {});
    }
    for (const rule of rules) {
      const stateId = rule.trigger_config?.state_id;
      if (!stateId || !map.has(stateId)) continue;
      const entry = map.get(stateId)!;
      if (
        rule.trigger_type === 'story.state_entered' &&
        rule.action_type === 'start_agent_run'
      ) {
        entry.runRule = rule;
      } else if (rule.trigger_type === 'agent_run.approved' && rule.action_type === 'move_to_state') {
        entry.advanceRule = rule;
      } else if (rule.trigger_type === 'story.state_entered' && rule.action_type === 'merge_branch') {
        entry.mergeRule = rule;
      }
    }
    return map;
  }, [states, rules]);

  const handleAgentChange = async (stateId: string, stateName: string, agentId: string) => {
    setSaving(stateId);
    const entry = stateRuleMap.get(stateId);
    const existing = entry?.runRule;

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
        trigger_type: 'story.state_entered',
        trigger_config: { state_id: stateId },
        action_type: 'start_agent_run',
        action_config: { agent_id: agentId },
      });
    }
    setSaving(null);
    onChanged();
  };

  const handleAutoAdvanceToggle = async (stateId: string, stateName: string, enabled: boolean) => {
    setSaving(stateId);
    const existing = stateRuleMap.get(stateId)?.advanceRule;
    const stateIdx = states.findIndex((s) => s.id === stateId);
    const nextState = states[stateIdx + 1];

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
    setSaving(null);
    onChanged();
  };

  const handleMergeBranchToggle = async (stateId: string, stateName: string, branch: string) => {
    setSaving(stateId);
    const existing = stateRuleMap.get(stateId)?.mergeRule;

    if (!branch && existing) {
      await automationRuleService.remove(workspaceId, existing.id);
    } else if (branch && existing) {
      await automationRuleService.update(workspaceId, existing.id, { action_config: { target_branch: branch } });
    } else if (branch) {
      await automationRuleService.create(workspaceId, {
        workspace_id: workspaceId,
        name: `Merge branch on ${stateName}`,
        workflow_id: workflowId,
        trigger_type: 'story.state_entered',
        trigger_config: { state_id: stateId },
        action_type: 'merge_branch',
        action_config: { target_branch: branch },
      });
    }
    setSaving(null);
    onChanged();
  };

  if (states.length === 0) return null;

  return (
    <div className="space-y-3">
      <div className="flex items-center gap-2">
        <Bot className="h-4 w-4 text-violet-500" />
        <span className="text-sm font-semibold">Pipeline</span>
        <span className="text-xs text-muted-foreground">Assign agents to workflow stages</span>
      </div>

      <div className="overflow-x-auto">
        <div className="flex items-start gap-0 min-w-max pb-2">
          {states.map((state, idx) => {
            const entry = stateRuleMap.get(state.id);
            const runRule = entry?.runRule;
            const selectedAgentId = (runRule?.action_config?.agent_id as string) ?? '';
            const hasAdvance = !!entry?.advanceRule;
            const mergeBranch = (entry?.mergeRule?.action_config?.target_branch as string) ?? '';
            const isLast = idx === states.length - 1;
            const isSaving = saving === state.id;
            const hasExecution = !!selectedAgentId;

            return (
              <div key={state.id} className="flex items-start">
                {/* State card */}
                <div className={cn(
                  'relative w-[180px] shrink-0 rounded-lg border bg-background p-3 transition-colors',
                  hasExecution ? 'border-violet-300 dark:border-violet-800' : 'border-border',
                  isSaving && 'opacity-60',
                )}>
                  {/* State header */}
                  <div className="flex items-center gap-1.5 mb-2">
                    {state.color ? (
                      <span className="h-2.5 w-2.5 rounded-full shrink-0" style={{ backgroundColor: state.color }} />
                    ) : (
                      <StateTypeIcon stateType={state.state_type} className="h-3.5 w-3.5 shrink-0" />
                    )}
                    <span className="text-xs font-semibold truncate">{state.name}</span>
                  </div>

                  {/* Execution selector */}
                  {editable ? (
                    <div className="space-y-1.5">
                      <Select
                        value={selectedAgentId || '__none__'}
                        onValueChange={(v) => handleAgentChange(state.id, state.name, v === '__none__' ? '' : v)}
                      >
                        <SelectTrigger className="h-7 text-xs w-full">
                          <SelectValue placeholder="No agent" />
                        </SelectTrigger>
                        <SelectContent>
                          <SelectItem value="__none__">
                            <span className="text-muted-foreground">No agent</span>
                          </SelectItem>
                          {agents.map((agent) => (
                            <SelectItem key={agent.id} value={agent.id}>
                              <span className="flex items-center gap-1.5">
                                <Bot className="h-3 w-3 text-violet-500" />
                                {agent.name}
                              </span>
                            </SelectItem>
                          ))}
                        </SelectContent>
                      </Select>
                    </div>
                  ) : (
                    <div className="space-y-1 text-xs text-muted-foreground px-1">
                      {selectedAgentId ? (
                        <div className="flex items-center gap-1.5">
                          <Bot className="h-3 w-3 text-violet-500" />
                          <span className="truncate">{agents.find((agent) => agent.id === selectedAgentId)?.name ?? 'Agent'}</span>
                        </div>
                      ) : (
                        <span>No agent</span>
                      )}
                    </div>
                  )}

                  {/* Auto-advance toggle (only if execution configured and not last state) */}
                  {hasExecution && !isLast && (
                    <div className="mt-2 flex items-center gap-1.5">
                      <Switch
                        className="h-3.5 w-7 [&>span]:h-2.5 [&>span]:w-2.5"
                        checked={hasAdvance}
                        onCheckedChange={(v) => handleAutoAdvanceToggle(state.id, state.name, v)}
                        disabled={!editable}
                      />
                      <span className="text-[10px] text-muted-foreground">Auto-advance</span>
                    </div>
                  )}

                  {/* Merge branch (compact input) */}
                  {editable && (
                    <MergeBranchInput
                      value={mergeBranch}
                      onChange={(v) => handleMergeBranchToggle(state.id, state.name, v)}
                    />
                  )}
                </div>

                {/* Arrow connector */}
                {!isLast && (
                  <div className="flex items-center self-center pt-5">
                    <div className={cn(
                      'h-[2px] w-6',
                      hasAdvance ? 'bg-violet-400' : 'bg-border',
                    )} />
                    <ChevronRight className={cn(
                      'h-3.5 w-3.5 -ml-1',
                      hasAdvance ? 'text-violet-400' : 'text-border',
                    )} />
                  </div>
                )}
              </div>
            );
          })}
        </div>
      </div>
    </div>
  );
}

/** Small inline input for merge branch that appears on click */
function MergeBranchInput({ value, onChange }: { value: string; onChange: (v: string) => void }) {
  const [editing, setEditing] = useState(false);
  const isBaseBranch = value === '{base_branch}';
  const mode = isBaseBranch ? '{base_branch}' : value ? '__custom__' : '';
  const [customDraft, setCustomDraft] = useState(isBaseBranch ? '' : value);

  if (!editing && !value) {
    return (
      <button
        type="button"
        className="mt-2 flex items-center gap-1 text-[10px] text-muted-foreground hover:text-foreground transition-colors"
        onClick={() => { onChange('{base_branch}'); }}
      >
        <GitBranch className="h-2.5 w-2.5" />
        Merge branch...
      </button>
    );
  }

  if (!editing) {
    return (
      <div className="mt-2 flex items-center gap-1 text-[10px]">
        <GitBranch className="h-2.5 w-2.5 text-muted-foreground" />
        {isBaseBranch ? (
          <span className="truncate text-muted-foreground">→ base branch <span className="opacity-60">(from team defaults)</span></span>
        ) : (
          <span className="truncate text-muted-foreground">→ {value}</span>
        )}
        <button type="button" className="text-muted-foreground hover:text-foreground ml-0.5" onClick={() => setEditing(true)}>
          <ChevronRight className="h-2.5 w-2.5 rotate-90" />
        </button>
        <button type="button" className="text-muted-foreground hover:text-destructive" onClick={() => onChange('')}>
          <X className="h-2.5 w-2.5" />
        </button>
      </div>
    );
  }

  return (
    <div className="mt-2 space-y-1">
      <div className="flex items-center gap-1">
        <GitBranch className="h-2.5 w-2.5 text-muted-foreground shrink-0" />
        <Select
          value={mode || '{base_branch}'}
          onValueChange={(v) => {
            if (v === '{base_branch}') {
              onChange('{base_branch}');
              setEditing(false);
            } else {
              setCustomDraft('');
            }
          }}
        >
          <SelectTrigger className="h-5 text-[10px] px-1 flex-1">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="{base_branch}">Story&apos;s base branch</SelectItem>
            <SelectItem value="__custom__">Custom branch...</SelectItem>
          </SelectContent>
        </Select>
      </div>
      {mode === '__custom__' && (
        <Input
          className="h-5 text-[10px] px-1"
          value={customDraft}
          onChange={(e) => setCustomDraft(e.target.value)}
          placeholder="branch name"
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
