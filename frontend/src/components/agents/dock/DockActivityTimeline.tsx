import { useEffect, useId, useLayoutEffect, useRef, useState } from 'react';
import { AskAgentWorkAnimation } from '@/components/agents/AskAgentWorkAnimation';
import { Cancel01Icon, ClipboardIcon, CodeIcon, File01Icon, Search01Icon, Tick01Icon } from '@/lib/icons';
import { cn } from '@/lib/utils';
import { canonicalToolName } from '@/lib/toolNames';
import { TranscriptSegmentView, type TranscriptSegment } from '@/components/agents/transcript';
import { DisclosureChevron } from '@/components/agents/transcript/DisclosureChevron';
import { MarkdownContent } from '@/components/pm/CodingSession/MarkdownContent';
import { describeToolCall } from '@/components/pm/CodingSession/toolCallPresentation';
import { formatCodingSessionElapsed } from '@/components/pm/CodingSession/codingSessionPresentation';
import type { AgentRunPauseReason, CodingSessionActor, CodingSessionLiveToolCall, CodingSessionTranscriptMessage } from '@/lib/pmTypes';
import { DockDecisionRow } from './DockDecisionRow';
import type { DockWorkingGroupEntry } from './dockWorkingGroups';
import type { AgentLiveProgress } from './agentProgress';
import styles from './DockActivityTimeline.module.css';
import { useBrowserOnline } from '@/hooks/useBrowserOnline';

function ActivityIcon({ name }: { name: string }) {
  const Icon = /search|list|find/.test(name) ? Search01Icon
    : /plan|task/.test(name) ? ClipboardIcon
    : /read|file|document/.test(name) ? File01Icon : CodeIcon;
  return <Icon className="h-4 w-4" />;
}

function ActivityRow({ segment, active, toolCalls = [] }: { segment: TranscriptSegment; active: boolean; toolCalls?: CodingSessionLiveToolCall[] }) {
  const failedCount = toolCalls.filter(tool => tool.status === 'failed').length;
  const failed = failedCount > 0;
  const [open, setOpen] = useState(failed);
  const detailsId = useId();
  const tool = segment.kind === 'tool' ? segment.toolCall : null;
  const [knownFailures, setKnownFailures] = useState(failedCount);
  if (knownFailures !== failedCount) {
    setKnownFailures(failedCount);
    if (failedCount > knownFailures) setOpen(true);
  }
  const running = active && toolCalls.some(tool => tool.status === 'running');
  const repeated = toolCalls.length > 1;
  const presentation = tool ? describeToolCall(tool) : null;
  // A group may search different queries or read different documents. Its
  // heading describes the action, while each call retains its own details.
  const label = presentation ? repeated ? presentation.secondaryLabel : presentation.primaryLabel
    : segment.kind === 'assistant' ? segment.content : 'Reasoning';
  const narration = segment.kind === 'assistant';
  const previewRef = useRef<HTMLDivElement>(null);
  const [clipped, setClipped] = useState(false);
  useLayoutEffect(() => {
    const preview = previewRef.current;
    if (!preview || open) return;
    // Measure the actual two-line preview, including font and dock-width changes.
    const measure = () => setClipped(preview.scrollHeight > preview.clientHeight + 1);
    measure();
    const observer = typeof ResizeObserver === 'undefined' ? null : new ResizeObserver(measure);
    observer?.observe(preview);
    return () => observer?.disconnect();
  }, [label, open]);
  if (segment.kind !== 'tool' && segment.kind !== 'assistant' && segment.kind !== 'reasoning') return null;
  return (
    <li className={cn(styles.row, narration && styles.narration)} data-state={failed ? 'failed' : running ? 'running' : 'completed'}>
      <span className={styles.marker} aria-hidden="true">
        {narration ? <span className={styles.dot} /> : failed ? <Cancel01Icon className="h-4 w-4" /> : <ActivityIcon name={tool ? canonicalToolName(tool.tool_name) : 'plan'} />}
      </span>
      <div className={styles.step}>
        {narration ? (
          <>
            <div id={detailsId} ref={previewRef} className={cn(styles.narrativeText, !open && styles.preview)}>
              <MarkdownContent content={label} className="text-[13px] leading-[1.6] text-muted-foreground" />
            </div>
            {(clipped || open) && <button type="button" className={styles.expand} aria-expanded={open} aria-controls={detailsId} onClick={() => setOpen(!open)}>{open ? 'Show less' : 'Show more'}</button>}
          </>
        ) : (
          <button type="button" className={styles.stepButton} aria-expanded={open} aria-controls={detailsId} onClick={() => setOpen(!open)}>
            <span className="sr-only">{failed ? 'Failed: ' : running ? 'In progress: ' : tool ? 'Completed: ' : ''}</span>
            <span className={cn(running && styles.shimmer)}>{label}</span>
            {repeated && <span className="shrink-0 tabular-nums">×{toolCalls.length}</span>}
            {repeated && failed && <span className="shrink-0 text-destructive">{failedCount} failed</span>}
            <DisclosureChevron open={open} className="mt-1 h-3 w-3 shrink-0" />
          </button>
        )}
        {open && !narration && <div id={detailsId} className={styles.details}>
          {tool ? <div className="space-y-3">
            {toolCalls.map((call, index) => <div key={call.tool_call_id} className={cn("space-y-2", repeated && index > 0 && "border-t border-border/60 pt-3")}>
              {repeated && <p className={cn("text-[11px] font-medium", call.status === 'failed' && "text-destructive")}>Call {index + 1} · {call.status === 'failed' ? 'Failed' : call.status === 'running' ? active ? 'In progress' : 'Interrupted' : 'Completed'}</p>}
              {call.result?.error && <p className="whitespace-pre-wrap text-destructive">{call.result.error}</p>}
              {call.args_text.trim() && <div><span className="text-[11px] font-medium">Input</span><pre className="mt-1 whitespace-pre-wrap break-words text-[11px]">{call.args_text}</pre></div>}
              {(call.result?.content || call.result?.output_summary) && <div><span className="text-[11px] font-medium">Result</span><pre className="mt-1 whitespace-pre-wrap break-words text-[11px]">{call.result.content || call.result.output_summary}</pre></div>}
              {!call.args_text.trim() && !call.result && <p>No additional details.</p>}
            </div>)}
          </div> : <TranscriptSegmentView segment={segment} options={{ expandable: true, showReasoningDetails: true, collapseLongAssistantContent: false, assistantPresentation: 'progress' }} />}
        </div>}
      </div>
    </li>
  );
}

type ResolveActor = (message: CodingSessionTranscriptMessage) => CodingSessionActor | null;

export function DockActivityTimeline({ group, runStatus, pauseReason, resolveActor, progress }: { progress?: AgentLiveProgress | null; group: DockWorkingGroupEntry; runStatus?: string; pauseReason?: AgentRunPauseReason; resolveActor?: ResolveActor }) {
  const delegated = group.active && progress?.delegated ? progress : null;
  const waiting = delegated?.tone === 'waiting';
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
  const toolStarts = group.segments.flatMap(segment => segment.kind === 'tool' && segment.toolCall.started_at ? [Date.parse(segment.toolCall.started_at)] : []).filter(Number.isFinite);
  const startedAt = group.startedAt ?? (toolStarts.length ? Math.min(...toolStarts) : undefined);
  const duration = group.durationMs ?? (group.active && startedAt !== undefined ? Math.max(0, now - startedAt) : undefined);
  const label = offline ? 'Offline — live updates paused' : delegated ? delegated.label : failed ? 'Some steps failed'
    : group.active ? 'Working…'
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
        {offline || waiting ? <span className="h-1.5 w-1.5 rounded-full bg-amber-500" /> : group.active ? <AskAgentWorkAnimation /> : failed || (!group.completed && (runStatus === 'failed' || runStatus === 'cancelled')) ? <Cancel01Icon className="h-4 w-4" /> : <Tick01Icon className="h-3.5 w-3.5" />}
      </span>
      <span className={styles.label} role={group.active ? 'status' : undefined}>{label}</span>
      {!offline && !waiting && duration !== undefined && duration > 0 && <span className={styles.elapsed}>{formatCodingSessionElapsed(duration)}</span>}
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
  const steps: { segment: TranscriptSegment; toolCalls?: CodingSessionLiveToolCall[] }[] = [];
  for (const segment of segments) {
    const previous = steps.at(-1);
    if (segment.kind === 'tool') {
      const name = canonicalToolName(segment.toolCall.tool_name);
      if (name && previous?.segment.kind === 'tool' && canonicalToolName(previous.segment.toolCall.tool_name) === name) {
        previous.toolCalls!.push(segment.toolCall);
      } else {
        steps.push({ segment, toolCalls: [segment.toolCall] });
      }
    } else {
      // Keep even invisible boundaries while grouping, so a final answer or
      // new user request cannot join two otherwise identical actions.
      steps.push({ segment });
    }
  }
  return <ol className={styles.list} aria-label="Activity steps">
    {steps.filter(({ segment }) => segment.kind !== 'assistant' || !segment.final).map(({ segment, toolCalls }) => segment.kind === 'review_decision'
      ? <li key={segment.id} className="relative"><DockDecisionRow message={segment.message} actor={resolveActor?.(segment.message) ?? null} timeline /></li>
      : <ActivityRow key={segment.kind === 'tool' ? segment.toolCall.tool_call_id : segment.kind === 'assistant' ? segment.messageId ?? segment.id.replace(/^live:/, '') : segment.kind === 'reasoning' ? segment.reasoning.message_id : segment.id} segment={segment} toolCalls={toolCalls} active={active} />)}
  </ol>;
}
