export const QUOTE_ATTR = 'data-helpin-quote';
export const COLLAPSE_HOST_ATTR = 'data-helpin-collapse-host';
export const COLLAPSED_BODY_ATTR = 'data-helpin-collapsed';
// Outlook for iOS/Android wraps quoted history here. Emails stored before the
// backend marked this wrapper rely on matching it directly.
const OUTLOOK_MOBILE_QUOTE_SELECTOR = '#mail-editor-reference-message-container';
export const COLLAPSIBLE_SELECTOR = `[${QUOTE_ATTR}], ${OUTLOOK_MOBILE_QUOTE_SELECTOR}, .gmail_signature, .gmail_signature_prefix`;
const OUTLOOK_QUOTE_BOUNDARY_SELECTOR = `#divRplyFwdMsg[${QUOTE_ATTR}]`;

const ELEMENT_NODE = 1;
const TEXT_NODE = 3;
const SHOW_ELEMENT = 1;
const SHOW_TEXT = 4;

function elementIsHidden(el: Element, win: Window | null): boolean {
  if (!win) return false;
  const style = win.getComputedStyle(el);
  return style.display === 'none' || style.visibility === 'hidden';
}

function isInsideCollapsibleContent(node: Node): boolean {
  const el = node.nodeType === ELEMENT_NODE ? node as Element : node.parentElement;
  return !!el?.closest(COLLAPSIBLE_SELECTOR);
}

function isReplacedOrAtomicElement(el: Element): boolean {
  return ['IMG', 'HR', 'TABLE', 'SVG', 'CANVAS', 'VIDEO', 'AUDIO'].includes(el.tagName);
}

function isWordQuoteHeader(el: Element): boolean {
  const style = el.getAttribute('style')?.toLowerCase() ?? '';
  if (!style.includes('border-top') && el.previousElementSibling?.tagName !== 'HR') return false;
  const firstLabel = el.querySelector('b, strong')?.textContent?.trim();
  if (!firstLabel || !el.textContent?.trim().startsWith(firstLabel)) return false;
  if (!['from', 'sent', 'to', 'subject'].includes(firstLabel.toLowerCase().replace(/[:：]+$/, '').trim())) return false;
  // Read the labels separately: textContent concatenates rows separated by
  // <br>, so a whole-header regex misses valid compact Outlook markup.
  const labels = new Set(Array.from(el.querySelectorAll('b, strong'), (label) =>
    label.textContent?.trim().toLowerCase().replace(/[:：]+$/, '').trim(),
  ));
  return labels.has('from') && labels.has('subject') && (labels.has('sent') || labels.has('to'));
}

function markQuoteBoundaryAndFollowingSiblings(marker: Element, includeMarker: boolean): void {
  if (includeMarker) marker.setAttribute(QUOTE_ATTR, 'true');
  if (marker.previousElementSibling?.tagName === 'HR') {
    marker.previousElementSibling.setAttribute(QUOTE_ATTR, 'true');
  }
  let sibling = marker.nextElementSibling;
  while (sibling) {
    sibling.setAttribute(QUOTE_ATTR, 'true');
    sibling = sibling.nextElementSibling;
  }
}

function maxTextNodeBottom(doc: Document, textNode: Text): number {
  if (!textNode.textContent?.trim()) return 0;
  const range = doc.createRange();
  range.selectNodeContents(textNode);
  const bottom = Array.from(range.getClientRects()).reduce((value, rect) => Math.max(value, rect.bottom), 0);
  range.detach();
  return bottom;
}

export function prepareCollapsedEmailLayout(doc: Document, collapsed: boolean): void {
  doc.body?.setAttribute(COLLAPSED_BODY_ATTR, collapsed ? 'true' : 'false');
  doc.querySelectorAll(`[${COLLAPSE_HOST_ATTR}]`).forEach((el) => {
    el.removeAttribute(COLLAPSE_HOST_ATTR);
  });
  if (!collapsed) return;

  // This Outlook composition placeholder can come before the new reply.
  // It is not a quote boundary; actual reply headers determine the split.
  doc.querySelectorAll('#appendonsend').forEach((el) => el.removeAttribute(QUOTE_ATTR));

  doc.querySelectorAll(OUTLOOK_QUOTE_BOUNDARY_SELECTOR).forEach((marker) => {
    markQuoteBoundaryAndFollowingSiblings(marker, false);
  });

  doc.querySelectorAll('*').forEach((element) => {
    if (isWordQuoteHeader(element)) {
      markQuoteBoundaryAndFollowingSiblings(element, true);
    }
  });

  doc.querySelectorAll(COLLAPSIBLE_SELECTOR).forEach((el) => {
    let parent = el.parentElement;
    while (parent && parent !== doc.body && parent !== doc.documentElement) {
      parent.setAttribute(COLLAPSE_HOST_ATTR, 'true');
      parent = parent.parentElement;
    }
  });
}

export function measureVisibleEmailContentHeight(doc: Document, collapsed: boolean): number {
  const body = doc.body;
  if (!body) return 40;

  const win = doc.defaultView;
  // Classic scrollbars occupy height inside the frame; overlay scrollbars
  // occupy none. Include that space so the final line remains visible.
  const viewportHeight = doc.documentElement.clientHeight;
  const scrollbarHeight = win && viewportHeight > 0 ? Math.max(0, win.innerHeight - viewportHeight) : 0;
  const bodyStyle = win ? win.getComputedStyle(body) : null;
  const paddingBottom = bodyStyle ? Number.parseFloat(bodyStyle.paddingBottom || '0') || 0 : 0;
  const walker = doc.createTreeWalker(body, SHOW_ELEMENT | SHOW_TEXT);
  let visibleBottom = 0;

  while (walker.nextNode()) {
    const node = walker.currentNode;
    if (isInsideCollapsibleContent(node)) {
      continue;
    }

    if (node.nodeType === TEXT_NODE) {
      const parent = node.parentElement;
      if (!parent || elementIsHidden(parent, win)) {
        continue;
      }
      visibleBottom = Math.max(visibleBottom, maxTextNodeBottom(doc, node as Text));
      continue;
    }

    const el = node as Element;
    if (elementIsHidden(el, win) || !isReplacedOrAtomicElement(el)) {
      continue;
    }
    visibleBottom = Math.max(visibleBottom, el.getBoundingClientRect().bottom);
  }

  // Some email layouts let the final signature/content overflow the body's
  // reported scroll height (for example, positioned table-based signatures).
  // Include measured descendant bounds so the host iframe never ends before
  // visible email content and the following attachment section stays below it.
  const measuredVisibleHeight = visibleBottom + paddingBottom;
  if (!collapsed) {
    return Math.ceil(Math.max(body.scrollHeight, doc.documentElement?.scrollHeight ?? 0, measuredVisibleHeight, 40) + scrollbarHeight);
  }

  return Math.ceil(Math.max(measuredVisibleHeight, 40) + scrollbarHeight);
}
