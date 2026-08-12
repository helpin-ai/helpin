import { describe, expect, it } from 'vitest'
import { collectMermaidSources } from '../mermaidContent'

describe('collectMermaidSources', () => {
  it('collects Mermaid code blocks in document order', () => {
    expect(collectMermaidSources({
      type: 'doc',
      content: [
        {
          type: 'codeBlock',
          attrs: { language: 'Mermaid' },
          content: [{ type: 'text', text: 'graph TD\nA-->B' }],
        },
        {
          type: 'paragraph',
          content: [{ type: 'text', text: 'Not a diagram' }],
        },
        {
          type: 'codeBlock',
          attrs: { language: 'mermaid' },
          content: [{ type: 'text', text: 'sequenceDiagram\nA->>B: Hello' }],
        },
      ],
    })).toEqual([
      'graph TD\nA-->B',
      'sequenceDiagram\nA->>B: Hello',
    ])
  })

  it('ignores non-Mermaid and empty code blocks', () => {
    expect(collectMermaidSources({
      type: 'doc',
      content: [
        { type: 'codeBlock', attrs: { language: 'typescript' }, content: [{ type: 'text', text: 'const a = 1' }] },
        { type: 'codeBlock', attrs: { language: 'mermaid' }, content: [] },
      ],
    })).toEqual([])
  })
})
