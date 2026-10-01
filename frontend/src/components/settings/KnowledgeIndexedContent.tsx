import { useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import { QuietSearchInput } from '@/components/design-system/quiet';
import { Button } from '@/components/ui/button';
import { Sheet, SheetContent, SheetDescription, SheetHeader, SheetTitle, SheetTrigger } from '@/components/ui/sheet';
import { Skeleton } from '@/components/ui/skeleton';
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip';
import { ArrowUpRight01Icon, File01Icon, GlobeIcon } from '@/lib/icons';
import type { KnowledgeSourceRow } from '@/lib/knowledgeSourcesPresentation';
import { queryKeys } from '@/lib/queryKeys';
import { unwrap } from '@/lib/queryUtils';
import { agentService } from '@/lib/services/agentService';
import { PageContentPreview } from './support-content-sources/PageContentPreview';

type IndexedItem = { id: string; title: string; url?: string; documentId?: string };

export function KnowledgeIndexedContent({ source, workspaceId, workspaceSlug, agentId }: {
  source: KnowledgeSourceRow;
  workspaceId: string;
  workspaceSlug: string;
  agentId: string;
}) {
  const [open, setOpen] = useState(false);
  const [search, setSearch] = useState('');
  const [previewPageId, setPreviewPageId] = useState<string | null>(null);
  const { data, isPending, isError, refetch } = useQuery({
    queryKey: [
      ...queryKeys.agents.indexedKnowledgeContent(workspaceId, source.id),
      { status: source.status, count: source.countLabel, lastSyncAt: source.lastSyncAt },
    ],
    queryFn: async (): Promise<IndexedItem[]> => {
      if (source.type === 'helpin_docs') {
        if (!agentId || !source.knowledgeSourceId) throw new Error('Knowledge source is unavailable');
        const documents = unwrap(await agentService.listIndexedKnowledgeDocuments(workspaceId, agentId, source.knowledgeSourceId));
        return documents.map(document => ({ id: document.id, title: document.title, documentId: document.id }));
      }
      const pages = unwrap(await agentService.listContentSourcePages(workspaceId, source.sourceId, true));
      return pages.map(page => ({ id: page.id, title: page.title || source.name, url: page.url }));
    },
    enabled: open && Boolean(workspaceId),
    staleTime: 30_000,
    retry: false,
  });
  const items = data ?? [];
  const term = search.trim().toLocaleLowerCase();
  const filtered = items.filter(item => `${item.title} ${item.url ?? ''}`.toLocaleLowerCase().includes(term));
  const unit = source.type === 'helpin_docs' ? 'article' : 'page';
  const countLabel = data ? `${data.length} ${unit}${data.length === 1 ? '' : 's'}` : source.countLabel;
  const syncing = source.status === 'queued' || source.status === 'running';

  return (
    <Sheet open={open} onOpenChange={next => {
      setOpen(next);
      if (!next) { setSearch(''); setPreviewPageId(null); }
    }}>
      <Tooltip>
        <TooltipTrigger asChild>
          <SheetTrigger asChild>
            <button
              type="button"
              aria-label={`View indexed content in ${source.name}`}
              className="cursor-pointer py-1 text-sm text-quiet-text-secondary underline decoration-dotted decoration-quiet-text-tertiary underline-offset-4 transition-colors hover:text-quiet-accent hover:decoration-current focus-visible:outline-2 focus-visible:outline-ring"
            >
              {source.countLabel}
            </button>
          </SheetTrigger>
        </TooltipTrigger>
        <TooltipContent>View indexed content</TooltipContent>
      </Tooltip>
      <SheetContent side="right" className="gap-0 overflow-hidden border-quiet-divider-strong p-0 data-[side=right]:w-full data-[side=right]:sm:max-w-[640px]">
        <SheetHeader className="shrink-0 border-b border-quiet-divider-strong px-6 py-5 pr-16 text-left">
          <SheetTitle>Indexed content</SheetTitle>
          <SheetDescription className="flex min-w-0 gap-2 text-xs">
            <span className="truncate" title={source.name}>{source.name}</span>
            <span className="shrink-0 text-quiet-text-tertiary">· {countLabel}</span>
          </SheetDescription>
        </SheetHeader>
        {previewPageId ? (
          <div className="min-h-0 flex-1">
            <PageContentPreview workspaceId={workspaceId} contentSourceId={source.sourceId} pageId={previewPageId} onBack={() => setPreviewPageId(null)} backLabel="Back to indexed content" />
          </div>
        ) : isPending ? (
          <div className="space-y-4 px-6 py-5" role="status" aria-label="Loading indexed content">
            {[0, 1, 2].map(index => <Skeleton key={index} className="h-10 w-full" />)}
          </div>
        ) : isError ? (
          <div className="space-y-3 px-6 py-8" role="alert">
            <p className="text-sm text-quiet-text-secondary">Could not load indexed content.</p>
            <Button type="button" variant="outline" size="sm" onClick={() => void refetch()}>Retry</Button>
          </div>
        ) : items.length === 0 ? (
          <div className="space-y-2 px-6 py-8">
            <p className="text-sm font-medium">No indexed content yet.</p>
            <p className="text-sm text-quiet-text-secondary">{syncing ? 'Indexing is in progress. Content will appear here when it’s ready.' : 'Sync this source to add content to the index.'}</p>
          </div>
        ) : (
          <>
            <div className="shrink-0 px-6 py-4">
              <QuietSearchInput type="search" aria-label="Search indexed content" placeholder="Search indexed content…" value={search} onChange={event => setSearch(event.target.value)} />
            </div>
            <div className="min-h-0 flex-1 overflow-y-auto px-6 pb-6">
              {filtered.length === 0 ? <p className="py-4 text-sm text-quiet-text-secondary">No content matches your search.</p> : (
                <ul className="divide-y divide-quiet-divider-strong" aria-label="Indexed items">
                  {filtered.map(item => {
                    const externalURL = /^https?:\/\//i.test(item.url ?? '') ? item.url : undefined;
                    const Icon = source.type === 'website' ? GlobeIcon : File01Icon;
                    return (
                      <li key={item.id} className="flex min-w-0 items-start gap-3 py-4 first:pt-1">
                        <Icon className="mt-0.5 h-4 w-4 shrink-0 text-quiet-text-tertiary" aria-hidden="true" />
                        <div className="min-w-0 flex-1">
                          {item.documentId && workspaceSlug ? (
                            <a className="inline-flex max-w-full items-center gap-2 text-sm font-medium text-quiet-text-primary hover:text-quiet-accent hover:underline focus-visible:outline-2 focus-visible:outline-ring" href={`/w/${encodeURIComponent(workspaceSlug)}/docs/documents/${encodeURIComponent(item.documentId)}`} target="_blank" rel="noopener noreferrer">
                              <span className="break-words">{item.title}</span><ArrowUpRight01Icon className="h-3.5 w-3.5 shrink-0" aria-hidden="true" />
                            </a>
                          ) : (
                            <button type="button" className="block max-w-full cursor-pointer break-words text-left text-sm font-medium text-quiet-text-primary hover:text-quiet-accent hover:underline focus-visible:outline-2 focus-visible:outline-ring" onClick={() => setPreviewPageId(item.id)}>{item.title}</button>
                          )}
                          {externalURL && <p className="mt-1 truncate text-xs text-quiet-text-tertiary" title={externalURL}>{externalURL}</p>}
                        </div>
                        {externalURL && <a href={externalURL} target="_blank" rel="noopener noreferrer" className="shrink-0 p-1 text-quiet-text-tertiary hover:text-quiet-accent focus-visible:outline-2 focus-visible:outline-ring" aria-label={`Open ${item.title}`}><ArrowUpRight01Icon className="h-4 w-4" aria-hidden="true" /></a>}
                      </li>
                    );
                  })}
                </ul>
              )}
            </div>
          </>
        )}
      </SheetContent>
    </Sheet>
  );
}
