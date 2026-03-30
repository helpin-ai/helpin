import { ExternalLink, X } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { Favicon } from '@/components/ui/favicon';
import { Skeleton } from '@/components/ui/skeleton';
import { useSupportContentSourcePage } from '@/hooks/queries/useSupport';

export function PageContentPreview({
  workspaceId,
  contentSourceId,
  pageId,
  onBack,
}: {
  workspaceId: string;
  contentSourceId: string;
  pageId: string;
  onBack: () => void;
}) {
  const { data: page, isLoading } = useSupportContentSourcePage(workspaceId, contentSourceId, pageId);

  return (
    <div className="flex h-full flex-col">
      <div className="flex items-center gap-3 border-b border-border/70 px-6 py-3">
        <Button type="button" size="icon" variant="ghost" className="h-7 w-7 shrink-0" onClick={onBack} title="Back to pages">
          <X className="h-4 w-4" />
        </Button>
        <div className="min-w-0 flex-1">
          <div className="flex items-center gap-2">
            <Favicon
              url={page?.url}
              name={page?.title || page?.url}
              size={16}
              className="h-4 w-4 rounded-sm border-none bg-transparent"
              fallbackClassName="text-[8px]"
            />
            <p className="truncate text-sm font-medium">{page?.title || 'Loading…'}</p>
          </div>
          {page?.url && (
            <a
              href={page.url}
              target="_blank"
              rel="noreferrer"
              className="inline-flex items-center gap-1 text-xs text-muted-foreground transition-colors hover:text-foreground"
            >
              <ExternalLink className="h-3 w-3" />
              <span className="max-w-[400px] truncate">{page.url}</span>
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
