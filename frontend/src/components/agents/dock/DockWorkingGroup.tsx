import { useState, type ReactNode } from 'react';
import { Loading01Icon, Tick01Icon } from '@/lib/icons';
import { cn } from '@/lib/utils';
import { describeToolCall } from '@/components/pm/CodingSession/toolCallPresentation';
import type { TranscriptSegment } from '@/components/agents/transcript';

function groupLabel(segments: TranscriptSegment[]): string {
  for (let index = segments.length - 1; index >= 0; index -= 1) {
    const segment = segments[index];
    if (segment.kind === 'tool') return describeToolCall(segment.toolCall).primaryLabel;
    if (segment.kind === 'assistant') {
      const firstLine = segment.content.split('\n').find((line) => line.trim())?.trim();
      if (firstLine) return firstLine.length > 90 ? `${firstLine.slice(0, 87)}…` : firstLine;
    }
    if (segment.kind === 'reasoning') return segment.reasoning.status === 'streaming' ? 'Thinking…' : 'Thought';
  }
  return 'Working';
}

export function DockWorkingGroup({
  id,
  segments,
  active,
  children,
}: {
  id: string;
  segments: TranscriptSegment[];
  active: boolean;
  children: ReactNode;
}) {
  const [open, setOpen] = useState(active);
  const [manuallyToggled, setManuallyToggled] = useState(false);
  const [previousActive, setPreviousActive] = useState(active);
  if (previousActive !== active) {
    setPreviousActive(active);
    if (!manuallyToggled) setOpen(active);
  }

  return (
    <section
      className="rounded-lg border border-border/70 bg-muted/10"
      data-agent-working-group
      data-working-group-id={id}
      data-working-group-active={active ? 'true' : 'false'}
    >
      <button
        type="button"
        className="flex w-full items-center gap-2 rounded-lg px-2.5 py-2 text-left transition-colors hover:bg-muted/30"
        aria-expanded={open}
        onClick={() => {
          setManuallyToggled(true);
          setOpen((current) => !current);
        }}
      >
        <span className={cn(
          'flex h-4 w-4 shrink-0 items-center justify-center',
          active ? 'text-orange-500' : 'text-emerald-600 dark:text-emerald-400',
        )}>
          {active ? <Loading01Icon className="h-3.5 w-3.5 animate-spin" /> : <Tick01Icon className="h-3.5 w-3.5" />}
        </span>
        <span className="min-w-0 flex-1 truncate text-[11px] font-medium text-foreground/80" title={groupLabel(segments)}>
          {groupLabel(segments)}
        </span>
        {active ? <span className="shrink-0 text-[10px] text-orange-600 dark:text-orange-400">Live</span> : null}
        <span aria-hidden className="shrink-0 text-[10px] text-muted-foreground">{open ? '▾' : '▸'}</span>
      </button>
      {open ? (
        <div className="space-y-2 border-t border-border/60 px-2.5 py-2.5" data-working-group-body>
          {children}
        </div>
      ) : null}
    </section>
  );
}
