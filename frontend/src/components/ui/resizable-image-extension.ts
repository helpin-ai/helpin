import { Node, mergeAttributes } from '@tiptap/core';
import { ReactNodeViewRenderer } from '@tiptap/react';
import { ResizableImageComponent } from './resizable-image-component';

export interface ResizableImageOptions {
  HTMLAttributes: Record<string, unknown>;
}

declare module '@tiptap/core' {
  interface Commands<ReturnType> {
    resizableImage: {
      setResizableImage: (options: {
        src: string;
        alt?: string;
        title?: string | null;
        width?: string;
        height?: string;
        aspectRatio?: number | null;
        attachmentId?: string | null;
      }) => ReturnType;
    };
  }
}

export const ResizableImageExtension = Node.create<ResizableImageOptions>({
  name: 'resizableImage',
  group: 'block',
  atom: true,
  draggable: true,

  addOptions() {
    return {
      HTMLAttributes: {},
    };
  },

  addAttributes() {
    return {
      src: { default: null },
      alt: { default: null },
      title: { default: null },
      width: { default: '35%' },
      height: { default: 'auto' },
      aspectRatio: { default: null },
      attachmentId: { default: null },
    };
  },

  parseHTML() {
    return [
      { tag: 'img[src]', getAttrs: (el) => {
        const dom = el as HTMLElement;
        return {
          src: dom.getAttribute('src'),
          alt: dom.getAttribute('alt'),
          title: dom.getAttribute('title'),
          width: dom.getAttribute('width') || dom.style.width || '35%',
          height: dom.getAttribute('height') || dom.style.height || 'auto',
          aspectRatio: dom.getAttribute('data-aspect-ratio')
            ? Number(dom.getAttribute('data-aspect-ratio'))
            : null,
          attachmentId: dom.getAttribute('data-attachment-id'),
        };
      }},
    ];
  },

  renderHTML({ HTMLAttributes }) {
    const { aspectRatio, attachmentId, ...rest } = HTMLAttributes;
    return ['img', mergeAttributes(this.options.HTMLAttributes, rest, {
      ...(aspectRatio ? { 'data-aspect-ratio': aspectRatio } : {}),
      ...(attachmentId ? { 'data-attachment-id': attachmentId } : {}),
    })];
  },

  addNodeView() {
    return ReactNodeViewRenderer(ResizableImageComponent);
  },

  addCommands() {
    return {
      setResizableImage:
        (options) =>
        ({ commands }) =>
          commands.insertContent({
            type: this.name,
            attrs: options,
          }),
    };
  },
});
