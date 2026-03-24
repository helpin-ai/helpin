import { Node, mergeAttributes } from '@tiptap/core';
import { ReactNodeViewRenderer } from '@tiptap/react';
import { HtmlBlockNodeView } from './HtmlBlockNodeView';
import { sanitizeHtml } from './htmlSanitizer';

declare module '@tiptap/core' {
  interface Commands<ReturnType> {
    htmlBlock: {
      setHtmlBlock: (attrs?: { html?: string }) => ReturnType;
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
    };
  },

  parseHTML() {
    return [
      {
        tag: 'div[data-html-block]',
        getAttrs: (el) => ({
          html: (el as HTMLElement).innerHTML,
        }),
      },
    ];
  },

  renderHTML({ HTMLAttributes }) {
    // Sanitize for export — same policy as server and editor preview
    const safe = sanitizeHtml(HTMLAttributes.html || '');
    return ['div', mergeAttributes({ 'data-html-block': '' }), safe];
  },

  addNodeView() {
    return ReactNodeViewRenderer(HtmlBlockNodeView);
  },

  addCommands() {
    return {
      setHtmlBlock:
        (attrs) =>
        ({ commands }) =>
          commands.insertContent({
            type: this.name,
            attrs: { html: attrs?.html ?? '' },
          }),
    };
  },

  addStorage() {
    return {
      markdown: {
        serialize(state: any, node: any) {
          const raw = node.attrs.html || '';
          if (raw) {
            const safe = sanitizeHtml(raw);
            if (safe) state.write(safe + '\n\n');
          }
        },
        parse: {},
      },
    };
  },
});
