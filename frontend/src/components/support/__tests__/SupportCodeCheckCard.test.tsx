// @vitest-environment jsdom
import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { afterEach, beforeEach, describe, expect, it } from 'vitest'
import type { SupportMessage } from '@/lib/pmTypes'
import { SupportCodeCheckCard } from '../SupportCodeCheckCard'
import type { SupportCodeCheck } from '../supportAIActivityModel'

Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true })

const message: SupportMessage = {
  id: 'card', workspace_id: 'ws', conversation_id: 'conv', sender_type: 'agent', message_type: 'note', is_internal: true,
  content: '', metadata: '{}', created_at: '2026-10-26T10:42:00Z', updated_at: '2026-10-26T10:42:00Z',
}

describe('SupportCodeCheckCard', () => {
  let container: HTMLDivElement
  let root: Root

  beforeEach(() => {
    container = document.createElement('div')
    document.body.appendChild(container)
    root = createRoot(container)
  })
  afterEach(() => {
    act(() => root.unmount())
    container.remove()
  })

  const render = (check: SupportCodeCheck) => act(() => root.render(
    <QueryClientProvider client={new QueryClient()}>
      <SupportCodeCheckCard message={message} check={check} renderMarkdown={(markdown) => markdown} />
    </QueryClientProvider>,
  ))

  it('says the AI is checking the code before replying', () => {
    render({ status: 'running', phase: 'before_reply', focus: 'Does the CSV export include the Notes column by default?' })
    expect(container.textContent).toContain('Checking the code before replying: Does the CSV export include the Notes column by default?')
    render({ status: 'running', phase: 'before_reply' })
    expect(container.textContent).toContain('Checking the code before replying.')
  })

  it('keeps the handoff wording for checks after a handoff', () => {
    render({ status: 'running', focus: 'How approving a post sets its time' })
    expect(container.textContent).toContain('Checking the code: How approving a post sets its time')
  })
})
