import type { JSONContent } from '@tiptap/core'

type RawNode = Record<string, unknown>

const inlineNodeTypes = new Set(['text', 'hardBreak', 'entityMention'])
const listNodeTypes = new Set(['bulletList', 'orderedList'])
const requiredBlockContentTypes = new Set([
  'aiSection',
  'blockquote',
  'callout',
  'tableCell',
  'tableHeader',
  'toggleSection',
])

function isObject(value: unknown): value is RawNode {
  return value !== null && typeof value === 'object' && !Array.isArray(value)
}

function paragraph(content: JSONContent[] = []): JSONContent {
  return content.length > 0 ? { type: 'paragraph', content } : { type: 'paragraph' }
}

function normalizeMarks(value: unknown): JSONContent['marks'] {
  if (!Array.isArray(value)) return undefined

  const marks: NonNullable<JSONContent['marks']> = []
  const seen = new Set<string>()
  for (const candidate of value) {
    if (!isObject(candidate) || typeof candidate.type !== 'string' || seen.has(candidate.type)) continue
    seen.add(candidate.type)
    marks.push(candidate as NonNullable<JSONContent['marks']>[number])
  }

  // TipTap's code mark excludes every other mark. Older importers sometimes
  // emitted code together with bold/link, which makes the whole document fail
  // the strict content check.
  const code = marks.find((mark) => mark.type === 'code')
  if (code) return [code]
  return marks.length > 0 ? marks : undefined
}

function rawChildren(node: RawNode): unknown[] {
  if (Array.isArray(node.content)) return node.content
  if (typeof node.content === 'string') {
    return node.content.length > 0 ? [{ type: 'text', text: node.content }] : []
  }
  if (isObject(node.content)) return [node.content]
  return []
}

function withContent(node: RawNode, content: JSONContent[]): JSONContent {
  const repaired = { ...node } as JSONContent
  if (content.length > 0) repaired.content = content
  else delete repaired.content
  return repaired
}

function headingAsParagraph(node: JSONContent): JSONContent {
  const attrs = node.attrs ? { ...node.attrs } : undefined
  if (attrs) delete attrs.level
  return {
    ...node,
    type: 'paragraph',
    ...(attrs && Object.keys(attrs).length > 0 ? { attrs } : { attrs: undefined }),
  }
}

function normalizeListItem(node: JSONContent, itemType: 'listItem' | 'taskItem'): JSONContent {
  const content = [...(node.content ?? [])]
  if (content.length === 0) content.push(paragraph())
  else if (content[0]?.type === 'heading') content[0] = headingAsParagraph(content[0])
  else if (content[0]?.type !== 'paragraph') content.unshift(paragraph())

  return {
    ...node,
    type: itemType,
    ...(itemType === 'taskItem'
      ? { attrs: { checked: false, ...(node.attrs ?? {}) } }
      : {}),
    content,
  }
}

function wrapListChild(node: JSONContent, itemType: 'listItem' | 'taskItem'): JSONContent {
  if (node.type === itemType) return normalizeListItem(node, itemType)
  if (node.type === 'paragraph') return normalizeListItem({ type: itemType, content: [node] }, itemType)
  if (node.type === 'heading') {
    return normalizeListItem({ type: itemType, content: [headingAsParagraph(node)] }, itemType)
  }
  return normalizeListItem({ type: itemType, content: [paragraph(), node] }, itemType)
}

function splitTextBlockAroundBlocks(node: RawNode, children: JSONContent[]): JSONContent[] {
  const output: JSONContent[] = []
  let inlineRun: JSONContent[] = []

  const flushInlineRun = () => {
    if (inlineRun.length === 0) return
    output.push(withContent(node, inlineRun))
    inlineRun = []
  }

  for (const child of children) {
    if (child.type && inlineNodeTypes.has(child.type)) {
      inlineRun.push(child)
      continue
    }
    flushInlineRun()
    output.push(child)
  }
  flushInlineRun()

  // Empty paragraphs/headings are valid and should stay present. A text block
  // containing only block nodes, however, is replaced by those preserved nodes.
  return output.length > 0 ? output : [withContent(node, [])]
}

function repairNode(value: unknown): JSONContent[] {
  if (!isObject(value)) return []

  const type = typeof value.type === 'string' ? value.type : ''
  if (!type) return rawChildren(value).flatMap(repairNode)

  if (type === 'text') {
    if (typeof value.text !== 'string' || value.text.length === 0) return []
    const repaired = { ...value, type, text: value.text } as JSONContent
    const marks = normalizeMarks(value.marks)
    if (marks) repaired.marks = marks
    else delete repaired.marks
    delete repaired.content
    return [repaired]
  }

  const children = rawChildren(value).flatMap(repairNode)

  if (type === 'paragraph' || type === 'heading') {
    return splitTextBlockAroundBlocks(value, children)
  }

  if (listNodeTypes.has(type)) {
    const content = children.map((child) => wrapListChild(child, 'listItem'))
    return content.length > 0 ? [withContent(value, content)] : []
  }

  if (type === 'taskList') {
    const content = children.map((child) => wrapListChild(child, 'taskItem'))
    return content.length > 0 ? [withContent(value, content)] : []
  }

  if (type === 'listItem' || type === 'taskItem') {
    return [normalizeListItem(withContent(value, children), type)]
  }

  if (requiredBlockContentTypes.has(type) && children.length === 0) {
    return [withContent(value, [paragraph()])]
  }

  return [withContent(value, children)]
}

/**
 * Losslessly repairs the legacy structural mistakes we have observed in stored
 * TipTap JSON. The caller must still validate the result against the live
 * editor schema before saving it, so a future/unknown shape is never guessed at.
 */
export function repairTiptapDocument(value: unknown): JSONContent {
  const source = isObject(value) && value.type === 'doc'
    ? value
    : { type: 'doc', content: Array.isArray(value) ? value : [value] }
  const content = rawChildren(source).flatMap(repairNode)
  return withContent({ ...source, type: 'doc' }, content.length > 0 ? content : [paragraph()])
}
