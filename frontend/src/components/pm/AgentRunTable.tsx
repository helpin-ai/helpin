import { useMemo } from 'react';
import { Clock01Icon, Loading01Icon, CheckmarkCircle02Icon, Key01Icon, MessagePreview01Icon, CancelCircleIcon, SecurityCheckIcon } from '@/lib/icons';
import { Badge } from '@/components/ui/badge';
import { AgentAvatar } from '@/components/agents/AgentAvatar';
import { QuickTooltip } from '@/components/ui/quick-tooltip';
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip';
import { formatRunTokenUsageBreakdown, formatRunTokenUsageTotal } from '@/lib/agentTokenUsage';
import { TABLE_HEADER, TABLE_HEADER_CELL, TABLE_ROW, TABLE_CELL } from '@/lib/tableStyles';
import { getAgentRunDisplayStatus, STATUS_META } from './agentRunConstants';
import type { Agent, AgentRun } from '@/lib/pmTypes';
import { formatDistanceToNow, parseISO } from 'date-fns';

interface Props {
  runs: AgentRun[];
  agents?: Agent[];
  selectedRunId: string | null;
  onSelectRun: (run: AgentRun) => void;
  loading: boolean;
}

const BRANCH_MAX_CHARS = 40;
const TOKEN_COLUMN_WIDTH = 100;

function truncateMiddle(value: string, max: number) {
  if (value.length <= max) return value;
  const keep = max - 1;
  const head = Math.ceil(keep / 2);
  const tail = Math.floor(keep / 2);
  return `${value.slice(0, head)}…${value.slice(value.length - tail)}`;
}

const STATUS_ICONS: Record<string, React.ReactNode> = {
  queued: <Clock01Icon className="h-3 w-3" />,
  running: <Loading01Icon className="h-3 w-3 animate-spin" />,
  awaiting_input: <MessagePreview01Icon className="h-3 w-3" />,
  awaiting_approval: <SecurityCheckIcon className="h-3 w-3" />,
  awaiting_auth: <Key01Icon className="h-3 w-3" />,
  completed: <CheckmarkCircle02Icon className="h-3 w-3" />,
  failed: <CancelCircleIcon className="h-3 w-3" />,
  cancelled: <CancelCircleIcon className="h-3 w-3" />,
};

export function AgentRunTable({ runs, agents, selectedRunId, onSelectRun, loading }: Props) {
  const agentsById = useMemo(() => {
    const map = new Map<string, Agent>();
    (agents ?? []).forEach((agent) => map.set(agent.id, agent));
    return map;
  }, [agents]);

  if (loading) {
    return (
      <div className="flex items-center gap-2 px-3 py-4 text-xs text-muted-foreground">
        <Loading01Icon className="h-3.5 w-3.5 animate-spin" />
        Loading runs...
      </div>
    );
  }

  if (runs.length === 0) {
    return (
      <p className="px-3 py-4 text-xs text-muted-foreground">No runs yet. Choose an agent and click Run.</p>
    );
  }

  return (
    <div>
      {/* Header */}
      <div className={`${TABLE_HEADER} flex`}>
        <div className={TABLE_HEADER_CELL} style={{ width: 120 }}>Status</div>
        <div className={TABLE_HEADER_CELL} style={{ width: 150 }}>Agent</div>
        <div className={TABLE_HEADER_CELL} style={{ flex: '1 1 0%', minWidth: 140 }}>Branch</div>
        <div className={TABLE_HEADER_CELL} style={{ width: 130 }}>Time</div>
        <div className={TABLE_HEADER_CELL} style={{ width: TOKEN_COLUMN_WIDTH }}>Tokens</div>
      </div>

      {/* Rows */}
      {runs.map((run) => {
        const displayStatus = getAgentRunDisplayStatus(run);
        const meta = STATUS_META[displayStatus] ?? STATUS_META.queued;
        const isSelected = run.id === selectedRunId;
        const branch = run.working_branch ? `task ${run.working_branch}` : run.base_branch ? `base ${run.base_branch}` : '';
        const branchDisplay = branch ? truncateMiddle(branch, BRANCH_MAX_CHARS) : '-';
        const agent = agentsById.get(run.agent_id);
        const agentName = agent?.name ?? 'Agent';
        const totalTokens = formatRunTokenUsageTotal(run);
        const tokenBreakdown = formatRunTokenUsageBreakdown(run).filter((line) => !line.startsWith('Total:'));

        return (
          <button
            key={run.id}
            onClick={() => onSelectRun(run)}
            className={`${TABLE_ROW} w-full cursor-pointer text-left text-xs ${
              isSelected ? 'border-l-2 border-l-primary bg-accent/30' : 'border-l-2 border-l-transparent'
            }`}
          >
            <div className={TABLE_CELL} style={{ width: 120 }}>
              {run.error_message ? (
                <Tooltip>
                  <TooltipTrigger asChild>
                    <Badge variant={meta.variant} className={`cursor-help gap-1 px-1.5 py-0 text-[10px] ${meta.className ?? ''}`}>
                      {STATUS_ICONS[displayStatus]}
                      {meta.label}
                    </Badge>
                  </TooltipTrigger>
                  <TooltipContent side="top" align="start" className="max-w-[360px] whitespace-pre-wrap break-words px-3 py-2 text-xs leading-relaxed">
                    {run.error_message}
                  </TooltipContent>
                </Tooltip>
              ) : (
                <QuickTooltip label="View run">
                  <Badge variant={meta.variant} className={`gap-1 px-1.5 py-0 text-[10px] ${meta.className ?? ''}`}>
                    {STATUS_ICONS[displayStatus]}
                    {meta.label}
                  </Badge>
                </QuickTooltip>
              )}
            </div>
            <div className={`${TABLE_CELL} text-foreground`} style={{ width: 150 }}>
              <div className="flex min-w-0 items-center gap-1.5">
                <AgentAvatar agent={agent} className="h-5 w-5 rounded-none border-0 bg-transparent shadow-none" genericBare />
                <span className="truncate">{agentName}</span>
                {run.input.execution_location === 'local' && <span className="text-[11px] text-quiet-text-tertiary">Local</span>}
              </div>
            </div>
            <div className={`${TABLE_CELL} text-muted-foreground`} style={{ flex: '1 1 0%', minWidth: 140 }}>
              {branch ? (
                <QuickTooltip label={branch}>
                  <span className="truncate font-mono text-[11px]">{branchDisplay}</span>
                </QuickTooltip>
              ) : (
                <span className="truncate">-</span>
              )}
            </div>
            <div className={`${TABLE_CELL} text-muted-foreground`} style={{ width: 130 }}>
              {formatDistanceToNow(parseISO(run.created_at), { addSuffix: true })}
            </div>
            <div className={`${TABLE_CELL} text-muted-foreground`} style={{ width: TOKEN_COLUMN_WIDTH }}>
              {tokenBreakdown.length > 0 ? (
                <Tooltip>
                  <TooltipTrigger asChild>
                    <span className="cursor-help tabular-nums">{totalTokens}</span>
                  </TooltipTrigger>
                  <TooltipContent side="top" align="start" className="space-y-1 px-3 py-2 text-xs">
                    {tokenBreakdown.map((line) => (
                      <div key={line}>{line}</div>
                    ))}
                  </TooltipContent>
                </Tooltip>
              ) : (
                <span className="tabular-nums">{totalTokens}</span>
              )}
            </div>
          </button>
        );
      })}
    </div>
  );
}
