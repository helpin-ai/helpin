import { useEffect, useState } from 'react';
import { Loading01Icon } from '@/lib/icons';
import type { AgentLiveProgress } from './agentProgress';
import { formatAgentElapsed } from './agentProgress';

export function AgentLiveStatus({ progress }: { progress: AgentLiveProgress }) {
  const [now, setNow] = useState(() => Date.now());

  useEffect(() => {
    if (!progress.startedAt) return;
    const timer = window.setInterval(() => setNow(Date.now()), 1_000);
    return () => window.clearInterval(timer);
  }, [progress.startedAt]);

  const elapsed = formatAgentElapsed(progress.startedAt, now);
  return (
    <div
      className="flex min-w-0 items-center gap-1.5 py-0.5 text-[11px] text-muted-foreground"
      role="status"
      aria-live="polite"
      data-agent-live-status
    >
      {progress.tone === 'waiting' ? (
        <span className="flex h-3.5 w-3.5 shrink-0 items-center justify-center" aria-hidden>
          <span className="h-1.5 w-1.5 rounded-full bg-amber-500" />
        </span>
      ) : (
        <Loading01Icon className="h-3.5 w-3.5 shrink-0 animate-spin text-orange-500" aria-hidden="true" />
      )}
      <span className="min-w-0 flex-1 truncate" title={progress.label}>{progress.label}</span>
      {elapsed ? <span className="shrink-0 tabular-nums">{elapsed}</span> : null}
    </div>
  );
}
