import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import DOMPurify from 'dompurify';
import { API_BASE } from '@/lib/api';

interface EmailBodyRendererProps {
  /** Backend-sanitized HTML. Still re-sanitized here as defense-in-depth. */
  html: string;
}

// Matches the data attributes the backend (server/internal/email/inboundhtml)
// applies to mark quoted sections and remote images. Keep these in sync with
// the Go constants in that package.
const QUOTE_ATTR = 'data-helpin-quote';
const REMOTE_IMAGE_ATTR = 'data-helpin-remote-image';
const REMOTE_IMAGE_SRC_ATTR = 'data-helpin-remote-src';

// Typography baseline injected inside the sandbox iframe. We don't want email
// authors' styles to collide with our app, and we also want a sensible default
// if the email has no styling of its own (like the case that sparked this
// redesign). Keep this minimal — the email's own <style>-derived attributes
// still carry through for anything more specific.
const IFRAME_STYLES = `
  html, body {
    margin: 0;
    padding: 0;
    background: transparent;
    color: #111827;
    font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
    font-size: 14px;
    line-height: 1.6;
    word-wrap: break-word;
    overflow-wrap: anywhere;
  }
  body { padding: 2px; }
  a { color: #2563eb; text-decoration: underline; }
  a:hover { text-decoration: none; }
  img { max-width: 100%; height: auto; }
  table { border-collapse: collapse; max-width: 100%; }
  ul, ol { padding-left: 1.5em; }
  blockquote {
    border-left: 3px solid #e5e7eb;
    margin: 0.75em 0;
    padding: 0 0 0 1em;
    color: #4b5563;
  }
  pre, code { font-family: ui-monospace, SFMono-Regular, Menlo, monospace; font-size: 0.9em; }
  pre { background: #f3f4f6; padding: 0.6em; border-radius: 6px; overflow-x: auto; }
  hr { border: none; border-top: 1px solid #e5e7eb; margin: 1em 0; }
  [${QUOTE_ATTR}] { display: none; }
  [${QUOTE_ATTR}].helpin-quote-open { display: block; }
  img[${REMOTE_IMAGE_ATTR}]:not([data-helpin-loaded]) {
    background: repeating-linear-gradient(45deg, #f3f4f6, #f3f4f6 6px, #e5e7eb 6px, #e5e7eb 12px);
    min-height: 24px;
    min-width: 40px;
  }
`;

function buildSrcDoc(safeHtml: string): string {
  // Inline script is blocked by sandbox (we don't allow-scripts). That's
  // intentional — any script from the email author would fail silently.
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
    // Bluemonday on the server already stripped script/style/etc. This pass
    // is defense-in-depth — keeps us safe if the stored HTML ever gets
    // tampered with at rest.
    FORBID_TAGS: ['script', 'style', 'iframe', 'object', 'embed', 'form', 'input', 'button'],
    ADD_ATTR: [QUOTE_ATTR, REMOTE_IMAGE_ATTR, REMOTE_IMAGE_SRC_ATTR, 'data-helpin-cid', 'target', 'rel'],
    ADD_TAGS: ['font'],
    ALLOW_DATA_ATTR: true,
  });
}

async function fetchImageAsObjectURL(url: string): Promise<string | null> {
  const token = localStorage.getItem('access_token');
  const res = await fetch(`${API_BASE}/email/image-proxy?url=${encodeURIComponent(url)}`, {
    headers: token ? { Authorization: `Bearer ${token}` } : undefined,
  });
  if (!res.ok) return null;
  const blob = await res.blob();
  if (!blob.type.startsWith('image/')) return null;
  return URL.createObjectURL(blob);
}

export function EmailBodyRenderer({ html }: EmailBodyRendererProps) {
  const iframeRef = useRef<HTMLIFrameElement | null>(null);
  const objectUrlsRef = useRef<string[]>([]);
  const [height, setHeight] = useState(120);
  const [ready, setReady] = useState(false);
  const [quoteCount, setQuoteCount] = useState(0);
  const [quoteOpen, setQuoteOpen] = useState(false);
  const [remoteImageCount, setRemoteImageCount] = useState(0);
  const [imagesLoading, setImagesLoading] = useState(false);
  const [imagesLoaded, setImagesLoaded] = useState(false);

  const sanitized = useMemo(() => sanitize(html), [html]);
  const srcDoc = useMemo(() => buildSrcDoc(sanitized), [sanitized]);

  const measure = useCallback(() => {
    const doc = iframeRef.current?.contentDocument;
    if (!doc) return;
    const next = Math.max(doc.documentElement.scrollHeight, doc.body.scrollHeight);
    if (next > 0) setHeight(next);
  }, []);

  // On load: mark external links, inventory quote & remote-image counts, size.
  const handleLoad = useCallback(() => {
    const doc = iframeRef.current?.contentDocument;
    if (!doc) return;
    doc.querySelectorAll('a').forEach((a) => {
      a.setAttribute('target', '_blank');
      a.setAttribute('rel', 'noopener noreferrer nofollow');
    });
    setQuoteCount(doc.querySelectorAll(`[${QUOTE_ATTR}]`).length);
    setRemoteImageCount(doc.querySelectorAll(`img[${REMOTE_IMAGE_ATTR}]`).length);
    setReady(true);
    measure();
  }, [measure]);

  // Keep height in sync with content changes (image loads, toggles).
  useEffect(() => {
    if (!ready) return;
    const doc = iframeRef.current?.contentDocument;
    if (!doc) return;
    const observer = new ResizeObserver(() => measure());
    observer.observe(doc.documentElement);
    return () => observer.disconnect();
  }, [ready, measure]);

  // Quote toggle: flip class on every quoted node so our CSS rule shows/hides.
  useEffect(() => {
    if (!ready) return;
    const doc = iframeRef.current?.contentDocument;
    if (!doc) return;
    doc.querySelectorAll(`[${QUOTE_ATTR}]`).forEach((el) => {
      el.classList.toggle('helpin-quote-open', quoteOpen);
    });
    // Let layout settle before re-measuring.
    requestAnimationFrame(measure);
  }, [quoteOpen, ready, measure]);

  // Revoke any blob URLs when this renderer unmounts or the HTML changes.
  useEffect(() => {
    const urls = objectUrlsRef.current;
    return () => {
      urls.forEach((u) => URL.revokeObjectURL(u));
      urls.length = 0;
    };
  }, [sanitized]);

  const loadRemoteImages = useCallback(async () => {
    const doc = iframeRef.current?.contentDocument;
    if (!doc || imagesLoading || imagesLoaded) return;
    setImagesLoading(true);
    const imgs = Array.from(
      doc.querySelectorAll<HTMLImageElement>(`img[${REMOTE_IMAGE_ATTR}]`),
    );
    await Promise.all(
      imgs.map(async (img) => {
        if (img.dataset.helpinLoaded === 'true') return;
        const url = img.getAttribute(REMOTE_IMAGE_SRC_ATTR);
        if (!url) return;
        const objectURL = await fetchImageAsObjectURL(url).catch(() => null);
        if (!objectURL) {
          img.dataset.helpinLoaded = 'failed';
          return;
        }
        objectUrlsRef.current.push(objectURL);
        img.src = objectURL;
        img.dataset.helpinLoaded = 'true';
      }),
    );
    setImagesLoading(false);
    setImagesLoaded(true);
    requestAnimationFrame(measure);
  }, [imagesLoading, imagesLoaded, measure]);

  return (
    <div className="flex flex-col gap-2">
      {(remoteImageCount > 0 || quoteCount > 0) && (
        <div className="flex flex-wrap items-center gap-2 text-xs">
          {remoteImageCount > 0 && (
            <button
              type="button"
              onClick={loadRemoteImages}
              disabled={imagesLoaded || imagesLoading}
              className="rounded-md border border-border/70 bg-muted/40 px-2.5 py-1 font-medium text-muted-foreground transition-colors hover:bg-muted disabled:cursor-default disabled:opacity-60"
            >
              {imagesLoaded
                ? 'Images loaded'
                : imagesLoading
                  ? 'Loading images…'
                  : `Load ${remoteImageCount} remote image${remoteImageCount === 1 ? '' : 's'}`}
            </button>
          )}
          {quoteCount > 0 && (
            <button
              type="button"
              onClick={() => setQuoteOpen((v) => !v)}
              className="rounded-md border border-border/70 bg-muted/40 px-2.5 py-1 font-medium text-muted-foreground transition-colors hover:bg-muted"
            >
              {quoteOpen ? 'Hide quoted history' : 'Show quoted history'}
            </button>
          )}
        </div>
      )}
      <iframe
        ref={iframeRef}
        srcDoc={srcDoc}
        onLoad={handleLoad}
        // allow-same-origin lets us manipulate the DOM (quote toggle, image
        // swaps). allow-popups-to-escape-sandbox lets user-clicked links open
        // normally. No allow-scripts — any author script is neutralized.
        sandbox="allow-same-origin allow-popups allow-popups-to-escape-sandbox"
        title="Email body"
        style={{ width: '100%', border: 'none', height: `${height}px` }}
      />
    </div>
  );
}
