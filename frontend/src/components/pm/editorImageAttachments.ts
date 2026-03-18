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
