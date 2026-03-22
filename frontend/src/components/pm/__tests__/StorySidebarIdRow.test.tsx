// @vitest-environment jsdom
import { act } from 'react'
import { createRoot } from 'react-dom/client'
import { afterEach, describe, expect, it, vi } from 'vitest'

import { StorySidebarIdRow } from '../StorySidebarIdRow'

(globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true

describe('StorySidebarIdRow', () => {
  const originalClipboard = navigator.clipboard
  const originalExecCommand = document.execCommand
  const originalSecureContext = window.isSecureContext

  afterEach(() => {
    Object.defineProperty(navigator, 'clipboard', {
      configurable: true,
      value: originalClipboard,
    })
    document.execCommand = originalExecCommand
    Object.defineProperty(window, 'isSecureContext', {
      configurable: true,
      value: originalSecureContext,
    })
  })

  it('renders the Story ID label and copies the display ID', async () => {
    const writeText = vi.fn().mockResolvedValue(undefined)
    const execCommand = vi.fn(() => true)
    Object.defineProperty(navigator, 'clipboard', {
      configurable: true,
      value: { writeText },
    })
    document.execCommand = execCommand
    Object.defineProperty(window, 'isSecureContext', {
      configurable: true,
      value: true,
    })

    const container = document.createElement('div')
    document.body.appendChild(container)
    const root = createRoot(container)

    act(() => {
      root.render(<StorySidebarIdRow displayId="ST-123" />)
    })

    expect(container.textContent).toContain('Story ID:')
    expect(container.textContent).toContain('ST-123')

    const button = container.querySelector('button')
    expect(button).toBeTruthy()

    await act(async () => {
      button?.dispatchEvent(new MouseEvent('click', { bubbles: true }))
    })

    expect(execCommand).toHaveBeenCalledWith('copy')
    expect(container.querySelector('svg.text-green-500')).toBeTruthy()

    act(() => {
      root.unmount()
    })
    container.remove()
  })
})
