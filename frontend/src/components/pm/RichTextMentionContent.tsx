import { createElement, useCallback, useMemo, useRef } from 'react'

import { LoadingImage } from '@/components/ui/loading-image'
import { NwdiagBlock } from '@/components/editor/NwdiagBlock'
import { MermaidBlock } from '@/components/editor/MermaidBlock'
import { MentionText } from '@/components/pm/MentionText'
import { normalizeInlineAttachmentImageSrcs } from '@/components/pm/editorImageAttachments'
import type { AssignableMember, WorkspaceTeam } from '@/lib/types'
import { cn } from '@/lib/utils'

interface RichTextMentionContentProps {
  html: string
  members?: AssignableMember[]
  teams?: Pick<WorkspaceTeam, 'id' | 'name' | 'handle'>[]
  className?: string
  variant?: 'default' | 'pm'
  /** When provided, checkboxes become interactive and changes are reported back */
  onHtmlChange?: (html: string) => void
}

function parseStyleString(style: string): Record<string, string> {
  const result: Record<string, string> = {}
  for (const part of style.split(';')) {
    const colon = part.indexOf(':')
    if (colon < 0) continue
    const key = part.slice(0, colon).trim()
    const value = part.slice(colon + 1).trim()
    if (!key || !value) continue
    // Convert kebab-case to camelCase
    const camel = key.replace(/-([a-z])/g, (_, c: string) => c.toUpperCase())
    result[camel] = value
  }
  return result
}

function mapAttributes(element: HTMLElement): Record<string, unknown> {
  const props: Record<string, unknown> = {}
  for (const attribute of Array.from(element.attributes)) {
    if (attribute.name === 'class') {
      props.className = attribute.value
      continue
    }
    if (attribute.name === 'style') {
      props.style = parseStyleString(attribute.value)
      continue
    }
    props[attribute.name] = attribute.value
  }
  return props
}

const INLINE_IMAGE_ALIGNMENT_CLASS: Record<string, string> = {
  left: 'justify-start',
  center: 'justify-center',
  right: 'justify-end',
}

function renderNode(
  node: ChildNode,
  key: string,
  members: AssignableMember[],
  teams: Pick<WorkspaceTeam, 'id' | 'name' | 'handle'>[],
  onCheckToggle?: (index: number) => void,
  checkCounter?: { current: number },
): React.ReactNode {
  if (node.nodeType === Node.TEXT_NODE) {
    const text = node.textContent ?? ''
    if (text === '') return null
    return <MentionText key={key} text={text} members={members} teams={teams} />
  }

  if (node.nodeType !== Node.ELEMENT_NODE) {
    return null
  }

  const element = node as HTMLElement
  const tag = element.tagName.toLowerCase()
  const props: Record<string, unknown> = { key, ...mapAttributes(element) }

  if (tag === 'pre') {
    const code = Array.from(element.children).find((child) => child.tagName.toLowerCase() === 'code')
    const isMermaid = code && Array.from(code.classList)
      .some((className) => className.toLowerCase() === 'language-mermaid')
    const isNwdiag = code && Array.from(code.classList)
      .some((className) => className.toLowerCase() === 'language-nwdiag')
    if (code && (isMermaid || isNwdiag)) {
      const source = code.textContent ?? ''
      return (
        <div key={key} className="my-3 overflow-hidden rounded-md border border-border bg-muted/20">
          {isNwdiag ? <NwdiagBlock source={source} /> : <MermaidBlock source={source} />}
        </div>
      )
    }
  }

  if (tag === 'img') {
    const alignment = element.getAttribute('data-alignment') || 'left'
    return (
      <span
        key={key}
        data-inline-image-align={alignment}
        className={cn('my-4 flex w-full', INLINE_IMAGE_ALIGNMENT_CLASS[alignment] ?? 'justify-center')}
      >
        {createElement(LoadingImage, props)}
      </span>
    )
  }

  // Render checkboxes — interactive when onCheckToggle is provided
  if (tag === 'input') {
    const inputEl = element as HTMLInputElement
    if (inputEl.type === 'checkbox' && checkCounter && onCheckToggle) {
      const idx = checkCounter.current++
      const checked = inputEl.checked || inputEl.getAttribute('checked') !== null
      return (
        <input
          key={key}
          type="checkbox"
          checked={checked}
          onChange={() => onCheckToggle(idx)}
          className="cursor-pointer accent-[var(--primary)]"
        />
      )
    }
    const inputProps: Record<string, unknown> = { key, type: inputEl.type, readOnly: true }
    if (inputEl.type === 'checkbox') {
      inputProps.defaultChecked = inputEl.checked
    }
    return createElement('input', inputProps)
  }

  // Enforce links open in new tab with consistent styling
  if (tag === 'a') {
    props.target = '_blank'
    props.rel = 'noopener noreferrer'
    props.className = 'text-blue-600 dark:text-blue-400 underline cursor-pointer'
  }

  const children = Array.from(element.childNodes)
    .map((child, index) => renderNode(child, `${key}-${index}`, members, teams, onCheckToggle, checkCounter))
    .filter((child) => child !== null)

  return createElement(tag, props, children.length > 0 ? children : undefined)
}

/** Toggle the nth checkbox in the HTML string (both <input> and data-checked attributes). */
function toggleCheckboxInHtml(html: string, targetIndex: number): string {
  let index = 0
  // Toggle data-checked on <li> task items
  let result = html.replace(/data-checked="(true|false)"/g, (match, value) => {
    if (index++ === targetIndex) {
      return `data-checked="${value === 'true' ? 'false' : 'true'}"`
    }
    return match
  })
  // If data-checked didn't match (plain checkbox), toggle checked attribute on <input>
  if (index === 0) {
    index = 0
    result = html.replace(/<input[^>]*type=["']checkbox["'][^>]*>/gi, (match) => {
      if (index++ === targetIndex) {
        if (match.includes('checked')) {
          return match.replace(/\s*checked(?:="[^"]*")?/, '')
        }
        return match.replace(/>$/, ' checked="checked">')
      }
      return match
    })
  }
  return result
}

export function RichTextMentionContent({
  html,
  members = [],
  teams = [],
  className,
  variant = 'default',
  onHtmlChange,
}: RichTextMentionContentProps) {
  const htmlRef = useRef(html)
  htmlRef.current = html

  const handleCheckToggle = useCallback((index: number) => {
    if (!onHtmlChange) return
    const updated = toggleCheckboxInHtml(htmlRef.current, index)
    onHtmlChange(updated)
  }, [onHtmlChange])

  const content = useMemo(() => {
    if (!html || typeof DOMParser === 'undefined') return null

    const normalizedHtml = normalizeInlineAttachmentImageSrcs(html)
    const parsed = new DOMParser().parseFromString(normalizedHtml, 'text/html')
    const counter = { current: 0 }
    return Array.from(parsed.body.childNodes)
      .map((node, index) => renderNode(node, `node-${index}`, members, teams, onHtmlChange ? handleCheckToggle : undefined, onHtmlChange ? counter : undefined))
      .filter((node) => node !== null)
  }, [html, members, teams, onHtmlChange, handleCheckToggle])

  if (!content) {
    return (
      <div
        className={cn(variant === 'pm' && 'pm-rich-text prose prose-sm dark:prose-invert max-w-none', className)}
        dangerouslySetInnerHTML={{ __html: html }}
      />
    )
  }

  return (
    <div className={cn(
      'tiptap',
      variant === 'pm' && 'pm-rich-text prose prose-sm dark:prose-invert max-w-none',
      className,
    )}>
      {content}
    </div>
  )
}
