import { Node, mergeAttributes } from '@tiptap/core';
import { ReactNodeViewRenderer } from '@tiptap/react';
import { ResizableImageComponent } from './resizable-image-component';
import { pickBlockNodeViewAttrs } from '@/components/docs/nodeViewAttrs';

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
        caption?: string | null;
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
      width: { default: '100%' },
      height: { default: 'auto' },
      aspectRatio: { default: null },
      attachmentId: { default: null },
      caption: { default: null },
      alignment: { default: 'center' },
      linkUrl: { default: null },
      linkNewTab: { default: true },
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
          caption: dom.getAttribute('data-caption'),
          alignment: dom.getAttribute('data-alignment') || 'center',
          linkUrl: dom.getAttribute('data-link-url') || null,
          linkNewTab: dom.getAttribute('data-link-new-tab') !== 'false',
        };
      }},
    ];
  },

  renderHTML({ HTMLAttributes }) {
    const { aspectRatio, attachmentId, caption, alignment, linkUrl, linkNewTab, ...rest } = HTMLAttributes;
    return ['img', mergeAttributes(this.options.HTMLAttributes, rest, {
      ...(aspectRatio ? { 'data-aspect-ratio': aspectRatio } : {}),
      ...(attachmentId ? { 'data-attachment-id': attachmentId } : {}),
      ...(caption ? { 'data-caption': caption } : {}),
      ...(alignment && alignment !== 'center' ? { 'data-alignment': alignment } : {}),
      ...(linkUrl ? { 'data-link-url': linkUrl, 'data-link-new-tab': String(linkNewTab ?? true) } : {}),
    })];
  },

  addNodeView() {
    return ReactNodeViewRenderer(ResizableImageComponent, { attrs: ({ HTMLAttributes }) => pickBlockNodeViewAttrs(HTMLAttributes) });
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
