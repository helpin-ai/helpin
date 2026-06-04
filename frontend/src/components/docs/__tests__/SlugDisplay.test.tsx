// @vitest-environment jsdom
import { act } from 'react'
import type { ReactNode } from 'react'
import { createRoot } from 'react-dom/client'
import { describe, expect, it, vi } from 'vitest'

vi.mock('@/components/ui/quick-tooltip', () => ({
  QuickTooltip: ({ children, label }: { children: ReactNode; label: string }) => (
    <span data-tooltip={label}>{children}</span>
  ),
}))

vi.mock('sonner', () => ({
  toast: {
    error: vi.fn(),
  },
}))

import { SlugDisplay } from '../SlugDisplay'

(globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true

describe('SlugDisplay', () => {
  it('shows save and cancel tooltips for slug edit actions', () => {
    const container = document.createElement('div')
    document.body.appendChild(container)
    const root = createRoot(container)

    act(() => {
      root.render(<SlugDisplay slug="article-slug" onSlugChange={vi.fn().mockResolvedValue(undefined)} />)
    })

    const editButton = Array.from(container.querySelectorAll('button')).find((button) =>
      button.textContent?.includes('/article-slug'),
    )

    expect(editButton).toBeTruthy()

    act(() => {
      editButton?.dispatchEvent(new MouseEvent('click', { bubbles: true }))
    })

    expect(container.querySelector('[data-tooltip="Save slug"]')).toBeTruthy()
    expect(container.querySelector('[data-tooltip="Cancel"]')).toBeTruthy()

    act(() => {
      root.unmount()
    })
    container.remove()
  })

  it('shows helper text only after a slug save succeeds', async () => {
    const container = document.createElement('div')
    document.body.appendChild(container)
    const root = createRoot(container)
    const onSlugChange = vi.fn().mockResolvedValue(undefined)

    await act(async () => {
      root.render(
        <SlugDisplay
          slug="article-slug"
          onSlugChange={onSlugChange}
          helperText="Slug changes take effect when you publish an update."
        />,
      )
    })

    expect(container.textContent).not.toContain('Slug changes take effect when you publish an update.')

    const editButton = Array.from(container.querySelectorAll('button')).find((button) =>
      button.textContent?.includes('/article-slug'),
    )

    await act(async () => {
      editButton?.dispatchEvent(new MouseEvent('click', { bubbles: true }))
    })

    const input = container.querySelector('input')
    expect(input).toBeTruthy()

    await act(async () => {
      const valueSetter = Object.getOwnPropertyDescriptor(window.HTMLInputElement.prototype, 'value')?.set
      valueSetter?.call(input, 'better-slug')
      input!.dispatchEvent(new InputEvent('input', { bubbles: true, inputType: 'insertText', data: 'better-slug' }))
    })

    const saveButton = container.querySelector('[data-tooltip="Save slug"] button')
    await act(async () => {
      saveButton?.dispatchEvent(new MouseEvent('click', { bubbles: true }))
    })

    expect(onSlugChange).toHaveBeenCalledWith('better-slug')
    expect(container.textContent).toContain('Slug changes take effect when you publish an update.')

    act(() => {
      root.unmount()
    })
    container.remove()
  })
})
