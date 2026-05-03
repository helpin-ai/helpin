import { Node, mergeAttributes, type JSONContent } from '@tiptap/core';
import { ReactNodeViewRenderer } from '@tiptap/react';
import { ExcalidrawNodeView } from './ExcalidrawNodeView';
import { normalizeExcalidrawScene } from '@/lib/excalidrawRenderer';

export type ExcalidrawOptions = {
  onImmediateSave?: (content: JSONContent) => void | Promise<void>;
};

declare module '@tiptap/core' {
  interface Commands<ReturnType> {
    excalidraw: {
      setExcalidraw: (attrs?: { title?: string; scene?: unknown }) => ReturnType;
    };
  }
}

function encodeAttr(value: unknown): string {
  return String(value)
    .replace(/&/g, '&amp;')
    .replace(/"/g, '&quot;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;');
}

export const ExcalidrawExtension = Node.create<ExcalidrawOptions>({
  name: 'excalidraw',
  group: 'block',
  atom: true,
  draggable: true,

  addOptions() {
    return {
      onImmediateSave: undefined,
    };
  },

  addAttributes() {
    return {
      title: { default: 'Excalidraw drawing' },
      scene: { default: { elements: [], appState: {}, files: {} } },
      width: { default: '100%' },
      height: { default: 'auto' },
      alignment: { default: 'center' },
    };
  },

  parseHTML() {
    return [
      {
        tag: 'div[data-excalidraw]',
        getAttrs: (el) => {
          const dom = el as HTMLElement;
          let scene: unknown = { elements: [], appState: {}, files: {} };
          const rawScene = dom.getAttribute('data-excalidraw-scene');
          if (rawScene) {
            try {
              scene = JSON.parse(rawScene);
            } catch {
              scene = { elements: [], appState: {}, files: {} };
            }
          }
          return {
            title: dom.getAttribute('data-excalidraw-title') || 'Excalidraw drawing',
            scene: normalizeExcalidrawScene(scene),
          };
        },
      },
    ];
  },

  renderHTML({ HTMLAttributes }) {
    const scene = normalizeExcalidrawScene(HTMLAttributes.scene);
    return ['div', mergeAttributes(HTMLAttributes, {
      'data-excalidraw': '',
      'data-excalidraw-title': HTMLAttributes.title || 'Excalidraw drawing',
      'data-excalidraw-scene': JSON.stringify(scene),
    }), HTMLAttributes.title || 'Excalidraw drawing'];
  },

  addNodeView() {
    return ReactNodeViewRenderer(ExcalidrawNodeView);
  },

  addStorage() {
    return {
      markdown: {
        serialize(
          this: { editor?: { storage?: { markdown?: { options?: { html?: boolean } } } } },
          state: any,
          node: any,
        ) {
          const title = node.attrs.title || 'Excalidraw drawing';
          const scene = JSON.stringify(normalizeExcalidrawScene(node.attrs.scene));
          if ((this as any).editor?.storage?.markdown?.options?.html) {
            state.write(
              `<div data-excalidraw data-excalidraw-title="${encodeAttr(title)}" data-excalidraw-scene="${encodeAttr(scene)}">${encodeAttr(title)}</div>`,
            );
            state.closeBlock(node);
          } else {
            state.write(`[${title}]\n\n`);
          }
        },
        parse: {},
      },
    };
  },

  addCommands() {
    return {
      setExcalidraw:
        (attrs = {}) =>
        ({ commands }) =>
          commands.insertContent({
            type: this.name,
            attrs: {
              title: attrs.title || 'Excalidraw drawing',
              scene: normalizeExcalidrawScene(attrs.scene),
            },
          }),
    };
  },
});
