// @vitest-environment jsdom
import { describe, expect, it } from 'vitest';

import { markdownToHtml } from '@/lib/tiptapMarkdown';
import { normalizePastedMarkdownText } from '../tiptapMarkdownPaste';

describe('normalizePastedMarkdownText', () => {
  it('dedents pasted markdown so it renders as markdown instead of an indented code block', () => {
    const pasted = [
      '    # Implementation Notes',
      '',
      '    - Preserve [links](https://example.com)',
      '    - Keep **important** emphasis',
    ].join('\n');

    const html = markdownToHtml(normalizePastedMarkdownText(pasted));

    expect(html).toContain('<h1>Implementation Notes</h1>');
    expect(html).toContain('<ul');
    expect(html).toContain('<a');
    expect(html).toContain('<strong>important</strong>');
    expect(html).not.toContain('<pre><code>');
  });

  it('leaves indented plain code unchanged', () => {
    const pasted = [
      '    const answer = 42;',
      '    console.log(answer);',
    ].join('\n');

    expect(normalizePastedMarkdownText(pasted)).toBe(pasted);
  });
});
