import { useEffect, useState } from 'react';
import { createPortal } from 'react-dom';
import { ArrowLeft01Icon, ArrowRight01Icon, Cancel01Icon } from '@/lib/icons';
import { SupportImageDownload } from './SupportImageDownload';
import { SupportAttachmentImage } from './SupportAttachmentImage';
import type { SupportAttachmentPayload } from '@/lib/pmTypes';
import { normalizeIndex } from './attachmentLightbox';

interface SupportAttachmentLightboxProps {
  attachments: SupportAttachmentPayload[];
  index: number;
  onIndexChange: (index: number) => void;
  onClose: () => void;
  loadImageContent?: (id: string) => Promise<string>;
}

function LightboxVideo({ attachment }: { attachment: SupportAttachmentPayload }) {
  const [failed, setFailed] = useState(false);
  return (
    <div className="flex flex-col items-center gap-3" onClick={(event) => event.stopPropagation()}>
      <video
        controls
        onErrorCapture={() => setFailed(true)}
        playsInline
        preload="metadata"
        aria-label={`Play ${attachment.file_name}`}
        className="max-h-[82vh] max-w-[88vw] rounded-lg bg-black"
      >
        <source src={attachment.url} type={attachment.file_type} />
      </video>
      {failed && (
        <p role="status" className="text-sm text-white/75">This video cannot be played here. Download it to watch.</p>
      )}
    </div>
  );
}

export function SupportAttachmentLightbox({
  attachments,
  index,
  onIndexChange,
  onClose,
  loadImageContent,
}: SupportAttachmentLightboxProps) {
  const attachment = attachments[index] ?? null;
  const hasMultiple = attachments.length > 1;

  useEffect(() => {
    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key === 'Escape') onClose();
      // Arrow keys seek a focused video instead of switching files.
      if (event.target instanceof HTMLMediaElement) return;
      if (event.key === 'ArrowLeft') onIndexChange(normalizeIndex(index - 1, attachments.length));
      if (event.key === 'ArrowRight') onIndexChange(normalizeIndex(index + 1, attachments.length));
    };
    window.addEventListener('keydown', onKeyDown);
    return () => window.removeEventListener('keydown', onKeyDown);
  }, [attachments.length, index, onClose, onIndexChange]);

  if (!attachment) return null;

  const isVideo = attachment.file_type.startsWith('video/');
  const isPDF = attachment.file_type === 'application/pdf';

  return createPortal(
    <div
      data-testid="support-attachment-lightbox"
      className="fixed inset-0 z-[9999] flex items-center justify-center bg-black/85 p-5 backdrop-blur-sm animate-in fade-in duration-150"
      onClick={onClose}
    >
      <div className="absolute inset-x-4 top-4 flex items-center justify-between gap-3 text-white">
        <div className="min-w-0">
          <div className="truncate text-sm font-medium">{attachment.file_name}</div>
          <div className="text-xs text-white/65">
            {index + 1} / {attachments.length}
          </div>
        </div>
        <div className="flex items-center gap-2">
          <SupportImageDownload
            key={attachment.id}
            attachment={attachment}
            loadImageContent={loadImageContent}
            className="flex h-9 w-9 items-center justify-center rounded-full bg-white/15 transition-colors hover:bg-white/25"
            iconClassName="h-4 w-4"
          />
          <button
            type="button"
            onClick={(event) => {
              event.stopPropagation();
              onClose();
            }}
            className="flex h-9 w-9 items-center justify-center rounded-full bg-white/15 transition-colors hover:bg-white/25"
            aria-label="Close preview"
          >
            <Cancel01Icon className="h-5 w-5" />
          </button>
        </div>
      </div>

      {hasMultiple && (
        <button
          type="button"
          onClick={(event) => {
            event.stopPropagation();
            onIndexChange(normalizeIndex(index - 1, attachments.length));
          }}
          className="absolute left-4 top-1/2 flex h-10 w-10 -translate-y-1/2 items-center justify-center rounded-full bg-white/15 text-white transition-colors hover:bg-white/25"
          aria-label="Previous attachment"
        >
          <ArrowLeft01Icon className="h-6 w-6" />
        </button>
      )}

      {isVideo ? (
        <LightboxVideo key={attachment.id} attachment={attachment} />
      ) : isPDF ? (
        <iframe
          key={attachment.id}
          src={attachment.url}
          title={attachment.file_name}
          className="h-[82vh] w-[min(88vw,64rem)] rounded-lg bg-white"
        />
      ) : (
        <SupportAttachmentImage
          attachment={attachment}
          loadFallback={loadImageContent}
          className="h-[82vh] w-[88vw] text-white"
        />
      )}

      {hasMultiple && (
        <button
          type="button"
          onClick={(event) => {
            event.stopPropagation();
            onIndexChange(normalizeIndex(index + 1, attachments.length));
          }}
          className="absolute right-4 top-1/2 flex h-10 w-10 -translate-y-1/2 items-center justify-center rounded-full bg-white/15 text-white transition-colors hover:bg-white/25"
          aria-label="Next attachment"
        >
          <ArrowRight01Icon className="h-6 w-6" />
        </button>
      )}
    </div>,
    document.body,
  );
}
