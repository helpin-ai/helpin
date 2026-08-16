import { useEffect, useState } from 'react';
import { Tick01Icon } from '@/lib/icons';
import { cn } from '@/lib/utils';
import type { AgentLiveProgress } from './agentProgress';
import { formatAgentElapsed } from './agentProgress';

export function AgentLiveStatus({ progress }: { progress: AgentLiveProgress }) {
  const [now, setNow] = useState(() => Date.now());

  useEffect(() => {
    if (!progress.startedAt || progress.completed) {
      if (progress.completed) setNow(Date.now());
      return;
    }
    const timer = window.setInterval(() => setNow(Date.now()), 1_000);
    return () => window.clearInterval(timer);
  }, [progress.completed, progress.startedAt]);

  const elapsed = formatAgentElapsed(progress.startedAt, now);
  const label = elapsed
    ? `${progress.completed ? 'Worked' : progress.label.replace(/…$/, '')} for ${elapsed}`
    : progress.label;
  return (
    <div
      className="flex min-w-0 items-center gap-1.5 py-0.5 text-[11px] text-muted-foreground"
      role="status"
      aria-live="polite"
      data-agent-live-status
    >
      {progress.completed ? (
        <Tick01Icon className="h-3.5 w-3.5 shrink-0 text-emerald-600 dark:text-emerald-400" aria-hidden="true" />
      ) : progress.tone === 'waiting' ? (
        <span className="flex h-3.5 w-3.5 shrink-0 items-center justify-center" aria-hidden>
          <span className="h-1.5 w-1.5 rounded-full bg-amber-500" />
        </span>
      ) : null}
      <span className={cn(
        'min-w-0 flex-1 truncate',
        !progress.completed && progress.tone === 'working' && 'agent-streaming-text',
      )} title={label}>{label}</span>
    </div>
  );
}
