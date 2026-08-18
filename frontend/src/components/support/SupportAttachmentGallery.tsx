import { useCallback, useEffect, useMemo, useState } from 'react';
import { createPortal } from 'react-dom';
import { ArrowLeft01Icon, ArrowRight01Icon, AttachmentIcon, Cancel01Icon, Download04Icon } from '@/lib/icons';
import type { SupportAttachmentPayload } from '@/lib/pmTypes';

type SupportAttachmentGalleryTone = 'default' | 'note';
type SupportAttachmentThumbnailSize = 'sm' | 'md';

interface SupportAttachmentGalleryProps {
  attachments: SupportAttachmentPayload[];
  tone?: SupportAttachmentGalleryTone;
  thumbnailSize?: SupportAttachmentThumbnailSize;
  className?: string;
}

function formatFileSize(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(0)} KB`;
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
}

function isImageAttachment(attachment: SupportAttachmentPayload): boolean {
  return attachment.file_type.startsWith('image/') && !!attachment.url;
}

function normalizeIndex(index: number, length: number): number {
  if (length <= 0) return 0;
  return ((index % length) + length) % length;
}

export function SupportAttachmentGallery({
  attachments,
  tone = 'default',
  thumbnailSize = 'sm',
  className = '',
}: SupportAttachmentGalleryProps) {
  const imageAttachments = useMemo(() => attachments.filter(isImageAttachment), [attachments]);
  const fileAttachments = useMemo(() => attachments.filter((attachment) => !isImageAttachment(attachment)), [attachments]);
  const [hoverIndex, setHoverIndex] = useState<number | null>(null);
  const [lightboxIndex, setLightboxIndex] = useState<number | null>(null);

  const hoverAttachment = hoverIndex === null ? null : imageAttachments[hoverIndex] ?? null;
  const lightboxAttachment = lightboxIndex === null ? null : imageAttachments[lightboxIndex] ?? null;
  const hasMultipleImages = imageAttachments.length > 1;

  const setAdjacentHover = useCallback((direction: -1 | 1) => {
    setHoverIndex((current) => normalizeIndex((current ?? 0) + direction, imageAttachments.length));
  }, [imageAttachments.length]);

  const setAdjacentLightbox = useCallback((direction: -1 | 1) => {
    setLightboxIndex((current) => normalizeIndex((current ?? 0) + direction, imageAttachments.length));
  }, [imageAttachments.length]);

  useEffect(() => {
    if (lightboxIndex === null) return undefined;
    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key === 'Escape') setLightboxIndex(null);
      if (event.key === 'ArrowLeft') setAdjacentLightbox(-1);
      if (event.key === 'ArrowRight') setAdjacentLightbox(1);
    };
    window.addEventListener('keydown', onKeyDown);
    return () => window.removeEventListener('keydown', onKeyDown);
  }, [lightboxIndex, setAdjacentLightbox]);

  if (attachments.length === 0) return null;

  const thumbnailClassName = thumbnailSize === 'md' ? 'h-20 w-20' : 'h-16 w-16';
  const fileLinkClassName = tone === 'note'
    ? 'flex items-center gap-2 rounded-lg border border-amber-200 bg-amber-100/40 px-3 py-2 text-xs text-amber-900 transition-colors hover:bg-amber-100 dark:border-amber-800/70 dark:bg-amber-950/30 dark:text-amber-100 dark:hover:bg-amber-900/30'
    : 'flex items-center gap-2 rounded-lg border border-border px-3 py-2 text-xs text-foreground transition-colors hover:bg-muted/50';

  const navButtonClassName = 'flex h-7 w-7 items-center justify-center rounded-full bg-background/90 text-foreground shadow-sm ring-1 ring-border transition-colors hover:bg-background';

  const lightboxPortal = lightboxAttachment ? createPortal(
    <div
      data-testid="support-attachment-lightbox"
      className="fixed inset-0 z-[9999] flex items-center justify-center bg-black/85 p-5 backdrop-blur-sm animate-in fade-in duration-150"
      onClick={() => setLightboxIndex(null)}
    >
      <div className="absolute inset-x-4 top-4 flex items-center justify-between gap-3 text-white">
        <div className="min-w-0">
          <div className="truncate text-sm font-medium">{lightboxAttachment.file_name}</div>
          <div className="text-xs text-white/65">
            {lightboxIndex! + 1} / {imageAttachments.length}
          </div>
        </div>
        <div className="flex items-center gap-2">
          <a
            href={lightboxAttachment.url}
            target="_blank"
            rel="noopener noreferrer"
            className="flex h-9 w-9 items-center justify-center rounded-full bg-white/15 transition-colors hover:bg-white/25"
            onClick={(event) => event.stopPropagation()}
            aria-label="Download image attachment"
          >
            <Download04Icon className="h-4 w-4" />
          </a>
          <button
            type="button"
            onClick={(event) => {
              event.stopPropagation();
              setLightboxIndex(null);
            }}
            className="flex h-9 w-9 items-center justify-center rounded-full bg-white/15 transition-colors hover:bg-white/25"
            aria-label="Close preview"
          >
            <Cancel01Icon className="h-5 w-5" />
          </button>
        </div>
      </div>

      {hasMultipleImages && (
        <button
          type="button"
          onClick={(event) => {
            event.stopPropagation();
            setAdjacentLightbox(-1);
          }}
          className="absolute left-4 top-1/2 flex h-10 w-10 -translate-y-1/2 items-center justify-center rounded-full bg-white/15 text-white transition-colors hover:bg-white/25"
          aria-label="Previous image attachment"
        >
          <ArrowLeft01Icon className="h-6 w-6" />
        </button>
      )}

      <img
        src={lightboxAttachment.url}
        alt={lightboxAttachment.file_name}
        className="max-h-[82vh] max-w-[88vw] rounded-lg object-contain shadow-2xl"
        onClick={(event) => event.stopPropagation()}
      />

      {hasMultipleImages && (
        <button
          type="button"
          onClick={(event) => {
            event.stopPropagation();
            setAdjacentLightbox(1);
          }}
          className="absolute right-4 top-1/2 flex h-10 w-10 -translate-y-1/2 items-center justify-center rounded-full bg-white/15 text-white transition-colors hover:bg-white/25"
          aria-label="Next image attachment"
        >
          <ArrowRight01Icon className="h-6 w-6" />
        </button>
      )}
    </div>,
    document.body,
  ) : null;

  return (
    <>
      <div className={`${className} space-y-2`.trim()} onMouseLeave={() => setHoverIndex(null)}>
        {imageAttachments.length > 0 && (
          <div className="relative">
            <div className="flex flex-wrap gap-1.5">
              {imageAttachments.map((attachment, index) => (
                <button
                  key={attachment.id}
                  type="button"
                  onMouseEnter={() => setHoverIndex(index)}
                  onMouseOver={() => setHoverIndex(index)}
                  onFocus={() => setHoverIndex(index)}
                  onClick={() => setLightboxIndex(index)}
                  className={`${thumbnailClassName} group relative shrink-0 cursor-zoom-in overflow-hidden rounded-lg border border-border/70 bg-muted/40 transition-colors hover:border-border focus:outline-none focus:ring-2 focus:ring-ring/40`}
                  aria-label={`Preview ${attachment.file_name}`}
                >
                  <img
                    src={attachment.url}
                    alt={attachment.file_name}
                    className="h-full w-full object-cover"
                    loading="lazy"
                  />
                  <span className="pointer-events-none absolute inset-x-0 bottom-0 truncate bg-black/55 px-1.5 py-1 text-left text-[10px] font-medium text-white opacity-0 transition-opacity group-hover:opacity-100 group-focus:opacity-100">
                    {attachment.file_name}
                  </span>
                </button>
              ))}
            </div>

            {hoverAttachment && (
              <div
                data-testid="support-attachment-hover-preview"
                className="absolute bottom-full left-0 z-30 w-72 pb-2"
                onMouseEnter={() => {
                  if (hoverIndex === null) setHoverIndex(0);
                }}
              >
                <div className="overflow-hidden rounded-lg border border-border bg-popover text-popover-foreground shadow-xl animate-in fade-in zoom-in-95 duration-100">
                  <div className="relative bg-muted">
                    <img src={hoverAttachment.url} alt={hoverAttachment.file_name} className="h-44 w-full object-contain" />
                    {hasMultipleImages && (
                      <>
                        <button
                          type="button"
                          className={`${navButtonClassName} absolute left-2 top-1/2 -translate-y-1/2`}
                          aria-label="Previous image attachment"
                          onClick={(event) => {
                            event.stopPropagation();
                            setAdjacentHover(-1);
                          }}
                        >
                          <ArrowLeft01Icon className="h-4 w-4" />
                        </button>
                        <button
                          type="button"
                          className={`${navButtonClassName} absolute right-2 top-1/2 -translate-y-1/2`}
                          aria-label="Next image attachment"
                          onClick={(event) => {
                            event.stopPropagation();
                            setAdjacentHover(1);
                          }}
                        >
                          <ArrowRight01Icon className="h-4 w-4" />
                        </button>
                      </>
                    )}
                  </div>
                  <div className="flex items-center gap-2 px-3 py-2 text-xs">
                    <span className="min-w-0 flex-1 truncate font-medium">{hoverAttachment.file_name}</span>
                    <span className="shrink-0 text-muted-foreground">
                      {(hoverIndex ?? 0) + 1} / {imageAttachments.length}
                    </span>
                    <a
                      href={hoverAttachment.url}
                      target="_blank"
                      rel="noopener noreferrer"
                      className="shrink-0 text-muted-foreground transition-colors hover:text-foreground"
                      aria-label="Download image attachment"
                      onClick={(event) => event.stopPropagation()}
                    >
                      <Download04Icon className="h-3.5 w-3.5" />
                    </a>
                  </div>
                </div>
              </div>
            )}
          </div>
        )}

        {fileAttachments.length > 0 && (
          <div className="space-y-1.5">
            {fileAttachments.map((attachment) => (
              <a
                key={attachment.id}
                href={attachment.url}
                target="_blank"
                rel="noopener noreferrer"
                className={fileLinkClassName}
              >
                <AttachmentIcon className="h-3.5 w-3.5 shrink-0 opacity-60" />
                <span className="truncate font-medium">{attachment.file_name}</span>
                <span className="shrink-0 opacity-60">{formatFileSize(attachment.file_size)}</span>
                <Download04Icon className="ml-auto h-3.5 w-3.5 shrink-0 opacity-60" />
              </a>
            ))}
          </div>
        )}
      </div>
      {lightboxPortal}
    </>
  );
}
