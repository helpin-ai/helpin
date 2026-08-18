import type { JSONContent } from '@tiptap/react'

function childNodes(node: JSONContent | undefined): JSONContent[] {
  return Array.isArray(node?.content) ? node.content : []
}

function nodeText(node: JSONContent | undefined): string {
  if (!node || typeof node !== 'object') return ''
  if (typeof node.text === 'string') return node.text
  return childNodes(node).map(nodeText).join('')
}

export function collectMermaidSources(content: JSONContent | null | undefined): string[] {
  const sources: string[] = []

  const walk = (node: JSONContent | undefined) => {
    if (!node || typeof node !== 'object') return
    const language = typeof node.attrs?.language === 'string'
      ? node.attrs.language.toLowerCase()
      : ''
    if (node.type === 'codeBlock' && language === 'mermaid') {
      const source = nodeText(node).trim()
      if (source) sources.push(source)
    }
    childNodes(node).forEach(walk)
  }

  walk(content ?? undefined)
  return sources
}
