// @vitest-environment jsdom
import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, beforeEach, describe, expect, it } from 'vitest'
import { useSupportPresenceStore } from '@/stores/supportPresenceStore'
import { AIThinkingBubble } from '../AIThinkingBubble'

Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true })

describe('AIThinkingBubble', () => {
  let container: HTMLDivElement
  let root: Root

  beforeEach(() => {
    container = document.createElement('div')
    document.body.appendChild(container)
    root = createRoot(container)
    useSupportPresenceStore.setState({ aiThinking: {} })
  })
  afterEach(() => {
    act(() => root.unmount())
    container.remove()
  })

  it('shows while the AI works on this conversation', () => {
    act(() => root.render(<AIThinkingBubble conversationId="conv-1" />))
    expect(container.textContent).toBe('')
    act(() => useSupportPresenceStore.getState().setAIThinking('conv-1', true))
    expect(container.textContent).toContain('Helpin AI is thinking')
    act(() => useSupportPresenceStore.getState().setAIThinking('conv-1', false))
    expect(container.textContent).toBe('')
  })

  it('stays out of other conversations and behind a running code check', () => {
    useSupportPresenceStore.getState().setAIThinking('conv-2', true)
    act(() => root.render(<AIThinkingBubble conversationId="conv-1" />))
    expect(container.textContent).toBe('')
    useSupportPresenceStore.getState().setAIThinking('conv-1', true)
    act(() => root.render(<AIThinkingBubble conversationId="conv-1" hidden />))
    expect(container.textContent).toBe('')
  })
})
