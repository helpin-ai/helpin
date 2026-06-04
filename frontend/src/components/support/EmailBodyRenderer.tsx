import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import DOMPurify from 'dompurify';

interface EmailBodyRendererProps {
  /** Backend-sanitized HTML. Still re-sanitized here as defense-in-depth. */
  html: string;
  collapsedByDefault?: boolean;
}

// Matches the attribute the backend (server/internal/email/inboundhtml) uses
// to flag quoted reply history. We keep the attribute in case we want to add
// a collapse affordance later, but quotes always render inline today.
const QUOTE_ATTR = 'data-helpin-quote';

// Minimal iframe baseline. We want author stylesheets to win, so we only
// set inherit-able defaults on the document root — email inline styles and
// <style> blocks override for anything specific.
// Email HTML is authored for light backgrounds. Force a white canvas inside
// the iframe so author colors remain readable when the host app is in dark
// mode (matches Gmail/Outlook behavior).
const IFRAME_STYLES = `
  *, *::before, *::after {
    box-sizing: border-box;
  }
  html, body {
    margin: 0;
    padding: 0;
    min-width: 0 !important;
    max-width: 100% !important;
    background: #ffffff;
    color: #111827;
    color-scheme: light;
    font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
    font-size: 14px;
    line-height: 1.5;
    word-wrap: break-word !important;
    overflow-wrap: anywhere !important;
    overflow-x: hidden;
  }
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

function buildSrcDoc(safeHtml: string): string {
  // The sandbox blocks inline scripts (no allow-scripts) — any script from
  // the email author is neutralized. Author <style> blocks pass through.
  return `<!doctype html>
<html>
<head>
<meta charset="utf-8" />
<meta name="viewport" content="width=device-width, initial-scale=1" />
<base target="_blank" />
<style>${IFRAME_STYLES}</style>
</head>
<body>${safeHtml}</body>
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
  [${QUOTE_ATTR}], .gmail_signature, .gmail_signature_prefix {
    display: none !important;
  }
`;

// Checks whether the email iframe has any collapsible sections.
function hasCollapsibleContent(doc: Document): boolean {
  return doc.querySelector(`[${QUOTE_ATTR}], .gmail_signature`) !== null;
}

export function EmailBodyRenderer({ html, collapsedByDefault = true }: EmailBodyRendererProps) {
  const iframeRef = useRef<HTMLIFrameElement | null>(null);
  const [height, setHeight] = useState(40);
  const [ready, setReady] = useState(false);
  const [collapsed, setCollapsed] = useState(collapsedByDefault);
  const [hasCollapsible, setHasCollapsible] = useState(false);
  const collapseSheetRef = useRef<HTMLStyleElement | null>(null);

  const sanitized = useMemo(() => sanitize(html), [html]);
  const srcDoc = useMemo(() => buildSrcDoc(sanitized), [sanitized]);

  useEffect(() => {
    setCollapsed(collapsedByDefault);
    setReady(false);
    setHasCollapsible(false);
    setHeight(40);
  }, [collapsedByDefault, srcDoc]);

  const measure = useCallback(() => {
    const doc = iframeRef.current?.contentDocument;
    if (!doc) return;
    // body.scrollHeight reflects content height independent of the iframe's
    // current height; documentElement.scrollHeight is clamped to the iframe
    // size, so it can't shrink when content collapses. Prefer body.
    const next = doc.body.scrollHeight;
    if (next > 0) setHeight(next);
  }, []);

  const handleLoad = useCallback(() => {
    const doc = iframeRef.current?.contentDocument;
    if (!doc) return;
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

    setHasCollapsible(hasCollapsibleContent(doc));
    setReady(true);
    measure();
  }, [collapsed, measure]);

  // Toggle collapse stylesheet on/off.
  useEffect(() => {
    if (!collapseSheetRef.current) return;
    collapseSheetRef.current.disabled = !collapsed;
    measure();
  }, [collapsed, measure]);

  useEffect(() => {
    if (!ready) return;
    const doc = iframeRef.current?.contentDocument;
    if (!doc) return;
    const observer = new ResizeObserver(() => measure());
    observer.observe(doc.documentElement);
    return () => observer.disconnect();
  }, [ready, measure]);

  return (
    <div className="min-w-0 max-h-[60vh] w-full max-w-full overflow-auto">
      <iframe
        ref={iframeRef}
        srcDoc={srcDoc}
        onLoad={handleLoad}
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
