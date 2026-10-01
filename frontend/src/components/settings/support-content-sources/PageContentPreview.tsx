import { LinkSquare01Icon, ArrowLeft01Icon } from '@/lib/icons';
import { Button } from '@/components/ui/button';
import { Favicon } from '@/components/ui/favicon';
import { Skeleton } from '@/components/ui/skeleton';
import { useSupportContentSourcePage } from '@/hooks/queries/useSupport';

export function PageContentPreview({
  workspaceId,
  contentSourceId,
  pageId,
  onBack,
  backLabel = 'Back to pages',
}: {
  workspaceId: string;
  contentSourceId: string;
  pageId: string;
  onBack: () => void;
  backLabel?: string;
}) {
  const { data: page, isLoading, isError, refetch } = useSupportContentSourcePage(workspaceId, contentSourceId, pageId);
  const externalURL = /^https?:\/\//i.test(page?.url ?? '') ? page?.url : undefined;

  return (
    <div className="flex h-full flex-col">
      <div className="flex items-center gap-3 border-b border-border/70 px-6 py-3">
        <Button type="button" size="icon" variant="ghost" className="h-7 w-7 shrink-0" onClick={onBack} aria-label={backLabel} title={backLabel}>
          <ArrowLeft01Icon className="h-4 w-4" />
        </Button>
        <div className="min-w-0 flex-1">
          <div className="flex items-center gap-2">
            <Favicon
              url={externalURL}
              name={page?.title || page?.url}
              size={16}
              className="h-4 w-4 rounded-sm border-none bg-transparent"
              fallbackClassName="text-[8px]"
            />
            <p className="truncate text-sm font-medium">{page?.title || 'Loading…'}</p>
          </div>
          {externalURL && (
            <a
              href={externalURL}
              target="_blank"
              rel="noreferrer"
              className="inline-flex max-w-full items-center gap-1 text-xs text-muted-foreground transition-colors hover:text-foreground"
            >
              <LinkSquare01Icon className="h-3 w-3 shrink-0" />
              <span className="min-w-0 max-w-[400px] truncate">{externalURL}</span>
            </a>
          )}
        </div>
      </div>

      {isLoading ? (
        <div className="space-y-3 p-6">
          <Skeleton className="h-4 w-3/4 rounded" />
          <Skeleton className="h-4 w-full rounded" />
          <Skeleton className="h-4 w-5/6 rounded" />
          <Skeleton className="h-4 w-full rounded" />
          <Skeleton className="h-4 w-2/3 rounded" />
        </div>
      ) : isError ? (
        <div className="space-y-3 px-6 py-8" role="alert">
          <p className="text-sm text-muted-foreground">Could not load this page.</p>
          <Button type="button" variant="outline" size="sm" onClick={() => void refetch()}>Retry</Button>
        </div>
      ) : page?.content_text ? (
        <div className="flex-1 overflow-auto p-6">
          <pre className="whitespace-pre-wrap break-words font-mono text-sm leading-relaxed text-foreground/90">
            {page.content_text}
          </pre>
        </div>
      ) : (
        <div className="px-6 py-12 text-center">
          <p className="text-sm text-muted-foreground">No content available for this page.</p>
        </div>
      )}
    </div>
  );
}
