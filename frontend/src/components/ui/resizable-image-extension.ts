import { Node, mergeAttributes, type JSONContent } from '@tiptap/core';
import { ReactNodeViewRenderer } from '@tiptap/react';
import { ResizableImageComponent } from './resizable-image-component';
import { pickBlockNodeViewAttrs } from '@/components/editor/nodeViewAttrs';
import { parseAnnotationState } from '@/components/docs/annotator/core/annotationTypes';
import type { EditorUploadConfig } from '@/hooks/useEditorImageUpload';

export interface ResizableImageOptions {
  HTMLAttributes: Record<string, unknown>;
  enableCaption: boolean;
  defaultAlignment: 'left' | 'center' | 'right';
  workspaceId?: string;
  documentId?: string;
  /** Required for image annotation — without it the annotate action is hidden. */
  uploadConfig?: EditorUploadConfig;
  /** Persists node-view mutations that must survive an immediate page refresh. */
  onImmediateSave?: (content: JSONContent) => void | Promise<void>;
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
        artifactId?: string | null;
        caption?: string | null;
        annotationState?: unknown;
        sourceAttachmentId?: string | null;
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
      enableCaption: true,
      defaultAlignment: 'center',
      workspaceId: undefined,
      documentId: undefined,
      uploadConfig: undefined,
      onImmediateSave: undefined,
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
      artifactId: { default: null },
      caption: { default: null },
      alignment: { default: this.options.defaultAlignment },
      linkUrl: { default: null },
      linkNewTab: { default: true },
      // Annotation state is kept so an annotated image can be re-opened and edited. It is
      // stripped at publish time — see docsPublishTransforms.
      annotationState: { default: null },
      // The un-annotated original. Re-editing always reloads from here, never from `src`, so
      // annotations never compound onto already-flattened pixels.
      sourceAttachmentId: { default: null },
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
          artifactId: dom.getAttribute('data-artifact-id'),
          caption: dom.getAttribute('data-caption'),
          alignment: dom.getAttribute('data-alignment') || this.options.defaultAlignment,
          linkUrl: dom.getAttribute('data-link-url') || null,
          linkNewTab: dom.getAttribute('data-link-new-tab') !== 'false',
          // Never throws: malformed state parses to null rather than breaking document load.
          annotationState: parseAnnotationState(dom.getAttribute('data-annotation')),
          sourceAttachmentId: dom.getAttribute('data-source-attachment-id') || null,
        };
      }},
    ];
  },

  renderHTML({ HTMLAttributes }) {
    const {
      aspectRatio,
      attachmentId,
      artifactId,
      caption,
      alignment,
      linkUrl,
      linkNewTab,
      annotationState,
      sourceAttachmentId,
      ...rest
    } = HTMLAttributes;
    return ['img', mergeAttributes(this.options.HTMLAttributes, rest, {
      ...(aspectRatio ? { 'data-aspect-ratio': aspectRatio } : {}),
      ...(attachmentId ? { 'data-attachment-id': attachmentId } : {}),
      ...(artifactId ? { 'data-artifact-id': artifactId } : {}),
      ...(this.options.enableCaption && caption ? { 'data-caption': caption } : {}),
      ...(alignment && alignment !== 'center' ? { 'data-alignment': alignment } : {}),
      ...(linkUrl ? { 'data-link-url': linkUrl, 'data-link-new-tab': String(linkNewTab ?? true) } : {}),
      ...(annotationState ? { 'data-annotation': JSON.stringify(annotationState) } : {}),
      ...(sourceAttachmentId ? { 'data-source-attachment-id': sourceAttachmentId } : {}),
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
