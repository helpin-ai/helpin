import { useCallback, useEffect, useRef, useState, type ReactNode } from 'react';
import { ArrowUpRight01Icon, LinkSquare01Icon, Link01Icon, PlusSignIcon, Delete01Icon, Cancel01Icon } from '@/lib/icons';
import { Button } from '@/components/ui/button';
import { quietRelatedItemTitleClassName } from '@/components/design-system/quiet';
import { Favicon } from '@/components/ui/favicon';
import { QuickTooltip } from '@/components/ui/quick-tooltip';
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip';
import { pmExternalLinkService } from '@/lib/services/pmExternalLinkService';
import type { ExternalLink } from '@/lib/pmTypes';

interface ExternalLinksProps {
  workspaceId: string;
  entityType: 'task' | 'epic';
  entityId: string;
  onContentChange?: (hasContent: boolean) => void;
  onCountChange?: (count: number) => void;
  flat?: boolean;
}

function getHostname(url: string): string {
  try {
    return new URL(url).hostname.replace(/^www\./, '');
  } catch {
    return url;
  }
}

function ExternalLinkTooltip({ children }: { children: ReactNode }) {
  return (
    <Tooltip>
      <TooltipTrigger asChild>{children}</TooltipTrigger>
      <TooltipContent side="top" sideOffset={4} className="z-[1000]">
        Opens in a new tab
      </TooltipContent>
    </Tooltip>
  );
}

export function ExternalLinks({ workspaceId, entityType, entityId, onContentChange, onCountChange, flat = false }: ExternalLinksProps) {
  const [links, setLinks] = useState<ExternalLink[]>([]);
  const [newUrl, setNewUrl] = useState('');
  const [adding, setAdding] = useState(false);
  const [addingLink, setAddingLink] = useState(false);
  const inputRef = useRef<HTMLInputElement>(null);

  const reload = useCallback(async () => {
    const { data } = await pmExternalLinkService.listByEntity(workspaceId, entityType, entityId);
    setLinks(data ?? []);
  }, [workspaceId, entityType, entityId]);

  useEffect(() => {
    let cancelled = false;

    pmExternalLinkService.listByEntity(workspaceId, entityType, entityId).then(({ data }) => {
      if (!cancelled) {
        setLinks(data ?? []);
      }
    });

    return () => {
      cancelled = true;
    };
  }, [workspaceId, entityType, entityId]);

  useEffect(() => {
    onContentChange?.(links.length > 0);
    onCountChange?.(links.length);
  }, [links.length, onContentChange, onCountChange]);

  useEffect(() => {
    if (addingLink) inputRef.current?.focus();
  }, [addingLink]);

  // Re-fetch when another client changes external links
  useEffect(() => {
    const handler = (e: Event) => {
      const d = (e as CustomEvent)?.detail;
      if (d?.parent_id === entityId && d?.entity === 'external_link') reload();
    };
    window.addEventListener('task-child-updated', handler);
    window.addEventListener('epic-child-updated', handler);
    return () => {
      window.removeEventListener('task-child-updated', handler);
      window.removeEventListener('epic-child-updated', handler);
    };
  }, [entityId, reload]);

  const handleAdd = useCallback(async () => {
    const url = newUrl.trim();
    if (!url) return;
    setAdding(true);
    const { data, error } = await pmExternalLinkService.createForEntity(workspaceId, entityType, entityId, { url });
    setAdding(false);
    if (error || !data) return;
    setLinks((prev) => [...prev, data]);
    setNewUrl('');
    setAddingLink(false);
  }, [workspaceId, entityType, entityId, newUrl]);

  const closeAddLinkInput = useCallback(() => {
    setAddingLink(false);
    setNewUrl('');
  }, []);

  const handleDelete = useCallback(
    async (id: string) => {
      setLinks((prev) => prev.filter((l) => l.id !== id));
      await pmExternalLinkService.remove(workspaceId, id);
    },
    [workspaceId],
  );

  return (
    <div className="space-y-3">
      <div className={flat ? undefined : 'rounded-lg border border-border/60 bg-card'}>
        <div className={flat ? 'flex items-center justify-between' : 'flex items-center justify-between border-b border-border/40 px-3 py-2'}>
          <div className="flex items-center gap-1.5">
            {!flat && <Link01Icon className="h-3.5 w-3.5 text-muted-foreground" />}
            <span className="text-[12.5px] font-semibold text-foreground/70 uppercase tracking-wide">
              External Links
            </span>
            {links.length > 0 ? (
              <span className="text-xs font-normal text-muted-foreground">({links.length})</span>
            ) : null}
          </div>
          <button
            type="button"
            className="rounded-md p-0.5 text-muted-foreground transition-colors hover:bg-accent hover:text-foreground"
            onClick={() => setAddingLink(true)}
            aria-label="Add external link"
          >
            <span className="text-sm leading-none">+</span>
          </button>
        </div>

        {links.length > 0 ? (
          <div className="divide-y divide-border/40">
            {links.map((link) => (
              <div
                key={link.id}
                data-testid="external-link-row"
                className={flat ? 'group flex items-center gap-2 px-1 py-1.5 transition-colors hover:bg-muted/30' : 'group flex items-center gap-2 px-3 py-2 transition-colors hover:bg-muted/30'}
              >
                <Favicon
                  url={link.url}
                  name={link.title || getHostname(link.url)}
                  size={14}
                  className="h-3.5 w-3.5 shrink-0 rounded-sm border-none bg-transparent"
                  fallbackClassName="text-[7px]"
                />
                <div className="min-w-0 flex-1">
                  <ExternalLinkTooltip>
                    <a
                      href={link.url}
                      target="_blank"
                      rel="noopener noreferrer"
                      aria-label={`Open ${link.title || getHostname(link.url)} in a new tab`}
                      className={`${quietRelatedItemTitleClassName} inline-flex max-w-full items-center gap-1 transition-colors hover:text-foreground hover:underline hover:decoration-muted-foreground/60 hover:underline-offset-2`}
                    >
                      <span className="min-w-0 truncate">{link.title || getHostname(link.url)}</span>
                      <ArrowUpRight01Icon className="h-3.5 w-3.5 shrink-0 text-muted-foreground transition-colors group-hover:text-foreground" />
                    </a>
                  </ExternalLinkTooltip>
                </div>
                <QuickTooltip label="Remove link">
                  <button
                    type="button"
                    aria-label={`Remove ${link.title || getHostname(link.url)}`}
                    className="h-6 w-6 shrink-0 flex items-center justify-center rounded-md text-muted-foreground opacity-0 transition-[opacity,color,background-color] hover:bg-muted hover:text-destructive group-hover:opacity-100 group-focus-within:opacity-100 cursor-pointer"
                    onClick={() => handleDelete(link.id)}
                  >
                    <Delete01Icon className="h-3.5 w-3.5" />
                  </button>
                </QuickTooltip>
              </div>
            ))}
          </div>
        ) : null}

        {links.length === 0 && !addingLink ? (
          <p className={flat ? 'mt-1 py-0.5 text-xs italic text-muted-foreground' : 'px-3 py-2 text-xs italic text-muted-foreground'}>
            No external links
          </p>
        ) : null}

        {addingLink ? (
          <div className={flat ? 'flex items-center gap-2 px-1 py-1.5' : 'flex items-center gap-2 border-t border-border/40 px-3 py-2'}>
            <LinkSquare01Icon className="h-3.5 w-3.5 shrink-0 text-muted-foreground/50" />
            <input
              ref={inputRef}
              type="url"
              value={newUrl}
              placeholder="https://..."
              className="min-w-0 flex-1 bg-transparent py-1 text-sm placeholder:text-muted-foreground/50 focus:outline-none"
              disabled={adding}
              onChange={(e) => setNewUrl(e.target.value)}
              onKeyDown={(e) => {
                if (e.key === 'Escape') {
                  e.preventDefault();
                  e.stopPropagation();
                  closeAddLinkInput();
                }
                if (e.key === 'Enter') {
                  e.preventDefault();
                  handleAdd();
                }
              }}
            />
            <QuickTooltip label="Close">
              <Button
                type="button"
                variant="ghost"
                size="icon-xs"
                aria-label="Close external link input"
                onClick={closeAddLinkInput}
              >
                <Cancel01Icon />
              </Button>
            </QuickTooltip>
            {newUrl.trim() && (
              <Button
                variant="outline"
                size="xs"
                disabled={adding}
                onClick={handleAdd}
              >
                <PlusSignIcon />
                Add link
              </Button>
            )}
          </div>
        ) : null}
      </div>
    </div>
  );
}
