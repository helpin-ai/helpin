// @vitest-environment jsdom
import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const mutate = vi.fn()
let settings: Record<string, unknown> | undefined = {}

vi.mock('@/hooks/queries/useSupport', () => ({
  useChatSettings: () => ({ data: settings ? { settings } : undefined, isLoading: false }),
  useUpdateChatSettings: () => ({ mutate, isPending: false }),
}))

import { SupportAIPrioritySetting } from '../SupportAIPrioritySetting'

Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true })

describe('SupportAIPrioritySetting', () => {
  let container: HTMLDivElement
  let root: Root

  beforeEach(() => {
    mutate.mockReset()
    container = document.createElement('div')
    document.body.appendChild(container)
    root = createRoot(container)
  })

  afterEach(() => {
    act(() => root.unmount())
    container.remove()
  })

  function render(editable = true) {
    act(() => root.render(<SupportAIPrioritySetting workspaceId="ws" editable={editable} />))
    return container.querySelector<HTMLButtonElement>('button[role="switch"]')!
  }

  it('is on when the workspace has not saved the setting', () => {
    settings = {}
    expect(render().getAttribute('aria-checked')).toBe('true')
  })

  it('saves the change when turned off', () => {
    settings = { ai_priority_enabled: true }
    const toggle = render()
    act(() => toggle.click())
    expect(mutate).toHaveBeenCalledWith({ ai_priority_enabled: false })
  })

  it('shows a saved off setting', () => {
    settings = { ai_priority_enabled: false }
    expect(render().getAttribute('aria-checked')).toBe('false')
  })

  it('is read-only without support admin access', () => {
    settings = { ai_priority_enabled: true }
    expect(render(false).disabled).toBe(true)
  })
})
