import { describe, expect, it } from 'vitest'
import { parseAnswerBlocks } from '@/lib/answerMarkdown'

describe('parseAnswerBlocks', () => {
  it('splits paragraphs on blank lines', () => {
    const blocks = parseAnswerBlocks('First paragraph.\n\nSecond paragraph.')
    expect(blocks).toEqual([
      { kind: 'paragraph', lines: ['First paragraph.'] },
      { kind: 'paragraph', lines: ['Second paragraph.'] },
    ])
  })

  it('parses bullet and numbered lists', () => {
    const blocks = parseAnswerBlocks(
      'Steps:\n1. Open settings\n2. Click save\n\nNotes:\n- fast\n- safe',
    )
    expect(blocks).toEqual([
      { kind: 'paragraph', lines: ['Steps:'] },
      { kind: 'list', ordered: true, items: ['Open settings', 'Click save'] },
      { kind: 'paragraph', lines: ['Notes:'] },
      { kind: 'list', ordered: false, items: ['fast', 'safe'] },
    ])
  })

  it('strips heading markers the model should not emit', () => {
    expect(parseAnswerBlocks('## Pricing\nCosts $84.')).toEqual([
      { kind: 'paragraph', lines: ['Pricing', 'Costs $84.'] },
    ])
  })

  it('keeps windows newlines and mixed markers stable', () => {
    expect(parseAnswerBlocks('- one\r\n- two')).toEqual([
      { kind: 'list', ordered: false, items: ['one', 'two'] },
    ])
  })
})
