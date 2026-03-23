/**
 * Lightweight markdown renderer for the chat widget.
 * Zero dependencies. Security model:
 *   1. HTML-escape all text content at the leaf level (prevents XSS via HTML tags)
 *   2. sanitizeUrl() allowlists schemes for links (prevents javascript: injection)
 *   3. All <a> tags get target="_blank" rel="noopener noreferrer"
 */

// ── HTML escaping ──

function escapeHtml(str: string): string {
  return str
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;')
    .replace(/'/g, '&#39;');
}

// ── URL sanitization ──

const SAFE_URL_RE = /^(https?:|mailto:|tel:|\/|#)/i;

export function sanitizeUrl(url: string): string {
  const trimmed = url.trim();
  if (!trimmed) return '';
  if (trimmed.startsWith('/') || trimmed.startsWith('#')) return trimmed;
  if (SAFE_URL_RE.test(trimmed)) return trimmed;
  return '';
}

// ── Placeholder helpers ──

type PlaceholderMap = Map<string, string>;

let placeholderCounter = 0;

function makePlaceholder(prefix: string): string {
  return `\x00${prefix}_${++placeholderCounter}\x00`;
}

// ── Inline transforms (operates on raw text, escapes at leaf level) ──

const BR_PLACEHOLDER = '\x00BR\x00';

function applyInlineTransforms(raw: string, inlineCodeMap: PlaceholderMap): string {
  // 1. Inline code → placeholders (content is escaped, not processed further)
  let result = raw.replace(/`([^`\n]+?)`/g, (_match, code) => {
    const ph = makePlaceholder('IC');
    inlineCodeMap.set(ph, `<code>${escapeHtml(code)}</code>`);
    return ph;
  });

  // 2. Escape HTML in remaining text (between placeholders and <br> markers)
  const parts = result.split(/(\x00(?:IC_\d+|BR)\x00)/);
  result = parts.map(part => {
    if (part.startsWith('\x00') && part.endsWith('\x00')) return part;
    return escapeHtml(part);
  }).join('');

  // 3. Bold+italic nesting: **text *inner*** → <strong>text <em>inner</em></strong>
  result = result.replace(/\*\*([^*]*)\*([^*]+)\*\*\*/g, '<strong>$1<em>$2</em></strong>');

  // 4. Bold: **text**
  result = result.replace(/\*\*(.+?)\*\*/g, '<strong>$1</strong>');

  // 5. Italic: *text* (must run after bold)
  result = result.replace(/(?<!\*)\*(?!\*)(.+?)(?<!\*)\*(?!\*)/g, '<em>$1</em>');

  // 6. Links: [text](url)
  result = result.replace(/\[([^\]]+)\]\(([^)]+)\)/g, (_match, text, url) => {
    const safeUrl = sanitizeUrl(url);
    return `<a href="${safeUrl}" target="_blank" rel="noopener noreferrer">${text}</a>`;
  });

  // 7. Restore <br> placeholders
  result = result.split(BR_PLACEHOLDER).join('<br>');

  return result;
}

// ── Block-level processing (operates on raw lines) ──

function isHorizontalRule(line: string): boolean {
  const trimmed = line.trim();
  return /^[-*_]{3,}$/.test(trimmed);
}

function processBlocks(lines: string[], inlineCodeMap: PlaceholderMap): string {
  const output: string[] = [];
  let i = 0;

  while (i < lines.length) {
    const line = lines[i];

    // Horizontal rule
    if (isHorizontalRule(line)) {
      output.push('<hr>');
      i++;
      continue;
    }

    // Headings: # through ###
    const headingMatch = line.match(/^(#{1,3})\s+(.+)$/);
    if (headingMatch) {
      const level = headingMatch[1].length + 2; // # → h3, ## → h4, ### → h5
      const content = applyInlineTransforms(headingMatch[2], inlineCodeMap);
      output.push(`<h${level}>${content}</h${level}>`);
      i++;
      continue;
    }

    // Unordered list: - or *  (require space after to avoid matching ---)
    if (/^[-*]\s+/.test(line)) {
      const items: string[] = [];
      while (i < lines.length && /^[-*]\s+/.test(lines[i])) {
        const content = applyInlineTransforms(lines[i].replace(/^[-*]\s+/, ''), inlineCodeMap);
        items.push(`<li>${content}</li>`);
        i++;
      }
      output.push(`<ul>${items.join('')}</ul>`);
      continue;
    }

    // Ordered list: 1.
    if (/^\d+\.\s+/.test(line)) {
      const items: string[] = [];
      while (i < lines.length && /^\d+\.\s+/.test(lines[i])) {
        const content = applyInlineTransforms(lines[i].replace(/^\d+\.\s+/, ''), inlineCodeMap);
        items.push(`<li>${content}</li>`);
        i++;
      }
      output.push(`<ol>${items.join('')}</ol>`);
      continue;
    }

    // Blockquote: >
    if (/^>\s?/.test(line)) {
      const quoteLines: string[] = [];
      while (i < lines.length && /^>\s?/.test(lines[i])) {
        quoteLines.push(lines[i].replace(/^>\s?/, ''));
        i++;
      }
      const content = applyInlineTransforms(quoteLines.join(BR_PLACEHOLDER), inlineCodeMap);
      output.push(`<blockquote>${content}</blockquote>`);
      continue;
    }

    // Empty line — paragraph break
    if (line.trim() === '') {
      i++;
      continue;
    }

    // Paragraph: consecutive non-empty, non-special lines
    const paraLines: string[] = [];
    while (
      i < lines.length &&
      lines[i].trim() !== '' &&
      !isHorizontalRule(lines[i]) &&
      !/^#{1,3}\s+/.test(lines[i]) &&
      !/^[-*]\s+/.test(lines[i]) &&
      !/^\d+\.\s+/.test(lines[i]) &&
      !/^>\s?/.test(lines[i])
    ) {
      paraLines.push(lines[i]);
      i++;
    }
    if (paraLines.length > 0) {
      const content = applyInlineTransforms(paraLines.join(BR_PLACEHOLDER), inlineCodeMap);
      output.push(`<p>${content}</p>`);
    }
  }

  return output.join('');
}

// ── Main entry point ──

export function renderMarkdown(text: string): string {
  if (!text) return '';

  placeholderCounter = 0;

  // Phase 1: Extract fenced code blocks before any processing
  const codeBlockMap: PlaceholderMap = new Map();
  let processed = text.replace(/```(\w*)\n([\s\S]*?)```/g, (_match, lang, code) => {
    const ph = makePlaceholder('CB');
    const langAttr = lang ? ` class="language-${lang}"` : '';
    codeBlockMap.set(ph, `<pre><code${langAttr}>${escapeHtml(code.replace(/\n$/, ''))}</code></pre>`);
    return ph;
  });

  // Phase 2: Split into lines, process blocks (block detection on raw text, escaping at leaf level)
  const lines = processed.split('\n');
  const inlineCodeMap: PlaceholderMap = new Map();
  let result = processBlocks(lines, inlineCodeMap);

  // Phase 3: Restore placeholders
  for (const [ph, html] of inlineCodeMap) {
    result = result.replace(ph, html);
  }
  for (const [ph, html] of codeBlockMap) {
    result = result.replace(ph, html);
  }

  return result;
}
