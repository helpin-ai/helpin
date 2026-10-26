import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import DOMPurify from 'dompurify';
import { useTheme } from 'next-themes';
import {
  COLLAPSED_BODY_ATTR,
  COLLAPSE_HOST_ATTR,
  COLLAPSIBLE_SELECTOR,
  QUOTE_ATTR,
  measureVisibleEmailContentHeight,
  prepareCollapsedEmailLayout,
} from './EmailBodyRendererLayout';

interface EmailBodyRendererProps {
  /** Backend-sanitized HTML. Still re-sanitized here as defense-in-depth. */
  html: string;
  collapsedByDefault?: boolean;
  constrainHeight?: boolean;
  themeAware?: boolean;
}

interface EmailBodyFrameProps {
  srcDoc: string;
  collapsedByDefault: boolean;
  constrainHeight: boolean;
  themed: boolean;
}

// Original formatting uses a light canvas for authored email colors. The
// default reading view overlays theme colors without changing email layout.
export const EMAIL_BODY_IFRAME_STYLES = `
  *, *::before, *::after {
    box-sizing: border-box;
  }
  /* Gmail authors Drive links as fixed-height chips using the content-box
     sizing model. Keep this narrow exception: applying our border-box reset
     makes the chip's padding consume its declared height and clips the file
     icon/name (including Drive-hosted videos, which are links rather than
     MIME attachments). The substring selector also covers Gmail variants
     that combine or suffix the class name. */
  [class*="gmail_drive_chip"] {
    box-sizing: content-box !important;
  }
  [class*="gmail_drive_chip"] a img {
    display: inline-block;
  }
  html, body {
    margin: 0;
    padding: 0;
    min-width: 0 !important;
    max-width: 100% !important;
    background: #ffffff;
    color: rgba(17, 24, 39, 0.9);
    color-scheme: light;
    font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
    font-size: 14px;
    line-height: 1.5;
    word-wrap: break-word !important;
    overflow-wrap: anywhere !important;
  }
  /* Keep fixed-width email layouts reachable without widening the thread.
     Only the document scrolls; a second scroller on body would clip content. */
  html { overflow-x: auto; overflow-y: hidden; }
  /* Padding lives on body only so body.scrollHeight reflects the full
     visible content height — measure() relies on this to size the iframe. */
  body { padding: 8px 12px; }
  body, p, div, span, a, li, td, th, blockquote {
    max-width: 100% !important;
    overflow-wrap: anywhere !important;
    word-break: break-word;
  }
  pre {
    max-width: 100% !important;
    white-space: pre-wrap !important;
    overflow-wrap: anywhere !important;
  }
  table {
    max-width: 100% !important;
  }
  td, th {
    min-width: 0 !important;
  }
  img { max-width: 100% !important; height: auto; }
`;

// Preserve layout and imagery; only presentation colors change in reading mode.
const EMAIL_READING_STYLES = `
  html[data-helpin-reading], html[data-helpin-reading] body {
    background: transparent !important;
    color: var(--email-text, #242424) !important;
    color-scheme: var(--email-scheme, light);
  }
  :is(#helpin-email-reading, html[data-helpin-reading]) body :not(img, svg, svg *) {
    color: inherit !important;
    background-color: transparent !important;
    background-image: none !important;
    border-color: var(--email-border, #dedede) !important;
    box-shadow: none !important;
    text-shadow: none !important;
  }
  :is(#helpin-email-reading, html[data-helpin-reading]) body a,
  :is(#helpin-email-reading, html[data-helpin-reading]) body a * {
    color: var(--email-link, #3e63dd) !important;
  }
  :is(#helpin-email-reading, html[data-helpin-reading]) body pre,
  :is(#helpin-email-reading, html[data-helpin-reading]) body code {
    background-color: color-mix(in srgb, var(--email-text, #242424) 5%, transparent) !important;
    border-radius: 4px;
  }
  :is(#helpin-email-reading, html[data-helpin-reading]) body pre code {
    background-color: transparent !important;
  }
`;

function readingHtml(safeHtml: string): string {
  const doc = new DOMParser().parseFromString(safeHtml, 'text/html');
  doc.querySelectorAll<HTMLElement>('[style]').forEach(element => {
    if (element.closest('svg') || element.tagName === 'IMG') return;
    // Inline !important declarations would otherwise override the reading sheet.
    for (const property of Array.from(element.style)) {
      if (/^(color|background.*|border.*color|text-shadow|box-shadow)$/.test(property)) {
        element.style.removeProperty(property);
      }
    }
  });
  return doc.head.innerHTML + doc.body.innerHTML;
}

function buildSrcDoc(safeHtml: string, themed: boolean): string {
  // The sandbox blocks inline scripts (no allow-scripts) — any script from
  // the email author is neutralized. Author <style> blocks pass through.
  return `<!doctype html>
<html${themed ? ' data-helpin-reading="true"' : ''}>
<head>
<meta charset="utf-8" />
<meta name="viewport" content="width=device-width, initial-scale=1" />
<base target="_blank" />
<style>${EMAIL_BODY_IFRAME_STYLES}</style>
</head>
<body>${themed ? readingHtml(safeHtml) : safeHtml}${themed ? `<style>${EMAIL_READING_STYLES}</style>` : ''}</body>
</html>`;
}

function sanitize(html: string): string {
  return DOMPurify.sanitize(html, {
    // Scripts and event handlers are the real security boundary. <style>
    // tags are preserved so email class-based typography and @media rules
    // render like they do in Gmail/Crisp.
    FORBID_TAGS: ['script', 'iframe', 'object', 'embed', 'form', 'input', 'button'],
    ADD_ATTR: [QUOTE_ATTR, 'target', 'rel'],
    ADD_TAGS: ['style', 'font'],
    ALLOW_DATA_ATTR: true,
  });
}

// CSS injected into the iframe to hide quoted replies and known signature
// wrappers. Toggled on/off via a stylesheet enable/disable.
const COLLAPSE_STYLES = `
  ${COLLAPSIBLE_SELECTOR} {
    display: none !important;
  }
  body[${COLLAPSED_BODY_ATTR}="true"] [${COLLAPSE_HOST_ATTR}="true"] {
    height: auto !important;
    min-height: 0 !important;
    max-height: none !important;
  }
`;

// Checks whether the email iframe has any collapsible sections.
function hasCollapsibleContent(doc: Document): boolean {
  return doc.querySelector(COLLAPSIBLE_SELECTOR) !== null;
}

export function EmailBodyRenderer({ html, collapsedByDefault = true, constrainHeight = true, themeAware = false }: EmailBodyRendererProps) {
  const [original, setOriginal] = useState(false);
  const themed = themeAware && !original;
  const sanitized = useMemo(() => sanitize(html), [html]);
  const srcDoc = useMemo(() => buildSrcDoc(sanitized, themed), [sanitized, themed]);
  const rendererKey = `${collapsedByDefault ? 'collapsed' : 'expanded'}:${constrainHeight ? 'constrained' : 'full'}:${srcDoc}`;

  return (
    <div className="min-w-0">
      {themeAware && <div className="mb-1 flex justify-end">
        <button type="button" aria-pressed={original} onClick={() => setOriginal(value => !value)}
          className="rounded px-1 text-[11px] text-muted-foreground transition-colors hover:bg-muted hover:text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring">
          {original ? 'Use theme colors' : 'Original formatting'}
        </button>
      </div>}
      <EmailBodyFrame key={rendererKey} srcDoc={srcDoc} themed={themed} collapsedByDefault={collapsedByDefault} constrainHeight={constrainHeight} />
    </div>
  );
}

function EmailBodyFrame({ srcDoc, collapsedByDefault, constrainHeight, themed }: EmailBodyFrameProps) {
  const { resolvedTheme } = useTheme();
  const iframeRef = useRef<HTMLIFrameElement | null>(null);
  const [height, setHeight] = useState(40);
  const [ready, setReady] = useState(false);
  const [collapsed, setCollapsed] = useState(collapsedByDefault);
  const [hasCollapsible, setHasCollapsible] = useState(false);
  const collapseSheetRef = useRef<HTMLStyleElement | null>(null);
  const initializedDocumentRef = useRef<Document | null>(null);
  const measureTimerRef = useRef<number | null>(null);

  const measure = useCallback(() => {
    const doc = iframeRef.current?.contentDocument;
    if (!doc) return;
    setHeight(measureVisibleEmailContentHeight(doc, collapsed));
  }, [collapsed]);

  const scheduleMeasure = useCallback(() => {
    measure();
    const win = iframeRef.current?.contentWindow;
    if (win) {
      win.requestAnimationFrame(() => {
        measure();
        win.requestAnimationFrame(() => measure());
      });
    }
    if (measureTimerRef.current !== null) {
      window.clearTimeout(measureTimerRef.current);
    }
    measureTimerRef.current = window.setTimeout(() => {
      measureTimerRef.current = null;
      measure();
    }, 80);
  }, [measure]);

  useEffect(() => {
    return () => {
      if (measureTimerRef.current !== null) {
        window.clearTimeout(measureTimerRef.current);
      }
    };
  }, []);

  const applyTheme = useCallback(() => {
    const iframe = iframeRef.current;
    if (!themed || !iframe?.contentDocument) return;
    const host = window.getComputedStyle(iframe);
    const root = iframe.contentDocument.documentElement;
    root.style.setProperty('--email-text', host.color);
    root.style.setProperty('--email-border', host.getPropertyValue('--border').trim() || '#dedede');
    root.style.setProperty('--email-link', host.getPropertyValue(resolvedTheme === 'dark' ? '--brand-400' : '--brand-500').trim() || (resolvedTheme === 'dark' ? '#8aabff' : '#3e63dd'));
    root.style.setProperty('--email-scheme', resolvedTheme === 'dark' ? 'dark' : 'light');
  }, [resolvedTheme, themed]);

  useEffect(() => {
    // next-themes updates the host class in an effect; read its colors afterward.
    const frame = requestAnimationFrame(applyTheme);
    return () => cancelAnimationFrame(frame);
  }, [applyTheme]);

  const handleLoad = useCallback(() => {
    const doc = iframeRef.current?.contentDocument;
    if (!doc || doc.URL === 'about:blank' || doc.readyState === 'loading') return;
    // The final load event still remeasures after images settle, but must
    // not inject a second collapse stylesheet or reset the user's toggle.
    if (initializedDocumentRef.current === doc) {
      scheduleMeasure();
      return;
    }
    initializedDocumentRef.current = doc;
    applyTheme();
    doc.querySelectorAll('a').forEach((a) => {
      a.setAttribute('target', '_blank');
      a.setAttribute('rel', 'noopener noreferrer nofollow');
    });

    // Inject collapse stylesheet; forwarded messages can opt into showing
    // quoted sections immediately.
    const sheet = doc.createElement('style');
    sheet.textContent = COLLAPSE_STYLES;
    doc.head.appendChild(sheet);
    collapseSheetRef.current = sheet;
    sheet.disabled = !collapsed;
    prepareCollapsedEmailLayout(doc, collapsed);

    setHasCollapsible(hasCollapsibleContent(doc));
    setReady(true);
    scheduleMeasure();
  }, [collapsed, scheduleMeasure, applyTheme]);

  useEffect(() => {
    if (ready) return;
    // srcDoc replaces the initial about:blank document. Check for its parsed
    // DOM rather than waiting for iframe load, which can stall on remote images.
    handleLoad();
    const timer = window.setInterval(handleLoad, 50);
    return () => window.clearInterval(timer);
  }, [ready, handleLoad]);

  // Toggle collapse stylesheet on/off.
  useEffect(() => {
    if (!collapseSheetRef.current) return;
    collapseSheetRef.current.disabled = !collapsed;
    const doc = iframeRef.current?.contentDocument;
    if (doc) {
      prepareCollapsedEmailLayout(doc, collapsed);
    }
    scheduleMeasure();
  }, [collapsed, scheduleMeasure]);

  useEffect(() => {
    if (!ready) return;
    const doc = iframeRef.current?.contentDocument;
    if (!doc) return;
    const observer = new ResizeObserver(() => scheduleMeasure());
    observer.observe(doc.documentElement);
    observer.observe(doc.body);
    return () => observer.disconnect();
  }, [ready, scheduleMeasure]);

  return (
    <div className={constrainHeight ? 'min-w-0 max-h-[60vh] w-full max-w-full overflow-auto' : 'min-w-0 w-full max-w-full overflow-visible'}>
      <iframe
        ref={iframeRef}
        srcDoc={srcDoc}
        onLoad={handleLoad}
        data-collapsed-by-default={collapsedByDefault ? 'true' : 'false'}
        // allow-same-origin lets us manipulate the DOM (link rewrite).
        // allow-popups-to-escape-sandbox lets clicked links open normally.
        // No allow-scripts — author script is neutralized.
        sandbox="allow-same-origin allow-popups allow-popups-to-escape-sandbox"
        title="Email body"
        style={{ display: 'block', width: '100%', maxWidth: '100%', minWidth: 0, border: 'none', height: `${height}px` }}
      />
      {hasCollapsible && (
        <button
          type="button"
          onClick={() => setCollapsed((c) => !c)}
          className="mt-1 text-[11px] font-medium text-muted-foreground/60 hover:text-muted-foreground transition-colors"
        >
          {collapsed ? '··· Show quoted content' : '··· Hide quoted content'}
        </button>
      )}
    </div>
  );
}
