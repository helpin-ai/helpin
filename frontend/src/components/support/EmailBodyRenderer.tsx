import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import DOMPurify from 'dompurify';

interface EmailBodyRendererProps {
  /** Backend-sanitized HTML. Still re-sanitized here as defense-in-depth. */
  html: string;
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
  html, body {
    margin: 0;
    padding: 0;
    background: #ffffff;
    color: #111827;
    color-scheme: light;
    font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
    font-size: 14px;
    line-height: 1.5;
    word-wrap: break-word;
    overflow-wrap: anywhere;
  }
  img { max-width: 100%; height: auto; }
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

export function EmailBodyRenderer({ html }: EmailBodyRendererProps) {
  const iframeRef = useRef<HTMLIFrameElement | null>(null);
  const [height, setHeight] = useState(40);
  const [ready, setReady] = useState(false);

  const sanitized = useMemo(() => sanitize(html), [html]);
  const srcDoc = useMemo(() => buildSrcDoc(sanitized), [sanitized]);

  const measure = useCallback(() => {
    const doc = iframeRef.current?.contentDocument;
    if (!doc) return;
    const next = Math.max(doc.documentElement.scrollHeight, doc.body.scrollHeight);
    if (next > 0) setHeight(next);
  }, []);

  const handleLoad = useCallback(() => {
    const doc = iframeRef.current?.contentDocument;
    if (!doc) return;
    doc.querySelectorAll('a').forEach((a) => {
      a.setAttribute('target', '_blank');
      a.setAttribute('rel', 'noopener noreferrer nofollow');
    });
    setReady(true);
    measure();
  }, [measure]);

  useEffect(() => {
    if (!ready) return;
    const doc = iframeRef.current?.contentDocument;
    if (!doc) return;
    const observer = new ResizeObserver(() => measure());
    observer.observe(doc.documentElement);
    return () => observer.disconnect();
  }, [ready, measure]);

  return (
    <div className="w-full max-h-[60vh] overflow-auto">
      <iframe
        ref={iframeRef}
        srcDoc={srcDoc}
        onLoad={handleLoad}
        // allow-same-origin lets us manipulate the DOM (link rewrite).
        // allow-popups-to-escape-sandbox lets clicked links open normally.
        // No allow-scripts — author script is neutralized.
        sandbox="allow-same-origin allow-popups allow-popups-to-escape-sandbox"
        title="Email body"
        style={{ width: '100%', minWidth: '602px', border: 'none', height: `${height}px` }}
      />
    </div>
  );
}
