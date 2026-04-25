import { useEffect, useMemo, useState } from 'react';
import { ArrowExpandIcon, File01Icon, SparklesIcon } from '@/lib/icons';

import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { Textarea } from '@/components/ui/textarea';
import type { PublishedPreview } from '@/components/pm/runPreviews';
import type { CodingSessionInteraction } from '@/lib/pmTypes';
import { MarkdownContent } from './MarkdownContent';
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
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="z-[140] gap-0 overflow-hidden border-border/70 bg-background p-0 shadow-2xl sm:max-w-5xl">
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
      </DialogContent>
    </Dialog>
  );
}

function ExpandPreviewButton({ onClick }: { onClick: () => void }) {
  return (
    <button
      type="button"
      className="mt-2 inline-flex items-center gap-1 text-[11px] font-medium text-primary hover:underline"
      onClick={onClick}
    >
      <ArrowExpandIcon className="h-3 w-3" />
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
      <ArrowExpandIcon className="h-3.5 w-3.5" />
    </Button>
  );
}

function PreviewApprovalFooter({
  approval,
  acting,
  onResolve,
}: {
  approval: AttachedApprovalRequest;
  acting: string | null;
  onResolve: (interactionId: string, responsePayload: Record<string, unknown>, followupMessage?: string) => void;
}) {
  const [followupMessage, setFollowupMessage] = useState('');

  useEffect(() => {
    setFollowupMessage('');
  }, [approval.interaction.interaction_id]);

  const isBusy = acting === 'resolve-interaction';

  return (
    <div className="mt-3 rounded-lg border border-primary/15 bg-primary/[0.04] p-3">
      <div className="flex flex-wrap items-center gap-2">
        <Badge variant="outline" className="border-primary/20 bg-background text-[10px] uppercase tracking-wide text-primary">
          {approval.phase ? `${approval.phase} approval` : 'Approval required'}
        </Badge>
        <p className="text-sm font-semibold text-foreground">{approval.title ?? 'Approval required'}</p>
      </div>
      {approval.summary ? (
        <p className="mt-1 text-sm leading-6 text-muted-foreground">{approval.summary}</p>
      ) : null}
      <Textarea
        value={followupMessage}
        onChange={(event) => setFollowupMessage(event.target.value)}
        placeholder="Optional note for the agent"
        className="mt-3 min-h-[76px] bg-background"
        disabled={isBusy}
      />
      <div className="mt-3 flex flex-wrap gap-2">
        <Button
          size="sm"
          disabled={isBusy}
          onClick={() => onResolve(
            approval.interaction.interaction_id,
            { decision: 'approve', ...(followupMessage.trim() ? { message: followupMessage.trim() } : {}) },
            followupMessage.trim() || undefined,
          )}
        >
          Approve
        </Button>
        <Button
          variant="outline"
          size="sm"
          disabled={isBusy}
          onClick={() => onResolve(
            approval.interaction.interaction_id,
            { decision: 'request_changes', ...(followupMessage.trim() ? { message: followupMessage.trim() } : {}) },
            followupMessage.trim() || undefined,
          )}
        >
          Request changes
        </Button>
      </div>
    </div>
  );
}

function GenericPreviewPanel({
  preview,
  attachedApproval,
  acting,
  onResolveInteraction,
}: {
  preview: PublishedPreview;
  attachedApproval?: AttachedApprovalRequest | null;
  acting?: string | null;
  onResolveInteraction?: (interactionId: string, responsePayload: Record<string, unknown>, followupMessage?: string) => void;
}) {
  const [dialogOpen, setDialogOpen] = useState(false);
  const isMarkdown = preview.format === 'markdown' && typeof preview.content === 'string';

  return (
    <>
      <div
        data-preview-panel-key={preview.panelKey}
        className="rounded-md border border-border/60 bg-card/80 p-3 transition-shadow data-[preview-flash=true]:ring-2 data-[preview-flash=true]:ring-primary/40"
      >
        <button
          type="button"
          className="mb-2 flex w-full items-center gap-2 text-left transition-colors hover:text-primary"
          onClick={() => setDialogOpen(true)}
        >
          <File01Icon className="h-4 w-4 text-muted-foreground" />
          <p className="text-xs font-semibold uppercase tracking-wide text-muted-foreground">{preview.title}</p>
        </button>
        <div className="relative max-h-[200px] overflow-hidden rounded-md bg-muted/40 p-3">
          {isMarkdown ? (
            <MarkdownContent content={preview.content as string} className="text-[12px] leading-5" />
          ) : (
            <pre className="whitespace-pre-wrap text-[12px] leading-5 text-foreground">
              {JSON.stringify(preview.content, null, 2)}
            </pre>
          )}
          <div className="pointer-events-none absolute inset-x-0 bottom-0 h-12 rounded-b-md bg-gradient-to-t from-muted/80 to-transparent" />
        </div>
        <ExpandPreviewButton onClick={() => setDialogOpen(true)} />
        {attachedApproval && onResolveInteraction ? (
          <PreviewApprovalFooter
            approval={attachedApproval}
            acting={acting ?? null}
            onResolve={onResolveInteraction}
          />
        ) : null}
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
  acting,
  onResolveInteraction,
}: {
  title: string;
  preview: TaskPlanPreviewModel;
  attachedApproval?: AttachedApprovalRequest | null;
  acting?: string | null;
  onResolveInteraction?: (interactionId: string, responsePayload: Record<string, unknown>, followupMessage?: string) => void;
}) {
  const [dialogOpen, setDialogOpen] = useState(false);

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

      {attachedApproval && onResolveInteraction ? (
        <PreviewApprovalFooter
          approval={attachedApproval}
          acting={acting ?? null}
          onResolve={onResolveInteraction}
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
        <div className="mb-2 flex items-center justify-between gap-2">
          <div className="flex min-w-0 items-center gap-2">
            <SparklesIcon className="h-4 w-4 shrink-0 text-muted-foreground" />
            <p className="truncate text-xs font-semibold uppercase tracking-wide text-muted-foreground">{title}</p>
          </div>
          <ExpandPreviewIconButton label={`Open ${title}`} onClick={() => setDialogOpen(true)} />
        </div>
        {taskPlanContent}
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
  acting,
  onResolveInteraction,
}: {
  previewsByKey: Map<string, PublishedPreview>;
  attachedApprovalInteraction?: CodingSessionInteraction | null;
  acting?: string | null;
  onResolveInteraction?: (interactionId: string, responsePayload: Record<string, unknown>, followupMessage?: string) => void;
}) {
  const attachedApproval = useMemo(
    () => parseAttachedApprovalRequest(attachedApprovalInteraction),
    [attachedApprovalInteraction],
  );
  const latestSpecDraftPreview = (() => {
    const preview = previewsByKey.get('prd_draft');
    if (preview?.format === 'markdown' && typeof preview.content === 'string') {
      return preview;
    }
    return null;
  })();

  const latestTaskPlanPreview = parseTaskPlanPreviewModel(previewsByKey.get('task_plan'));
  const otherPreviewPanels = Array.from(previewsByKey.values()).filter((preview) => {
    if (preview.panelKey === 'prd_draft' && latestSpecDraftPreview) return false;
    if (preview.panelKey === 'task_plan' && latestTaskPlanPreview) return false;
    return true;
  });
  const [prdDialogOpen, setPrdDialogOpen] = useState(false);

  if (!latestSpecDraftPreview && !latestTaskPlanPreview && otherPreviewPanels.length === 0) {
    return null;
  }

  return (
    <>
      {latestSpecDraftPreview ? (
        <>
          <div
            data-preview-panel-key="prd_draft"
            className="rounded-md border border-border/60 bg-card/80 p-3 transition-shadow data-[preview-flash=true]:ring-2 data-[preview-flash=true]:ring-primary/40"
          >
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
            {attachedApproval?.previewPanelKey === 'prd_draft' && onResolveInteraction ? (
              <PreviewApprovalFooter
                approval={attachedApproval}
                acting={acting ?? null}
                onResolve={onResolveInteraction}
              />
            ) : null}
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
          attachedApproval={attachedApproval?.previewPanelKey === 'task_plan' ? attachedApproval : null}
          acting={acting}
          onResolveInteraction={onResolveInteraction}
        />
      ) : null}

      {otherPreviewPanels.map((preview) => (
        <GenericPreviewPanel
          key={preview.panelKey}
          preview={preview}
          attachedApproval={attachedApproval?.previewPanelKey === preview.panelKey ? attachedApproval : null}
          acting={acting}
          onResolveInteraction={onResolveInteraction}
        />
      ))}
    </>
  );
}
