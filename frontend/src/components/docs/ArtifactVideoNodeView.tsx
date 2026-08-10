import { useEffect, useState } from 'react';
import { NodeViewWrapper, type NodeViewProps } from '@tiptap/react';
import { Delete01Icon, Download04Icon, PlayCircleIcon } from '@/lib/icons';
import { QuickTooltip } from '@/components/ui/quick-tooltip';
import { parseHelpinReference } from '@/lib/helpinReferences';
import { automationService } from '@/lib/services/automationService';

interface MediaState {
  key: string;
  url: string | null;
  error: boolean;
}

export function ArtifactVideoNodeView({ node, deleteNode, editor, extension }: NodeViewProps) {
  const { src, artifactId, fileName, contentType, description, caption } = node.attrs;
  const workspaceId = extension.options.workspaceId as string | undefined;
  const reference = parseHelpinReference(typeof src === 'string' ? src : undefined);
  const resolvedArtifactId = typeof artifactId === 'string' && artifactId.trim()
    ? artifactId.trim()
    : reference?.type === 'artifacts' ? reference.id : '';
  const [refresh, setRefresh] = useState(0);
  const mediaKey = workspaceId && resolvedArtifactId ? `${workspaceId}/${resolvedArtifactId}/${refresh}` : '';
  const [mediaState, setMediaState] = useState<MediaState>({ key: '', url: null, error: false });
  const mediaURL = mediaState.key === mediaKey ? mediaState.url : null;
  const mediaError = mediaState.key === mediaKey && mediaState.error;

  useEffect(() => {
    if (!mediaKey || !workspaceId || !resolvedArtifactId) return;
    let cancelled = false;
    let refreshTimer: number | undefined;
    void automationService.getArtifactContentURL(workspaceId, resolvedArtifactId).then((response) => {
      if (cancelled) return;
      if (response.error || !response.data?.url) {
        setMediaState({ key: mediaKey, url: null, error: true });
        return;
      }
      setMediaState({ key: mediaKey, url: response.data.url, error: false });
      const expiresAt = Date.parse(response.data.expires_at);
      if (Number.isFinite(expiresAt)) {
        refreshTimer = window.setTimeout(
          () => setRefresh((value) => value + 1),
          Math.max(30_000, expiresAt - Date.now() - 60_000),
        );
      }
    });
    return () => {
      cancelled = true;
      if (refreshTimer !== undefined) window.clearTimeout(refreshTimer);
    };
  }, [mediaKey, resolvedArtifactId, workspaceId]);

  const label = typeof description === 'string' && description.trim() ? description : 'Private video recording';
  const name = typeof fileName === 'string' && fileName.trim() ? fileName : 'browser-recording.mp4';

  return (
    <NodeViewWrapper data-artifact-video-wrapper>
      <figure className="group/artifact-video relative my-6 overflow-hidden rounded-lg border border-border bg-muted/20" contentEditable={false}>
        {mediaURL ? (
          <video
            src={mediaURL}
            controls
            preload="metadata"
            aria-label={label}
            className="max-h-[34rem] w-full bg-black"
            onError={() => setMediaState({ key: mediaKey, url: null, error: true })}
          >
            Video playback is not supported by this browser.
          </video>
        ) : (
          <div className="flex min-h-48 flex-col items-center justify-center gap-2 px-4 py-8 text-center text-sm text-muted-foreground">
            <PlayCircleIcon className="h-8 w-8" />
            <span>{mediaError || !mediaKey ? 'Private recording is unavailable' : 'Loading private recording…'}</span>
          </div>
        )}
        {caption && <figcaption className="border-t border-border px-3 py-2 text-center text-sm text-muted-foreground">{caption}</figcaption>}
        {editor.isEditable && (
          <div className="absolute right-2 top-2 flex items-center rounded-lg border bg-popover/95 opacity-0 shadow-md backdrop-blur-sm transition-opacity group-hover/artifact-video:opacity-100">
            <span className="max-w-48 truncate px-2 text-xs text-muted-foreground">{name}</span>
            {mediaURL && (
              <QuickTooltip label="Open recording">
                <a href={mediaURL} target="_blank" rel="noreferrer" className="flex h-8 w-8 items-center justify-center text-muted-foreground hover:text-foreground">
                  <Download04Icon className="h-3.5 w-3.5" />
                </a>
              </QuickTooltip>
            )}
            <QuickTooltip label="Delete">
              <button type="button" onClick={() => deleteNode()} className="flex h-8 w-8 items-center justify-center text-muted-foreground hover:text-destructive">
                <Delete01Icon className="h-3.5 w-3.5" />
              </button>
            </QuickTooltip>
          </div>
        )}
        <span className="sr-only">{typeof contentType === 'string' ? contentType : 'video/mp4'}</span>
      </figure>
    </NodeViewWrapper>
  );
}
