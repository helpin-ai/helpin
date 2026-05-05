import { Node, mergeAttributes } from '@tiptap/core';
import { ReactNodeViewRenderer } from '@tiptap/react';
import { HtmlBlockNodeView } from './HtmlBlockNodeView';
import { sanitizeHtml } from './htmlSanitizer';
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
    if (HTMLAttributes.renderMode === 'sandboxed') {
      return [
        'div',
        mergeAttributes({ 'data-html-block': '', 'data-render-mode': 'sandboxed' }),
        ['iframe', {
          class: 'docs-html-block-frame',
          sandbox: 'allow-scripts allow-forms allow-popups allow-presentation',
          referrerpolicy: 'no-referrer',
          loading: 'lazy',
          srcdoc: HTMLAttributes.html || '',
        }],
      ];
    }
    // Sanitize for export — same policy as server and editor preview
    const safe = sanitizeHtml(HTMLAttributes.html || '');
    return ['div', mergeAttributes({ 'data-html-block': '' }), safe];
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
