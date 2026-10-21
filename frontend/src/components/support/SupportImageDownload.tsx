import { useRef, useState } from 'react';
import { toast } from 'sonner';
import { Download04Icon, Loading01Icon } from '@/lib/icons';
import type { SupportAttachmentPayload } from '@/lib/pmTypes';

interface SupportImageDownloadProps {
  attachment: SupportAttachmentPayload;
  className: string;
  iconClassName: string;
  loadImageContent?: (id: string) => Promise<string>;
}

export function SupportImageDownload({ attachment, className, iconClassName, loadImageContent }: SupportImageDownloadProps) {
  const inFlight = useRef(false);
  const [downloading, setDownloading] = useState(false);

  async function download() {
    if (inFlight.current) return;
    inFlight.current = true;
    setDownloading(true);
    const controller = new AbortController();
    const timeout = window.setTimeout(() => controller.abort(), 30_000);
    let errorMessage = 'Could not download the attachment. Please try again.';

    try {
      // Preview URLs return inline content. A local blob download saves the file
      // without opening storage in another tab or relying on cross-origin `download`.
      const source = loadImageContent ? await loadImageContent(attachment.id) : attachment.url;
      const response = await fetch(source, { credentials: 'omit', signal: controller.signal });
      if (!response.ok) {
        if (response.status === 403) {
          errorMessage = 'Could not download the attachment. Refresh the conversation and try again.';
        }
        throw new Error('Attachment request failed');
      }
      const blob = await response.blob();
      const objectURL = URL.createObjectURL(blob);
      const link = document.createElement('a');
      link.href = objectURL;
      link.download = attachment.file_name;
      document.body.append(link);
      try {
        link.click();
      } finally {
        link.remove();
        // Allow the browser to start saving before releasing the blob.
        window.setTimeout(() => URL.revokeObjectURL(objectURL), 60_000);
      }
    } catch {
      toast.error(controller.signal.aborted ? 'Download timed out. Please try again.' : errorMessage);
    } finally {
      window.clearTimeout(timeout);
      inFlight.current = false;
      setDownloading(false);
    }
  }

  return (
    <button
      type="button"
      className={`${className} disabled:cursor-wait disabled:opacity-60`}
      disabled={downloading}
      aria-busy={downloading}
      aria-label="Download image attachment"
      onClick={(event) => {
        event.stopPropagation();
        void download();
      }}
    >
      {downloading
        ? <Loading01Icon className={`${iconClassName} animate-spin`} />
        : <Download04Icon className={iconClassName} />}
    </button>
  );
}
