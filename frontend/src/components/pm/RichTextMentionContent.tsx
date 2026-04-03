import { createElement, useMemo } from 'react'

import { LoadingImage } from '@/components/ui/loading-image'
import { MentionText } from '@/components/pm/MentionText'
import type { AssignableMember, WorkspaceTeam } from '@/lib/types'

interface RichTextMentionContentProps {
  html: string
  members?: AssignableMember[]
  teams?: Pick<WorkspaceTeam, 'id' | 'name' | 'handle'>[]
  className?: string
}

function mapAttributes(element: HTMLElement): Record<string, string> {
  const props: Record<string, string> = {}
  for (const attribute of Array.from(element.attributes)) {
    if (attribute.name === 'class') {
      props.className = attribute.value
      continue
    }
    props[attribute.name] = attribute.value
  }
  return props
}

function renderNode(
  node: ChildNode,
  key: string,
  members: AssignableMember[],
  teams: Pick<WorkspaceTeam, 'id' | 'name' | 'handle'>[],
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
  const props: Record<string, string> = { key, ...mapAttributes(element) }

  if (tag === 'img') {
    return createElement(LoadingImage, props)
  }

  // Render checkboxes as read-only React inputs
  if (tag === 'input') {
    const inputEl = element as HTMLInputElement
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
    .map((child, index) => renderNode(child, `${key}-${index}`, members, teams))
    .filter((child) => child !== null)

  return createElement(tag, props, children.length > 0 ? children : undefined)
}

export function RichTextMentionContent({
  html,
  members = [],
  teams = [],
  className,
}: RichTextMentionContentProps) {
  const content = useMemo(() => {
    if (!html || typeof DOMParser === 'undefined') return null

    const parsed = new DOMParser().parseFromString(html, 'text/html')
    return Array.from(parsed.body.childNodes)
      .map((node, index) => renderNode(node, `node-${index}`, members, teams))
      .filter((node) => node !== null)
  }, [html, members, teams])

  if (!content) {
    return <div className={className} dangerouslySetInnerHTML={{ __html: html }} />
  }

  return <div className={`tiptap ${className ?? ''}`}>{content}</div>
}
