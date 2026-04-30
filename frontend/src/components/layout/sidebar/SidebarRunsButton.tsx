import { useMemo } from 'react';
import { useCommandBarRunStore } from '@/stores/commandBarStore';
import { ACTIVE_RUN_STATUSES } from '@/components/pm/agentRunConstants';
import type { AgentRun } from '@/lib/pm-types/agents';

export function SidebarRunsButton() {
  const runsById = useCommandBarRunStore((s) => s.runsById);

  const { active, failed } = useMemo(() => {
    let a = 0;
    let f = 0;
    for (const r of Object.values(runsById) as AgentRun[]) {
      if (ACTIVE_RUN_STATUSES.has(r.status)) a++;
      else if (r.status === 'failed') f++;
    }
    return { active: a, failed: f };
  }, [runsById]);

  const titleParts: string[] = [];
  if (active) titleParts.push(`${active} running`);
  if (failed) titleParts.push(`${failed} failed`);
  const title =
    titleParts.length > 0 ? `Recent runs · ${titleParts.join(' · ')}` : 'Recent runs';

  const onClick = () => {
    window.dispatchEvent(
      new CustomEvent('helpin:ask-agents', { detail: { mode: 'runs' } }),
    );
  };

  return (
    <button
      type="button"
      aria-label={title}
      title={title}
      onClick={onClick}
      className="relative flex size-8 items-center justify-center rounded-md text-muted-foreground transition-colors hover:bg-muted/80 hover:text-foreground"
    >
      <svg
        xmlns="http://www.w3.org/2000/svg"
        viewBox="0 0 24 24"
        fill="none"
        stroke="currentColor"
        strokeWidth={2}
        strokeLinecap="round"
        strokeLinejoin="round"
        aria-hidden
        className="h-3.5 w-3.5"
      >
        <path d="M3 12h3.28a1 1 0 0 1 .948.684l2.298 7.934a.5.5 0 0 0 .96-.044L13.82 4.771A1 1 0 0 1 14.792 4H21" />
      </svg>
      {active > 0 ? (
        <span className="absolute -right-1 -top-1 flex h-3.5 min-w-[14px] items-center justify-center rounded-full bg-orange-500 px-0.5 text-[9px] font-bold leading-none text-white">
          {active > 9 ? '9+' : active}
        </span>
      ) : failed > 0 ? (
        <span className="absolute -right-0.5 -top-0.5 h-2 w-2 rounded-full bg-destructive" />
      ) : null}
    </button>
  );
}
