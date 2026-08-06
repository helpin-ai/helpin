import { describe, it, expect } from 'vitest';
import { renderMarkdown, sanitizeUrl } from '../utils/markdownRenderer';

describe('renderMarkdown', () => {
  it('renders fenced code blocks with language hint', () => {
    const input = '```js\nconst x = 1;\n```';
    const result = renderMarkdown(input);
    expect(result).toContain('<pre><code class="language-js">const x = 1;</code></pre>');
  });

  it('does not process markdown inside fenced code blocks', () => {
    const input = '```\n**not bold** *not italic*\n```';
    const result = renderMarkdown(input);
    expect(result).toContain('**not bold** *not italic*');
    expect(result).not.toContain('<strong>');
    expect(result).not.toContain('<em>');
  });

  it('renders nested emphasis: bold with italic inside', () => {
    const input = '**bold *and italic***';
    const result = renderMarkdown(input);
    expect(result).toContain('<strong>bold <em>and italic</em></strong>');
  });

  it('groups consecutive unordered list lines into a single <ul>', () => {
    const input = '- item one\n- item two\n- item three';
    const result = renderMarkdown(input);
    expect(result).toBe('<ul><li>item one</li><li>item two</li><li>item three</li></ul>');
  });

  it('groups consecutive ordered list lines into a single <ol>', () => {
    const input = '1. first\n2. second\n3. third';
    const result = renderMarkdown(input);
    expect(result).toBe('<ol><li>first</li><li>second</li><li>third</li></ol>');
  });

  it('renders safe links with target="_blank"', () => {
    const input = '[Example](https://example.com)';
    const result = renderMarkdown(input);
    expect(result).toContain('<a href="https://example.com" target="_blank" rel="noopener noreferrer">Example</a>');
  });

  it('auto-links bare urls', () => {
    const input = 'Visit https://example.com/docs for details.';
    const result = renderMarkdown(input);
    expect(result).toContain('<a href="https://example.com/docs" target="_blank" rel="noopener noreferrer">https://example.com/docs</a>');
  });

  it('blocks javascript: URLs', () => {
    const input = '[click me](javascript:alert(1))';
    const result = renderMarkdown(input);
    expect(result).toContain('href=""');
    expect(result).not.toContain('javascript:');
  });

  it('blocks data: URLs', () => {
    const input = '[x](data:text/html,<script>alert(1)</script>)';
    const result = renderMarkdown(input);
    expect(result).toContain('href=""');
    expect(result).not.toContain('data:');
  });

  it('escapes raw HTML to prevent XSS', () => {
    const input = '<script>alert("xss")</script>';
    const result = renderMarkdown(input);
    expect(result).toContain('&lt;script&gt;');
    expect(result).not.toContain('<script>');
  });

  it('creates separate <p> blocks for double newlines', () => {
    const input = 'paragraph one\n\nparagraph two';
    const result = renderMarkdown(input);
    expect(result).toBe('<p>paragraph one</p><p>paragraph two</p>');
  });

  it('converts single newlines within a paragraph to <br>', () => {
    const input = 'line one\nline two';
    const result = renderMarkdown(input);
    expect(result).toBe('<p>line one<br>line two</p>');
  });

  it('treats markdown hard-break escapes before newlines as line breaks', () => {
    const input = 'Hi Caleb,\\\n\\\nThank you for reaching out.';
    const result = renderMarkdown(input);
    expect(result).toBe('<p>Hi Caleb,</p><p>Thank you for reaching out.</p>');
  });

  it('removes repeated hard-break backslashes before newlines', () => {
    const input = 'How may we help you?\\\\\n\\\\\nWhich plan are you interested in?';
    const result = renderMarkdown(input);
    expect(result).toBe('<p>How may we help you?</p><p>Which plan are you interested in?</p>');
  });

  it('renders headings downscaled to h3-h5', () => {
    const input = '# Heading 1\n## Heading 2\n### Heading 3';
    const result = renderMarkdown(input);
    expect(result).toContain('<h3>Heading 1</h3>');
    expect(result).toContain('<h4>Heading 2</h4>');
    expect(result).toContain('<h5>Heading 3</h5>');
  });

  it('renders blockquotes', () => {
    const input = '> quoted text\n> more quoted';
    const result = renderMarkdown(input);
    expect(result).toBe('<blockquote>quoted text<br>more quoted</blockquote>');
  });

  it('renders GitHub-style tables', () => {
    const input = '| Feature | ContentStudio | Publer |\n| --- | --- | --- |\n| Planner Views | Calendar, Feed, List | Basic calendar |\n| AI Tools | Captions, hashtags | Basic AI |';
    const result = renderMarkdown(input);
    expect(result).toContain('<div class="helpin-table-wrap"><table>');
    expect(result).toContain('<thead><tr><th>Feature</th><th>ContentStudio</th><th>Publer</th></tr></thead>');
    expect(result).toContain('<tbody><tr><td>Planner Views</td><td>Calendar, Feed, List</td><td>Basic calendar</td></tr><tr><td>AI Tools</td><td>Captions, hashtags</td><td>Basic AI</td></tr></tbody>');
  });

  it('renders inline markdown inside table cells', () => {
    const input = '| Feature | Winner |\n| --- | --- |\n| AI Tools | **ContentStudio** |\n| Docs | [Read more](https://example.com) |';
    const result = renderMarkdown(input);
    expect(result).toContain('<td><strong>ContentStudio</strong></td>');
    expect(result).toContain('<td><a href="https://example.com" target="_blank" rel="noopener noreferrer">Read more</a></td>');
  });

  it('renders horizontal rules', () => {
    const input = 'above\n\n---\n\nbelow';
    const result = renderMarkdown(input);
    expect(result).toContain('<hr>');
    expect(result).toContain('<p>above</p>');
    expect(result).toContain('<p>below</p>');
  });

  it('renders inline code without processing inner markdown', () => {
    const input = 'Use `**not bold**` here';
    const result = renderMarkdown(input);
    expect(result).toContain('<code>**not bold**</code>');
    expect(result).not.toContain('<strong>');
  });

  it('handles mixed blocks: heading → paragraph → list → code block', () => {
    const input = '# Title\n\nSome text here.\n\n- item a\n- item b\n\n```py\nprint("hi")\n```';
    const result = renderMarkdown(input);
    expect(result).toContain('<h3>Title</h3>');
    expect(result).toContain('<p>Some text here.</p>');
    expect(result).toContain('<ul><li>item a</li><li>item b</li></ul>');
    expect(result).toContain('<pre><code class="language-py">print(&quot;hi&quot;)</code></pre>');
  });

  it('returns empty string for empty input', () => {
    expect(renderMarkdown('')).toBe('');
  });
});

describe('sanitizeUrl', () => {
  it('allows https URLs', () => {
    expect(sanitizeUrl('https://example.com')).toBe('https://example.com');
  });

  it('allows http URLs', () => {
    expect(sanitizeUrl('http://example.com')).toBe('http://example.com');
  });

  it('allows mailto URLs', () => {
    expect(sanitizeUrl('mailto:test@example.com')).toBe('mailto:test@example.com');
  });

  it('allows tel URLs', () => {
    expect(sanitizeUrl('tel:+1234567890')).toBe('tel:+1234567890');
  });

  it('allows relative paths', () => {
    expect(sanitizeUrl('/path/to/page')).toBe('/path/to/page');
  });

  it('allows hash links', () => {
    expect(sanitizeUrl('#section')).toBe('#section');
  });

  it('blocks javascript: URLs', () => {
    expect(sanitizeUrl('javascript:alert(1)')).toBe('');
  });

  it('blocks data: URLs', () => {
    expect(sanitizeUrl('data:text/html,<script>')).toBe('');
  });

  it('blocks vbscript: URLs', () => {
    expect(sanitizeUrl('vbscript:MsgBox("XSS")')).toBe('');
  });

  it('returns empty for empty input', () => {
    expect(sanitizeUrl('')).toBe('');
  });
});
