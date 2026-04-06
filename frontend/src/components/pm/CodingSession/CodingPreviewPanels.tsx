import { File01Icon, SparklesIcon } from '@/lib/icons';

import { Badge } from '@/components/ui/badge';
import type { PublishedPreview } from '@/components/pm/runPreviews';
import { MarkdownContent } from './MarkdownContent';

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

function GenericPreviewPanel({ preview }: { preview: PublishedPreview }) {
  return (
    <div className="rounded-md border border-border/60 bg-card/80 p-3">
      <div className="mb-2 flex items-center gap-2">
        <File01Icon className="h-4 w-4 text-muted-foreground" />
        <p className="text-xs font-semibold uppercase tracking-wide text-muted-foreground">{preview.title}</p>
      </div>
      <div className="max-h-[280px] overflow-auto rounded-md bg-muted/40 p-3">
        {preview.format === 'markdown' && typeof preview.content === 'string' ? (
          <MarkdownContent content={preview.content} className="text-[12px] leading-5" />
        ) : (
          <pre className="whitespace-pre-wrap text-[12px] leading-5 text-foreground">
            {JSON.stringify(preview.content, null, 2)}
          </pre>
        )}
      </div>
    </div>
  );
}

function TaskPlanPanel({
  title,
  preview,
}: {
  title: string;
  preview: TaskPlanPreviewModel;
}) {
  return (
    <div className="rounded-md border border-border/60 bg-card/80 p-3">
      <div className="mb-2 flex items-center gap-2">
        <SparklesIcon className="h-4 w-4 text-muted-foreground" />
        <p className="text-xs font-semibold uppercase tracking-wide text-muted-foreground">{title}</p>
      </div>
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
      </div>
    </div>
  );
}

export function CodingPreviewPanels({ previewsByKey }: { previewsByKey: Map<string, PublishedPreview> }) {
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

  if (!latestSpecDraftPreview && !latestTaskPlanPreview && otherPreviewPanels.length === 0) {
    return null;
  }

  return (
    <>
      {latestSpecDraftPreview ? (
        <div className="rounded-md border border-border/60 bg-card/80 p-3">
          <div className="mb-2 flex items-center gap-2">
            <File01Icon className="h-4 w-4 text-muted-foreground" />
            <p className="text-xs font-semibold uppercase tracking-wide text-muted-foreground">
              {latestSpecDraftPreview.title || 'PRD Draft'}
            </p>
          </div>
          <div className="max-h-[280px] overflow-auto rounded-md bg-muted/40 p-3">
            <MarkdownContent content={latestSpecDraftPreview.content as string} className="text-[12px] leading-5" />
          </div>
        </div>
      ) : null}

      {latestTaskPlanPreview ? (
        <TaskPlanPanel
          title={previewsByKey.get('task_plan')?.title || 'Task Plan'}
          preview={latestTaskPlanPreview}
        />
      ) : null}

      {otherPreviewPanels.map((preview) => (
        <GenericPreviewPanel key={preview.panelKey} preview={preview} />
      ))}
    </>
  );
}
