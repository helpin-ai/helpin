import { Node, mergeAttributes } from '@tiptap/core';
import { ReactNodeViewRenderer } from '@tiptap/react';
import { ArtifactVideoNodeView } from './ArtifactVideoNodeView';
import { pickBlockNodeViewAttrs } from './nodeViewAttrs';

export interface ArtifactVideoOptions {
  workspaceId?: string;
}

export interface ArtifactVideoAttrs {
  src: string;
  artifactId: string;
  fileName?: string | null;
  contentType?: string | null;
  description: string;
  caption?: string | null;
}

declare module '@tiptap/core' {
  interface Commands<ReturnType> {
    artifactVideo: {
      setArtifactVideo: (attrs: ArtifactVideoAttrs) => ReturnType;
    };
  }
}

export const ArtifactVideoExtension = Node.create<ArtifactVideoOptions>({
  name: 'artifactVideo',
  group: 'block',
  atom: true,
  draggable: true,

  addOptions() {
    return { workspaceId: undefined };
  },

  addAttributes() {
    return {
      src: { default: null },
      artifactId: { default: null },
      fileName: { default: null },
      contentType: { default: 'video/mp4' },
      description: { default: null },
      caption: { default: null },
    };
  },

  parseHTML() {
    return [{
      tag: 'div[data-artifact-video]',
      getAttrs: (element) => {
        const dom = element as HTMLElement;
        return {
          artifactId: dom.getAttribute('data-artifact-video'),
          src: dom.getAttribute('data-artifact-src'),
          fileName: dom.getAttribute('data-file-name'),
          contentType: dom.getAttribute('data-content-type'),
          description: dom.getAttribute('data-description'),
          caption: dom.getAttribute('data-caption'),
        };
      },
    }];
  },

  renderHTML({ HTMLAttributes }) {
    return ['div', mergeAttributes({
      'data-artifact-video': HTMLAttributes.artifactId,
      'data-artifact-src': HTMLAttributes.src,
      'data-file-name': HTMLAttributes.fileName,
      'data-content-type': HTMLAttributes.contentType,
      'data-description': HTMLAttributes.description,
      ...(HTMLAttributes.caption ? { 'data-caption': HTMLAttributes.caption } : {}),
    }), HTMLAttributes.fileName || 'Private video recording'];
  },

  addNodeView() {
    return ReactNodeViewRenderer(ArtifactVideoNodeView, { attrs: ({ HTMLAttributes }) => pickBlockNodeViewAttrs(HTMLAttributes) });
  },

  addCommands() {
    return {
      setArtifactVideo:
        (attrs) =>
        ({ commands }) =>
          commands.insertContent({ type: this.name, attrs }),
    };
  },
});
