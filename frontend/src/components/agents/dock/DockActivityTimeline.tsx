import { useEffect, useId, useState } from 'react';
import { AskAgentWorkAnimation } from '@/components/agents/AskAgentWorkAnimation';
import { Cancel01Icon, ClipboardIcon, CodeIcon, File01Icon, Search01Icon, Tick01Icon } from '@/lib/icons';
import { cn } from '@/lib/utils';
import { canonicalToolName } from '@/lib/toolNames';
import { TranscriptSegmentView, type TranscriptSegment } from '@/components/agents/transcript';
import { DisclosureChevron } from '@/components/agents/transcript/DisclosureChevron';
import { describeToolCall } from '@/components/pm/CodingSession/toolCallPresentation';
import { formatCodingSessionElapsed } from '@/components/pm/CodingSession/codingSessionPresentation';
import type { AgentRunPauseReason, CodingSessionActor, CodingSessionTranscriptMessage } from '@/lib/pmTypes';
import { DockDecisionRow } from './DockDecisionRow';
import type { DockWorkingGroupEntry } from './dockWorkingGroups';
import styles from './DockActivityTimeline.module.css';
import { useBrowserOnline } from '@/hooks/useBrowserOnline';

function ActivityIcon({ name }: { name: string }) {
  const Icon = /search|list|find/.test(name) ? Search01Icon
    : /plan|task/.test(name) ? ClipboardIcon
    : /read|file|document/.test(name) ? File01Icon : CodeIcon;
  return <Icon className="h-4 w-4" />;
}

function ActivityRow({ segment, active }: { segment: TranscriptSegment; active: boolean }) {
  const [open, setOpen] = useState(segment.kind === 'tool' && segment.toolCall.status === 'failed');
  const detailsId = useId();
  const tool = segment.kind === 'tool' ? segment.toolCall : null;
  const failed = tool?.status === 'failed';
  const [wasFailed, setWasFailed] = useState(failed);
  if (wasFailed !== failed) {
    setWasFailed(failed);
    if (failed) setOpen(true);
  }
  const running = active && tool?.status === 'running';
  const label = tool ? describeToolCall(tool).primaryLabel : segment.kind === 'assistant' ? segment.content : 'Reasoning';
  const narration = segment.kind === 'assistant';
  const longNarration = narration && (label.length > 160 || label.includes('\n'));
  if (segment.kind !== 'tool' && segment.kind !== 'assistant' && segment.kind !== 'reasoning') return null;
  return (
    <li className={cn(styles.row, narration && styles.narration)} data-state={failed ? 'failed' : running ? 'running' : 'completed'}>
      <span className={styles.marker} aria-hidden="true">
        {narration ? <span className={styles.dot} /> : failed ? <Cancel01Icon className="h-4 w-4" /> : <ActivityIcon name={tool ? canonicalToolName(tool.tool_name) : 'plan'} />}
      </span>
      <div className={styles.step}>
        {narration ? (
          <>
            {!open && <p className={cn(styles.narrativeText, longNarration && styles.preview)}>{label}</p>}
            {longNarration && <button type="button" className={styles.expand} aria-expanded={open} aria-controls={detailsId} onClick={() => setOpen(!open)}>{open ? 'Hide update' : 'Read update'}</button>}
          </>
        ) : (
          <button type="button" className={styles.stepButton} aria-expanded={open} aria-controls={detailsId} onClick={() => setOpen(!open)}>
            <span className="sr-only">{failed ? 'Failed: ' : running ? 'In progress: ' : tool ? 'Completed: ' : ''}</span>
            <span className={cn(running && styles.shimmer)}>{label}</span>
            <DisclosureChevron open={open} className="mt-1 h-3 w-3 shrink-0" />
          </button>
        )}
        {open && <div id={detailsId} className={styles.details}>
          {tool ? <div className="space-y-2">
            {tool.result?.error && <p className="whitespace-pre-wrap text-destructive">{tool.result.error}</p>}
            {tool.args_text.trim() && <div><span className="text-[11px] font-medium">Input</span><pre className="mt-1 whitespace-pre-wrap break-words text-[11px]">{tool.args_text}</pre></div>}
            {(tool.result?.content || tool.result?.output_summary) && <div><span className="text-[11px] font-medium">Result</span><pre className="mt-1 whitespace-pre-wrap break-words text-[11px]">{tool.result.content || tool.result.output_summary}</pre></div>}
            {!tool.args_text.trim() && !tool.result && <p>No additional details.</p>}
          </div> : <TranscriptSegmentView segment={segment} options={{ expandable: true, showReasoningDetails: true, collapseLongAssistantContent: false, assistantPresentation: 'progress' }} />}
        </div>}
      </div>
    </li>
  );
}

type ResolveActor = (message: CodingSessionTranscriptMessage) => CodingSessionActor | null;

export function DockActivityTimeline({ group, runStatus, pauseReason, resolveActor }: { group: DockWorkingGroupEntry; runStatus?: string; pauseReason?: AgentRunPauseReason; resolveActor?: ResolveActor }) {
  const online = useBrowserOnline();
  const offline = group.active && !online;
  const failed = group.segments.some(segment => segment.kind === 'tool' && segment.toolCall.status === 'failed');
  const [expanded, setExpanded] = useState(group.active || failed);
  const [wasActive, setWasActive] = useState(group.active);
  const [manuallyToggled, setManuallyToggled] = useState(false);
  const [contentMounted, setContentMounted] = useState(expanded);
  const [now, setNow] = useState(Date.now);
  const detailsId = useId();
  // Streaming updates keep the user's disclosure choice. A settled run folds
  // into the same compact completion summary used by Usermaven.
  if (wasActive !== group.active) {
    setWasActive(group.active);
    if (failed || !manuallyToggled) {
      if (group.active || failed) setExpanded(true);
      else if (group.completed) setExpanded(false);
    }
  }
  const animateDisclosure = window.matchMedia?.('(prefers-reduced-motion: reduce)').matches === false;
  if (expanded && !contentMounted) setContentMounted(true);
  if (!expanded && contentMounted && !animateDisclosure) setContentMounted(false);
  useEffect(() => {
    if (expanded || !contentMounted) return;
    const timer = window.setTimeout(() => setContentMounted(false), 220);
    return () => window.clearTimeout(timer);
  }, [contentMounted, expanded]);
  useEffect(() => {
    if (!group.active) return;
    const timer = window.setInterval(() => setNow(Date.now()), 1000);
    return () => window.clearInterval(timer);
  }, [group.active]);
  const latestTool = [...group.segments].reverse().find(segment => segment.kind === 'tool' && segment.toolCall.status === 'running');
  const toolStarts = group.segments.flatMap(segment => segment.kind === 'tool' && segment.toolCall.started_at ? [Date.parse(segment.toolCall.started_at)] : []).filter(Number.isFinite);
  const startedAt = group.startedAt ?? (toolStarts.length ? Math.min(...toolStarts) : undefined);
  const duration = group.durationMs ?? (group.active && startedAt !== undefined ? Math.max(0, now - startedAt) : undefined);
  const label = offline ? 'Offline — live updates paused' : failed ? 'Some steps failed'
    : group.active ? (latestTool?.kind === 'tool' ? describeToolCall(latestTool.toolCall).primaryLabel : 'Working…')
    : group.completed || group.durationMs !== undefined ? 'Work completed'
    : runStatus === 'cancelled' ? 'Stopped'
    : runStatus === 'failed' ? 'Couldn’t finish'
    : runStatus === 'paused' && pauseReason === 'human_input' ? 'Needs your input'
    : runStatus === 'paused' && pauseReason === 'human_approval' ? 'Waiting for approval'
    : runStatus === 'paused' && pauseReason === 'authentication' ? 'Waiting for sign-in'
    : 'Activity';
  const summary = (
    <button type="button" className={styles.summary} aria-expanded={expanded} aria-controls={detailsId} onClick={() => { setManuallyToggled(true); setExpanded(!expanded); }}>
      <span className={cn(styles.summaryIcon, failed ? 'text-destructive' : 'text-muted-foreground')} aria-hidden="true">
        {offline ? <span className="h-1.5 w-1.5 rounded-full bg-amber-500" /> : group.active ? <AskAgentWorkAnimation /> : failed || (!group.completed && (runStatus === 'failed' || runStatus === 'cancelled')) ? <Cancel01Icon className="h-4 w-4" /> : <Tick01Icon className="h-3.5 w-3.5" />}
      </span>
      <span className={styles.label} role={group.active ? 'status' : undefined}>{label}</span>
      {!offline && duration !== undefined && duration > 0 && <span className={styles.elapsed}>{formatCodingSessionElapsed(duration)}</span>}
      <DisclosureChevron open={expanded} className="h-3 w-3 shrink-0 opacity-65" />
    </button>
  );
  return (
    <section className={styles.container} aria-label="Agent activity" data-dock-activity-timeline data-running={group.active}>
      {!group.active && summary}
      {contentMounted && <div id={detailsId} className={styles.disclosure} data-open={expanded} aria-hidden={!expanded} inert={!expanded}>
        <div className={styles.disclosureInner}>
          <div className={styles.history}><DockActivitySteps segments={group.segments} active={group.active} resolveActor={resolveActor} /></div>
        </div>
      </div>}
      {group.active && summary}
    </section>
  );
}

export function DockActivitySteps({ segments, active = false, resolveActor }: { segments: TranscriptSegment[]; active?: boolean; resolveActor?: ResolveActor }) {
  return <ol className={styles.list} aria-label="Activity steps">
    {segments.filter(segment => segment.kind !== 'assistant' || !segment.final).map(segment => segment.kind === 'review_decision'
      ? <li key={segment.id} className="relative"><DockDecisionRow message={segment.message} actor={resolveActor?.(segment.message) ?? null} timeline /></li>
      : <ActivityRow key={segment.kind === 'tool' ? segment.toolCall.tool_call_id : segment.kind === 'assistant' ? segment.messageId ?? segment.id.replace(/^live:/, '') : segment.kind === 'reasoning' ? segment.reasoning.message_id : segment.id} segment={segment} active={active} />)}
  </ol>;
}
