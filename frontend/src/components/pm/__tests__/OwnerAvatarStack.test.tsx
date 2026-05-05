// @vitest-environment jsdom
import { createRoot } from 'react-dom/client'
import { act } from 'react'
import { describe, expect, it } from 'vitest'

import { OwnerAvatarStack } from '../OwnerAvatarStack'

(globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true

describe('OwnerAvatarStack', () => {
  it('shows avatar and name inline for one owner', () => {
    const container = document.createElement('div')
    document.body.appendChild(container)
    const root = createRoot(container)
    const nameMap = new Map([['member-a', 'Alice Adams']])

    act(() => {
      root.render(<OwnerAvatarStack memberIds={['member-a']} nameMap={nameMap} />)
    })

    expect(container.querySelectorAll('[data-owner-avatar]').length).toBe(1)
    expect(container.querySelector('[data-owner-single-name]')?.textContent).toBe('Alice Adams')
    expect(container.querySelector('[data-owner-hover-list]')).toBeNull()

    act(() => root.unmount())
    container.remove()
  })

  it('can render one owner as avatar-only for compact cards', () => {
    const container = document.createElement('div')
    document.body.appendChild(container)
    const root = createRoot(container)
    const nameMap = new Map([['member-a', 'Alice Adams']])

    act(() => {
      root.render(<OwnerAvatarStack memberIds={['member-a']} nameMap={nameMap} showSingleName={false} />)
    })

    expect(container.querySelectorAll('[data-owner-avatar]').length).toBe(1)
    expect(container.querySelector('[data-owner-single-name]')).toBeNull()
    expect(container.querySelector('[data-owner-hover-list]')?.textContent).toContain('Alice Adams')

    act(() => root.unmount())
    container.remove()
  })

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
    expect(container.querySelector('[data-owner-overflow]')?.textContent).toBe('+1')
    expect(container.querySelector('[data-owner-overflow]')?.className).toContain('ring-2')
    expect(container.querySelector('[data-owner-single-name]')).toBeNull()
    expect(container.querySelector('[data-owner-avatar-stack]')?.getAttribute('title')).toBe(
      'Alice Adams, Bina Brooks, Chen Carter, Dev Diaz',
    )
    expect(container.querySelector('[data-owner-hover-list]')?.textContent).toContain('Alice Adams')
    expect(container.querySelector('[data-owner-hover-list]')?.textContent).toContain('Dev Diaz')

    act(() => root.unmount())
    container.remove()
  })
})
