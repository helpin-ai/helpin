import { useEffect, useState } from 'react';

import { Image01Icon } from '@/lib/icons';
import { automationService } from '@/lib/services/automationService';
import { useWorkspaceStore } from '@/stores/workspaceStore';

/**
 * Renders a private image artifact (a generated image or screenshot) inline in
 * agent chat. The image loads through a short-lived signed URL and opens at
 * full size in a new tab. Spans keep the markup valid inside paragraphs.
 */
export function ArtifactInlineImage({ artifactId, alt }: { artifactId: string; alt: string }) {
  const workspaceId = useWorkspaceStore((state) => state.currentWorkspace?.id);
  const key = `${workspaceId ?? ''}/${artifactId}`;
  const [state, setState] = useState<{ key: string; url: string | null; error: boolean }>({ key: '', url: null, error: false });
  const url = state.key === key ? state.url : null;
  const error = state.key === key && state.error;
  const label = alt.trim() || 'Generated image';

  useEffect(() => {
    if (!workspaceId) return undefined;
    let cancelled = false;
    void automationService.getArtifactContentURL(workspaceId, artifactId).then((response) => {
      if (cancelled) return;
      setState(response.error || !response.data?.url
        ? { key, url: null, error: true }
        : { key, url: response.data.url, error: false });
    });
    return () => { cancelled = true; };
  }, [artifactId, key, workspaceId]);

  if (error) {
    return (
      <span className="my-2 flex items-center gap-2 rounded-md border border-border/60 bg-muted/30 px-3 py-2 text-xs text-muted-foreground">
        <Image01Icon className="h-4 w-4 shrink-0" aria-hidden="true" />
        {label} is unavailable
      </span>
    );
  }
  if (!url) {
    return (
      <span
        className="my-2 flex aspect-[3/2] max-h-80 w-full max-w-md animate-pulse items-center justify-center rounded-md border border-border/60 bg-muted/40"
        role="img"
        aria-label={`Loading ${label}`}
      >
        <Image01Icon className="h-5 w-5 text-muted-foreground/60" aria-hidden="true" />
      </span>
    );
  }
  return (
    <a
      href={url}
      target="_blank"
      rel="noreferrer"
      className="my-2 block w-fit max-w-full overflow-hidden rounded-md border border-border/60 bg-muted/20"
      title={`Open ${label} at full size`}
      data-agent-inline-image
    >
      <img
        src={url}
        alt={label}
        className="block max-h-96 max-w-full object-contain"
        onError={() => setState({ key, url: null, error: true })}
      />
    </a>
  );
}
