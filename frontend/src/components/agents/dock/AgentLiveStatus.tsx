import { useEffect, useState } from 'react';
import { cn } from '@/lib/utils';
import type { AgentLiveProgress } from './agentProgress';
import { formatAgentElapsed } from './agentProgress';

export function AgentLiveStatus({ progress }: { progress: AgentLiveProgress }) {
  const [now, setNow] = useState(() => Date.now());

  useEffect(() => {
    if (!progress.startedAt) return;
    setNow(Date.now());
    const timer = window.setInterval(() => setNow(Date.now()), 1_000);
    return () => window.clearInterval(timer);
  }, [progress.startedAt]);

  const elapsed = formatAgentElapsed(progress.startedAt, now);
  return (
    <div
      className="flex min-w-0 items-center gap-2 px-3.5 pb-0.5 pt-2 text-[11px] text-muted-foreground"
      role="status"
      aria-live="polite"
      data-agent-live-status
    >
      <span
        className={cn(
          'h-1.5 w-1.5 shrink-0 rounded-full',
          progress.tone === 'waiting' ? 'bg-amber-500' : 'animate-pulse bg-orange-500',
        )}
        aria-hidden
      />
      <span className="min-w-0 flex-1 truncate" title={progress.label}>{progress.label}</span>
      {elapsed ? <span className="shrink-0 tabular-nums">{elapsed}</span> : null}
    </div>
  );
}
