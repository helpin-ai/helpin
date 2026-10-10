import { useEffect, useMemo, useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import {
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { ScrollArea } from '@/components/ui/scroll-area';
import { BookOpen01Icon, Loading01Icon } from '@/lib/icons';
import { QuietSearchInput } from '@/components/design-system/quiet';
import { unwrap } from '@/lib/queryUtils';
import { docsService } from '@/lib/services/docsService';
import { queryKeys } from '@/lib/queryKeys';
import type { DocsDocument, DocsHelpcenterConfig } from '@/lib/docsTypes';
import { cn } from '@/lib/utils';
import { normalizeSupportLinkInput } from './supportComposerExtensions';

interface LinkInsertModalProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  workspaceId: string;
  initialLabel?: string;
  initialUrl?: string;
  onInsert: (label: string, url: string) => void;
  onRemove?: () => void;
}

type TabValue = 'external' | 'kb';

function buildArticleUrl(config: DocsHelpcenterConfig | null | undefined, slug: string): string {
  if (!slug) return '';
  const host = config?.custom_domain?.trim() || (config?.subdomain ? `${config.subdomain}.helpin.center` : '');
  if (!host) return `/${slug.replace(/^\/+/, '')}`;
  const cleanSlug = slug.replace(/^\/+/, '');
  return `https://${host}/${cleanSlug}`;
}

export function LinkInsertModal({
  open,
  onOpenChange,
  workspaceId,
  initialLabel = '',
  initialUrl = '',
  onInsert,
  onRemove,
}: LinkInsertModalProps) {
  const [tab, setTab] = useState<TabValue>('external');
  const [label, setLabel] = useState(initialLabel);
  const [url, setUrl] = useState(initialUrl);
  const [search, setSearch] = useState('');
  const [selectedArticleId, setSelectedArticleId] = useState<string | null>(null);

  useEffect(() => {
    if (open) {
      setLabel(initialLabel);
      setUrl(initialUrl);
      setSearch('');
      setSelectedArticleId(null);
      setTab('external');
    }
  }, [open, initialLabel, initialUrl]);

  const { data: helpcenterConfig } = useQuery({
    queryKey: queryKeys.docs.helpcenterConfig(workspaceId),
    queryFn: async () => unwrap(await docsService.getHelpcenterConfig(workspaceId)),
    enabled: open && !!workspaceId,
    staleTime: 5 * 60_000,
  });

  const trimmedSearch = search.trim();
  const { data: searchResults, isLoading: isSearching } = useQuery({
    queryKey: [...queryKeys.docs.search(workspaceId, trimmedSearch), { status: 'published' }],
    queryFn: async () => unwrap(await docsService.search(workspaceId, trimmedSearch, { status: 'published', limit: 25 })),
    enabled: open && tab === 'kb' && !!workspaceId && trimmedSearch.length > 0,
  });

  const { data: publishedDocs, isLoading: isLoadingDocs } = useQuery({
    queryKey: [...queryKeys.docs.documents(workspaceId), { status: 'published' }],
    queryFn: async () => unwrap(await docsService.listDocuments(workspaceId, { status: 'published' })),
    enabled: open && tab === 'kb' && !!workspaceId && trimmedSearch.length === 0,
    staleTime: 60_000,
  });

  const articles: DocsDocument[] = useMemo(() => {
    if (trimmedSearch && searchResults) return searchResults;
    return publishedDocs ?? [];
  }, [trimmedSearch, searchResults, publishedDocs]);

  const selectedArticle = articles.find((a) => a.id === selectedArticleId) ?? null;

  function handleSelectArticle(article: DocsDocument) {
    setSelectedArticleId(article.id);
    setLabel(article.title);
    const slug = article.live_slug || article.hc_slug || '';
    setUrl(buildArticleUrl(helpcenterConfig, slug));
  }

  function handleInsert() {
    const trimmedLabel = label.trim();
    const trimmedUrl = url.trim();
    if (!trimmedLabel || !trimmedUrl) return;
    onInsert(trimmedLabel, normalizeSupportLinkInput(trimmedUrl));
    onOpenChange(false);
  }

  const externalCanInsert = label.trim().length > 0 && url.trim().length > 0;
  const kbCanInsert = !!selectedArticle && url.trim().length > 0;
  const isLoadingArticles = isLoadingDocs || isSearching;

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-[520px]">
        <DialogHeader>
          <DialogTitle>{initialUrl ? 'Edit link' : 'Insert link'}</DialogTitle>
        </DialogHeader>

        <Tabs value={tab} onValueChange={(v) => setTab(v as TabValue)}>
          <TabsList className="grid w-full grid-cols-2">
            <TabsTrigger value="external">External URL</TabsTrigger>
            <TabsTrigger value="kb">Knowledge Base Article</TabsTrigger>
          </TabsList>

          <TabsContent value="external" className="space-y-4 pt-4">
            <div className="space-y-1.5">
              <Label htmlFor="link-label">Link label</Label>
              <Input
                id="link-label"
                value={label}
                onChange={(e) => setLabel(e.target.value)}
                placeholder="What people see"
                autoFocus
              />
            </div>
            <div className="space-y-1.5">
              <Label htmlFor="link-url">Link URL</Label>
              <Input
                id="link-url"
                value={url}
                onChange={(e) => setUrl(e.target.value)}
                placeholder="Enter the link URL..."
                onKeyDown={(e) => {
                  if (e.key === 'Enter' && externalCanInsert) {
                    e.preventDefault();
                    handleInsert();
                  }
                }}
              />
            </div>
          </TabsContent>

          <TabsContent value="kb" className="space-y-3 pt-4">
            <QuietSearchInput
              value={search}
              onChange={(e) => setSearch(e.target.value)}
              placeholder="Search published articles..."
            />

            <ScrollArea className="h-64 rounded-md border">
              {isLoadingArticles ? (
                <div className="flex h-32 items-center justify-center text-muted-foreground">
                  <Loading01Icon className="h-4 w-4 animate-spin" />
                </div>
              ) : articles.length === 0 ? (
                <div className="flex h-32 flex-col items-center justify-center gap-2 text-center text-sm text-muted-foreground">
                  <BookOpen01Icon className="h-5 w-5 opacity-50" />
                  <p>{trimmedSearch ? 'No articles match this search.' : 'No published articles yet.'}</p>
                </div>
              ) : (
                <ul className="divide-y">
                  {articles.map((article) => {
                    const isSelected = selectedArticleId === article.id;
                    const slug = article.live_slug || article.hc_slug || '';
                    return (
                      <li key={article.id}>
                        <button
                          type="button"
                          onClick={() => handleSelectArticle(article)}
                          className={cn(
                            'flex w-full flex-col items-start gap-0.5 px-3 py-2 text-left transition-colors hover:bg-muted/50',
                            isSelected && 'bg-primary/10 hover:bg-primary/15',
                          )}
                        >
                          <span className="text-sm font-medium">{article.title || 'Untitled'}</span>
                          {slug && (
                            <span className="truncate text-[11px] text-muted-foreground">/{slug.replace(/^\/+/, '')}</span>
                          )}
                        </button>
                      </li>
                    );
                  })}
                </ul>
              )}
            </ScrollArea>

            {selectedArticle && (
              <div className="space-y-1.5">
                <Label htmlFor="kb-label">Link label</Label>
                <Input
                  id="kb-label"
                  value={label}
                  onChange={(e) => setLabel(e.target.value)}
                />
              </div>
            )}
          </TabsContent>
        </Tabs>

        <DialogFooter className="gap-2 sm:gap-2">
          {onRemove && initialUrl && (
            <Button
              type="button"
              variant="ghost"
              className="mr-auto text-destructive hover:text-destructive"
              onClick={() => {
                onRemove();
                onOpenChange(false);
              }}
            >
              Remove link
            </Button>
          )}
          <Button type="button" variant="outline" onClick={() => onOpenChange(false)}>
            Cancel
          </Button>
          <Button
            type="button"
            onClick={handleInsert}
            disabled={tab === 'external' ? !externalCanInsert : !kbCanInsert}
          >
            Insert Link
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
