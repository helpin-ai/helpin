// @vitest-environment jsdom
import { describe, expect, it, afterEach } from 'vitest'
import { Editor } from '@tiptap/core'
import Document from '@tiptap/extension-document'
import Paragraph from '@tiptap/extension-paragraph'
import Text from '@tiptap/extension-text'
import { ProposalAnchorExtension } from '../ProposalAnchorExtension'
import {
  asProposalBlockNode,
  proposalNodeText,
  proposalPreviewKind,
} from '../proposalPreview'

const BLOCK_A = '11111111-1111-4111-8111-111111111111'
const BLOCK_B = '22222222-2222-4222-8222-222222222222'

// Minimal stand-in for BlockIdExtension: declares the blockId attr this
// plugin anchors to, without pulling the whole docs editor into the test.
const BlockIdTestExtension = Paragraph.extend({
  addAttributes() {
    return {
      blockId: {
        default: null,
        renderHTML: (attributes) =>
          attributes.blockId ? { 'data-block-id': attributes.blockId } : {},
      },
    }
  },
})

let editor: Editor | null = null

function createEditor() {
  const element = document.createElement('div')
  document.body.appendChild(element)
  editor = new Editor({
    element,
    extensions: [Document, BlockIdTestExtension, Text, ProposalAnchorExtension],
    content: {
      type: 'doc',
      content: [
        { type: 'paragraph', attrs: { blockId: BLOCK_A }, content: [{ type: 'text', text: 'Alpha' }] },
        { type: 'paragraph', attrs: { blockId: BLOCK_B }, content: [{ type: 'text', text: 'Beta' }] },
      ],
    },
  })
  return editor
}

function makeCard(id: string) {
  const dom = document.createElement('div')
  dom.setAttribute('data-test-card', id)
  return dom
}

afterEach(() => {
  editor?.destroy()
  editor = null
  document.body.innerHTML = ''
})

describe('ProposalAnchorExtension', () => {
  it('places the card immediately after the targeted block and highlights it', () => {
    const ed = createEditor()
    const card = makeCard('card-a')

    ed.commands.setProposalAnchors([{ id: 'proposal-1', blockId: BLOCK_A, dom: card }])

    const target = ed.view.dom.querySelector(`[data-block-id="${BLOCK_A}"]`)
    expect(target).not.toBeNull()
    expect(target!.classList.contains('docs-proposal-target')).toBe(true)
    // The widget lands between the targeted block and the next one.
    expect(target!.nextElementSibling).toBe(card)

    const other = ed.view.dom.querySelector(`[data-block-id="${BLOCK_B}"]`)
    expect(other!.classList.contains('docs-proposal-target')).toBe(false)
  })

  it('anchors each proposal to its own block', () => {
    const ed = createEditor()
    const cardA = makeCard('card-a')
    const cardB = makeCard('card-b')

    ed.commands.setProposalAnchors([
      { id: 'proposal-1', blockId: BLOCK_A, dom: cardA },
      { id: 'proposal-2', blockId: BLOCK_B, dom: cardB },
    ])

    expect(ed.view.dom.querySelector(`[data-block-id="${BLOCK_A}"]`)!.nextElementSibling).toBe(cardA)
    expect(ed.view.dom.querySelector(`[data-block-id="${BLOCK_B}"]`)!.nextElementSibling).toBe(cardB)
  })

  it('ignores anchors whose block is not in the document', () => {
    const ed = createEditor()
    const card = makeCard('card-missing')

    ed.commands.setProposalAnchors([
      { id: 'proposal-1', blockId: '33333333-3333-4333-8333-333333333333', dom: card },
    ])

    expect(ed.view.dom.contains(card)).toBe(false)
    expect(ed.view.dom.querySelectorAll('.docs-proposal-target')).toHaveLength(0)
  })

  it('removes the card when the anchor is cleared', () => {
    const ed = createEditor()
    const card = makeCard('card-a')

    ed.commands.setProposalAnchors([{ id: 'proposal-1', blockId: BLOCK_A, dom: card }])
    expect(ed.view.dom.contains(card)).toBe(true)

    ed.commands.clearProposalAnchors()
    expect(ed.view.dom.contains(card)).toBe(false)
    expect(ed.view.dom.querySelectorAll('.docs-proposal-target')).toHaveLength(0)
  })

  it('keeps the card anchored after the document changes', () => {
    const ed = createEditor()
    const card = makeCard('card-b')

    ed.commands.setProposalAnchors([{ id: 'proposal-1', blockId: BLOCK_B, dom: card }])
    // Type into the first block; the anchored card must follow its own block.
    ed.commands.insertContentAt(3, ' more')

    const target = ed.view.dom.querySelector(`[data-block-id="${BLOCK_B}"]`)
    expect(target!.nextElementSibling).toBe(card)
  })
})

describe('proposalPreviewKind', () => {
  it('treats paragraphs and headings as text', () => {
    expect(proposalPreviewKind({ type: 'paragraph' }, { type: 'paragraph' })).toBe('text')
    expect(proposalPreviewKind({ type: 'heading' }, { type: 'heading' })).toBe('text')
  })

  it('detects mermaid code blocks', () => {
    const node = { type: 'codeBlock', attrs: { language: 'mermaid' } }
    expect(proposalPreviewKind(node, node)).toBe('mermaid')
  })

  it('treats non-mermaid code blocks as text', () => {
    const node = { type: 'codeBlock', attrs: { language: 'go' } }
    expect(proposalPreviewKind(node, node)).toBe('text')
  })

  it('detects images', () => {
    expect(proposalPreviewKind({ type: 'resizableImage' }, { type: 'resizableImage' })).toBe('image')
  })

  it('prefers the proposed node type when the block type changes', () => {
    expect(proposalPreviewKind({ type: 'paragraph' }, { type: 'resizableImage' })).toBe('image')
  })
})

describe('proposalNodeText', () => {
  it('concatenates nested text nodes', () => {
    const node = {
      type: 'codeBlock',
      content: [{ type: 'text', text: 'graph TD;' }, { type: 'text', text: ' A-->B;' }],
    }
    expect(proposalNodeText(node)).toBe('graph TD; A-->B;')
  })

  it('returns an empty string for a missing node', () => {
    expect(proposalNodeText(null)).toBe('')
  })
})

describe('asProposalBlockNode', () => {
  it('rejects non-object values', () => {
    expect(asProposalBlockNode(null)).toBeNull()
    expect(asProposalBlockNode('paragraph')).toBeNull()
    expect(asProposalBlockNode([{ type: 'paragraph' }])).toBeNull()
  })

  it('accepts a node object', () => {
    expect(asProposalBlockNode({ type: 'paragraph' })).toEqual({ type: 'paragraph' })
  })
})
