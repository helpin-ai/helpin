import { FileText, Sparkles } from 'lucide-react';

import { Badge } from '@/components/ui/badge';
import type { PublishedPreview } from '@/components/pm/runPreviews';
import { MarkdownContent } from './MarkdownContent';

interface StoryPlanStoryPreview {
  ref?: string;
  title: string;
  type?: string;
  description?: string;
  acceptanceCriteria: string[];
  dependencyRefs: string[];
  filesToModify: string[];
}

interface StoryPlanPreviewModel {
  summary?: string;
  proposedStories: StoryPlanStoryPreview[];
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

function parseStoryPlanPreviewModel(preview: PublishedPreview | undefined): StoryPlanPreviewModel | null {
  if (!preview || preview.format !== 'json') return null;
  const record = asRecord(preview.content);
  const proposedStories = Array.isArray(record?.proposed_stories) ? record.proposed_stories : [];
  if (!record || proposedStories.length === 0) return null;

  const normalizedStories: StoryPlanStoryPreview[] = proposedStories.flatMap((entry) => {
    const story = asRecord(entry);
    if (!story) return [];

    const implementationBrief = asRecord(story.implementation_brief);
    const filesToModify = Array.isArray(implementationBrief?.files_to_modify)
      ? implementationBrief.files_to_modify
        .map((fileEntry) => asRecord(fileEntry))
        .map((fileEntry) => asString(fileEntry?.path).trim())
        .filter((path) => path.length > 0)
      : [];

    const title = asString(story.name).trim() || asString(story.title).trim();
    if (!title) return [];

    return [{
      ref: asString(story.ref).trim() || undefined,
      title,
      type: asString(story.story_type).trim() || asString(story.type).trim() || asString(story.slice_type).trim() || undefined,
      description: asString(story.description).trim() || undefined,
      acceptanceCriteria: asStringArray(story.acceptance_criteria),
      dependencyRefs: asStringArray(story.dependency_refs),
      filesToModify,
    } satisfies StoryPlanStoryPreview];
  });

  if (normalizedStories.length === 0) return null;

  return {
    summary: asString(record.summary).trim() || undefined,
    proposedStories: normalizedStories,
    risks: asStringArray(record.risks),
    openQuestions: asStringArray(record.open_questions),
  };
}

function GenericPreviewPanel({ preview }: { preview: PublishedPreview }) {
  return (
    <div className="rounded-md border border-border/60 bg-background/80 p-3">
      <div className="mb-2 flex items-center gap-2">
        <FileText className="h-4 w-4 text-muted-foreground" />
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

function StoryPlanPanel({
  title,
  preview,
}: {
  title: string;
  preview: StoryPlanPreviewModel;
}) {
  return (
    <div className="rounded-md border border-border/60 bg-background/80 p-3">
      <div className="mb-2 flex items-center gap-2">
        <Sparkles className="h-4 w-4 text-muted-foreground" />
        <p className="text-xs font-semibold uppercase tracking-wide text-muted-foreground">{title}</p>
      </div>
      <div className="space-y-3">
        {preview.summary ? <MarkdownContent content={preview.summary} /> : null}

        <div className="flex flex-wrap gap-2 text-[11px] text-muted-foreground">
          <span>{preview.proposedStories.length} stories</span>
          {preview.risks.length ? <span>{preview.risks.length} risks</span> : null}
          {preview.openQuestions.length ? <span>{preview.openQuestions.length} open questions</span> : null}
        </div>

        <div className="space-y-1">
          {preview.proposedStories.map((story, index) => (
            <div
              key={`${story.ref ?? story.title}-${index}`}
              className="rounded-md px-1.5 py-1.5 transition-colors hover:bg-accent/30"
            >
              <div className="flex items-start gap-1.5">
                <Sparkles className="mt-0.5 h-3.5 w-3.5 shrink-0 text-muted-foreground" />
                <div className="min-w-0 flex-1">
                  <div className="flex flex-wrap items-center gap-1.5">
                    {story.type ? (
                      <span className="shrink-0 rounded-md bg-muted/60 px-1.5 py-0.5 text-[10px] font-medium text-muted-foreground">
                        {toTitleCase(story.type)}
                      </span>
                    ) : null}
                    {story.ref ? (
                      <Badge variant="outline" className="h-5 rounded-full px-1.5 text-[10px] font-medium text-muted-foreground">
                        {story.ref}
                      </Badge>
                    ) : null}
                    <p className="min-w-0 text-sm font-medium text-foreground">{story.title}</p>
                  </div>

                  {story.description ? (
                    <p className="mt-1 text-xs leading-5 text-muted-foreground">{story.description}</p>
                  ) : null}

                  <div className="mt-1.5 flex flex-wrap gap-1.5">
                    {story.acceptanceCriteria.length ? (
                      <span className="rounded-md bg-blue-50 px-1.5 py-0.5 text-[10px] font-medium text-blue-700 dark:bg-blue-950/40 dark:text-blue-300">
                        {story.acceptanceCriteria.length} acceptance criteria
                      </span>
                    ) : null}
                    {story.dependencyRefs.length ? (
                      <span className="rounded-md bg-amber-50 px-1.5 py-0.5 text-[10px] font-medium text-amber-700 dark:bg-amber-950/40 dark:text-amber-300">
                        Depends on {story.dependencyRefs.join(', ')}
                      </span>
                    ) : null}
                    {story.filesToModify.length ? (
                      <span className="rounded-md bg-violet-50 px-1.5 py-0.5 text-[10px] font-medium text-violet-700 dark:bg-violet-950/40 dark:text-violet-300">
                        {story.filesToModify.length} files touched
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

  const latestStoryPlanPreview = parseStoryPlanPreviewModel(previewsByKey.get('task_plan'));
  const otherPreviewPanels = Array.from(previewsByKey.values()).filter((preview) => {
    if (preview.panelKey === 'prd_draft' && latestSpecDraftPreview) return false;
    if (preview.panelKey === 'task_plan' && latestStoryPlanPreview) return false;
    return true;
  });

  if (!latestSpecDraftPreview && !latestStoryPlanPreview && otherPreviewPanels.length === 0) {
    return null;
  }

  return (
    <>
      {latestSpecDraftPreview ? (
        <div className="rounded-md border border-border/60 bg-background/80 p-3">
          <div className="mb-2 flex items-center gap-2">
            <FileText className="h-4 w-4 text-muted-foreground" />
            <p className="text-xs font-semibold uppercase tracking-wide text-muted-foreground">
              {latestSpecDraftPreview.title || 'PRD Draft'}
            </p>
          </div>
          <div className="max-h-[280px] overflow-auto rounded-md bg-muted/40 p-3">
            <MarkdownContent content={latestSpecDraftPreview.content as string} className="text-[12px] leading-5" />
          </div>
        </div>
      ) : null}

      {latestStoryPlanPreview ? (
        <StoryPlanPanel
          title={previewsByKey.get('task_plan')?.title || 'Task Plan'}
          preview={latestStoryPlanPreview}
        />
      ) : null}

      {otherPreviewPanels.map((preview) => (
        <GenericPreviewPanel key={preview.panelKey} preview={preview} />
      ))}
    </>
  );
}
