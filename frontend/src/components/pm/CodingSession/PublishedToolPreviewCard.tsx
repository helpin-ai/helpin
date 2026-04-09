import { useMemo, useState } from 'react';
import { ArrowExpandIcon, File01Icon } from '@/lib/icons';

import { Badge } from '@/components/ui/badge';
import { parseToolInvocationPublishedPreview, type PublishedPreview } from '@/components/pm/runPreviews';
import { cn } from '@/lib/utils';
import { MarkdownContent } from './MarkdownContent';

function parsePreview(toolName: string, argsText: string): PublishedPreview | null {
  let input: unknown = argsText;
  try {
    input = JSON.parse(argsText);
  } catch {
    input = argsText;
  }
  return parseToolInvocationPublishedPreview({
    tool_name: toolName,
    input,
  });
}

function stripMarkdown(value: string) {
  return value
    .replace(/```[\s\S]*?```/g, ' ')
    .replace(/`([^`]+)`/g, '$1')
    .replace(/\[([^\]]+)\]\([^)]+\)/g, '$1')
    .replace(/^#{1,6}\s+/gm, '')
    .replace(/^\s*[-*+]\s+/gm, '')
    .replace(/^\s*\d+\.\s+/gm, '')
    .replace(/[*_>~]/g, '')
    .replace(/\n{2,}/g, '\n');
}

function compactLine(value: string, limit = 140) {
  const normalized = value.replace(/\s+/g, ' ').trim();
  if (normalized.length <= limit) return normalized;
  return `${normalized.slice(0, limit - 1).trimEnd()}…`;
}

function jsonSummaryLines(content: unknown): string[] {
  if (!content || typeof content !== 'object' || Array.isArray(content)) {
    return [compactLine(JSON.stringify(content) ?? '')].filter(Boolean);
  }

  const record = content as Record<string, unknown>;
  const lines: string[] = [];

  const summary = typeof record.summary === 'string' ? compactLine(record.summary) : '';
  if (summary) lines.push(summary);

  const proposedTasks = Array.isArray(record.proposed_tasks)
    ? record.proposed_tasks
    : Array.isArray(record.proposed_stories)
      ? record.proposed_stories
      : [];
  if (proposedTasks.length > 0) lines.push(`${proposedTasks.length} proposed tasks`);

  const risks = Array.isArray(record.risks) ? record.risks.length : 0;
  if (risks > 0) lines.push(`${risks} risks called out`);

  const openQuestions = Array.isArray(record.open_questions) ? record.open_questions.length : 0;
  if (openQuestions > 0) lines.push(`${openQuestions} open questions`);

  if (lines.length > 0) return lines.slice(0, 3);

  return Object.entries(record)
    .slice(0, 3)
    .map(([key, value]) => `${key}: ${compactLine(typeof value === 'string' ? value : JSON.stringify(value) ?? '')}`)
    .filter((line) => !line.endsWith(': '));
}

function previewSummaryLines(preview: PublishedPreview): string[] {
  if (preview.format === 'markdown' && typeof preview.content === 'string') {
    return stripMarkdown(preview.content)
      .split('\n')
      .map((line) => compactLine(line))
      .filter((line) => line.length > 0)
      .slice(0, 3);
  }
  return jsonSummaryLines(preview.content);
}

function actionLabel(toolName: string) {
  return toolName.startsWith('publish_') ? 'Published preview to right pane' : 'Updated preview panel';
}

export function PublishedToolPreviewCard({
  toolName,
  argsText,
  resultText,
  compact = false,
}: {
  toolName: string;
  argsText: string;
  resultText?: string;
  compact?: boolean;
}) {
  const preview = useMemo(() => parsePreview(toolName, argsText), [toolName, argsText]);
  const [expanded, setExpanded] = useState(false);

  if (!preview) return null;

  const summaryLines = previewSummaryLines(preview);
  const subtitle = actionLabel(toolName);
  const secondaryResult = resultText && resultText !== subtitle ? compactLine(resultText, 180) : '';

  return (
    <div className={cn('rounded-lg border border-border/60 bg-card/80 p-3 shadow-sm', compact && 'p-2.5')}>
      <div className="flex items-start justify-between gap-3">
        <div className="min-w-0 space-y-1">
          <div className="flex items-center gap-1.5 text-[11px] font-medium uppercase tracking-wide text-muted-foreground">
            <File01Icon className="h-3.5 w-3.5" />
            {subtitle}
          </div>
          <div className="flex flex-wrap items-center gap-1.5">
            <p className="text-sm font-semibold text-foreground">{preview.title}</p>
            <Badge variant="outline" className="h-5 rounded-full px-1.5 text-[10px]">
              {preview.panelKey}
            </Badge>
          </div>
        </div>
      </div>

      {summaryLines.length > 0 ? (
        <div className="mt-2 space-y-1 text-xs leading-5 text-muted-foreground">
          {summaryLines.map((line, index) => (
            <p key={`${line}-${index}`}>{line}</p>
          ))}
        </div>
      ) : null}

      {secondaryResult ? (
        <p className="mt-2 text-[11px] text-muted-foreground">{secondaryResult}</p>
      ) : null}

      <div className="mt-3 flex items-center gap-3">
        <button
          type="button"
          className="inline-flex items-center gap-1 text-[11px] font-medium text-primary hover:underline"
          onClick={() => setExpanded((prev) => !prev)}
        >
          <ArrowExpandIcon className="h-3.5 w-3.5" />
          {expanded ? 'Collapse preview' : 'Expand preview'}
        </button>
      </div>

      {expanded ? (
        <div className="mt-3 max-h-[360px] overflow-auto rounded-md border border-border/60 bg-muted/30 p-3">
          {preview.format === 'markdown' && typeof preview.content === 'string' ? (
            <MarkdownContent content={preview.content} className="text-[12px] leading-5" />
          ) : (
            <pre className="whitespace-pre-wrap break-all text-[11px] leading-5 text-foreground">
              {JSON.stringify(preview.content, null, 2)}
            </pre>
          )}
        </div>
      ) : null}
    </div>
  );
}
