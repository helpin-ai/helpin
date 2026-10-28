import type { SupportLinkSecurity } from '@/lib/pmTypes';
import { findSupportLinkSecurity } from './helpers';

export const RISKY_EMAIL_LINK_ATTR = 'data-helpin-risky-href';
const RISKY_EMAIL_BADGE_ATTR = 'data-helpin-risky-badge';
const RISKY_EMAIL_STYLE_ID = 'helpin-risky-link-styles';

// AlertCircleIcon's shape, so email warnings match risky links in chat messages.
const WARNING_ICON = '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><circle cx="12" cy="12" r="10"></circle><path d="M12 8V12"></path><path d="M12.125 15.75H12M12.25 15.75C12.25 15.8881 12.1381 16 12 16C11.8619 16 11.75 15.8881 11.75 15.75C11.75 15.6119 11.8619 15.5 12 15.5C12.1381 15.5 12.25 15.6119 12.25 15.75Z"></path></svg>';

// The id in :is() outranks the reading-mode link color, which is scoped the same way.
const RISKY_EMAIL_LINK_STYLES = `
  :is(#helpin-email-reading, html) body [${RISKY_EMAIL_LINK_ATTR}],
  :is(#helpin-email-reading, html) body [${RISKY_EMAIL_LINK_ATTR}] * {
    color: var(--email-risk, #dc2626) !important;
    cursor: pointer;
  }
  :is(#helpin-email-reading, html) body a[${RISKY_EMAIL_LINK_ATTR}] {
    text-decoration: underline !important;
  }
  [${RISKY_EMAIL_BADGE_ATTR}] {
    display: inline-flex;
    margin-left: 2px;
    vertical-align: text-bottom;
  }
  [${RISKY_EMAIL_BADGE_ATTR}] svg {
    width: 14px;
    height: 14px;
  }
`;

/**
 * Disarms email links that Google Web Risk flagged. The address moves to a data
 * attribute, so a click can never reach the site, and a warning icon follows the
 * link text. Returns how many links were newly marked.
 */
export function markRiskyEmailLinks(doc: Document, entries: SupportLinkSecurity[]): number {
  if (!entries.some((entry) => entry.status === 'malicious')) return 0;
  let marked = 0;
  doc.querySelectorAll<HTMLElement>('a[href], area[href]').forEach((link) => {
    const href = link.getAttribute('href') ?? '';
    if (findSupportLinkSecurity(entries, href)?.status !== 'malicious') return;
    link.removeAttribute('href');
    link.setAttribute(RISKY_EMAIL_LINK_ATTR, href);
    link.setAttribute('role', 'link');
    link.setAttribute('tabindex', '0');
    link.setAttribute('title', `Potentially harmful link: ${href}`);
    if (link.tagName === 'A') {
      const badge = doc.createElement('span');
      badge.setAttribute(RISKY_EMAIL_BADGE_ATTR, 'true');
      badge.setAttribute('role', 'img');
      badge.setAttribute('aria-label', 'Potentially harmful link');
      badge.innerHTML = WARNING_ICON;
      link.appendChild(badge);
    }
    marked += 1;
  });
  if (marked > 0 && !doc.getElementById(RISKY_EMAIL_STYLE_ID)) {
    const style = doc.createElement('style');
    style.id = RISKY_EMAIL_STYLE_ID;
    style.textContent = RISKY_EMAIL_LINK_STYLES;
    doc.head.appendChild(style);
  }
  return marked;
}

/** Returns the flagged address behind a click or key press inside a marked email. */
export function riskyEmailLinkHref(target: EventTarget | null): string | null {
  // Iframe nodes come from another window, so `instanceof Element` would fail.
  const element = target as Element | null;
  if (!element || typeof element.closest !== 'function') return null;
  return element.closest(`[${RISKY_EMAIL_LINK_ATTR}]`)?.getAttribute(RISKY_EMAIL_LINK_ATTR) ?? null;
}
