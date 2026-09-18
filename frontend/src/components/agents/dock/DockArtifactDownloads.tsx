import { useEffect, useMemo, useState } from 'react';
import { toast } from 'sonner';

import { Download04Icon, File01Icon, Loading01Icon, LockKeyIcon } from '@/lib/icons';
import type { AgentRunArtifact } from '@/lib/pmTypes';
import { automationService } from '@/lib/services/automationService';
import { cn } from '@/lib/utils';

interface ArtifactMetadata {
  file_name?: string;
  content_type?: string;
  size_bytes?: number;
}

function artifactMetadata(artifact: AgentRunArtifact): ArtifactMetadata {
  if (!artifact.metadata || typeof artifact.metadata !== 'object') return {};
  return artifact.metadata as ArtifactMetadata;
}

function formatBytes(bytes: number | undefined): string {
  if (!bytes || bytes < 1) return '';
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1024 * 1024) return `${Math.round(bytes / 1024)} KB`;
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
}

function ArtifactDownload({ workspaceId, artifact }: { workspaceId: string; artifact: AgentRunArtifact }) {
  const metadata = artifactMetadata(artifact);
  const name = metadata.file_name?.trim() || `output.${artifact.format || 'file'}`;
  const contentType = metadata.content_type?.trim() || '';
  const isPNG = artifact.format === 'png' || contentType === 'image/png';
  const [previewURL, setPreviewURL] = useState<string | null>(null);
  const [downloading, setDownloading] = useState(false);

  useEffect(() => {
    if (!isPNG) return undefined;
    let active = true;
    void automationService.getArtifactContentURL(workspaceId, artifact.id).then((response) => {
      if (active && response.data?.url) setPreviewURL(response.data.url);
    });
    return () => { active = false; };
  }, [artifact.id, isPNG, workspaceId]);

  const download = async () => {
    if (downloading) return;
    const pendingWindow = window.open('about:blank', '_blank');
    if (pendingWindow) pendingWindow.opener = null;
    setDownloading(true);
    const response = await automationService.getArtifactContentURL(workspaceId, artifact.id);
    setDownloading(false);
    if (response.error || !response.data?.url) {
      pendingWindow?.close();
      toast.error(response.error || `Unable to download ${name}`);
      return;
    }
    if (pendingWindow) pendingWindow.location.replace(response.data.url);
    else window.open(response.data.url, '_blank', 'noopener,noreferrer');
  };

  return (
    <article className={cn('group min-w-0 border-t border-border/50 py-2 first:border-t-0', isPNG && 'pt-2.5')}>
      {isPNG && previewURL ? (
        <button type="button" className="mb-2 block w-full overflow-hidden rounded-md bg-muted/40" onClick={() => void download()}>
          <img src={previewURL} alt={name} className="max-h-52 w-full object-contain" />
        </button>
      ) : null}
      <button
        type="button"
        className="flex w-full min-w-0 items-center gap-2 text-left text-sm outline-none transition-colors hover:text-primary focus-visible:text-primary"
        onClick={() => void download()}
        aria-label={`Download ${name}`}
      >
        <File01Icon className="h-4 w-4 shrink-0 text-muted-foreground" aria-hidden="true" />
        <span className="min-w-0 flex-1">
          <span className="block truncate font-medium text-foreground" title={name}>{name}</span>
          <span className="block text-[11px] uppercase tracking-wide text-muted-foreground">
            {[artifact.format, formatBytes(metadata.size_bytes)].filter(Boolean).join(' · ')}
          </span>
        </span>
        {downloading
          ? <Loading01Icon className="h-4 w-4 shrink-0 animate-spin text-muted-foreground" aria-hidden="true" />
          : <Download04Icon className="h-4 w-4 shrink-0 text-muted-foreground transition-colors group-hover:text-primary" aria-hidden="true" />}
      </button>
    </article>
  );
}

export function DockArtifactDownloads({ workspaceId, artifacts }: { workspaceId: string; artifacts: AgentRunArtifact[] }) {
  const downloads = useMemo(() => artifacts.filter((artifact) => (
    artifact.storage_mode === 'object'
    && ['analysis_output', 'browser_screenshot', 'browser_recording'].includes(artifact.artifact_type)
  )), [artifacts]);
  if (downloads.length === 0) return null;

  return (
    <section className="mt-3 border-y border-border/60 py-1" data-agent-artifact-downloads>
      <div className="flex items-center justify-between px-0.5 py-1.5">
        <span className="text-[11px] font-semibold uppercase tracking-wide text-muted-foreground">
          {downloads.length === 1 ? 'File' : `${downloads.length} files`}
        </span>
        <span className="inline-flex items-center gap-1 text-[11px] text-muted-foreground" title="Only workspace members with access to this conversation can download these files">
          <LockKeyIcon className="h-3 w-3" aria-hidden="true" /> Private
        </span>
      </div>
      <div>
        {downloads.map((artifact) => <ArtifactDownload key={artifact.id} workspaceId={workspaceId} artifact={artifact} />)}
      </div>
    </section>
  );
}
