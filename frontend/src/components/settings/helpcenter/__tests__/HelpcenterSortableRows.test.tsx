// @vitest-environment jsdom
import { act } from 'react'
import type { ButtonHTMLAttributes, InputHTMLAttributes, ReactNode } from 'react'
import { createRoot } from 'react-dom/client'
import { describe, expect, it, vi } from 'vitest'

vi.mock('@dnd-kit/sortable', () => ({
  useSortable: () => ({
    attributes: {},
    listeners: {},
    setNodeRef: vi.fn(),
    transform: null,
    transition: undefined,
    isDragging: false,
  }),
}))

vi.mock('@dnd-kit/utilities', () => ({
  CSS: {
    Transform: {
      toString: () => '',
    },
  },
}))

vi.mock('@/components/ui/button', () => ({
  Button: ({ children, ...props }: ButtonHTMLAttributes<HTMLButtonElement>) => <button {...props}>{children}</button>,
}))

vi.mock('@/components/ui/input', () => ({
  Input: (props: InputHTMLAttributes<HTMLInputElement>) => <input {...props} />,
}))

vi.mock('@/components/ui/select', () => ({
  Select: ({ children }: { children: ReactNode }) => <div>{children}</div>,
  SelectTrigger: ({ children }: { children: ReactNode }) => <button type="button">{children}</button>,
  SelectValue: ({ children }: { children?: ReactNode }) => <span>{children ?? 'GitHub'}</span>,
  SelectContent: ({ children }: { children: ReactNode }) => <div role="listbox">{children}</div>,
  SelectItem: ({ children, value }: { children: ReactNode; value: string }) => (
    <div role="option" data-value={value}>{children}</div>
  ),
}))

import { SortableSocialLinkRow } from '../HelpcenterSortableRows'

(globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true

describe('SortableSocialLinkRow', () => {
  it('shows public footer icons next to social platform options', () => {
    const container = document.createElement('div')
    document.body.appendChild(container)
    const root = createRoot(container)

    act(() => {
      root.render(
        <SortableSocialLinkRow
          id={0}
          link={{ platform: 'github', url: 'https://github.com/helpin-ai' }}
          onUpdate={vi.fn()}
          onRemove={vi.fn()}
        />,
      )
    })

    const githubOption = container.querySelector('[role="option"][data-value="github"]')
    expect(githubOption?.textContent).toContain('GitHub')
    const githubIcon = githubOption?.querySelector('[data-social-brand-icon="github"]') as HTMLElement | null
    expect(githubIcon?.style.maskImage).toContain('/brands/github.svg')
    expect(githubOption?.querySelector('svg')).toBeNull()

    const trigger = Array.from(container.querySelectorAll('button')).find((button) => button.textContent?.includes('GitHub'))
    expect(trigger?.textContent?.match(/GitHub/g)).toHaveLength(1)

    const xOption = container.querySelector('[role="option"][data-value="x"]')
    expect(xOption?.textContent).toContain('X')
    const xIcon = xOption?.querySelector('[data-social-brand-icon="x"]') as HTMLElement | null
    expect(xIcon?.style.maskImage).toContain('/brands/x-twitter.svg')
    expect(xOption?.querySelector('svg')).toBeNull()

    const websiteOption = container.querySelector('[role="option"][data-value="website"]')
    expect(websiteOption?.textContent).toContain('Website')
    expect(websiteOption?.querySelector('svg')).not.toBeNull()

    act(() => {
      root.unmount()
    })
    container.remove()
  })
})
