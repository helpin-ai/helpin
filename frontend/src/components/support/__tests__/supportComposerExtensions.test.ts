// @vitest-environment jsdom
import { Editor, type Content } from '@tiptap/core';
import { createElement } from 'react';
import { renderToStaticMarkup } from 'react-dom/server';
import Markdown from 'react-markdown';
import remarkGfm from 'remark-gfm';
import { afterEach, describe, expect, it } from 'vitest';
import { createSupportComposerExtensions, normalizeSupportLinkInput } from '../supportComposerExtensions';

let editor: Editor | undefined;
afterEach(() => editor?.destroy());

function createEditor(content: Content = '') {
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
    'https://contentstudio.io/?_ptxn=txn_01m4tvpfyjv4rq2v9qky346agq',
    'https://example.com/_files/a*b?first=one&second=two#_section',
  ])('preserves pasted URL %s in replies, notes, and restored drafts', (url) => {
    const instance = createEditor();
    const paste = new Event('paste', { bubbles: true, cancelable: true });
    Object.defineProperty(paste, 'clipboardData', { value: {
      getData: (type: string) => type === 'text/plain' ? url : '',
    } });
    instance.view.dom.dispatchEvent(paste);
    expect(hrefs(instance)).toEqual([url]);

    const markdown = (instance.storage as unknown as { markdown: { getMarkdown(): string } }).markdown.getMarkdown();
    const rendered = document.createElement('div');
    rendered.innerHTML = renderToStaticMarkup(createElement(Markdown, { remarkPlugins: [remarkGfm] }, markdown));
    expect(rendered.querySelector('a')?.getAttribute('href')).toBe(url);
    expect(rendered.textContent).toBe(url);
    instance.commands.setContent(markdown);
    expect(hrefs(instance)).toEqual([url]);
    expect(instance.getText()).toBe(url);
  });

  it('preserves literal Markdown characters in ordinary text and named links', () => {
    const instance = createEditor({
      type: 'doc', content: [{ type: 'paragraph', content: [
        { type: 'text', text: '*literal* _text_ <tag> ' },
        { type: 'text', text: '_billing_', marks: [{ type: 'link', attrs: { href: 'https://example.com/?_ref=one' } }] },
      ] }],
    });
    const text = instance.getText();
    const markdown = (instance.storage as unknown as { markdown: { getMarkdown(): string } }).markdown.getMarkdown();
    instance.commands.setContent(markdown);
    expect(instance.getText()).toBe(text);
    expect(hrefs(instance)).toEqual(['https://example.com/?_ref=one']);
    expect(instance.getHTML()).not.toMatch(/<(em|strong|tag)>/);
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
