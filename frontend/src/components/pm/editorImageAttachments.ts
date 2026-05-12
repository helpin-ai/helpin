import { pmAttachmentService } from '@/lib/services/pmAttachmentService';

const extractAttachmentIdsFromHtml = (html: string): string[] => {
  const ids = new Set<string>();
  const matches = html.matchAll(/data-attachment-id=["']([^"']+)["']/g);

  for (const match of matches) {
    const attachmentId = match[1]?.trim();
    if (attachmentId) {
      ids.add(attachmentId);
    }
  }

  return [...ids];
};

export function extractInlineAttachmentIds(html: string | null | undefined): string[] {
  if (!html) {
    return [];
  }

  return extractAttachmentIdsFromHtml(html);
}

export function diffRemovedInlineAttachmentIds(
  previousHtml: string | null | undefined,
  nextHtml: string | null | undefined,
): string[] {
  const previousIds = new Set(extractInlineAttachmentIds(previousHtml));
  const nextIds = new Set(extractInlineAttachmentIds(nextHtml));

  return [...previousIds].filter((attachmentId) => !nextIds.has(attachmentId));
}

export function removeInlineImagesByAttachmentIds(
  html: string | null | undefined,
  attachmentIds: string[],
): string {
  if (!html || attachmentIds.length === 0) {
    return html ?? '';
  }

  if (typeof DOMParser === 'undefined') {
    return html;
  }

  const idsToRemove = new Set(attachmentIds);
  const document = new DOMParser().parseFromString(`<body>${html}</body>`, 'text/html');

  document.body.querySelectorAll('img[data-attachment-id]').forEach((image) => {
    const attachmentId = image.getAttribute('data-attachment-id');
    if (attachmentId && idsToRemove.has(attachmentId)) {
      image.remove();
    }
  });

  return document.body.innerHTML;
}

export function normalizeInlineAttachmentImageSrcs(html: string | null | undefined): string {
  if (!html) {
    return html ?? '';
  }

  if (typeof DOMParser === 'undefined') {
    return html.replace(
      /<img\b([^>]*?)\sdata-attachment-id=["']([^"']+)["']([^>]*)>/gi,
      (match, before: string, attachmentId: string, after: string) => {
        const nextSrc = `src="${pmAttachmentService.contentUrl(attachmentId)}"`;
        if (/\ssrc=["'][^"']*["']/i.test(match)) {
          return match.replace(/\ssrc=["'][^"']*["']/i, ` ${nextSrc}`);
        }
        return `<img ${nextSrc}${before} data-attachment-id="${attachmentId}"${after}>`;
      },
    );
  }

  const document = new DOMParser().parseFromString(`<body>${html}</body>`, 'text/html');
  let changed = false;

  document.body.querySelectorAll('img[data-attachment-id]').forEach((image) => {
    const attachmentId = image.getAttribute('data-attachment-id')?.trim();
    if (!attachmentId) return;
    const contentUrl = pmAttachmentService.contentUrl(attachmentId);
    if (image.getAttribute('src') !== contentUrl) {
      image.setAttribute('src', contentUrl);
      changed = true;
    }
  });

  return changed ? document.body.innerHTML : html;
}
