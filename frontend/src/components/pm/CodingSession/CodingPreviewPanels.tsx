import { useEffect, useMemo, useState } from 'react';
import { Dialog as DialogPrimitive } from 'radix-ui';
import { Cancel01Icon } from '@hugeicons/core-free-icons';
import { HugeiconsIcon } from '@hugeicons/react';
import { ExpandIcon, File01Icon, SparklesIcon } from '@/lib/icons';

import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { DialogDescription, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import type { PublishedPreview } from '@/components/pm/runPreviews';
import type { CodingSessionInteraction } from '@/lib/pmTypes';
import { cn } from '@/lib/utils';
import { MarkdownContent } from './MarkdownContent';
import { formatCodingSessionRelative, type CodingSessionPreviewApprovalState } from './codingSessionUtils';
import { normalizeCodingSessionPreviewPanelKey } from './previewPanelKeys';

interface TaskPlanTaskPreview {
  ref?: string;
  title: string;
  type?: string;
  description?: string;
  acceptanceCriteria: string[];
  dependencyRefs: string[];
  filesToModify: string[];
}

interface TaskPlanPreviewModel {
  summary?: string;
  proposedTasks: TaskPlanTaskPreview[];
  risks: string[];
  openQuestions: string[];
}

interface DocsChangeSource {
  title?: string;
  url?: string;
}

interface DocsChangePreviewModel {
  scope: 'document' | 'block';
  summary?: string;
  contentMarkdown: string;
  blockId?: string;
  revision?: number;
  sources: DocsChangeSource[];
}

interface AttachedApprovalRequest {
  interaction: CodingSessionInteraction;
  title?: string;
  summary?: string;
  phase?: string;
  previewPanelKey?: string;
}

function asRecord(value: unknown): Record<string, unknown> | null {
  if (!value || typeof value !== 'object' || Array.isArray(value)) return null;
  return value as Record<string, unknown>;
}

function asString(value: unknown): string {
  return typeof value === 'string' ? value : '';
}

function asStringArray(value: unknown): string[] {
  if (!Array.isArray(value)) return [];
  return value.map((entry) => asString(entry).trim()).filter((entry) => entry.length > 0);
}

function toTitleCase(value: string) {
  return value
    .split(/[_\s-]+/)
    .filter(Boolean)
    .map((part) => part.charAt(0).toUpperCase() + part.slice(1))
    .join(' ');
}

function parseTaskPlanPreviewModel(preview: PublishedPreview | undefined): TaskPlanPreviewModel | null {
  if (!preview || preview.format !== 'json') return null;
  const record = asRecord(preview.content);
  const proposedTasks = Array.isArray(record?.proposed_tasks)
    ? record.proposed_tasks
    : Array.isArray(record?.proposed_stories)
      ? record.proposed_stories
      : [];
  if (!record || proposedTasks.length === 0) return null;

  const normalizedTasks: TaskPlanTaskPreview[] = proposedTasks.flatMap((entry) => {
    const task = asRecord(entry);
    if (!task) return [];

    const implementationBrief = asRecord(task.implementation_brief);
    const filesToModify = Array.isArray(implementationBrief?.files_to_modify)
      ? implementationBrief.files_to_modify
        .map((fileEntry) => {
          if (typeof fileEntry === 'string') {
            return fileEntry.trim();
          }
          return asString(asRecord(fileEntry)?.path).trim();
        })
        .filter((path) => path.length > 0)
      : [];

    const title = asString(task.name).trim() || asString(task.title).trim();
    if (!title) return [];

    return [{
      ref: asString(task.ref).trim() || undefined,
      title,
      type: asString(task.task_type).trim()
        || asString(task.story_type).trim()
        || asString(task.type).trim()
        || asString(task.slice_type).trim()
        || undefined,
      description: asString(task.description).trim() || undefined,
      acceptanceCriteria: asStringArray(task.acceptance_criteria).concat(asStringArray(task.acceptanceCriteria)),
      dependencyRefs: asStringArray(task.dependency_refs).concat(asStringArray(task.dependencyRefs)),
      filesToModify,
    } satisfies TaskPlanTaskPreview];
  });

  if (normalizedTasks.length === 0) return null;

  return {
    summary: asString(record.summary).trim() || undefined,
    proposedTasks: normalizedTasks,
    risks: asStringArray(record.risks),
    openQuestions: asStringArray(record.open_questions),
  };
}

function parseDocsChangePreviewModel(preview: PublishedPreview | undefined): DocsChangePreviewModel | null {
  if (!preview || preview.panelKey !== 'docs_change' || preview.format !== 'json') return null;
  const record = asRecord(preview.content);
  if (!record) return null;
  const scope = asString(record.scope).trim().toLowerCase();
  if (scope !== 'document' && scope !== 'block') return null;
  const contentMarkdown = asString(record.content_markdown).trim();
  if (!contentMarkdown) return null;
  const sources = Array.isArray(record.sources)
    ? record.sources.flatMap((entry) => {
      const source = asRecord(entry);
      if (!source) return [];
      const title = asString(source.title).trim() || asString(source.name).trim() || undefined;
      const url = asString(source.url).trim() || asString(source.href).trim() || undefined;
      if (!title && !url) return [];
      return [{ title, url } satisfies DocsChangeSource];
    })
    : [];

  return {
    scope,
    summary: asString(record.summary).trim() || undefined,
    contentMarkdown,
    blockId: asString(record.block_id).trim() || undefined,
    revision: typeof record.revision === 'number' && Number.isFinite(record.revision) ? record.revision : undefined,
    sources,
  };
}

function parseAttachedApprovalRequest(interaction: CodingSessionInteraction | null | undefined): AttachedApprovalRequest | null {
  if (!interaction || interaction.interaction_kind !== 'approval_request') return null;
  const payload = asRecord(interaction.request_payload);
  const previewPanelKey = normalizeCodingSessionPreviewPanelKey(asString(payload?.preview_panel_key));
  if (!previewPanelKey) return null;
  const phase = asString(payload?.phase).trim().toLowerCase() || undefined;
  const title = asString(payload?.title).trim() || interaction.title || undefined;
  const summary = asString(payload?.summary).trim() || interaction.summary || undefined;
  return {
    interaction,
    title,
    summary,
    phase,
    previewPanelKey,
  };
}

function previewApprovalStateFromAttachedApproval(
  approval: AttachedApprovalRequest | null,
): CodingSessionPreviewApprovalState | null {
  if (!approval) return null;
  if (!approval.previewPanelKey) return null;
  return {
    interaction: approval.interaction,
    status: 'pending',
    title: approval.title,
    summary: approval.summary,
    phase: approval.phase,
    previewPanelKey: approval.previewPanelKey,
  };
}

function DocsChangePanel({
  title,
  preview,
  attachedApproval,
  resolverNamesByUserId,
  onReviewApproval,
  openPreviewPanelKey,
  openPreviewRequestId,
}: {
  title: string;
  preview: DocsChangePreviewModel;
  attachedApproval?: CodingSessionPreviewApprovalState | null;
  resolverNamesByUserId?: Map<string, string>;
  onReviewApproval?: (interactionId: string) => void;
  openPreviewPanelKey?: string | null;
  openPreviewRequestId?: number;
}) {
  const [dialogOpen, setDialogOpen] = useState(false);
  const scopeLabel = preview.scope === 'block' ? 'Block change' : 'Document change';

  useEffect(() => {
    if (openPreviewPanelKey === 'docs_change') {
      setDialogOpen(true);
    }
  }, [openPreviewPanelKey, openPreviewRequestId]);

  const proposalContent = (
    <div className="space-y-3">
      <div className="flex flex-wrap items-center gap-2">
        <Badge variant="outline" className="border-orange-500/25 bg-orange-500/10 text-[10px] uppercase tracking-wide text-orange-700 dark:text-orange-300">
          {scopeLabel}
        </Badge>
        {preview.revision ? (
          <span className="text-[11px] text-muted-foreground">Revision {preview.revision}</span>
        ) : null}
      </div>
      {preview.summary ? (
        <p className="text-sm leading-6 text-muted-foreground">{preview.summary}</p>
      ) : null}
      <div className="max-h-[320px] overflow-auto rounded-md bg-muted/40 p-3">
        <MarkdownContent content={preview.contentMarkdown} className="text-[12px] leading-5" />
      </div>
      {preview.sources.length ? (
        <div className="rounded-lg border border-border/60 bg-background/60 p-3">
          <p className="text-[11px] font-semibold uppercase tracking-wide text-muted-foreground">Sources</p>
          <ul className="mt-2 space-y-1 text-xs leading-5">
            {preview.sources.map((source, index) => (
              <li key={`${source.url ?? source.title}-${index}`} className="truncate text-muted-foreground">
                {source.url ? (
                  <a href={source.url} target="_blank" rel="noreferrer" className="text-primary hover:underline">
                    {source.title || source.url}
                  </a>
                ) : (
                  source.title
                )}
              </li>
            ))}
          </ul>
        </div>
      ) : null}
      {attachedApproval ? (
        <PreviewApprovalReference
          approval={attachedApproval}
          resolverNamesByUserId={resolverNamesByUserId}
          onReview={onReviewApproval}
          onOpenDocument={() => setDialogOpen(true)}
        />
      ) : null}
    </div>
  );

  return (
    <>
      <div
        data-preview-panel-key="docs_change"
        className="rounded-md border border-orange-500/20 bg-card/80 p-3 transition-shadow data-[preview-flash=true]:ring-2 data-[preview-flash=true]:ring-orange-500/40"
      >
        {attachedApproval ? (
          <PreviewApprovalReference
            approval={attachedApproval}
            resolverNamesByUserId={resolverNamesByUserId}
            onReview={onReviewApproval}
            onOpenDocument={() => setDialogOpen(true)}
          />
        ) : (
          <>
            <div className="mb-2 flex items-center justify-between gap-2">
              <div className="flex min-w-0 items-center gap-2">
                <File01Icon className="h-4 w-4 shrink-0 text-orange-600 dark:text-orange-300" />
                <p className="truncate text-xs font-semibold uppercase tracking-wide text-muted-foreground">
                  {title || 'Docs Change Proposal'}
                </p>
              </div>
              <ExpandPreviewIconButton label={`Open ${title || 'Docs Change Proposal'}`} onClick={() => setDialogOpen(true)} />
            </div>
            {proposalContent}
          </>
        )}
      </div>
      <PreviewExpandDialog open={dialogOpen} onOpenChange={setDialogOpen} title={title || 'Docs Change Proposal'}>
        {proposalContent}
      </PreviewExpandDialog>
    </>
  );
}

function PreviewExpandDialog({
  open,
  onOpenChange,
  title,
  children,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  title: string;
  children: React.ReactNode;
}) {
  // Rendered with explicit z-indexes above the parent CodingSessionDrawer's
  // Sheet (z-70). The shadcn DialogContent wraps content in a z-50 div that
  // creates a stacking context, which would trap any inner override below the
  // Sheet — so we use Radix primitives directly here.
  return (
    <DialogPrimitive.Root open={open} onOpenChange={onOpenChange}>
      <DialogPrimitive.Portal>
        <DialogPrimitive.Overlay
          className="fixed inset-0 z-[140] bg-black/40 supports-backdrop-filter:backdrop-blur-sm data-open:animate-in data-open:fade-in-0 data-closed:animate-out data-closed:fade-out-0"
        />
        <div className="pointer-events-none fixed inset-0 z-[150] grid place-items-center overflow-y-auto p-4">
          <DialogPrimitive.Content
            className="pointer-events-auto relative z-[150] grid w-full max-w-[calc(100%-2rem)] gap-0 overflow-hidden rounded-xl border border-border/70 bg-background p-0 text-sm text-popover-foreground shadow-2xl outline-none sm:max-w-5xl data-open:animate-in data-open:fade-in-0 data-open:zoom-in-95 data-closed:animate-out data-closed:fade-out-0 data-closed:zoom-out-95"
          >
            <DialogHeader className="border-b border-border/70 px-6 py-4">
              <DialogTitle className="pr-10 text-lg font-semibold leading-tight text-foreground">
                {title}
              </DialogTitle>
              <DialogDescription className="sr-only">Preview content</DialogDescription>
            </DialogHeader>
            <div className="max-h-[78vh] overflow-auto">
              <div className="px-6 py-6 sm:px-10 sm:py-8">
                {children}
              </div>
            </div>
            <DialogPrimitive.Close asChild>
              <Button variant="ghost" className="absolute top-4 right-4 bg-secondary" size="icon-sm">
                <HugeiconsIcon icon={Cancel01Icon} strokeWidth={2} />
                <span className="sr-only">Close</span>
              </Button>
            </DialogPrimitive.Close>
          </DialogPrimitive.Content>
        </div>
      </DialogPrimitive.Portal>
    </DialogPrimitive.Root>
  );
}

function ExpandPreviewButton({ onClick }: { onClick: () => void }) {
  return (
    <button
      type="button"
      className="mt-2 inline-flex items-center gap-1 text-[11px] font-medium text-primary hover:underline"
      onClick={onClick}
    >
      <ExpandIcon className="h-3 w-3" />
      Expand preview
    </button>
  );
}

function ExpandPreviewIconButton({
  label,
  onClick,
}: {
  label: string;
  onClick: () => void;
}) {
  return (
    <Button
      type="button"
      variant="ghost"
      size="icon"
      className="h-7 w-7 shrink-0 text-muted-foreground hover:text-foreground"
      onClick={onClick}
      aria-label={label}
      title={label}
    >
      <ExpandIcon className="h-3.5 w-3.5" />
    </Button>
  );
}

function PreviewApprovalReference({
  approval,
  resolverNamesByUserId,
  onReview,
  onOpenDocument,
}: {
  approval: CodingSessionPreviewApprovalState;
  resolverNamesByUserId?: Map<string, string>;
  onReview?: (interactionId: string) => void;
  onOpenDocument?: () => void;
}) {
  const isPending = approval.status === 'pending';
  const isApproved = approval.status === 'approved';
  const resolverName = approval.resolvedBy
    ? resolverNamesByUserId?.get(approval.resolvedBy) ?? 'Someone'
    : 'Someone';
  const statusLabel = isPending
    ? 'Waiting for approval'
    : isApproved
      ? 'Approved'
      : 'Changes requested';
  const resolutionVerb = isApproved ? 'approved' : 'requested changes';
  const resolutionText = isPending
    ? null
    : `${resolverName} ${resolutionVerb}${approval.resolvedAt ? ` · ${formatCodingSessionRelative(approval.resolvedAt)}` : ''}`;
  const badgeClassName = isPending
    ? 'border-amber-500/25 bg-background text-amber-700 dark:text-amber-300'
    : isApproved
      ? 'border-emerald-500/25 bg-emerald-50 text-emerald-700 dark:bg-emerald-950/30 dark:text-emerald-300'
      : 'border-blue-500/25 bg-blue-50 text-blue-700 dark:bg-blue-950/30 dark:text-blue-300';

  return (
    <div className={cn(
      'mt-3 rounded-lg border p-3',
      isPending
        ? 'border-amber-500/25 bg-amber-500/[0.06]'
        : 'border-border/70 bg-muted/25',
    )}>
      <div className="flex flex-wrap items-center gap-x-2 gap-y-1">
        <p className="min-w-0 text-sm font-semibold text-foreground">{approval.title ?? 'Approval required'}</p>
        <Badge variant="outline" className={cn('text-[10px] uppercase tracking-wide', badgeClassName)}>
          {statusLabel}
        </Badge>
      </div>
      {resolutionText ? (
        <p className="mt-1 text-xs leading-5 text-muted-foreground">{resolutionText}</p>
      ) : null}
      {approval.summary ? (
        <p className="mt-1 text-sm leading-6 text-muted-foreground">{approval.summary}</p>
      ) : null}
      {approval.note ? (
        <p className="mt-2 rounded-md bg-background/70 px-2.5 py-2 text-xs leading-5 text-muted-foreground">
          {approval.note}
        </p>
      ) : null}
      <div className="mt-2 flex flex-wrap items-center gap-3">
        {isPending && onReview ? (
          <Button
            variant="ghost"
            size="sm"
            className="h-7 rounded-md bg-primary/10 px-2.5 text-xs font-medium text-primary hover:bg-primary/15 hover:text-primary"
            onClick={() => onReview(approval.interaction.interaction_id)}
          >
            Review
          </Button>
        ) : null}
        {onOpenDocument ? (
          <Button
            type="button"
            variant="ghost"
            size="sm"
            className="h-7 rounded-md border border-border/70 bg-background/70 px-2.5 text-xs font-medium text-muted-foreground hover:bg-muted hover:text-foreground"
            onClick={onOpenDocument}
          >
            Open doc
          </Button>
        ) : null}
      </div>
    </div>
  );
}

function GenericPreviewPanel({
  preview,
  attachedApproval,
  resolverNamesByUserId,
  expanded = false,
  approvalReferenceOnly = false,
  onReviewApproval,
  openPreviewPanelKey,
  openPreviewRequestId,
}: {
  preview: PublishedPreview;
  attachedApproval?: CodingSessionPreviewApprovalState | null;
  resolverNamesByUserId?: Map<string, string>;
  expanded?: boolean;
  approvalReferenceOnly?: boolean;
  onReviewApproval?: (interactionId: string) => void;
  openPreviewPanelKey?: string | null;
  openPreviewRequestId?: number;
}) {
  const [dialogOpen, setDialogOpen] = useState(false);
  const isMarkdown = preview.format === 'markdown' && typeof preview.content === 'string';

  useEffect(() => {
    if (openPreviewPanelKey === preview.panelKey) {
      setDialogOpen(true);
    }
  }, [openPreviewPanelKey, openPreviewRequestId, preview.panelKey]);

  return (
    <>
      <div
        data-preview-panel-key={preview.panelKey}
        className="rounded-md border border-border/60 bg-card/80 p-3 transition-shadow data-[preview-flash=true]:ring-2 data-[preview-flash=true]:ring-primary/40"
      >
        {approvalReferenceOnly && attachedApproval ? (
          <PreviewApprovalReference
            approval={attachedApproval}
            resolverNamesByUserId={resolverNamesByUserId}
            onReview={onReviewApproval}
            onOpenDocument={() => setDialogOpen(true)}
          />
        ) : (
          <>
            <div className="mb-2 flex items-center justify-between gap-2">
              <div className="flex min-w-0 items-center gap-2">
                <File01Icon className="h-4 w-4 shrink-0 text-muted-foreground" />
                <p className="truncate text-xs font-semibold uppercase tracking-wide text-muted-foreground">{preview.title}</p>
              </div>
              <ExpandPreviewIconButton label={`Open ${preview.title}`} onClick={() => setDialogOpen(true)} />
            </div>
            <div
              className={cn(
                'relative overflow-hidden rounded-md bg-muted/40 p-3',
                expanded ? 'max-h-[55vh]' : 'max-h-[200px]',
              )}
            >
              {isMarkdown ? (
                <MarkdownContent content={preview.content as string} className="text-[12px] leading-5" />
              ) : (
                <pre className="whitespace-pre-wrap text-[12px] leading-5 text-foreground">
                  {JSON.stringify(preview.content, null, 2)}
                </pre>
              )}
              <div
                className={cn(
                  'pointer-events-none absolute inset-x-0 bottom-0 rounded-b-md bg-gradient-to-t from-muted via-muted/70 to-transparent',
                  expanded ? 'h-24' : 'h-12',
                )}
              />
            </div>
            {expanded ? null : (
              <ExpandPreviewButton onClick={() => setDialogOpen(true)} />
            )}
            {attachedApproval ? (
              <PreviewApprovalReference
                approval={attachedApproval}
                resolverNamesByUserId={resolverNamesByUserId}
                onReview={onReviewApproval}
                onOpenDocument={() => setDialogOpen(true)}
              />
            ) : null}
          </>
        )}
      </div>

      <PreviewExpandDialog open={dialogOpen} onOpenChange={setDialogOpen} title={preview.title}>
        {isMarkdown ? (
          <MarkdownContent content={preview.content as string} className="text-[15px] leading-7 text-foreground" />
        ) : (
          <pre className="whitespace-pre-wrap break-all text-[12px] leading-6 text-foreground">
            {JSON.stringify(preview.content, null, 2)}
          </pre>
        )}
      </PreviewExpandDialog>
    </>
  );
}

function TaskPlanPanel({
  title,
  preview,
  attachedApproval,
  resolverNamesByUserId,
  onReviewApproval,
  openPreviewPanelKey,
  openPreviewRequestId,
}: {
  title: string;
  preview: TaskPlanPreviewModel;
  attachedApproval?: CodingSessionPreviewApprovalState | null;
  resolverNamesByUserId?: Map<string, string>;
  onReviewApproval?: (interactionId: string) => void;
  openPreviewPanelKey?: string | null;
  openPreviewRequestId?: number;
}) {
  const [dialogOpen, setDialogOpen] = useState(false);

  useEffect(() => {
    if (openPreviewPanelKey === 'task_plan') {
      setDialogOpen(true);
    }
  }, [openPreviewPanelKey, openPreviewRequestId]);

  const taskPlanContent = (
    <div className="space-y-3">
      {preview.summary ? <MarkdownContent content={preview.summary} /> : null}

      <div className="flex flex-wrap gap-2 text-[11px] text-muted-foreground">
        <span>{preview.proposedTasks.length} tasks</span>
        {preview.risks.length ? <span>{preview.risks.length} risks</span> : null}
        {preview.openQuestions.length ? <span>{preview.openQuestions.length} open questions</span> : null}
      </div>

      <div className="space-y-1">
        {preview.proposedTasks.map((item, index) => (
          <div
            key={`${item.ref ?? item.title}-${index}`}
            className="rounded-md px-1.5 py-1.5 transition-colors hover:bg-accent/30"
          >
            <div className="flex items-start gap-1.5">
              <SparklesIcon className="mt-0.5 h-3.5 w-3.5 shrink-0 text-muted-foreground" />
              <div className="min-w-0 flex-1">
                <div className="flex flex-wrap items-center gap-1.5">
                  {item.type ? (
                    <span className="shrink-0 rounded-md bg-muted/60 px-1.5 py-0.5 text-[10px] font-medium text-muted-foreground">
                      {toTitleCase(item.type)}
                    </span>
                  ) : null}
                  {item.ref ? (
                    <Badge variant="outline" className="h-5 rounded-full px-1.5 text-[10px] font-medium text-muted-foreground">
                      {item.ref}
                    </Badge>
                  ) : null}
                  <p className="min-w-0 text-sm font-medium text-foreground">{item.title}</p>
                </div>

                {item.description ? (
                  <p className="mt-1 text-xs leading-5 text-muted-foreground">{item.description}</p>
                ) : null}

                <div className="mt-1.5 flex flex-wrap gap-1.5">
                  {item.acceptanceCriteria.length ? (
                    <span className="rounded-md bg-blue-50 px-1.5 py-0.5 text-[10px] font-medium text-blue-700 dark:bg-blue-950/40 dark:text-blue-300">
                      {item.acceptanceCriteria.length} acceptance criteria
                    </span>
                  ) : null}
                  {item.dependencyRefs.length ? (
                    <span className="rounded-md bg-amber-50 px-1.5 py-0.5 text-[10px] font-medium text-amber-700 dark:bg-amber-950/40 dark:text-amber-300">
                      Depends on {item.dependencyRefs.join(', ')}
                    </span>
                  ) : null}
                  {item.filesToModify.length ? (
                    <span className="rounded-md bg-violet-50 px-1.5 py-0.5 text-[10px] font-medium text-violet-700 dark:bg-violet-950/40 dark:text-violet-300">
                      {item.filesToModify.length} files touched
                    </span>
                  ) : null}
                </div>
              </div>
            </div>
          </div>
        ))}
      </div>

      {preview.risks.length ? (
        <div className="rounded-lg border border-amber-200/70 bg-amber-50/70 p-3 dark:border-amber-900/60 dark:bg-amber-950/20">
          <p className="text-[11px] font-semibold uppercase tracking-wide text-amber-800 dark:text-amber-200">Risks</p>
          <ul className="mt-2 space-y-1 text-xs leading-5 text-foreground">
            {preview.risks.map((risk, index) => (
              <li key={`${risk}-${index}`}>{risk}</li>
            ))}
          </ul>
        </div>
      ) : null}

      {preview.openQuestions.length ? (
        <div className="rounded-lg border border-border/60 bg-muted/30 p-3">
          <p className="text-[11px] font-semibold uppercase tracking-wide text-muted-foreground">Open questions</p>
          <ul className="mt-2 space-y-1 text-xs leading-5 text-foreground">
            {preview.openQuestions.map((question, index) => (
              <li key={`${question}-${index}`}>{question}</li>
            ))}
          </ul>
        </div>
      ) : null}

      {attachedApproval ? (
        <PreviewApprovalReference
          approval={attachedApproval}
          resolverNamesByUserId={resolverNamesByUserId}
          onReview={onReviewApproval}
          onOpenDocument={() => setDialogOpen(true)}
        />
      ) : null}
    </div>
  );

  return (
    <>
      <div
        data-preview-panel-key="task_plan"
        className="rounded-md border border-border/60 bg-card/80 p-3 transition-shadow data-[preview-flash=true]:ring-2 data-[preview-flash=true]:ring-primary/40"
      >
        {attachedApproval ? (
          <PreviewApprovalReference
            approval={attachedApproval}
            resolverNamesByUserId={resolverNamesByUserId}
            onReview={onReviewApproval}
            onOpenDocument={() => setDialogOpen(true)}
          />
        ) : (
          <>
            <div className="mb-2 flex items-center justify-between gap-2">
              <div className="flex min-w-0 items-center gap-2">
                <SparklesIcon className="h-4 w-4 shrink-0 text-muted-foreground" />
                <p className="truncate text-xs font-semibold uppercase tracking-wide text-muted-foreground">{title}</p>
              </div>
              <ExpandPreviewIconButton label={`Open ${title}`} onClick={() => setDialogOpen(true)} />
            </div>
            {taskPlanContent}
          </>
        )}
      </div>
      <PreviewExpandDialog open={dialogOpen} onOpenChange={setDialogOpen} title={title}>
        {taskPlanContent}
      </PreviewExpandDialog>
    </>
  );
}

export function CodingPreviewPanels({
  previewsByKey,
  attachedApprovalInteraction,
  approvalStatesByPreviewKey,
  resolverNamesByUserId,
  onReviewApproval,
  openPreviewPanelKey,
  openPreviewRequestId = 0,
}: {
  previewsByKey: Map<string, PublishedPreview>;
  attachedApprovalInteraction?: CodingSessionInteraction | null;
  approvalStatesByPreviewKey?: Map<string, CodingSessionPreviewApprovalState>;
  resolverNamesByUserId?: Map<string, string>;
  onReviewApproval?: (interactionId: string) => void;
  openPreviewPanelKey?: string | null;
  openPreviewRequestId?: number;
}) {
  const attachedApproval = useMemo(
    () => parseAttachedApprovalRequest(attachedApprovalInteraction),
    [attachedApprovalInteraction],
  );
  const attachedApprovalState = useMemo(
    () => previewApprovalStateFromAttachedApproval(attachedApproval),
    [attachedApproval],
  );
  const approvalForPanel = (panelKey: string) => {
    const normalizedPanelKey = normalizeCodingSessionPreviewPanelKey(panelKey) || panelKey;
    const state = approvalStatesByPreviewKey?.get(normalizedPanelKey);
    if (state) return state;
    return attachedApprovalState?.previewPanelKey === normalizedPanelKey ? attachedApprovalState : null;
  };
  const latestSpecDraftPreview = (() => {
    const preview = previewsByKey.get('prd_draft');
    if (preview?.format === 'markdown' && typeof preview.content === 'string') {
      return preview;
    }
    return null;
  })();

  const latestTaskPlanPreview = parseTaskPlanPreviewModel(previewsByKey.get('task_plan'));
  const latestDocsChangePreview = parseDocsChangePreviewModel(previewsByKey.get('docs_change'));
  const otherPreviewPanels = Array.from(previewsByKey.values()).filter((preview) => {
    if (preview.panelKey === 'prd_draft' && latestSpecDraftPreview) return false;
    if (preview.panelKey === 'task_plan' && latestTaskPlanPreview) return false;
    if (preview.panelKey === 'docs_change' && latestDocsChangePreview) return false;
    return true;
  });
  const [prdDialogOpen, setPrdDialogOpen] = useState(false);

  useEffect(() => {
    if (openPreviewPanelKey === 'prd_draft') {
      setPrdDialogOpen(true);
    }
  }, [openPreviewPanelKey, openPreviewRequestId]);

  const panelCount = (latestSpecDraftPreview ? 1 : 0)
    + (latestTaskPlanPreview ? 1 : 0)
    + (latestDocsChangePreview ? 1 : 0)
    + otherPreviewPanels.length;
  const isSolo = panelCount === 1;

  if (!latestSpecDraftPreview && !latestTaskPlanPreview && !latestDocsChangePreview && otherPreviewPanels.length === 0) {
    return null;
  }
  const prdApproval = approvalForPanel('prd_draft');

  return (
    <>
      {latestSpecDraftPreview ? (
        <>
          <div
            data-preview-panel-key="prd_draft"
            className="rounded-md border border-border/60 bg-card/80 p-3 transition-shadow data-[preview-flash=true]:ring-2 data-[preview-flash=true]:ring-primary/40"
          >
            {prdApproval ? (
              <PreviewApprovalReference
                approval={prdApproval}
                resolverNamesByUserId={resolverNamesByUserId}
                onReview={onReviewApproval}
                onOpenDocument={() => setPrdDialogOpen(true)}
              />
            ) : (
              <>
                <div className="mb-2 flex items-center justify-between gap-2">
                  <div className="flex min-w-0 items-center gap-2">
                    <File01Icon className="h-4 w-4 shrink-0 text-muted-foreground" />
                    <p className="truncate text-xs font-semibold uppercase tracking-wide text-muted-foreground">
                      {latestSpecDraftPreview.title || 'PRD Draft'}
                    </p>
                  </div>
                  <ExpandPreviewIconButton
                    label={`Open ${latestSpecDraftPreview.title || 'PRD Draft'}`}
                    onClick={() => setPrdDialogOpen(true)}
                  />
                </div>
                <div className="max-h-[280px] overflow-auto rounded-md bg-muted/40 p-3">
                  <MarkdownContent content={latestSpecDraftPreview.content as string} className="text-[12px] leading-5" />
                </div>
              </>
            )}
          </div>
          <PreviewExpandDialog
            open={prdDialogOpen}
            onOpenChange={setPrdDialogOpen}
            title={latestSpecDraftPreview.title || 'PRD Draft'}
          >
            <MarkdownContent
              content={latestSpecDraftPreview.content as string}
              className="text-[15px] leading-7 text-foreground"
            />
          </PreviewExpandDialog>
        </>
      ) : null}

      {latestTaskPlanPreview ? (
        <TaskPlanPanel
          title={previewsByKey.get('task_plan')?.title || 'Task Plan'}
          preview={latestTaskPlanPreview}
          attachedApproval={approvalForPanel('task_plan')}
          resolverNamesByUserId={resolverNamesByUserId}
          onReviewApproval={onReviewApproval}
          openPreviewPanelKey={openPreviewPanelKey}
          openPreviewRequestId={openPreviewRequestId}
        />
      ) : null}

      {latestDocsChangePreview ? (
        <DocsChangePanel
          title={previewsByKey.get('docs_change')?.title || 'Docs Change Proposal'}
          preview={latestDocsChangePreview}
          attachedApproval={approvalForPanel('docs_change')}
          resolverNamesByUserId={resolverNamesByUserId}
          onReviewApproval={onReviewApproval}
          openPreviewPanelKey={openPreviewPanelKey}
          openPreviewRequestId={openPreviewRequestId}
        />
      ) : null}

      {otherPreviewPanels.map((preview) => (
        <GenericPreviewPanel
          key={preview.panelKey}
          preview={preview}
          expanded={isSolo}
          attachedApproval={approvalForPanel(preview.panelKey)}
          resolverNamesByUserId={resolverNamesByUserId}
          approvalReferenceOnly={approvalForPanel(preview.panelKey) !== null}
          onReviewApproval={onReviewApproval}
          openPreviewPanelKey={openPreviewPanelKey}
          openPreviewRequestId={openPreviewRequestId}
        />
      ))}
    </>
  );
}
