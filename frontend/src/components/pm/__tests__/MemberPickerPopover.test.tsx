// @vitest-environment jsdom
import { act } from 'react'
import { createRoot } from 'react-dom/client'
import { describe, expect, it, vi } from 'vitest'

import * as MemberPickerModule from '../MemberPickerPopover'
import type { AssignableMember } from '@/lib/types'

(globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true
vi.stubGlobal(
  'ResizeObserver',
  class ResizeObserver {
    observe() {}
    unobserve() {}
    disconnect() {}
  },
)
window.HTMLElement.prototype.scrollIntoView = vi.fn()

const members: AssignableMember[] = [
  {
    id: 'member-1',
    user_id: 'user-1',
    role: 'member',
    email: 'alice@example.com',
    display_name: 'Alice Johnson',
    avatar_url: 'https://cdn.example.com/alice.png',
    status: 'active',
  },
  {
    id: 'member-2',
    user_id: 'user-2',
    role: 'member',
    email: 'pending@example.com',
    display_name: 'Pending Invite',
    status: 'pending',
  },
  {
    id: 'member-3',
    user_id: 'user-3',
    role: 'member',
    email: 'marco@example.com',
    display_name: 'Marco Rivera',
    status: 'active',
  },
]

describe('MemberPickerPopover', () => {
  it('shows active members with avatars and hides pending invites in the single-select picker', () => {
    const handleChange = vi.fn()
    const container = document.createElement('div')
    document.body.appendChild(container)
    const root = createRoot(container)

    act(() => {
      root.render(
        <MemberPickerModule.MemberPickerPopover
          value="__none__"
          members={members}
          onChange={handleChange}
          noneLabel="No owner"
          renderTrigger={() => <span>Assign owner</span>}
        />,
      )
    })

    const trigger = container.querySelector('button')
    expect(trigger).toBeTruthy()

    act(() => {
      trigger?.dispatchEvent(new MouseEvent('click', { bubbles: true }))
    })

    expect(document.body.textContent).toContain('Alice Johnson')
    expect(document.body.textContent).toContain('Marco Rivera')
    expect(document.body.textContent).not.toContain('Pending Invite')
    expect(document.body.querySelector('input[placeholder="Search members..."]')).toBeTruthy()
    expect(document.body.textContent).toContain('AJ')

    act(() => {
      root.unmount()
    })
    container.remove()
  })

  it('treats a stored user_id value as the selected member in the shared picker', () => {
    const handleChange = vi.fn()
    const container = document.createElement('div')
    document.body.appendChild(container)
    const root = createRoot(container)

    act(() => {
      root.render(
        <MemberPickerModule.MemberPickerPopover
          value="user-1"
          members={members}
          onChange={handleChange}
          renderTrigger={() => <span>Assign owner</span>}
          open
        />,
      )
    })

    const aliceOption = Array.from(document.querySelectorAll('[cmdk-item]')).find((element) =>
      element.textContent?.includes('Alice Johnson'),
    )
    expect(aliceOption?.querySelector('svg')).toBeTruthy()

    act(() => {
      root.unmount()
    })
    container.remove()
  })

  it('keeps the selected trigger content constrained to a single line', () => {
    const handleChange = vi.fn()
    const container = document.createElement('div')
    document.body.appendChild(container)
    const root = createRoot(container)

    act(() => {
      root.render(
        <MemberPickerModule.MemberPickerPopover
          value="member-1"
          members={members}
          onChange={handleChange}
          renderTrigger={() => (
            <>
              <span>AJ</span>
              <span>Alexandria Johnson With A Very Long Name That Should Truncate</span>
            </>
          )}
        />,
      )
    })

    const trigger = container.querySelector('button')
    const triggerContent = trigger?.querySelector('span')

    expect(trigger?.className).toContain('min-w-0')
    expect(triggerContent?.className).toContain('[&_span:last-child]:truncate')
    expect(triggerContent?.className).toContain('overflow-hidden')

    act(() => {
      root.unmount()
    })
    container.remove()
  })

  it('exports a multi-select member picker that toggles active selections and ignores pending invites', () => {
    const MultiMemberPickerPopover = (
      MemberPickerModule as typeof MemberPickerModule & {
        MultiMemberPickerPopover?: React.ComponentType<{
          values: string[]
          members: AssignableMember[]
          onChange: (values: string[]) => void
          renderTrigger: () => React.ReactNode
        }>
      }
    ).MultiMemberPickerPopover

    expect(MultiMemberPickerPopover).toBeTypeOf('function')
    if (!MultiMemberPickerPopover) return

    const handleChange = vi.fn()
    const container = document.createElement('div')
    document.body.appendChild(container)
    const root = createRoot(container)

    act(() => {
      root.render(
        <MultiMemberPickerPopover
          values={['member-1']}
          members={members}
          onChange={handleChange}
          renderTrigger={() => <span>Owners</span>}
        />,
      )
    })

    const trigger = container.querySelector('button')
    expect(trigger).toBeTruthy()

    act(() => {
      trigger?.dispatchEvent(new MouseEvent('click', { bubbles: true }))
    })

    expect(document.body.textContent).toContain('Alice Johnson')
    expect(document.body.textContent).toContain('Marco Rivera')
    expect(document.body.textContent).not.toContain('Pending Invite')

    const marcoOption = Array.from(document.querySelectorAll('[cmdk-item]')).find((element) =>
      element.textContent?.includes('Marco Rivera'),
    )
    expect(marcoOption).toBeTruthy()

    act(() => {
      marcoOption?.dispatchEvent(new MouseEvent('click', { bubbles: true }))
    })

    expect(handleChange).toHaveBeenCalledWith(['member-1', 'member-3'])

    act(() => {
      root.unmount()
    })
    container.remove()
  })
})
