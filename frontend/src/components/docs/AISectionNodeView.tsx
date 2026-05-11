import { NodeViewContent, NodeViewWrapper } from '@tiptap/react';

export type AISectionStatus = 'draft' | 'generated' | 'needs_review' | 'approved' | 'error';

export function AISectionNodeView() {
  return (
    <NodeViewWrapper>
      <section className="docs-ai-section my-5 border-l-2 border-border pl-4" data-ai-section="">
        <NodeViewContent />
      </section>
    </NodeViewWrapper>
  );
}
