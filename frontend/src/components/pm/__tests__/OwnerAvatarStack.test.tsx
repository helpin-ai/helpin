// @vitest-environment jsdom
import { createRoot } from 'react-dom/client'
import { act } from 'react'
import { describe, expect, it } from 'vitest'

import { OwnerAvatarStack } from '../OwnerAvatarStack'

(globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true

describe('OwnerAvatarStack', () => {
  it('renders three visible owners plus overflow with all names in the title', () => {
    const container = document.createElement('div')
    document.body.appendChild(container)
    const root = createRoot(container)
    const nameMap = new Map([
      ['member-a', 'Alice Adams'],
      ['member-b', 'Bina Brooks'],
      ['member-c', 'Chen Carter'],
      ['member-d', 'Dev Diaz'],
    ])

    act(() => {
      root.render(<OwnerAvatarStack memberIds={['member-a', 'member-b', 'member-c', 'member-d']} nameMap={nameMap} />)
    })

    expect(container.querySelectorAll('[data-owner-avatar]').length).toBe(3)
    expect(container.textContent).toContain('+1')
    expect(container.querySelector('[data-owner-avatar-stack]')?.getAttribute('title')).toBe(
      'Alice Adams, Bina Brooks, Chen Carter, Dev Diaz',
    )

    act(() => root.unmount())
    container.remove()
  })
})
