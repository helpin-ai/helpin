import { useEffect, useMemo, useState } from 'react';

import { AgentAvatar, getAgentPersonaMeta } from '@/components/agents/AgentAvatar';
import { Button } from '@/components/ui/button';
import { Command, CommandEmpty, CommandGroup, CommandInput, CommandItem, CommandList } from '@/components/ui/command';
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover';
import { useAgents } from '@/hooks/queries/useAgents';
import { BotIcon, CheckmarkCircle02Icon, Loading01Icon, PlayIcon } from '@/lib/icons';
import type { Agent, AgentPresetKey } from '@/lib/pmTypes';
import { cn } from '@/lib/utils';

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
  hasRepoContext = false,
  disabled = false,
  autoSelectDefault = false,
  onRun,
  runDisabled = false,
  running = false,
  className,
}: AgentPickerCardProps) {
  const { data: agents = [], isLoading } = useAgents(workspaceId);
  const [open, setOpen] = useState(false);

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

  useEffect(() => {
    if (!autoSelectDefault || value || !defaultAgent || disabled) return;
    onChange(defaultAgent.id);
  }, [autoSelectDefault, defaultAgent, disabled, onChange, value]);

  return (
    <section className={cn('rounded-lg border border-border/70 bg-background p-3 shadow-sm', className)}>
      <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
        <div className="flex min-w-0 flex-1 items-center gap-2">
          <span className="inline-flex h-7 w-7 items-center justify-center rounded-md bg-[linear-gradient(135deg,hsl(var(--primary)/0.16),hsl(var(--accent)),hsl(var(--primary)/0.08))] text-primary ring-1 ring-primary/15">
            <BotIcon className="h-4 w-4 text-primary" />
          </span>
          <div className="min-w-0">
            <p className="text-sm font-medium leading-tight">Assign agent</p>
            <p className="text-xs leading-snug text-muted-foreground">{helper}</p>
          </div>
        </div>

        <div className="flex min-w-0 items-center gap-2 sm:w-[280px] sm:shrink-0">
          <Popover open={open} onOpenChange={setOpen}>
            <PopoverTrigger asChild>
              <Button
                type="button"
                variant="outline"
                size="sm"
                className="h-10 min-w-0 flex-1 justify-start gap-2 px-2"
                disabled={disabled || isLoading}
              >
                {selectedAgent ? <AgentAvatar agent={selectedAgent} className="h-6 w-6 rounded-lg" /> : null}
                <span className="min-w-0 flex-1 truncate text-left">
                  {selectedAgent ? formatAgentOptionLabel(selectedAgent) : isLoading ? 'Loading agents...' : 'No agent'}
                </span>
                {selectedAgent ? <CheckmarkCircle02Icon className="h-4 w-4 shrink-0 text-primary" /> : null}
              </Button>
            </PopoverTrigger>
            <PopoverContent align="end" className="w-72 p-0">
              <Command>
                <CommandInput placeholder="Search agents..." />
                <CommandList>
                  <CommandEmpty>No runnable agents found.</CommandEmpty>
                  <CommandGroup>
                    <CommandItem
                      value="No agent"
                      onSelect={() => {
                        onChange(undefined);
                        setOpen(false);
                      }}
                    >
                      <span className="inline-flex h-7 w-7 items-center justify-center rounded-lg bg-muted text-xs text-muted-foreground">-</span>
                      <div className="min-w-0">
                        <div className="truncate text-sm">No agent</div>
                        <div className="truncate text-xs text-muted-foreground">{manualTargetLabel}</div>
                      </div>
                    </CommandItem>
                    {runnableAgents.map((agent) => {
                      const meta = getAgentPersonaMeta({ agent });
                      return (
                        <CommandItem
                          key={agent.id}
                          value={`${agent.name} ${agent.preset_key ?? ''}`}
                          onSelect={() => {
                            onChange(agent.id);
                            setOpen(false);
                          }}
                        >
                          <AgentAvatar agent={agent} className="h-7 w-7 rounded-lg" />
                          <div className="min-w-0">
                            <div className="truncate text-sm">{formatAgentOptionLabel(agent)}</div>
                            <div className="truncate text-xs text-muted-foreground">{meta.role}</div>
                          </div>
                        </CommandItem>
                      );
                    })}
                  </CommandGroup>
                </CommandList>
              </Command>
            </PopoverContent>
          </Popover>
          {onRun ? (
            <Button
              type="button"
              size="sm"
              className="h-10 shrink-0"
              onClick={onRun}
              disabled={disabled || runDisabled || running || !selectedAgent}
            >
              {running ? <Loading01Icon className="h-4 w-4 animate-spin" /> : <PlayIcon className="h-4 w-4" />}
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
