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
    <div className="space-y-2">
      <div className="flex items-center gap-1.5">
        <Link01Icon className="h-3.5 w-3.5 text-muted-foreground" />
        <h3 className="text-xs font-semibold text-muted-foreground uppercase tracking-wide">External Links</h3>
      </div>

      {/* Link list */}
      {links.length > 0 && (
        <div className="space-y-0.5">
          {links.map((link) => (
            <div
              key={link.id}
              className="group flex items-center gap-2 rounded-md px-1 py-1.5 hover:bg-accent/50 transition-colors"
            >
              <Favicon
                url={link.url}
                name={link.title || getHostname(link.url)}
                size={16}
                className="h-4 w-4 rounded-sm border-none bg-transparent"
                fallbackClassName="text-[8px]"
              />
              <div className="min-w-0 flex-1">
                <span className="text-sm font-medium text-foreground truncate block">
                  {link.title || getHostname(link.url)}
                </span>
                <span className="text-[11px] text-muted-foreground truncate block">{link.url}</span>
              </div>
              <a
                href={link.url}
                target="_blank"
                rel="noopener noreferrer"
                className="h-5 w-5 shrink-0 flex items-center justify-center rounded text-muted-foreground hover:text-foreground transition-colors"
              >
                <LinkSquare01Icon className="h-3 w-3" />
              </a>
              <button
                type="button"
                className="h-5 w-5 shrink-0 flex items-center justify-center rounded opacity-0 group-hover:opacity-100 transition-opacity text-muted-foreground hover:text-destructive cursor-pointer"
                onClick={() => handleDelete(link.id)}
              >
                <Delete01Icon className="h-3 w-3" />
              </button>
            </div>
          ))}
        </div>
      )}

      {/* Add input */}
      <div className="flex items-center gap-2">
        <PlusSignIcon className="h-3.5 w-3.5 shrink-0 text-muted-foreground" />
        <input
          ref={inputRef}
          type="url"
          value={newUrl}
          placeholder="Paste a URL..."
          className="flex-1 bg-transparent text-sm placeholder:text-muted-foreground/50 focus:outline-none"
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
            className="h-6 px-2 text-xs"
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
