import { ArrowReloadHorizontalIcon, BookOpen01Icon, PlusSignIcon } from '@/lib/icons';
import { Button } from '@/components/ui/button';
import { Checkbox } from '@/components/ui/checkbox';
import { Badge } from '@/components/ui/badge';
import { Progress } from '@/components/ui/progress';
import { cn } from '@/lib/utils';
import type { DocsSpace } from '@/lib/docsTypes';
import type { AgentKnowledgeSource } from '@/lib/pmTypes';

const STATUS_META: Record<string, { label: string; className: string }> = {
  queued: { label: 'Queued', className: 'border-amber-500/40 bg-amber-500/10 text-amber-700' },
  running: { label: 'Indexing', className: 'border-sky-500/40 bg-sky-500/10 text-sky-700' },
  ready: { label: 'Indexed', className: 'border-emerald-500/40 bg-emerald-500/10 text-emerald-700' },
  failed: { label: 'Failed', className: 'border-destructive/40 bg-destructive/10 text-destructive' },
  stale: { label: 'Stale', className: 'border-orange-500/40 bg-orange-500/10 text-orange-700' },
  disabled: { label: 'Disabled', className: 'border-muted-foreground/30 bg-muted text-muted-foreground' },
};

export function SupportKnowledgeSourcesField({
  agentId,
  spaces,
  knowledgeSources,
  onToggle,
  onReindex,
  reindexingSpaceId,
  disabled = false,
  onOpenDocs,
}: {
  agentId?: string;
  spaces: DocsSpace[];
  knowledgeSources: AgentKnowledgeSource[];
  onToggle: (spaceId: string) => void;
  onReindex: (spaceId: string) => void;
  reindexingSpaceId?: string;
  disabled?: boolean;
  onOpenDocs?: () => void;
}) {
  if (spaces.length === 0) {
    return (
      <div className="rounded-xl border border-dashed border-border/80 bg-muted/20 px-4 py-8 text-center">
        <div className="mx-auto mb-3 flex h-10 w-10 items-center justify-center rounded-full bg-muted text-muted-foreground">
          <BookOpen01Icon className="h-5 w-5" />
        </div>
        <p className="text-sm font-medium">No public docs spaces yet</p>
        <p className="mx-auto mt-1 max-w-lg text-sm leading-6 text-muted-foreground">
          Create a Help Center docs space, publish articles, then return here to make them searchable by AI.
        </p>
        {onOpenDocs ? (
          <Button type="button" variant="outline" size="sm" className="mt-4" onClick={onOpenDocs}>
            <PlusSignIcon className="h-4 w-4" />
            Open docs
          </Button>
        ) : null}
      </div>
    );
  }

  const sourceBySpaceId = new Map(knowledgeSources.map((source) => [source.space_id, source]));

  return (
    <div className="space-y-2 rounded-md border p-3">
      {spaces.map((space) => {
        const source = sourceBySpaceId.get(space.id);
        const selected = !!source;
        const status = source?.sync_status ?? 'idle';
        const statusMeta = STATUS_META[status];
        const showProgress = status === 'queued' || status === 'running';

        return (
          <label
            key={space.id}
            className={cn(
              'flex cursor-pointer flex-col gap-2 rounded-md border px-3 py-2 transition-colors',
              selected ? 'border-border bg-muted/30' : 'border-border/60 bg-background',
            )}
          >
            <div className="flex items-start gap-3">
              <Checkbox
                checked={selected}
                disabled={disabled}
                onCheckedChange={() => onToggle(space.id)}
              />

              <div className="min-w-0 flex-1 space-y-1">
                <div className="flex flex-wrap items-center gap-2">
                  <span className="text-sm font-medium">{space.name}</span>
                  <Badge variant="outline">Help Center</Badge>
                  {selected && statusMeta && (
                    <Badge variant="outline" className={statusMeta.className}>
                      {statusMeta.label}
                    </Badge>
                  )}
                  {selected && agentId && (
                    <Button
                      type="button"
                      variant="ghost"
                      size="icon"
                      className="h-7 w-7"
                      disabled={disabled}
                      onClick={(event) => {
                        event.preventDefault();
                        event.stopPropagation();
                        onReindex(space.id);
                      }}
                      title="Reindex help center docs"
                    >
                      <ArrowReloadHorizontalIcon className={cn('h-3.5 w-3.5', reindexingSpaceId === space.id && 'animate-spin')} />
                    </Button>
                  )}
                </div>

                <p className="text-xs text-muted-foreground">
                  {selected
                    ? indexedSummary(source)
                    : agentId
                      ? 'Select to chunk published public docs, generate embeddings, and make this space searchable by the support agent.'
                      : 'Published help center space. Select a support agent in AI Assistant to attach it to support AI.'}
                </p>
              </div>
            </div>

            {selected && showProgress && (
              <div className="space-y-1 pl-7">
                <Progress value={Math.max(0, Math.min(source.sync_progress ?? 0, 100))} className="h-2" />
                <p className="text-[11px] text-muted-foreground">
                  {status === 'queued'
                    ? 'Waiting to start indexing.'
                    : `Embedding ${source.indexed_documents ?? 0} docs and ${source.indexed_chunks ?? 0} chunks.`}
                </p>
              </div>
            )}

            {selected && source?.last_sync_error && (
              <p className="pl-7 text-xs text-destructive">{source.last_sync_error}</p>
            )}
          </label>
        );
      })}
    </div>
  );
}

function indexedSummary(source?: AgentKnowledgeSource) {
  if (!source) {
    return '';
  }

  if (source.sync_status === 'queued' && (source.indexed_documents ?? 0) > 0) {
    return `Re-index queued. Current index includes ${source.indexed_documents ?? 0} published docs across ${source.indexed_chunks ?? 0} chunks.`;
  }
  if (source.sync_status === 'ready') {
    return `${source.indexed_documents ?? 0} published docs indexed across ${source.indexed_chunks ?? 0} chunks.`;
  }
  if (source.sync_status === 'disabled') {
    return source.last_sync_error ?? 'Embedding provider is not configured for this workspace.';
  }
  if (source.sync_status === 'failed') {
    return 'Last indexing run failed. Fix the issue and reselect the space to retry.';
  }
  if (source.sync_status === 'stale') {
    return 'Source changes are waiting to be re-indexed.';
  }
  return `${source.indexed_documents ?? 0} docs indexed so far across ${source.indexed_chunks ?? 0} chunks.`;
}
