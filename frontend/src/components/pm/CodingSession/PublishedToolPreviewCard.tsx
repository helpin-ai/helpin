import { useMemo, useState } from 'react';
import { ExpandIcon, File01Icon } from '@/lib/icons';

import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from '@/components/ui/dialog';
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

  const proposedTasks = Array.isArray(record.proposed_tasks) ? record.proposed_tasks : [];
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

export function PublishedToolPreviewCard({
  toolName,
  argsText,
  compact = false,
}: {
  toolName: string;
  argsText: string;
  resultText?: string;
  compact?: boolean;
}) {
  const preview = useMemo(() => parsePreview(toolName, argsText), [toolName, argsText]);
  const [dialogOpen, setDialogOpen] = useState(false);

  if (!preview) return null;

  const summaryLines = previewSummaryLines(preview);

  const hasMarkdown = preview.format === 'markdown' && typeof preview.content === 'string';

  return (
    <>
      <button
        type="button"
        className={cn(
          'w-full rounded-xl border border-border/60 bg-card/90 p-3 text-left shadow-sm transition-colors hover:border-border hover:bg-card',
          compact && 'p-2.5',
        )}
        onClick={() => setDialogOpen(true)}
      >
        <div className="flex items-center gap-2">
          <File01Icon className="h-3.5 w-3.5 shrink-0 text-muted-foreground" />
          <p className="min-w-0 truncate text-sm font-semibold leading-5 text-foreground">{preview.title}</p>
        </div>

        {hasMarkdown ? (
          <div className="relative mt-2.5 max-h-[8rem] overflow-hidden">
            <MarkdownContent content={preview.content as string} className="text-xs leading-5 text-muted-foreground" />
            <div className="pointer-events-none absolute inset-x-0 bottom-0 h-14 bg-gradient-to-t from-card to-transparent" />
          </div>
        ) : summaryLines.length > 0 ? (
          <div className="mt-2.5 text-xs leading-5 text-muted-foreground">
            {summaryLines.map((line, index) => (
              <p key={`${line}-${index}`}>{line}</p>
            ))}
          </div>
        ) : null}

        <div className="mt-2 flex items-center gap-1 text-[11px] font-medium text-primary">
          <ExpandIcon className="h-3 w-3" />
          View full document
        </div>
      </button>

      <Dialog open={dialogOpen} onOpenChange={setDialogOpen}>
        <DialogContent className="z-[140] gap-0 overflow-hidden border-border/70 bg-background p-0 shadow-2xl sm:max-w-5xl">
          <DialogHeader className="border-b border-border/70 px-6 py-4">
            <DialogTitle className="pr-10 text-lg font-semibold leading-tight text-foreground">
              {preview.title}
            </DialogTitle>
            <DialogDescription className="sr-only">Preview content</DialogDescription>
          </DialogHeader>

          <div className="max-h-[78vh] overflow-auto">
            <div className="px-6 py-6 sm:px-10 sm:py-8">
              {preview.format === 'markdown' && typeof preview.content === 'string' ? (
                <MarkdownContent content={preview.content} className="text-[15px] leading-7 text-foreground" />
              ) : (
                <pre className="whitespace-pre-wrap break-all text-[12px] leading-6 text-foreground">
                  {JSON.stringify(preview.content, null, 2)}
                </pre>
              )}
            </div>
          </div>
        </DialogContent>
      </Dialog>
    </>
  );
}
