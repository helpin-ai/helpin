import { useState, type ReactNode } from 'react';
import { Loading01Icon, Tick01Icon } from '@/lib/icons';
import { cn } from '@/lib/utils';
import { canonicalToolName } from '@/lib/toolNames';
import type { TranscriptSegment } from '@/components/agents/transcript';
import { DisclosureChevron } from '@/components/agents/transcript/DisclosureChevron';
import { formatCodingSessionElapsed } from '@/components/pm/CodingSession/codingSessionPresentation';

function groupPresentation(segments: TranscriptSegment[]): {
  label: string;
  title: string;
  meta: string;
} {
  const counts = new Map<string, number>();
  let callCount = 0;
  for (const segment of segments) {
    if (segment.kind !== 'tool') continue;
    callCount += 1;
    const name = canonicalToolName(segment.toolCall.tool_name).toLowerCase();
    counts.set(name, (counts.get(name) ?? 0) + 1);
  }
  const inventory = [...counts.entries()].map(([name, count]) => `${name}${count > 1 ? ` ×${count}` : ''}`);
  const visible = inventory.slice(0, 3);
  if (inventory.length > visible.length) visible.push(`+${inventory.length - visible.length} tools`);
  return {
    label: visible.join(' · ') || 'Tool activity',
    title: inventory.join(' · ') || 'Tool activity',
    meta: `${callCount} ${callCount === 1 ? 'call' : 'calls'}`,
  };
}

export function DockWorkingGroup({
  id,
  segments,
  active,
  completedDurationMs,
  children,
}: {
  id: string;
  segments: TranscriptSegment[];
  active: boolean;
  completedDurationMs?: number;
  children: ReactNode;
}) {
  const presentation = groupPresentation(segments);
  const completedLabel = completedDurationMs === undefined
    ? null
    : `Worked for ${formatCodingSessionElapsed(completedDurationMs)}`;
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
        <span data-working-group-label className="min-w-0 truncate font-mono text-[11px] font-medium text-foreground/80" title={presentation.title}>
          {completedLabel ?? presentation.label}
        </span>
        <span className="min-w-0 flex-1" />
        {completedLabel ? null : <span className="shrink-0 text-[10px] text-muted-foreground">{presentation.meta}</span>}
        <DisclosureChevron open={open} className="h-3.5 w-3.5" />
      </button>
      {open ? (
        <div className="space-y-2 border-t border-border/60 px-2.5 py-2.5" data-working-group-body>
          {children}
        </div>
      ) : null}
    </section>
  );
}
