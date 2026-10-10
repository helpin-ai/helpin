// @vitest-environment jsdom
import { Editor } from '@tiptap/core';
import { afterEach, describe, expect, it } from 'vitest';
import { createSupportComposerExtensions, normalizeSupportLinkInput } from '../supportComposerExtensions';

let editor: Editor | undefined;
afterEach(() => editor?.destroy());

function createEditor(content = '') {
  editor = new Editor({ extensions: createSupportComposerExtensions(), content });
  return editor;
}

function hrefs(instance: Editor) {
  const element = document.createElement('div');
  element.innerHTML = instance.getHTML();
  return [...element.querySelectorAll('a')].map(link => link.getAttribute('href'));
}

describe('support composer links', () => {
  it('uses HTTPS when a bare domain is typed', () => {
    const instance = createEditor();
    instance.view.dispatch(instance.state.tr.insertText('help.example.com '));
    expect(hrefs(instance)).toEqual(['https://help.example.com']);
  });

  it('uses HTTPS in drafts, inserted Markdown, and pasted text', () => {
    const instance = createEditor('Visit **help.example.com** for help.');
    expect(hrefs(instance)).toEqual(['https://help.example.com']);
    expect(instance.getHTML()).toContain('<strong>');
    instance.commands.setContent('Visit www.example.com/path.');
    expect(hrefs(instance)).toEqual(['https://www.example.com/path']);
    instance.commands.setContent('');
    const paste = new Event('paste', { bubbles: true, cancelable: true });
    Object.defineProperty(paste, 'clipboardData', { value: {
      getData: (type: string) => type === 'text/plain' ? 'Visit **help.example.com/docs** for help.' : '',
    } });
    instance.view.dom.dispatchEvent(paste);
    expect(hrefs(instance)).toEqual(['https://help.example.com/docs']);
    expect(instance.getHTML()).toContain('<strong>');
  });

  it('preserves explicit HTTP, HTTPS, email, and relative links', () => {
    const instance = createEditor('http://example.com https://example.org user@example.com [Article](/docs/article)');
    expect(hrefs(instance)).toEqual(['http://example.com', 'https://example.org', 'mailto:user@example.com', '/docs/article']);
    instance.commands.setContent('[Named link](http://example.com) and `help.example.com`');
    expect(hrefs(instance)).toEqual(['http://example.com']);
  });

  it('keeps the HTTPS URL when a draft is saved and restored', () => {
    const instance = createEditor('help.example.com');
    const markdown = (instance.storage as unknown as { markdown: { getMarkdown(): string } }).markdown.getMarkdown();
    expect(markdown).toContain('(https://help.example.com)');
    instance.commands.setContent(markdown);
    expect(hrefs(instance)).toEqual(['https://help.example.com']);
  });

  it.each([
    [' help.example.com/docs ', 'https://help.example.com/docs'],
    ['//help.example.com/docs', 'https://help.example.com/docs'],
    ['http://example.com', 'http://example.com'],
    ['https://example.com', 'https://example.com'],
    ['mailto:user@example.com', 'mailto:user@example.com'],
    ['tel:+123456789', 'tel:+123456789'],
    ['/docs/article', '/docs/article'],
    ['../article', '../article'],
    ['#section', '#section'],
    ['', ''],
  ])('normalizes manually entered link %s without changing explicit destinations', (input, expected) => {
    expect(normalizeSupportLinkInput(input)).toBe(expected);
  });
});
