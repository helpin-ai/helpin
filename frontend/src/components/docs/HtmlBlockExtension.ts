import { Node, mergeAttributes } from '@tiptap/core';
import { ReactNodeViewRenderer } from '@tiptap/react';
import { HtmlBlockNodeView } from './HtmlBlockNodeView';
import { sanitizeHtml } from './htmlSanitizer';
import { shouldRenderHtmlBlockIsolated } from './htmlBlockRendering';
import { pickBlockNodeViewAttrs } from './nodeViewAttrs';

declare module '@tiptap/core' {
  interface Commands<ReturnType> {
    htmlBlock: {
      setHtmlBlock: (attrs?: { html?: string; renderMode?: string }) => ReturnType;
    };
  }
}

export const HtmlBlockExtension = Node.create({
  name: 'htmlBlock',
  group: 'block',
  atom: true,
  draggable: true,

  addAttributes() {
    return {
      html: { default: '' },
      renderMode: { default: 'inline' },
    };
  },

  parseHTML() {
    return [
      {
        tag: 'div[data-html-block]',
        getAttrs: (el) => ({
          html: (el as HTMLElement).innerHTML,
          renderMode: (el as HTMLElement).getAttribute('data-render-mode') || 'inline',
        }),
      },
    ];
  },

  renderHTML({ HTMLAttributes }) {
    const rawHTML = HTMLAttributes.html || '';
    const isolated = shouldRenderHtmlBlockIsolated(rawHTML, HTMLAttributes.renderMode);
    const innerHTML = isolated
      ? createHtmlBlockFrameHTML(rawHTML)
      : sanitizeHtml(rawHTML);
    const dom = createHtmlBlockDOM(innerHTML, isolated ? 'sandboxed' : 'inline');
    if (dom) return dom;

    const attrs = isolated
      ? { 'data-html-block': '', 'data-render-mode': 'sandboxed' }
      : { 'data-html-block': '' };
    return ['div', mergeAttributes(attrs)];
  },

  addNodeView() {
    return ReactNodeViewRenderer(HtmlBlockNodeView, { attrs: ({ HTMLAttributes }) => pickBlockNodeViewAttrs(HTMLAttributes) });
  },

  addCommands() {
    return {
      setHtmlBlock:
        (attrs) =>
        ({ commands }) =>
          commands.insertContent({
            type: this.name,
            attrs: { html: attrs?.html ?? '', renderMode: attrs?.renderMode ?? 'inline' },
          }),
    };
  },

  addStorage() {
    return {
      markdown: {
        serialize(state: any, node: any) {
          const raw = node.attrs.html || '';
          if (raw) {
            if (node.attrs.renderMode === 'sandboxed') {
              state.write(`<div data-html-block data-render-mode="sandboxed">${raw}</div>`);
              state.closeBlock(node);
              return;
            }
            const safe = sanitizeHtml(raw);
            if (safe) {
              state.write(`<div data-html-block>${safe}</div>`);
              state.closeBlock(node);
            }
          }
        },
        parse: {},
      },
    };
  },
});

function createHtmlBlockDOM(html: string, renderMode: 'inline' | 'sandboxed'): HTMLElement | null {
  if (typeof document === 'undefined') return null;
  const element = document.createElement('div');
  element.setAttribute('data-html-block', '');
  if (renderMode === 'sandboxed') {
    element.setAttribute('data-render-mode', 'sandboxed');
  }
  element.innerHTML = html;
  return element;
}

function createHtmlBlockFrameHTML(html: string): string {
  if (typeof document === 'undefined') return '';
  const iframe = document.createElement('iframe');
  iframe.className = 'docs-html-block-frame';
  iframe.setAttribute('title', 'Rendered HTML block');
  iframe.setAttribute('sandbox', 'allow-scripts allow-popups allow-forms allow-presentation');
  iframe.setAttribute('referrerpolicy', 'no-referrer');
  iframe.setAttribute('srcdoc', html);
  iframe.setAttribute('style', 'width:100%;height:720px;border:0;background:#fff;border-radius:6px;');
  return iframe.outerHTML;
}
