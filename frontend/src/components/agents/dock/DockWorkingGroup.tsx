import { useState, type ReactNode } from 'react';
import { Loading01Icon, Tick01Icon } from '@/lib/icons';
import { cn } from '@/lib/utils';
import { canonicalToolName } from '@/lib/toolNames';
import { describeToolCall } from '@/components/pm/CodingSession/toolCallPresentation';
import type { TranscriptSegment } from '@/components/agents/transcript';
import { DisclosureChevron } from '@/components/agents/transcript/DisclosureChevron';
import { formatCodingSessionElapsed } from '@/components/pm/CodingSession/codingSessionPresentation';

function groupPresentation(segments: TranscriptSegment[]): {
  label: string;
  title: string;
  meta: string;
  latestToolLabel: string | null;
  previousCallCount: number;
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
  const latestTool = [...segments].reverse().find((segment) => segment.kind === 'tool');
  return {
    label: visible.join(' · ') || 'Tool activity',
    title: inventory.join(' · ') || 'Tool activity',
    meta: `${callCount} ${callCount === 1 ? 'call' : 'calls'}`,
    latestToolLabel: latestTool?.kind === 'tool' ? describeToolCall(latestTool.toolCall).primaryLabel : null,
    previousCallCount: Math.max(0, callCount - 1),
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
  const activeLabel = active ? presentation.latestToolLabel : null;
  const label = activeLabel ?? completedLabel ?? presentation.label;
  const meta = active
    ? presentation.previousCallCount > 0
      ? `${presentation.previousCallCount} previous`
      : null
    : presentation.meta;
  const [open, setOpen] = useState(false);
  const [manuallyToggled, setManuallyToggled] = useState(false);
  const [previousActive, setPreviousActive] = useState(active);
  if (previousActive !== active) {
    setPreviousActive(active);
    if (!manuallyToggled) setOpen(false);
  }

  return (
    <section
      className="py-0.5"
      data-agent-working-group
      data-working-group-id={id}
      data-working-group-active={active ? 'true' : 'false'}
    >
      <button
        type="button"
        className="-mx-1 flex w-[calc(100%+0.5rem)] items-center gap-2 rounded-md px-1 py-1.5 text-left transition-colors hover:bg-quiet-row-hover focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-quiet-text-primary focus-visible:ring-offset-2"
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
        <span data-working-group-label className="min-w-0 truncate text-[11.5px] font-medium text-quiet-text-secondary" title={activeLabel ?? presentation.title}>
          {label}
        </span>
        <span className="min-w-0 flex-1" />
        {meta ? <span className="shrink-0 text-[11px] text-quiet-muted">{meta}</span> : null}
        <DisclosureChevron open={open} className="h-3.5 w-3.5" />
      </button>
      {open ? (
        <div className="ml-[7px] mt-1 space-y-1.5 border-l border-quiet-divider-light pb-1 pl-[17px]" data-working-group-body>
          {children}
        </div>
      ) : null}
    </section>
  );
}
