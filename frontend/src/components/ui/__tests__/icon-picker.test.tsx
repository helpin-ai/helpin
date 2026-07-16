// @vitest-environment jsdom
import { act, type ReactNode } from 'react'
import { createRoot } from 'react-dom/client'
import { afterEach, describe, expect, it, vi } from 'vitest'

import { IconPicker, StoredIcon } from '@/components/ui/icon-picker'

;(globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true

function renderNode(node: ReactNode) {
  const container = document.createElement('div')
  document.body.appendChild(container)
  const root = createRoot(container)
  act(() => root.render(node))
  return {
    container,
    cleanup: () => {
      act(() => root.unmount())
      container.remove()
    },
  }
}

describe('StoredIcon', () => {
  afterEach(() => {
    vi.unstubAllGlobals()
    document.body.innerHTML = ''
  })

  it('renders canonical and legacy values from static assets without loading the manifest', () => {
    const fetchMock = vi.fn()
    vi.stubGlobal('fetch', fetchMock)

    const canonical = renderNode(<StoredIcon name="rocket01" className="h-4 w-4" />)
    expect((canonical.container.firstElementChild as HTMLElement).style.maskImage).toContain(
      '/assets/helpin-icons/hugeicons/4.1.1/rocket01.svg',
    )
    canonical.cleanup()

    const legacy = renderNode(<StoredIcon name="gear-six" className="h-4 w-4" />)
    expect((legacy.container.firstElementChild as HTMLElement).style.maskImage).toContain(
      '/assets/helpin-icons/hugeicons/4.1.1/settings.svg',
    )
    legacy.cleanup()
    expect(fetchMock).not.toHaveBeenCalled()
  })

  it('preserves non-icon display text', () => {
    const rendered = renderNode(<StoredIcon name="🚀" />)
    expect(rendered.container.textContent).toBe('🚀')
    rendered.cleanup()
  })
})

describe('IconPicker', () => {
  afterEach(() => {
    vi.unstubAllGlobals()
    document.body.innerHTML = ''
  })

  it('loads the searchable catalog only on interaction intent', async () => {
    const fetchMock = vi.fn().mockResolvedValue({
      ok: true,
      json: async () => ({
        version: '4.1.1',
        icons: [{ id: 'rocket01', label: 'Rocket 01' }],
      }),
    })
    vi.stubGlobal('fetch', fetchMock)

    const rendered = renderNode(<IconPicker value="" onChange={() => undefined} />)
    expect(fetchMock).not.toHaveBeenCalled()

    const trigger = rendered.container.querySelector('[role="combobox"]') as HTMLButtonElement
    await act(async () => {
      trigger.focus()
      await Promise.resolve()
    })
    expect(fetchMock).toHaveBeenCalledTimes(1)
    expect(fetchMock.mock.calls[0]?.[0]).toContain('/assets/helpin-icons/catalog.json')
    rendered.cleanup()
  })
})
