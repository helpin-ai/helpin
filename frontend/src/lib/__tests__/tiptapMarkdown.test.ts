// @vitest-environment jsdom
import { describe, expect, it } from 'vitest';

import { htmlToMarkdown, markdownToHtml } from '@/lib/tiptapMarkdown';

describe('tiptapMarkdown', () => {
  it('converts markdown into rendered html', () => {
    const html = markdownToHtml('# Title\n\n- one\n- two');
    expect(html).toContain('<h1>Title</h1>');
    expect(html).toContain('<ul');
    expect(html.endsWith('<p></p>')).toBe(false);
  });

  it('converts rich html into markdown source', () => {
    const markdown = htmlToMarkdown('<h2>Scope</h2><p>Build it</p><ul><li>First</li></ul>');
    expect(markdown).toContain('## Scope');
    expect(markdown).toContain('- First');
  });
});
