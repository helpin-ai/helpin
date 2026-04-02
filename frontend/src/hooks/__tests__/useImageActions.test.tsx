// @vitest-environment jsdom
import { act } from 'react'
import { createRoot } from 'react-dom/client'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { toast } from 'sonner'

import { useImageActions } from '../useImageActions'

vi.mock('sonner', () => ({
  toast: {
    success: vi.fn(),
    error: vi.fn(),
  },
}))

Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true })

function CopyImageButton({ src }: { src: string }) {
  const { copyImage } = useImageActions()

  return (
    <button
      type="button"
      onClick={() => {
        void copyImage(src)
      }}
    >
      Copy
    </button>
  )
}

const originalClipboard = navigator.clipboard
const originalImage = globalThis.Image
const originalClipboardItem = globalThis.ClipboardItem
const originalFetch = globalThis.fetch

describe('useImageActions', () => {
  afterEach(() => {
    vi.mocked(toast.success).mockReset()
    vi.mocked(toast.error).mockReset()
    vi.restoreAllMocks()
    vi.unstubAllGlobals()
    Object.defineProperty(navigator, 'clipboard', {
      configurable: true,
      value: originalClipboard,
    })
    vi.stubGlobal('Image', originalImage)
    if (originalClipboardItem) vi.stubGlobal('ClipboardItem', originalClipboardItem)
    if (originalFetch) {
      vi.stubGlobal('fetch', originalFetch)
    }
  })

  it('copies the fetched image blob to the clipboard before falling back to the url', async () => {
    const write = vi.fn().mockResolvedValue(undefined)
    const writeText = vi.fn().mockResolvedValue(undefined)
    Object.defineProperty(navigator, 'clipboard', {
      configurable: true,
      value: { write, writeText },
    })

    const imageBlob = new Blob(['image-bytes'], { type: 'image/png' })
    vi.stubGlobal(
      'fetch',
      vi.fn().mockResolvedValue({
        ok: true,
        blob: async () => imageBlob,
      }),
    )

    class MockImage {
      onload: null | (() => void) = null
      onerror: null | (() => void) = null
      naturalWidth = 100
      naturalHeight = 50

      set crossOrigin(_value: string) {}

      set src(_value: string) {
        queueMicrotask(() => {
          this.onerror?.()
        })
      }
    }

    const clipboardItem = vi.fn((items: Record<string, Blob>) => items)
    vi.stubGlobal('Image', MockImage as typeof Image)
    vi.stubGlobal('ClipboardItem', clipboardItem as typeof ClipboardItem)

    const container = document.createElement('div')
    document.body.appendChild(container)
    const root = createRoot(container)

    act(() => {
      root.render(<CopyImageButton src="https://cdn.example.com/task.png" />)
    })

    const button = container.querySelector('button')
    expect(button).toBeTruthy()

    await act(async () => {
      button?.dispatchEvent(new MouseEvent('click', { bubbles: true }))
      await Promise.resolve()
    })

    expect(write).toHaveBeenCalledTimes(1)
    expect(clipboardItem).toHaveBeenCalledWith({ 'image/png': imageBlob })
    expect(writeText).not.toHaveBeenCalled()
    expect(toast.success).toHaveBeenCalledWith('Image copied to clipboard')

    act(() => {
      root.unmount()
    })
    container.remove()
  })
})
