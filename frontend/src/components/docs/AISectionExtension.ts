import { Node, mergeAttributes } from '@tiptap/core';
import { ReactNodeViewRenderer } from '@tiptap/react';
import { AISectionNodeView, type AISectionStatus } from './AISectionNodeView';
import { pickBlockNodeViewAttrs } from './nodeViewAttrs';

export interface AISectionOptions {
  workspaceId?: string;
  documentId?: string;
}

declare module '@tiptap/core' {
  interface Commands<ReturnType> {
    aiSection: {
      setAISection: (attrs?: {
        title?: string;
        status?: AISectionStatus;
        ownerAgentId?: string | null;
        ownerAgentName?: string | null;
        lastGeneratedAt?: string | null;
        model?: string | null;
        promptHash?: string | null;
        sourceCount?: number;
      }) => ReturnType;
    };
  }
}

export const AISectionExtension = Node.create<AISectionOptions>({
  name: 'aiSection',
  group: 'block',
  content: 'block+',
  defining: true,

  addOptions() {
    return {
      workspaceId: undefined,
      documentId: undefined,
    };
  },

  addAttributes() {
    return {
      title: {
        default: 'AI section',
        parseHTML: (el) => (el as HTMLElement).getAttribute('data-ai-section-title') || 'AI section',
        renderHTML: (attrs) => ({ 'data-ai-section-title': attrs.title || 'AI section' }),
      },
      status: {
        default: 'draft',
        parseHTML: (el) => (el as HTMLElement).getAttribute('data-ai-section-status') || 'draft',
        renderHTML: (attrs) => ({ 'data-ai-section-status': attrs.status || 'draft' }),
      },
      ownerAgentId: { default: null },
      ownerAgentName: {
        default: null,
        parseHTML: (el) => (el as HTMLElement).getAttribute('data-ai-section-agent-name'),
        renderHTML: (attrs) => (
          attrs.ownerAgentName ? { 'data-ai-section-agent-name': attrs.ownerAgentName } : {}
        ),
      },
      lastGeneratedAt: {
        default: null,
        parseHTML: (el) => (el as HTMLElement).getAttribute('data-ai-section-generated-at'),
        renderHTML: (attrs) => (
          attrs.lastGeneratedAt ? { 'data-ai-section-generated-at': attrs.lastGeneratedAt } : {}
        ),
      },
      model: {
        default: null,
        parseHTML: (el) => (el as HTMLElement).getAttribute('data-ai-section-model'),
        renderHTML: (attrs) => (
          attrs.model ? { 'data-ai-section-model': attrs.model } : {}
        ),
      },
      promptHash: { default: null },
      sourceCount: {
        default: 0,
        parseHTML: (el) => Number((el as HTMLElement).getAttribute('data-ai-section-source-count') || 0),
        renderHTML: (attrs) => (
          attrs.sourceCount ? { 'data-ai-section-source-count': String(attrs.sourceCount) } : {}
        ),
      },
    };
  },

  parseHTML() {
    return [{ tag: 'section[data-ai-section]' }];
  },

  renderHTML({ HTMLAttributes }) {
    return ['section', mergeAttributes(HTMLAttributes, { 'data-ai-section': '' }), 0];
  },

  addNodeView() {
    return ReactNodeViewRenderer(AISectionNodeView, { attrs: ({ HTMLAttributes }) => pickBlockNodeViewAttrs(HTMLAttributes) });
  },

  addCommands() {
    return {
      setAISection:
        (attrs) =>
        ({ commands }) =>
          commands.insertContent({
            type: this.name,
            attrs: {
              title: attrs?.title ?? 'AI section',
              status: attrs?.status ?? 'draft',
              ownerAgentId: attrs?.ownerAgentId ?? null,
              ownerAgentName: attrs?.ownerAgentName ?? null,
              lastGeneratedAt: attrs?.lastGeneratedAt ?? null,
              model: attrs?.model ?? null,
              promptHash: attrs?.promptHash ?? null,
              sourceCount: attrs?.sourceCount ?? 0,
            },
            content: [{ type: 'paragraph' }],
          }),
    };
  },

  addStorage() {
    return {
      markdown: {
        serialize(state: any, node: any) {
          const title = node.attrs.title || 'AI section';
          const status = node.attrs.status || 'draft';
          state.write(`<section data-ai-section data-ai-section-title="${escapeAttr(title)}" data-ai-section-status="${escapeAttr(status)}">`);
          state.ensureNewLine();
          state.renderContent(node);
          state.write('</section>');
          state.closeBlock(node);
        },
        parse: {},
      },
    };
  },
});

function escapeAttr(value: string) {
  return String(value)
    .replace(/&/g, '&amp;')
    .replace(/"/g, '&quot;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;');
}
