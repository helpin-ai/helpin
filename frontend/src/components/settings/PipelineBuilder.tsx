import { useMemo, useEffect, useState } from 'react';
import { automationRuleService } from '@/lib/services/automationRuleService';
import { flowService } from '@/lib/services/flowService';
import { StateTypeIcon } from '@/lib/pmConstants';
import type { Agent, AutomationRule, FlowSpec, FlowTemplateNode, WorkflowState } from '@/lib/pmTypes';
import { extractAgentInputKeys, type AgentInputKeyConfig } from '@/components/pm/flowConstants';
import { Input } from '@/components/ui/input';
import { Switch } from '@/components/ui/switch';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { Bot, ChevronRight, GitBranch, Workflow, X } from 'lucide-react';
import { cn } from '@/lib/utils';

const KNOWN_STORY_TEMPLATES: FlowSpec[] = [
  { template_id: 'pm.agent_story_run', name: 'Agent Story Run', template_version: 1, target_type: 'story', nodes: [] },
  { template_id: 'pm.story_completion_v1', name: 'Story Completion', template_version: 1, target_type: 'story', nodes: [] },
];

export function PipelineBuilder({
  workspaceId,
  workflowId,
  states,
  agents,
  rules,
  flowTemplates,
  editable,
  onChanged,
}: {
  workspaceId: string;
  workflowId: string;
  states: WorkflowState[];
  agents: Agent[];
  rules: AutomationRule[];
  flowTemplates?: FlowSpec[];
  editable: boolean;
  onChanged: () => void;
}) {
  const [saving, setSaving] = useState<string | null>(null);

  // Filter to story-only templates (pipelines operate on stories)
  const storyTemplates = useMemo(() => {
    const fromApi = flowTemplates?.filter((ft) => ft.target_type === 'story') ?? [];
    const apiIds = new Set(fromApi.map((ft) => ft.template_id));
    const missing = KNOWN_STORY_TEMPLATES.filter((ft) => !apiIds.has(ft.template_id));
    return [...fromApi, ...missing];
  }, [flowTemplates]);

  // Cache of template nodes for resolving agent_input_key (keyed by template_id)
  const [templateNodes, setTemplateNodes] = useState<Map<string, FlowTemplateNode[]>>(new Map());

  // Build a lookup: stateId → { flowRule, advanceRule, mergeRule }
  const stateRuleMap = useMemo(() => {
    const map = new Map<string, { flowRule?: AutomationRule; advanceRule?: AutomationRule; mergeRule?: AutomationRule }>();
    for (const state of states) {
      map.set(state.id, {});
    }
    for (const rule of rules) {
      const stateId = rule.trigger_config?.state_id;
      if (!stateId || !map.has(stateId)) continue;
      const entry = map.get(stateId)!;
      if (rule.trigger_type === 'story.state_entered' && rule.action_type === 'start_flow') {
        entry.flowRule = rule;
      } else if (rule.trigger_type === 'agent_run.approved' && rule.action_type === 'move_to_state') {
        entry.advanceRule = rule;
      } else if (rule.trigger_type === 'story.state_entered' && rule.action_type === 'merge_branch') {
        entry.mergeRule = rule;
      }
    }
    return map;
  }, [states, rules]);

  // Collect unique template slugs in use to fetch their nodes
  const templateSlugsInUse = useMemo(() => {
    const slugs = new Set<string>();
    for (const [, entry] of stateRuleMap) {
      const tid = entry.flowRule?.action_config?.template_id as string | undefined;
      if (tid) slugs.add(tid);
    }
    return slugs;
  }, [stateRuleMap]);

  // Fetch template details for non-builtin templates that we haven't cached
  useEffect(() => {
    for (const slug of templateSlugsInUse) {
      if (templateNodes.has(slug)) continue;
      // Find the template entry to get its DB id
      const ft = storyTemplates.find((t) => t.template_id === slug);
      if (!ft) continue;
      flowService.getTemplate(workspaceId, ft.template_id).then((res) => {
        if (res.data?.nodes) {
          setTemplateNodes((prev) => new Map(prev).set(slug, res.data!.nodes));
        }
      });
    }
  }, [templateSlugsInUse, storyTemplates, workspaceId]); // eslint-disable-line react-hooks/exhaustive-deps

  const handleFlowTemplateChange = async (stateId: string, stateName: string, templateId: string) => {
    setSaving(stateId);
    const entry = stateRuleMap.get(stateId);
    const existing = entry?.flowRule;

    if (!templateId) {
      if (existing) await automationRuleService.remove(workspaceId, existing.id);
    } else if (existing) {
      // When changing template, clear flow_input (agent selections no longer apply)
      await automationRuleService.update(workspaceId, existing.id, { action_config: { template_id: templateId } });
    } else {
      await automationRuleService.create(workspaceId, {
        workspace_id: workspaceId,
        name: `Start flow on ${stateName}`,
        workflow_id: workflowId,
        trigger_type: 'story.state_entered',
        trigger_config: { state_id: stateId },
        action_type: 'start_flow',
        action_config: { template_id: templateId },
      });
    }
    setSaving(null);
    onChanged();
  };

  const handleAgentInputChange = async (stateId: string, key: string, agentId: string) => {
    setSaving(stateId);
    const entry = stateRuleMap.get(stateId);
    const rule = entry?.flowRule;
    if (!rule) { setSaving(null); return; }

    const existingInput = (rule.action_config?.flow_input as Record<string, string>) ?? {};
    // Carry over legacy top-level agent_id into flow_input on first edit
    const legacyAgentId = rule.action_config?.agent_id as string | undefined;
    const newFlowInput: Record<string, string> = { ...existingInput };
    if (key === 'agent_id' && legacyAgentId && !existingInput.agent_id) {
      // Will be overwritten below anyway
    } else if (legacyAgentId && !existingInput.agent_id) {
      newFlowInput.agent_id = legacyAgentId;
    }

    if (agentId) {
      newFlowInput[key] = agentId;
    } else {
      delete newFlowInput[key];
    }

    await automationRuleService.update(workspaceId, rule.id, {
      action_config: {
        template_id: rule.action_config?.template_id as string,
        flow_input: Object.keys(newFlowInput).length > 0 ? newFlowInput : undefined,
      },
    });
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

  /** Resolve agent input keys for a template, using DB nodes if available */
  const getAgentInputKeys = (templateId: string): AgentInputKeyConfig[] => {
    return extractAgentInputKeys(templateId, templateNodes.get(templateId));
  };

  /** Get the currently selected agent for a given input key from a rule's action_config */
  const getAgentForKey = (rule: AutomationRule, key: string): string => {
    // Check flow_input first (new format)
    const flowInput = rule.action_config?.flow_input as Record<string, string> | undefined;
    if (flowInput?.[key]) return flowInput[key];
    // Legacy: top-level agent_id
    if (key === 'agent_id') {
      return (rule.action_config?.agent_id as string) ?? '';
    }
    return '';
  };

  if (states.length === 0) return null;

  return (
    <div className="space-y-3">
      <div className="flex items-center gap-2">
        <Bot className="h-4 w-4 text-violet-500" />
        <span className="text-sm font-semibold">Pipeline</span>
        <span className="text-xs text-muted-foreground">Assign flows to workflow stages</span>
      </div>

      <div className="overflow-x-auto">
        <div className="flex items-start gap-0 min-w-max pb-2">
          {states.map((state, idx) => {
            const entry = stateRuleMap.get(state.id);
            const flowRule = entry?.flowRule;
            const flowTemplateId = (flowRule?.action_config?.template_id as string) ?? '';
            const hasAdvance = !!entry?.advanceRule;
            const mergeBranch = (entry?.mergeRule?.action_config?.target_branch as string) ?? '';
            const isLast = idx === states.length - 1;
            const isSaving = saving === state.id;
            const hasExecution = !!flowRule;

            // Resolve agent input keys for the selected template
            const agentInputKeys = flowTemplateId ? getAgentInputKeys(flowTemplateId) : [];

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

                  {/* Execution selector — flow template + inline agent pickers */}
                  {editable ? (
                    <div className="space-y-1.5">
                      {/* Flow template picker */}
                      <Select
                        value={flowTemplateId || '__none__'}
                        onValueChange={(v) => handleFlowTemplateChange(state.id, state.name, v === '__none__' ? '' : v)}
                      >
                        <SelectTrigger className="h-7 text-xs w-full">
                          <SelectValue placeholder="No flow" />
                        </SelectTrigger>
                        <SelectContent>
                          <SelectItem value="__none__">
                            <span className="text-muted-foreground">No flow</span>
                          </SelectItem>
                          {storyTemplates.map((ft) => (
                            <SelectItem key={ft.template_id} value={ft.template_id}>
                              <span className="flex items-center gap-1.5">
                                <Workflow className="h-3 w-3 text-blue-500" />
                                {ft.name || ft.template_id}
                              </span>
                            </SelectItem>
                          ))}
                        </SelectContent>
                      </Select>

                      {/* Inline agent pickers driven by template's agent_input_key nodes */}
                      {flowRule && agentInputKeys.map((aik) => {
                        const selectedAgent = getAgentForKey(flowRule, aik.key);
                        const filteredAgents = aik.agentClassFilter
                          ? agents.filter((a) => a.agent_class === aik.agentClassFilter)
                          : agents;
                        return (
                          <Select
                            key={aik.key}
                            value={selectedAgent || '__none__'}
                            onValueChange={(v) => handleAgentInputChange(state.id, aik.key, v === '__none__' ? '' : v)}
                          >
                            <SelectTrigger className="h-7 text-xs w-full">
                              <SelectValue placeholder={aik.optionalHint ?? `Select ${aik.label}...`} />
                            </SelectTrigger>
                            <SelectContent>
                              <SelectItem value="__none__">
                                <span className="text-muted-foreground">
                                  {aik.required ? `Select ${aik.label}...` : (aik.optionalHint ?? 'None')}
                                </span>
                              </SelectItem>
                              {filteredAgents.map((a) => (
                                <SelectItem key={a.id} value={a.id}>
                                  <span className="flex items-center gap-1.5">
                                    <Bot className="h-3 w-3 text-violet-500" />
                                    {a.name}
                                  </span>
                                </SelectItem>
                              ))}
                            </SelectContent>
                          </Select>
                        );
                      })}
                    </div>
                  ) : (
                    <div className="space-y-1 text-xs text-muted-foreground px-1">
                      {flowRule ? (
                        <>
                          <div className="flex items-center gap-1.5">
                            <Workflow className="h-3 w-3 text-blue-500" />
                            <span className="truncate">{storyTemplates.find((ft) => ft.template_id === flowTemplateId)?.name ?? flowTemplateId}</span>
                          </div>
                          {agentInputKeys.map((aik) => {
                            const agentId = getAgentForKey(flowRule, aik.key);
                            const agentName = agents.find((a) => a.id === agentId)?.name;
                            if (!agentId) return null;
                            return (
                              <div key={aik.key} className="flex items-center gap-1.5 ml-4">
                                <Bot className="h-3 w-3 text-violet-500" />
                                <span className="truncate">{agentName ?? 'Agent'}</span>
                              </div>
                            );
                          })}
                        </>
                      ) : (
                        <span>No flow</span>
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
