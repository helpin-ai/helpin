// @vitest-environment jsdom
import { act } from 'react'
import { createRoot } from 'react-dom/client'
import { afterEach, describe, expect, it, vi } from 'vitest'

import { useCopyToClipboard } from '../useCopyToClipboard'

(globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true

function CopyHarness({ text }: { text: string }) {
  const { copied, copy } = useCopyToClipboard(10_000)

  return (
    <div>
      <button type="button" onClick={() => { void copy(text) }}>
        Copy
      </button>
      <span>{copied ? 'copied' : 'idle'}</span>
    </div>
  )
}

describe('useCopyToClipboard', () => {
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

  it('uses the textarea fallback immediately when the page is not in a secure context', async () => {
    const writeText = vi.fn().mockResolvedValue(undefined)
    const execCommand = vi.fn(() => true)

    Object.defineProperty(navigator, 'clipboard', {
      configurable: true,
      value: { writeText },
    })
    Object.defineProperty(window, 'isSecureContext', {
      configurable: true,
      value: false,
    })
    document.execCommand = execCommand

    const container = document.createElement('div')
    document.body.appendChild(container)
    const root = createRoot(container)

    act(() => {
      root.render(<CopyHarness text="ST-321" />)
    })

    const button = container.querySelector('button')

    await act(async () => {
      button?.dispatchEvent(new MouseEvent('click', { bubbles: true }))
    })

    expect(writeText).not.toHaveBeenCalled()
    expect(execCommand).toHaveBeenCalledWith('copy')
    expect(container.textContent).toContain('copied')

    act(() => {
      root.unmount()
    })
    container.remove()
  })

  it('does not report success when both clipboard strategies fail', async () => {
    const writeText = vi.fn().mockRejectedValue(new Error('clipboard denied'))
    const execCommand = vi.fn(() => false)

    Object.defineProperty(navigator, 'clipboard', {
      configurable: true,
      value: { writeText },
    })
    Object.defineProperty(window, 'isSecureContext', {
      configurable: true,
      value: true,
    })
    document.execCommand = execCommand

    const container = document.createElement('div')
    document.body.appendChild(container)
    const root = createRoot(container)

    act(() => {
      root.render(<CopyHarness text="https://example.com/task/ST-321" />)
    })

    const button = container.querySelector('button')

    await act(async () => {
      button?.dispatchEvent(new MouseEvent('click', { bubbles: true }))
    })

    expect(writeText).toHaveBeenCalledWith('https://example.com/task/ST-321')
    expect(execCommand).toHaveBeenCalledWith('copy')
    expect(container.textContent).toContain('idle')

    act(() => {
      root.unmount()
    })
    container.remove()
  })
})
