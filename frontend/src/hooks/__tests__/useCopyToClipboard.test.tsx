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

  afterEach(() => {
    Object.defineProperty(navigator, 'clipboard', {
      configurable: true,
      value: originalClipboard,
    })
    document.execCommand = originalExecCommand
  })

  it('copies via navigator.clipboard.writeText when available', async () => {
    const writeText = vi.fn().mockResolvedValue(undefined)

    Object.defineProperty(navigator, 'clipboard', {
      configurable: true,
      value: { writeText },
    })

    const container = document.createElement('div')
    document.body.appendChild(container)
    const root = createRoot(container)

    act(() => {
      root.render(<CopyHarness text="ST-123" />)
    })

    const button = container.querySelector('button')

    await act(async () => {
      button?.dispatchEvent(new MouseEvent('click', { bubbles: true }))
    })

    expect(writeText).toHaveBeenCalledWith('ST-123')
    expect(container.textContent).toContain('copied')

    act(() => {
      root.unmount()
    })
    container.remove()
  })

  it('uses the textarea fallback when clipboard API is unavailable', async () => {
    const execCommand = vi.fn(() => true)

    Object.defineProperty(navigator, 'clipboard', {
      configurable: true,
      value: undefined,
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

    expect(execCommand).toHaveBeenCalledWith('copy')
    expect(container.textContent).toContain('copied')

    act(() => {
      root.unmount()
    })
    container.remove()
  })

  it('falls back to execCommand when clipboard API rejects', async () => {
    const writeText = vi.fn().mockRejectedValue(new Error('clipboard denied'))
    const execCommand = vi.fn(() => true)

    Object.defineProperty(navigator, 'clipboard', {
      configurable: true,
      value: { writeText },
    })
    document.execCommand = execCommand

    const container = document.createElement('div')
    document.body.appendChild(container)
    const root = createRoot(container)

    act(() => {
      root.render(<CopyHarness text="ST-456" />)
    })

    const button = container.querySelector('button')

    await act(async () => {
      button?.dispatchEvent(new MouseEvent('click', { bubbles: true }))
    })

    expect(writeText).toHaveBeenCalledWith('ST-456')
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
