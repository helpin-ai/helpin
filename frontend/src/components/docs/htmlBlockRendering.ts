const ISOLATED_HTML_PATTERNS = [
  /<!doctype\b/i,
  /<html[\s>]/i,
  /<head[\s>]/i,
  /<body[\s>]/i,
  /<style[\s>]/i,
  /<script[\s>]/i,
  /<svg[\s>]/i,
  /<iframe[\s>]/i,
  /<object[\s>]/i,
  /<embed[\s>]/i,
  /<form[\s>]/i,
  /\son[a-z]+\s*=/i,
];

export function shouldRenderHtmlBlockIsolated(html: string, renderMode?: string): boolean {
  if (renderMode === 'sandboxed') return true;
  return ISOLATED_HTML_PATTERNS.some((pattern) => pattern.test(html));
}

export function buildHtmlBlockSrcDoc(html: string, frameId?: string): string {
  const resizeScript = frameId ? htmlBlockResizeScript(frameId) : '';
  if (!resizeScript) return html;

  if (/<\/body\s*>/i.test(html)) {
    return html.replace(/<\/body\s*>/i, `${resizeScript}</body>`);
  }
  if (/<\/html\s*>/i.test(html)) {
    return html.replace(/<\/html\s*>/i, `${resizeScript}</html>`);
  }
  return `<!doctype html><html><head><meta charset="utf-8"></head><body style="margin:0">${html}${resizeScript}</body></html>`;
}

function htmlBlockResizeScript(frameId: string): string {
  return `<script>
(function () {
  var frameId = ${JSON.stringify(frameId)};
  function measure() {
    var doc = document.documentElement;
    var body = document.body;
    var height = Math.max(
      doc ? doc.scrollHeight : 0,
      doc ? doc.offsetHeight : 0,
      body ? body.scrollHeight : 0,
      body ? body.offsetHeight : 0
    );
    parent.postMessage({ type: 'helpin:html-block:resize', id: frameId, height: height }, '*');
  }
  window.addEventListener('load', measure);
  window.addEventListener('resize', measure);
  if (typeof ResizeObserver !== 'undefined') {
    new ResizeObserver(measure).observe(document.documentElement);
  }
  setTimeout(measure, 50);
  setTimeout(measure, 300);
  setTimeout(measure, 1000);
})();
</script>`;
}
