import { useState, type ReactNode } from 'react';
import {
  SourceCodeIcon,
  GitBranchIcon,
  Key01Icon,
  CheckListIcon,
  MessagePreview01Icon,
  SecurityCheckIcon,
  SparklesIcon,
  TerminalIcon,
  CancelCircleIcon,
  Wrench01Icon,
  File01Icon,
  Globe02Icon,
} from '@/lib/icons';

import { Badge } from '@/components/ui/badge';
import { cn } from '@/lib/utils';
import { canonicalToolName, isToolName } from '@/lib/toolNames';
import type { CodingSessionEvent, CodingSessionLiveToolCall, RunPlanArtifact } from '@/lib/pmTypes';
import { codingSessionEventContent, formatCodingSessionRelative, prettyCodingSessionEventType } from './codingSessionUtils';
import { PublishedToolPreviewCard } from './PublishedToolPreviewCard';
import { describeToolCall } from './toolCallPresentation';

type TimelineItem =
  | { kind: 'event'; event: CodingSessionEvent }
  | { kind: 'tool'; toolCall: CodingSessionLiveToolCall };

function itemTimestamp(item: TimelineItem): string {
  if (item.kind === 'event') return item.event.timestamp;
  return item.toolCall.completed_at ?? item.toolCall.started_at ?? '';
}

export function CodingActivityRail({
  events,
  completedToolCalls,
  loading = false,
}: {
  events: CodingSessionEvent[];
  completedToolCalls: CodingSessionLiveToolCall[];
  loading?: boolean;
}) {
  const items: TimelineItem[] = [
    ...events.map((e): TimelineItem => ({ kind: 'event', event: e })),
    ...completedToolCalls.map((tc): TimelineItem => ({ kind: 'tool', toolCall: tc })),
  ].sort((a, b) => {
    const ta = itemTimestamp(a);
    const tb = itemTimestamp(b);
    return ta < tb ? -1 : ta > tb ? 1 : 0;
  });

  return (
    <section className="flex min-h-0 flex-col overflow-hidden rounded-xl border border-border bg-card shadow-sm">
      <div className="flex items-center justify-between border-b border-border px-4 py-3">
        <div className="flex items-center gap-2 text-[11px] font-medium uppercase tracking-wide text-muted-foreground">
          <TerminalIcon className="h-3.5 w-3.5" />
          Activity
        </div>
        <Badge variant="outline" className="text-[10px]">
          {items.length} events
        </Badge>
      </div>

      <div
        className="min-h-0 flex-1 overflow-auto px-4 pb-[calc(env(safe-area-inset-bottom)+4rem)] pt-4"
        data-coding-session-activity-rail-scroll
      >
        <div className="space-y-0">
          {items.map((item, index) => (
            item.kind === 'event'
              ? <EventTimelineItem key={item.event.id} event={item.event} isLast={index === items.length - 1} />
              : <ToolCallTimelineItem key={item.toolCall.tool_call_id} toolCall={item.toolCall} isLast={index === items.length - 1} />
          ))}
        </div>

        {!loading && items.length === 0 ? (
          <div className="rounded-lg border border-dashed border-border px-5 py-8 text-center text-sm text-muted-foreground">
            No activity yet. Interrupts, repo refreshes, and tool executions will stream here.
          </div>
        ) : null}
      </div>
    </section>
  );
}

function TimelineRow({
  icon,
  iconClass,
  title,
  timestamp,
  isLast,
  children,
}: {
  icon: ReactNode;
  iconClass: string;
  title: string;
  timestamp: string;
  isLast: boolean;
  children?: ReactNode;
}) {
  return (
    <div className="flex gap-3">
      <div className="flex flex-col items-center">
        <div className={cn('flex h-7 w-7 shrink-0 items-center justify-center rounded-full border', iconClass)}>
          {icon}
        </div>
        {!isLast && <div className="mt-1 h-full min-h-[1rem] w-px bg-border/50" />}
      </div>
      <div className={cn('min-w-0 flex-1', isLast ? 'pb-0' : 'pb-4')}>
        <div className="mb-1 flex items-start justify-between gap-2">
          <span className="text-xs font-medium capitalize text-foreground">{title}</span>
          {timestamp ? (
            <span className="shrink-0 text-[11px] text-muted-foreground">{formatCodingSessionRelative(timestamp)}</span>
          ) : null}
        </div>
        {children}
      </div>
    </div>
  );
}

function EventTimelineItem({ event, isLast }: { event: CodingSessionEvent; isLast: boolean }) {
  const { icon, iconClass } = eventChrome(event);
  const reviewFindings = event.type === 'review.findings.updated' ? parseReviewFindingsEventContent(event.payload.content) : null;
  const content = codingSessionEventContent(event.payload);
  const status = typeof event.payload.status === 'string' ? event.payload.status : null;
  const pauseReason = typeof event.payload.pause_reason === 'string' ? event.payload.pause_reason : null;
  const summary = typeof event.payload.summary === 'string' ? event.payload.summary : null;
  const title = typeof event.payload.title === 'string' ? event.payload.title : null;

  return (
    <TimelineRow
      icon={icon}
      iconClass={iconClass}
      title={prettyCodingSessionEventType(event.type)}
      timestamp=""
      isLast={isLast}
    >
      <div className="space-y-1 text-xs text-muted-foreground">
        {reviewFindings ? (
          <ReviewFindingsTimelineCard review={reviewFindings} />
        ) : null}
        {title ? <p className="font-medium text-foreground">{title}</p> : null}
        {summary ? <p>{summary}</p> : null}
        {content && !summary && !reviewFindings ? <p className="whitespace-pre-wrap">{content}</p> : null}
        {status ? <p>Status: <span className="font-medium capitalize text-foreground">{status}</span></p> : null}
        {pauseReason && pauseReason !== 'none' ? (
          <p>Pause reason: <span className="font-medium capitalize text-foreground">{pauseReason.replaceAll('_', ' ')}</span></p>
        ) : null}
        {(event.type.startsWith('interaction.') || event.type.startsWith('auth.') || event.type.startsWith('activity.')) && !reviewFindings ? (
          <pre className="overflow-auto whitespace-pre-wrap rounded-md border border-border bg-slate-950 px-2 py-1.5 text-[11px] leading-5 text-slate-100">
            {JSON.stringify(event.payload.content ?? event.payload, null, 2)}
          </pre>
        ) : null}
      </div>
    </TimelineRow>
  );
}

function ToolCallTimelineItem({ toolCall, isLast }: { toolCall: CodingSessionLiveToolCall; isLast: boolean }) {
  const [showReadOutput, setShowReadOutput] = useState(false);
  const isFailed = toolCall.status === 'failed';

  if (isToolName(toolCall.tool_name, 'update_plan')) {
    const plan = parsePlanForTimeline(toolCall.args_text);
    const completedCount = plan?.plan.filter((s) => s.status === 'completed').length ?? 0;
    const totalCount = plan?.plan.length ?? 0;
    return (
      <TimelineRow
        icon={<CheckListIcon className="h-3.5 w-3.5" />}
        iconClass="bg-blue-50 border-blue-200 dark:bg-blue-950/30 dark:border-blue-900/50 text-blue-600 dark:text-blue-400"
        title="Plan updated"
        timestamp=""
        isLast={isLast}
      >
        {totalCount > 0 ? (
          <p className="text-xs text-muted-foreground">{completedCount}/{totalCount} steps complete</p>
        ) : null}
      </TimelineRow>
    );
  }

  const { icon, iconClass } = toolChrome(toolCall.tool_name, isFailed);
  const argsText = toolCall.args_text.trim();
  const resultText = toolCall.result?.output_summary?.trim() || toolCall.result?.content?.trim() || '';
  const readOutputText = !isFailed && (isToolName(toolCall.tool_name, 'read_file') || isToolName(toolCall.tool_name, 'read_file_range'))
    ? toolCall.result?.content?.trim() || ''
    : '';
  const readLineCount = readOutputText ? readOutputText.split('\n').length : null;
  const readLineChip = readLineCount ? `${readLineCount} line${readLineCount === 1 ? '' : 's'}` : null;
  const publishedPreviewCard = !isFailed && argsText ? (
    <PublishedToolPreviewCard toolName={toolCall.tool_name} argsText={argsText} resultText={resultText} compact />
  ) : null;
  const presentation = describeToolCall(toolCall);
  const showSecondaryBadge = presentation.secondaryLabel.trim().toLowerCase() !== presentation.primaryLabel.trim().toLowerCase();
  const filePaths = !publishedPreviewCard && argsText ? extractFilePathsFromText(argsText) : [];
  const chips = presentation.chips.filter((chip) => !readLineChip || !/^\d+\s+lines?$/.test(chip));
  for (const filePath of filePaths) {
    if (!chips.includes(filePath)) chips.push(filePath);
  }

  return (
    <TimelineRow
      icon={icon}
      iconClass={iconClass}
      title={presentation.primaryLabel}
      timestamp=""
      isLast={isLast}
    >
      <div className="space-y-1.5 text-xs text-muted-foreground">
        <div className="flex flex-wrap items-center gap-1.5">
          {showSecondaryBadge ? (
            <Badge variant="outline" className="h-5 rounded-full px-1.5 text-[10px] font-medium text-muted-foreground">
              {presentation.secondaryLabel}
            </Badge>
          ) : null}
          {readLineChip ? (
            <ExpandableChip
              label={readLineChip}
              expanded={showReadOutput}
              onToggle={() => setShowReadOutput((prev) => !prev)}
            />
          ) : null}
          {chips.map((chip) => (
            <span key={chip} className="inline-flex items-center rounded bg-primary/10 px-1.5 py-0.5 text-[10px] font-medium text-primary">
              {chip}
            </span>
          ))}
        </div>
        {publishedPreviewCard ?? (
          <>
            {argsText ? <CollapsibleCodeBlock text={argsText} /> : null}
            {showReadOutput && readOutputText ? <CollapsibleCodeBlock text={readOutputText} /> : null}
            {resultText && !readOutputText ? (
              isFailed
                ? <CollapsibleCodeBlock text={resultText} failed />
                : <p className="text-[11px]">{resultText.length > 150 ? `${resultText.slice(0, 150)}…` : resultText}</p>
            ) : null}
          </>
        )}
        {typeof toolCall.duration_ms === 'number' ? (
          <Badge variant="outline" className={cn(
            'text-[10px]',
            isFailed ? 'border-destructive/30 text-destructive' : 'border-emerald-300 text-emerald-700 dark:border-emerald-800 dark:text-emerald-400',
          )}>
            {toolCall.duration_ms < 1000
              ? `${toolCall.duration_ms}ms`
              : `${(toolCall.duration_ms / 1000).toFixed(1)}s`}
          </Badge>
        ) : null}
      </div>
    </TimelineRow>
  );
}

function eventChrome(event: CodingSessionEvent): { icon: ReactNode; iconClass: string } {
  if (event.type.startsWith('repo.')) {
    return {
      icon: <GitBranchIcon className="h-3.5 w-3.5" />,
      iconClass: 'bg-emerald-50 border-emerald-200 dark:bg-emerald-950/20 dark:border-emerald-900/50 text-emerald-600 dark:text-emerald-400',
    };
  }
  if (event.type.startsWith('interaction.') || event.type === 'approval.requested' || event.type === 'review.findings.updated') {
    return {
      icon: <SecurityCheckIcon className="h-3.5 w-3.5" />,
      iconClass: 'bg-amber-50 border-amber-200 dark:bg-amber-950/20 dark:border-amber-900/50 text-amber-600 dark:text-amber-400',
    };
  }
  if (event.type === 'input.requested') {
    return {
      icon: <MessagePreview01Icon className="h-3.5 w-3.5" />,
      iconClass: 'bg-blue-50 border-blue-200 dark:bg-blue-950/20 dark:border-blue-900/50 text-blue-600 dark:text-blue-400',
    };
  }
  if (event.type.startsWith('auth.')) {
    return {
      icon: <Key01Icon className="h-3.5 w-3.5" />,
      iconClass: 'bg-amber-50 border-amber-200 dark:bg-amber-950/20 dark:border-amber-900/50 text-amber-600 dark:text-amber-400',
    };
  }
  return {
    icon: <SparklesIcon className="h-3.5 w-3.5" />,
    iconClass: 'bg-muted/50 border-border text-muted-foreground',
  };
}

function toolChrome(toolName: string, isFailed: boolean): { icon: ReactNode; iconClass: string } {
  if (isFailed) {
    return {
      icon: <CancelCircleIcon className="h-3.5 w-3.5" />,
      iconClass: 'bg-destructive/10 border-destructive/30 text-destructive',
    };
  }
  const name = canonicalToolName(toolName).toLowerCase();
  if (name.includes('web_search')) {
    return {
      icon: <Globe02Icon className="h-3.5 w-3.5" />,
      iconClass: 'bg-sky-50 border-sky-200 dark:bg-sky-950/20 dark:border-sky-900/50 text-sky-600 dark:text-sky-400',
    };
  }
  if (name === 'run_command' || name === 'bash' || name.includes('shell') || name.includes('exec')) {
    return {
      icon: <TerminalIcon className="h-3.5 w-3.5" />,
      iconClass: 'bg-slate-100 border-slate-300 dark:bg-slate-900 dark:border-slate-700 text-slate-600 dark:text-slate-400',
    };
  }
  if (
    name.startsWith('publish_')
    || name.startsWith('preview_')
    || name.includes('plan_doc')
    || name.includes('draft')
  ) {
    return {
      icon: <File01Icon className="h-3.5 w-3.5" />,
      iconClass: 'bg-blue-50 border-blue-200 dark:bg-blue-950/20 dark:border-blue-900/50 text-blue-600 dark:text-blue-400',
    };
  }
  if (
    name === 'apply_patch'
    || name === 'write_file'
    || name === 'str_replace_editor'
    || name.includes('file')
    || name.includes('patch')
    || name.includes('write')
    || name.includes('edit')
  ) {
    return {
      icon: <SourceCodeIcon className="h-3.5 w-3.5" />,
      iconClass: 'bg-violet-50 border-violet-200 dark:bg-violet-950/20 dark:border-violet-900/50 text-violet-600 dark:text-violet-400',
    };
  }
  return {
    icon: <Wrench01Icon className="h-3.5 w-3.5" />,
    iconClass: 'bg-muted/50 border-border text-muted-foreground',
  };
}

const RAIL_COLLAPSED_LINES = 2;

function extractFilePathsFromText(text: string): string[] {
  const matches = text.match(/(?:^|\s)((?:\/|\.\.?\/)?[\w./-]+\.(?:ts|tsx|js|jsx|go|py|css|html|json|sql|md|yaml|yml|toml|sh))\b/g);
  if (!matches) return [];
  const unique = [...new Set(matches.map((m) => m.trim()))];
  return unique.slice(0, 6);
}

function CollapsibleCodeBlock({ text, failed }: { text: string; failed?: boolean }) {
  const [expanded, setExpanded] = useState(false);
  const lines = text.split('\n');
  const isLong = lines.length > RAIL_COLLAPSED_LINES;

  return (
    <div className="relative">
      <pre className={cn(
        'overflow-auto whitespace-pre-wrap break-all rounded-md border px-2 py-1.5 font-mono text-[11px] leading-5',
        failed
          ? 'border-destructive/20 bg-destructive/5 text-destructive dark:bg-destructive/10'
          : 'border-border/60 bg-muted/50 text-foreground/80',
        !expanded && isLong && 'max-h-[52px]',
        expanded && 'max-h-48',
      )}>
        {expanded || !isLong ? text : lines.slice(0, RAIL_COLLAPSED_LINES).join('\n')}
      </pre>
      {isLong && !expanded && (
        <div className="pointer-events-none absolute inset-x-0 bottom-0 h-6 rounded-b-md bg-gradient-to-t from-muted/80 to-transparent" />
      )}
      {isLong && (
        <button
          type="button"
          className="mt-1 text-[10px] font-medium text-primary hover:underline"
          onClick={() => setExpanded((prev) => !prev)}
        >
          {expanded ? 'Show less' : `Show more (${lines.length} lines)`}
        </button>
      )}
    </div>
  );
}

function ExpandableChip({
  label,
  expanded,
  onToggle,
}: {
  label: string;
  expanded: boolean;
  onToggle: () => void;
}) {
  return (
    <button
      type="button"
      className={cn(
        'inline-flex items-center rounded px-1.5 py-0.5 text-[10px] font-medium transition-colors',
        expanded
          ? 'bg-primary text-primary-foreground'
          : 'bg-primary/10 text-primary hover:bg-primary/15',
      )}
      onClick={onToggle}
    >
      {label}
    </button>
  );
}

function parsePlanForTimeline(argsText: string): RunPlanArtifact | null {
  try {
    const parsed = JSON.parse(argsText) as unknown;
    if (!parsed || typeof parsed !== 'object' || Array.isArray(parsed)) return null;
    const obj = parsed as Record<string, unknown>;
    if (!Array.isArray(obj.plan)) return null;
    return obj as unknown as RunPlanArtifact;
  } catch {
    return null;
  }
}

interface ReviewFindingsTimelinePayload {
  phase?: string;
  title?: string;
  summary?: string;
  findings: Array<{
    id: string;
    title: string;
    body: string;
    priority?: string;
    confidence?: string;
    codeLocation?: string;
  }>;
  overallCorrectness?: string;
  overallExplanation?: string;
  overallConfidenceScore?: number;
}

function parseReviewFindingsEventContent(value: unknown): ReviewFindingsTimelinePayload | null {
  if (!value || typeof value !== 'object' || Array.isArray(value)) return null;
  const payload = value as Record<string, unknown>;
  const findings = Array.isArray(payload.findings) ? payload.findings : [];
  const normalizedFindings = findings.flatMap((rawFinding, index) => {
    if (!rawFinding || typeof rawFinding !== 'object' || Array.isArray(rawFinding)) return [];
    const finding = rawFinding as Record<string, unknown>;
    const id = typeof finding.id === 'string' && finding.id.trim() ? finding.id.trim() : `finding_${index + 1}`;
    const title = typeof finding.title === 'string' ? finding.title.trim() : '';
    if (!id || !title) return [];
    return [{
      id,
      title,
      body: typeof finding.body === 'string' ? finding.body.trim() : '',
      priority: typeof finding.priority === 'string' ? finding.priority.trim() : '',
      confidence: formatReviewConfidenceLabel(finding.confidence),
      codeLocation: typeof finding.code_location === 'string' ? finding.code_location.trim() : '',
    }];
  });
  const title = typeof payload.title === 'string' ? payload.title.trim() : '';
  const summary = typeof payload.summary === 'string' ? payload.summary.trim() : '';
  const overallCorrectness = typeof payload.overall_correctness === 'string' ? payload.overall_correctness.trim() : '';
  const overallExplanation = typeof payload.overall_explanation === 'string' ? payload.overall_explanation.trim() : '';
  const overallConfidenceScore = typeof payload.overall_confidence_score === 'number' && Number.isFinite(payload.overall_confidence_score)
    ? payload.overall_confidence_score
    : undefined;
  if (!title && !summary && !overallCorrectness && !overallExplanation && typeof overallConfidenceScore !== 'number' && normalizedFindings.length === 0) {
    return null;
  }
  return {
    phase: typeof payload.phase === 'string' ? payload.phase.trim() : '',
    title,
    summary,
    findings: normalizedFindings,
    overallCorrectness,
    overallExplanation,
    overallConfidenceScore,
  };
}

function ReviewFindingsTimelineCard({ review }: { review: ReviewFindingsTimelinePayload }) {
  return (
    <div className="rounded-md border border-border/60 bg-muted/30 p-3">
      <div className="mb-2 flex flex-wrap items-center gap-1.5">
        <span className="text-xs font-medium text-foreground">
          {review.title || 'Persisted review findings'}
        </span>
        {review.phase ? (
          <Badge variant="outline" className="h-5 rounded-full px-1.5 text-[10px] uppercase tracking-wide">
            {review.phase}
          </Badge>
        ) : null}
        {review.overallCorrectness ? (
          <Badge variant="outline" className="h-5 rounded-full px-1.5 text-[10px] uppercase tracking-wide">
            {review.overallCorrectness.replaceAll('_', ' ')}
          </Badge>
        ) : null}
        {typeof review.overallConfidenceScore === 'number' ? (
          <Badge variant="outline" className="h-5 rounded-full px-1.5 text-[10px] uppercase tracking-wide">
            {Math.round(review.overallConfidenceScore * 100)}% confidence
          </Badge>
        ) : null}
      </div>
      {review.summary ? <p className="mb-2 text-[11px] leading-5">{review.summary}</p> : null}
      {review.overallExplanation ? <p className="mb-2 text-[11px] leading-5">{review.overallExplanation}</p> : null}
      <div className="space-y-2">
        {review.findings.map((finding) => (
          <div key={finding.id} className="rounded-md border border-border/60 bg-background px-2.5 py-2">
            <div className="mb-1 flex flex-wrap items-center gap-1.5">
              <span className="text-[11px] font-medium text-foreground">{finding.title}</span>
              {finding.priority ? (
                <Badge variant="outline" className={cn('h-5 rounded-full px-1.5 text-[10px] uppercase tracking-wide', reviewPriorityBadgeClassName(finding.priority))}>
                  {finding.priority.toUpperCase()}
                </Badge>
              ) : null}
              {finding.confidence ? (
                <Badge variant="outline" className="h-5 rounded-full px-1.5 text-[10px] uppercase tracking-wide">
                  {finding.confidence}
                </Badge>
              ) : null}
            </div>
            {finding.body ? <p className="text-[11px] leading-5 text-muted-foreground">{finding.body}</p> : null}
            <div className="mt-1 flex flex-wrap items-center gap-2 text-[10px] text-muted-foreground">
              <code>{finding.id}</code>
              {finding.codeLocation ? <code>{finding.codeLocation}</code> : null}
            </div>
          </div>
        ))}
      </div>
    </div>
  );
}

function formatReviewConfidenceLabel(value: unknown) {
  if (typeof value === 'string') return value.trim();
  if (typeof value === 'number' && Number.isFinite(value)) {
    if (value >= 0 && value <= 1) return `${Math.round(value * 100)}%`;
    return String(value);
  }
  return '';
}

function reviewPriorityBadgeClassName(priority: string) {
  switch (priority.trim().toUpperCase()) {
    case 'P0':
      return 'border-red-200 bg-red-50 text-red-700 dark:border-red-900/60 dark:bg-red-950/30 dark:text-red-300';
    case 'P1':
      return 'border-orange-200 bg-orange-50 text-orange-700 dark:border-orange-900/60 dark:bg-orange-950/30 dark:text-orange-300';
    case 'P2':
      return 'border-amber-200 bg-amber-50 text-amber-700 dark:border-amber-900/60 dark:bg-amber-950/30 dark:text-amber-300';
    default:
      return '';
  }
}
