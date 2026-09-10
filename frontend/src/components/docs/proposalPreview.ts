/** Shape helpers for rendering a change proposal against its current block. */

export type ProposalPreviewKind = 'text' | 'nwdiag' | 'mermaid' | 'image'

export interface ProposalBlockNode {
  type?: string
  attrs?: Record<string, unknown>
  content?: ProposalBlockNode[]
  text?: string
}

export function asProposalBlockNode(value: unknown): ProposalBlockNode | null {
  if (!value || typeof value !== 'object' || Array.isArray(value)) return null
  return value as ProposalBlockNode
}

export function proposalNodeText(node: ProposalBlockNode | null): string {
  if (!node) return ''
  if (typeof node.text === 'string') return node.text
  return (node.content ?? []).map(proposalNodeText).join('')
}

/**
 * Diagrams and images are compared side by side; everything else gets a
 * textual diff. The proposed node wins when the two disagree, since that is
 * what the block becomes if accepted.
 */
export function proposalPreviewKind(
  current: ProposalBlockNode | null,
  proposed: ProposalBlockNode | null,
): ProposalPreviewKind {
  const type = proposed?.type ?? current?.type
  if (type === 'resizableImage' || type === 'image') return 'image'
  if (type === 'codeBlock') {
    const language = String(proposed?.attrs?.language ?? current?.attrs?.language ?? '').toLowerCase()
    if (language === 'nwdiag') return 'nwdiag'
    if (language === 'mermaid') return 'mermaid'
  }
  return 'text'
}
