import { useMemo } from 'react';
import { Clock01Icon, Loading01Icon, CheckmarkCircle02Icon, Key01Icon, MessagePreview01Icon, CancelCircleIcon, SecurityCheckIcon } from '@/lib/icons';
import { Badge } from '@/components/ui/badge';
import { AgentAvatar } from '@/components/agents/AgentAvatar';
import { QuickTooltip } from '@/components/ui/quick-tooltip';
import { AGENT_RUNTIME_LABELS } from '@/lib/agentRuntime';
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

const BRANCH_MAX_CHARS = 28;

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
      <p className="px-3 py-4 text-xs text-muted-foreground">No runs yet. Click "Run Agent" to start.</p>
    );
  }

  return (
    <div>
      {/* Header */}
      <div className={`${TABLE_HEADER} flex`}>
        <div className={TABLE_HEADER_CELL} style={{ width: 120 }}>Status</div>
        <div className={TABLE_HEADER_CELL} style={{ width: 150 }}>Agent</div>
        <div className={TABLE_HEADER_CELL} style={{ flex: '1 1 0%', minWidth: 100 }}>Branch</div>
        <div className={TABLE_HEADER_CELL} style={{ width: 100 }}>Time</div>
        <div className={`${TABLE_HEADER_CELL} text-right`} style={{ width: 70 }}>Tokens</div>
      </div>

      {/* Rows */}
      {runs.map((run) => {
        const displayStatus = getAgentRunDisplayStatus(run);
        const meta = STATUS_META[displayStatus] ?? STATUS_META.queued;
        const isSelected = run.id === selectedRunId;
        const branch = run.working_branch ? `task ${run.working_branch}` : run.base_branch ? `base ${run.base_branch}` : '';
        const branchDisplay = branch ? truncateMiddle(branch, BRANCH_MAX_CHARS) : '-';
        const agent = agentsById.get(run.agent_id);
        const agentName = agent?.name ?? AGENT_RUNTIME_LABELS[run.runtime_kind] ?? run.runtime_kind;

        return (
          <button
            key={run.id}
            onClick={() => onSelectRun(run)}
            className={`${TABLE_ROW} w-full cursor-pointer text-left text-xs ${
              isSelected ? 'border-l-2 border-l-primary bg-accent/30' : 'border-l-2 border-l-transparent'
            }`}
          >
            <div className={TABLE_CELL} style={{ width: 120 }}>
              <Badge variant={meta.variant} className={`gap-1 px-1.5 py-0 text-[10px] ${meta.className ?? ''}`}>
                {STATUS_ICONS[displayStatus]}
                {meta.label}
              </Badge>
            </div>
            <div className={`${TABLE_CELL} text-foreground`} style={{ width: 150 }}>
              <div className="flex min-w-0 items-center gap-1.5">
                <AgentAvatar agent={agent} className="h-4 w-4" />
                <span className="truncate">{agentName}</span>
              </div>
            </div>
            <div className={`${TABLE_CELL} text-muted-foreground`} style={{ flex: '1 1 0%', minWidth: 100 }}>
              {branch ? (
                <QuickTooltip label={branch}>
                  <span className="truncate font-mono text-[11px]">{branchDisplay}</span>
                </QuickTooltip>
              ) : (
                <span className="truncate">-</span>
              )}
            </div>
            <div className={`${TABLE_CELL} text-muted-foreground`} style={{ width: 100 }}>
              {formatDistanceToNow(parseISO(run.created_at), { addSuffix: true })}
            </div>
            <div className={`${TABLE_CELL} justify-end text-muted-foreground`} style={{ width: 70 }}>
              {run.tokens_used > 0 ? `${(run.tokens_used / 1000).toFixed(1)}k` : '-'}
            </div>
          </button>
        );
      })}
    </div>
  );
}
