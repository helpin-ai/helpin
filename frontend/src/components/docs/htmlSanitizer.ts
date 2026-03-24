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
  'colspan', 'rowspan',
];

export function sanitizeHtml(html: string): string {
  return DOMPurify.sanitize(html, {
    ALLOWED_TAGS,
    ALLOWED_ATTR,
    ALLOW_DATA_ATTR: true,
    ALLOWED_URI_REGEXP: /^(?:(?:https?|mailto|tel):|[^a-z]|[a-z+.-]+(?:[^a-z+.\-:]|$))/i,
  });
}
