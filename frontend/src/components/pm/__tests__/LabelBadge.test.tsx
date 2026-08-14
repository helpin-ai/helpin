// @vitest-environment jsdom
import { act } from 'react'
import { createRoot } from 'react-dom/client'
import { describe, expect, it } from 'vitest'

import { LabelBadge, LabelPicker } from '../LabelPicker'
import { TooltipProvider } from '@/components/ui/tooltip'

(globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true

describe('LabelBadge', () => {
  it('truncates long label names to a single line', () => {
    const container = document.createElement('div')
    document.body.appendChild(container)
    const root = createRoot(container)

    act(() => {
      root.render(
        <LabelBadge
          label={{
            id: 'label-1',
            workspace_id: 'ws-1',
            name: 'Very Long Label Name That Should Truncate Instead Of Expanding Forever',
            color: '#22c55e',
            team_id: null,
            archived: false,
            created_at: '',
            updated_at: '',
          }}
        />,
      )
    })

    const textNode = Array.from(container.querySelectorAll('span')).find(
      (element) =>
        element.textContent?.includes('Very Long Label Name') &&
        element.className.includes('truncate'),
    )
    const badge = container.firstElementChild

    expect(textNode?.className).toContain('truncate')
    expect(textNode?.className).toContain('min-w-0')
    expect(badge?.className).toContain('h-5')
    expect(badge?.className).toContain('text-[11px]')
    expect(badge?.className).not.toContain('text-ui')

    act(() => {
      root.unmount()
    })
    container.remove()
  })

  it('uses a consistent color summary for table labels while keeping Add visible', () => {
    const container = document.createElement('div')
    document.body.appendChild(container)
    const root = createRoot(container)
    const labels = [
      { id: 'label-1', name: 'Frontend', color: '#22c55e' },
      { id: 'label-2', name: 'Customer request', color: '#3b82f6' },
      { id: 'label-3', name: 'High confidence', color: '#f59e0b' },
    ].map((label) => ({
      ...label,
      workspace_id: 'ws-1',
      team_id: null,
      archived: false,
      created_at: '',
      updated_at: '',
    }))

    act(() => {
      root.render(
        <TooltipProvider>
          <LabelPicker
            workspaceId="ws-1"
            selectedLabelIds={labels.map((label) => label.id)}
            labels={labels}
            onChange={() => {}}
            singleLine
          />
        </TooltipProvider>,
      )
    })

    const viewport = container.querySelector('[data-slot="label-picker-viewport"]')
    expect(viewport).toBeTruthy()

    expect(container.querySelector('[aria-label="Frontend, Customer request, High confidence"]')).toBeTruthy()
    expect(container.querySelector('[data-slot="label-picker-measure"]')).toBeNull()
    expect(container.textContent).not.toContain('Frontend')
    expect(Array.from(container.querySelectorAll('button')).some((button) => button.textContent?.trim() === 'Add')).toBe(true)

    act(() => {
      root.unmount()
    })
    container.remove()
  })
})
