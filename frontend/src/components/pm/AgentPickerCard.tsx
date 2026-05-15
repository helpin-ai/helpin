import { useEffect, useMemo } from 'react';

import { AgentAvatar, getAgentPersonaMeta } from '@/components/agents/AgentAvatar';
import { Button } from '@/components/ui/button';
import { useAgents } from '@/hooks/queries/useAgents';
import { BotIcon, Loading01Icon, PlayIcon } from '@/lib/icons';
import type { Agent, AgentPresetKey } from '@/lib/pmTypes';
import { cn } from '@/lib/utils';
import { SidebarPopoverSelect } from './SidebarPopoverSelect';

type RunnableTarget = 'task' | 'epic';

interface AgentPickerCardProps {
  workspaceId: string;
  value?: string;
  onChange: (agentId: string | undefined) => void;
  runnableTarget: RunnableTarget;
  targetTeamId?: string | null;
  hasRepoContext?: boolean;
  disabled?: boolean;
  autoSelectDefault?: boolean;
  onRun?: () => void | Promise<void>;
  runDisabled?: boolean;
  running?: boolean;
  className?: string;
}

const DEFAULT_PRESET: Record<RunnableTarget, AgentPresetKey> = {
  epic: 'epic_planner',
  task: 'task_planner',
};

const DEFAULT_NAME: Record<RunnableTarget, string> = {
  epic: 'Atlas',
  task: 'Scribe',
};

export function isRunnableAgentForTarget(agent: Agent, target: RunnableTarget) {
  return agent.allowed_targets?.includes(target);
}

export function pickDefaultAgentForTarget(agents: Agent[], target: RunnableTarget): Agent | undefined {
  const runnable = agents.filter((agent) => isRunnableAgentForTarget(agent, target));
  const preset = DEFAULT_PRESET[target];
  const name = DEFAULT_NAME[target].toLowerCase();

  return (
    runnable.find((agent) => agent.is_system && agent.preset_key === preset) ??
    runnable.find((agent) => agent.preset_key === preset) ??
    runnable.find((agent) => agent.name.trim().toLowerCase() === name) ??
    runnable[0]
  );
}

export function formatAgentOptionLabel(agent: Agent) {
  const name = agent.name.trim();
  const role = agent.role.trim();
  if (!role || role.toLowerCase() === name.toLowerCase()) {
    return name;
  }
  return `${name} - ${role}`;
}

export function AgentPickerCard({
  workspaceId,
  value,
  onChange,
  runnableTarget,
  targetTeamId,
  disabled = false,
  autoSelectDefault = false,
  onRun,
  runDisabled = false,
  running = false,
  className,
}: AgentPickerCardProps) {
  const { data: agents = [], isLoading } = useAgents(workspaceId);

  const runnableAgents = useMemo(
    () => agents.filter((agent) => isRunnableAgentForTarget(agent, runnableTarget) && isAgentInTargetTeamScope(agent, targetTeamId)),
    [agents, runnableTarget, targetTeamId],
  );
  const defaultAgent = useMemo(
    () => pickDefaultAgentForTarget(agents.filter((agent) => isAgentInTargetTeamScope(agent, targetTeamId)), runnableTarget),
    [agents, runnableTarget, targetTeamId],
  );
  const selectedAgent = runnableAgents.find((agent) => agent.id === value);
  const helper = 'Use AI agents with business and repo context for coding, planning, marketing, support, and more.';
  const manualTargetLabel = runnableTarget === 'epic' ? 'Manual epic' : 'Manual task';
  const selectedValue = value ?? '__none__';
  const options = useMemo(
    () => [
      { value: '__none__', label: 'No agent' },
      ...runnableAgents.map((agent) => ({
        value: agent.id,
        label: formatAgentOptionLabel(agent),
      })),
    ],
    [runnableAgents],
  );
  const agentById = useMemo(
    () => new Map(runnableAgents.map((agent) => [agent.id, agent])),
    [runnableAgents],
  );

  useEffect(() => {
    if (!autoSelectDefault || value || !defaultAgent || disabled) return;
    onChange(defaultAgent.id);
  }, [autoSelectDefault, defaultAgent, disabled, onChange, value]);

  return (
    <section className={cn('py-2', className)}>
      <div className="flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between">
        <div className="flex min-w-0 flex-1 items-start gap-2">
          <BotIcon className="mt-0.5 h-4 w-4 shrink-0 text-primary" />
          <div className="min-w-0">
            <p className="text-sm font-medium leading-tight">Assign agent</p>
            <p className="mt-0.5 text-xs leading-snug text-muted-foreground">{helper}</p>
          </div>
        </div>

        <div className="flex min-w-0 items-center gap-2 sm:w-[260px] sm:shrink-0">
          <SidebarPopoverSelect
            value={selectedValue}
            options={options}
            onChange={(nextValue) => onChange(nextValue === '__none__' ? undefined : nextValue)}
            width="w-72"
            searchPlaceholder="Search agents..."
            disabled={disabled || isLoading}
            showChevron
            triggerClassName="h-7 flex-1 justify-start text-xs"
            emptyContent={<div className="px-2 py-3 text-xs text-muted-foreground">No runnable agents found.</div>}
            renderTrigger={() => (
              <>
                {selectedAgent ? <AgentAvatar agent={selectedAgent} className="h-5 w-5 rounded-md" /> : null}
                <span className="min-w-0 flex-1 truncate text-left">
                  {selectedAgent ? formatAgentOptionLabel(selectedAgent) : isLoading ? 'Loading agents...' : 'No agent selected'}
                </span>
              </>
            )}
            renderOption={(optionValue) => {
              if (optionValue === '__none__') {
                return (
                  <>
                    <span className="inline-flex h-5 w-5 shrink-0 items-center justify-center rounded-md bg-muted text-[10px] text-muted-foreground">-</span>
                    <span className="min-w-0 flex-1 text-left">
                      <span className="block truncate">No agent</span>
                      <span className="block truncate text-[11px] font-normal text-muted-foreground">{manualTargetLabel}</span>
                    </span>
                  </>
                );
              }

              const agent = agentById.get(optionValue);
              if (!agent) return null;
              const meta = getAgentPersonaMeta({ agent });

              return (
                <>
                  <AgentAvatar agent={agent} className="h-5 w-5 shrink-0 rounded-md" />
                  <span className="min-w-0 flex-1 text-left">
                    <span className="block truncate">{formatAgentOptionLabel(agent)}</span>
                    <span className="block truncate text-[11px] font-normal text-muted-foreground">{meta.role}</span>
                  </span>
                </>
              );
            }}
          />
          {onRun ? (
            <Button
              type="button"
              size="sm"
              className="h-7 shrink-0 text-xs"
              onClick={onRun}
              disabled={disabled || runDisabled || running || !selectedAgent}
            >
              {running ? <Loading01Icon className="h-3.5 w-3.5 animate-spin" /> : <PlayIcon className="h-3.5 w-3.5" />}
              Run
            </Button>
          ) : null}
        </div>
      </div>
    </section>
  );
}

function isAgentInTargetTeamScope(agent: Agent, targetTeamId?: string | null) {
  if (!agent.team_id) return true;
  return Boolean(targetTeamId) && agent.team_id === targetTeamId;
}
