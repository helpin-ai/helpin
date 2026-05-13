import { useCallback, useEffect, useRef, useState } from 'react';
import { LinkSquare01Icon, Link01Icon, PlusSignIcon, Delete01Icon } from '@/lib/icons';
import { Button } from '@/components/ui/button';
import { Favicon } from '@/components/ui/favicon';
import { pmExternalLinkService } from '@/lib/services/pmExternalLinkService';
import type { ExternalLink } from '@/lib/pmTypes';

interface ExternalLinksProps {
  workspaceId: string;
  entityType: 'task' | 'epic';
  entityId: string;
}

function getHostname(url: string): string {
  try {
    return new URL(url).hostname.replace(/^www\./, '');
  } catch {
    return url;
  }
}

export function ExternalLinks({ workspaceId, entityType, entityId }: ExternalLinksProps) {
  const [links, setLinks] = useState<ExternalLink[]>([]);
  const [newUrl, setNewUrl] = useState('');
  const [adding, setAdding] = useState(false);
  const inputRef = useRef<HTMLInputElement>(null);
  const linkCountLabel = `${links.length} ${links.length === 1 ? 'link' : 'links'}`;

  const reload = useCallback(async () => {
    const { data } = await pmExternalLinkService.listByEntity(workspaceId, entityType, entityId);
    setLinks(data ?? []);
  }, [workspaceId, entityType, entityId]);

  useEffect(() => { reload(); }, [reload]);

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
    inputRef.current?.focus();
  }, [workspaceId, entityType, entityId, newUrl]);

  const handleDelete = useCallback(
    async (id: string) => {
      setLinks((prev) => prev.filter((l) => l.id !== id));
      await pmExternalLinkService.remove(workspaceId, id);
    },
    [workspaceId],
  );

  return (
    <div className="space-y-3">
      <div className="flex items-center justify-between gap-3">
        <div className="flex items-center gap-1.5">
          <Link01Icon className="h-3.5 w-3.5 text-muted-foreground" />
          <h3 className="text-xs font-semibold text-muted-foreground uppercase tracking-wide">External Links</h3>
        </div>
        <span className="rounded-full bg-muted px-2 py-0.5 text-[11px] font-medium text-muted-foreground">
          {linkCountLabel}
        </span>
      </div>

      {links.length > 0 && (
        <div className="space-y-1.5">
          {links.map((link) => (
            <div
              key={link.id}
              data-testid="external-link-row"
              className="group flex items-center gap-2 rounded-lg border border-border/50 bg-muted/35 px-2.5 py-2 transition-colors hover:border-border hover:bg-muted/55"
            >
              <Favicon
                url={link.url}
                name={link.title || getHostname(link.url)}
                size={16}
                className="h-4 w-4 shrink-0 rounded-sm border-none bg-transparent"
                fallbackClassName="text-[8px]"
              />
              <div className="min-w-0 flex-1">
                <span className="text-sm font-medium text-foreground truncate block">
                  {link.title || getHostname(link.url)}
                </span>
                <span className="text-[11px] text-muted-foreground truncate block">
                  {getHostname(link.url)} - {link.url}
                </span>
              </div>
              <a
                href={link.url}
                target="_blank"
                rel="noopener noreferrer"
                aria-label={`Open ${link.title || getHostname(link.url)}`}
                className="h-7 w-7 shrink-0 flex items-center justify-center rounded-md text-muted-foreground transition-colors hover:bg-background hover:text-foreground"
              >
                <LinkSquare01Icon className="h-3.5 w-3.5" />
              </a>
              <button
                type="button"
                aria-label={`Remove ${link.title || getHostname(link.url)}`}
                className="h-7 w-7 shrink-0 flex items-center justify-center rounded-md text-muted-foreground opacity-70 transition-colors hover:bg-background hover:text-destructive group-hover:opacity-100 cursor-pointer"
                onClick={() => handleDelete(link.id)}
              >
                <Delete01Icon className="h-3.5 w-3.5" />
              </button>
            </div>
          ))}
        </div>
      )}
      {links.length === 0 && (
        <div className="rounded-lg border border-dashed border-border/70 bg-muted/20 px-3 py-2 text-xs text-muted-foreground">
          No external links yet.
        </div>
      )}

      <div className="flex items-center gap-2 rounded-lg border border-border/60 bg-background px-2.5 py-2 transition-colors focus-within:border-primary/40 focus-within:ring-2 focus-within:ring-primary/10">
        <PlusSignIcon className="h-3.5 w-3.5 shrink-0 text-muted-foreground" />
        <input
          ref={inputRef}
          type="url"
          value={newUrl}
          placeholder="Paste a URL..."
          className="min-w-0 flex-1 bg-transparent text-sm placeholder:text-muted-foreground/50 focus:outline-none"
          disabled={adding}
          onChange={(e) => setNewUrl(e.target.value)}
          onKeyDown={(e) => {
            if (e.key === 'Enter') {
              e.preventDefault();
              handleAdd();
            }
          }}
        />
        {newUrl.trim() && (
          <Button
            variant="ghost"
            size="sm"
            className="h-6 shrink-0 px-2 text-xs"
            disabled={adding}
            onClick={handleAdd}
          >
            Add
          </Button>
        )}
      </div>
    </div>
  );
}
