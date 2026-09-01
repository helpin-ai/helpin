// @vitest-environment jsdom

import { act } from 'react'
import { createRoot } from 'react-dom/client'
import { afterEach, describe, expect, it } from 'vitest'

import { SupportInboxPanelHeader } from '../SupportInboxPanelHeader'

;(globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true

describe('SupportInboxPanelHeader', () => {
  afterEach(() => {
    document.body.innerHTML = ''
  })

  it('centralizes the panel height and bottom hairline', () => {
    const container = document.createElement('div')
    document.body.appendChild(container)
    const root = createRoot(container)

    act(() => root.render(<SupportInboxPanelHeader>Inbox</SupportInboxPanelHeader>))

    const header = container.querySelector('[data-slot="support-inbox-panel-header"]')
    expect(header?.classList.contains('h-11')).toBe(true)
    expect(header?.classList.contains('shrink-0')).toBe(true)
    expect(header?.classList.contains('border-b')).toBe(true)
    expect(header?.classList.contains('border-border/60')).toBe(true)
    expect(header?.classList.contains('dark:border-sidebar-border')).toBe(true)
    expect(header?.classList.contains('dark:bg-sidebar')).toBe(true)

    act(() => root.unmount())
  })
})
