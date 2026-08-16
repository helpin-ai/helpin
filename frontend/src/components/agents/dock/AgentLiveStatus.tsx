import { useEffect, useRef, useState } from 'react';
import { Tick01Icon } from '@/lib/icons';
import { AskAgentWorkAnimation } from '@/components/agents/AskAgentWorkAnimation';
import { cn } from '@/lib/utils';
import type { AgentLiveProgress } from './agentProgress';
import { formatAgentElapsed } from './agentProgress';

export function AgentLiveStatus({ progress }: { progress: AgentLiveProgress }) {
  const [now, setNow] = useState(() => Date.now());
  const [pausedMs, setPausedMs] = useState(0);
  const pauseStartedAtRef = useRef<number | null>(null);
  const timerStartedAtRef = useRef(progress.startedAt);
  const isStarting = progress.label === 'Starting…' || progress.label === 'Waiting to start…';

  useEffect(() => {
    const currentNow = Date.now();
    if (timerStartedAtRef.current !== progress.startedAt) {
      timerStartedAtRef.current = progress.startedAt;
      pauseStartedAtRef.current = null;
      setPausedMs(0);
    }

    if (!progress.completed && progress.tone === 'waiting') {
      pauseStartedAtRef.current ??= currentNow;
      return;
    }

    if (pauseStartedAtRef.current !== null) {
      setPausedMs((current) => current + currentNow - pauseStartedAtRef.current!);
      pauseStartedAtRef.current = null;
    }

    if (!progress.startedAt || progress.completed || progress.tone !== 'working' || isStarting) {
      if (progress.completed) setNow(currentNow);
      return;
    }

    const timer = window.setInterval(() => setNow(Date.now()), 1_000);
    return () => window.clearInterval(timer);
  }, [isStarting, progress.completed, progress.startedAt, progress.tone]);

  const elapsed = (progress.tone === 'waiting' && !progress.completed) || isStarting
    ? null
    : formatAgentElapsed(progress.startedAt, now, pausedMs);
  const label = elapsed
    ? `${progress.completed ? 'Worked' : progress.label.replace(/…$/, '')} for ${elapsed}`
    : progress.label;

  return (
    <div
      className="flex min-w-0 items-center gap-2 py-0.5 text-[11px] text-muted-foreground"
      role="status"
      aria-live="polite"
      data-agent-live-status
    >
      {progress.tone === 'working' && !progress.completed ? (
        <AskAgentWorkAnimation className="h-7 w-7" />
      ) : progress.completed ? (
        <Tick01Icon className="h-3.5 w-3.5 shrink-0 text-emerald-600 dark:text-emerald-400" aria-hidden="true" />
      ) : progress.tone === 'waiting' ? (
        <span className="flex h-3.5 w-3.5 shrink-0 items-center justify-center" aria-hidden>
          <span className="agent-paused-dot-pulse h-1.5 w-1.5 rounded-full bg-amber-500" />
        </span>
      ) : null}
      <span className={cn(
        'min-w-0 flex-1 truncate',
        !progress.completed && progress.tone === 'working' && 'agent-streaming-text',
      )} title={label}>{label}</span>
    </div>
  );
}
