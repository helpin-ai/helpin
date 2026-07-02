export const QUOTE_ATTR = 'data-helpin-quote';
export const COLLAPSE_HOST_ATTR = 'data-helpin-collapse-host';
export const COLLAPSED_BODY_ATTR = 'data-helpin-collapsed';
export const COLLAPSIBLE_SELECTOR = `[${QUOTE_ATTR}], .gmail_signature, .gmail_signature_prefix`;

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

  if (!collapsed) {
    return Math.ceil(Math.max(body.scrollHeight, doc.documentElement?.scrollHeight ?? 0, 40));
  }

  const win = doc.defaultView;
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

  return Math.ceil(Math.max(visibleBottom + paddingBottom, 40));
}
