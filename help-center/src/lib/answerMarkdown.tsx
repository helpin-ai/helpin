import type { ReactNode } from 'react'

// Minimal renderer for the Markdown subset AI answers are allowed to use:
// paragraphs, bullet/numbered lists, **bold**, *italic*, `inline code`.
// Links are rejected server-side, so anything link-shaped renders as text.
// Everything is emitted as React nodes — no HTML injection surface.

const INLINE_PATTERN = /(\*\*[^*]+\*\*|\*[^*\n]+\*|`[^`\n]+`)/g

export function renderAnswerInline(text: string): ReactNode[] {
  const nodes: ReactNode[] = []
  let key = 0
  for (const part of text.split(INLINE_PATTERN)) {
    if (!part) continue
    if (part.startsWith('**') && part.endsWith('**') && part.length > 4) {
      nodes.push(<strong key={key++}>{part.slice(2, -2)}</strong>)
    } else if (part.startsWith('`') && part.endsWith('`') && part.length > 2) {
      nodes.push(
        <code
          key={key++}
          className="rounded px-1 py-0.5 font-mono text-[0.85em]"
          style={{ background: 'var(--hc-border)' }}
        >
          {part.slice(1, -1)}
        </code>,
      )
    } else if (part.startsWith('*') && part.endsWith('*') && part.length > 2) {
      nodes.push(<em key={key++}>{part.slice(1, -1)}</em>)
    } else {
      nodes.push(part)
    }
  }
  return nodes
}

type AnswerBlock =
  | { kind: 'paragraph'; lines: string[] }
  | { kind: 'list'; ordered: boolean; items: string[] }

export function parseAnswerBlocks(text: string): AnswerBlock[] {
  const blocks: AnswerBlock[] = []
  let paragraph: string[] = []
  let list: { ordered: boolean; items: string[] } | null = null

  const flushParagraph = () => {
    if (paragraph.length > 0) {
      blocks.push({ kind: 'paragraph', lines: paragraph })
      paragraph = []
    }
  }
  const flushList = () => {
    if (list) {
      blocks.push({ kind: 'list', ordered: list.ordered, items: list.items })
      list = null
    }
  }

  for (const rawLine of text.replace(/\r\n?/g, '\n').split('\n')) {
    const line = rawLine.trim()
    if (line === '') {
      flushParagraph()
      flushList()
      continue
    }
    const bullet = /^[-*•]\s+(.+)$/.exec(line)
    const numbered = /^\d+[.)]\s+(.+)$/.exec(line)
    if (bullet || numbered) {
      flushParagraph()
      const ordered = Boolean(numbered)
      if (!list || list.ordered !== ordered) {
        flushList()
        list = { ordered, items: [] }
      }
      list.items.push((bullet ?? numbered)?.[1] ?? '')
      continue
    }
    flushList()
    // Strip heading markers the model might emit despite instructions.
    paragraph.push(line.replace(/^#{1,6}\s+/, ''))
  }
  flushParagraph()
  flushList()
  return blocks
}

export function renderAnswerMarkdown(text: string): ReactNode {
  const blocks = parseAnswerBlocks(text)
  return (
    <>
      {blocks.map((block, index) =>
        block.kind === 'list' ? (
          block.ordered ? (
            <ol key={index} className="my-2 list-decimal space-y-1 pl-5">
              {block.items.map((item, itemIndex) => (
                <li key={itemIndex}>{renderAnswerInline(item)}</li>
              ))}
            </ol>
          ) : (
            <ul key={index} className="my-2 list-disc space-y-1 pl-5">
              {block.items.map((item, itemIndex) => (
                <li key={itemIndex}>{renderAnswerInline(item)}</li>
              ))}
            </ul>
          )
        ) : (
          <p key={index} className="my-2 first:mt-0 last:mb-0">
            {renderAnswerInline(block.lines.join(' '))}
          </p>
        ),
      )}
    </>
  )
}
