import DOMPurify from 'dompurify';

// Client-side HTML sanitizer for htmlBlock content.
// Mirrors the server-side bluemonday policy in server/internal/tiptap/sanitize.go.
// Used for editor preview AND export flows.

const ALLOWED_TAGS = [
  // Structural
  'div', 'section', 'article', 'aside', 'header', 'footer', 'nav', 'main',
  'figure', 'figcaption', 'dl', 'dt', 'dd', 'details', 'summary',
  'blockquote', 'br', 'hr', 'wbr',
  // Text
  'p', 'span', 'strong', 'b', 'em', 'i', 'u', 's', 'del', 'ins',
  'small', 'mark', 'abbr', 'cite', 'q', 'sub', 'sup',
  'code', 'pre', 'kbd', 'samp', 'var',
  // Lists
  'ul', 'ol', 'li',
  // Headings
  'h1', 'h2', 'h3', 'h4', 'h5', 'h6',
  // Tables
  'table', 'thead', 'tbody', 'tfoot', 'tr', 'th', 'td', 'caption', 'colgroup', 'col',
  // Links & images
  'a', 'img',
];

const ALLOWED_ATTR = [
  'href', 'target', 'rel', 'title',
  'src', 'alt', 'width', 'height', 'loading',
  'class', 'id',
  'style',
  'colspan', 'rowspan',
];

const ALLOWED_STYLE_PROPS = new Set([
  'background', 'background-color', 'border', 'border-color', 'border-radius', 'border-style', 'border-width',
  'align-content', 'align-items', 'align-self', 'box-sizing', 'color', 'column-gap', 'display', 'flex-basis',
  'flex-direction', 'flex-grow', 'flex-shrink', 'flex-wrap',
  'font-size', 'font-style', 'font-weight', 'gap', 'grid-template-columns', 'grid-template-rows', 'height',
  'justify-content', 'justify-items', 'justify-self', 'letter-spacing', 'line-height',
  'list-style', 'list-style-position', 'list-style-type',
  'margin', 'margin-bottom', 'margin-left', 'margin-right', 'margin-top',
  'max-height', 'max-width', 'min-height', 'min-width', 'object-fit', 'overflow', 'overflow-x', 'overflow-y',
  'padding', 'padding-bottom', 'padding-left', 'padding-right', 'padding-top',
  'row-gap', 'text-align', 'text-decoration', 'text-transform', 'vertical-align', 'white-space', 'width',
]);

export function sanitizeHtml(html: string, options: { allowSvgImages?: boolean } = {}): string {
  const safe = DOMPurify.sanitize(html, {
    ALLOWED_TAGS,
    ALLOWED_ATTR,
    ALLOW_DATA_ATTR: true,
    ALLOWED_URI_REGEXP: options.allowSvgImages
      ? /^(?:(?:https?|mailto|tel):|data:image\/(?:gif|jpe?g|png|webp|svg\+xml);base64,[a-z0-9+/=]+$|[^a-z]|[a-z+.-]+(?:[^a-z+.\-:]|$))/i
      : /^(?:(?:https?|mailto|tel):|data:image\/(?:gif|jpe?g|png|webp);base64,[a-z0-9+/=]+$|[^a-z]|[a-z+.-]+(?:[^a-z+.\-:]|$))/i,
  });
  return sanitizeInlineStyles(safe);
}

function sanitizeInlineStyles(html: string): string {
  if (typeof document === 'undefined' || !html.includes('style=')) {
    return html;
  }

  const template = document.createElement('template');
  template.innerHTML = html;
  template.content.querySelectorAll<HTMLElement>('[style]').forEach((el) => {
    const style = filterStyleAttribute(el.getAttribute('style') ?? '');
    if (style) {
      el.setAttribute('style', style);
    } else {
      el.removeAttribute('style');
    }
  });
  return template.innerHTML;
}

function filterStyleAttribute(style: string): string {
  const rules: string[] = [];
  for (const declaration of style.split(';')) {
    const index = declaration.indexOf(':');
    if (index <= 0) continue;
    const prop = declaration.slice(0, index).trim().toLowerCase();
    const value = declaration.slice(index + 1).trim();
    if (!ALLOWED_STYLE_PROPS.has(prop) || !isSafeStyleValue(value)) continue;
    rules.push(`${prop}: ${value}`);
  }
  return rules.join('; ');
}

function isSafeStyleValue(value: string): boolean {
  const lower = value.trim().toLowerCase();
  if (!lower || lower.includes('url(') || lower.includes('expression') || lower.includes('@import')) {
    return false;
  }
  return /^[a-z0-9\s#.,%()/_-]+$/i.test(value);
}
