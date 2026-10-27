// htmlToMarkdown converts a rendered help article to Markdown for "Copy page".
// It covers what articles render: headings, paragraphs, lists, emphasis,
// code, links, images, quotes, tables and callout blocks. Anything else keeps
// its text.

export function htmlToMarkdown(root: ParentNode, baseUrl?: string): string {
  return blocks(root, baseUrl).replace(/\n{3,}/g, '\n\n').trim() + '\n'
}

function blocks(parent: ParentNode, baseUrl?: string): string {
  let out = ''
  for (const node of Array.from(parent.childNodes)) out += block(node, baseUrl)
  return out
}

function block(node: Node, baseUrl?: string): string {
  if (node.nodeType === Node.TEXT_NODE) return (node.textContent ?? '').trim() ? inlineText(node.textContent ?? '') : ''
  if (!(node instanceof Element)) return ''
  const tag = node.tagName.toLowerCase()
  const heading = /^h([1-6])$/.exec(tag)
  if (heading) return `\n\n${'#'.repeat(Number(heading[1]))} ${inline(node, baseUrl).trim()}\n\n`
  switch (tag) {
    case 'p':
      return `\n\n${inline(node, baseUrl).trim()}\n\n`
    case 'ul':
    case 'ol':
      return `\n\n${list(node, tag === 'ol', 0, baseUrl)}\n\n`
    case 'pre': {
      const code = node.querySelector('code')
      const language = /language-([\w-]+)/.exec(code?.className ?? '')?.[1] ?? ''
      return `\n\n\`\`\`${language}\n${(code ?? node).textContent?.replace(/\n$/, '') ?? ''}\n\`\`\`\n\n`
    }
    case 'blockquote':
      return `\n\n${blocks(node, baseUrl).trim().split('\n').map((line) => `> ${line}`.trimEnd()).join('\n')}\n\n`
    case 'hr':
      return '\n\n---\n\n'
    case 'table':
      return `\n\n${table(node, baseUrl)}\n\n`
    case 'img':
      return `\n\n${image(node, baseUrl)}\n\n`
    case 'script':
    case 'style':
    case 'button':
      return ''
    case 'div':
    case 'section':
    case 'article':
    case 'aside':
    case 'figure':
    case 'details':
      return `\n\n${blocks(node, baseUrl)}\n\n`
    default:
      // An inline element among blocks keeps its formatting.
      return inline({ childNodes: [node] } as unknown as Node, baseUrl)
  }
}

function inline(node: Node, baseUrl?: string): string {
  let out = ''
  for (const child of Array.from(node.childNodes)) {
    if (child.nodeType === Node.TEXT_NODE) {
      out += inlineText(child.textContent ?? '')
      continue
    }
    if (!(child instanceof Element)) continue
    const tag = child.tagName.toLowerCase()
    const text = () => inline(child, baseUrl)
    switch (tag) {
      case 'strong':
      case 'b':
        out += wrap(text(), '**')
        break
      case 'em':
      case 'i':
        out += wrap(text(), '*')
        break
      case 's':
      case 'del':
        out += wrap(text(), '~~')
        break
      case 'code':
        out += `\`${child.textContent ?? ''}\``
        break
      case 'a': {
        const raw = child.getAttribute('href') ?? ''
        const label = text().trim()
        // Heading anchors are page furniture, not content.
        if (raw.startsWith('#')) break
        const href = absolute(raw, baseUrl)
        out += href ? `[${label || href}](${href})` : label
        break
      }
      case 'img':
        out += image(child, baseUrl)
        break
      case 'br':
        out += '  \n'
        break
      case 'button':
        break
      default:
        out += text()
    }
  }
  return out
}

function list(node: Element, ordered: boolean, depth: number, baseUrl?: string): string {
  const indent = '  '.repeat(depth)
  return Array.from(node.children)
    .filter((child) => child.tagName.toLowerCase() === 'li')
    .map((item, index) => {
      const marker = ordered ? `${index + 1}.` : '-'
      const nested = Array.from(item.children).filter((child) => /^(ul|ol)$/i.test(child.tagName))
      const clone = item.cloneNode(true) as Element
      clone.querySelectorAll('ul, ol').forEach((child) => child.remove())
      const checkbox = clone.querySelector('input[type="checkbox"]') as HTMLInputElement | null
      const task = checkbox ? (checkbox.checked ? '[x] ' : '[ ] ') : ''
      const body = blocks(clone, baseUrl).replace(/\s*\n\s*/g, ' ').trim()
      const children = nested.map((child) => '\n' + list(child, child.tagName.toLowerCase() === 'ol', depth + 1, baseUrl)).join('')
      return `${indent}${marker} ${task}${body}${children}`
    })
    .join('\n')
}

function table(node: Element, baseUrl?: string): string {
  const rows = Array.from(node.querySelectorAll('tr')).map((row) =>
    Array.from(row.children).map((cell) => inline(cell, baseUrl).replace(/\|/g, '\\|').replace(/\s+/g, ' ').trim()))
  if (rows.length === 0) return ''
  const width = Math.max(...rows.map((row) => row.length))
  const line = (cells: string[]) => `| ${Array.from({ length: width }, (_, i) => cells[i] ?? '').join(' | ')} |`
  return [line(rows[0]!), `| ${Array.from({ length: width }, () => '---').join(' | ')} |`, ...rows.slice(1).map(line)].join('\n')
}

function image(node: Element, baseUrl?: string): string {
  const src = absolute(node.getAttribute('src') ?? '', baseUrl)
  return src ? `![${node.getAttribute('alt') ?? ''}](${src})` : ''
}

function wrap(text: string, mark: string): string {
  const trimmed = text.trim()
  if (!trimmed) return text
  const lead = text.slice(0, text.length - text.trimStart().length)
  const trail = text.slice(text.trimEnd().length)
  return `${lead}${mark}${trimmed}${mark}${trail}`
}

function inlineText(text: string): string {
  return text.replace(/\s+/g, ' ')
}

function absolute(url: string, baseUrl?: string): string {
  if (!url || !baseUrl) return url
  try {
    return new URL(url, baseUrl).toString()
  } catch {
    return url
  }
}
