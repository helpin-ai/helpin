import type { SupportAttachmentPayload } from '@/lib/pmTypes';
import { isPreviewableImageType } from '@/lib/supportAttachmentFiles';

export function normalizeIndex(index: number, length: number): number {
  if (length <= 0) return 0;
  return ((index % length) + length) % length;
}

// isLightboxAttachment reports whether the lightbox can show the file:
// images the browser can render, videos, and PDFs.
export function isLightboxAttachment(attachment: Pick<SupportAttachmentPayload, 'file_type' | 'url'>): boolean {
  if (!attachment.url) return false;
  return isPreviewableImageType(attachment.file_type)
    || attachment.file_type.startsWith('video/')
    || attachment.file_type === 'application/pdf';
}
